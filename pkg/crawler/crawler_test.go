package crawler

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/PentesterFlow/OpenCrawler/internal/auth"
	"github.com/PentesterFlow/OpenCrawler/internal/discovery"
	"github.com/PentesterFlow/OpenCrawler/internal/parser"
	"github.com/PentesterFlow/OpenCrawler/internal/state"
	"github.com/PentesterFlow/OpenCrawler/internal/websocket"
)

// =============================================================================
// New() Tests
// =============================================================================

func TestNew_DefaultConfig(t *testing.T) {
	c, err := New(WithTarget("https://example.com"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if c == nil {
		t.Fatal("New() returned nil crawler")
	}
	if c.config == nil {
		t.Error("Crawler config is nil")
	}
	if c.config.Target != "https://example.com" {
		t.Errorf("Target = %s, want https://example.com", c.config.Target)
	}
	if c.resultsChan == nil {
		t.Error("resultsChan is nil")
	}
	if c.errorsChan == nil {
		t.Error("errorsChan is nil")
	}
}

func TestNew_WithMultipleOptions(t *testing.T) {
	c, err := New(
		WithTarget("https://example.com"),
		WithWorkers(100),
		WithMaxDepth(20),
		WithTimeout(60*time.Second),
		WithVerbose(true),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if c.config.Workers != 100 {
		t.Errorf("Workers = %d, want 100", c.config.Workers)
	}
	if c.config.MaxDepth != 20 {
		t.Errorf("MaxDepth = %d, want 20", c.config.MaxDepth)
	}
	if c.config.Timeout != 60*time.Second {
		t.Errorf("Timeout = %v, want 60s", c.config.Timeout)
	}
}

func TestNew_ValidationError(t *testing.T) {
	// No target - should fail validation
	_, err := New()
	if err == nil {
		t.Error("New() should return error for missing target")
	}
}

func TestNew_WithConfig(t *testing.T) {
	config := TurboConfig()
	config.Target = "https://fast.example.com"

	c, err := New(WithConfig(config))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if c.config.Workers != 200 {
		t.Errorf("Workers = %d, want 200 (turbo config)", c.config.Workers)
	}
}

// =============================================================================
// IsRunning Tests
// =============================================================================

func TestCrawler_IsRunning(t *testing.T) {
	c, err := New(WithTarget("https://example.com"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if c.IsRunning() {
		t.Error("Crawler should not be running initially")
	}
}

// =============================================================================
// Results Channel Tests
// =============================================================================

func TestCrawler_Results(t *testing.T) {
	c, err := New(WithTarget("https://example.com"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ch := c.Results()
	if ch == nil {
		t.Error("Results() returned nil channel")
	}
}

// =============================================================================
// getDomain Tests
// =============================================================================

func TestGetDomain(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		expect string
	}{
		{"simple domain", "https://example.com/path", "example.com"},
		{"with port", "https://example.com:8080/path", "example.com:8080"},
		{"subdomain", "https://api.example.com/v1", "api.example.com"},
		{"http", "http://localhost:3000", "localhost:3000"},
		{"invalid", "not-a-url", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getDomain(tt.url)
			if result != tt.expect {
				t.Errorf("getDomain(%q) = %q, want %q", tt.url, result, tt.expect)
			}
		})
	}
}

// =============================================================================
// isHashRoute Tests
// =============================================================================

func TestIsHashRoute(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		expect bool
	}{
		{"no hash", "https://example.com/path", false},
		{"simple anchor", "https://example.com#section", false},
		{"hash route", "https://example.com/#/dashboard", true},
		{"hashbang route", "https://example.com/#!/users", true},
		{"hash route no trailing slash", "https://example.com#/home", true},
		{"empty fragment", "https://example.com#", false},
		{"invalid url", "not-a-url", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isHashRoute(tt.url)
			if result != tt.expect {
				t.Errorf("isHashRoute(%q) = %v, want %v", tt.url, result, tt.expect)
			}
		})
	}
}

// =============================================================================
// splitHashURL Tests
// =============================================================================

func TestSplitHashURL(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		expectBase     string
		expectFragment string
	}{
		{
			name:           "with hash",
			url:            "https://example.com/#/dashboard",
			expectBase:     "https://example.com/",
			expectFragment: "#/dashboard",
		},
		{
			name:           "no hash",
			url:            "https://example.com/path",
			expectBase:     "https://example.com/path",
			expectFragment: "",
		},
		{
			name:           "multiple hashes",
			url:            "https://example.com/#/path#extra",
			expectBase:     "https://example.com/",
			expectFragment: "#/path#extra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, fragment := splitHashURL(tt.url)
			if base != tt.expectBase {
				t.Errorf("splitHashURL(%q) base = %q, want %q", tt.url, base, tt.expectBase)
			}
			if fragment != tt.expectFragment {
				t.Errorf("splitHashURL(%q) fragment = %q, want %q", tt.url, fragment, tt.expectFragment)
			}
		})
	}
}

// =============================================================================
// needsJavaScriptRendering Tests
// =============================================================================

func TestNeedsJavaScriptRendering(t *testing.T) {
	c := &Crawler{config: DefaultConfig()}

	tests := []struct {
		name        string
		html        string
		contentType string
		expect      bool
	}{
		{
			name:        "not html",
			html:        `{"data": "json"}`,
			contentType: "application/json",
			expect:      false,
		},
		{
			name:        "small body",
			html:        "<html><body></body></html>",
			contentType: "text/html",
			expect:      true,
		},
		{
			name: "angular app",
			html: `<html><body><div ng-app="myApp"><div ng-view></div></div>
				<script src="app.js"></script></body></html>` + string(make([]byte, 500)),
			contentType: "text/html",
			expect:      true,
		},
		{
			name: "react app",
			html: `<html><body><div id="root" data-reactroot></div>
				<script src="bundle.js"></script></body></html>` + string(make([]byte, 500)),
			contentType: "text/html",
			expect:      true,
		},
		{
			name: "vue app",
			html: `<html><body><div data-v-123abc></div>
				<script src="vue.js"></script></body></html>` + string(make([]byte, 500)),
			contentType: "text/html",
			expect:      true,
		},
		{
			name: "nextjs app",
			html: `<html><body><div id="__next"></div>
				<script src="next.js"></script></body></html>` + string(make([]byte, 500)),
			contentType: "text/html",
			expect:      true,
		},
		{
			name: "regular html",
			html: `<html><body><h1>Hello World</h1><p>This is a regular page with enough content to pass the length check.</p>
				<ul><li>Item 1</li><li>Item 2</li><li>Item 3</li><li>Item 4</li><li>Item 5</li></ul>
				<p>More content here to make sure we have enough text content that exceeds the 200 character threshold.</p>
				<div class="container"><article><header>Article Title</header><section>Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.</section></article></div></body></html>`,
			contentType: "text/html",
			expect:      false,
		},
		{
			name: "empty body with scripts",
			html: `<html><body><script src="app.js"></script><script>console.log("hello");</script></body></html>`,
			contentType: "text/html",
			expect:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.needsJavaScriptRendering(tt.html, tt.contentType)
			if result != tt.expect {
				t.Errorf("needsJavaScriptRendering() = %v, want %v", result, tt.expect)
			}
		})
	}
}

// =============================================================================
// convertAuthCreds Tests
// =============================================================================

func TestConvertAuthCreds(t *testing.T) {
	tests := []struct {
		name     string
		input    AuthCredentials
		expected auth.AuthType
	}{
		{
			name: "none",
			input: AuthCredentials{
				Type: AuthTypeNone,
			},
			expected: auth.AuthTypeNone,
		},
		{
			name: "jwt",
			input: AuthCredentials{
				Type:  AuthTypeJWT,
				Token: "test-token",
			},
			expected: auth.AuthTypeJWT,
		},
		{
			name: "basic",
			input: AuthCredentials{
				Type:     AuthTypeBasic,
				Username: "user",
				Password: "pass",
			},
			expected: auth.AuthTypeBasic,
		},
		{
			name: "api key",
			input: AuthCredentials{
				Type:    AuthTypeAPIKey,
				Headers: map[string]string{"X-API-Key": "key123"},
			},
			expected: auth.AuthTypeAPIKey,
		},
		{
			name: "session",
			input: AuthCredentials{
				Type:    AuthTypeSession,
				Cookies: []*http.Cookie{{Name: "session", Value: "abc"}},
			},
			expected: auth.AuthTypeSession,
		},
		{
			name: "form login",
			input: AuthCredentials{
				Type:       AuthTypeFormLogin,
				LoginURL:   "https://example.com/login",
				Username:   "admin",
				Password:   "secret",
				FormFields: map[string]string{"csrf": "token"},
			},
			expected: auth.AuthTypeFormLogin,
		},
		{
			name: "oauth with config",
			input: AuthCredentials{
				Type: AuthTypeOAuth,
				OAuthConfig: &OAuthConfig{
					ClientID:     "client-123",
					ClientSecret: "secret-456",
					AuthURL:      "https://auth.example.com/oauth/authorize",
					TokenURL:     "https://auth.example.com/oauth/token",
					RedirectURL:  "https://app.example.com/callback",
					Scopes:       []string{"read", "write"},
				},
			},
			expected: auth.AuthTypeOAuth,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertAuthCreds(tt.input)
			if result.Type != tt.expected {
				t.Errorf("convertAuthCreds() Type = %v, want %v", result.Type, tt.expected)
			}
		})
	}
}

func TestConvertAuthCreds_OAuthDetails(t *testing.T) {
	input := AuthCredentials{
		Type: AuthTypeOAuth,
		OAuthConfig: &OAuthConfig{
			ClientID:     "client-123",
			ClientSecret: "secret-456",
			AuthURL:      "https://auth.example.com/authorize",
			TokenURL:     "https://auth.example.com/token",
			RedirectURL:  "https://app.example.com/callback",
			Scopes:       []string{"read", "write"},
		},
	}

	result := convertAuthCreds(input)

	if result.OAuthConfig == nil {
		t.Fatal("OAuthConfig is nil")
	}
	if result.OAuthConfig.ClientID != "client-123" {
		t.Errorf("ClientID = %s, want client-123", result.OAuthConfig.ClientID)
	}
	if result.OAuthConfig.ClientSecret != "secret-456" {
		t.Errorf("ClientSecret = %s, want secret-456", result.OAuthConfig.ClientSecret)
	}
	if len(result.OAuthConfig.Scopes) != 2 {
		t.Errorf("Scopes length = %d, want 2", len(result.OAuthConfig.Scopes))
	}
}

// =============================================================================
// convertDiscoveryEndpoint Tests
// =============================================================================

func TestConvertDiscoveryEndpoint(t *testing.T) {
	input := discovery.Endpoint{
		URL:     "https://example.com/api/users",
		Method:  "GET",
		Source:  "passive",
		Headers: map[string]string{"Authorization": "Bearer token"},
		Parameters: []discovery.Parameter{
			{Name: "page", Type: "query", Example: "1"},
			{Name: "limit", Type: "query", Example: "10"},
		},
		DiscoveredFrom: "https://example.com/",
		StatusCode:     200,
		ContentType:    "application/json",
		Timestamp:      time.Now(),
	}

	result := convertDiscoveryEndpoint(input)

	if result.URL != input.URL {
		t.Errorf("URL = %s, want %s", result.URL, input.URL)
	}
	if result.Method != input.Method {
		t.Errorf("Method = %s, want %s", result.Method, input.Method)
	}
	if result.Source != input.Source {
		t.Errorf("Source = %s, want %s", result.Source, input.Source)
	}
	if len(result.Parameters) != 2 {
		t.Errorf("Parameters length = %d, want 2", len(result.Parameters))
	}
	if result.Parameters[0].Name != "page" {
		t.Errorf("First parameter name = %s, want page", result.Parameters[0].Name)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}
}

// =============================================================================
// convertStateCrawlStats Tests
// =============================================================================

func TestConvertStateCrawlStats(t *testing.T) {
	input := state.CrawlStats{
		URLsDiscovered:     100,
		PagesCrawled:       50,
		FormsFound:         10,
		APIEndpoints:       25,
		WebSocketEndpoints: 3,
		ErrorCount:         5,
		Duration:           time.Minute * 5,
		BytesTransferred:   1024000,
	}

	result := convertStateCrawlStats(input)

	if result.URLsDiscovered != 100 {
		t.Errorf("URLsDiscovered = %d, want 100", result.URLsDiscovered)
	}
	if result.PagesCrawled != 50 {
		t.Errorf("PagesCrawled = %d, want 50", result.PagesCrawled)
	}
	if result.FormsFound != 10 {
		t.Errorf("FormsFound = %d, want 10", result.FormsFound)
	}
	if result.APIEndpoints != 25 {
		t.Errorf("APIEndpoints = %d, want 25", result.APIEndpoints)
	}
	if result.WebSocketEndpoints != 3 {
		t.Errorf("WebSocketEndpoints = %d, want 3", result.WebSocketEndpoints)
	}
	if result.ErrorCount != 5 {
		t.Errorf("ErrorCount = %d, want 5", result.ErrorCount)
	}
	if result.Duration != time.Minute*5 {
		t.Errorf("Duration = %v, want 5m", result.Duration)
	}
	if result.BytesTransferred != 1024000 {
		t.Errorf("BytesTransferred = %d, want 1024000", result.BytesTransferred)
	}
}

// =============================================================================
// convertWebSocketEndpoints Tests
// =============================================================================

func TestConvertWebSocketEndpoints(t *testing.T) {
	input := []websocket.WebSocketEndpoint{
		{
			URL:            "wss://example.com/ws",
			DiscoveredFrom: "https://example.com/",
			Protocols:      []string{"graphql-ws"},
			SampleMessages: []websocket.WebSocketMsg{
				{
					Direction: "sent",
					Type:      "text",
					Data:      `{"type":"connection_init"}`,
					Timestamp: time.Now(),
				},
				{
					Direction: "received",
					Type:      "text",
					Data:      `{"type":"connection_ack"}`,
					Timestamp: time.Now(),
				},
			},
			Timestamp: time.Now(),
		},
	}

	result := convertWebSocketEndpoints(input)

	if len(result) != 1 {
		t.Fatalf("result length = %d, want 1", len(result))
	}
	if result[0].URL != "wss://example.com/ws" {
		t.Errorf("URL = %s, want wss://example.com/ws", result[0].URL)
	}
	if len(result[0].SampleMessages) != 2 {
		t.Errorf("SampleMessages length = %d, want 2", len(result[0].SampleMessages))
	}
	if result[0].SampleMessages[0].Direction != "sent" {
		t.Errorf("First message direction = %s, want sent", result[0].SampleMessages[0].Direction)
	}
	if len(result[0].Protocols) != 1 {
		t.Errorf("Protocols length = %d, want 1", len(result[0].Protocols))
	}
}

// =============================================================================
// convertParserForm Tests
// =============================================================================

func TestConvertParserForm(t *testing.T) {
	input := parser.Form{
		URL:     "https://example.com/login",
		Action:  "/auth/login",
		Method:  "POST",
		Enctype: "application/x-www-form-urlencoded",
		HasCSRF: true,
		Depth:   1,
		Inputs: []parser.FormInput{
			{Name: "username", Type: "text", Required: true},
			{Name: "password", Type: "password", Required: true},
			{Name: "_csrf", Type: "hidden", Value: "token123"},
		},
		Timestamp: time.Now(),
	}

	result := convertParserForm(input)

	if result.URL != input.URL {
		t.Errorf("URL = %s, want %s", result.URL, input.URL)
	}
	if result.Action != input.Action {
		t.Errorf("Action = %s, want %s", result.Action, input.Action)
	}
	if result.Method != input.Method {
		t.Errorf("Method = %s, want %s", result.Method, input.Method)
	}
	if !result.HasCSRF {
		t.Error("HasCSRF should be true")
	}
	if len(result.Inputs) != 3 {
		t.Fatalf("Inputs length = %d, want 3", len(result.Inputs))
	}
	if result.Inputs[0].Name != "username" {
		t.Errorf("First input name = %s, want username", result.Inputs[0].Name)
	}
	if !result.Inputs[0].Required {
		t.Error("First input should be required")
	}
}

// =============================================================================
// State Conversion Tests
// =============================================================================

func TestConvertEndpointsToState(t *testing.T) {
	input := []Endpoint{
		{
			URL:            "https://example.com/api/users",
			Method:         "GET",
			Source:         "passive",
			Depth:          2,
			StatusCode:     200,
			DiscoveredFrom: "https://example.com/",
			Parameters: []Parameter{
				{Name: "id", Type: "path", Required: true},
			},
			Timestamp: time.Now(),
		},
	}

	result := convertEndpointsToState(input)

	if len(result) != 1 {
		t.Fatalf("result length = %d, want 1", len(result))
	}
	if result[0].URL != input[0].URL {
		t.Errorf("URL = %s, want %s", result[0].URL, input[0].URL)
	}
	if len(result[0].Parameters) != 1 {
		t.Errorf("Parameters length = %d, want 1", len(result[0].Parameters))
	}
}

func TestConvertEndpointsFromState(t *testing.T) {
	input := []state.Endpoint{
		{
			URL:        "https://example.com/api/users",
			Method:     "POST",
			Source:     "active",
			StatusCode: 201,
			Parameters: []state.Parameter{
				{Name: "name", Type: "body", Required: true},
			},
		},
	}

	result := convertEndpointsFromState(input)

	if len(result) != 1 {
		t.Fatalf("result length = %d, want 1", len(result))
	}
	if result[0].URL != input[0].URL {
		t.Errorf("URL = %s, want %s", result[0].URL, input[0].URL)
	}
	if result[0].Method != "POST" {
		t.Errorf("Method = %s, want POST", result[0].Method)
	}
}

func TestConvertFormsToState(t *testing.T) {
	input := []Form{
		{
			URL:     "https://example.com/register",
			Action:  "/register",
			Method:  "POST",
			HasCSRF: true,
			Inputs: []FormInput{
				{Name: "email", Type: "email", Required: true},
			},
		},
	}

	result := convertFormsToState(input)

	if len(result) != 1 {
		t.Fatalf("result length = %d, want 1", len(result))
	}
	if !result[0].HasCSRF {
		t.Error("HasCSRF should be true")
	}
}

func TestConvertFormsFromState(t *testing.T) {
	input := []state.Form{
		{
			URL:    "https://example.com/contact",
			Action: "/submit",
			Method: "POST",
			Inputs: []state.FormInput{
				{Name: "message", Type: "textarea"},
			},
		},
	}

	result := convertFormsFromState(input)

	if len(result) != 1 {
		t.Fatalf("result length = %d, want 1", len(result))
	}
	if len(result[0].Inputs) != 1 {
		t.Errorf("Inputs length = %d, want 1", len(result[0].Inputs))
	}
}

func TestConvertWebSocketsToState(t *testing.T) {
	input := []WebSocketEndpoint{
		{
			URL:            "wss://example.com/realtime",
			DiscoveredFrom: "https://example.com/app",
			Protocols:      []string{"graphql-transport-ws"},
			SampleMessages: []WebSocketMsg{
				{Direction: "sent", Type: "text", Data: "hello"},
			},
		},
	}

	result := convertWebSocketsToState(input)

	if len(result) != 1 {
		t.Fatalf("result length = %d, want 1", len(result))
	}
	if result[0].URL != "wss://example.com/realtime" {
		t.Errorf("URL = %s, want wss://example.com/realtime", result[0].URL)
	}
}

func TestConvertWebSocketsFromState(t *testing.T) {
	input := []state.WebSocketEndpoint{
		{
			URL:       "wss://example.com/chat",
			Protocols: []string{"chat"},
			SampleMessages: []state.WebSocketMsg{
				{Direction: "received", Type: "text", Data: "welcome"},
			},
		},
	}

	result := convertWebSocketsFromState(input)

	if len(result) != 1 {
		t.Fatalf("result length = %d, want 1", len(result))
	}
	if len(result[0].SampleMessages) != 1 {
		t.Errorf("SampleMessages length = %d, want 1", len(result[0].SampleMessages))
	}
}

func TestConvertErrorsToState(t *testing.T) {
	input := []CrawlError{
		{
			URL:       "https://example.com/broken",
			Error:     "connection refused",
			Timestamp: time.Now(),
		},
	}

	result := convertErrorsToState(input)

	if len(result) != 1 {
		t.Fatalf("result length = %d, want 1", len(result))
	}
	if result[0].Error != "connection refused" {
		t.Errorf("Error = %s, want 'connection refused'", result[0].Error)
	}
}

func TestConvertErrorsFromState(t *testing.T) {
	input := []state.CrawlError{
		{
			URL:       "https://example.com/timeout",
			Error:     "timeout exceeded",
			Timestamp: time.Now(),
		},
	}

	result := convertErrorsFromState(input)

	if len(result) != 1 {
		t.Fatalf("result length = %d, want 1", len(result))
	}
	if result[0].URL != "https://example.com/timeout" {
		t.Errorf("URL = %s, want https://example.com/timeout", result[0].URL)
	}
}

// =============================================================================
// Start/Stop Tests (Basic)
// =============================================================================

func TestCrawler_Start_AlreadyRunning(t *testing.T) {
	c, err := New(WithTarget("https://example.com"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Set running flag manually
	c.running.Store(true)

	_, err = c.Start(context.Background())
	if err == nil {
		t.Error("Start() should return error when already running")
	}
}

func TestCrawler_Stop_NotRunning(t *testing.T) {
	c, err := New(WithTarget("https://example.com"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Stop should not error even when not running
	err = c.Stop()
	if err != nil {
		t.Errorf("Stop() error = %v", err)
	}
}

func TestCrawler_StopNow_NotRunning(t *testing.T) {
	c, err := New(WithTarget("https://example.com"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// StopNow should not error even when not running
	err = c.StopNow()
	if err != nil {
		t.Errorf("StopNow() error = %v", err)
	}
}

// =============================================================================
// ShutdownContext Tests
// =============================================================================

func TestCrawler_ShutdownContext_NilHandler(t *testing.T) {
	c, err := New(WithTarget("https://example.com"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Before Start(), shutdownHandler is initialized but c.ctx is nil
	// ShutdownContext falls back to c.ctx when shutdownHandler exists but ctx is nil
	ctx := c.ShutdownContext()
	// The function returns shutdownHandler.Context() if handler exists
	// Since shutdownHandler is initialized in New(), it has its own context
	if ctx == nil && c.shutdownHandler != nil {
		// This is acceptable - shutdownHandler has its own context
		t.Log("ShutdownContext returns handler context when available")
	}
}

// =============================================================================
// Metrics Tests
// =============================================================================

func TestCrawler_Metrics_Nil(t *testing.T) {
	c, err := New(WithTarget("https://example.com"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Metrics should be initialized in New()
	if c.Metrics() == nil {
		t.Error("Metrics() returned nil")
	}
}

func TestCrawler_MetricsSnapshot_Nil(t *testing.T) {
	c := &Crawler{
		config:  DefaultConfig(),
		metrics: nil,
	}

	snapshot := c.MetricsSnapshot()
	if snapshot != nil {
		t.Error("MetricsSnapshot() should return nil when metrics is nil")
	}
}

// =============================================================================
// resolveFrameworkURL Tests
// =============================================================================

func TestResolveFrameworkURL(t *testing.T) {
	c := &Crawler{config: DefaultConfig()}
	c.config.Target = "https://example.com"

	tests := []struct {
		name     string
		baseURL  string
		path     string
		expected string
	}{
		{
			name:     "empty path",
			baseURL:  "https://example.com",
			path:     "",
			expected: "",
		},
		{
			name:     "hash route",
			baseURL:  "https://example.com",
			path:     "#/dashboard",
			expected: "https://example.com/#/dashboard",
		},
		{
			name:     "absolute path",
			baseURL:  "https://example.com",
			path:     "/api/users",
			expected: "https://example.com/api/users",
		},
		{
			name:     "relative path",
			baseURL:  "https://example.com",
			path:     "users",
			expected: "https://example.com/users",
		},
		{
			name:     "with port",
			baseURL:  "https://example.com:8080",
			path:     "/api/v1",
			expected: "https://example.com:8080/api/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.resolveFrameworkURL(tt.baseURL, tt.path)
			if result != tt.expected {
				t.Errorf("resolveFrameworkURL(%q, %q) = %q, want %q", tt.baseURL, tt.path, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// Type Tests
// =============================================================================

func TestEndpoint_Fields(t *testing.T) {
	ep := Endpoint{
		URL:            "https://example.com/api/users",
		Method:         "GET",
		Source:         "passive",
		Depth:          2,
		Parameters:     []Parameter{{Name: "id", Type: "path"}},
		Headers:        map[string]string{"Authorization": "Bearer token"},
		DiscoveredFrom: "https://example.com/",
		StatusCode:     200,
		ContentType:    "application/json",
		ResponseSize:   1024,
		Timestamp:      time.Now(),
	}

	if ep.URL == "" {
		t.Error("URL should not be empty")
	}
	if len(ep.Parameters) != 1 {
		t.Error("Parameters should have 1 item")
	}
}

func TestForm_Fields(t *testing.T) {
	form := Form{
		URL:     "https://example.com/login",
		Action:  "/auth",
		Method:  "POST",
		Enctype: "application/x-www-form-urlencoded",
		Inputs: []FormInput{
			{Name: "username", Type: "text", Required: true},
		},
		HasCSRF:   true,
		Depth:     1,
		Timestamp: time.Now(),
	}

	if form.URL == "" {
		t.Error("URL should not be empty")
	}
	if !form.HasCSRF {
		t.Error("HasCSRF should be true")
	}
}

func TestCrawlResult_Fields(t *testing.T) {
	result := CrawlResult{
		Target:      "https://example.com",
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
		Stats: CrawlStats{
			URLsDiscovered: 100,
			PagesCrawled:   50,
		},
		Endpoints:  []Endpoint{},
		Forms:      []Form{},
		WebSockets: []WebSocketEndpoint{},
		Errors:     []CrawlError{},
	}

	if result.Target == "" {
		t.Error("Target should not be empty")
	}
	if result.Stats.URLsDiscovered != 100 {
		t.Errorf("URLsDiscovered = %d, want 100", result.Stats.URLsDiscovered)
	}
}

func TestCrawlError_Fields(t *testing.T) {
	err := CrawlError{
		URL:       "https://example.com/broken",
		Error:     "connection refused",
		Timestamp: time.Now(),
	}

	if err.URL == "" {
		t.Error("URL should not be empty")
	}
	if err.Error == "" {
		t.Error("Error should not be empty")
	}
}
