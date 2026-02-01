package browser

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
)

// AJAXHandler provides comprehensive AJAX handling for web pages.
type AJAXHandler struct {
	requests     []NetworkRequest
	mu           sync.Mutex
	intercepting bool
}

// AJAXResult contains results from AJAX analysis.
type AJAXResult struct {
	Requests       []NetworkRequest
	AJAXEndpoints  []AJAXEndpoint
	AJAXForms      []AJAXForm
	DynamicContent []string
}

// AJAXEndpoint represents a discovered AJAX endpoint.
type AJAXEndpoint struct {
	URL         string
	Method      string
	ContentType string
	Parameters  []string
	Source      string // click, scroll, form, script, etc.
	Trigger     string // Element or event that triggered the request
}

// AJAXForm represents a form that submits via AJAX.
type AJAXForm struct {
	FormID      string
	FormName    string
	Action      string
	Method      string
	SubmitType  string // jquery, fetch, xhr, axios
	Inputs      []InputData
	CallbackURL string
}

// NewAJAXHandler creates a new AJAX handler.
func NewAJAXHandler() *AJAXHandler {
	return &AJAXHandler{
		requests: make([]NetworkRequest, 0),
	}
}

// InjectAJAXInterceptor injects JavaScript to intercept all AJAX calls.
func (h *AJAXHandler) InjectAJAXInterceptor(page *rod.Page) error {
	script := `
	(function() {
		// Store for captured requests
		if (!window.__ajaxCapture) {
			window.__ajaxCapture = {
				requests: [],
				maxRequests: 500
			};
		}

		function captureRequest(method, url, data, type, trigger) {
			if (window.__ajaxCapture.requests.length >= window.__ajaxCapture.maxRequests) {
				window.__ajaxCapture.requests.shift();
			}
			window.__ajaxCapture.requests.push({
				method: method.toUpperCase(),
				url: url,
				data: data,
				type: type,
				trigger: trigger || 'unknown',
				timestamp: Date.now()
			});
		}

		// Intercept XMLHttpRequest
		if (!window.__xhrIntercepted) {
			window.__xhrIntercepted = true;
			let originalXHROpen = XMLHttpRequest.prototype.open;
			let originalXHRSend = XMLHttpRequest.prototype.send;

			XMLHttpRequest.prototype.open = function(method, url, async, user, password) {
				this.__method = method;
				this.__url = url;
				return originalXHROpen.apply(this, arguments);
			};

			XMLHttpRequest.prototype.send = function(data) {
				captureRequest(this.__method || 'GET', this.__url, data, 'xhr', 'XMLHttpRequest');
				return originalXHRSend.apply(this, arguments);
			};
		}

		// Intercept fetch
		if (!window.__fetchIntercepted) {
			window.__fetchIntercepted = true;
			let originalFetch = window.fetch;

			window.fetch = function(input, init) {
				let url = typeof input === 'string' ? input : input.url;
				let method = (init && init.method) ? init.method : 'GET';
				let body = (init && init.body) ? init.body : null;

				captureRequest(method, url, body, 'fetch', 'fetch API');
				return originalFetch.apply(this, arguments);
			};
		}

		// Intercept jQuery AJAX (if jQuery is present)
		if (window.jQuery && !window.__jqueryIntercepted) {
			window.__jqueryIntercepted = true;

			jQuery(document).ajaxSend(function(event, jqXHR, settings) {
				captureRequest(
					settings.type || 'GET',
					settings.url,
					settings.data,
					'jquery',
					'jQuery.ajax'
				);
			});
		}

		// Intercept axios (if axios is present)
		if (window.axios && !window.__axiosIntercepted) {
			window.__axiosIntercepted = true;

			axios.interceptors.request.use(function(config) {
				captureRequest(
					config.method || 'GET',
					config.url,
					config.data,
					'axios',
					'axios'
				);
				return config;
			});
		}

		// Intercept Angular $http (if AngularJS is present)
		if (window.angular && !window.__angularIntercepted) {
			window.__angularIntercepted = true;
			try {
				let el = document.querySelector('[ng-app], [data-ng-app]');
				if (el) {
					let injector = angular.element(el).injector();
					if (injector) {
						let $http = injector.get('$http');
						if ($http) {
							let originalHttp = $http;
							['get', 'post', 'put', 'delete', 'patch', 'head', 'options'].forEach(method => {
								if ($http[method]) {
									let original = $http[method];
									$http[method] = function(url, config) {
										captureRequest(method.toUpperCase(), url, config, 'angular', '$http.' + method);
										return original.apply(this, arguments);
									};
								}
							});
						}
					}
				}
			} catch(e) {}
		}

		// Monitor form submissions that might be AJAX
		if (!window.__formIntercepted) {
			window.__formIntercepted = true;

			document.addEventListener('submit', function(e) {
				let form = e.target;
				if (form.tagName === 'FORM') {
					// Check if this form might submit via AJAX
					let action = form.getAttribute('action') || window.location.href;
					let method = (form.getAttribute('method') || 'GET').toUpperCase();

					// Check for common AJAX form patterns
					let onsubmit = form.getAttribute('onsubmit') || '';
					let hasAjaxHandler = onsubmit.includes('ajax') ||
						onsubmit.includes('fetch') ||
						onsubmit.includes('return false') ||
						onsubmit.includes('preventDefault');

					if (hasAjaxHandler) {
						let formData = new FormData(form);
						let data = {};
						for (let pair of formData.entries()) {
							data[pair[0]] = pair[1];
						}
						captureRequest(method, action, JSON.stringify(data), 'ajax-form', 'form#' + (form.id || form.name || 'unnamed'));
					}
				}
			}, true);
		}

		// Monitor click events that might trigger AJAX
		if (!window.__clickIntercepted) {
			window.__clickIntercepted = true;

			document.addEventListener('click', function(e) {
				let target = e.target;

				// Check if the click target has data attributes suggesting AJAX
				let ajaxUrl = target.getAttribute('data-ajax-url') ||
					target.getAttribute('data-url') ||
					target.getAttribute('data-href') ||
					target.getAttribute('data-action') ||
					target.getAttribute('data-endpoint');

				if (ajaxUrl) {
					let method = target.getAttribute('data-method') || 'GET';
					captureRequest(method, ajaxUrl, null, 'data-attr', 'click on ' + target.tagName);
				}

				// Check for ng-click handlers
				let ngClick = target.getAttribute('ng-click');
				if (ngClick) {
					// Try to extract URL from ng-click if it contains AJAX calls
					let urlMatch = ngClick.match(/['"](\/[^'"]+|https?:\/\/[^'"]+)['"]/);
					if (urlMatch) {
						captureRequest('POST', urlMatch[1], null, 'ng-click', 'ng-click: ' + ngClick);
					}
				}
			}, true);
		}

		console.log('[AJAX Interceptor] Installed successfully');
	})();
	`

	_, err := page.Eval(script)
	return err
}

// GetCapturedRequests retrieves all captured AJAX requests from the page.
func (h *AJAXHandler) GetCapturedRequests(page *rod.Page) []NetworkRequest {
	requests := make([]NetworkRequest, 0)

	result, err := page.Eval(`() => {
		if (window.__ajaxCapture && window.__ajaxCapture.requests) {
			return window.__ajaxCapture.requests;
		}
		return [];
	}`)

	if err != nil {
		return requests
	}

	if arr, ok := result.Value.Val().([]interface{}); ok {
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				req := NetworkRequest{
					ResourceType: "ajax",
					Timestamp:    time.Now(),
				}
				if url, ok := m["url"].(string); ok {
					req.URL = url
				}
				if method, ok := m["method"].(string); ok {
					req.Method = method
				}
				if data, ok := m["data"].(string); ok {
					req.PostData = data
				}
				if reqType, ok := m["type"].(string); ok {
					req.ResourceType = reqType
				}
				requests = append(requests, req)
			}
		}
	}

	return requests
}

// TriggerAJAXEvents triggers common events that might cause AJAX requests.
func (h *AJAXHandler) TriggerAJAXEvents(page *rod.Page) error {
	// Script to trigger various events that commonly load content via AJAX
	script := `
	(function() {
		let triggered = [];

		// Trigger click on elements with AJAX-related attributes
		let ajaxElements = document.querySelectorAll([
			'[data-ajax]',
			'[data-ajax-url]',
			'[data-url]',
			'[data-action]',
			'[data-load]',
			'[data-toggle="ajax"]',
			'[ng-click]',
			'[v-on\\:click]',
			'[@click]',
			'.ajax-link',
			'.load-more',
			'.pagination a',
			'[data-page]',
			'button[type="button"]:not([disabled])',
			'[role="button"]'
		].join(', '));

		ajaxElements.forEach((el, index) => {
			if (index < 20) { // Limit to prevent overwhelming
				try {
					el.click();
					triggered.push(el.outerHTML.substring(0, 100));
				} catch(e) {}
			}
		});

		// Trigger scroll to load lazy content
		window.scrollTo(0, document.body.scrollHeight / 2);
		setTimeout(() => {
			window.scrollTo(0, document.body.scrollHeight);
		}, 500);
		setTimeout(() => {
			window.scrollTo(0, 0);
		}, 1000);

		// Trigger resize event (some apps load content on resize)
		window.dispatchEvent(new Event('resize'));

		// Trigger custom events that might load content
		['load', 'ready', 'init', 'refresh'].forEach(eventName => {
			try {
				document.dispatchEvent(new CustomEvent(eventName));
			} catch(e) {}
		});

		return triggered;
	})();
	`

	_, err := page.Eval(script)
	if err != nil {
		return err
	}

	// Wait for AJAX requests to complete
	time.Sleep(2 * time.Second)

	return nil
}

// WaitForAJAX waits for pending AJAX requests to complete.
func (h *AJAXHandler) WaitForAJAX(page *rod.Page, timeout time.Duration) error {
	maxWaitMs := int(timeout.Milliseconds())
	script := fmt.Sprintf(`
	() => {
		return new Promise((resolve) => {
			let maxWait = %d;
			let checkInterval = 100;
			let elapsed = 0;

			function checkPending() {
				let pending = false;

				// Check jQuery AJAX
				if (window.jQuery && jQuery.active > 0) {
					pending = true;
				}

				// Check axios (if it has pending requests indicator)
				if (window.axios && window.axios.pendingRequests > 0) {
					pending = true;
				}

				// Check Angular (if http service has pending requests)
				if (window.angular) {
					try {
						let el = document.querySelector('[ng-app], [data-ng-app]');
						if (el) {
							let injector = angular.element(el).injector();
							let httpSvc = injector.get('$'+'http');
							if (httpSvc && httpSvc.pendingRequests && httpSvc.pendingRequests.length > 0) {
								pending = true;
							}
						}
					} catch(e) {}
				}

				if (!pending || elapsed >= maxWait) {
					resolve(true);
				} else {
					elapsed += checkInterval;
					setTimeout(checkPending, checkInterval);
				}
			}

			checkPending();
		});
	}
	`, maxWaitMs)

	_, err := page.Eval(script)
	return err
}

// ExtractAJAXEndpoints extracts AJAX endpoints from JavaScript code.
func (h *AJAXHandler) ExtractAJAXEndpoints(page *rod.Page) []AJAXEndpoint {
	endpoints := make([]AJAXEndpoint, 0)

	script := `
	() => {
		let endpoints = [];
		let seen = new Set();

		function addEndpoint(url, method, source, params) {
			if (!url || seen.has(url + method)) return;
			seen.add(url + method);

			// Skip non-API URLs
			if (url.match(/\.(js|css|png|jpg|gif|svg|ico|woff|woff2|ttf|eot)(\?|$)/i)) return;

			endpoints.push({
				url: url,
				method: method || 'GET',
				source: source,
				parameters: params || []
			});
		}

		// Search all script tags for AJAX patterns
		document.querySelectorAll('script').forEach(script => {
			let text = script.textContent || script.innerText || '';

			// jQuery AJAX patterns
			let jqueryPatterns = [
				/\$\.ajax\s*\(\s*\{[^}]*url\s*:\s*['"]([^'"]+)['"]/g,
				/\$\.get\s*\(\s*['"]([^'"]+)['"]/g,
				/\$\.post\s*\(\s*['"]([^'"]+)['"]/g,
				/\$\.getJSON\s*\(\s*['"]([^'"]+)['"]/g,
				/\$\.(get|post|put|delete|patch)\s*\(\s*['"]([^'"]+)['"]/g,
				/jQuery\.ajax\s*\(\s*\{[^}]*url\s*:\s*['"]([^'"]+)['"]/g
			];

			jqueryPatterns.forEach(pattern => {
				let matches = text.matchAll(pattern);
				for (let match of matches) {
					let url = match[2] || match[1];
					let method = match[1] && ['get','post','put','delete','patch'].includes(match[1].toLowerCase())
						? match[1].toUpperCase()
						: 'GET';
					addEndpoint(url, method, 'jquery', []);
				}
			});

			// Fetch API patterns
			let fetchPatterns = [
				/fetch\s*\(\s*['"]([^'"]+)['"](?:\s*,\s*\{[^}]*method\s*:\s*['"]([^'"]+)['"])?\s*\)/g,
				/fetch\s*\(\s*['"](\/[^'"]+)['"]/g
			];

			fetchPatterns.forEach(pattern => {
				let matches = text.matchAll(pattern);
				for (let match of matches) {
					addEndpoint(match[1], match[2] || 'GET', 'fetch', []);
				}
			});

			// XMLHttpRequest patterns
			let xhrPatterns = [
				/\.open\s*\(\s*['"](\w+)['"]\s*,\s*['"]([^'"]+)['"]/g
			];

			xhrPatterns.forEach(pattern => {
				let matches = text.matchAll(pattern);
				for (let match of matches) {
					addEndpoint(match[2], match[1], 'xhr', []);
				}
			});

			// Axios patterns
			let axiosPatterns = [
				/axios\s*\.\s*(get|post|put|delete|patch|head|options)\s*\(\s*['"]([^'"]+)['"]/g,
				/axios\s*\(\s*\{[^}]*url\s*:\s*['"]([^'"]+)['"][^}]*method\s*:\s*['"]([^'"]+)['"]/g,
				/axios\s*\(\s*\{[^}]*method\s*:\s*['"]([^'"]+)['"][^}]*url\s*:\s*['"]([^'"]+)['"]/g
			];

			axiosPatterns.forEach(pattern => {
				let matches = text.matchAll(pattern);
				for (let match of matches) {
					if (match[2] && match[1]) {
						addEndpoint(match[2], match[1].toUpperCase(), 'axios', []);
					}
				}
			});

			// Angular $http patterns
			let angularPatterns = [
				/\$http\s*\.\s*(get|post|put|delete|patch|head)\s*\(\s*['"]([^'"]+)['"]/g,
				/\$http\s*\(\s*\{[^}]*url\s*:\s*['"]([^'"]+)['"][^}]*method\s*:\s*['"]([^'"]+)['"]/g
			];

			angularPatterns.forEach(pattern => {
				let matches = text.matchAll(pattern);
				for (let match of matches) {
					if (match[2] && match[1]) {
						addEndpoint(match[2], match[1].toUpperCase(), 'angular', []);
					}
				}
			});

			// Generic API URL patterns
			let apiPatterns = [
				/['"](\/?api\/[^'"]+)['"]/g,
				/['"](\/?v[0-9]+\/[^'"]+)['"]/g,
				/['"](\/?rest\/[^'"]+)['"]/g,
				/['"](\/?graphql[^'"]*)['"]/g,
				/['"](\/?ajax\/[^'"]+)['"]/g
			];

			apiPatterns.forEach(pattern => {
				let matches = text.matchAll(pattern);
				for (let match of matches) {
					addEndpoint(match[1], 'GET', 'pattern', []);
				}
			});
		});

		// Check data attributes on elements
		document.querySelectorAll('[data-ajax-url], [data-url], [data-action], [data-endpoint]').forEach(el => {
			let url = el.getAttribute('data-ajax-url') ||
				el.getAttribute('data-url') ||
				el.getAttribute('data-action') ||
				el.getAttribute('data-endpoint');
			let method = el.getAttribute('data-method') || 'GET';
			if (url) {
				addEndpoint(url, method.toUpperCase(), 'data-attribute', []);
			}
		});

		return endpoints;
	}
	`

	result, err := page.Eval(script)
	if err != nil {
		return endpoints
	}

	if arr, ok := result.Value.Val().([]interface{}); ok {
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				ep := AJAXEndpoint{}
				if url, ok := m["url"].(string); ok {
					ep.URL = url
				}
				if method, ok := m["method"].(string); ok {
					ep.Method = method
				}
				if source, ok := m["source"].(string); ok {
					ep.Source = source
				}
				if params, ok := m["parameters"].([]interface{}); ok {
					for _, p := range params {
						if ps, ok := p.(string); ok {
							ep.Parameters = append(ep.Parameters, ps)
						}
					}
				}
				endpoints = append(endpoints, ep)
			}
		}
	}

	return endpoints
}

// ExtractAJAXForms finds forms that submit via AJAX.
func (h *AJAXHandler) ExtractAJAXForms(page *rod.Page) []AJAXForm {
	forms := make([]AJAXForm, 0)

	script := `
	() => {
		let ajaxForms = [];

		document.querySelectorAll('form').forEach(form => {
			let isAjax = false;
			let submitType = 'unknown';
			let callbackURL = '';

			// Check form attributes
			let onsubmit = form.getAttribute('onsubmit') || '';
			let action = form.getAttribute('action') || '';
			let method = (form.getAttribute('method') || 'GET').toUpperCase();
			let dataAjax = form.getAttribute('data-ajax');
			let ngSubmit = form.getAttribute('ng-submit');
			let vOnSubmit = form.getAttribute('v-on:submit') || form.getAttribute('@submit');

			// Determine if form is AJAX-based
			if (dataAjax === 'true' || dataAjax === '1') {
				isAjax = true;
				submitType = 'data-ajax';
			} else if (onsubmit.includes('ajax') || onsubmit.includes('fetch') ||
				onsubmit.includes('XMLHttpRequest') || onsubmit.includes('$.post') ||
				onsubmit.includes('$.ajax') || onsubmit.includes('axios')) {
				isAjax = true;
				submitType = 'onsubmit';

				// Try to extract callback URL
				let urlMatch = onsubmit.match(/['"](\/[^'"]+|https?:\/\/[^'"]+)['"]/);
				if (urlMatch) {
					callbackURL = urlMatch[1];
				}
			} else if (onsubmit.includes('return false') || onsubmit.includes('preventDefault')) {
				isAjax = true;
				submitType = 'prevented';
			} else if (ngSubmit) {
				isAjax = true;
				submitType = 'angular';
			} else if (vOnSubmit) {
				isAjax = true;
				submitType = 'vue';
			}

			// Check for React form handlers (look for synthetic event handlers)
			if (form.__reactProps$) {
				isAjax = true;
				submitType = 'react';
			}

			// Check submit button for AJAX indicators
			let submitBtn = form.querySelector('button[type="submit"], input[type="submit"]');
			if (submitBtn) {
				let btnClick = submitBtn.getAttribute('onclick') || '';
				if (btnClick.includes('ajax') || btnClick.includes('fetch')) {
					isAjax = true;
					submitType = 'button-onclick';
				}
			}

			if (isAjax) {
				let inputs = [];
				form.querySelectorAll('input, select, textarea').forEach(input => {
					inputs.push({
						name: input.getAttribute('name') || '',
						type: input.getAttribute('type') || input.tagName.toLowerCase(),
						value: input.value || '',
						required: input.hasAttribute('required')
					});
				});

				ajaxForms.push({
					id: form.id || '',
					name: form.name || '',
					action: action,
					method: method,
					submitType: submitType,
					inputs: inputs,
					callbackURL: callbackURL
				});
			}
		});

		return ajaxForms;
	}
	`

	result, err := page.Eval(script)
	if err != nil {
		return forms
	}

	if arr, ok := result.Value.Val().([]interface{}); ok {
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				form := AJAXForm{
					Inputs: make([]InputData, 0),
				}
				if id, ok := m["id"].(string); ok {
					form.FormID = id
				}
				if name, ok := m["name"].(string); ok {
					form.FormName = name
				}
				if action, ok := m["action"].(string); ok {
					form.Action = action
				}
				if method, ok := m["method"].(string); ok {
					form.Method = method
				}
				if submitType, ok := m["submitType"].(string); ok {
					form.SubmitType = submitType
				}
				if callbackURL, ok := m["callbackURL"].(string); ok {
					form.CallbackURL = callbackURL
				}
				if inputs, ok := m["inputs"].([]interface{}); ok {
					for _, inp := range inputs {
						if inputMap, ok := inp.(map[string]interface{}); ok {
							input := InputData{}
							if name, ok := inputMap["name"].(string); ok {
								input.Name = name
							}
							if t, ok := inputMap["type"].(string); ok {
								input.Type = t
							}
							if v, ok := inputMap["value"].(string); ok {
								input.Value = v
							}
							if req, ok := inputMap["required"].(bool); ok {
								input.Required = req
							}
							form.Inputs = append(form.Inputs, input)
						}
					}
				}
				forms = append(forms, form)
			}
		}
	}

	return forms
}

// MonitorDynamicContent monitors for dynamically loaded content.
func (h *AJAXHandler) MonitorDynamicContent(page *rod.Page, duration time.Duration) []string {
	newContent := make([]string, 0)

	// Get initial content hash
	initialScript := `
	() => {
		window.__contentMonitor = {
			initialLinks: new Set(),
			newContent: []
		};

		document.querySelectorAll('a[href]').forEach(a => {
			window.__contentMonitor.initialLinks.add(a.href);
		});

		// Set up mutation observer
		window.__contentObserver = new MutationObserver((mutations) => {
			mutations.forEach(mutation => {
				mutation.addedNodes.forEach(node => {
					if (node.nodeType === Node.ELEMENT_NODE) {
						// Check for new links
						if (node.tagName === 'A' && node.href) {
							if (!window.__contentMonitor.initialLinks.has(node.href)) {
								window.__contentMonitor.newContent.push({
									type: 'link',
									value: node.href
								});
							}
						}
						// Check children
						node.querySelectorAll && node.querySelectorAll('a[href]').forEach(a => {
							if (!window.__contentMonitor.initialLinks.has(a.href)) {
								window.__contentMonitor.newContent.push({
									type: 'link',
									value: a.href
								});
							}
						});
					}
				});
			});
		});

		window.__contentObserver.observe(document.body, {
			childList: true,
			subtree: true
		});

		return true;
	}
	`

	page.Eval(initialScript)

	// Wait for specified duration
	time.Sleep(duration)

	// Get new content
	resultScript := `
	() => {
		if (window.__contentObserver) {
			window.__contentObserver.disconnect();
		}
		return window.__contentMonitor ? window.__contentMonitor.newContent : [];
	}
	`

	result, err := page.Eval(resultScript)
	if err != nil {
		return newContent
	}

	if arr, ok := result.Value.Val().([]interface{}); ok {
		seen := make(map[string]bool)
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				if value, ok := m["value"].(string); ok && !seen[value] {
					seen[value] = true
					newContent = append(newContent, value)
				}
			}
		}
	}

	return newContent
}

// AnalyzeAJAX performs comprehensive AJAX analysis on a page.
func (h *AJAXHandler) AnalyzeAJAX(page *rod.Page) *AJAXResult {
	result := &AJAXResult{
		Requests:       make([]NetworkRequest, 0),
		AJAXEndpoints:  make([]AJAXEndpoint, 0),
		AJAXForms:      make([]AJAXForm, 0),
		DynamicContent: make([]string, 0),
	}

	// Inject interceptor
	h.InjectAJAXInterceptor(page)

	// Wait for initial AJAX requests
	h.WaitForAJAX(page, 5*time.Second)

	// Trigger events that might cause AJAX
	h.TriggerAJAXEvents(page)

	// Wait again for triggered AJAX
	h.WaitForAJAX(page, 3*time.Second)

	// Collect results
	result.Requests = h.GetCapturedRequests(page)
	result.AJAXEndpoints = h.ExtractAJAXEndpoints(page)
	result.AJAXForms = h.ExtractAJAXForms(page)
	result.DynamicContent = h.MonitorDynamicContent(page, 2*time.Second)

	return result
}

// ExtractAJAXURLsFromJS extracts AJAX URLs from JavaScript content.
func ExtractAJAXURLsFromJS(jsContent string) []string {
	urls := make([]string, 0)
	seen := make(map[string]bool)

	patterns := []*regexp.Regexp{
		// jQuery patterns
		regexp.MustCompile(`\$\.ajax\s*\(\s*\{[^}]*url\s*:\s*['"]([^'"]+)['"]`),
		regexp.MustCompile(`\$\.(get|post|getJSON)\s*\(\s*['"]([^'"]+)['"]`),
		regexp.MustCompile(`jQuery\.ajax\s*\(\s*\{[^}]*url\s*:\s*['"]([^'"]+)['"]`),

		// Fetch patterns
		regexp.MustCompile(`fetch\s*\(\s*['"\x60]([^'"\x60]+)['"\x60]`),

		// XMLHttpRequest patterns
		regexp.MustCompile(`\.open\s*\(\s*['"]\w+['"]\s*,\s*['"]([^'"]+)['"]`),

		// Axios patterns
		regexp.MustCompile(`axios\s*\.\s*(?:get|post|put|delete|patch)\s*\(\s*['"]([^'"]+)['"]`),
		regexp.MustCompile(`axios\s*\(\s*\{[^}]*url\s*:\s*['"]([^'"]+)['"]`),

		// Angular patterns
		regexp.MustCompile(`\$http\s*\.\s*(?:get|post|put|delete)\s*\(\s*['"]([^'"]+)['"]`),
		regexp.MustCompile(`\$resource\s*\(\s*['"]([^'"]+)['"]`),

		// Generic API patterns
		regexp.MustCompile(`['"](\/?api\/[^'"]+)['"]`),
		regexp.MustCompile(`['"](\/?v[0-9]+\/[^'"]+)['"]`),
		regexp.MustCompile(`['"](\/?ajax\/[^'"]+)['"]`),
		regexp.MustCompile(`['"](\/?rest\/[^'"]+)['"]`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(jsContent, -1)
		for _, match := range matches {
			url := ""
			if len(match) > 2 {
				url = match[2]
			} else if len(match) > 1 {
				url = match[1]
			}
			if url != "" && !seen[url] && isValidAJAXURL(url) {
				seen[url] = true
				urls = append(urls, url)
			}
		}
	}

	return urls
}

// isValidAJAXURL checks if a URL looks like a valid AJAX endpoint.
func isValidAJAXURL(url string) bool {
	// Skip static resources
	staticExts := []string{".js", ".css", ".png", ".jpg", ".gif", ".svg", ".ico", ".woff", ".ttf"}
	lowerURL := strings.ToLower(url)
	for _, ext := range staticExts {
		if strings.HasSuffix(lowerURL, ext) {
			return false
		}
	}

	// Skip data URIs and javascript:
	if strings.HasPrefix(url, "data:") || strings.HasPrefix(url, "javascript:") {
		return false
	}

	// Should have some content
	if len(url) < 2 {
		return false
	}

	return true
}
