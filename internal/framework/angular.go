package framework

import (
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// AngularHandler handles Angular 2+ applications.
type AngularHandler struct{}

// NewAngularHandler creates a new Angular handler.
func NewAngularHandler() *AngularHandler {
	return &AngularHandler{}
}

// Type returns the framework type.
func (h *AngularHandler) Type() Type {
	return TypeAngular
}

// Detect checks if Angular 2+ is present on the page.
func (h *AngularHandler) Detect(page *rod.Page) bool {
	result, err := page.Eval(`() => {
		// Check for Angular 2+ indicators
		return !!(
			window.ng ||
			window.getAllAngularRootElements ||
			document.querySelector('[ng-version]') ||
			document.querySelector('app-root') ||
			document.querySelector('[_nghost-ng-c]') ||
			(window.angular && window.angular.version && window.angular.version.major >= 2)
		);
	}`)
	if err != nil {
		return false
	}
	return result.Value.Bool()
}

// WaitForReady waits for Angular to be fully loaded.
func (h *AngularHandler) WaitForReady(page *rod.Page) error {
	_, err := page.Eval(`() => {
		return new Promise((resolve) => {
			let timeout = setTimeout(() => resolve(false), 10000);

			function checkReady() {
				// Check if Angular is stable
				if (window.getAllAngularRootElements) {
					let roots = window.getAllAngularRootElements();
					if (roots && roots.length > 0) {
						// Try to check if Angular is stable
						try {
							let testability = window.getAllAngularTestabilities();
							if (testability && testability.length > 0) {
								testability[0].whenStable(() => {
									clearTimeout(timeout);
									resolve(true);
								});
								return;
							}
						} catch(e) {}
						clearTimeout(timeout);
						resolve(true);
						return;
					}
				}

				// Check for app-root
				let appRoot = document.querySelector('app-root, [ng-version]');
				if (appRoot && appRoot.innerHTML.trim() !== '') {
					clearTimeout(timeout);
					resolve(true);
					return;
				}

				setTimeout(checkReady, 100);
			}

			// Wait a bit for initial bootstrap
			setTimeout(checkReady, 500);
		});
	}`)
	if err != nil {
		time.Sleep(2 * time.Second)
	}
	return nil
}

// ExtractRoutes extracts all routes from Angular application.
func (h *AngularHandler) ExtractRoutes(page *rod.Page) ([]Route, error) {
	result, err := page.Eval(`() => {
		let routes = [];

		// Try to get routes from Angular Router
		if (window.ng) {
			try {
				// Get the router instance
				let roots = window.getAllAngularRootElements();
				if (roots && roots.length > 0) {
					let injector = window.ng.getInjector(roots[0]);
					if (injector) {
						let router = injector.get(window.ng.coreTokens.Router);
						if (router && router.config) {
							function extractRoutes(config, prefix = '') {
								config.forEach(route => {
									let path = prefix + (route.path || '');
									if (path || route.path === '') {
										routes.push({
											path: '/' + path,
											name: route.component ? route.component.name : '',
											component: route.component ? route.component.name : '',
											parameters: (path.match(/:\w+/g) || []).map(p => p.substring(1)),
											type: 'angular-router',
											hasChildren: !!route.children
										});
									}
									if (route.children) {
										extractRoutes(route.children, path + '/');
									}
									if (route.loadChildren) {
										routes.push({
											path: '/' + path,
											name: 'lazy-' + path,
											component: 'LazyLoaded',
											parameters: [],
											type: 'angular-lazy',
											hasChildren: true
										});
									}
								});
							}
							extractRoutes(router.config);
						}
					}
				}
			} catch(e) {}
		}

		// Extract from script tags
		document.querySelectorAll('script').forEach(script => {
			let text = script.textContent || '';

			// Match { path: 'xxx', component: YYY }
			let routeMatches = text.matchAll(/\{\s*path\s*:\s*['"]([^'"]*)['"]\s*,\s*component\s*:\s*(\w+)/g);
			for (let match of routeMatches) {
				routes.push({
					path: '/' + match[1],
					name: match[2],
					component: match[2],
					parameters: (match[1].match(/:\w+/g) || []).map(p => p.substring(1)),
					type: 'angular-config'
				});
			}

			// Match { path: 'xxx', loadChildren: 'yyy' }
			let lazyMatches = text.matchAll(/\{\s*path\s*:\s*['"]([^'"]*)['"]\s*,\s*loadChildren\s*:/g);
			for (let match of lazyMatches) {
				routes.push({
					path: '/' + match[1],
					name: 'lazy-' + match[1],
					component: 'LazyLoaded',
					parameters: [],
					type: 'angular-lazy-config'
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
func (h *AngularHandler) ExtractLinks(page *rod.Page) ([]Link, error) {
	result, err := page.Eval(`() => {
		let links = [];
		let seen = new Set();

		function addLink(url, text, type, attrs) {
			if (url && !seen.has(url)) {
				seen.add(url);
				links.push({url, text, type, attributes: attrs || {}});
			}
		}

		// routerLink directives
		document.querySelectorAll('[routerLink], [routerlink]').forEach(el => {
			let routerLink = el.getAttribute('routerLink') || el.getAttribute('routerlink');
			if (routerLink) {
				// Handle array format ['/', 'users', userId]
				routerLink = routerLink.replace(/^\[['"]/, '').replace(/['"]\]$/, '');
				routerLink = routerLink.replace(/['"]/g, '').replace(/,\s*/g, '/');
				if (!routerLink.startsWith('/')) {
					routerLink = '/' + routerLink;
				}
				addLink(routerLink, el.textContent.trim(), 'routerLink', {});
			}
		});

		// href links
		document.querySelectorAll('a[href]').forEach(el => {
			let href = el.getAttribute('href');
			if (href && !href.startsWith('http') && !href.startsWith('mailto:') && !href.startsWith('tel:')) {
				addLink(href, el.textContent.trim(), 'href', {});
			}
		});

		// Click handlers with router.navigate
		document.querySelectorAll('[click], (click)').forEach(el => {
			let handler = el.getAttribute('click') || el.getAttribute('(click)');
			if (handler) {
				// Look for router.navigate(['/path'])
				let navMatch = handler.match(/router\.navigate\s*\(\s*\[['"]([^'"]+)['"]/);
				if (navMatch) {
					addLink(navMatch[1], el.textContent.trim(), 'click-navigate', {});
				}
				// Look for router.navigateByUrl('/path')
				let urlMatch = handler.match(/router\.navigateByUrl\s*\(\s*['"]([^'"]+)['"]/);
				if (urlMatch) {
					addLink(urlMatch[1], el.textContent.trim(), 'click-navigateByUrl', {});
				}
			}
		});

		// Look in mat-list-item, mat-nav-list (Angular Material)
		document.querySelectorAll('mat-list-item[routerLink], mat-nav-list a[routerLink]').forEach(el => {
			let routerLink = el.getAttribute('routerLink');
			if (routerLink) {
				addLink(routerLink, el.textContent.trim(), 'mat-routerLink', {});
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
func (h *AngularHandler) NavigateToRoute(page *rod.Page, route string) error {
	_, err := page.Eval(`(route) => {
		return new Promise((resolve, reject) => {
			try {
				if (window.ng && window.getAllAngularRootElements) {
					let roots = window.getAllAngularRootElements();
					if (roots && roots.length > 0) {
						let injector = window.ng.getInjector(roots[0]);
						if (injector) {
							let router = injector.get(window.ng.coreTokens.Router);
							if (router) {
								router.navigateByUrl(route).then(() => {
									resolve(true);
								}).catch(err => {
									// Fallback to direct navigation
									window.location.href = route;
									resolve(true);
								});
								return;
							}
						}
					}
				}
				// Fallback
				window.location.href = route;
				resolve(true);
			} catch(e) {
				window.location.href = route;
				resolve(true);
			}
		});
	}`, route)

	if err != nil {
		return err
	}

	time.Sleep(500 * time.Millisecond)
	return h.WaitForReady(page)
}

// GetRouteChangeScript returns JavaScript to detect route changes.
func (h *AngularHandler) GetRouteChangeScript() string {
	return `
		(function() {
			if (window.ng && window.getAllAngularRootElements) {
				try {
					let roots = window.getAllAngularRootElements();
					if (roots && roots.length > 0) {
						let injector = window.ng.getInjector(roots[0]);
						if (injector) {
							let router = injector.get(window.ng.coreTokens.Router);
							if (router && router.events) {
								router.events.subscribe(event => {
									if (event.constructor.name === 'NavigationEnd') {
										window.__routeChanged = {
											type: 'angular',
											url: event.url,
											urlAfterRedirects: event.urlAfterRedirects,
											timestamp: Date.now()
										};
									}
								});
							}
						}
					}
				} catch(e) {}
			}
		})();
	`
}

// parseRoutes parses the JavaScript result into Route structs.
func (h *AngularHandler) parseRoutes(result *proto.RuntimeRemoteObject) ([]Route, error) {
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
			if hasChildren, ok := m["hasChildren"].(bool); ok && hasChildren {
				route.Meta["hasChildren"] = "true"
			}

			routes = append(routes, route)
		}
	}

	return routes, nil
}

// parseLinks parses the JavaScript result into Link structs.
func (h *AngularHandler) parseLinks(result *proto.RuntimeRemoteObject) ([]Link, error) {
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
