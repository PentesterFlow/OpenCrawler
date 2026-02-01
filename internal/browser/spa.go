package browser

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// SPAHandler handles Single Page Application specific challenges.
type SPAHandler struct {
	config SPAConfig
}

// SPAConfig contains SPA handling configuration.
type SPAConfig struct {
	// Maximum wait time for content to load
	MaxWaitTime time.Duration

	// Minimum content length to consider page loaded
	MinContentLength int

	// Selectors that indicate loading state
	LoadingSelectors []string

	// Selectors that indicate content is ready
	ReadySelectors []string

	// Enable content hash deduplication
	EnableContentDedup bool

	// Maximum retries for rendering
	MaxRetries int

	// Enable stealth mode (anti-detection)
	StealthMode bool
}

// DefaultSPAConfig returns sensible defaults for SPA handling.
func DefaultSPAConfig() SPAConfig {
	return SPAConfig{
		MaxWaitTime:      20 * time.Second,
		MinContentLength: 500,
		LoadingSelectors: []string{
			".loading", ".spinner", ".loader", ".skeleton",
			"[class*='loading']", "[class*='spinner']",
			"mat-spinner", "mat-progress-spinner",
			".ng-loading", ".ng-cloak",
			"[data-loading]", "[aria-busy='true']",
		},
		ReadySelectors: []string{
			"[ng-view]:not(:empty)",
			"[ui-view]:not(:empty)",
			"router-outlet + *",
			"#root > *:not(.loading)",
			"#app > *:not(.loading)",
			"app-root > *",
			"main:not(:empty)",
			"article",
			".content:not(:empty)",
		},
		EnableContentDedup: true,
		MaxRetries:         2,
		StealthMode:        true,
	}
}

// NewSPAHandler creates a new SPA handler.
func NewSPAHandler(config SPAConfig) *SPAHandler {
	return &SPAHandler{config: config}
}

// WaitForContent waits for SPA content to be fully loaded.
func (h *SPAHandler) WaitForContent(page *rod.Page) error {
	startTime := time.Now()
	checkInterval := 200 * time.Millisecond

	for {
		if time.Since(startTime) > h.config.MaxWaitTime {
			return fmt.Errorf("timeout waiting for SPA content")
		}

		// Check if loading indicators are gone
		loadingGone, _ := h.checkLoadingGone(page)

		// Check if content is ready
		contentReady, _ := h.checkContentReady(page)

		// Check minimum content length
		hasContent, _ := h.checkMinContent(page)

		// Check for network idle
		networkIdle := h.checkNetworkIdle(page)

		if loadingGone && contentReady && hasContent && networkIdle {
			// Extra small wait for any final renders
			time.Sleep(100 * time.Millisecond)
			return nil
		}

		time.Sleep(checkInterval)
	}
}

// checkLoadingGone checks if all loading indicators are gone.
func (h *SPAHandler) checkLoadingGone(page *rod.Page) (bool, error) {
	for _, selector := range h.config.LoadingSelectors {
		elements, err := page.Elements(selector)
		if err != nil {
			continue
		}
		for _, el := range elements {
			visible, err := el.Visible()
			if err == nil && visible {
				return false, nil
			}
		}
	}
	return true, nil
}

// checkContentReady checks if content is rendered.
func (h *SPAHandler) checkContentReady(page *rod.Page) (bool, error) {
	for _, selector := range h.config.ReadySelectors {
		elements, err := page.Elements(selector)
		if err != nil {
			continue
		}
		if len(elements) > 0 {
			// Check if element has actual content
			for _, el := range elements {
				text, err := el.Text()
				if err == nil && len(strings.TrimSpace(text)) > 10 {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

// checkMinContent checks if page has minimum content.
func (h *SPAHandler) checkMinContent(page *rod.Page) (bool, error) {
	result, err := page.Eval(`() => {
		let body = document.body;
		if (!body) return 0;

		// Get text content length, excluding scripts and styles
		let clone = body.cloneNode(true);
		let scripts = clone.querySelectorAll('script, style, noscript');
		scripts.forEach(s => s.remove());

		return clone.textContent.trim().length;
	}`)

	if err != nil {
		return false, err
	}

	length := int(result.Value.Num())
	return length >= h.config.MinContentLength, nil
}

// checkNetworkIdle checks if network is idle.
func (h *SPAHandler) checkNetworkIdle(page *rod.Page) bool {
	result, err := page.Eval(`() => {
		// Check for pending XHR/fetch requests
		if (window.__pendingRequests && window.__pendingRequests > 0) {
			return false;
		}

		// Check jQuery if present
		if (window.jQuery && jQuery.active > 0) {
			return false;
		}

		// Check Angular if present
		if (window.angular) {
			try {
				let el = document.querySelector('[ng-app], [data-ng-app]');
				if (el) {
					let injector = angular.element(el).injector();
					let $http = injector.get('$' + 'http');
					if ($http && $http.pendingRequests && $http.pendingRequests.length > 0) {
						return false;
					}
				}
			} catch(e) {}
		}

		return true;
	}`)

	if err != nil {
		return true // Assume idle on error
	}

	return result.Value.Bool()
}

// InjectNetworkMonitor injects a network request monitor.
func (h *SPAHandler) InjectNetworkMonitor(page *rod.Page) error {
	script := `
	(function() {
		if (window.__networkMonitorInjected) return;
		window.__networkMonitorInjected = true;
		window.__pendingRequests = 0;

		// Monitor XMLHttpRequest
		let origOpen = XMLHttpRequest.prototype.open;
		let origSend = XMLHttpRequest.prototype.send;

		XMLHttpRequest.prototype.open = function() {
			this.__monitored = true;
			return origOpen.apply(this, arguments);
		};

		XMLHttpRequest.prototype.send = function() {
			if (this.__monitored) {
				window.__pendingRequests++;
				this.addEventListener('loadend', () => {
					window.__pendingRequests = Math.max(0, window.__pendingRequests - 1);
				});
			}
			return origSend.apply(this, arguments);
		};

		// Monitor fetch
		let origFetch = window.fetch;
		window.fetch = function() {
			window.__pendingRequests++;
			return origFetch.apply(this, arguments).finally(() => {
				window.__pendingRequests = Math.max(0, window.__pendingRequests - 1);
			});
		};
	})();
	`

	_, err := page.Eval(script)
	return err
}

// ApplyStealthMode applies anti-detection measures.
func (h *SPAHandler) ApplyStealthMode(page *rod.Page) error {
	if !h.config.StealthMode {
		return nil
	}

	script := `
	(function() {
		// Hide webdriver
		Object.defineProperty(navigator, 'webdriver', {
			get: () => undefined
		});

		// Fix plugins
		Object.defineProperty(navigator, 'plugins', {
			get: () => [
				{name: 'Chrome PDF Plugin', filename: 'internal-pdf-viewer'},
				{name: 'Chrome PDF Viewer', filename: 'mhjfbmdgcfjbbpaeojofohoefgiehjai'},
				{name: 'Native Client', filename: 'internal-nacl-plugin'}
			]
		});

		// Fix languages
		Object.defineProperty(navigator, 'languages', {
			get: () => ['en-US', 'en']
		});

		// Fix permissions
		const originalQuery = window.navigator.permissions.query;
		window.navigator.permissions.query = (parameters) => (
			parameters.name === 'notifications' ?
				Promise.resolve({ state: Notification.permission }) :
				originalQuery(parameters)
		);

		// Fix chrome object
		window.chrome = {
			runtime: {},
			loadTimes: function() {},
			csi: function() {},
			app: {}
		};

		// Remove automation indicators
		delete window.callPhantom;
		delete window._phantom;
		delete window.__nightmare;
		delete window.Buffer;
		delete window.emit;
		delete window.spawn;

		// Fix iframe contentWindow
		Object.defineProperty(HTMLIFrameElement.prototype, 'contentWindow', {
			get: function() {
				return window;
			}
		});

		console.log('[Stealth] Applied anti-detection measures');
	})();
	`

	_, err := page.Eval(script)
	return err
}

// GetContentHash returns a hash of the page's meaningful content.
func (h *SPAHandler) GetContentHash(page *rod.Page) (string, error) {
	result, err := page.Eval(`() => {
		let body = document.body;
		if (!body) return '';

		// Clone and clean
		let clone = body.cloneNode(true);

		// Remove scripts, styles, comments
		clone.querySelectorAll('script, style, noscript, svg, iframe').forEach(el => el.remove());

		// Get text content
		let text = clone.textContent || '';

		// Normalize whitespace
		text = text.replace(/\s+/g, ' ').trim();

		// Also include key structural elements
		let structure = '';
		document.querySelectorAll('h1, h2, h3, a[href], img[src], form').forEach(el => {
			structure += el.tagName + ':' + (el.textContent || '').substring(0, 50) + ';';
		});

		return text.substring(0, 5000) + '|||' + structure;
	}`)

	if err != nil {
		return "", err
	}

	content := result.Value.Str()
	hash := md5.Sum([]byte(content))
	return hex.EncodeToString(hash[:]), nil
}

// IsSoftError checks if the page shows a soft error (404 page with 200 status).
func (h *SPAHandler) IsSoftError(page *rod.Page) (bool, string) {
	result, err := page.Eval(`() => {
		let body = document.body;
		if (!body) return {isError: false, reason: ''};

		let text = body.textContent.toLowerCase();

		// Check for common error patterns
		let errorPatterns = [
			{pattern: /page\s*not\s*found/i, reason: 'Page not found'},
			{pattern: /404\s*error/i, reason: '404 error'},
			{pattern: /not\s*found/i, reason: 'Not found'},
			{pattern: /does\s*not\s*exist/i, reason: 'Does not exist'},
			{pattern: /no\s*results?\s*found/i, reason: 'No results'},
			{pattern: /access\s*denied/i, reason: 'Access denied'},
			{pattern: /unauthorized/i, reason: 'Unauthorized'},
			{pattern: /forbidden/i, reason: 'Forbidden'},
			{pattern: /error\s*occurred/i, reason: 'Error occurred'},
			{pattern: /something\s*went\s*wrong/i, reason: 'Something went wrong'},
		];

		for (let ep of errorPatterns) {
			if (ep.pattern.test(text)) {
				return {isError: true, reason: ep.reason};
			}
		}

		// Check for error-specific elements
		let errorSelectors = [
			'.error-page', '.not-found', '.error-404', '#error',
			'[class*="error"]', '[class*="not-found"]'
		];

		for (let selector of errorSelectors) {
			let el = document.querySelector(selector);
			if (el && el.offsetHeight > 100) {
				return {isError: true, reason: 'Error element found: ' + selector};
			}
		}

		return {isError: false, reason: ''};
	}`)

	if err != nil {
		return false, ""
	}

	if m, ok := result.Value.Val().(map[string]interface{}); ok {
		isError, _ := m["isError"].(bool)
		reason, _ := m["reason"].(string)
		return isError, reason
	}

	return false, ""
}

// NormalizeHashURL normalizes a hash-based URL for deduplication.
func NormalizeHashURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	// Get the fragment (hash)
	fragment := parsed.Fragment

	// Remove hashbang prefix
	fragment = strings.TrimPrefix(fragment, "!")

	// Parse fragment as if it were a path with query
	if strings.Contains(fragment, "?") {
		parts := strings.SplitN(fragment, "?", 2)
		path := parts[0]
		queryStr := parts[1]

		// Parse and sort query parameters
		query, _ := url.ParseQuery(queryStr)

		// Remove non-routing parameters (common UI state params)
		nonRoutingParams := []string{
			"modal", "popup", "dialog", "overlay",
			"scroll", "scrollTop", "scrollY",
			"tab", "panel", "accordion",
			"expanded", "collapsed", "open", "closed",
			"highlight", "focus", "selected",
			"timestamp", "t", "_", "nocache",
		}

		for _, param := range nonRoutingParams {
			query.Del(param)
		}

		// Reconstruct
		if len(query) > 0 {
			fragment = path + "?" + query.Encode()
		} else {
			fragment = path
		}
	}

	// Normalize path
	fragment = strings.TrimSuffix(fragment, "/")
	if !strings.HasPrefix(fragment, "/") {
		fragment = "/" + fragment
	}

	// Reconstruct URL
	parsed.Fragment = fragment
	return parsed.String()
}

// IsNonRoutingHash checks if a hash change is likely non-routing (UI state).
func IsNonRoutingHash(hash string) bool {
	// Remove # prefix
	hash = strings.TrimPrefix(hash, "#")
	hash = strings.TrimPrefix(hash, "!")

	// Empty or just /
	if hash == "" || hash == "/" {
		return true
	}

	// Common non-routing patterns
	nonRoutingPatterns := []*regexp.Regexp{
		regexp.MustCompile(`^modal[=-]`),
		regexp.MustCompile(`^popup[=-]`),
		regexp.MustCompile(`^tab[=-]`),
		regexp.MustCompile(`^scroll[=-]?\d*$`),
		regexp.MustCompile(`^page[=-]?\d+$`),
		regexp.MustCompile(`^filter[=-]`),
		regexp.MustCompile(`^sort[=-]`),
		regexp.MustCompile(`^[a-zA-Z]+-\d+$`), // Like element-123 (anchor links)
		regexp.MustCompile(`^\d+$`),            // Just a number
		regexp.MustCompile(`^[a-f0-9]{32}$`),   // MD5 hash
	}

	for _, pattern := range nonRoutingPatterns {
		if pattern.MatchString(hash) {
			return true
		}
	}

	return false
}

// ExtractShadowDOMContent extracts content from Shadow DOM elements.
func (h *SPAHandler) ExtractShadowDOMContent(page *rod.Page) ([]string, error) {
	result, err := page.Eval(`() => {
		let content = [];

		function extractFromShadow(root) {
			if (!root) return;

			// Get all elements that might have shadow roots
			let elements = root.querySelectorAll('*');
			elements.forEach(el => {
				if (el.shadowRoot) {
					// Extract links from shadow DOM
					el.shadowRoot.querySelectorAll('a[href]').forEach(a => {
						content.push({type: 'link', value: a.href});
					});

					// Extract forms
					el.shadowRoot.querySelectorAll('form').forEach(form => {
						content.push({type: 'form', value: form.action || ''});
					});

					// Recurse into nested shadow roots
					extractFromShadow(el.shadowRoot);
				}
			});
		}

		extractFromShadow(document);
		return content;
	}`)

	if err != nil {
		return nil, err
	}

	urls := make([]string, 0)
	if arr, ok := result.Value.Val().([]interface{}); ok {
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				if value, ok := m["value"].(string); ok && value != "" {
					urls = append(urls, value)
				}
			}
		}
	}

	return urls, nil
}

// HandleInfiniteScroll handles infinite scroll pages with limits.
func (h *SPAHandler) HandleInfiniteScroll(page *rod.Page, maxScrolls int, scrollDelay time.Duration) ([]string, error) {
	discoveredURLs := make([]string, 0)
	seen := make(map[string]bool)

	// Get initial links
	initialLinks := extractAllLinks(page)
	for _, link := range initialLinks {
		seen[link] = true
		discoveredURLs = append(discoveredURLs, link)
	}

	lastHeight := 0
	scrollCount := 0

	for scrollCount < maxScrolls {
		// Get current height
		result, err := page.Eval(`() => document.body.scrollHeight`)
		if err != nil {
			break
		}
		currentHeight := int(result.Value.Num())

		// Scroll to bottom
		_, _ = page.Eval(`() => window.scrollTo(0, document.body.scrollHeight)`)

		// Wait for content to load
		time.Sleep(scrollDelay)

		// Check for new links
		newLinks := extractAllLinks(page)
		for _, link := range newLinks {
			if !seen[link] {
				seen[link] = true
				discoveredURLs = append(discoveredURLs, link)
			}
		}

		// Check if we've reached the bottom
		if currentHeight == lastHeight {
			break
		}
		lastHeight = currentHeight
		scrollCount++
	}

	// Scroll back to top
	_, _ = page.Eval(`() => window.scrollTo(0, 0)`)

	return discoveredURLs, nil
}

func extractAllLinks(page *rod.Page) []string {
	links := make([]string, 0)

	result, _ := page.Eval(`() => {
		let links = [];
		document.querySelectorAll('a[href]').forEach(a => {
			links.push(a.href);
		});
		return links;
	}`)

	if result != nil {
		if arr, ok := result.Value.Val().([]interface{}); ok {
			for _, item := range arr {
				if s, ok := item.(string); ok {
					links = append(links, s)
				}
			}
		}
	}

	return links
}

// HandleAuthRedirect detects and handles auth redirects.
func (h *SPAHandler) HandleAuthRedirect(page *rod.Page) (bool, string) {
	result, err := page.Eval(`() => {
		let url = window.location.href.toLowerCase();
		let path = window.location.pathname.toLowerCase();
		let hash = window.location.hash.toLowerCase();

		// Check for login/auth pages
		let authPatterns = [
			'/login', '/signin', '/sign-in', '/auth',
			'/authenticate', '/sso', '/oauth', '/saml',
			'#/login', '#/signin', '#/auth'
		];

		for (let pattern of authPatterns) {
			if (url.includes(pattern) || path.includes(pattern) || hash.includes(pattern)) {
				return {isAuth: true, url: window.location.href};
			}
		}

		// Check for login form
		let loginForm = document.querySelector('form[action*="login"], form[action*="auth"], #loginForm, .login-form');
		if (loginForm) {
			let inputs = loginForm.querySelectorAll('input[type="password"]');
			if (inputs.length > 0) {
				return {isAuth: true, url: window.location.href};
			}
		}

		return {isAuth: false, url: ''};
	}`)

	if err != nil {
		return false, ""
	}

	if m, ok := result.Value.Val().(map[string]interface{}); ok {
		isAuth, _ := m["isAuth"].(bool)
		authURL, _ := m["url"].(string)
		return isAuth, authURL
	}

	return false, ""
}

// DetectMicroFrontends detects multiple Angular/SPA instances.
func (h *SPAHandler) DetectMicroFrontends(page *rod.Page) ([]string, error) {
	result, err := page.Eval(`() => {
		let apps = [];

		// Angular apps
		document.querySelectorAll('[ng-app], [data-ng-app]').forEach(el => {
			apps.push({type: 'angularjs', selector: el.tagName + '#' + (el.id || 'unknown')});
		});

		// Angular 2+ apps (look for multiple app-root or custom elements)
		let angularApps = document.querySelectorAll('app-root, [ng-version]');
		if (angularApps.length > 1) {
			angularApps.forEach((el, i) => {
				apps.push({type: 'angular', selector: el.tagName + '[' + i + ']'});
			});
		}

		// React roots
		let reactRoots = document.querySelectorAll('[data-reactroot]');
		if (reactRoots.length > 1) {
			reactRoots.forEach((el, i) => {
				apps.push({type: 'react', selector: el.tagName + '[' + i + ']'});
			});
		}

		// Vue apps
		let vueApps = document.querySelectorAll('[data-v-app], [data-server-rendered]');
		if (vueApps.length > 1) {
			vueApps.forEach((el, i) => {
				apps.push({type: 'vue', selector: el.tagName + '[' + i + ']'});
			});
		}

		// Iframes with SPAs
		document.querySelectorAll('iframe').forEach(iframe => {
			try {
				let doc = iframe.contentDocument;
				if (doc && (doc.querySelector('[ng-app]') || doc.querySelector('app-root') || doc.querySelector('[data-reactroot]'))) {
					apps.push({type: 'iframe-spa', selector: 'iframe[src="' + iframe.src + '"]'});
				}
			} catch(e) {} // Cross-origin iframe
		});

		return apps;
	}`)

	if err != nil {
		return nil, err
	}

	apps := make([]string, 0)
	if arr, ok := result.Value.Val().([]interface{}); ok {
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				appType, _ := m["type"].(string)
				selector, _ := m["selector"].(string)
				apps = append(apps, appType+":"+selector)
			}
		}
	}

	return apps, nil
}

// RecoverFromHang attempts to recover from a stuck page.
func (h *SPAHandler) RecoverFromHang(page *rod.Page) error {
	// Try to stop any infinite loops
	_, _ = page.Eval(`() => {
		// Stop all intervals and timeouts
		let highestId = setTimeout(() => {}, 0);
		for (let i = 0; i < highestId; i++) {
			clearTimeout(i);
			clearInterval(i);
		}

		// Cancel any pending requests
		if (window.__pendingRequests) {
			window.__pendingRequests = 0;
		}

		// Try to stop Angular digest if stuck
		if (window.angular) {
			try {
				let el = document.querySelector('[ng-app], [data-ng-app]');
				if (el) {
					let scope = angular.element(el).scope();
					if (scope && scope.$root) {
						scope.$root.$$phase = null;
					}
				}
			} catch(e) {}
		}

		return true;
	}`)

	return nil
}

// SetupPageErrorHandling sets up error handlers for the page.
func (h *SPAHandler) SetupPageErrorHandling(page *rod.Page) {
	// Handle JavaScript errors
	go page.EachEvent(func(e *proto.RuntimeExceptionThrown) {
		// Log but don't crash
		fmt.Printf("[JS Error] %s\n", e.ExceptionDetails.Text)
	})()
}
