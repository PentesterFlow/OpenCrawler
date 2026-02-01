package framework

import (
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// GenericHandler handles generic SPAs and traditional pages.
type GenericHandler struct{}

// NewGenericHandler creates a new generic handler.
func NewGenericHandler() *GenericHandler {
	return &GenericHandler{}
}

// Type returns the framework type.
func (h *GenericHandler) Type() Type {
	return TypeUnknown
}

// Detect always returns true as this is the fallback handler.
func (h *GenericHandler) Detect(page *rod.Page) bool {
	return true
}

// WaitForReady waits for the page to be fully loaded.
func (h *GenericHandler) WaitForReady(page *rod.Page) error {
	_, err := page.Eval(`() => {
		return new Promise((resolve) => {
			// Check if document is ready
			if (document.readyState === 'complete') {
				// Wait a bit for any async content
				setTimeout(() => resolve(true), 500);
				return;
			}

			window.addEventListener('load', () => {
				setTimeout(() => resolve(true), 500);
			});

			// Timeout
			setTimeout(() => resolve(true), 5000);
		});
	}`)
	if err != nil {
		time.Sleep(1 * time.Second)
	}
	return nil
}

// ExtractRoutes extracts all routes from the page.
func (h *GenericHandler) ExtractRoutes(page *rod.Page) ([]Route, error) {
	result, err := page.Eval(`() => {
		let routes = [];
		let seen = new Set();

		function addRoute(path, type) {
			if (path && !seen.has(path)) {
				seen.add(path);
				routes.push({
					path: path,
					name: '',
					component: '',
					parameters: (path.match(/:\w+|\[\w+\]|\{\w+\}/g) || []).map(p => p.replace(/[:\[\]\{\}]/g, '')),
					type: type
				});
			}
		}

		// Look for hash-based routes
		document.querySelectorAll('a[href^="#"]').forEach(el => {
			let href = el.getAttribute('href');
			if (href && href.length > 1) {
				addRoute(href, 'hash-href');
			}
		});

		// Look for onclick handlers with common navigation patterns
		document.querySelectorAll('[onclick]').forEach(el => {
			let handler = el.getAttribute('onclick');

			// window.location
			let locationMatch = handler.match(/(?:window\.)?location(?:\.href)?\s*=\s*['"]([^'"]+)['"]/);
			if (locationMatch) {
				addRoute(locationMatch[1], 'onclick-location');
			}

			// location.hash
			let hashMatch = handler.match(/location\.hash\s*=\s*['"]([^'"]+)['"]/);
			if (hashMatch) {
				addRoute(hashMatch[1], 'onclick-hash');
			}

			// history.pushState
			let pushMatch = handler.match(/history\.pushState\s*\([^,]*,\s*[^,]*,\s*['"]([^'"]+)['"]/);
			if (pushMatch) {
				addRoute(pushMatch[1], 'onclick-pushstate');
			}
		});

		// Look in script tags for route patterns
		document.querySelectorAll('script:not([src])').forEach(script => {
			let text = script.textContent || '';

			// Generic route patterns
			let patterns = [
				/['"]\/[\w\-\/]+['"]/g,  // '/path/to/something'
				/#\/[\w\-\/]+/g,          // #/path
			];

			patterns.forEach(pattern => {
				let matches = text.match(pattern);
				if (matches) {
					matches.forEach(match => {
						let path = match.replace(/['"]/g, '');
						// Filter out likely non-routes
						if (!path.match(/\.(js|css|png|jpg|gif|svg|ico|json|xml)$/i) &&
							!path.includes('http') &&
							path.length < 100) {
							addRoute(path, 'script-pattern');
						}
					});
				}
			});
		});

		// Look for data attributes that might contain routes
		document.querySelectorAll('[data-href], [data-url], [data-link], [data-route], [data-path]').forEach(el => {
			let attrs = ['data-href', 'data-url', 'data-link', 'data-route', 'data-path'];
			attrs.forEach(attr => {
				let value = el.getAttribute(attr);
				if (value && !value.startsWith('http')) {
					addRoute(value, 'data-attribute');
				}
			});
		});

		return routes;
	}`)

	if err != nil {
		return nil, err
	}

	return h.parseRoutes(result)
}

// ExtractLinks extracts all navigation links from the page.
func (h *GenericHandler) ExtractLinks(page *rod.Page) ([]Link, error) {
	result, err := page.Eval(`() => {
		let links = [];
		let seen = new Set();

		function addLink(url, text, type, attrs) {
			if (url && !seen.has(url)) {
				seen.add(url);
				links.push({url, text, type, attributes: attrs || {}});
			}
		}

		// Standard href links
		document.querySelectorAll('a[href]').forEach(el => {
			let href = el.getAttribute('href');
			if (href &&
				!href.startsWith('javascript:') &&
				!href.startsWith('mailto:') &&
				!href.startsWith('tel:') &&
				!href.startsWith('data:')) {
				addLink(href, el.textContent.trim(), 'href', {
					target: el.getAttribute('target') || '',
					rel: el.getAttribute('rel') || ''
				});
			}
		});

		// area elements in image maps
		document.querySelectorAll('area[href]').forEach(el => {
			let href = el.getAttribute('href');
			if (href && !href.startsWith('javascript:')) {
				addLink(href, el.getAttribute('alt') || '', 'area', {});
			}
		});

		// Forms with GET method
		document.querySelectorAll('form[method="get"], form:not([method])').forEach(form => {
			let action = form.getAttribute('action');
			if (action && !action.startsWith('javascript:')) {
				addLink(action, '', 'form-get', {
					method: 'GET'
				});
			}
		});

		// Forms with POST method (for completeness)
		document.querySelectorAll('form[method="post"]').forEach(form => {
			let action = form.getAttribute('action');
			if (action && !action.startsWith('javascript:')) {
				addLink(action, '', 'form-post', {
					method: 'POST'
				});
			}
		});

		// iframes
		document.querySelectorAll('iframe[src]').forEach(el => {
			let src = el.getAttribute('src');
			if (src && !src.startsWith('javascript:') && !src.startsWith('data:') && !src.startsWith('about:')) {
				addLink(src, '', 'iframe', {});
			}
		});

		// Links in meta refresh
		document.querySelectorAll('meta[http-equiv="refresh"]').forEach(el => {
			let content = el.getAttribute('content');
			if (content) {
				let urlMatch = content.match(/url=([^;]+)/i);
				if (urlMatch) {
					addLink(urlMatch[1].trim(), '', 'meta-refresh', {});
				}
			}
		});

		// base href
		let base = document.querySelector('base[href]');
		if (base) {
			addLink(base.getAttribute('href'), '', 'base-href', {});
		}

		// Links in comments (useful for finding hidden/debug routes)
		let walker = document.createTreeWalker(document.body, NodeFilter.SHOW_COMMENT);
		while (walker.nextNode()) {
			let comment = walker.currentNode.textContent;
			let urlMatches = comment.match(/https?:\/\/[^\s<>"]+|\/[\w\-\.\/]+/g);
			if (urlMatches) {
				urlMatches.forEach(url => {
					if (!url.match(/\.(js|css|png|jpg|gif|svg)$/i)) {
						addLink(url, '', 'comment', {});
					}
				});
			}
		}

		return links;
	}`)

	if err != nil {
		return nil, err
	}

	return h.parseLinks(result)
}

// NavigateToRoute navigates to a specific route.
func (h *GenericHandler) NavigateToRoute(page *rod.Page, route string) error {
	_, err := page.Eval(`(route) => {
		if (route.startsWith('#')) {
			window.location.hash = route;
		} else if (route.startsWith('/')) {
			window.history.pushState({}, '', route);
			window.dispatchEvent(new PopStateEvent('popstate'));
		} else {
			window.location.href = route;
		}
	}`, route)

	if err != nil {
		return err
	}

	time.Sleep(500 * time.Millisecond)
	return nil
}

// GetRouteChangeScript returns JavaScript to detect route changes.
func (h *GenericHandler) GetRouteChangeScript() string {
	return `
		(function() {
			// Monitor hash changes
			window.addEventListener('hashchange', (e) => {
				window.__routeChanged = {
					type: 'hashchange',
					oldURL: e.oldURL,
					newURL: e.newURL,
					timestamp: Date.now()
				};
			});

			// Monitor history changes
			let originalPushState = history.pushState;
			history.pushState = function() {
				originalPushState.apply(this, arguments);
				window.__routeChanged = {
					type: 'pushState',
					url: arguments[2],
					timestamp: Date.now()
				};
			};

			let originalReplaceState = history.replaceState;
			history.replaceState = function() {
				originalReplaceState.apply(this, arguments);
				window.__routeChanged = {
					type: 'replaceState',
					url: arguments[2],
					timestamp: Date.now()
				};
			};

			window.addEventListener('popstate', (e) => {
				window.__routeChanged = {
					type: 'popstate',
					url: window.location.href,
					timestamp: Date.now()
				};
			});
		})();
	`
}

// parseRoutes parses the JavaScript result into Route structs.
func (h *GenericHandler) parseRoutes(result *proto.RuntimeRemoteObject) ([]Route, error) {
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
func (h *GenericHandler) parseLinks(result *proto.RuntimeRemoteObject) ([]Link, error) {
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
