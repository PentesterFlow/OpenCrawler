package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PentesterFlow/OpenCrawler/internal/errors"
)

// =============================================================================
// FastClientConfig Tests
// =============================================================================

func TestDefaultFastClientConfig(t *testing.T) {
	config := DefaultFastClientConfig()

	if config.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s", config.Timeout)
	}
	if config.MaxIdleConns != 500 {
		t.Errorf("MaxIdleConns = %d, want 500", config.MaxIdleConns)
	}
	if config.MaxIdleConnsPerHost != 100 {
		t.Errorf("MaxIdleConnsPerHost = %d, want 100", config.MaxIdleConnsPerHost)
	}
	if config.MaxConnsPerHost != 100 {
		t.Errorf("MaxConnsPerHost = %d, want 100", config.MaxConnsPerHost)
	}
	if config.UserAgent == "" {
		t.Error("UserAgent should not be empty")
	}
	if !config.SkipTLSVerify {
		t.Error("SkipTLSVerify should be true by default")
	}
}

// =============================================================================
// NewFastClient Tests
// =============================================================================

func TestNewFastClient(t *testing.T) {
	config := DefaultFastClientConfig()
	client := NewFastClient(config)

	if client == nil {
		t.Fatal("NewFastClient returned nil")
	}
	if client.client == nil {
		t.Error("Internal HTTP client is nil")
	}
	if client.userAgent != config.UserAgent {
		t.Errorf("UserAgent = %s, want %s", client.userAgent, config.UserAgent)
	}
}

func TestNewFastClient_CustomConfig(t *testing.T) {
	config := FastClientConfig{
		Timeout:             5 * time.Second,
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 50,
		MaxConnsPerHost:     50,
		UserAgent:           "CustomBot/1.0",
		Headers:             map[string]string{"X-Custom": "test"},
		SkipTLSVerify:       false,
	}

	client := NewFastClient(config)

	if client.userAgent != "CustomBot/1.0" {
		t.Errorf("UserAgent = %s, want CustomBot/1.0", client.userAgent)
	}
	if client.headers["X-Custom"] != "test" {
		t.Error("Custom header not set")
	}
}

// =============================================================================
// SetCookies/SetHeaders Tests
// =============================================================================

func TestFastClient_SetCookies(t *testing.T) {
	client := NewFastClient(DefaultFastClientConfig())

	cookies := []*http.Cookie{
		{Name: "session", Value: "abc123"},
		{Name: "token", Value: "xyz789"},
	}

	client.SetCookies(cookies)

	if len(client.cookies) != 2 {
		t.Errorf("Cookies length = %d, want 2", len(client.cookies))
	}
}

func TestFastClient_SetHeaders(t *testing.T) {
	client := NewFastClient(DefaultFastClientConfig())

	headers := map[string]string{
		"Authorization": "Bearer token123",
		"X-API-Key":     "apikey456",
	}

	client.SetHeaders(headers)

	if len(client.headers) != 2 {
		t.Errorf("Headers length = %d, want 2", len(client.headers))
	}
}

// =============================================================================
// Get Tests
// =============================================================================

func TestFastClient_Get_SimpleHTML(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<!DOCTYPE html>
			<html>
			<head><title>Test Page</title></head>
			<body>
				<a href="/page1">Page 1</a>
				<a href="/page2">Page 2</a>
				<a href="https://external.com/link">External</a>
			</body>
			</html>
		`))
	}))
	defer server.Close()

	client := NewFastClient(DefaultFastClientConfig())
	result, err := client.Get(context.Background(), server.URL)

	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}
	if result.Title != "Test Page" {
		t.Errorf("Title = %s, want 'Test Page'", result.Title)
	}
	if len(result.Links) != 3 {
		t.Errorf("Links length = %d, want 3", len(result.Links))
	}
	if result.Duration == 0 {
		t.Error("Duration should not be zero")
	}
}

func TestFastClient_Get_WithForms(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<!DOCTYPE html>
			<html>
			<body>
				<form action="/login" method="POST">
					<input type="text" name="username">
					<input type="password" name="password">
					<input type="hidden" name="csrf_token">
					<input type="submit" value="Login">
				</form>
				<form action="/search">
					<input type="text" name="q">
					<select name="category">
						<option>All</option>
					</select>
					<textarea name="notes"></textarea>
				</form>
			</body>
			</html>
		`))
	}))
	defer server.Close()

	client := NewFastClient(DefaultFastClientConfig())
	result, err := client.Get(context.Background(), server.URL)

	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if len(result.Forms) != 2 {
		t.Errorf("Forms length = %d, want 2", len(result.Forms))
	}

	// Check first form
	form1 := result.Forms[0]
	if form1.Method != "POST" {
		t.Errorf("Form1 method = %s, want POST", form1.Method)
	}
	if len(form1.Inputs) != 3 { // username, password, csrf_token (submit is excluded by name check)
		t.Errorf("Form1 inputs = %d, want 3", len(form1.Inputs))
	}

	// Check second form
	form2 := result.Forms[1]
	if form2.Method != "GET" {
		t.Errorf("Form2 method = %s, want GET", form2.Method)
	}
}

func TestFastClient_Get_WithScripts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<!DOCTYPE html>
			<html>
			<head>
				<script src="/js/app.js"></script>
				<script src="https://cdn.example.com/lib.js"></script>
			</head>
			<body>
				<script>inline();</script>
			</body>
			</html>
		`))
	}))
	defer server.Close()

	client := NewFastClient(DefaultFastClientConfig())
	result, err := client.Get(context.Background(), server.URL)

	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if len(result.Scripts) != 2 {
		t.Errorf("Scripts length = %d, want 2", len(result.Scripts))
	}
}

func TestFastClient_Get_NonHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"key": "value"}`))
	}))
	defer server.Close()

	client := NewFastClient(DefaultFastClientConfig())
	result, err := client.Get(context.Background(), server.URL)

	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}
	// Should not parse non-HTML
	if result.HTML != "" {
		t.Error("HTML should be empty for non-HTML content")
	}
	if len(result.Links) != 0 {
		t.Errorf("Links should be empty for non-HTML, got %d", len(result.Links))
	}
}

func TestFastClient_Get_Redirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><title>Final Page</title></html>`))
	}))
	defer server.Close()

	client := NewFastClient(DefaultFastClientConfig())
	result, err := client.Get(context.Background(), server.URL)

	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !strings.HasSuffix(result.FinalURL, "/final") {
		t.Errorf("FinalURL = %s, want suffix /final", result.FinalURL)
	}
}

func TestFastClient_Get_404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><title>Not Found</title></html>`))
	}))
	defer server.Close()

	client := NewFastClient(DefaultFastClientConfig())
	result, err := client.Get(context.Background(), server.URL)

	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if result.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", result.StatusCode)
	}
}

func TestFastClient_Get_InvalidURL(t *testing.T) {
	client := NewFastClient(DefaultFastClientConfig())
	result, err := client.Get(context.Background(), "://invalid")

	if err == nil {
		t.Error("Get() should return error for invalid URL")
	}
	if result.Error == nil {
		t.Error("Result.Error should be set")
	}
}

func TestFastClient_Get_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	config := DefaultFastClientConfig()
	config.Timeout = 100 * time.Millisecond
	client := NewFastClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.Get(ctx, server.URL)

	if err == nil {
		t.Error("Get() should return error on timeout")
	}
}

func TestFastClient_Get_WithCookies(t *testing.T) {
	var receivedCookies []*http.Cookie
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCookies = r.Cookies()
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html></html>`))
	}))
	defer server.Close()

	client := NewFastClient(DefaultFastClientConfig())
	client.SetCookies([]*http.Cookie{
		{Name: "session", Value: "test123"},
	})

	_, err := client.Get(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if len(receivedCookies) != 1 {
		t.Errorf("Received cookies = %d, want 1", len(receivedCookies))
	}
	if receivedCookies[0].Value != "test123" {
		t.Errorf("Cookie value = %s, want test123", receivedCookies[0].Value)
	}
}

func TestFastClient_Get_WithCustomHeaders(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html></html>`))
	}))
	defer server.Close()

	client := NewFastClient(DefaultFastClientConfig())
	client.SetHeaders(map[string]string{
		"Authorization": "Bearer mytoken",
	})

	_, err := client.Get(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if receivedAuth != "Bearer mytoken" {
		t.Errorf("Authorization header = %s, want Bearer mytoken", receivedAuth)
	}
}

// =============================================================================
// GetBatch Tests
// =============================================================================

func TestFastClient_GetBatch(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><title>Page</title></html>`))
	}))
	defer server.Close()

	client := NewFastClient(DefaultFastClientConfig())
	urls := []string{
		server.URL + "/page1",
		server.URL + "/page2",
		server.URL + "/page3",
	}

	results := client.GetBatch(context.Background(), urls, 3)

	if len(results) != 3 {
		t.Errorf("Results length = %d, want 3", len(results))
	}
	for i, result := range results {
		if result.StatusCode != 200 {
			t.Errorf("Result[%d] StatusCode = %d, want 200", i, result.StatusCode)
		}
	}
}

func TestFastClient_GetBatch_WithConcurrencyLimit(t *testing.T) {
	active := 0
	maxActive := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		active++
		if active > maxActive {
			maxActive = active
		}
		time.Sleep(50 * time.Millisecond)
		active--
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html></html>`))
	}))
	defer server.Close()

	client := NewFastClient(DefaultFastClientConfig())
	urls := make([]string, 10)
	for i := range urls {
		urls[i] = server.URL
	}

	client.GetBatch(context.Background(), urls, 2)

	if maxActive > 2 {
		t.Errorf("Max concurrent requests = %d, want <= 2", maxActive)
	}
}

// =============================================================================
// Head Tests
// =============================================================================

func TestFastClient_Head(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" {
			t.Errorf("Method = %s, want HEAD", r.Method)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewFastClient(DefaultFastClientConfig())
	statusCode, contentType, err := client.Head(context.Background(), server.URL)

	if err != nil {
		t.Fatalf("Head() error = %v", err)
	}
	if statusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", statusCode)
	}
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("ContentType = %s, want text/html", contentType)
	}
}

func TestFastClient_Head_InvalidURL(t *testing.T) {
	client := NewFastClient(DefaultFastClientConfig())
	_, _, err := client.Head(context.Background(), "://invalid")

	if err == nil {
		t.Error("Head() should return error for invalid URL")
	}
}

func TestFastClient_Head_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewFastClient(DefaultFastClientConfig())
	statusCode, _, err := client.Head(context.Background(), server.URL)

	if err != nil {
		t.Fatalf("Head() error = %v", err)
	}
	if statusCode != 500 {
		t.Errorf("StatusCode = %d, want 500", statusCode)
	}
}

// =============================================================================
// Close Tests
// =============================================================================

func TestFastClient_Close(t *testing.T) {
	client := NewFastClient(DefaultFastClientConfig())
	client.Close() // Should not panic
}

// =============================================================================
// URL Resolution Tests
// =============================================================================

func TestFastClient_resolveURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<!DOCTYPE html>
			<html>
			<body>
				<a href="/absolute">Absolute</a>
				<a href="relative.html">Relative</a>
				<a href="../parent">Parent</a>
				<a href="?query=1">Query only</a>
				<a href="#anchor">Anchor only</a>
				<a href="javascript:void(0)">JavaScript</a>
				<a href="mailto:test@example.com">Mailto</a>
				<a href="tel:+1234567890">Tel</a>
				<a href="data:text/plain,test">Data URL</a>
				<a href="ftp://ftp.example.com">FTP</a>
			</body>
			</html>
		`))
	}))
	defer server.Close()

	client := NewFastClient(DefaultFastClientConfig())
	result, err := client.Get(context.Background(), server.URL+"/dir/page")

	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	// Should only have http/https links, no javascript/mailto/tel/data/ftp/anchor-only
	for _, link := range result.Links {
		if strings.HasPrefix(link, "javascript:") ||
			strings.HasPrefix(link, "mailto:") ||
			strings.HasPrefix(link, "tel:") ||
			strings.HasPrefix(link, "data:") ||
			strings.HasPrefix(link, "ftp:") ||
			strings.HasPrefix(link, "#") {
			t.Errorf("Should not include link: %s", link)
		}
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

func TestFastClient_Get_EmptyHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(``))
	}))
	defer server.Close()

	client := NewFastClient(DefaultFastClientConfig())
	result, err := client.Get(context.Background(), server.URL)

	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}
}

func TestFastClient_Get_MalformedHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><a href="/test">link<body><div><a href="/test2">`))
	}))
	defer server.Close()

	client := NewFastClient(DefaultFastClientConfig())
	result, err := client.Get(context.Background(), server.URL)

	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	// Parser should handle malformed HTML gracefully
	if len(result.Links) == 0 {
		t.Error("Should extract some links from malformed HTML")
	}
}

func TestFastClient_Get_AreaTag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<html>
			<body>
				<map name="test">
					<area href="/area-link1">
					<area href="/area-link2">
				</map>
			</body>
			</html>
		`))
	}))
	defer server.Close()

	client := NewFastClient(DefaultFastClientConfig())
	result, err := client.Get(context.Background(), server.URL)

	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if len(result.Links) != 2 {
		t.Errorf("Links from area tags = %d, want 2", len(result.Links))
	}
}

func TestFastClient_Get_LinkTag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<html>
			<head>
				<link rel="canonical" href="/canonical">
				<link rel="stylesheet" href="/style.css">
				<link rel="preload" href="/preload.html">
			</head>
			</html>
		`))
	}))
	defer server.Close()

	client := NewFastClient(DefaultFastClientConfig())
	result, err := client.Get(context.Background(), server.URL)

	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	// Should include canonical and preload.html but not .css
	cssFound := false
	for _, link := range result.Links {
		if strings.HasSuffix(link, ".css") {
			cssFound = true
		}
	}
	if cssFound {
		t.Error("Should not include .css links")
	}
}

// =============================================================================
// Error Categorization Tests
// =============================================================================

func TestFastClient_Get_ServerError_Categorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewFastClient(DefaultFastClientConfig())
	result, err := client.Get(context.Background(), server.URL)

	if err == nil {
		t.Fatal("Expected error for 500 response")
	}
	if result.StatusCode != 500 {
		t.Errorf("StatusCode = %d, want 500", result.StatusCode)
	}
	// Verify error is retryable (server errors should be)
	if !client.IsRetryableError(err) {
		t.Error("Server error should be retryable")
	}
}

func TestFastClient_Get_404_Categorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewFastClient(DefaultFastClientConfig())
	result, _ := client.Get(context.Background(), server.URL)

	// 404 should be captured but not cause a return error
	if result.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", result.StatusCode)
	}
}

// =============================================================================
// Retry Tests
// =============================================================================

func TestFastClient_GetWithRetry_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><body>OK</body></html>"))
	}))
	defer server.Close()

	client := NewFastClient(DefaultFastClientConfig())
	result, err := client.GetWithRetry(context.Background(), server.URL)

	if err != nil {
		t.Fatalf("GetWithRetry() error = %v", err)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}
}

func TestFastClient_GetWithRetry_EventualSuccess(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><body>OK</body></html>"))
	}))
	defer server.Close()

	client := NewFastClient(DefaultFastClientConfig())
	result, err := client.GetWithRetry(context.Background(), server.URL)

	if err != nil {
		t.Fatalf("GetWithRetry() error = %v", err)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}
	if attempts < 2 {
		t.Errorf("Expected at least 2 attempts, got %d", attempts)
	}
}

func TestFastClient_SetRetryConfig(t *testing.T) {
	client := NewFastClient(DefaultFastClientConfig())

	// Set custom retry config
	client.SetRetryConfig(errors.RetryConfig{
		MaxRetries:   5,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   1.5,
	})

	// Just verify it doesn't panic
	if client.retrier == nil {
		t.Error("Retrier should not be nil after SetRetryConfig")
	}
}

func TestFastClient_IsRetryableError(t *testing.T) {
	client := NewFastClient(DefaultFastClientConfig())

	// Test with retryable error
	retryableErr := errors.NewNetworkError("http://example.com", "test", nil)
	if !client.IsRetryableError(retryableErr) {
		t.Error("Network error should be retryable")
	}

	// Test with non-retryable error
	nonRetryableErr := errors.NewNotFoundError("http://example.com")
	if client.IsRetryableError(nonRetryableErr) {
		t.Error("NotFound error should not be retryable")
	}
}
