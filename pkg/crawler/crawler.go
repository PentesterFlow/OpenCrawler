package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/PentesterFlow/OpenCrawler/internal/auth"
	"github.com/PentesterFlow/OpenCrawler/internal/browser"
	"github.com/PentesterFlow/OpenCrawler/internal/discovery"
	"github.com/PentesterFlow/OpenCrawler/internal/discovery/enhanced"
	"github.com/PentesterFlow/OpenCrawler/internal/framework"
	fasthttp "github.com/PentesterFlow/OpenCrawler/internal/http"
	"github.com/PentesterFlow/OpenCrawler/internal/logger"
	"github.com/PentesterFlow/OpenCrawler/internal/metrics"
	"github.com/PentesterFlow/OpenCrawler/internal/output"
	"github.com/PentesterFlow/OpenCrawler/internal/progress"
	"github.com/PentesterFlow/OpenCrawler/internal/shutdown"
	"github.com/PentesterFlow/OpenCrawler/internal/parser"
	"github.com/PentesterFlow/OpenCrawler/internal/queue"
	"github.com/PentesterFlow/OpenCrawler/internal/ratelimit"
	"github.com/PentesterFlow/OpenCrawler/internal/scope"
	"github.com/PentesterFlow/OpenCrawler/internal/state"
	"github.com/PentesterFlow/OpenCrawler/internal/websocket"
)

// Crawler is the main crawler orchestrator.
type Crawler struct {
	config           *Config
	browserPool      *browser.Pool
	fastClient       *fasthttp.FastClient  // Fast HTTP client for non-JS pages
	queue            *queue.FastQueue
	state            *state.Manager
	scope            *scope.Checker
	limiter          *ratelimit.Limiter
	auth             auth.Provider
	output           output.Writer
	outputWriter     io.Writer
	htmlParser        *parser.HTMLParser
	formAnalyzer      *parser.FormAnalyzer
	jsParser          *parser.JSParser
	passiveDiscovery  *discovery.PassiveDiscovery
	activeDiscovery   *discovery.ActiveDiscovery
	enhancedDiscovery *enhanced.EnhancedDiscovery
	wsHandler         *websocket.Handler
	logger           *logger.Logger
	metrics          *metrics.Collector
	shutdownHandler  *shutdown.Handler

	mu          sync.RWMutex
	running     atomic.Bool
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	startTime   time.Time
	results     *CrawlResult
	resultsChan chan interface{}
	errorsChan  chan CrawlError

	// Turbo mode stats
	httpPages    atomic.Int64  // Pages fetched via HTTP
	browserPages atomic.Int64  // Pages fetched via browser

	// Progress display
	progress     *progress.Display
	showProgress bool
}

// New creates a new crawler with the given options.
func New(opts ...Option) (*Crawler, error) {
	c := &Crawler{
		config:       DefaultConfig(),
		resultsChan:  make(chan interface{}, 1000),
		errorsChan:   make(chan CrawlError, 100),
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// Validate config
	if err := c.config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Initialize logger based on config
	logLevel := logger.InfoLevel
	if c.config.Debug {
		logLevel = logger.DebugLevel
	} else if !c.config.Verbose {
		logLevel = logger.WarnLevel
	}
	c.logger = logger.New(logger.Config{
		Level:     logLevel,
		Pretty:    true,
		Component: "crawler",
	})

	// Initialize metrics collector
	c.metrics = metrics.New()

	// Initialize shutdown handler with callbacks for log and cleanup
	c.shutdownHandler = shutdown.New(shutdown.Config{
		Timeout: c.config.Timeout * 2, // Double timeout for graceful shutdown
		OnShutdownStart: func() {
			c.logger.Info("Graceful shutdown initiated...")
		},
		OnShutdownDone: func(elapsed time.Duration, errs []error) {
			if len(errs) > 0 {
				c.logger.Warnf("Shutdown completed in %v with %d errors", elapsed, len(errs))
			} else {
				c.logger.Infof("Shutdown completed in %v", elapsed)
			}
		},
	})

	return c, nil
}

// convertAuthCreds converts crawler.AuthCredentials to auth.Credentials
func convertAuthCreds(creds AuthCredentials) auth.Credentials {
	var oauthConfig *auth.OAuthConfig
	if creds.OAuthConfig != nil {
		oauthConfig = &auth.OAuthConfig{
			ClientID:     creds.OAuthConfig.ClientID,
			ClientSecret: creds.OAuthConfig.ClientSecret,
			AuthURL:      creds.OAuthConfig.AuthURL,
			TokenURL:     creds.OAuthConfig.TokenURL,
			RedirectURL:  creds.OAuthConfig.RedirectURL,
			Scopes:       creds.OAuthConfig.Scopes,
		}
	}

	return auth.Credentials{
		Type:        auth.AuthType(creds.Type),
		Username:    creds.Username,
		Password:    creds.Password,
		Token:       creds.Token,
		Headers:     creds.Headers,
		Cookies:     creds.Cookies,
		LoginURL:    creds.LoginURL,
		FormFields:  creds.FormFields,
		OAuthConfig: oauthConfig,
	}
}

// initialize sets up all crawler components.
func (c *Crawler) initialize() error {
	var err error

	// Create scope checker
	scopeRules := scope.ScopeRules{
		IncludePatterns: c.config.Scope.IncludePatterns,
		ExcludePatterns: c.config.Scope.ExcludePatterns,
		AllowedDomains:  c.config.Scope.AllowedDomains,
		MaxDepth:        c.config.Scope.MaxDepth,
		FollowExternal:  c.config.Scope.FollowExternal,
	}
	c.scope, err = scope.NewChecker(c.config.Target, scopeRules)
	if err != nil {
		return fmt.Errorf("failed to create scope checker: %w", err)
	}

	// Create browser pool
	c.browserPool, err = browser.NewPool(c.config.Browser)
	if err != nil {
		return fmt.Errorf("failed to create browser pool: %w", err)
	}

	// Create fast HTTP client for non-JS pages (turbo mode)
	c.fastClient = fasthttp.NewFastClient(fasthttp.FastClientConfig{
		Timeout:             c.config.Timeout,
		MaxIdleConns:        500,
		MaxIdleConnsPerHost: 100,
		MaxConnsPerHost:     100,
		UserAgent:           c.config.Browser.UserAgent,
		Headers:             c.config.CustomHeaders,
		SkipTLSVerify:       c.config.Browser.IgnoreHTTPSErrors,
	})

	// Create high-performance queue
	c.queue = queue.NewFastQueue(100000)

	// Create state manager
	var store state.Store
	if c.config.State.Enabled && c.config.State.FilePath != "" {
		store, err = state.NewBoltStore(c.config.State.FilePath)
		if err != nil {
			return fmt.Errorf("failed to create state store: %w", err)
		}
	}
	c.state = state.NewManager(store, 100000)
	c.state.SetTarget(c.config.Target)

	// Create rate limiter
	c.limiter = ratelimit.NewLimiter(
		c.config.RateLimit.RequestsPerSecond,
		c.config.RateLimit.Burst,
	)

	// Create auth provider
	c.auth, err = auth.NewProvider(convertAuthCreds(c.config.Auth))
	if err != nil {
		return fmt.Errorf("failed to create auth provider: %w", err)
	}

	// Create parsers
	c.htmlParser, err = parser.NewHTMLParser(c.config.Target)
	if err != nil {
		return fmt.Errorf("failed to create HTML parser: %w", err)
	}
	c.formAnalyzer = parser.NewFormAnalyzer()
	c.jsParser = parser.NewJSParser()

	// Create discovery components
	c.passiveDiscovery = discovery.NewPassiveDiscovery()
	c.activeDiscovery = discovery.NewActiveDiscovery(
		c.config.Browser.UserAgent,
		c.config.CustomHeaders,
	)

	// Create WebSocket handler
	c.wsHandler = websocket.NewHandler()
	if c.config.CustomHeaders != nil {
		c.wsHandler.SetHeaders(c.config.CustomHeaders)
	}

	// Create enhanced discovery
	if c.config.EnhancedDiscovery.Enabled {
		c.enhancedDiscovery = enhanced.NewEnhancedDiscovery(enhanced.Config{
			UserAgent:         c.config.Browser.UserAgent,
			Concurrency:       c.config.EnhancedDiscovery.Concurrency,
			EnableAll:         false, // Use individual settings
			EnableRobots:      c.config.EnhancedDiscovery.EnableRobots,
			EnableSitemap:     c.config.EnhancedDiscovery.EnableSitemap,
			EnableSourceMaps:  c.config.EnhancedDiscovery.EnableSourceMaps,
			EnablePathBrute:   c.config.EnhancedDiscovery.EnablePathBrute,
			EnableFingerprint: c.config.EnhancedDiscovery.EnableFingerprint,
			EnableParamDiscov: c.config.EnhancedDiscovery.EnableParamDiscovery,
			EnableJSExtract:   c.config.EnhancedDiscovery.EnableJSExtract,
		})
	}

	// Setup output writer
	if c.outputWriter == nil {
		if c.config.Output.FilePath != "" {
			f, err := os.Create(c.config.Output.FilePath)
			if err != nil {
				return fmt.Errorf("failed to create output file: %w", err)
			}
			c.outputWriter = f
		} else {
			c.outputWriter = os.Stdout
		}
	}

	c.output = output.NewWriter(c.outputWriter, output.Config{
		Format: c.config.Output.Format,
		Pretty: c.config.Output.Pretty,
		Stream: c.config.Output.StreamMode,
	})

	// Initialize results
	c.results = &CrawlResult{
		Target:    c.config.Target,
		StartedAt: time.Now(),
		Stats:     CrawlStats{},
		Endpoints: make([]Endpoint, 0),
		Forms:     make([]Form, 0),
		WebSockets: make([]WebSocketEndpoint, 0),
		Errors:    make([]CrawlError, 0),
	}

	return nil
}

// Start begins the crawling process.
func (c *Crawler) Start(ctx context.Context) (*CrawlResult, error) {
	if c.running.Load() {
		return nil, fmt.Errorf("crawler is already running")
	}

	c.mu.Lock()
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.startTime = time.Now()
	c.running.Store(true)
	c.mu.Unlock()

	defer func() {
		c.running.Store(false)
	}()

	// Initialize components
	if err := c.initialize(); err != nil {
		return nil, err
	}
	defer c.cleanup()

	// Initialize progress display if enabled
	if c.showProgress {
		c.progress = progress.New()
		c.progress.Start(c.config.Target)
		defer func() {
			c.progress.Stop()
			c.progress.PrintSummary()
		}()
	}

	// Register cleanup callbacks with shutdown handler
	c.registerShutdownCallbacks()

	// Start listening for shutdown signals in background
	go c.shutdownHandler.WaitWithContext(c.ctx)

	// Authenticate if needed
	if c.auth.Type() != auth.AuthTypeNone {
		if err := c.auth.Authenticate(c.ctx, c.browserPool); err != nil {
			return nil, fmt.Errorf("authentication failed: %w", err)
		}
	}

	// Perform active API discovery first
	if c.config.ActiveAPIDiscovery {
		c.runActiveDiscovery()
	}

	// Perform enhanced discovery
	if c.enhancedDiscovery != nil {
		c.runEnhancedDiscovery()
	}

	// Add seed URL to queue
	seedItem := &queue.QueueItem{
		URL:       c.config.Target,
		Method:    "GET",
		Depth:     0,
		Timestamp: time.Now(),
	}
	c.queue.Push(seedItem)

	// Start workers
	workerCount := c.config.Workers
	if workerCount > c.config.Browser.PoolSize {
		workerCount = c.config.Browser.PoolSize
	}

	for i := 0; i < workerCount; i++ {
		c.wg.Add(1)
		go c.worker(i)
	}

	// Start periodic status reporter if verbose or progress is enabled
	if c.config.Verbose || c.showProgress {
		go c.statusReporter()
	}

	// Wait for completion
	c.wg.Wait()

	// Finalize results
	c.mu.Lock()
	c.results.CompletedAt = time.Now()
	c.results.Stats = convertStateCrawlStats(c.state.GetStats())
	c.results.Endpoints = c.collectEndpoints()
	c.results.Forms = c.collectForms()
	c.results.WebSockets = convertWebSocketEndpoints(c.wsHandler.GetEndpoints())
	c.mu.Unlock()

	// Write output
	if err := c.output.WriteResult(c.convertToOutputResult(c.results)); err != nil {
		return c.results, fmt.Errorf("failed to write output: %w", err)
	}

	return c.results, nil
}

// worker is a crawl worker with batch processing support.
func (c *Crawler) worker(id int) {
	defer c.wg.Done()

	emptyCount := 0
	maxEmptyChecks := 15
	batchSize := 5 // Process items in batches to reduce lock overhead

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		// Try to get a batch of items
		items, err := c.queue.PopBatch(batchSize)
		if err != nil {
			if c.queue.IsEmpty() {
				emptyCount++
				if emptyCount >= maxEmptyChecks {
					// Queue has been empty for a while, exit
					return
				}
				// Progressive backoff
				sleepTime := time.Duration(20+emptyCount*10) * time.Millisecond
				time.Sleep(sleepTime)
				continue
			}
			continue
		}

		// Reset empty counter when we get work
		emptyCount = 0

		// Process batch
		for _, item := range items {
			select {
			case <-c.ctx.Done():
				return
			default:
			}

			// Check scope
			if !c.scope.IsInScope(item.URL, item.Depth) {
				continue
			}

			// Check if already visited
			if c.state.HasVisited(item.URL) {
				continue
			}

			// Rate limit
			if err := c.limiter.WaitDomain(c.ctx, getDomain(item.URL)); err != nil {
				continue
			}

			// Process URL
			c.processURL(item)
		}
	}
}

// log logs an info message.
func (c *Crawler) log(format string, args ...interface{}) {
	c.logger.Infof(format, args...)
}

// logDebug logs a debug message.
func (c *Crawler) logDebug(format string, args ...interface{}) {
	c.logger.Debugf(format, args...)
}

// processURL processes a single URL.
func (c *Crawler) processURL(item *queue.QueueItem) {
	// Normalize URL for hash-based SPAs
	normalizedURL := c.state.NormalizeURL(item.URL)

	// Check for UI state fragments that should be skipped
	if parsed, err := url.Parse(item.URL); err == nil && parsed.Fragment != "" {
		if c.state.ShouldSkipFragment(parsed.Fragment) {
			c.logDebug("Skipping UI state fragment: %s", item.URL)
			return
		}
	}

	// Mark as visited
	c.state.MarkVisited(normalizedURL)
	c.state.AddDiscoveredURL()

	// Record metrics
	c.metrics.RecordPageDiscovered()

	c.log("Crawling: %s (depth: %d)", item.URL, item.Depth)

	// Get auth headers and cookies
	headers := c.auth.GetHeaders()
	if headers == nil {
		headers = make(map[string]string)
	}
	for k, v := range c.config.CustomHeaders {
		headers[k] = v
	}
	cookies := c.auth.GetCookies()

	// Check if this is a hash-based SPA route (always needs browser)
	isHash := isHashRoute(item.URL)

	// Use fast mode for deep pages (depth > 2) or if configured
	fastMode := c.config.FastMode || item.Depth > 2

	var result *browser.PageResult
	var err error
	var needsBrowser bool

	// HTTP-FIRST STRATEGY: Always try HTTP first for non-hash URLs
	// This is much faster and works for most pages
	if !isHash && c.fastClient != nil {
		c.metrics.RecordRequest()
		httpResult, httpErr := c.fastClient.Get(c.ctx, item.URL)
		if httpErr == nil && httpResult.StatusCode >= 200 && httpResult.StatusCode < 400 {
			// Record metrics for successful HTTP fetch
			c.metrics.RecordStatusCode(httpResult.StatusCode)
			c.metrics.RecordResponseTime(httpResult.Duration)
			// Check if page likely needs JavaScript rendering
			needsBrowser = c.needsJavaScriptRendering(httpResult.HTML, httpResult.ContentType)

			if !needsBrowser || fastMode {
				// Convert to browser.PageResult format
				result = &browser.PageResult{
					URL:         httpResult.URL,
					FinalURL:    httpResult.FinalURL,
					StatusCode:  httpResult.StatusCode,
					ContentType: httpResult.ContentType,
					HTML:        httpResult.HTML,
					Title:       httpResult.Title,
					Links:       httpResult.Links,
					Scripts:     httpResult.Scripts,
					Forms:       make([]browser.FormData, 0, len(httpResult.Forms)),
					XHRRequests: make([]browser.NetworkRequest, 0),
					Framework:   &framework.DetectionResult{},
				}
				// Convert forms
				for _, f := range httpResult.Forms {
					form := browser.FormData{
						Action: f.Action,
						Method: f.Method,
						Inputs: make([]browser.InputData, 0, len(f.Inputs)),
					}
					for _, inputName := range f.Inputs {
						form.Inputs = append(form.Inputs, browser.InputData{Name: inputName, Type: "text"})
					}
					result.Forms = append(result.Forms, form)
				}
				c.httpPages.Add(1)
				c.logDebug("[HTTP] Fast fetch: %s (%d links)", item.URL, len(result.Links))
			}
		}
	}

	// BROWSER PATH: Use headless browser only for:
	// 1. Hash-based SPA routes
	// 2. Pages that need JS rendering (and not in fast mode)
	// 3. When HTTP fetch failed
	if result == nil {
		// Configure visit options
		visitOpts := browser.VisitOptions{
			FastMode:       fastMode,
			SPAMode:        !fastMode,           // Enable SPA handling for non-fast mode
			EnableStealth:  true,                // Always enable stealth mode
			CheckSoftError: !fastMode,           // Check for soft 404s (skip in fast mode)
		}

		if isHash {
			baseURL, hashRoute := splitHashURL(item.URL)
			result, err = c.browserPool.VisitHashRouteWithOptions(c.ctx, baseURL, hashRoute, headers, cookies, visitOpts)
		} else {
			result, err = c.browserPool.VisitWithOptions(c.ctx, item.URL, headers, cookies, visitOpts)
		}
		if err != nil {
			c.addError(item.URL, err)
			return
		}
		c.browserPages.Add(1)
	}

	if result.Error != nil {
		c.log("Error on %s: %v", item.URL, result.Error)
		c.addError(item.URL, result.Error)
		return
	}

	// Handle soft 404 pages
	if result.IsSoftError {
		c.log("[SOFT-404] %s: %s", item.URL, result.SoftErrorMsg)
		c.state.MarkSoftError(item.URL, result.SoftErrorMsg)
		// Still process links but mark it as an error
		c.addError(item.URL, fmt.Errorf("soft 404: %s", result.SoftErrorMsg))
	}

	// Check for auth redirect
	if result.IsAuthPage {
		c.log("[AUTH] Redirected to auth page: %s", result.AuthURL)
		// Don't continue crawling from auth pages
		return
	}

	// Check for duplicate content using content hash
	if result.ContentHash != "" {
		isDupe, dupeURL := c.state.HasDuplicateContent(item.URL, result.ContentHash)
		if isDupe {
			c.logDebug("[DEDUP] Duplicate content: %s same as %s", item.URL, dupeURL)
			// Skip processing but still mark as visited
			return
		}
		// Store the content hash
		c.state.SetContentHash(item.URL, result.ContentHash)
	}

	// Log framework detection
	if result.Framework != nil && result.Framework.IsSPA {
		c.log("Detected SPA framework: %s (routes: %d, links: %d)", result.Framework.Primary, len(result.FrameworkRoutes), len(result.FrameworkLinks))
	}

	c.log("Found %d links, %d forms, %d XHR requests, %d AJAX endpoints on %s",
		len(result.Links), len(result.Forms), len(result.XHRRequests), len(result.AJAXEndpoints), item.URL)

	// Process passive API discovery
	if c.config.PassiveAPIDiscovery && len(result.XHRRequests) > 0 {
		endpoints := c.passiveDiscovery.ProcessRequests(result.XHRRequests, item.URL)
		c.addDiscoveryEndpoints(endpoints)
	}

	// Process AJAX endpoints
	if len(result.AJAXEndpoints) > 0 {
		c.processAJAXEndpoints(result.AJAXEndpoints, item)
	}

	// Process AJAX forms
	if len(result.AJAXForms) > 0 {
		c.processAJAXForms(result.AJAXForms, item)
	}

	// Queue dynamically loaded content URLs
	if len(result.DynamicContent) > 0 {
		c.logDebug("[AJAX] Found %d dynamically loaded URLs", len(result.DynamicContent))
		for _, dynamicURL := range result.DynamicContent {
			if c.scope.IsInScope(dynamicURL, item.Depth+1) && !c.state.HasVisited(dynamicURL) {
				queueItem := &queue.QueueItem{
					URL:       dynamicURL,
					Method:    "GET",
					Depth:     item.Depth + 1,
					ParentURL: item.URL,
					Timestamp: time.Now(),
				}
				c.queue.Push(queueItem)
			}
		}
	}

	// Process framework routes
	if result.Framework != nil && result.Framework.IsSPA {
		c.processFrameworkRoutes(result.FrameworkRoutes, result.FrameworkLinks, item)
	}

	// Process Shadow DOM links
	if len(result.ShadowDOMLinks) > 0 {
		c.log("[SHADOW-DOM] Found %d links in Shadow DOM", len(result.ShadowDOMLinks))
		for _, shadowLink := range result.ShadowDOMLinks {
			// Resolve the URL
			resolvedURL, err := scope.ResolveURL(item.URL, shadowLink)
			if err != nil {
				continue
			}
			if c.scope.IsInScope(resolvedURL, item.Depth+1) && !c.state.HasVisited(resolvedURL) {
				queueItem := &queue.QueueItem{
					URL:       resolvedURL,
					Method:    "GET",
					Depth:     item.Depth + 1,
					ParentURL: item.URL,
					Timestamp: time.Now(),
				}
				c.queue.Push(queueItem)
			}
		}
	}

	// Run enhanced discovery on the page (runs in background for efficiency)
	if c.enhancedDiscovery != nil && item.Depth == 0 {
		go c.runEnhancedDiscoveryOnPage(result, item.URL)
	}

	// Parse HTML
	if result.HTML != "" {
		c.processHTML(result.HTML, item)
	}

	// Process forms
	if c.config.FormAnalysis && len(result.Forms) > 0 {
		c.processForms(result.Forms, item)
	}

	// Process WebSocket URLs
	if c.config.WebSocketDiscovery && len(result.WebSockets) > 0 {
		for _, wsURL := range result.WebSockets {
			c.log("[WEBSOCKET] Discovered: %s (from: %s)", wsURL, item.URL)
			go c.wsHandler.Connect(c.ctx, wsURL, item.URL)
		}
	}

	// Update cookies from response
	if len(result.Cookies) > 0 {
		if sessionAuth, ok := c.auth.(*auth.SessionAuth); ok {
			for _, cookie := range result.Cookies {
				sessionAuth.AddCookie(cookie)
			}
		}
	}
}

// processFrameworkRoutes processes routes and links discovered by framework handlers.
func (c *Crawler) processFrameworkRoutes(routes []framework.Route, links []framework.Link, parent *queue.QueueItem) {
	baseURL := c.config.Target
	items := make([]*queue.QueueItem, 0, len(routes)+len(links))

	// Process discovered routes
	for _, route := range routes {
		// Skip routes with unresolved parameters like :id
		if len(route.Parameters) > 0 {
			c.logDebug("Skipping parameterized route: %s (params: %v)", route.Path, route.Parameters)
			continue
		}

		fullURL := c.resolveFrameworkURL(baseURL, route.Path)
		if fullURL == "" || !c.scope.IsInScope(fullURL, parent.Depth+1) {
			continue
		}

		c.log("[ROUTE] Found: %s (type: %s)", fullURL, route.Meta["type"])

		items = append(items, &queue.QueueItem{
			URL:       fullURL,
			Method:    "GET",
			Depth:     parent.Depth + 1,
			ParentURL: parent.URL,
			Timestamp: time.Now(),
		})
	}

	// Process discovered links
	for _, link := range links {
		// Skip special links
		if strings.HasPrefix(link.URL, "$") {
			continue
		}

		fullURL := c.resolveFrameworkURL(baseURL, link.URL)
		if fullURL == "" || !c.scope.IsInScope(fullURL, parent.Depth+1) {
			continue
		}

		items = append(items, &queue.QueueItem{
			URL:       fullURL,
			Method:    "GET",
			Depth:     parent.Depth + 1,
			ParentURL: parent.URL,
			Timestamp: time.Now(),
		})
	}

	// Batch push for efficiency
	if len(items) > 0 {
		c.queue.PushBatch(items)
	}
}

// resolveFrameworkURL resolves a framework route/link to a full URL.
func (c *Crawler) resolveFrameworkURL(baseURL, path string) string {
	if path == "" {
		return ""
	}

	// Parse base URL
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}

	// Handle hash routes
	if strings.HasPrefix(path, "#") {
		return base.Scheme + "://" + base.Host + "/" + path
	}

	// Handle absolute paths
	if strings.HasPrefix(path, "/") {
		return base.Scheme + "://" + base.Host + path
	}

	// Handle relative paths
	return base.Scheme + "://" + base.Host + "/" + path
}

// processHTML processes HTML content.
func (c *Crawler) processHTML(html string, parent *queue.QueueItem) {
	parseResult, err := c.htmlParser.Parse(html)
	if err != nil {
		return
	}

	// Batch queue discovered links for better performance
	items := make([]*queue.QueueItem, 0, len(parseResult.Links))
	for _, link := range parseResult.Links {
		if !c.scope.IsInScope(link.URL, parent.Depth+1) {
			continue
		}

		if !scope.IsValidURL(link.URL) {
			continue
		}

		items = append(items, &queue.QueueItem{
			URL:       link.URL,
			Method:    "GET",
			Depth:     parent.Depth + 1,
			ParentURL: parent.URL,
			Timestamp: time.Now(),
		})
	}

	// Batch push for efficiency
	if len(items) > 0 {
		c.queue.PushBatch(items)
	}

	// Process scripts for JS analysis
	if c.config.JSAnalysis {
		for _, scriptURL := range parseResult.Scripts {
			c.processScript(scriptURL, parent.URL)
		}
	}
}

// processScript analyzes a JavaScript file.
func (c *Crawler) processScript(scriptURL, sourceURL string) {
	// Fetch script content
	result, err := c.browserPool.Visit(c.ctx, scriptURL, nil, nil)
	if err != nil || result.Error != nil {
		return
	}

	// Analyze JavaScript
	jsResult := c.jsParser.Parse(result.HTML)

	// Add discovered API endpoints
	for _, ep := range jsResult.APIEndpoints {
		resolvedURL, err := scope.ResolveURL(sourceURL, ep.URL)
		if err != nil {
			continue
		}

		endpoint := Endpoint{
			URL:            resolvedURL,
			Method:         ep.Method,
			Source:         "javascript",
			DiscoveredFrom: sourceURL,
			Timestamp:      time.Now(),
		}

		for _, param := range ep.Parameters {
			endpoint.Parameters = append(endpoint.Parameters, Parameter{
				Name: param,
				Type: "path",
			})
		}

		c.addEndpoint(endpoint)
	}

	// Add discovered WebSocket URLs
	for _, wsURL := range jsResult.WebSockets {
		resolvedURL, err := scope.ResolveURL(sourceURL, wsURL)
		if err != nil {
			continue
		}
		go c.wsHandler.Connect(c.ctx, resolvedURL, sourceURL)
	}
}

// processForms processes discovered forms.
func (c *Crawler) processForms(forms []browser.FormData, parent *queue.QueueItem) {
	for _, form := range forms {
		// Convert to parser format
		formInfo := parser.FormInfo{
			Action:  form.Action,
			Method:  form.Method,
			Enctype: form.Enctype,
			ID:      form.ID,
			Name:    form.Name,
		}

		for _, input := range form.Inputs {
			formInfo.Inputs = append(formInfo.Inputs, parser.InputInfo{
				Name:        input.Name,
				Type:        input.Type,
				Value:       input.Value,
				Required:    input.Required,
				Placeholder: input.Placeholder,
			})
		}

		// Analyze form
		result := c.formAnalyzer.Analyze(formInfo, parent.URL)
		c.addForm(convertParserForm(result.Form))
	}
}

// runActiveDiscovery performs active API discovery.
func (c *Crawler) runActiveDiscovery() {
	endpoints := c.activeDiscovery.Probe(c.ctx, c.config.Target)
	c.addDiscoveryEndpoints(endpoints)

	// Probe for GraphQL
	if ep := c.activeDiscovery.ProbeGraphQL(c.ctx, c.config.Target); ep != nil {
		c.addEndpoint(convertDiscoveryEndpoint(*ep))
	}
}

// convertDiscoveryEndpoint converts discovery.Endpoint to crawler.Endpoint
func convertDiscoveryEndpoint(ep discovery.Endpoint) Endpoint {
	params := make([]Parameter, 0, len(ep.Parameters))
	for _, p := range ep.Parameters {
		params = append(params, Parameter{
			Name:    p.Name,
			Type:    p.Type,
			Example: p.Example,
		})
	}
	return Endpoint{
		URL:            ep.URL,
		Method:         ep.Method,
		Source:         ep.Source,
		Parameters:     params,
		Headers:        ep.Headers,
		DiscoveredFrom: ep.DiscoveredFrom,
		StatusCode:     ep.StatusCode,
		ContentType:    ep.ContentType,
		Timestamp:      ep.Timestamp,
	}
}

// addDiscoveryEndpoints adds discovery.Endpoint slice to results.
func (c *Crawler) addDiscoveryEndpoints(eps []discovery.Endpoint) {
	for _, ep := range eps {
		c.addEndpoint(convertDiscoveryEndpoint(ep))
	}
}

// convertStateCrawlStats converts state.CrawlStats to crawler.CrawlStats
func convertStateCrawlStats(s state.CrawlStats) CrawlStats {
	return CrawlStats{
		URLsDiscovered:     s.URLsDiscovered,
		PagesCrawled:       s.PagesCrawled,
		FormsFound:         s.FormsFound,
		APIEndpoints:       s.APIEndpoints,
		WebSocketEndpoints: s.WebSocketEndpoints,
		ErrorCount:         s.ErrorCount,
		Duration:           s.Duration,
		BytesTransferred:   s.BytesTransferred,
	}
}

// convertWebSocketEndpoints converts websocket.WebSocketEndpoint slice to crawler.WebSocketEndpoint slice
func convertWebSocketEndpoints(wss []websocket.WebSocketEndpoint) []WebSocketEndpoint {
	result := make([]WebSocketEndpoint, 0, len(wss))
	for _, ws := range wss {
		msgs := make([]WebSocketMsg, 0, len(ws.SampleMessages))
		for _, m := range ws.SampleMessages {
			msgs = append(msgs, WebSocketMsg{
				Direction: m.Direction,
				Type:      m.Type,
				Data:      m.Data,
				Timestamp: m.Timestamp,
			})
		}
		result = append(result, WebSocketEndpoint{
			URL:            ws.URL,
			DiscoveredFrom: ws.DiscoveredFrom,
			SampleMessages: msgs,
			Protocols:      ws.Protocols,
			Timestamp:      ws.Timestamp,
		})
	}
	return result
}

// convertParserForm converts parser.Form to crawler.Form
func convertParserForm(f parser.Form) Form {
	inputs := make([]FormInput, 0, len(f.Inputs))
	for _, inp := range f.Inputs {
		inputs = append(inputs, FormInput{
			Name:        inp.Name,
			Type:        inp.Type,
			Value:       inp.Value,
			Required:    inp.Required,
			Placeholder: inp.Placeholder,
			Pattern:     inp.Pattern,
			MaxLength:   inp.MaxLength,
			MinLength:   inp.MinLength,
		})
	}
	return Form{
		URL:       f.URL,
		Action:    f.Action,
		Method:    f.Method,
		Enctype:   f.Enctype,
		Inputs:    inputs,
		HasCSRF:   f.HasCSRF,
		Depth:     f.Depth,
		Timestamp: f.Timestamp,
	}
}

// convertToOutputResult converts internal CrawlResult to output.CrawlResult
func (c *Crawler) convertToOutputResult(r *CrawlResult) *output.CrawlResult {
	result := &output.CrawlResult{
		Target:      r.Target,
		StartedAt:   r.StartedAt,
		CompletedAt: r.CompletedAt,
		Stats: output.CrawlStats{
			URLsDiscovered:     r.Stats.URLsDiscovered,
			PagesCrawled:       r.Stats.PagesCrawled,
			FormsFound:         r.Stats.FormsFound,
			APIEndpoints:       r.Stats.APIEndpoints,
			WebSocketEndpoints: r.Stats.WebSocketEndpoints,
			ErrorCount:         r.Stats.ErrorCount,
			Duration:           r.Stats.Duration,
			BytesTransferred:   r.Stats.BytesTransferred,
		},
		Endpoints:  make([]output.Endpoint, 0, len(r.Endpoints)),
		Forms:      make([]output.Form, 0, len(r.Forms)),
		WebSockets: make([]output.WebSocketEndpoint, 0, len(r.WebSockets)),
		Errors:     make([]output.CrawlError, 0, len(r.Errors)),
	}

	for _, ep := range r.Endpoints {
		params := make([]output.Parameter, 0, len(ep.Parameters))
		for _, p := range ep.Parameters {
			params = append(params, output.Parameter{
				Name:     p.Name,
				Type:     p.Type,
				Example:  p.Example,
				Required: p.Required,
			})
		}
		result.Endpoints = append(result.Endpoints, output.Endpoint{
			URL:            ep.URL,
			Method:         ep.Method,
			Source:         ep.Source,
			Depth:          ep.Depth,
			Parameters:     params,
			Headers:        ep.Headers,
			DiscoveredFrom: ep.DiscoveredFrom,
			StatusCode:     ep.StatusCode,
			ContentType:    ep.ContentType,
			ResponseSize:   ep.ResponseSize,
			Timestamp:      ep.Timestamp,
		})
	}

	for _, f := range r.Forms {
		inputs := make([]output.FormInput, 0, len(f.Inputs))
		for _, inp := range f.Inputs {
			inputs = append(inputs, output.FormInput{
				Name:        inp.Name,
				Type:        inp.Type,
				Value:       inp.Value,
				Required:    inp.Required,
				Placeholder: inp.Placeholder,
				Pattern:     inp.Pattern,
				MaxLength:   inp.MaxLength,
				MinLength:   inp.MinLength,
			})
		}
		result.Forms = append(result.Forms, output.Form{
			URL:       f.URL,
			Action:    f.Action,
			Method:    f.Method,
			Enctype:   f.Enctype,
			Inputs:    inputs,
			HasCSRF:   f.HasCSRF,
			Depth:     f.Depth,
			Timestamp: f.Timestamp,
		})
	}

	for _, ws := range r.WebSockets {
		msgs := make([]output.WebSocketMsg, 0, len(ws.SampleMessages))
		for _, m := range ws.SampleMessages {
			msgs = append(msgs, output.WebSocketMsg{
				Direction: m.Direction,
				Type:      m.Type,
				Data:      m.Data,
				Timestamp: m.Timestamp,
			})
		}
		result.WebSockets = append(result.WebSockets, output.WebSocketEndpoint{
			URL:            ws.URL,
			DiscoveredFrom: ws.DiscoveredFrom,
			SampleMessages: msgs,
			Protocols:      ws.Protocols,
			Timestamp:      ws.Timestamp,
		})
	}

	for _, e := range r.Errors {
		result.Errors = append(result.Errors, output.CrawlError{
			URL:       e.URL,
			Error:     e.Error,
			Timestamp: e.Timestamp,
		})
	}

	return result
}

// addEndpoint adds an endpoint to results.
func (c *Crawler) addEndpoint(ep Endpoint) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results.Endpoints = append(c.results.Endpoints, ep)
	c.state.AddAPIEndpoint()
	c.metrics.RecordAPIEndpoint()
	c.logger.DiscoveryEvent("endpoint", ep.URL, ep.Source)
}

// addEndpoints adds multiple endpoints.
func (c *Crawler) addEndpoints(eps []Endpoint) {
	for _, ep := range eps {
		c.addEndpoint(ep)
	}
}

// addForm adds a form to results.
func (c *Crawler) addForm(form Form) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results.Forms = append(c.results.Forms, form)
	c.state.AddForm()
	c.metrics.RecordFormFound()
	c.logger.DiscoveryEvent("form", form.URL, fmt.Sprintf("%s->%s (%d inputs)", form.Method, form.Action, len(form.Inputs)))
}

// addError adds an error to results.
func (c *Crawler) addError(url string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results.Errors = append(c.results.Errors, CrawlError{
		URL:       url,
		Error:     err.Error(),
		Timestamp: time.Now(),
	})
	c.state.AddError()

	// Record error in metrics with error type
	errType := "unknown"
	if err != nil {
		errType = fmt.Sprintf("%T", err)
	}
	c.metrics.RecordError(errType)
}

// collectEndpoints collects all discovered endpoints.
func (c *Crawler) collectEndpoints() []Endpoint {
	endpoints := make([]Endpoint, 0)
	endpoints = append(endpoints, c.results.Endpoints...)

	// Convert passive discovery endpoints
	for _, ep := range c.passiveDiscovery.GetEndpoints() {
		endpoints = append(endpoints, convertDiscoveryEndpoint(ep))
	}

	// Convert active discovery endpoints
	for _, ep := range c.activeDiscovery.GetEndpoints() {
		endpoints = append(endpoints, convertDiscoveryEndpoint(ep))
	}

	return endpoints
}

// collectForms returns all discovered forms.
func (c *Crawler) collectForms() []Form {
	return c.results.Forms
}

// registerShutdownCallbacks registers cleanup callbacks with the shutdown handler.
func (c *Crawler) registerShutdownCallbacks() {
	// Register browser pool cleanup (first to close as it's resource-intensive)
	c.shutdownHandler.Register("browser-pool", func(ctx context.Context) error {
		if c.browserPool != nil {
			c.logger.Debug("Closing browser pool...")
			c.browserPool.Close()
		}
		return nil
	})

	// Register HTTP client cleanup
	c.shutdownHandler.Register("http-client", func(ctx context.Context) error {
		if c.fastClient != nil {
			c.logger.Debug("Closing HTTP client...")
			c.fastClient.Close()
		}
		return nil
	})

	// Register queue cleanup
	c.shutdownHandler.Register("queue", func(ctx context.Context) error {
		if c.queue != nil {
			c.logger.Debug("Closing queue...")
			c.queue.Close()
		}
		return nil
	})

	// Register output writer cleanup
	c.shutdownHandler.Register("output", func(ctx context.Context) error {
		if c.output != nil {
			c.logger.Debug("Closing output writer...")
			c.output.Close()
		}
		return nil
	})

	// Register cancel context callback
	c.shutdownHandler.Register("context-cancel", func(ctx context.Context) error {
		if c.cancel != nil {
			c.logger.Debug("Cancelling crawler context...")
			c.cancel()
		}
		return nil
	})
}

// cleanup releases resources.
func (c *Crawler) cleanup() {
	// Trigger shutdown handler for graceful cleanup
	if c.shutdownHandler != nil && c.shutdownHandler.IsShuttingDown() {
		// Wait for shutdown to complete
		<-c.shutdownHandler.Done()
		return
	}

	// Fallback direct cleanup if shutdown handler wasn't used
	if c.browserPool != nil {
		c.browserPool.Close()
	}
	if c.fastClient != nil {
		c.fastClient.Close()
	}
	if c.output != nil {
		c.output.Close()
	}
	if c.queue != nil {
		c.queue.Close()
	}
}

// Stop stops the crawler gracefully.
func (c *Crawler) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Trigger graceful shutdown
	if c.shutdownHandler != nil {
		c.shutdownHandler.Shutdown()
	} else if c.cancel != nil {
		c.cancel()
	}

	return nil
}

// StopNow stops the crawler immediately without waiting for cleanup.
func (c *Crawler) StopNow() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Trigger immediate shutdown
	if c.shutdownHandler != nil {
		c.shutdownHandler.ShutdownNow()
	} else if c.cancel != nil {
		c.cancel()
	}

	return nil
}

// ShutdownContext returns the shutdown context for monitoring.
func (c *Crawler) ShutdownContext() context.Context {
	if c.shutdownHandler != nil {
		return c.shutdownHandler.Context()
	}
	return c.ctx
}

// SaveState saves the current crawl state.
func (c *Crawler) SaveState(path string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	crawlerState := &state.CrawlerState{
		Target:      c.config.Target,
		StartedAt:   c.startTime,
		UpdatedAt:   time.Now(),
		Stats:       c.state.GetStats(),
		QueueURLs:   c.queue.URLs(),
		VisitedURLs: c.state.GetDeduplicator().GetAll(),
		Endpoints:   convertEndpointsToState(c.results.Endpoints),
		Forms:       convertFormsToState(c.results.Forms),
		WebSockets:  convertWebSocketsToState(c.results.WebSockets),
		Errors:      convertErrorsToState(c.results.Errors),
	}

	configData, _ := json.Marshal(c.config)
	crawlerState.Config = configData

	return c.state.Save(crawlerState)
}

// LoadState loads a saved crawl state.
func (c *Crawler) LoadState(path string) error {
	store, err := state.NewBoltStore(path)
	if err != nil {
		return err
	}

	savedState, err := store.Load()
	if err != nil {
		return err
	}

	if savedState == nil {
		return fmt.Errorf("no saved state found")
	}

	// Restore queue
	for _, url := range savedState.QueueURLs {
		c.queue.Push(&queue.QueueItem{
			URL:       url,
			Method:    "GET",
			Timestamp: time.Now(),
		})
	}

	// Restore visited URLs
	c.state.GetDeduplicator().AddBatch(savedState.VisitedURLs)

	// Restore results
	c.results = &CrawlResult{
		Target:     savedState.Target,
		StartedAt:  savedState.StartedAt,
		Stats:      convertStateCrawlStats(savedState.Stats),
		Endpoints:  convertEndpointsFromState(savedState.Endpoints),
		Forms:      convertFormsFromState(savedState.Forms),
		WebSockets: convertWebSocketsFromState(savedState.WebSockets),
		Errors:     convertErrorsFromState(savedState.Errors),
	}

	return nil
}

// Results returns a channel for streaming results.
func (c *Crawler) Results() <-chan interface{} {
	return c.resultsChan
}

// Stats returns current crawl statistics.
func (c *Crawler) Stats() CrawlStats {
	return convertStateCrawlStats(c.state.GetStats())
}

// Metrics returns the metrics collector for external access.
func (c *Crawler) Metrics() *metrics.Collector {
	return c.metrics
}

// MetricsSnapshot returns a point-in-time snapshot of all metrics.
func (c *Crawler) MetricsSnapshot() *metrics.Snapshot {
	if c.metrics == nil {
		return nil
	}
	return c.metrics.Snapshot()
}

// convertEndpointsToState converts crawler.Endpoint slice to state.Endpoint slice
func convertEndpointsToState(eps []Endpoint) []state.Endpoint {
	result := make([]state.Endpoint, 0, len(eps))
	for _, ep := range eps {
		params := make([]state.Parameter, 0, len(ep.Parameters))
		for _, p := range ep.Parameters {
			params = append(params, state.Parameter{
				Name:     p.Name,
				Type:     p.Type,
				Example:  p.Example,
				Required: p.Required,
			})
		}
		result = append(result, state.Endpoint{
			URL:            ep.URL,
			Method:         ep.Method,
			Source:         ep.Source,
			Depth:          ep.Depth,
			Parameters:     params,
			Headers:        ep.Headers,
			DiscoveredFrom: ep.DiscoveredFrom,
			StatusCode:     ep.StatusCode,
			ContentType:    ep.ContentType,
			ResponseSize:   ep.ResponseSize,
			Timestamp:      ep.Timestamp,
		})
	}
	return result
}

// convertEndpointsFromState converts state.Endpoint slice to crawler.Endpoint slice
func convertEndpointsFromState(eps []state.Endpoint) []Endpoint {
	result := make([]Endpoint, 0, len(eps))
	for _, ep := range eps {
		params := make([]Parameter, 0, len(ep.Parameters))
		for _, p := range ep.Parameters {
			params = append(params, Parameter{
				Name:     p.Name,
				Type:     p.Type,
				Example:  p.Example,
				Required: p.Required,
			})
		}
		result = append(result, Endpoint{
			URL:            ep.URL,
			Method:         ep.Method,
			Source:         ep.Source,
			Depth:          ep.Depth,
			Parameters:     params,
			Headers:        ep.Headers,
			DiscoveredFrom: ep.DiscoveredFrom,
			StatusCode:     ep.StatusCode,
			ContentType:    ep.ContentType,
			ResponseSize:   ep.ResponseSize,
			Timestamp:      ep.Timestamp,
		})
	}
	return result
}

// convertFormsToState converts crawler.Form slice to state.Form slice
func convertFormsToState(forms []Form) []state.Form {
	result := make([]state.Form, 0, len(forms))
	for _, f := range forms {
		inputs := make([]state.FormInput, 0, len(f.Inputs))
		for _, inp := range f.Inputs {
			inputs = append(inputs, state.FormInput{
				Name:        inp.Name,
				Type:        inp.Type,
				Value:       inp.Value,
				Required:    inp.Required,
				Placeholder: inp.Placeholder,
				Pattern:     inp.Pattern,
				MaxLength:   inp.MaxLength,
				MinLength:   inp.MinLength,
			})
		}
		result = append(result, state.Form{
			URL:       f.URL,
			Action:    f.Action,
			Method:    f.Method,
			Enctype:   f.Enctype,
			Inputs:    inputs,
			HasCSRF:   f.HasCSRF,
			Depth:     f.Depth,
			Timestamp: f.Timestamp,
		})
	}
	return result
}

// convertFormsFromState converts state.Form slice to crawler.Form slice
func convertFormsFromState(forms []state.Form) []Form {
	result := make([]Form, 0, len(forms))
	for _, f := range forms {
		inputs := make([]FormInput, 0, len(f.Inputs))
		for _, inp := range f.Inputs {
			inputs = append(inputs, FormInput{
				Name:        inp.Name,
				Type:        inp.Type,
				Value:       inp.Value,
				Required:    inp.Required,
				Placeholder: inp.Placeholder,
				Pattern:     inp.Pattern,
				MaxLength:   inp.MaxLength,
				MinLength:   inp.MinLength,
			})
		}
		result = append(result, Form{
			URL:       f.URL,
			Action:    f.Action,
			Method:    f.Method,
			Enctype:   f.Enctype,
			Inputs:    inputs,
			HasCSRF:   f.HasCSRF,
			Depth:     f.Depth,
			Timestamp: f.Timestamp,
		})
	}
	return result
}

// convertWebSocketsToState converts crawler.WebSocketEndpoint slice to state.WebSocketEndpoint slice
func convertWebSocketsToState(wss []WebSocketEndpoint) []state.WebSocketEndpoint {
	result := make([]state.WebSocketEndpoint, 0, len(wss))
	for _, ws := range wss {
		msgs := make([]state.WebSocketMsg, 0, len(ws.SampleMessages))
		for _, m := range ws.SampleMessages {
			msgs = append(msgs, state.WebSocketMsg{
				Direction: m.Direction,
				Type:      m.Type,
				Data:      m.Data,
				Timestamp: m.Timestamp,
			})
		}
		result = append(result, state.WebSocketEndpoint{
			URL:            ws.URL,
			DiscoveredFrom: ws.DiscoveredFrom,
			SampleMessages: msgs,
			Protocols:      ws.Protocols,
			Timestamp:      ws.Timestamp,
		})
	}
	return result
}

// convertWebSocketsFromState converts state.WebSocketEndpoint slice to crawler.WebSocketEndpoint slice
func convertWebSocketsFromState(wss []state.WebSocketEndpoint) []WebSocketEndpoint {
	result := make([]WebSocketEndpoint, 0, len(wss))
	for _, ws := range wss {
		msgs := make([]WebSocketMsg, 0, len(ws.SampleMessages))
		for _, m := range ws.SampleMessages {
			msgs = append(msgs, WebSocketMsg{
				Direction: m.Direction,
				Type:      m.Type,
				Data:      m.Data,
				Timestamp: m.Timestamp,
			})
		}
		result = append(result, WebSocketEndpoint{
			URL:            ws.URL,
			DiscoveredFrom: ws.DiscoveredFrom,
			SampleMessages: msgs,
			Protocols:      ws.Protocols,
			Timestamp:      ws.Timestamp,
		})
	}
	return result
}

// convertErrorsToState converts crawler.CrawlError slice to state.CrawlError slice
func convertErrorsToState(errs []CrawlError) []state.CrawlError {
	result := make([]state.CrawlError, 0, len(errs))
	for _, e := range errs {
		result = append(result, state.CrawlError{
			URL:       e.URL,
			Error:     e.Error,
			Timestamp: e.Timestamp,
		})
	}
	return result
}

// convertErrorsFromState converts state.CrawlError slice to crawler.CrawlError slice
func convertErrorsFromState(errs []state.CrawlError) []CrawlError {
	result := make([]CrawlError, 0, len(errs))
	for _, e := range errs {
		result = append(result, CrawlError{
			URL:       e.URL,
			Error:     e.Error,
			Timestamp: e.Timestamp,
		})
	}
	return result
}

// IsRunning returns true if the crawler is running.
func (c *Crawler) IsRunning() bool {
	return c.running.Load()
}

func getDomain(urlStr string) string {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	return parsed.Host
}

// isHashRoute checks if a URL contains a hash-based SPA route.
func isHashRoute(urlStr string) bool {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	// Check for hash routes like #/ or #!/
	fragment := parsed.Fragment
	if fragment == "" {
		return false
	}
	return len(fragment) > 0 && (fragment[0] == '/' || fragment[0] == '!')
}

// needsJavaScriptRendering checks if a page likely needs JS rendering.
func (c *Crawler) needsJavaScriptRendering(html, contentType string) bool {
	// Not HTML - no JS rendering needed
	if !strings.Contains(contentType, "text/html") {
		return false
	}

	// Empty or very small body often means JS-rendered content
	if len(html) < 500 {
		return true
	}

	htmlLower := strings.ToLower(html)

	// Check for SPA framework indicators
	spaIndicators := []string{
		"ng-app", "ng-view", "ui-view",           // AngularJS
		"<app-root", "[ng-version]",              // Angular
		"data-reactroot", "__next",               // React/Next.js
		"data-v-", "data-server-rendered",        // Vue
		"data-ember",                             // Ember
		"window.__nuxt", "window.__svelte",       // Nuxt/Svelte
	}

	for _, indicator := range spaIndicators {
		if strings.Contains(htmlLower, indicator) {
			return true
		}
	}

	// Check for pages with minimal body content but lots of scripts
	bodyStart := strings.Index(htmlLower, "<body")
	bodyEnd := strings.Index(htmlLower, "</body>")
	if bodyStart > 0 && bodyEnd > bodyStart {
		bodyContent := html[bodyStart:bodyEnd]
		// Remove script tags
		scriptLess := bodyContent
		for {
			start := strings.Index(strings.ToLower(scriptLess), "<script")
			if start == -1 {
				break
			}
			end := strings.Index(strings.ToLower(scriptLess[start:]), "</script>")
			if end == -1 {
				break
			}
			scriptLess = scriptLess[:start] + scriptLess[start+end+9:]
		}
		// If body without scripts is very small, likely JS-rendered
		if len(strings.TrimSpace(scriptLess)) < 200 {
			return true
		}
	}

	return false
}

// splitHashURL splits a URL with hash into base URL and hash route.
func splitHashURL(urlStr string) (baseURL, hashRoute string) {
	idx := strings.Index(urlStr, "#")
	if idx == -1 {
		return urlStr, ""
	}
	return urlStr[:idx], urlStr[idx:]
}

// statusReporter prints crawl statistics periodically.
func (c *Crawler) statusReporter() {
	ticker := time.NewTicker(1 * time.Second) // Update more frequently for progress bar
	defer ticker.Stop()

	lastCrawled := 0
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			stats := c.state.GetStats()
			queueSize := c.queue.Len()
			crawlRate := stats.PagesCrawled - lastCrawled
			lastCrawled = stats.PagesCrawled
			poolStats := c.browserPool.Stats()
			httpCount := c.httpPages.Load()
			browserCount := c.browserPages.Load()
			ratePerSec := float64(crawlRate)

			// Update metrics collector with current state
			c.metrics.SetQueueDepth(int64(queueSize))
			c.metrics.SetActiveWorkers(int64(c.config.Workers))
			c.metrics.SetBrowserPoolStats(int64(poolStats.Size), int64(poolStats.Size-poolStats.Available))

			// Update progress bar if enabled
			if c.showProgress && c.progress != nil {
				c.progress.Update(
					stats.URLsDiscovered,
					stats.PagesCrawled,
					stats.FormsFound,
					stats.APIEndpoints,
					stats.WebSocketEndpoints,
					stats.ErrorCount,
					queueSize,
				)
			}

			// Log stats using structured logger (only if not showing progress bar)
			if !c.showProgress {
				c.logger.StatsEvent(map[string]interface{}{
					"pages_crawled":     stats.PagesCrawled,
					"rate_per_sec":      ratePerSec,
					"queue_depth":       queueSize,
					"http_pages":        httpCount,
					"browser_pages":     browserCount,
					"api_endpoints":     stats.APIEndpoints,
					"forms_found":       stats.FormsFound,
					"browser_pool_used": poolStats.Size - poolStats.Available,
					"browser_pool_size": poolStats.Size,
					"errors":            stats.ErrorCount,
					"requests_per_sec":  c.metrics.GetRequestsPerSecond(),
					"avg_response_ms":   c.metrics.GetAverageResponseTime().Milliseconds(),
				})
			}
		}
	}
}

// processAJAXEndpoints processes discovered AJAX endpoints.
func (c *Crawler) processAJAXEndpoints(ajaxEndpoints []browser.AJAXEndpoint, parent *queue.QueueItem) {
	for _, ep := range ajaxEndpoints {
		// Resolve URL
		fullURL := c.resolveFrameworkURL(parent.URL, ep.URL)
		if fullURL == "" {
			continue
		}

		c.log("[AJAX] Endpoint: [%s] %s (source: %s)", ep.Method, fullURL, ep.Source)

		// Add to discovered endpoints
		endpoint := Endpoint{
			URL:            fullURL,
			Method:         ep.Method,
			Source:         "ajax-" + ep.Source,
			DiscoveredFrom: parent.URL,
			Timestamp:      time.Now(),
		}

		for _, param := range ep.Parameters {
			endpoint.Parameters = append(endpoint.Parameters, Parameter{
				Name: param,
				Type: "query",
			})
		}

		c.addEndpoint(endpoint)

		// Queue GET endpoints for crawling
		if ep.Method == "GET" && c.scope.IsInScope(fullURL, parent.Depth+1) && !c.state.HasVisited(fullURL) {
			queueItem := &queue.QueueItem{
				URL:       fullURL,
				Method:    "GET",
				Depth:     parent.Depth + 1,
				ParentURL: parent.URL,
				Timestamp: time.Now(),
			}
			c.queue.Push(queueItem)
		}
	}
}

// processAJAXForms processes forms that submit via AJAX.
func (c *Crawler) processAJAXForms(ajaxForms []browser.AJAXForm, parent *queue.QueueItem) {
	for _, af := range ajaxForms {
		// Resolve action URL
		actionURL := c.resolveFrameworkURL(parent.URL, af.Action)
		if actionURL == "" {
			actionURL = parent.URL
		}

		c.log("[AJAX-FORM] %s -> %s (type: %s)", af.FormID, actionURL, af.SubmitType)

		// Convert to Form type
		form := Form{
			URL:       parent.URL,
			Action:    actionURL,
			Method:    af.Method,
			Enctype:   "application/x-www-form-urlencoded",
			Timestamp: time.Now(),
		}

		for _, input := range af.Inputs {
			form.Inputs = append(form.Inputs, FormInput{
				Name:     input.Name,
				Type:     input.Type,
				Value:    input.Value,
				Required: input.Required,
			})

			// Check for CSRF token
			if strings.Contains(strings.ToLower(input.Name), "csrf") ||
				strings.Contains(strings.ToLower(input.Name), "token") ||
				strings.Contains(strings.ToLower(input.Name), "_token") {
				form.HasCSRF = true
			}
		}

		c.addForm(form)

		// Also add the callback URL as an endpoint if present
		if af.CallbackURL != "" {
			callbackFullURL := c.resolveFrameworkURL(parent.URL, af.CallbackURL)
			if callbackFullURL != "" {
				c.addEndpoint(Endpoint{
					URL:            callbackFullURL,
					Method:         af.Method,
					Source:         "ajax-form-callback",
					DiscoveredFrom: parent.URL,
					Timestamp:      time.Now(),
				})
			}
		}
	}
}

// runEnhancedDiscovery performs enhanced discovery on the target.
func (c *Crawler) runEnhancedDiscovery() {
	c.log("[ENHANCED] Starting enhanced discovery on %s", c.config.Target)

	// First, do a quick discovery (robots.txt, sitemap.xml)
	quickResult := c.enhancedDiscovery.DiscoverQuick(c.config.Target)

	// Log and queue discovered URLs
	if quickResult.RobotsResult != nil {
		c.log("[ENHANCED] Found robots.txt with %d disallowed paths, %d sitemaps",
			len(quickResult.RobotsResult.DisallowedPaths),
			len(quickResult.RobotsResult.Sitemaps))
	}

	if len(quickResult.SitemapURLs) > 0 {
		c.log("[ENHANCED] Found %d URLs from sitemaps", len(quickResult.SitemapURLs))
	}

	// Queue discovered URLs
	for _, discoveredURL := range quickResult.AllURLs {
		if c.scope.IsInScope(discoveredURL, 1) {
			item := &queue.QueueItem{
				URL:       discoveredURL,
				Method:    "GET",
				Depth:     1,
				ParentURL: c.config.Target,
				Timestamp: time.Now(),
			}
			c.queue.Push(item)
		}
	}

	// Add API endpoints
	for _, apiEndpoint := range quickResult.AllAPIEndpoints {
		c.addEndpoint(Endpoint{
			URL:            apiEndpoint,
			Method:         "GET",
			Source:         "enhanced-discovery",
			DiscoveredFrom: c.config.Target,
			Timestamp:      time.Now(),
		})
	}

	c.log("[ENHANCED] Quick discovery complete: %d URLs queued, %d API endpoints found",
		len(quickResult.AllURLs), len(quickResult.AllAPIEndpoints))
}

// runEnhancedDiscoveryOnPage performs enhanced discovery on a specific page result.
func (c *Crawler) runEnhancedDiscoveryOnPage(result *browser.PageResult, parentURL string) {
	if c.enhancedDiscovery == nil {
		return
	}

	// Collect JS URLs from the page
	jsURLs := make([]string, 0)
	for _, script := range result.Scripts {
		if strings.HasPrefix(script, "http") {
			jsURLs = append(jsURLs, script)
		} else if strings.HasPrefix(script, "/") {
			base, _ := url.Parse(parentURL)
			if base != nil {
				jsURLs = append(jsURLs, base.Scheme+"://"+base.Host+script)
			}
		}
	}

	// Collect known URLs for parameter discovery
	knownURLs := make([]string, 0)
	for _, link := range result.Links {
		knownURLs = append(knownURLs, link)
	}

	// Use cookies directly (already []*http.Cookie)
	cookies := result.Cookies

	// Build headers (empty since PageResult doesn't have Headers)
	headers := make(http.Header)
	headers.Set("Content-Type", result.ContentType)

	// Run full discovery
	discoveryResult := c.enhancedDiscovery.Discover(
		parentURL,
		headers,
		result.HTML,
		cookies,
		jsURLs,
		knownURLs,
	)

	// Process technology fingerprinting
	if discoveryResult.TechResult != nil && len(discoveryResult.TechResult.Technologies) > 0 {
		c.logDebug("[ENHANCED] Detected %d technologies", len(discoveryResult.TechResult.Technologies))
		for _, tech := range discoveryResult.TechResult.Technologies {
			if tech.Confidence >= 80 {
				c.log("[TECH] %s (%s) - confidence: %d%%", tech.Name, tech.Category, tech.Confidence)
			}
		}
	}

	// Process secrets
	if len(discoveryResult.AllSecrets) > 0 {
		c.log("[ENHANCED] Found %d potential secrets!", len(discoveryResult.AllSecrets))
		for _, secret := range discoveryResult.AllSecrets {
			c.log("[SECRET] Type: %s, File: %s", secret.Type, secret.File)
		}
	}

	// Queue discovered URLs
	for _, discoveredURL := range discoveryResult.AllURLs {
		if c.scope.IsInScope(discoveredURL, 2) && !c.state.HasVisited(discoveredURL) {
			item := &queue.QueueItem{
				URL:       discoveredURL,
				Method:    "GET",
				Depth:     2,
				ParentURL: parentURL,
				Timestamp: time.Now(),
			}
			c.queue.Push(item)
		}
	}

	// Add discovered routes
	for _, route := range discoveryResult.AllRoutes {
		fullURL := c.resolveFrameworkURL(parentURL, route)
		if fullURL != "" && c.scope.IsInScope(fullURL, 2) && !c.state.HasVisited(fullURL) {
			item := &queue.QueueItem{
				URL:       fullURL,
				Method:    "GET",
				Depth:     2,
				ParentURL: parentURL,
				Timestamp: time.Now(),
			}
			c.queue.Push(item)
		}
	}

	// Add API endpoints
	for _, apiEndpoint := range discoveryResult.AllAPIEndpoints {
		fullURL := c.resolveFrameworkURL(parentURL, apiEndpoint)
		if fullURL != "" {
			c.addEndpoint(Endpoint{
				URL:            fullURL,
				Method:         "GET",
				Source:         "enhanced-js-discovery",
				DiscoveredFrom: parentURL,
				Timestamp:      time.Now(),
			})
		}
	}
}
