package framework

import (
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// VueHandler handles Vue.js applications.
type VueHandler struct{}

// NewVueHandler creates a new Vue handler.
func NewVueHandler() *VueHandler {
	return &VueHandler{}
}

// Type returns the framework type.
func (h *VueHandler) Type() Type {
	return TypeVue
}

// Detect checks if Vue.js is present on the page.
func (h *VueHandler) Detect(page *rod.Page) bool {
	result, err := page.Eval(`() => {
		return !!(
			window.Vue ||
			window.__VUE__ ||
			window.__VUE_DEVTOOLS_GLOBAL_HOOK__ ||
			document.querySelector('[data-v-]') ||
			document.querySelector('[data-v-app]') ||
			document.querySelector('#app')?.__vue__ ||
			document.querySelector('#app')?.__vue_app__
		);
	}`)
	if err != nil {
		return false
	}
	return result.Value.Bool()
}

// WaitForReady waits for Vue to be fully loaded.
func (h *VueHandler) WaitForReady(page *rod.Page) error {
	_, err := page.Eval(`() => {
		return new Promise((resolve) => {
			let timeout = setTimeout(() => resolve(false), 10000);

			function checkReady() {
				let app = document.querySelector('#app, [data-v-app]');
				if (app) {
					// Vue 2
					if (app.__vue__ && !app.__vue__.$data._isLoading) {
						clearTimeout(timeout);
						resolve(true);
						return;
					}
					// Vue 3
					if (app.__vue_app__) {
						clearTimeout(timeout);
						resolve(true);
						return;
					}
					// Check if content is rendered
					if (app.innerHTML.trim() !== '' && app.children.length > 0) {
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

// ExtractRoutes extracts all routes from Vue application.
func (h *VueHandler) ExtractRoutes(page *rod.Page) ([]Route, error) {
	result, err := page.Eval(`() => {
		let routes = [];

		// Try to get Vue Router instance
		let app = document.querySelector('#app, [data-v-app]');
		if (app) {
			// Vue 2 with Vue Router
			if (app.__vue__ && app.__vue__.$router) {
				let router = app.__vue__.$router;
				if (router.options && router.options.routes) {
					function extractRoutes(routeList, prefix = '') {
						routeList.forEach(route => {
							let path = route.path;
							if (!path.startsWith('/') && prefix) {
								path = prefix + '/' + path;
							}
							routes.push({
								path: path,
								name: route.name || '',
								component: route.component ? (route.component.name || 'Anonymous') : '',
								parameters: (path.match(/:\w+/g) || []).map(p => p.substring(1)),
								type: 'vue-router-2',
								meta: route.meta || {}
							});
							if (route.children) {
								extractRoutes(route.children, path);
							}
						});
					}
					extractRoutes(router.options.routes);
				}
			}

			// Vue 3 with Vue Router
			if (app.__vue_app__) {
				let router = app.__vue_app__.config.globalProperties.$router;
				if (router && router.options && router.options.routes) {
					function extractRoutes(routeList, prefix = '') {
						routeList.forEach(route => {
							let path = route.path;
							if (!path.startsWith('/') && prefix) {
								path = prefix + '/' + path;
							}
							routes.push({
								path: path,
								name: route.name || '',
								component: route.component ? (route.component.name || route.component.__name || 'Anonymous') : '',
								parameters: (path.match(/:\w+/g) || []).map(p => p.substring(1)),
								type: 'vue-router-3',
								meta: route.meta || {}
							});
							if (route.children) {
								extractRoutes(route.children, path);
							}
						});
					}
					extractRoutes(router.options.routes);
				}
			}
		}

		// Extract from script tags
		document.querySelectorAll('script').forEach(script => {
			let text = script.textContent || '';

			// Match { path: '/xxx', component: YYY }
			let routeMatches = text.matchAll(/\{\s*path\s*:\s*['"]([^'"]+)['"][^}]*component\s*:/g);
			for (let match of routeMatches) {
				let path = match[1];
				if (!routes.some(r => r.path === path)) {
					routes.push({
						path: path,
						name: '',
						component: '',
						parameters: (path.match(/:\w+/g) || []).map(p => p.substring(1)),
						type: 'vue-route-config'
					});
				}
			}

			// Match name: 'routeName', path: '/xxx'
			let namedMatches = text.matchAll(/name\s*:\s*['"]([^'"]+)['"][^}]*path\s*:\s*['"]([^'"]+)['"]/g);
			for (let match of namedMatches) {
				let path = match[2];
				if (!routes.some(r => r.path === path)) {
					routes.push({
						path: path,
						name: match[1],
						component: '',
						parameters: (path.match(/:\w+/g) || []).map(p => p.substring(1)),
						type: 'vue-named-route'
					});
				}
			}
		});

		// Nuxt.js: check for __NUXT__
		if (window.__NUXT__) {
			try {
				if (window.__NUXT__.routePath) {
					routes.push({
						path: window.__NUXT__.routePath,
						name: 'nuxt-current',
						component: '',
						parameters: [],
						type: 'nuxt-page'
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
func (h *VueHandler) ExtractLinks(page *rod.Page) ([]Link, error) {
	result, err := page.Eval(`() => {
		let links = [];
		let seen = new Set();

		function addLink(url, text, type, attrs) {
			if (url && !seen.has(url)) {
				seen.add(url);
				links.push({url, text, type, attributes: attrs || {}});
			}
		}

		// router-link components (render as <a>)
		document.querySelectorAll('a[href]').forEach(el => {
			let href = el.getAttribute('href');
			// Check if it's a Vue router link
			let isRouterLink = el.classList.contains('router-link-active') ||
							   el.classList.contains('router-link-exact-active') ||
							   el.hasAttribute('data-v-');
			if (href && !href.startsWith('http') && !href.startsWith('mailto:')) {
				addLink(href, el.textContent.trim(), isRouterLink ? 'router-link' : 'href', {});
			}
		});

		// v-bind:to or :to attributes (before Vue processes them)
		document.querySelectorAll('[to], [\\:to], [v-bind\\:to]').forEach(el => {
			let to = el.getAttribute('to') || el.getAttribute(':to') || el.getAttribute('v-bind:to');
			if (to) {
				// Handle object syntax { name: 'route' } or { path: '/xxx' }
				if (to.startsWith('{')) {
					let pathMatch = to.match(/path\s*:\s*['"]([^'"]+)['"]/);
					let nameMatch = to.match(/name\s*:\s*['"]([^'"]+)['"]/);
					if (pathMatch) {
						addLink(pathMatch[1], el.textContent.trim(), 'router-link-to-path', {});
					}
					if (nameMatch) {
						addLink('$route:' + nameMatch[1], el.textContent.trim(), 'router-link-to-name', {name: nameMatch[1]});
					}
				} else if (!to.includes('{')) {
					addLink(to.replace(/['"]/g, ''), el.textContent.trim(), 'router-link-to', {});
				}
			}
		});

		// nuxt-link components
		document.querySelectorAll('nuxt-link, [nuxt-link], NuxtLink').forEach(el => {
			let to = el.getAttribute('to') || el.getAttribute(':to');
			if (to) {
				addLink(to.replace(/['"]/g, ''), el.textContent.trim(), 'nuxt-link', {});
			}
		});

		// @click with $router.push
		document.querySelectorAll('[v-on\\:click], [@click]').forEach(el => {
			let handler = el.getAttribute('v-on:click') || el.getAttribute('@click') || '';

			// Look for $router.push('/xxx') or router.push({path: '/xxx'})
			let pushMatch = handler.match(/\$?router\.push\s*\(\s*['"]([^'"]+)['"]/);
			if (pushMatch) {
				addLink(pushMatch[1], el.textContent.trim(), 'click-router-push', {});
			}

			// Look for $router.push({path: '/xxx'})
			let pushObjMatch = handler.match(/\$?router\.push\s*\(\s*\{[^}]*path\s*:\s*['"]([^'"]+)['"]/);
			if (pushObjMatch) {
				addLink(pushObjMatch[1], el.textContent.trim(), 'click-router-push-obj', {});
			}

			// Look for navigateTo (Nuxt 3)
			let navigateMatch = handler.match(/navigateTo\s*\(\s*['"]([^'"]+)['"]/);
			if (navigateMatch) {
				addLink(navigateMatch[1], el.textContent.trim(), 'nuxt-navigateTo', {});
			}
		});

		// Hash links
		document.querySelectorAll('a[href^="#/"]').forEach(el => {
			addLink(el.getAttribute('href'), el.textContent.trim(), 'hash-link', {});
		});

		return links;
	}`)

	if err != nil {
		return nil, err
	}

	return h.parseLinks(result)
}

// NavigateToRoute navigates to a specific route within the SPA.
func (h *VueHandler) NavigateToRoute(page *rod.Page, route string) error {
	_, err := page.Eval(`(route) => {
		return new Promise((resolve) => {
			let app = document.querySelector('#app, [data-v-app]');

			// Vue 2
			if (app && app.__vue__ && app.__vue__.$router) {
				app.__vue__.$router.push(route).then(() => resolve(true)).catch(() => resolve(true));
				return;
			}

			// Vue 3
			if (app && app.__vue_app__) {
				let router = app.__vue_app__.config.globalProperties.$router;
				if (router) {
					router.push(route).then(() => resolve(true)).catch(() => resolve(true));
					return;
				}
			}

			// Fallback: use history API or hash
			if (route.startsWith('#')) {
				window.location.hash = route;
			} else {
				window.history.pushState({}, '', route);
				window.dispatchEvent(new PopStateEvent('popstate'));
			}
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
func (h *VueHandler) GetRouteChangeScript() string {
	return `
		(function() {
			let app = document.querySelector('#app, [data-v-app]');

			// Vue 2
			if (app && app.__vue__ && app.__vue__.$router) {
				app.__vue__.$router.afterEach((to, from) => {
					window.__routeChanged = {
						type: 'vue-router-2',
						to: to.path,
						from: from.path,
						name: to.name,
						timestamp: Date.now()
					};
				});
			}

			// Vue 3
			if (app && app.__vue_app__) {
				let router = app.__vue_app__.config.globalProperties.$router;
				if (router) {
					router.afterEach((to, from) => {
						window.__routeChanged = {
							type: 'vue-router-3',
							to: to.path,
							from: from.path,
							name: to.name,
							timestamp: Date.now()
						};
					});
				}
			}

			// Fallback: monitor hash changes
			window.addEventListener('hashchange', (e) => {
				window.__routeChanged = {
					type: 'vue-hash',
					url: window.location.hash,
					timestamp: Date.now()
				};
			});
		})();
	`
}

// parseRoutes parses the JavaScript result into Route structs.
func (h *VueHandler) parseRoutes(result *proto.RuntimeRemoteObject) ([]Route, error) {
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
			if meta, ok := m["meta"].(map[string]interface{}); ok {
				for k, v := range meta {
					if vs, ok := v.(string); ok {
						route.Meta[k] = vs
					}
				}
			}

			routes = append(routes, route)
		}
	}

	return routes, nil
}

// parseLinks parses the JavaScript result into Link structs.
func (h *VueHandler) parseLinks(result *proto.RuntimeRemoteObject) ([]Link, error) {
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
