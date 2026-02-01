package discovery

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ActiveDiscovery probes for API endpoints.
type ActiveDiscovery struct {
	mu        sync.RWMutex
	client    *http.Client
	endpoints map[string]*ProbeResult
	userAgent string
	headers   map[string]string
	enabled   bool
}

// ProbeResult represents the result of probing an endpoint.
type ProbeResult struct {
	URL         string
	Method      string
	StatusCode  int
	ContentType string
	Discovered  bool
	ProbeTime   time.Time
}

// NewActiveDiscovery creates a new active discovery instance.
func NewActiveDiscovery(userAgent string, headers map[string]string) *ActiveDiscovery {
	return &ActiveDiscovery{
		client: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
		endpoints: make(map[string]*ProbeResult),
		userAgent: userAgent,
		headers:   headers,
		enabled:   true,
	}
}

// CommonAPIPaths contains common API paths to probe.
var CommonAPIPaths = []string{
	"/api",
	"/api/v1",
	"/api/v2",
	"/api/v3",
	"/v1",
	"/v2",
	"/v3",
	"/rest",
	"/rest/api",
	"/graphql",
	"/graphiql",
	"/swagger",
	"/swagger-ui",
	"/swagger.json",
	"/swagger/v1/swagger.json",
	"/openapi",
	"/openapi.json",
	"/api-docs",
	"/api/docs",
	"/docs",
	"/doc",
	"/documentation",
	"/health",
	"/healthz",
	"/healthcheck",
	"/status",
	"/ping",
	"/info",
	"/version",
	"/metrics",
	"/.well-known/openid-configuration",
	"/oauth/token",
	"/auth/token",
	"/token",
	"/login",
	"/signin",
	"/signup",
	"/register",
	"/logout",
	"/users",
	"/user",
	"/me",
	"/profile",
	"/account",
	"/admin",
	"/dashboard",
	"/config",
	"/settings",
	"/search",
	"/query",
	"/debug",
	"/trace",
	"/actuator",
	"/actuator/health",
	"/actuator/info",
	"/actuator/env",
	"/env",
	"/console",
	"/admin/console",
	"/ws",
	"/websocket",
	"/socket.io",
}

// Probe probes a base URL for common API endpoints.
func (a *ActiveDiscovery) Probe(ctx context.Context, baseURL string) []Endpoint {
	if !a.enabled {
		return nil
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil
	}

	base := parsed.Scheme + "://" + parsed.Host

	discovered := make([]Endpoint, 0)
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Limit concurrent probes
	sem := make(chan struct{}, 10)

	for _, path := range CommonAPIPaths {
		path := path
		wg.Add(1)

		go func() {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			probeURL := base + path
			result := a.probeURL(ctx, probeURL)

			if result.Discovered {
				mu.Lock()
				discovered = append(discovered, Endpoint{
					URL:         result.URL,
					Method:      result.Method,
					Source:      "active",
					StatusCode:  result.StatusCode,
					ContentType: result.ContentType,
					Timestamp:   result.ProbeTime,
				})
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	return discovered
}

// probeURL probes a single URL.
func (a *ActiveDiscovery) probeURL(ctx context.Context, urlStr string) *ProbeResult {
	result := &ProbeResult{
		URL:       urlStr,
		Method:    "GET",
		ProbeTime: time.Now(),
	}

	a.mu.Lock()
	if existing, ok := a.endpoints[urlStr]; ok {
		a.mu.Unlock()
		return existing
	}
	a.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return result
	}

	// Set headers
	if a.userAgent != "" {
		req.Header.Set("User-Agent", a.userAgent)
	}
	for k, v := range a.headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Accept", "application/json, text/html, */*")

	resp, err := a.client.Do(req)
	if err != nil {
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.ContentType = resp.Header.Get("Content-Type")

	// Consider it discovered if we get a meaningful response
	result.Discovered = a.isDiscovered(resp)

	if result.Discovered {
		a.mu.Lock()
		a.endpoints[urlStr] = result
		a.mu.Unlock()
	}

	// Drain body
	io.Copy(io.Discard, resp.Body)

	return result
}

// isDiscovered determines if a response indicates a valid endpoint.
func (a *ActiveDiscovery) isDiscovered(resp *http.Response) bool {
	// 2xx responses are definitely discovered
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true
	}

	// 401/403 indicates the endpoint exists but requires auth
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return true
	}

	// 405 indicates the endpoint exists but wrong method
	if resp.StatusCode == 405 {
		return true
	}

	// Check for JSON/API content type even on errors
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return true
		}
	}

	return false
}

// ProbeMethods probes an endpoint with different HTTP methods.
func (a *ActiveDiscovery) ProbeMethods(ctx context.Context, urlStr string) []Endpoint {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"}
	discovered := make([]Endpoint, 0)

	for _, method := range methods {
		result := a.probeMethod(ctx, urlStr, method)
		if result.Discovered {
			discovered = append(discovered, Endpoint{
				URL:         result.URL,
				Method:      result.Method,
				Source:      "active_method_probe",
				StatusCode:  result.StatusCode,
				ContentType: result.ContentType,
				Timestamp:   result.ProbeTime,
			})
		}
	}

	return discovered
}

// probeMethod probes a URL with a specific HTTP method.
func (a *ActiveDiscovery) probeMethod(ctx context.Context, urlStr, method string) *ProbeResult {
	result := &ProbeResult{
		URL:       urlStr,
		Method:    method,
		ProbeTime: time.Now(),
	}

	key := method + " " + urlStr
	a.mu.Lock()
	if existing, ok := a.endpoints[key]; ok {
		a.mu.Unlock()
		return existing
	}
	a.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, method, urlStr, nil)
	if err != nil {
		return result
	}

	if a.userAgent != "" {
		req.Header.Set("User-Agent", a.userAgent)
	}
	for k, v := range a.headers {
		req.Header.Set(k, v)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.ContentType = resp.Header.Get("Content-Type")
	result.Discovered = a.isDiscovered(resp)

	if result.Discovered {
		a.mu.Lock()
		a.endpoints[key] = result
		a.mu.Unlock()
	}

	io.Copy(io.Discard, resp.Body)

	return result
}

// ProbeGraphQL specifically probes for GraphQL endpoints.
func (a *ActiveDiscovery) ProbeGraphQL(ctx context.Context, baseURL string) *Endpoint {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil
	}

	base := parsed.Scheme + "://" + parsed.Host

	graphqlPaths := []string{"/graphql", "/gql", "/api/graphql", "/v1/graphql"}

	for _, path := range graphqlPaths {
		testURL := base + path

		// Send introspection query
		query := `{"query": "{ __typename }"}`

		req, err := http.NewRequestWithContext(ctx, "POST", testURL, strings.NewReader(query))
		if err != nil {
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		if a.userAgent != "" {
			req.Header.Set("User-Agent", a.userAgent)
		}

		resp, err := a.client.Do(req)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Check if it looks like GraphQL
		if resp.StatusCode == 200 && strings.Contains(string(body), "__typename") {
			return &Endpoint{
				URL:         testURL,
				Method:      "POST",
				Source:      "active_graphql",
				StatusCode:  resp.StatusCode,
				ContentType: resp.Header.Get("Content-Type"),
				Timestamp:   time.Now(),
			}
		}
	}

	return nil
}

// ProbeSwagger probes for Swagger/OpenAPI documentation.
func (a *ActiveDiscovery) ProbeSwagger(ctx context.Context, baseURL string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	base := parsed.Scheme + "://" + parsed.Host

	swaggerPaths := []string{
		"/swagger.json",
		"/swagger/v1/swagger.json",
		"/api-docs",
		"/openapi.json",
		"/openapi/v3/api-docs",
		"/v2/api-docs",
		"/v3/api-docs",
	}

	for _, path := range swaggerPaths {
		testURL := base + path

		req, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
		if err != nil {
			continue
		}

		req.Header.Set("Accept", "application/json")
		if a.userAgent != "" {
			req.Header.Set("User-Agent", a.userAgent)
		}

		resp, err := a.client.Do(req)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Check if it looks like OpenAPI/Swagger
		if resp.StatusCode == 200 {
			bodyStr := string(body)
			if strings.Contains(bodyStr, "swagger") ||
				strings.Contains(bodyStr, "openapi") ||
				strings.Contains(bodyStr, "paths") {
				return testURL, nil
			}
		}
	}

	return "", fmt.Errorf("no swagger documentation found")
}

// GetEndpoints returns all discovered endpoints.
func (a *ActiveDiscovery) GetEndpoints() []Endpoint {
	a.mu.RLock()
	defer a.mu.RUnlock()

	endpoints := make([]Endpoint, 0, len(a.endpoints))
	for _, ep := range a.endpoints {
		if ep.Discovered {
			endpoints = append(endpoints, Endpoint{
				URL:         ep.URL,
				Method:      ep.Method,
				Source:      "active",
				StatusCode:  ep.StatusCode,
				ContentType: ep.ContentType,
				Timestamp:   ep.ProbeTime,
			})
		}
	}
	return endpoints
}

// Count returns the number of discovered endpoints.
func (a *ActiveDiscovery) Count() int {
	a.mu.RLock()
	defer a.mu.RUnlock()

	count := 0
	for _, ep := range a.endpoints {
		if ep.Discovered {
			count++
		}
	}
	return count
}

// Enable enables active discovery.
func (a *ActiveDiscovery) Enable() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enabled = true
}

// Disable disables active discovery.
func (a *ActiveDiscovery) Disable() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enabled = false
}

// Clear clears all discovered endpoints.
func (a *ActiveDiscovery) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.endpoints = make(map[string]*ProbeResult)
}
