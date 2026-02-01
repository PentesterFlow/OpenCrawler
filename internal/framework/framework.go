// Package framework provides modular SPA framework detection and handling.
package framework

import (
	"github.com/go-rod/rod"
)

// Type represents a JavaScript framework type.
type Type string

const (
	TypeUnknown   Type = "unknown"
	TypeAngularJS Type = "angularjs" // AngularJS 1.x
	TypeAngular   Type = "angular"   // Angular 2+
	TypeReact     Type = "react"
	TypeVue       Type = "vue"
	TypeEmber     Type = "ember"
	TypeBackbone  Type = "backbone"
	TypeSvelte    Type = "svelte"
	TypeNext      Type = "nextjs"
	TypeNuxt      Type = "nuxt"
	TypeGatsby    Type = "gatsby"
)

// Route represents a discovered route.
type Route struct {
	Path       string            // Route path (e.g., "/users", "#/popular")
	Name       string            // Route name if available
	Component  string            // Component name if available
	Parameters []string          // Route parameters (e.g., ":id", ":slug")
	Meta       map[string]string // Additional metadata
}

// Link represents a discovered link.
type Link struct {
	URL        string
	Text       string
	Attributes map[string]string
	Type       string // "href", "router-link", "ng-href", etc.
}

// Handler defines the interface for framework-specific handling.
type Handler interface {
	// Type returns the framework type.
	Type() Type

	// Detect checks if this framework is present on the page.
	Detect(page *rod.Page) bool

	// WaitForReady waits for the framework to be fully loaded.
	WaitForReady(page *rod.Page) error

	// ExtractRoutes extracts all routes defined in the application.
	ExtractRoutes(page *rod.Page) ([]Route, error)

	// ExtractLinks extracts all navigation links from the page.
	ExtractLinks(page *rod.Page) ([]Link, error)

	// NavigateToRoute navigates to a specific route within the SPA.
	NavigateToRoute(page *rod.Page, route string) error

	// GetRouteChangeScript returns JavaScript to detect route changes.
	GetRouteChangeScript() string
}

// DetectionResult contains the result of framework detection.
type DetectionResult struct {
	Frameworks []Type
	Primary    Type
	Version    string
	IsSPA      bool
	HasRouter  bool
	RouterType string
}

// Detector detects frameworks on a page.
type Detector struct {
	handlers []Handler
}

// NewDetector creates a new framework detector with all handlers.
func NewDetector() *Detector {
	return &Detector{
		handlers: []Handler{
			NewAngularJSHandler(),
			NewAngularHandler(),
			NewReactHandler(),
			NewVueHandler(),
			NewEmberHandler(),
			NewGenericHandler(),
		},
	}
}

// Detect detects all frameworks present on the page.
func (d *Detector) Detect(page *rod.Page) *DetectionResult {
	result := &DetectionResult{
		Frameworks: make([]Type, 0),
		Primary:    TypeUnknown,
	}

	for _, handler := range d.handlers {
		if handler.Detect(page) {
			result.Frameworks = append(result.Frameworks, handler.Type())
			if result.Primary == TypeUnknown {
				result.Primary = handler.Type()
			}
		}
	}

	result.IsSPA = len(result.Frameworks) > 0
	return result
}

// GetHandler returns the appropriate handler for a framework type.
func (d *Detector) GetHandler(t Type) Handler {
	for _, handler := range d.handlers {
		if handler.Type() == t {
			return handler
		}
	}
	return NewGenericHandler()
}

// GetPrimaryHandler returns the handler for the primary detected framework.
func (d *Detector) GetPrimaryHandler(page *rod.Page) Handler {
	result := d.Detect(page)
	return d.GetHandler(result.Primary)
}

// ExtractAllRoutes extracts routes using all applicable handlers.
func (d *Detector) ExtractAllRoutes(page *rod.Page) []Route {
	routes := make([]Route, 0)
	seen := make(map[string]bool)

	for _, handler := range d.handlers {
		if handler.Detect(page) {
			handlerRoutes, err := handler.ExtractRoutes(page)
			if err == nil {
				for _, route := range handlerRoutes {
					if !seen[route.Path] {
						seen[route.Path] = true
						routes = append(routes, route)
					}
				}
			}
		}
	}

	return routes
}

// ExtractAllLinks extracts links using all applicable handlers.
func (d *Detector) ExtractAllLinks(page *rod.Page) []Link {
	links := make([]Link, 0)
	seen := make(map[string]bool)

	for _, handler := range d.handlers {
		if handler.Detect(page) {
			handlerLinks, err := handler.ExtractLinks(page)
			if err == nil {
				for _, link := range handlerLinks {
					if !seen[link.URL] {
						seen[link.URL] = true
						links = append(links, link)
					}
				}
			}
		}
	}

	return links
}

// WaitForFrameworks waits for all detected frameworks to be ready.
func (d *Detector) WaitForFrameworks(page *rod.Page) error {
	for _, handler := range d.handlers {
		if handler.Detect(page) {
			if err := handler.WaitForReady(page); err != nil {
				// Log but don't fail - framework might partially load
				continue
			}
		}
	}
	return nil
}
