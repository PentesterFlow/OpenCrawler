package discovery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PentesterFlow/OpenCrawler/internal/browser"
)

// =============================================================================
// PassiveDiscovery Tests
// =============================================================================

func TestNewPassiveDiscovery(t *testing.T) {
	pd := NewPassiveDiscovery()

	if pd == nil {
		t.Fatal("NewPassiveDiscovery() returned nil")
	}
	if !pd.enabled {
		t.Error("should be enabled by default")
	}
	if pd.endpoints == nil {
		t.Error("endpoints map should be initialized")
	}
}

func TestPassiveDiscovery_ProcessRequests(t *testing.T) {
	pd := NewPassiveDiscovery()

	requests := []browser.NetworkRequest{
		{
			URL:          "https://api.example.com/v1/users?page=1",
			Method:       "GET",
			ResourceType: "XHR",
			Headers:      map[string]string{"Content-Type": "application/json"},
		},
		{
			URL:          "https://api.example.com/v1/users",
			Method:       "POST",
			ResourceType: "Fetch",
			Headers:      map[string]string{"Content-Type": "application/json"},
			PostData:     `{"name":"John"}`,
		},
		{
			URL:          "https://example.com/static/image.png",
			Method:       "GET",
			ResourceType: "Image",
		},
	}

	discovered := pd.ProcessRequests(requests, "https://example.com/dashboard")

	// Should discover 2 API endpoints (not the image)
	if len(discovered) != 2 {
		t.Errorf("len(discovered) = %d, want 2", len(discovered))
	}

	// Verify endpoints
	for _, ep := range discovered {
		if ep.Source != "passive" {
			t.Errorf("Source = %q, want passive", ep.Source)
		}
		if ep.DiscoveredFrom != "https://example.com/dashboard" {
			t.Errorf("DiscoveredFrom = %q", ep.DiscoveredFrom)
		}
	}
}

func TestPassiveDiscovery_ProcessRequests_Disabled(t *testing.T) {
	pd := NewPassiveDiscovery()
	pd.Disable()

	requests := []browser.NetworkRequest{
		{URL: "https://api.example.com/v1/users", Method: "GET", ResourceType: "XHR"},
	}

	discovered := pd.ProcessRequests(requests, "https://example.com")

	if discovered != nil {
		t.Error("should return nil when disabled")
	}
}

func TestPassiveDiscovery_ProcessRequests_Dedup(t *testing.T) {
	pd := NewPassiveDiscovery()

	// Same endpoint twice
	requests := []browser.NetworkRequest{
		{URL: "https://api.example.com/v1/users", Method: "GET", ResourceType: "XHR"},
		{URL: "https://api.example.com/v1/users", Method: "GET", ResourceType: "XHR"},
	}

	discovered := pd.ProcessRequests(requests, "https://example.com")

	// Should only discover once
	if len(discovered) != 1 {
		t.Errorf("len(discovered) = %d, want 1 (deduplicated)", len(discovered))
	}

	// Hit count should be 2
	if pd.Count() != 1 {
		t.Errorf("Count() = %d, want 1", pd.Count())
	}
}

func TestPassiveDiscovery_isAPIRequest(t *testing.T) {
	pd := NewPassiveDiscovery()

	tests := []struct {
		name string
		req  browser.NetworkRequest
		want bool
	}{
		{
			name: "XHR request",
			req:  browser.NetworkRequest{ResourceType: "XHR"},
			want: true,
		},
		{
			name: "Fetch request",
			req:  browser.NetworkRequest{ResourceType: "Fetch"},
			want: true,
		},
		{
			name: "API path /api/",
			req:  browser.NetworkRequest{URL: "https://example.com/api/users"},
			want: true,
		},
		{
			name: "API path /v1/",
			req:  browser.NetworkRequest{URL: "https://example.com/v1/data"},
			want: true,
		},
		{
			name: "API path /graphql",
			req:  browser.NetworkRequest{URL: "https://example.com/graphql"},
			want: true,
		},
		{
			name: "JSON content type",
			req: browser.NetworkRequest{
				URL:     "https://example.com/data",
				Headers: map[string]string{"Content-Type": "application/json"},
			},
			want: true,
		},
		{
			name: "Image request",
			req:  browser.NetworkRequest{URL: "https://example.com/image.png", ResourceType: "Image"},
			want: false,
		},
		{
			name: "HTML page",
			req:  browser.NetworkRequest{URL: "https://example.com/page.html"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pd.isAPIRequest(tt.req)
			if got != tt.want {
				t.Errorf("isAPIRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPassiveDiscovery_extractParameters(t *testing.T) {
	pd := NewPassiveDiscovery()

	t.Run("query parameters", func(t *testing.T) {
		req := browser.NetworkRequest{
			URL: "https://api.example.com/users?page=1&limit=10",
		}
		params := pd.extractParameters(req)

		if len(params) < 2 {
			t.Errorf("len(params) = %d, want >= 2", len(params))
		}

		found := map[string]bool{}
		for _, p := range params {
			found[p.Name] = true
			if p.Type != "query" && p.Type != "path" {
				t.Errorf("unexpected param type: %s", p.Type)
			}
		}
		if !found["page"] || !found["limit"] {
			t.Error("expected page and limit query params")
		}
	})

	t.Run("path parameters (numeric ID)", func(t *testing.T) {
		req := browser.NetworkRequest{
			URL: "https://api.example.com/users/123/posts/456",
		}
		params := pd.extractParameters(req)

		pathParams := 0
		for _, p := range params {
			if p.Type == "path" {
				pathParams++
			}
		}
		if pathParams < 2 {
			t.Errorf("expected at least 2 path params for numeric IDs, got %d", pathParams)
		}
	})

	t.Run("POST body JSON", func(t *testing.T) {
		req := browser.NetworkRequest{
			URL:      "https://api.example.com/users",
			Method:   "POST",
			PostData: `{"name":"John","email":"john@example.com"}`,
			Headers:  map[string]string{"Content-Type": "application/json"},
		}
		params := pd.extractParameters(req)

		found := false
		for _, p := range params {
			if p.Type == "body" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected body parameter for POST JSON")
		}
	})

	t.Run("POST body form-urlencoded", func(t *testing.T) {
		req := browser.NetworkRequest{
			URL:      "https://api.example.com/users",
			Method:   "POST",
			PostData: "name=John&email=john@example.com",
			Headers:  map[string]string{"Content-Type": "application/x-www-form-urlencoded"},
		}
		params := pd.extractParameters(req)

		bodyParams := 0
		for _, p := range params {
			if p.Type == "body" {
				bodyParams++
			}
		}
		if bodyParams < 2 {
			t.Errorf("expected at least 2 body params, got %d", bodyParams)
		}
	})
}

func TestPassiveDiscovery_looksLikeDynamicValue(t *testing.T) {
	pd := NewPassiveDiscovery()

	tests := []struct {
		segment string
		want    bool
	}{
		{"123", true},      // numeric ID
		{"456789", true},   // numeric ID
		{"users", false},   // static path
		{"api", false},     // static path
		{"550e8400-e29b-41d4-a716-446655440000", true}, // UUID
		{"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9", true},  // long string (token-like)
	}

	for _, tt := range tests {
		t.Run(tt.segment, func(t *testing.T) {
			got := pd.looksLikeDynamicValue(tt.segment)
			if got != tt.want {
				t.Errorf("looksLikeDynamicValue(%q) = %v, want %v", tt.segment, got, tt.want)
			}
		})
	}
}

func TestPassiveDiscovery_GetEndpoints(t *testing.T) {
	pd := NewPassiveDiscovery()

	requests := []browser.NetworkRequest{
		{URL: "https://api.example.com/v1/users", Method: "GET", ResourceType: "XHR"},
		{URL: "https://api.example.com/v1/posts", Method: "GET", ResourceType: "XHR"},
	}
	pd.ProcessRequests(requests, "https://example.com")

	endpoints := pd.GetEndpoints()
	if len(endpoints) != 2 {
		t.Errorf("len(endpoints) = %d, want 2", len(endpoints))
	}
}

func TestPassiveDiscovery_Count(t *testing.T) {
	pd := NewPassiveDiscovery()

	if pd.Count() != 0 {
		t.Error("initial count should be 0")
	}

	requests := []browser.NetworkRequest{
		{URL: "https://api.example.com/v1/users", Method: "GET", ResourceType: "XHR"},
	}
	pd.ProcessRequests(requests, "https://example.com")

	if pd.Count() != 1 {
		t.Errorf("Count() = %d, want 1", pd.Count())
	}
}

func TestPassiveDiscovery_EnableDisable(t *testing.T) {
	pd := NewPassiveDiscovery()

	pd.Disable()
	if pd.enabled {
		t.Error("should be disabled")
	}

	pd.Enable()
	if !pd.enabled {
		t.Error("should be enabled")
	}
}

func TestPassiveDiscovery_Clear(t *testing.T) {
	pd := NewPassiveDiscovery()

	requests := []browser.NetworkRequest{
		{URL: "https://api.example.com/v1/users", Method: "GET", ResourceType: "XHR"},
	}
	pd.ProcessRequests(requests, "https://example.com")

	if pd.Count() == 0 {
		t.Fatal("should have endpoints before clear")
	}

	pd.Clear()

	if pd.Count() != 0 {
		t.Error("Count() should be 0 after clear")
	}
}

func TestPassiveDiscovery_Concurrent(t *testing.T) {
	pd := NewPassiveDiscovery()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			requests := []browser.NetworkRequest{
				{URL: "https://api.example.com/v1/users", Method: "GET", ResourceType: "XHR"},
			}
			pd.ProcessRequests(requests, "https://example.com")
			pd.GetEndpoints()
			pd.Count()
		}(i)
	}
	wg.Wait()
}

func TestSuggestParamName(t *testing.T) {
	tests := []struct {
		name  string
		parts []string
		idx   int
		want  string
	}{
		{"user_id from users", []string{"", "users", "123"}, 2, "user_id"},
		{"post_id from posts", []string{"", "posts", "456"}, 2, "post_id"},
		{"api_id from api", []string{"", "api", "123"}, 2, "api_id"},
		{"_id from empty prev", []string{"", "123"}, 1, "_id"}, // prev is empty string
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := suggestParamName(tt.parts, tt.idx)
			if got != tt.want {
				t.Errorf("suggestParamName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTruncateBody(t *testing.T) {
	short := "short body"
	if truncateBody(short) != short {
		t.Error("short body should not be truncated")
	}

	long := strings.Repeat("x", 300)
	truncated := truncateBody(long)
	if len(truncated) != 203 { // 200 + "..."
		t.Errorf("len(truncated) = %d, want 203", len(truncated))
	}
	if !strings.HasSuffix(truncated, "...") {
		t.Error("truncated body should end with ...")
	}
}

// =============================================================================
// ActiveDiscovery Tests
// =============================================================================

func TestNewActiveDiscovery(t *testing.T) {
	ad := NewActiveDiscovery("TestBot/1.0", map[string]string{"X-Custom": "value"})

	if ad == nil {
		t.Fatal("NewActiveDiscovery() returned nil")
	}
	if ad.userAgent != "TestBot/1.0" {
		t.Errorf("userAgent = %q", ad.userAgent)
	}
	if ad.headers["X-Custom"] != "value" {
		t.Error("custom header not set")
	}
	if !ad.enabled {
		t.Error("should be enabled by default")
	}
}

func TestActiveDiscovery_Probe(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/health":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("healthy"))
		case "/admin":
			w.WriteHeader(http.StatusUnauthorized)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	ad := NewActiveDiscovery("TestBot/1.0", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	discovered := ad.Probe(ctx, server.URL)

	// Should discover /api/v1, /health, and /admin (401 = exists)
	if len(discovered) < 2 {
		t.Errorf("len(discovered) = %d, want >= 2", len(discovered))
	}

	// Verify sources
	for _, ep := range discovered {
		if ep.Source != "active" {
			t.Errorf("Source = %q, want active", ep.Source)
		}
	}
}

func TestActiveDiscovery_Probe_Disabled(t *testing.T) {
	ad := NewActiveDiscovery("TestBot/1.0", nil)
	ad.Disable()

	discovered := ad.Probe(context.Background(), "https://example.com")

	if discovered != nil {
		t.Error("should return nil when disabled")
	}
}

func TestActiveDiscovery_Probe_InvalidURL(t *testing.T) {
	ad := NewActiveDiscovery("TestBot/1.0", nil)

	discovered := ad.Probe(context.Background(), "://invalid")

	if discovered != nil {
		t.Error("should return nil for invalid URL")
	}
}

func TestActiveDiscovery_isDiscovered(t *testing.T) {
	ad := NewActiveDiscovery("", nil)

	tests := []struct {
		name        string
		statusCode  int
		contentType string
		want        bool
	}{
		{"200 OK", 200, "text/html", true},
		{"201 Created", 201, "application/json", true},
		{"401 Unauthorized", 401, "", true},
		{"403 Forbidden", 403, "", true},
		{"405 Method Not Allowed", 405, "", true},
		{"404 Not Found", 404, "text/html", false},
		{"404 with JSON", 404, "application/json", true},
		{"500 Server Error", 500, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Header:     http.Header{},
			}
			if tt.contentType != "" {
				resp.Header.Set("Content-Type", tt.contentType)
			}

			got := ad.isDiscovered(resp)
			if got != tt.want {
				t.Errorf("isDiscovered() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestActiveDiscovery_ProbeMethods(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET", "POST":
			w.WriteHeader(http.StatusOK)
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		case "OPTIONS":
			w.Header().Set("Allow", "GET, POST, DELETE, OPTIONS")
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	ad := NewActiveDiscovery("TestBot/1.0", nil)
	ctx := context.Background()

	discovered := ad.ProbeMethods(ctx, server.URL+"/api/users")

	// Should discover GET, POST, DELETE, OPTIONS, plus 405s indicate endpoint exists
	if len(discovered) < 4 {
		t.Errorf("len(discovered) = %d, want >= 4", len(discovered))
	}

	methods := make(map[string]bool)
	for _, ep := range discovered {
		methods[ep.Method] = true
		if ep.Source != "active_method_probe" {
			t.Errorf("Source = %q, want active_method_probe", ep.Source)
		}
	}

	if !methods["GET"] {
		t.Error("GET should be discovered")
	}
	if !methods["POST"] {
		t.Error("POST should be discovered")
	}
}

func TestActiveDiscovery_ProbeGraphQL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/graphql" && r.Method == "POST" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]string{"__typename": "Query"},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	ad := NewActiveDiscovery("TestBot/1.0", nil)
	ctx := context.Background()

	endpoint := ad.ProbeGraphQL(ctx, server.URL)

	if endpoint == nil {
		t.Fatal("expected GraphQL endpoint to be discovered")
	}
	if endpoint.Method != "POST" {
		t.Errorf("Method = %q, want POST", endpoint.Method)
	}
	if endpoint.Source != "active_graphql" {
		t.Errorf("Source = %q, want active_graphql", endpoint.Source)
	}
}

func TestActiveDiscovery_ProbeGraphQL_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	ad := NewActiveDiscovery("TestBot/1.0", nil)
	ctx := context.Background()

	endpoint := ad.ProbeGraphQL(ctx, server.URL)

	if endpoint != nil {
		t.Error("should return nil when GraphQL not found")
	}
}

func TestActiveDiscovery_ProbeSwagger(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/swagger.json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"swagger": "2.0",
				"info":    map[string]string{"title": "API"},
				"paths":   map[string]interface{}{},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	ad := NewActiveDiscovery("TestBot/1.0", nil)
	ctx := context.Background()

	swaggerURL, err := ad.ProbeSwagger(ctx, server.URL)

	if err != nil {
		t.Fatalf("ProbeSwagger() error = %v", err)
	}
	if swaggerURL != server.URL+"/swagger.json" {
		t.Errorf("swaggerURL = %q", swaggerURL)
	}
}

func TestActiveDiscovery_ProbeSwagger_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	ad := NewActiveDiscovery("TestBot/1.0", nil)
	ctx := context.Background()

	_, err := ad.ProbeSwagger(ctx, server.URL)

	if err == nil {
		t.Error("expected error when swagger not found")
	}
}

func TestActiveDiscovery_GetEndpoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ad := NewActiveDiscovery("TestBot/1.0", nil)
	ctx := context.Background()

	// Probe to populate endpoints
	ad.Probe(ctx, server.URL)

	endpoints := ad.GetEndpoints()
	if len(endpoints) == 0 {
		t.Error("expected some endpoints")
	}
}

func TestActiveDiscovery_Count(t *testing.T) {
	ad := NewActiveDiscovery("TestBot/1.0", nil)

	if ad.Count() != 0 {
		t.Error("initial count should be 0")
	}
}

func TestActiveDiscovery_EnableDisable(t *testing.T) {
	ad := NewActiveDiscovery("", nil)

	ad.Disable()
	if ad.enabled {
		t.Error("should be disabled")
	}

	ad.Enable()
	if !ad.enabled {
		t.Error("should be enabled")
	}
}

func TestActiveDiscovery_Clear(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ad := NewActiveDiscovery("TestBot/1.0", nil)
	ctx := context.Background()

	ad.Probe(ctx, server.URL)

	ad.Clear()

	if ad.Count() != 0 {
		t.Error("Count() should be 0 after clear")
	}
}

func TestActiveDiscovery_Concurrent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ad := NewActiveDiscovery("TestBot/1.0", nil)
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ad.Probe(ctx, server.URL)
			ad.GetEndpoints()
			ad.Count()
		}()
	}
	wg.Wait()
}

func TestActiveDiscovery_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ad := NewActiveDiscovery("TestBot/1.0", nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Should not panic and return quickly
	discovered := ad.Probe(ctx, server.URL)

	// May have partial results or none
	_ = discovered
}

// =============================================================================
// Type Tests
// =============================================================================

func TestEndpoint_Struct(t *testing.T) {
	ep := Endpoint{
		URL:            "https://api.example.com/users",
		Method:         "GET",
		Source:         "passive",
		Parameters:     []Parameter{{Name: "page", Type: "query"}},
		Headers:        map[string]string{"Authorization": "Bearer token"},
		DiscoveredFrom: "https://example.com",
		StatusCode:     200,
		ContentType:    "application/json",
		Timestamp:      time.Now(),
	}

	if ep.URL != "https://api.example.com/users" {
		t.Error("URL not set correctly")
	}
	if len(ep.Parameters) != 1 {
		t.Error("Parameters not set correctly")
	}
}

func TestParameter_Struct(t *testing.T) {
	param := Parameter{
		Name:    "page",
		Type:    "query",
		Example: "1",
	}

	if param.Name != "page" {
		t.Error("Name not set correctly")
	}
	if param.Type != "query" {
		t.Error("Type not set correctly")
	}
}

func TestCommonAPIPaths(t *testing.T) {
	if len(CommonAPIPaths) == 0 {
		t.Fatal("CommonAPIPaths should not be empty")
	}

	// Check some expected paths
	expected := map[string]bool{
		"/api":     true,
		"/graphql": true,
		"/health":  true,
		"/swagger": true,
	}

	found := make(map[string]bool)
	for _, path := range CommonAPIPaths {
		if expected[path] {
			found[path] = true
		}
	}

	for path := range expected {
		if !found[path] {
			t.Errorf("expected %q in CommonAPIPaths", path)
		}
	}
}
