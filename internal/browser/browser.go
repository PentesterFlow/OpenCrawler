// Package browser provides headless Chrome integration via Rod.
package browser

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/ysmood/gson"

	"github.com/PentesterFlow/OpenCrawler/internal/framework"
)

// Config defines browser configuration.
type Config struct {
	PoolSize          int           `json:"pool_size"`
	Headless          bool          `json:"headless"`
	Timeout           time.Duration `json:"timeout"`
	UserAgent         string        `json:"user_agent"`
	ViewportWidth     int           `json:"viewport_width"`
	ViewportHeight    int           `json:"viewport_height"`
	RecycleAfter      int           `json:"recycle_after"`
	IgnoreHTTPSErrors bool          `json:"ignore_https_errors"`
	FastMode          bool          `json:"fast_mode"` // Skip heavy analysis for speed
}

// VisitOptions contains options for a single page visit.
type VisitOptions struct {
	FastMode       bool      // Skip SPA framework detection and AJAX analysis
	SPAMode        bool      // Enable advanced SPA handling (content wait, stealth, etc.)
	MaxWaitTime    time.Duration // Maximum wait time for SPA content
	EnableStealth  bool      // Enable anti-detection measures
	CheckSoftError bool      // Check for soft 404 errors
}

// DefaultConfig returns default browser configuration.
func DefaultConfig() Config {
	return Config{
		PoolSize:          20,
		Headless:          true,
		Timeout:           15 * time.Second,
		UserAgent:         "DAST-Crawler/1.0 (Security Scanner)",
		ViewportWidth:     1920,
		ViewportHeight:    1080,
		RecycleAfter:      50,
		IgnoreHTTPSErrors: true,
	}
}

// Browser wraps a Rod browser instance.
type Browser struct {
	browser   *rod.Browser
	config    Config
	mu        sync.Mutex
	pageCount int
}

// PageResult contains the result of a page visit.
type PageResult struct {
	URL              string
	FinalURL         string
	StatusCode       int
	ContentType      string
	HTML             string
	Title            string
	Links            []string
	Scripts          []string
	Forms            []FormData
	XHRRequests      []NetworkRequest
	WebSockets       []string
	Cookies          []*http.Cookie
	ResponseTime     time.Duration
	Error            error
	Framework        *framework.DetectionResult
	FrameworkRoutes  []framework.Route
	FrameworkLinks   []framework.Link
	// AJAX-specific results
	AJAXEndpoints  []AJAXEndpoint
	AJAXForms      []AJAXForm
	DynamicContent []string

	// SPA-specific results
	ContentHash    string   // Hash of page content for dedup
	IsSoftError    bool     // True if page shows error content with 200 status
	SoftErrorMsg   string   // Error message if soft error detected
	ShadowDOMLinks []string // Links extracted from Shadow DOM
	IsAuthPage     bool     // True if redirected to auth page
	AuthURL        string   // URL of auth page if redirected
}

// FormData represents form data extracted from a page.
type FormData struct {
	Action   string
	Method   string
	Enctype  string
	ID       string
	Name     string
	Inputs   []InputData
}

// InputData represents form input data.
type InputData struct {
	Name        string
	Type        string
	Value       string
	Required    bool
	Placeholder string
	Pattern     string
	MaxLength   int
	MinLength   int
}

// NetworkRequest represents an intercepted network request.
type NetworkRequest struct {
	URL          string
	Method       string
	Headers      map[string]string
	PostData     string
	ResourceType string
	Timestamp    time.Time
}

// New creates a new browser instance.
func New(config Config) (*Browser, error) {
	// Configure launcher
	l := launcher.New()

	if config.Headless {
		l = l.Headless(true)
	}

	if config.IgnoreHTTPSErrors {
		l = l.Set("ignore-certificate-errors", "true")
	}

	// Launch browser
	url, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	browser := rod.New().ControlURL(url)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to browser: %w", err)
	}

	// Set default timeout
	browser = browser.Timeout(config.Timeout)

	return &Browser{
		browser: browser,
		config:  config,
	}, nil
}

// Visit navigates to a URL and extracts page data.
func (b *Browser) Visit(ctx context.Context, url string, headers map[string]string, cookies []*http.Cookie) (*PageResult, error) {
	return b.VisitWithOptions(ctx, url, headers, cookies, VisitOptions{FastMode: b.config.FastMode})
}

// VisitWithOptions navigates to a URL with custom options.
func (b *Browser) VisitWithOptions(ctx context.Context, url string, headers map[string]string, cookies []*http.Cookie, opts VisitOptions) (*PageResult, error) {
	b.mu.Lock()
	b.pageCount++
	b.mu.Unlock()

	start := time.Now()
	result := &PageResult{
		URL:         url,
		XHRRequests: make([]NetworkRequest, 0),
	}

	// Create new page with context
	page, err := b.browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}
	defer page.Close()

	// Set timeout from context or default
	page = page.Context(ctx)

	// Initialize SPA handler for advanced SPA handling
	var spaHandler *SPAHandler
	if opts.SPAMode || !opts.FastMode {
		spaHandler = NewSPAHandler(DefaultSPAConfig())

		// Apply stealth mode if enabled
		if opts.EnableStealth || spaHandler.config.StealthMode {
			_ = spaHandler.ApplyStealthMode(page)
		}

		// Inject network monitor
		_ = spaHandler.InjectNetworkMonitor(page)
	}

	// Set viewport (ignore errors, not critical)
	_ = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:  b.config.ViewportWidth,
		Height: b.config.ViewportHeight,
	})

	// Set user agent
	if b.config.UserAgent != "" {
		_ = proto.NetworkSetUserAgentOverride{
			UserAgent: b.config.UserAgent,
		}.Call(page)
	}

	// Set extra headers
	if len(headers) > 0 {
		networkHeaders := make(proto.NetworkHeaders)
		for k, v := range headers {
			networkHeaders[k] = gson.New(v)
		}
		_ = proto.NetworkSetExtraHTTPHeaders{Headers: networkHeaders}.Call(page)
	}

	// Set cookies
	if len(cookies) > 0 {
		cookieParams := make([]*proto.NetworkCookieParam, 0, len(cookies))
		for _, cookie := range cookies {
			cookieParams = append(cookieParams, &proto.NetworkCookieParam{
				Name:     cookie.Name,
				Value:    cookie.Value,
				Domain:   cookie.Domain,
				Path:     cookie.Path,
				Secure:   cookie.Secure,
				HTTPOnly: cookie.HttpOnly,
			})
		}
		_ = page.SetCookies(cookieParams)
	}

	// Enable network interception
	router := page.HijackRequests()
	xhrRequests := make([]NetworkRequest, 0)
	var xhrMu sync.Mutex

	err = router.Add("*", "", func(hijack *rod.Hijack) {
		resourceType := hijack.Request.Type()

		// Capture XHR/Fetch requests
		if resourceType == proto.NetworkResourceTypeXHR || resourceType == proto.NetworkResourceTypeFetch {
			// Convert http.Header to map[string]string
			reqHeaders := make(map[string]string)
			for k, vals := range hijack.Request.Req().Header {
				if len(vals) > 0 {
					reqHeaders[k] = vals[0]
				}
			}
			xhrMu.Lock()
			xhrRequests = append(xhrRequests, NetworkRequest{
				URL:          hijack.Request.URL().String(),
				Method:       hijack.Request.Method(),
				Headers:      reqHeaders,
				PostData:     hijack.Request.Body(),
				ResourceType: string(resourceType),
				Timestamp:    time.Now(),
			})
			xhrMu.Unlock()
		}

		hijack.ContinueRequest(&proto.FetchContinueRequest{})
	})
	if err != nil {
		// Continue without interception if it fails
		router = nil
	}

	if router != nil {
		go router.Run()
		defer router.Stop()
	}

	// Navigate to URL
	err = page.Navigate(url)
	if err != nil {
		result.Error = err
		return result, nil
	}

	// Wait for page to load with timeout handling
	err = page.WaitLoad()
	if err != nil {
		result.Error = err
		return result, nil
	}

	// Detect and wait for SPA frameworks (skip in fast mode)
	var detector *framework.Detector
	if !opts.FastMode {
		detector = framework.NewDetector()
		result.Framework = detector.Detect(page)

		// Wait for detected frameworks to be ready
		if result.Framework.IsSPA {
			_ = detector.WaitForFrameworks(page)
		}

		// Use SPA handler for advanced content waiting
		if spaHandler != nil && result.Framework.IsSPA {
			_ = spaHandler.WaitForContent(page)
		} else {
			// Short wait for dynamic content
			time.Sleep(200 * time.Millisecond)
		}

		// Check for auth redirect
		if spaHandler != nil {
			isAuth, authURL := spaHandler.HandleAuthRedirect(page)
			result.IsAuthPage = isAuth
			result.AuthURL = authURL
		}
	} else {
		// Minimal wait in fast mode
		time.Sleep(100 * time.Millisecond)
		result.Framework = &framework.DetectionResult{}
	}

	// Get final URL after redirects (with error handling)
	info, err := page.Info()
	if err == nil && info != nil {
		result.FinalURL = info.URL
	} else {
		result.FinalURL = url
	}

	// Get HTML content
	html, err := page.HTML()
	if err == nil {
		result.HTML = html
	}

	// Get title
	titleEl, err := page.Element("title")
	if err == nil && titleEl != nil {
		title, err := titleEl.Text()
		if err == nil {
			result.Title = title
		}
	}

	// Extract links (traditional method)
	result.Links = b.extractLinks(page)

	// Extract scripts
	result.Scripts = b.extractScripts(page)

	// Extract forms
	result.Forms = b.extractForms(page)

	// Extract WebSocket URLs
	result.WebSockets = b.extractWebSockets(page)

	// Extract framework-specific routes and links (skip in fast mode)
	if !opts.FastMode && result.Framework.IsSPA && detector != nil {
		result.FrameworkRoutes = detector.ExtractAllRoutes(page)
		result.FrameworkLinks = detector.ExtractAllLinks(page)
	}

	// Perform AJAX analysis (skip in fast mode)
	if !opts.FastMode {
		ajaxHandler := NewAJAXHandler()

		// Inject interceptor and extract endpoints (fast operations)
		ajaxHandler.InjectAJAXInterceptor(page)
		result.AJAXEndpoints = ajaxHandler.ExtractAJAXEndpoints(page)
		result.AJAXForms = ajaxHandler.ExtractAJAXForms(page)

		// Get already captured requests (from the interceptor we injected)
		capturedReqs := ajaxHandler.GetCapturedRequests(page)
		for _, req := range capturedReqs {
			isDupe := false
			for _, existing := range result.XHRRequests {
				if existing.URL == req.URL && existing.Method == req.Method {
					isDupe = true
					break
				}
			}
			if !isDupe {
				result.XHRRequests = append(result.XHRRequests, req)
			}
		}
	}

	// SPA-specific processing
	if spaHandler != nil {
		// Check for soft 404 errors
		if opts.CheckSoftError || opts.SPAMode {
			isSoftError, softErrorMsg := spaHandler.IsSoftError(page)
			result.IsSoftError = isSoftError
			result.SoftErrorMsg = softErrorMsg
		}

		// Calculate content hash for deduplication
		if !opts.FastMode {
			contentHash, err := spaHandler.GetContentHash(page)
			if err == nil {
				result.ContentHash = contentHash
			}
		}

		// Extract Shadow DOM content
		shadowLinks, err := spaHandler.ExtractShadowDOMContent(page)
		if err == nil && len(shadowLinks) > 0 {
			result.ShadowDOMLinks = shadowLinks
		}
	}

	// Get cookies
	rodCookies, _ := page.Cookies(nil)
	for _, c := range rodCookies {
		result.Cookies = append(result.Cookies, &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Secure:   c.Secure,
			HttpOnly: c.HTTPOnly,
		})
	}

	// Copy XHR requests
	xhrMu.Lock()
	result.XHRRequests = xhrRequests
	xhrMu.Unlock()

	result.ResponseTime = time.Since(start)

	return result, nil
}

// extractLinks extracts all links from the page, including SPA routes.
func (b *Browser) extractLinks(page *rod.Page) []string {
	links := make([]string, 0)
	seen := make(map[string]bool)

	// Extract standard href links
	elements, err := page.Elements("a[href]")
	if err == nil {
		for _, el := range elements {
			href, err := el.Attribute("href")
			if err == nil && href != nil && *href != "" && !seen[*href] {
				links = append(links, *href)
				seen[*href] = true
			}
		}
	}

	// Extract ng-href links (AngularJS)
	ngElements, err := page.Elements("[ng-href]")
	if err == nil {
		for _, el := range ngElements {
			href, err := el.Attribute("ng-href")
			if err == nil && href != nil && *href != "" && !seen[*href] {
				links = append(links, *href)
				seen[*href] = true
			}
		}
	}

	// Extract ui-sref links (Angular UI Router)
	srefElements, err := page.Elements("[ui-sref]")
	if err == nil {
		for _, el := range srefElements {
			sref, err := el.Attribute("ui-sref")
			if err == nil && sref != nil && *sref != "" {
				// Convert ui-sref to hash route
				stateName := *sref
				if idx := indexOf(stateName, "("); idx != -1 {
					stateName = stateName[:idx]
				}
				hashURL := "#/" + stateName
				if !seen[hashURL] {
					links = append(links, hashURL)
					seen[hashURL] = true
				}
			}
		}
	}

	// Extract routerLink (Angular 2+)
	routerElements, err := page.Elements("[routerLink]")
	if err == nil {
		for _, el := range routerElements {
			routerLink, err := el.Attribute("routerLink")
			if err == nil && routerLink != nil && *routerLink != "" && !seen[*routerLink] {
				links = append(links, *routerLink)
				seen[*routerLink] = true
			}
		}
	}

	// Try to extract routes from Angular's route configuration via JavaScript
	jsRoutes := b.extractAngularRoutes(page)
	for _, route := range jsRoutes {
		if !seen[route] {
			links = append(links, route)
			seen[route] = true
		}
	}

	return links
}

// extractAngularRoutes tries to extract route definitions from Angular apps.
func (b *Browser) extractAngularRoutes(page *rod.Page) []string {
	routes := make([]string, 0)

	// Try to get routes from AngularJS $route service and other sources
	js := `() => {
		let routes = [];
		let seen = new Set();

		function addRoute(path) {
			if (path && !seen.has(path)) {
				seen.add(path);
				// Normalize the path
				if (!path.startsWith('#')) {
					path = '#' + path;
				}
				if (!path.startsWith('#/')) {
					path = '#/' + path.substring(1);
				}
				routes.push(path);
			}
		}

		try {
			if (window.angular) {
				let el = document.querySelector('[ng-app], [data-ng-app]');
				if (el) {
					let injector = angular.element(el).injector();
					if (injector) {
						// Try ngRoute
						try {
							let $route = injector.get('$route');
							if ($route && $route.routes) {
								for (let path in $route.routes) {
									if (path && path !== 'null') {
										// Include routes with params too, just mark them
										addRoute(path);
									}
								}
							}
						} catch(e) {}

						// Try UI Router
						try {
							let $state = injector.get('$state');
							if ($state) {
								let states = $state.get();
								states.forEach(s => {
									if (s.url && !s.abstract) {
										addRoute(s.url);
									}
									if (s.name) {
										addRoute('/' + s.name.replace(/\./g, '/'));
									}
								});
							}
						} catch(e) {}
					}
				}
			}

			// Look for hash links in onclick/ng-click attributes
			document.querySelectorAll('[ng-click], [onclick]').forEach(el => {
				let handler = el.getAttribute('ng-click') || el.getAttribute('onclick') || '';
				// Look for patterns like location.path('/xxx') or $location.url('/xxx')
				let matches = handler.match(/(?:location\.(?:path|url)|go)\s*\(\s*['"]([^'"]+)['"]\s*\)/g);
				if (matches) {
					matches.forEach(m => {
						let pathMatch = m.match(/['"]([^'"]+)['"]/);
						if (pathMatch) {
							addRoute(pathMatch[1]);
						}
					});
				}
			});

			// Look for href attributes with hash
			document.querySelectorAll('a[href^="#"]').forEach(el => {
				let href = el.getAttribute('href');
				if (href && href.length > 1) {
					addRoute(href);
				}
			});

			// Look in script tags for route configurations
			document.querySelectorAll('script').forEach(script => {
				let text = script.textContent || '';
				// Match .when('/path', ...) or .state('name', {url: '/path'})
				let whenMatches = text.match(/\.when\s*\(\s*['"]([^'"]+)['"]/g);
				if (whenMatches) {
					whenMatches.forEach(m => {
						let pathMatch = m.match(/['"]([^'"]+)['"]/);
						if (pathMatch) {
							addRoute(pathMatch[1]);
						}
					});
				}

				let stateMatches = text.match(/url\s*:\s*['"]([^'"]+)['"]/g);
				if (stateMatches) {
					stateMatches.forEach(m => {
						let pathMatch = m.match(/['"]([^'"]+)['"]/);
						if (pathMatch) {
							addRoute(pathMatch[1]);
						}
					});
				}
			});

		} catch(e) {
			console.error('Route extraction error:', e);
		}

		return routes;
	}`

	result, err := page.Eval(js)
	if err == nil && result != nil {
		if arr, ok := result.Value.Val().([]interface{}); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					routes = append(routes, s)
				}
			}
		}
	}

	return routes
}

// extractScripts extracts all script sources from the page.
func (b *Browser) extractScripts(page *rod.Page) []string {
	scripts := make([]string, 0)

	elements, err := page.Elements("script[src]")
	if err != nil {
		return scripts
	}

	for _, el := range elements {
		src, err := el.Attribute("src")
		if err == nil && src != nil && *src != "" {
			scripts = append(scripts, *src)
		}
	}

	return scripts
}

// extractForms extracts all forms from the page.
func (b *Browser) extractForms(page *rod.Page) []FormData {
	forms := make([]FormData, 0)

	elements, err := page.Elements("form")
	if err != nil {
		return forms
	}

	for _, el := range elements {
		form := FormData{
			Inputs: make([]InputData, 0),
		}

		if action, _ := el.Attribute("action"); action != nil {
			form.Action = *action
		}
		if method, _ := el.Attribute("method"); method != nil {
			form.Method = *method
		} else {
			form.Method = "GET"
		}
		if enctype, _ := el.Attribute("enctype"); enctype != nil {
			form.Enctype = *enctype
		}
		if id, _ := el.Attribute("id"); id != nil {
			form.ID = *id
		}
		if name, _ := el.Attribute("name"); name != nil {
			form.Name = *name
		}

		// Extract inputs
		inputs, _ := el.Elements("input, textarea, select")
		for _, input := range inputs {
			inputData := InputData{}

			if name, _ := input.Attribute("name"); name != nil {
				inputData.Name = *name
			}
			if inputType, _ := input.Attribute("type"); inputType != nil {
				inputData.Type = *inputType
			} else {
				inputData.Type = "text"
			}
			if value, _ := input.Attribute("value"); value != nil {
				inputData.Value = *value
			}
			if _, err := input.Attribute("required"); err == nil {
				inputData.Required = true
			}
			if placeholder, _ := input.Attribute("placeholder"); placeholder != nil {
				inputData.Placeholder = *placeholder
			}
			if pattern, _ := input.Attribute("pattern"); pattern != nil {
				inputData.Pattern = *pattern
			}

			form.Inputs = append(form.Inputs, inputData)
		}

		forms = append(forms, form)
	}

	return forms
}

// extractWebSockets looks for WebSocket connections in the page.
func (b *Browser) extractWebSockets(page *rod.Page) []string {
	wsURLs := make([]string, 0)

	// Try to find WebSocket URLs in scripts
	scripts, _ := page.Elements("script")
	for _, script := range scripts {
		text, _ := script.Text()
		// Look for common WebSocket patterns
		if contains := containsWebSocket(text); contains != "" {
			wsURLs = append(wsURLs, contains)
		}
	}

	return wsURLs
}

// containsWebSocket checks if text contains WebSocket URLs.
func containsWebSocket(text string) string {
	// Simplified check - in production would use regex
	if idx := indexOfAny(text, []string{"wss://", "ws://"}); idx != -1 {
		end := idx
		for end < len(text) && text[end] != '"' && text[end] != '\'' && text[end] != ' ' && text[end] != ')' {
			end++
		}
		return text[idx:end]
	}
	return ""
}

func indexOfAny(s string, substrs []string) int {
	for _, substr := range substrs {
		if idx := indexOf(s, substr); idx != -1 {
			return idx
		}
	}
	return -1
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// VisitHashRoute navigates to a hash-based route within an existing page.
// This is used for SPA navigation where the hash change doesn't reload the page.
func (b *Browser) VisitHashRoute(ctx context.Context, baseURL string, hashRoute string, headers map[string]string, cookies []*http.Cookie) (*PageResult, error) {
	return b.VisitHashRouteWithOptions(ctx, baseURL, hashRoute, headers, cookies, VisitOptions{FastMode: b.config.FastMode})
}

// VisitHashRouteWithOptions navigates to a hash-based route with options.
func (b *Browser) VisitHashRouteWithOptions(ctx context.Context, baseURL string, hashRoute string, headers map[string]string, cookies []*http.Cookie, opts VisitOptions) (*PageResult, error) {
	b.mu.Lock()
	b.pageCount++
	b.mu.Unlock()

	start := time.Now()

	// Construct full URL
	fullURL := baseURL
	if !strings.HasSuffix(baseURL, "/") && !strings.HasPrefix(hashRoute, "/") {
		fullURL += "/"
	}
	if !strings.HasPrefix(hashRoute, "#") {
		hashRoute = "#" + hashRoute
	}
	fullURL += hashRoute

	result := &PageResult{
		URL:         fullURL,
		XHRRequests: make([]NetworkRequest, 0),
	}

	// Create new page with context
	page, err := b.browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}
	defer page.Close()

	page = page.Context(ctx)

	// Set viewport
	_ = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:  b.config.ViewportWidth,
		Height: b.config.ViewportHeight,
	})

	// Set user agent
	if b.config.UserAgent != "" {
		_ = proto.NetworkSetUserAgentOverride{
			UserAgent: b.config.UserAgent,
		}.Call(page)
	}

	// Set extra headers
	if len(headers) > 0 {
		networkHeaders := make(proto.NetworkHeaders)
		for k, v := range headers {
			networkHeaders[k] = gson.New(v)
		}
		_ = proto.NetworkSetExtraHTTPHeaders{Headers: networkHeaders}.Call(page)
	}

	// Set cookies
	if len(cookies) > 0 {
		cookieParams := make([]*proto.NetworkCookieParam, 0, len(cookies))
		for _, cookie := range cookies {
			cookieParams = append(cookieParams, &proto.NetworkCookieParam{
				Name:     cookie.Name,
				Value:    cookie.Value,
				Domain:   cookie.Domain,
				Path:     cookie.Path,
				Secure:   cookie.Secure,
				HTTPOnly: cookie.HttpOnly,
			})
		}
		_ = page.SetCookies(cookieParams)
	}

	// Navigate to the full URL with hash
	err = page.Navigate(fullURL)
	if err != nil {
		result.Error = err
		return result, nil
	}

	// Wait for page to load
	err = page.WaitLoad()
	if err != nil {
		result.Error = err
		return result, nil
	}

	// Wait for Angular to process the route change (reduced in fast mode)
	if !opts.FastMode {
		_, _ = page.Eval(`() => {
			return new Promise((resolve) => {
				if (window.angular) {
					let el = document.querySelector('[ng-app], [data-ng-app]');
					if (el) {
						let scope = angular.element(el).scope();
						if (scope && scope.$apply) {
							scope.$apply();
						}
					}
					setTimeout(() => resolve(true), 1000);
				} else {
					setTimeout(() => resolve(true), 300);
				}
			});
		}`)
		time.Sleep(300 * time.Millisecond)
	} else {
		time.Sleep(100 * time.Millisecond)
	}

	// Get final URL
	info, err := page.Info()
	if err == nil && info != nil {
		result.FinalURL = info.URL
	} else {
		result.FinalURL = fullURL
	}

	// Get HTML content
	html, err := page.HTML()
	if err == nil {
		result.HTML = html
	}

	// Get title
	titleEl, err := page.Element("title")
	if err == nil && titleEl != nil {
		title, err := titleEl.Text()
		if err == nil {
			result.Title = title
		}
	}

	// Extract links
	result.Links = b.extractLinks(page)

	// Extract forms
	result.Forms = b.extractForms(page)

	// Extract WebSocket URLs
	result.WebSockets = b.extractWebSockets(page)

	result.ResponseTime = time.Since(start)

	return result, nil
}

// Close closes the browser.
func (b *Browser) Close() error {
	return b.browser.Close()
}

// PageCount returns the number of pages visited.
func (b *Browser) PageCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.pageCount
}

// NeedsRecycle checks if the browser needs recycling.
func (b *Browser) NeedsRecycle() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.config.RecycleAfter > 0 && b.pageCount >= b.config.RecycleAfter
}

// GetConfig returns the browser configuration.
func (b *Browser) GetConfig() Config {
	return b.config
}
