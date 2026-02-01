package framework

import (
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// AngularJSHandler handles AngularJS 1.x applications.
type AngularJSHandler struct{}

// NewAngularJSHandler creates a new AngularJS handler.
func NewAngularJSHandler() *AngularJSHandler {
	return &AngularJSHandler{}
}

// Type returns the framework type.
func (h *AngularJSHandler) Type() Type {
	return TypeAngularJS
}

// Detect checks if AngularJS 1.x is present on the page.
func (h *AngularJSHandler) Detect(page *rod.Page) bool {
	result, err := page.Eval(`() => {
		return !!(window.angular && window.angular.version && window.angular.version.major === 1);
	}`)
	if err != nil {
		return false
	}
	return result.Value.Bool()
}

// WaitForReady waits for AngularJS to be fully loaded.
func (h *AngularJSHandler) WaitForReady(page *rod.Page) error {
	_, err := page.Eval(`() => {
		return new Promise((resolve, reject) => {
			let timeout = setTimeout(() => resolve(false), 10000);

			function checkReady() {
				if (window.angular) {
					let el = document.querySelector('[ng-app], [data-ng-app], [ng\\:app], [x-ng-app]');
					if (el) {
						let injector = angular.element(el).injector();
						if (injector) {
							try {
								let $rootScope = injector.get('$rootScope');
								let $http = injector.get('$http');

								// Wait for pending HTTP requests
								if ($http.pendingRequests && $http.pendingRequests.length > 0) {
									setTimeout(checkReady, 100);
									return;
								}

								// Wait for digest cycle
								if ($rootScope.$$phase) {
									setTimeout(checkReady, 100);
									return;
								}

								clearTimeout(timeout);
								resolve(true);
								return;
							} catch(e) {}
						}
					}
				}
				setTimeout(checkReady, 100);
			}

			checkReady();
		});
	}`)
	if err != nil {
		// Fallback: just wait a bit
		time.Sleep(2 * time.Second)
	}
	return nil
}

// ExtractRoutes extracts all routes from AngularJS application.
func (h *AngularJSHandler) ExtractRoutes(page *rod.Page) ([]Route, error) {
	result, err := page.Eval(`() => {
		let routes = [];

		if (!window.angular) return routes;

		let el = document.querySelector('[ng-app], [data-ng-app], [ng\\:app], [x-ng-app]');
		if (!el) return routes;

		let injector = angular.element(el).injector();
		if (!injector) return routes;

		// Extract from ngRoute ($routeProvider)
		try {
			let $route = injector.get('$route');
			if ($route && $route.routes) {
				for (let path in $route.routes) {
					if (path && path !== 'null') {
						let config = $route.routes[path];
						routes.push({
							path: path,
							name: config.name || '',
							component: config.controller || config.template || '',
							parameters: (path.match(/:\w+/g) || []).map(p => p.substring(1)),
							type: 'ngRoute'
						});
					}
				}
			}
		} catch(e) {}

		// Extract from UI Router ($stateProvider)
		try {
			let $state = injector.get('$state');
			if ($state) {
				let states = $state.get();
				states.forEach(state => {
					if (state.name && !state.abstract) {
						routes.push({
							path: state.url || ('/' + state.name.replace(/\./g, '/')),
							name: state.name,
							component: state.controller || state.component || '',
							parameters: state.url ? (state.url.match(/:\w+/g) || []).map(p => p.substring(1)) : [],
							type: 'uiRouter'
						});
					}
				});
			}
		} catch(e) {}

		// Extract from script tags (route configuration)
		document.querySelectorAll('script').forEach(script => {
			let text = script.textContent || '';

			// Match .when('/path', ...)
			let whenMatches = text.matchAll(/\.when\s*\(\s*['"]([^'"]+)['"]\s*,\s*\{([^}]*)\}/g);
			for (let match of whenMatches) {
				let path = match[1];
				let config = match[2];
				let controllerMatch = config.match(/controller\s*:\s*['"]([^'"]+)['"]/);
				routes.push({
					path: path,
					name: '',
					component: controllerMatch ? controllerMatch[1] : '',
					parameters: (path.match(/:\w+/g) || []).map(p => p.substring(1)),
					type: 'ngRoute-config'
				});
			}

			// Match .state('name', {url: '/path'})
			let stateMatches = text.matchAll(/\.state\s*\(\s*['"]([^'"]+)['"]\s*,\s*\{([^}]*(?:\{[^}]*\}[^}]*)*)\}/g);
			for (let match of stateMatches) {
				let name = match[1];
				let config = match[2];
				let urlMatch = config.match(/url\s*:\s*['"]([^'"]+)['"]/);
				if (urlMatch) {
					routes.push({
						path: urlMatch[1],
						name: name,
						component: '',
						parameters: (urlMatch[1].match(/:\w+/g) || []).map(p => p.substring(1)),
						type: 'uiRouter-config'
					});
				}
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
func (h *AngularJSHandler) ExtractLinks(page *rod.Page) ([]Link, error) {
	result, err := page.Eval(`() => {
		let links = [];
		let seen = new Set();

		function addLink(url, text, type, attrs) {
			if (url && !seen.has(url)) {
				seen.add(url);
				links.push({url, text, type, attributes: attrs || {}});
			}
		}

		// Standard href links with hash
		document.querySelectorAll('a[href^="#"]').forEach(el => {
			let href = el.getAttribute('href');
			if (href && href.length > 1 && (href[1] === '/' || href[1] === '!')) {
				addLink(href, el.textContent.trim(), 'href', {});
			}
		});

		// ng-href links
		document.querySelectorAll('[ng-href]').forEach(el => {
			let href = el.getAttribute('ng-href');
			if (href && !href.includes('{{')) {
				addLink(href, el.textContent.trim(), 'ng-href', {});
			}
		});

		// ui-sref links (UI Router)
		document.querySelectorAll('[ui-sref]').forEach(el => {
			let sref = el.getAttribute('ui-sref');
			if (sref) {
				// Parse state name from ui-sref
				let stateName = sref.split('(')[0].trim();
				addLink('#/' + stateName.replace(/\./g, '/'), el.textContent.trim(), 'ui-sref', {state: stateName});
			}
		});

		// ng-click with $location or $state
		document.querySelectorAll('[ng-click]').forEach(el => {
			let handler = el.getAttribute('ng-click');

			// Look for $location.path('/xxx') or $location.url('/xxx')
			let locationMatch = handler.match(/\$location\.(?:path|url)\s*\(\s*['"]([^'"]+)['"]\s*\)/);
			if (locationMatch) {
				addLink('#' + locationMatch[1], el.textContent.trim(), 'ng-click-location', {});
			}

			// Look for $state.go('stateName')
			let stateMatch = handler.match(/\$state\.go\s*\(\s*['"]([^'"]+)['"]/);
			if (stateMatch) {
				addLink('#/' + stateMatch[1].replace(/\./g, '/'), el.textContent.trim(), 'ng-click-state', {state: stateMatch[1]});
			}

			// Look for go('/path') or navigate('/path') functions
			let goMatch = handler.match(/(?:go|navigate|goTo|navigateTo)\s*\(\s*['"]([^'"]+)['"]\s*\)/);
			if (goMatch) {
				addLink('#' + goMatch[1], el.textContent.trim(), 'ng-click-go', {});
			}
		});

		// Look for links in ng-repeat items
		document.querySelectorAll('[ng-repeat] a, [data-ng-repeat] a').forEach(el => {
			let href = el.getAttribute('href') || el.getAttribute('ng-href');
			if (href && !href.includes('{{')) {
				addLink(href, el.textContent.trim(), 'ng-repeat-link', {});
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
func (h *AngularJSHandler) NavigateToRoute(page *rod.Page, route string) error {
	_, err := page.Eval(`(route) => {
		return new Promise((resolve, reject) => {
			if (!window.angular) {
				reject('Angular not found');
				return;
			}

			let el = document.querySelector('[ng-app], [data-ng-app]');
			if (!el) {
				reject('ng-app element not found');
				return;
			}

			let injector = angular.element(el).injector();
			if (!injector) {
				reject('Injector not found');
				return;
			}

			try {
				// Try UI Router first
				let $state = injector.get('$state');
				if ($state) {
					// Check if route is a state name or URL
					if (route.startsWith('#/') || route.startsWith('/')) {
						let $location = injector.get('$location');
						let $rootScope = injector.get('$rootScope');
						$rootScope.$apply(() => {
							$location.path(route.replace(/^#/, ''));
						});
					} else {
						$state.go(route);
					}
					setTimeout(() => resolve(true), 500);
					return;
				}
			} catch(e) {}

			try {
				// Try ngRoute
				let $location = injector.get('$location');
				let $rootScope = injector.get('$rootScope');
				$rootScope.$apply(() => {
					$location.path(route.replace(/^#/, ''));
				});
				setTimeout(() => resolve(true), 500);
				return;
			} catch(e) {}

			// Fallback: change hash directly
			window.location.hash = route.startsWith('#') ? route : '#' + route;
			setTimeout(() => resolve(true), 500);
		});
	}`, route)

	if err != nil {
		return err
	}

	// Wait for route change to complete
	time.Sleep(500 * time.Millisecond)
	return h.WaitForReady(page)
}

// GetRouteChangeScript returns JavaScript to detect route changes.
func (h *AngularJSHandler) GetRouteChangeScript() string {
	return `
		(function() {
			if (window.angular) {
				let el = document.querySelector('[ng-app], [data-ng-app]');
				if (el) {
					let $rootScope = angular.element(el).injector().get('$rootScope');
					$rootScope.$on('$routeChangeSuccess', function(event, current, previous) {
						window.__routeChanged = {
							type: 'ngRoute',
							current: current ? current.$$route : null,
							previous: previous ? previous.$$route : null,
							timestamp: Date.now()
						};
					});
					$rootScope.$on('$stateChangeSuccess', function(event, toState, toParams, fromState, fromParams) {
						window.__routeChanged = {
							type: 'uiRouter',
							current: toState,
							previous: fromState,
							timestamp: Date.now()
						};
					});
				}
			}
		})();
	`
}

// parseRoutes parses the JavaScript result into Route structs.
func (h *AngularJSHandler) parseRoutes(result *proto.RuntimeRemoteObject) ([]Route, error) {
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
func (h *AngularJSHandler) parseLinks(result *proto.RuntimeRemoteObject) ([]Link, error) {
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
