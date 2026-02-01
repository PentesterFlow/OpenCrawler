// Package discovery provides API discovery mechanisms.
package discovery

import (
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PentesterFlow/OpenCrawler/internal/browser"
)

// PassiveDiscovery captures API endpoints from network traffic.
type PassiveDiscovery struct {
	mu        sync.RWMutex
	endpoints map[string]*DiscoveredEndpoint
	enabled   bool
}

// DiscoveredEndpoint represents a passively discovered endpoint.
type DiscoveredEndpoint struct {
	URL            string
	Method         string
	Parameters     []Parameter
	Headers        map[string]string
	ContentType    string
	StatusCode     int
	DiscoveredAt   time.Time
	DiscoveredFrom string
	HitCount       int
}

// NewPassiveDiscovery creates a new passive discovery instance.
func NewPassiveDiscovery() *PassiveDiscovery {
	return &PassiveDiscovery{
		endpoints: make(map[string]*DiscoveredEndpoint),
		enabled:   true,
	}
}

// ProcessRequests processes intercepted network requests.
func (p *PassiveDiscovery) ProcessRequests(requests []browser.NetworkRequest, sourceURL string) []Endpoint {
	if !p.enabled {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	discovered := make([]Endpoint, 0)

	for _, req := range requests {
		// Skip non-API requests
		if !p.isAPIRequest(req) {
			continue
		}

		// Create endpoint key
		key := p.createEndpointKey(req.URL, req.Method)

		if existing, ok := p.endpoints[key]; ok {
			existing.HitCount++
			continue
		}

		endpoint := &DiscoveredEndpoint{
			URL:            req.URL,
			Method:         req.Method,
			Headers:        req.Headers,
			DiscoveredAt:   time.Now(),
			DiscoveredFrom: sourceURL,
			HitCount:       1,
		}

		// Extract parameters
		endpoint.Parameters = p.extractParameters(req)

		p.endpoints[key] = endpoint

		// Convert to Endpoint
		discovered = append(discovered, Endpoint{
			URL:            endpoint.URL,
			Method:         endpoint.Method,
			Source:         "passive",
			Parameters:     endpoint.Parameters,
			Headers:        endpoint.Headers,
			DiscoveredFrom: sourceURL,
			Timestamp:      endpoint.DiscoveredAt,
		})
	}

	return discovered
}

// isAPIRequest determines if a request is an API call.
func (p *PassiveDiscovery) isAPIRequest(req browser.NetworkRequest) bool {
	// Check resource type
	if req.ResourceType == "XHR" || req.ResourceType == "Fetch" {
		return true
	}

	// Check URL patterns
	parsedURL, err := url.Parse(req.URL)
	if err != nil {
		return false
	}

	path := strings.ToLower(parsedURL.Path)

	// Common API patterns
	apiPatterns := []string{
		"/api/",
		"/v1/",
		"/v2/",
		"/v3/",
		"/graphql",
		"/rest/",
		"/json/",
		"/ajax/",
		"/_api/",
		"/rpc/",
	}

	for _, pattern := range apiPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	// Check content type
	if ct := req.Headers["Content-Type"]; ct != "" {
		if strings.Contains(ct, "application/json") ||
			strings.Contains(ct, "application/xml") {
			return true
		}
	}

	return false
}

// createEndpointKey creates a unique key for an endpoint.
func (p *PassiveDiscovery) createEndpointKey(rawURL, method string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return method + " " + rawURL
	}

	// Normalize URL (remove query params for key)
	normalized := parsedURL.Scheme + "://" + parsedURL.Host + parsedURL.Path
	return method + " " + normalized
}

// extractParameters extracts parameters from a request.
func (p *PassiveDiscovery) extractParameters(req browser.NetworkRequest) []Parameter {
	params := make([]Parameter, 0)

	parsedURL, err := url.Parse(req.URL)
	if err != nil {
		return params
	}

	// Query parameters
	for name, values := range parsedURL.Query() {
		example := ""
		if len(values) > 0 {
			example = values[0]
		}
		params = append(params, Parameter{
			Name:    name,
			Type:    "query",
			Example: example,
		})
	}

	// Path parameters (detect patterns like :id, {id})
	pathParts := strings.Split(parsedURL.Path, "/")
	for i, part := range pathParts {
		if part == "" {
			continue
		}

		// Check if this looks like a dynamic value
		if p.looksLikeDynamicValue(part) {
			params = append(params, Parameter{
				Name:    suggestParamName(pathParts, i),
				Type:    "path",
				Example: part,
			})
		}
	}

	// POST body parameters (simplified - would need proper parsing)
	if req.PostData != "" && req.Method != "GET" {
		if strings.HasPrefix(req.Headers["Content-Type"], "application/json") {
			params = append(params, Parameter{
				Name:    "body",
				Type:    "body",
				Example: truncateBody(req.PostData),
			})
		} else if strings.HasPrefix(req.Headers["Content-Type"], "application/x-www-form-urlencoded") {
			formParams, _ := url.ParseQuery(req.PostData)
			for name, values := range formParams {
				example := ""
				if len(values) > 0 {
					example = values[0]
				}
				params = append(params, Parameter{
					Name:    name,
					Type:    "body",
					Example: example,
				})
			}
		}
	}

	return params
}

// looksLikeDynamicValue checks if a path segment looks like a dynamic value.
func (p *PassiveDiscovery) looksLikeDynamicValue(segment string) bool {
	// Pure numeric values are likely IDs
	for _, c := range segment {
		if c < '0' || c > '9' {
			break
		}
		return true
	}

	// UUID-like patterns
	if len(segment) >= 32 {
		dashes := strings.Count(segment, "-")
		if dashes >= 4 {
			return true
		}
	}

	// Base64-like strings
	if len(segment) > 20 {
		return true
	}

	return false
}

// suggestParamName suggests a parameter name based on context.
func suggestParamName(parts []string, idx int) string {
	if idx > 0 {
		prev := parts[idx-1]
		// Singular form of previous path segment
		if strings.HasSuffix(prev, "s") {
			return strings.TrimSuffix(prev, "s") + "_id"
		}
		return prev + "_id"
	}
	return "id"
}

func truncateBody(body string) string {
	if len(body) > 200 {
		return body[:200] + "..."
	}
	return body
}

// GetEndpoints returns all discovered endpoints.
func (p *PassiveDiscovery) GetEndpoints() []Endpoint {
	p.mu.RLock()
	defer p.mu.RUnlock()

	endpoints := make([]Endpoint, 0, len(p.endpoints))
	for _, ep := range p.endpoints {
		endpoints = append(endpoints, Endpoint{
			URL:            ep.URL,
			Method:         ep.Method,
			Source:         "passive",
			Parameters:     ep.Parameters,
			Headers:        ep.Headers,
			DiscoveredFrom: ep.DiscoveredFrom,
			Timestamp:      ep.DiscoveredAt,
		})
	}
	return endpoints
}

// Count returns the number of discovered endpoints.
func (p *PassiveDiscovery) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.endpoints)
}

// Enable enables passive discovery.
func (p *PassiveDiscovery) Enable() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.enabled = true
}

// Disable disables passive discovery.
func (p *PassiveDiscovery) Disable() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.enabled = false
}

// Clear clears all discovered endpoints.
func (p *PassiveDiscovery) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.endpoints = make(map[string]*DiscoveredEndpoint)
}
