package framework

import (
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// EmberHandler handles Ember.js applications.
type EmberHandler struct{}

// NewEmberHandler creates a new Ember handler.
func NewEmberHandler() *EmberHandler {
	return &EmberHandler{}
}

// Type returns the framework type.
func (h *EmberHandler) Type() Type {
	return TypeEmber
}

// Detect checks if Ember.js is present on the page.
func (h *EmberHandler) Detect(page *rod.Page) bool {
	result, err := page.Eval(`() => {
		return !!(
			window.Ember ||
			window.Em ||
			window.EmberENV ||
			document.querySelector('[id^="ember"]') ||
			document.querySelector('.ember-view') ||
			document.querySelector('[data-ember-action]')
		);
	}`)
	if err != nil {
		return false
	}
	return result.Value.Bool()
}

// WaitForReady waits for Ember to be fully loaded.
func (h *EmberHandler) WaitForReady(page *rod.Page) error {
	_, err := page.Eval(`() => {
		return new Promise((resolve) => {
			let timeout = setTimeout(() => resolve(false), 10000);

			function checkReady() {
				if (window.Ember || window.Em) {
					let Ember = window.Ember || window.Em;
					// Check if application is ready
					if (Ember.Application && Ember.Application.NAMESPACES) {
						let apps = Ember.Application.NAMESPACES;
						if (apps.length > 0) {
							let app = apps[0];
							if (app._booted && app._readyPromise) {
								app._readyPromise.then(() => {
									clearTimeout(timeout);
									resolve(true);
								});
								return;
							}
						}
					}
					// Check for rendered views
					let views = document.querySelectorAll('.ember-view');
					if (views.length > 0) {
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

// ExtractRoutes extracts all routes from Ember application.
func (h *EmberHandler) ExtractRoutes(page *rod.Page) ([]Route, error) {
	result, err := page.Eval(`() => {
		let routes = [];

		if (window.Ember || window.Em) {
			let Ember = window.Ember || window.Em;

			// Try to get the router
			if (Ember.Application && Ember.Application.NAMESPACES) {
				let apps = Ember.Application.NAMESPACES;
				if (apps.length > 0) {
					let app = apps[0];
					let router = app.__container__.lookup('router:main');

					if (router && router._routerMicrolib) {
						let recognizer = router._routerMicrolib.recognizer;
						if (recognizer && recognizer.names) {
							for (let name in recognizer.names) {
								let handlers = recognizer.names[name];
								if (handlers && handlers.length > 0) {
									let segments = handlers.map(h => h.handler);
									let path = '/' + segments.join('/');
									routes.push({
										path: path,
										name: name,
										component: '',
										parameters: (path.match(/:\w+/g) || []).map(p => p.substring(1)),
										type: 'ember-router'
									});
								}
							}
						}
					}
				}
			}
		}

		// Extract from script tags
		document.querySelectorAll('script').forEach(script => {
			let text = script.textContent || '';

			// Match this.route('name', { path: '/xxx' })
			let routeMatches = text.matchAll(/this\.route\s*\(\s*['"]([^'"]+)['"](?:\s*,\s*\{[^}]*path\s*:\s*['"]([^'"]+)['"])?/g);
			for (let match of routeMatches) {
				let name = match[1];
				let path = match[2] || '/' + name;
				routes.push({
					path: path,
					name: name,
					component: name,
					parameters: (path.match(/:\w+/g) || []).map(p => p.substring(1)),
					type: 'ember-route-config'
				});
			}
		});

		return routes;
	}`)

	if err != nil {
		return nil, err
	}

	return h.parseRoutes(result)
}

// ExtractLinks extracts all navigation links from the page.
func (h *EmberHandler) ExtractLinks(page *rod.Page) ([]Link, error) {
	result, err := page.Eval(`() => {
		let links = [];
		let seen = new Set();

		function addLink(url, text, type, attrs) {
			if (url && !seen.has(url)) {
				seen.add(url);
				links.push({url, text, type, attributes: attrs || {}});
			}
		}

		// LinkTo components (render as <a>)
		document.querySelectorAll('a.ember-view[href]').forEach(el => {
			let href = el.getAttribute('href');
			if (href && !href.startsWith('http') && !href.startsWith('mailto:')) {
				addLink(href, el.textContent.trim(), 'ember-link-to', {});
			}
		});

		// data-ember-action links
		document.querySelectorAll('[data-ember-action]').forEach(el => {
			let actionId = el.getAttribute('data-ember-action');
			let text = el.textContent.trim();
			// Can't extract route from action, but log for inspection
			if (text) {
				addLink('$action:' + actionId, text, 'ember-action', {actionId: actionId});
			}
		});

		// Standard links within Ember views
		document.querySelectorAll('.ember-view a[href]').forEach(el => {
			let href = el.getAttribute('href');
			if (href && !href.startsWith('http') && !href.startsWith('mailto:')) {
				addLink(href, el.textContent.trim(), 'ember-view-link', {});
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
func (h *EmberHandler) NavigateToRoute(page *rod.Page, route string) error {
	_, err := page.Eval(`(route) => {
		return new Promise((resolve) => {
			if (window.Ember || window.Em) {
				let Ember = window.Ember || window.Em;
				if (Ember.Application && Ember.Application.NAMESPACES) {
					let apps = Ember.Application.NAMESPACES;
					if (apps.length > 0) {
						let app = apps[0];
						let router = app.__container__.lookup('router:main');
						if (router) {
							router.transitionTo(route).then(() => resolve(true)).catch(() => resolve(true));
							return;
						}
					}
				}
			}
			// Fallback
			window.location.href = route;
			resolve(true);
		});
	}`, route)

	if err != nil {
		return err
	}

	time.Sleep(500 * time.Millisecond)
	return h.WaitForReady(page)
}

// GetRouteChangeScript returns JavaScript to detect route changes.
func (h *EmberHandler) GetRouteChangeScript() string {
	return `
		(function() {
			if (window.Ember || window.Em) {
				let Ember = window.Ember || window.Em;
				if (Ember.Application && Ember.Application.NAMESPACES) {
					let apps = Ember.Application.NAMESPACES;
					if (apps.length > 0) {
						let app = apps[0];
						let router = app.__container__.lookup('router:main');
						if (router) {
							router.on('routeDidChange', (transition) => {
								window.__routeChanged = {
									type: 'ember',
									to: transition.to ? transition.to.name : '',
									from: transition.from ? transition.from.name : '',
									timestamp: Date.now()
								};
							});
						}
					}
				}
			}
		})();
	`
}

// parseRoutes parses the JavaScript result into Route structs.
func (h *EmberHandler) parseRoutes(result *proto.RuntimeRemoteObject) ([]Route, error) {
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
func (h *EmberHandler) parseLinks(result *proto.RuntimeRemoteObject) ([]Link, error) {
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
