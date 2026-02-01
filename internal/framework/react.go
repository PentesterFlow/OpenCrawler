package framework

import (
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// ReactHandler handles React applications.
type ReactHandler struct{}

// NewReactHandler creates a new React handler.
func NewReactHandler() *ReactHandler {
	return &ReactHandler{}
}

// Type returns the framework type.
func (h *ReactHandler) Type() Type {
	return TypeReact
}

// Detect checks if React is present on the page.
func (h *ReactHandler) Detect(page *rod.Page) bool {
	result, err := page.Eval(`() => {
		// Check for React indicators
		return !!(
			window.React ||
			window.__REACT_DEVTOOLS_GLOBAL_HOOK__ ||
			document.querySelector('[data-reactroot]') ||
			document.querySelector('[data-reactid]') ||
			document.querySelector('#root, #app, #__next') &&
				(document.querySelector('#root, #app, #__next')._reactRootContainer ||
				 document.querySelector('[class*="react"]'))
		);
	}`)
	if err != nil {
		return false
	}
	return result.Value.Bool()
}

// WaitForReady waits for React to be fully loaded.
func (h *ReactHandler) WaitForReady(page *rod.Page) error {
	_, err := page.Eval(`() => {
		return new Promise((resolve) => {
			let timeout = setTimeout(() => resolve(false), 10000);

			function checkReady() {
				// Check if root element has content
				let root = document.querySelector('#root, #app, #__next, [data-reactroot]');
				if (root && root.innerHTML.trim() !== '' && root.children.length > 0) {
					// Check if there are no loading indicators
					let loading = document.querySelector('[class*="loading"], [class*="spinner"], .loader');
					if (!loading || getComputedStyle(loading).display === 'none') {
						clearTimeout(timeout);
						resolve(true);
						return;
					}
				}
				setTimeout(checkReady, 100);
			}

			setTimeout(checkReady, 500);
		});
	}`)
	if err != nil {
		time.Sleep(2 * time.Second)
	}
	return nil
}

// ExtractRoutes extracts all routes from React application.
func (h *ReactHandler) ExtractRoutes(page *rod.Page) ([]Route, error) {
	result, err := page.Eval(`() => {
		let routes = [];

		// Try to find React Router routes from the DOM
		// Look for Route components rendered in the page
		document.querySelectorAll('[class*="route"], [data-path]').forEach(el => {
			let path = el.getAttribute('data-path');
			if (path) {
				routes.push({
					path: path,
					name: '',
					component: '',
					parameters: (path.match(/:\w+/g) || []).map(p => p.substring(1)),
					type: 'react-data-path'
				});
			}
		});

		// Extract from script tags (React Router configuration)
		document.querySelectorAll('script').forEach(script => {
			let text = script.textContent || '';

			// Match <Route path="/xxx"
			let routeMatches = text.matchAll(/<Route[^>]*path\s*=\s*["']([^"']+)["']/g);
			for (let match of routeMatches) {
				routes.push({
					path: match[1],
					name: '',
					component: '',
					parameters: (match[1].match(/:\w+/g) || []).map(p => p.substring(1)),
					type: 'react-route-jsx'
				});
			}

			// Match path: "/xxx" in route config objects
			let configMatches = text.matchAll(/\{\s*path\s*:\s*["']([^"']+)["']/g);
			for (let match of configMatches) {
				routes.push({
					path: match[1],
					name: '',
					component: '',
					parameters: (match[1].match(/:\w+/g) || []).map(p => p.substring(1)),
					type: 'react-route-config'
				});
			}

			// Match createBrowserRouter/createHashRouter routes
			let browserRouterMatches = text.matchAll(/createBrowserRouter|createHashRouter/g);
			if (browserRouterMatches) {
				// Look for route definitions near these
				let routeDefMatches = text.matchAll(/path\s*:\s*["']([^"']+)["']/g);
				for (let match of routeDefMatches) {
					routes.push({
						path: match[1],
						name: '',
						component: '',
						parameters: (match[1].match(/:\w+/g) || []).map(p => p.substring(1)),
						type: 'react-router-v6'
					});
				}
			}
		});

		// Next.js: look for page routes in __NEXT_DATA__
		if (window.__NEXT_DATA__) {
			try {
				let nextData = window.__NEXT_DATA__;
				if (nextData.page) {
					routes.push({
						path: nextData.page,
						name: 'next-current',
						component: '',
						parameters: [],
						type: 'nextjs-page'
					});
				}
				// Look for dynamic routes
				if (nextData.query) {
					Object.keys(nextData.query).forEach(key => {
						routes.push({
							path: nextData.page.replace('[' + key + ']', ':' + key),
							name: 'next-dynamic',
							component: '',
							parameters: [key],
							type: 'nextjs-dynamic'
						});
					});
				}
			} catch(e) {}
		}

		return routes;
	}`)

	if err != nil {
		return nil, err
	}

	return h.parseRoutes(result)
}

// ExtractLinks extracts all navigation links from the page.
func (h *ReactHandler) ExtractLinks(page *rod.Page) ([]Link, error) {
	result, err := page.Eval(`() => {
		let links = [];
		let seen = new Set();

		function addLink(url, text, type, attrs) {
			if (url && !seen.has(url)) {
				seen.add(url);
				links.push({url, text, type, attributes: attrs || {}});
			}
		}

		// Standard links (React Router Link components render as <a>)
		document.querySelectorAll('a[href]').forEach(el => {
			let href = el.getAttribute('href');
			if (href && !href.startsWith('http') && !href.startsWith('mailto:') && !href.startsWith('tel:') && !href.startsWith('javascript:')) {
				addLink(href, el.textContent.trim(), 'link', {});
			}
		});

		// Next.js Link components
		document.querySelectorAll('[class*="next-link"], a[data-next]').forEach(el => {
			let href = el.getAttribute('href');
			if (href) {
				addLink(href, el.textContent.trim(), 'next-link', {});
			}
		});

		// Look for onClick handlers with history.push or navigate
		// This requires React DevTools or examining the fiber
		document.querySelectorAll('button, [role="button"], [class*="btn"], [class*="link"]').forEach(el => {
			// Check if element has click handler by looking at React fiber
			let fiber = el._reactFiber || el.__reactFiber$;
			if (fiber && fiber.memoizedProps && fiber.memoizedProps.onClick) {
				// Can't easily extract the path, but mark it as potentially navigable
				let text = el.textContent.trim();
				if (text && text.length < 50) {
					// Try to guess path from text
					let guessedPath = '/' + text.toLowerCase().replace(/\s+/g, '-');
					addLink(guessedPath, text, 'react-onClick-guess', {guessed: 'true'});
				}
			}
		});

		// Hash links for hash router
		document.querySelectorAll('a[href^="#/"]').forEach(el => {
			let href = el.getAttribute('href');
			addLink(href, el.textContent.trim(), 'hash-link', {});
		});

		// NavLink active class elements
		document.querySelectorAll('[class*="active"], [aria-current="page"]').forEach(el => {
			if (el.tagName === 'A') {
				let href = el.getAttribute('href');
				if (href) {
					addLink(href, el.textContent.trim(), 'active-link', {active: 'true'});
				}
			}
		});

		return links;
	}`)

	if err != nil {
		return nil, err
	}

	return h.parseLinks(result)
}

// NavigateToRoute navigates to a specific route within the SPA.
func (h *ReactHandler) NavigateToRoute(page *rod.Page, route string) error {
	_, err := page.Eval(`(route) => {
		return new Promise((resolve) => {
			// Try using history API
			if (window.history && window.history.pushState) {
				window.history.pushState({}, '', route);
				// Dispatch popstate event to trigger React Router
				window.dispatchEvent(new PopStateEvent('popstate', {state: {}}));
			}

			// Also try direct navigation for hash routes
			if (route.startsWith('#')) {
				window.location.hash = route;
			}

			setTimeout(() => resolve(true), 500);
		});
	}`, route)

	if err != nil {
		return err
	}

	time.Sleep(500 * time.Millisecond)
	return h.WaitForReady(page)
}

// GetRouteChangeScript returns JavaScript to detect route changes.
func (h *ReactHandler) GetRouteChangeScript() string {
	return `
		(function() {
			// Monitor history changes
			let originalPushState = history.pushState;
			history.pushState = function() {
				originalPushState.apply(this, arguments);
				window.__routeChanged = {
					type: 'react-history',
					url: arguments[2],
					timestamp: Date.now()
				};
			};

			let originalReplaceState = history.replaceState;
			history.replaceState = function() {
				originalReplaceState.apply(this, arguments);
				window.__routeChanged = {
					type: 'react-history-replace',
					url: arguments[2],
					timestamp: Date.now()
				};
			};

			window.addEventListener('popstate', function(e) {
				window.__routeChanged = {
					type: 'react-popstate',
					url: window.location.href,
					timestamp: Date.now()
				};
			});

			window.addEventListener('hashchange', function(e) {
				window.__routeChanged = {
					type: 'react-hashchange',
					url: window.location.hash,
					timestamp: Date.now()
				};
			});
		})();
	`
}

// parseRoutes parses the JavaScript result into Route structs.
func (h *ReactHandler) parseRoutes(result *proto.RuntimeRemoteObject) ([]Route, error) {
	routes := make([]Route, 0)
	if result == nil {
		return routes, nil
	}

	arr, ok := result.Value.Val().([]interface{})
	if !ok {
		return routes, nil
	}

	for _, item := range arr {
		if m, ok := item.(map[string]interface{}); ok {
			route := Route{
				Meta: make(map[string]string),
			}

			if path, ok := m["path"].(string); ok {
				route.Path = path
			}
			if name, ok := m["name"].(string); ok {
				route.Name = name
			}
			if comp, ok := m["component"].(string); ok {
				route.Component = comp
			}
			if params, ok := m["parameters"].([]interface{}); ok {
				for _, p := range params {
					if ps, ok := p.(string); ok {
						route.Parameters = append(route.Parameters, ps)
					}
				}
			}
			if t, ok := m["type"].(string); ok {
				route.Meta["type"] = t
			}

			routes = append(routes, route)
		}
	}

	return routes, nil
}

// parseLinks parses the JavaScript result into Link structs.
func (h *ReactHandler) parseLinks(result *proto.RuntimeRemoteObject) ([]Link, error) {
	links := make([]Link, 0)
	if result == nil {
		return links, nil
	}

	arr, ok := result.Value.Val().([]interface{})
	if !ok {
		return links, nil
	}

	for _, item := range arr {
		if m, ok := item.(map[string]interface{}); ok {
			link := Link{
				Attributes: make(map[string]string),
			}

			if url, ok := m["url"].(string); ok {
				link.URL = url
			}
			if text, ok := m["text"].(string); ok {
				link.Text = text
			}
			if t, ok := m["type"].(string); ok {
				link.Type = t
			}
			if attrs, ok := m["attributes"].(map[string]interface{}); ok {
				for k, v := range attrs {
					if vs, ok := v.(string); ok {
						link.Attributes[k] = vs
					}
				}
			}

			links = append(links, link)
		}
	}

	return links, nil
}
