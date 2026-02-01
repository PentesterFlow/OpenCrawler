package crawler

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

// =============================================================================
// Endpoint Type Tests
// =============================================================================

func TestEndpoint_JSON(t *testing.T) {
	ep := Endpoint{
		URL:            "https://example.com/api/users",
		Method:         "GET",
		Source:         "passive",
		Depth:          2,
		Parameters:     []Parameter{{Name: "id", Type: "path", Required: true}},
		Headers:        map[string]string{"Content-Type": "application/json"},
		DiscoveredFrom: "https://example.com/",
		StatusCode:     200,
		ContentType:    "application/json",
		ResponseSize:   1024,
		Timestamp:      time.Now(),
	}

	data, err := json.Marshal(ep)
	if err != nil {
		t.Fatalf("Failed to marshal Endpoint: %v", err)
	}

	var decoded Endpoint
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal Endpoint: %v", err)
	}

	if decoded.URL != ep.URL {
		t.Errorf("URL = %s, want %s", decoded.URL, ep.URL)
	}
	if decoded.Method != ep.Method {
		t.Errorf("Method = %s, want %s", decoded.Method, ep.Method)
	}
	if len(decoded.Parameters) != 1 {
		t.Errorf("Parameters length = %d, want 1", len(decoded.Parameters))
	}
}

// =============================================================================
// Parameter Type Tests
// =============================================================================

func TestParameter_JSON(t *testing.T) {
	param := Parameter{
		Name:     "userId",
		Type:     "path",
		Example:  "123",
		Required: true,
	}

	data, err := json.Marshal(param)
	if err != nil {
		t.Fatalf("Failed to marshal Parameter: %v", err)
	}

	var decoded Parameter
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal Parameter: %v", err)
	}

	if decoded.Name != param.Name {
		t.Errorf("Name = %s, want %s", decoded.Name, param.Name)
	}
	if decoded.Type != param.Type {
		t.Errorf("Type = %s, want %s", decoded.Type, param.Type)
	}
	if !decoded.Required {
		t.Error("Required should be true")
	}
}

// =============================================================================
// Form Type Tests
// =============================================================================

func TestForm_JSON(t *testing.T) {
	form := Form{
		URL:     "https://example.com/login",
		Action:  "/auth/login",
		Method:  "POST",
		Enctype: "application/x-www-form-urlencoded",
		Inputs: []FormInput{
			{Name: "username", Type: "text", Required: true, Placeholder: "Enter username"},
			{Name: "password", Type: "password", Required: true},
			{Name: "_csrf", Type: "hidden", Value: "token123"},
		},
		HasCSRF:   true,
		Depth:     1,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(form)
	if err != nil {
		t.Fatalf("Failed to marshal Form: %v", err)
	}

	var decoded Form
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal Form: %v", err)
	}

	if decoded.URL != form.URL {
		t.Errorf("URL = %s, want %s", decoded.URL, form.URL)
	}
	if len(decoded.Inputs) != 3 {
		t.Errorf("Inputs length = %d, want 3", len(decoded.Inputs))
	}
	if !decoded.HasCSRF {
		t.Error("HasCSRF should be true")
	}
}

// =============================================================================
// FormInput Type Tests
// =============================================================================

func TestFormInput_JSON(t *testing.T) {
	input := FormInput{
		Name:        "email",
		Type:        "email",
		Value:       "",
		Required:    true,
		Placeholder: "your@email.com",
		Pattern:     "[a-z0-9._%+-]+@[a-z0-9.-]+\\.[a-z]{2,}$",
		MaxLength:   100,
		MinLength:   5,
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal FormInput: %v", err)
	}

	var decoded FormInput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal FormInput: %v", err)
	}

	if decoded.Name != input.Name {
		t.Errorf("Name = %s, want %s", decoded.Name, input.Name)
	}
	if decoded.MaxLength != 100 {
		t.Errorf("MaxLength = %d, want 100", decoded.MaxLength)
	}
}

// =============================================================================
// WebSocketEndpoint Type Tests
// =============================================================================

func TestWebSocketEndpoint_JSON(t *testing.T) {
	ws := WebSocketEndpoint{
		URL:            "wss://example.com/ws",
		DiscoveredFrom: "https://example.com/dashboard",
		SampleMessages: []WebSocketMsg{
			{Direction: "sent", Type: "text", Data: `{"type":"ping"}`, Timestamp: time.Now()},
			{Direction: "received", Type: "text", Data: `{"type":"pong"}`, Timestamp: time.Now()},
		},
		Protocols: []string{"graphql-ws"},
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(ws)
	if err != nil {
		t.Fatalf("Failed to marshal WebSocketEndpoint: %v", err)
	}

	var decoded WebSocketEndpoint
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal WebSocketEndpoint: %v", err)
	}

	if decoded.URL != ws.URL {
		t.Errorf("URL = %s, want %s", decoded.URL, ws.URL)
	}
	if len(decoded.SampleMessages) != 2 {
		t.Errorf("SampleMessages length = %d, want 2", len(decoded.SampleMessages))
	}
}

// =============================================================================
// WebSocketMsg Type Tests
// =============================================================================

func TestWebSocketMsg_JSON(t *testing.T) {
	msg := WebSocketMsg{
		Direction: "sent",
		Type:      "binary",
		Data:      "base64encodeddata",
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal WebSocketMsg: %v", err)
	}

	var decoded WebSocketMsg
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal WebSocketMsg: %v", err)
	}

	if decoded.Direction != msg.Direction {
		t.Errorf("Direction = %s, want %s", decoded.Direction, msg.Direction)
	}
}

// =============================================================================
// CrawlResult Type Tests
// =============================================================================

func TestCrawlResult_JSON(t *testing.T) {
	result := CrawlResult{
		Target:      "https://example.com",
		StartedAt:   time.Now().Add(-5 * time.Minute),
		CompletedAt: time.Now(),
		Stats: CrawlStats{
			URLsDiscovered:     500,
			PagesCrawled:       250,
			FormsFound:         15,
			APIEndpoints:       50,
			WebSocketEndpoints: 2,
			ErrorCount:         10,
			Duration:           5 * time.Minute,
			BytesTransferred:   1024 * 1024,
		},
		Endpoints: []Endpoint{
			{URL: "https://example.com/api/v1/users", Method: "GET"},
		},
		Forms: []Form{
			{URL: "https://example.com/login", Action: "/login", Method: "POST"},
		},
		WebSockets: []WebSocketEndpoint{
			{URL: "wss://example.com/ws"},
		},
		Technologies: []Technology{
			{Name: "React", Category: "javascript-framework", Version: "18.2", Confidence: 95},
		},
		Secrets: []SecretFinding{
			{Type: "api_key", Value: "REDACTED", File: "config.js"},
		},
		Errors: []CrawlError{
			{URL: "https://example.com/broken", Error: "404 Not Found"},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal CrawlResult: %v", err)
	}

	var decoded CrawlResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal CrawlResult: %v", err)
	}

	if decoded.Target != result.Target {
		t.Errorf("Target = %s, want %s", decoded.Target, result.Target)
	}
	if decoded.Stats.URLsDiscovered != 500 {
		t.Errorf("URLsDiscovered = %d, want 500", decoded.Stats.URLsDiscovered)
	}
	if len(decoded.Endpoints) != 1 {
		t.Errorf("Endpoints length = %d, want 1", len(decoded.Endpoints))
	}
	if len(decoded.Technologies) != 1 {
		t.Errorf("Technologies length = %d, want 1", len(decoded.Technologies))
	}
}

// =============================================================================
// Technology Type Tests
// =============================================================================

func TestTechnology_JSON(t *testing.T) {
	tech := Technology{
		Name:       "Next.js",
		Category:   "web-framework",
		Version:    "14.0",
		Confidence: 90,
		Evidence:   "__NEXT_DATA__ found in page",
	}

	data, err := json.Marshal(tech)
	if err != nil {
		t.Fatalf("Failed to marshal Technology: %v", err)
	}

	var decoded Technology
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal Technology: %v", err)
	}

	if decoded.Name != tech.Name {
		t.Errorf("Name = %s, want %s", decoded.Name, tech.Name)
	}
	if decoded.Confidence != 90 {
		t.Errorf("Confidence = %d, want 90", decoded.Confidence)
	}
}

// =============================================================================
// SecretFinding Type Tests
// =============================================================================

func TestSecretFinding_JSON(t *testing.T) {
	secret := SecretFinding{
		Type:    "aws_access_key",
		Value:   "AKIA***REDACTED***",
		File:    "bundle.js",
		Context: "const apiKey = 'AKIA...'",
	}

	data, err := json.Marshal(secret)
	if err != nil {
		t.Fatalf("Failed to marshal SecretFinding: %v", err)
	}

	var decoded SecretFinding
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal SecretFinding: %v", err)
	}

	if decoded.Type != secret.Type {
		t.Errorf("Type = %s, want %s", decoded.Type, secret.Type)
	}
}

// =============================================================================
// CrawlStats Type Tests
// =============================================================================

func TestCrawlStats_JSON(t *testing.T) {
	stats := CrawlStats{
		URLsDiscovered:     1000,
		PagesCrawled:       500,
		FormsFound:         25,
		APIEndpoints:       100,
		WebSocketEndpoints: 5,
		ErrorCount:         20,
		Duration:           10 * time.Minute,
		BytesTransferred:   10 * 1024 * 1024,
	}

	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("Failed to marshal CrawlStats: %v", err)
	}

	var decoded CrawlStats
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal CrawlStats: %v", err)
	}

	if decoded.URLsDiscovered != 1000 {
		t.Errorf("URLsDiscovered = %d, want 1000", decoded.URLsDiscovered)
	}
	if decoded.BytesTransferred != 10*1024*1024 {
		t.Errorf("BytesTransferred = %d, want %d", decoded.BytesTransferred, 10*1024*1024)
	}
}

// =============================================================================
// CrawlError Type Tests
// =============================================================================

func TestCrawlError_JSON(t *testing.T) {
	crawlErr := CrawlError{
		URL:       "https://example.com/error",
		Error:     "connection timeout",
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(crawlErr)
	if err != nil {
		t.Fatalf("Failed to marshal CrawlError: %v", err)
	}

	var decoded CrawlError
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal CrawlError: %v", err)
	}

	if decoded.URL != crawlErr.URL {
		t.Errorf("URL = %s, want %s", decoded.URL, crawlErr.URL)
	}
	if decoded.Error != "connection timeout" {
		t.Errorf("Error = %s, want connection timeout", decoded.Error)
	}
}

// =============================================================================
// QueueItem Type Tests
// =============================================================================

func TestQueueItem_JSON(t *testing.T) {
	item := QueueItem{
		URL:       "https://example.com/page",
		Method:    "GET",
		Depth:     3,
		ParentURL: "https://example.com/",
		Headers:   map[string]string{"Authorization": "Bearer token"},
		Body:      []byte(`{"key":"value"}`),
		Priority:  1,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("Failed to marshal QueueItem: %v", err)
	}

	var decoded QueueItem
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal QueueItem: %v", err)
	}

	if decoded.URL != item.URL {
		t.Errorf("URL = %s, want %s", decoded.URL, item.URL)
	}
	if decoded.Depth != 3 {
		t.Errorf("Depth = %d, want 3", decoded.Depth)
	}
}

// =============================================================================
// AuthCredentials Type Tests
// =============================================================================

func TestAuthCredentials_JSON(t *testing.T) {
	auth := AuthCredentials{
		Type:       AuthTypeFormLogin,
		Username:   "admin",
		Password:   "secret",
		Token:      "",
		Headers:    map[string]string{"X-Custom": "value"},
		Cookies:    []*http.Cookie{{Name: "session", Value: "abc123"}},
		LoginURL:   "https://example.com/login",
		FormFields: map[string]string{"csrf": "token"},
		OAuthConfig: &OAuthConfig{
			ClientID:     "client123",
			ClientSecret: "secret456",
			AuthURL:      "https://auth.example.com/authorize",
			TokenURL:     "https://auth.example.com/token",
			RedirectURL:  "https://app.example.com/callback",
			Scopes:       []string{"read", "write"},
		},
	}

	data, err := json.Marshal(auth)
	if err != nil {
		t.Fatalf("Failed to marshal AuthCredentials: %v", err)
	}

	var decoded AuthCredentials
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal AuthCredentials: %v", err)
	}

	if decoded.Type != AuthTypeFormLogin {
		t.Errorf("Type = %v, want form", decoded.Type)
	}
	if decoded.Username != "admin" {
		t.Errorf("Username = %s, want admin", decoded.Username)
	}
	if decoded.OAuthConfig == nil {
		t.Fatal("OAuthConfig should not be nil")
	}
	if decoded.OAuthConfig.ClientID != "client123" {
		t.Errorf("ClientID = %s, want client123", decoded.OAuthConfig.ClientID)
	}
}

// =============================================================================
// OAuthConfig Type Tests
// =============================================================================

func TestOAuthConfig_JSON(t *testing.T) {
	config := OAuthConfig{
		ClientID:     "my-client-id",
		ClientSecret: "my-client-secret",
		AuthURL:      "https://oauth.example.com/authorize",
		TokenURL:     "https://oauth.example.com/token",
		RedirectURL:  "https://myapp.example.com/callback",
		Scopes:       []string{"openid", "profile", "email"},
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal OAuthConfig: %v", err)
	}

	var decoded OAuthConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal OAuthConfig: %v", err)
	}

	if decoded.ClientID != config.ClientID {
		t.Errorf("ClientID = %s, want %s", decoded.ClientID, config.ClientID)
	}
	if len(decoded.Scopes) != 3 {
		t.Errorf("Scopes length = %d, want 3", len(decoded.Scopes))
	}
}

// =============================================================================
// ScopeRules Type Tests
// =============================================================================

func TestScopeRules_JSON(t *testing.T) {
	rules := ScopeRules{
		IncludePatterns: []string{`.*api.*`, `.*v1.*`},
		ExcludePatterns: []string{`.*logout.*`, `.*admin.*`},
		AllowedDomains:  []string{"example.com", "api.example.com"},
		MaxDepth:        15,
		FollowExternal:  false,
	}

	data, err := json.Marshal(rules)
	if err != nil {
		t.Fatalf("Failed to marshal ScopeRules: %v", err)
	}

	var decoded ScopeRules
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal ScopeRules: %v", err)
	}

	if decoded.MaxDepth != 15 {
		t.Errorf("MaxDepth = %d, want 15", decoded.MaxDepth)
	}
	if len(decoded.IncludePatterns) != 2 {
		t.Errorf("IncludePatterns length = %d, want 2", len(decoded.IncludePatterns))
	}
}

// =============================================================================
// RateLimitConfig Type Tests
// =============================================================================

func TestRateLimitConfig_JSON(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerSecond: 100.5,
		Burst:             20,
		DelayBetween:      50 * time.Millisecond,
		RespectRobotsTxt:  true,
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal RateLimitConfig: %v", err)
	}

	var decoded RateLimitConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal RateLimitConfig: %v", err)
	}

	if decoded.RequestsPerSecond != 100.5 {
		t.Errorf("RequestsPerSecond = %v, want 100.5", decoded.RequestsPerSecond)
	}
	if decoded.Burst != 20 {
		t.Errorf("Burst = %d, want 20", decoded.Burst)
	}
}

// =============================================================================
// OutputConfig Type Tests
// =============================================================================

func TestOutputConfig_JSON(t *testing.T) {
	config := OutputConfig{
		Format:     "json",
		FilePath:   "/tmp/output.json",
		Pretty:     true,
		StreamMode: false,
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal OutputConfig: %v", err)
	}

	var decoded OutputConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal OutputConfig: %v", err)
	}

	if decoded.Format != "json" {
		t.Errorf("Format = %s, want json", decoded.Format)
	}
	if !decoded.Pretty {
		t.Error("Pretty should be true")
	}
}

// =============================================================================
// StateConfig Type Tests
// =============================================================================

func TestStateConfig_JSON(t *testing.T) {
	config := StateConfig{
		Enabled:  true,
		FilePath: "/tmp/state.db",
		AutoSave: true,
		Interval: 60,
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal StateConfig: %v", err)
	}

	var decoded StateConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal StateConfig: %v", err)
	}

	if !decoded.Enabled {
		t.Error("Enabled should be true")
	}
	if decoded.Interval != 60 {
		t.Errorf("Interval = %d, want 60", decoded.Interval)
	}
}
