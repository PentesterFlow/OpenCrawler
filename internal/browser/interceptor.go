package browser

import (
	"net/url"
	"strings"
	"sync"
	"time"
)

// Interceptor captures network requests during page loads.
type Interceptor struct {
	mu       sync.RWMutex
	requests []NetworkRequest
	filters  []RequestFilter
}

// RequestFilter defines a filter for network requests.
type RequestFilter struct {
	URLPattern   string
	ResourceType string
	Method       string
}

// NewInterceptor creates a new request interceptor.
func NewInterceptor() *Interceptor {
	return &Interceptor{
		requests: make([]NetworkRequest, 0),
	}
}

// AddFilter adds a filter for capturing specific requests.
func (i *Interceptor) AddFilter(filter RequestFilter) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.filters = append(i.filters, filter)
}

// Record records a network request.
func (i *Interceptor) Record(req NetworkRequest) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.requests = append(i.requests, req)
}

// GetRequests returns all captured requests.
func (i *Interceptor) GetRequests() []NetworkRequest {
	i.mu.RLock()
	defer i.mu.RUnlock()

	result := make([]NetworkRequest, len(i.requests))
	copy(result, i.requests)
	return result
}

// GetAPIRequests returns requests that look like API calls.
func (i *Interceptor) GetAPIRequests() []NetworkRequest {
	i.mu.RLock()
	defer i.mu.RUnlock()

	result := make([]NetworkRequest, 0)
	for _, req := range i.requests {
		if isAPIRequest(req) {
			result = append(result, req)
		}
	}
	return result
}

// Clear clears all captured requests.
func (i *Interceptor) Clear() {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.requests = make([]NetworkRequest, 0)
}

// isAPIRequest checks if a request looks like an API call.
func isAPIRequest(req NetworkRequest) bool {
	// Check resource type
	if req.ResourceType == "XHR" || req.ResourceType == "Fetch" {
		return true
	}

	// Check URL patterns
	u, err := url.Parse(req.URL)
	if err != nil {
		return false
	}

	path := strings.ToLower(u.Path)

	// Common API patterns
	apiPatterns := []string{
		"/api/",
		"/v1/",
		"/v2/",
		"/v3/",
		"/graphql",
		"/rest/",
		"/rpc/",
		"/ajax/",
		"/_api/",
		"/ws/",
	}

	for _, pattern := range apiPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	// Check for JSON/XML content type in headers
	contentType := req.Headers["Content-Type"]
	if strings.Contains(contentType, "application/json") ||
		strings.Contains(contentType, "application/xml") {
		return true
	}

	// Check for common API indicators in query string
	query := strings.ToLower(u.RawQuery)
	if strings.Contains(query, "format=json") ||
		strings.Contains(query, "callback=") ||
		strings.Contains(query, "jsonp=") {
		return true
	}

	return false
}

// RequestGroup groups requests by endpoint.
type RequestGroup struct {
	Endpoint   string
	Method     string
	Requests   []NetworkRequest
	Parameters map[string][]string
}

// GroupByEndpoint groups requests by their endpoint.
func (i *Interceptor) GroupByEndpoint() []RequestGroup {
	i.mu.RLock()
	defer i.mu.RUnlock()

	groups := make(map[string]*RequestGroup)

	for _, req := range i.requests {
		u, err := url.Parse(req.URL)
		if err != nil {
			continue
		}

		// Create endpoint key (without query params)
		endpoint := u.Scheme + "://" + u.Host + u.Path
		key := req.Method + " " + endpoint

		group, exists := groups[key]
		if !exists {
			group = &RequestGroup{
				Endpoint:   endpoint,
				Method:     req.Method,
				Requests:   make([]NetworkRequest, 0),
				Parameters: make(map[string][]string),
			}
			groups[key] = group
		}

		group.Requests = append(group.Requests, req)

		// Collect unique parameter values
		for param, values := range u.Query() {
			if _, exists := group.Parameters[param]; !exists {
				group.Parameters[param] = make([]string, 0)
			}
			for _, v := range values {
				if !contains(group.Parameters[param], v) {
					group.Parameters[param] = append(group.Parameters[param], v)
				}
			}
		}
	}

	result := make([]RequestGroup, 0, len(groups))
	for _, group := range groups {
		result = append(result, *group)
	}
	return result
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Stats returns interceptor statistics.
type InterceptorStats struct {
	TotalRequests int            `json:"total_requests"`
	APIRequests   int            `json:"api_requests"`
	ByType        map[string]int `json:"by_type"`
	ByMethod      map[string]int `json:"by_method"`
}

// Stats returns interceptor statistics.
func (i *Interceptor) Stats() InterceptorStats {
	i.mu.RLock()
	defer i.mu.RUnlock()

	stats := InterceptorStats{
		TotalRequests: len(i.requests),
		ByType:        make(map[string]int),
		ByMethod:      make(map[string]int),
	}

	for _, req := range i.requests {
		stats.ByType[req.ResourceType]++
		stats.ByMethod[req.Method]++

		if isAPIRequest(req) {
			stats.APIRequests++
		}
	}

	return stats
}

// RequestTimeline represents requests over time.
type RequestTimeline struct {
	Start    time.Time
	End      time.Time
	Requests []NetworkRequest
}

// GetTimeline returns requests grouped by time.
func (i *Interceptor) GetTimeline() *RequestTimeline {
	i.mu.RLock()
	defer i.mu.RUnlock()

	if len(i.requests) == 0 {
		return nil
	}

	timeline := &RequestTimeline{
		Requests: make([]NetworkRequest, len(i.requests)),
	}
	copy(timeline.Requests, i.requests)

	// Find start and end times
	timeline.Start = i.requests[0].Timestamp
	timeline.End = i.requests[0].Timestamp

	for _, req := range i.requests {
		if req.Timestamp.Before(timeline.Start) {
			timeline.Start = req.Timestamp
		}
		if req.Timestamp.After(timeline.End) {
			timeline.End = req.Timestamp
		}
	}

	return timeline
}
