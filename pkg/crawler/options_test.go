package crawler

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"github.com/PentesterFlow/OpenCrawler/internal/logger"
	"github.com/PentesterFlow/OpenCrawler/internal/metrics"
)

// Helper to create a minimal crawler for option testing
func newTestCrawler() *Crawler {
	return &Crawler{
		config: DefaultConfig(),
	}
}

// =============================================================================
// WithTarget Tests
// =============================================================================

func TestWithTarget(t *testing.T) {
	c := newTestCrawler()
	opt := WithTarget("https://example.com")
	err := opt(c)

	if err != nil {
		t.Fatalf("WithTarget() error = %v", err)
	}
	if c.config.Target != "https://example.com" {
		t.Errorf("Target = %s, want https://example.com", c.config.Target)
	}
}

// =============================================================================
// WithWorkers Tests
// =============================================================================

func TestWithWorkers(t *testing.T) {
	tests := []struct {
		name   string
		input  int
		expect int
	}{
		{"normal value", 100, 100},
		{"zero", 0, 1},
		{"negative", -5, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestCrawler()
			opt := WithWorkers(tt.input)
			err := opt(c)

			if err != nil {
				t.Fatalf("WithWorkers() error = %v", err)
			}
			if c.config.Workers != tt.expect {
				t.Errorf("Workers = %d, want %d", c.config.Workers, tt.expect)
			}
		})
	}
}

// =============================================================================
// WithMaxDepth Tests
// =============================================================================

func TestWithMaxDepth(t *testing.T) {
	tests := []struct {
		name   string
		input  int
		expect int
	}{
		{"normal value", 15, 15},
		{"zero", 0, 1},
		{"negative", -3, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestCrawler()
			opt := WithMaxDepth(tt.input)
			err := opt(c)

			if err != nil {
				t.Fatalf("WithMaxDepth() error = %v", err)
			}
			if c.config.MaxDepth != tt.expect {
				t.Errorf("MaxDepth = %d, want %d", c.config.MaxDepth, tt.expect)
			}
			if c.config.Scope.MaxDepth != tt.expect {
				t.Errorf("Scope.MaxDepth = %d, want %d", c.config.Scope.MaxDepth, tt.expect)
			}
		})
	}
}

// =============================================================================
// WithTimeout Tests
// =============================================================================

func TestWithTimeout(t *testing.T) {
	c := newTestCrawler()
	opt := WithTimeout(45 * time.Second)
	err := opt(c)

	if err != nil {
		t.Fatalf("WithTimeout() error = %v", err)
	}
	if c.config.Timeout != 45*time.Second {
		t.Errorf("Timeout = %v, want 45s", c.config.Timeout)
	}
}

// =============================================================================
// WithScope Tests
// =============================================================================

func TestWithScope(t *testing.T) {
	c := newTestCrawler()
	scope := ScopeRules{
		MaxDepth:        20,
		FollowExternal:  true,
		IncludePatterns: []string{`.*api.*`},
		ExcludePatterns: []string{`.*logout.*`},
		AllowedDomains:  []string{"trusted.com"},
	}
	opt := WithScope(scope)
	err := opt(c)

	if err != nil {
		t.Fatalf("WithScope() error = %v", err)
	}
	if c.config.Scope.MaxDepth != 20 {
		t.Errorf("Scope.MaxDepth = %d, want 20", c.config.Scope.MaxDepth)
	}
	if !c.config.Scope.FollowExternal {
		t.Error("Scope.FollowExternal should be true")
	}
	if len(c.config.Scope.IncludePatterns) != 1 {
		t.Errorf("IncludePatterns length = %d, want 1", len(c.config.Scope.IncludePatterns))
	}
}

// =============================================================================
// WithIncludePatterns Tests
// =============================================================================

func TestWithIncludePatterns(t *testing.T) {
	c := newTestCrawler()
	opt := WithIncludePatterns(`.*api.*`, `.*v1.*`)
	err := opt(c)

	if err != nil {
		t.Fatalf("WithIncludePatterns() error = %v", err)
	}
	if len(c.config.Scope.IncludePatterns) != 2 {
		t.Errorf("IncludePatterns length = %d, want 2", len(c.config.Scope.IncludePatterns))
	}
}

// =============================================================================
// WithExcludePatterns Tests
// =============================================================================

func TestWithExcludePatterns(t *testing.T) {
	c := newTestCrawler()
	opt := WithExcludePatterns(`.*logout.*`, `.*delete.*`)
	err := opt(c)

	if err != nil {
		t.Fatalf("WithExcludePatterns() error = %v", err)
	}
	if len(c.config.Scope.ExcludePatterns) != 2 {
		t.Errorf("ExcludePatterns length = %d, want 2", len(c.config.Scope.ExcludePatterns))
	}
}

// =============================================================================
// WithAllowedDomains Tests
// =============================================================================

func TestWithAllowedDomains(t *testing.T) {
	c := newTestCrawler()
	opt := WithAllowedDomains("trusted.com", "api.trusted.com")
	err := opt(c)

	if err != nil {
		t.Fatalf("WithAllowedDomains() error = %v", err)
	}
	if len(c.config.Scope.AllowedDomains) != 2 {
		t.Errorf("AllowedDomains length = %d, want 2", len(c.config.Scope.AllowedDomains))
	}
}

// =============================================================================
// WithFollowExternal Tests
// =============================================================================

func TestWithFollowExternal(t *testing.T) {
	c := newTestCrawler()
	opt := WithFollowExternal(true)
	err := opt(c)

	if err != nil {
		t.Fatalf("WithFollowExternal() error = %v", err)
	}
	if !c.config.Scope.FollowExternal {
		t.Error("FollowExternal should be true")
	}
}

// =============================================================================
// WithRateLimit Tests
// =============================================================================

func TestWithRateLimit(t *testing.T) {
	c := newTestCrawler()
	opt := WithRateLimit(200, 50)
	err := opt(c)

	if err != nil {
		t.Fatalf("WithRateLimit() error = %v", err)
	}
	if c.config.RateLimit.RequestsPerSecond != 200 {
		t.Errorf("RequestsPerSecond = %v, want 200", c.config.RateLimit.RequestsPerSecond)
	}
	if c.config.RateLimit.Burst != 50 {
		t.Errorf("Burst = %d, want 50", c.config.RateLimit.Burst)
	}
}

// =============================================================================
// WithRespectRobotsTxt Tests
// =============================================================================

func TestWithRespectRobotsTxt(t *testing.T) {
	c := newTestCrawler()
	opt := WithRespectRobotsTxt(false)
	err := opt(c)

	if err != nil {
		t.Fatalf("WithRespectRobotsTxt() error = %v", err)
	}
	if c.config.RateLimit.RespectRobotsTxt {
		t.Error("RespectRobotsTxt should be false")
	}
}

// =============================================================================
// WithBrowserPool Tests
// =============================================================================

func TestWithBrowserPool(t *testing.T) {
	tests := []struct {
		name   string
		input  int
		expect int
	}{
		{"normal value", 20, 20},
		{"zero", 0, 1},
		{"negative", -1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestCrawler()
			opt := WithBrowserPool(tt.input)
			err := opt(c)

			if err != nil {
				t.Fatalf("WithBrowserPool() error = %v", err)
			}
			if c.config.Browser.PoolSize != tt.expect {
				t.Errorf("Browser.PoolSize = %d, want %d", c.config.Browser.PoolSize, tt.expect)
			}
		})
	}
}

// =============================================================================
// WithHeadless Tests
// =============================================================================

func TestWithHeadless(t *testing.T) {
	c := newTestCrawler()
	opt := WithHeadless(false)
	err := opt(c)

	if err != nil {
		t.Fatalf("WithHeadless() error = %v", err)
	}
	if c.config.Browser.Headless {
		t.Error("Headless should be false")
	}
}

// =============================================================================
// WithUserAgent Tests
// =============================================================================

func TestWithUserAgent(t *testing.T) {
	c := newTestCrawler()
	opt := WithUserAgent("CustomBot/1.0")
	err := opt(c)

	if err != nil {
		t.Fatalf("WithUserAgent() error = %v", err)
	}
	if c.config.Browser.UserAgent != "CustomBot/1.0" {
		t.Errorf("UserAgent = %s, want CustomBot/1.0", c.config.Browser.UserAgent)
	}
}

// =============================================================================
// WithProxy Tests
// =============================================================================

func TestWithProxy(t *testing.T) {
	c := newTestCrawler()
	opt := WithProxy("http://proxy.example.com:8080")
	err := opt(c)

	if err != nil {
		t.Fatalf("WithProxy() error = %v", err)
	}
	if c.config.Proxy != "http://proxy.example.com:8080" {
		t.Errorf("Proxy = %s, want http://proxy.example.com:8080", c.config.Proxy)
	}
}

// =============================================================================
// WithAuth Tests
// =============================================================================

func TestWithAuth(t *testing.T) {
	c := newTestCrawler()
	auth := AuthCredentials{
		Type:     AuthTypeJWT,
		Token:    "test-token",
	}
	opt := WithAuth(auth)
	err := opt(c)

	if err != nil {
		t.Fatalf("WithAuth() error = %v", err)
	}
	if c.config.Auth.Type != AuthTypeJWT {
		t.Errorf("Auth.Type = %v, want jwt", c.config.Auth.Type)
	}
	if c.config.Auth.Token != "test-token" {
		t.Errorf("Auth.Token = %s, want test-token", c.config.Auth.Token)
	}
}

// =============================================================================
// WithFormAuth Tests
// =============================================================================

func TestWithFormAuth(t *testing.T) {
	c := newTestCrawler()
	auth := FormAuth{
		LoginURL:      "https://example.com/login",
		Username:      "admin",
		Password:      "password123",
		UsernameField: "user",
		PasswordField: "pass",
		ExtraFields:   map[string]string{"csrf": "token123"},
	}
	opt := WithFormAuth(auth)
	err := opt(c)

	if err != nil {
		t.Fatalf("WithFormAuth() error = %v", err)
	}
	if c.config.Auth.Type != AuthTypeFormLogin {
		t.Errorf("Auth.Type = %v, want form", c.config.Auth.Type)
	}
	if c.config.Auth.LoginURL != "https://example.com/login" {
		t.Errorf("Auth.LoginURL = %s, want https://example.com/login", c.config.Auth.LoginURL)
	}
	if c.config.Auth.FormFields["csrf"] != "token123" {
		t.Error("Extra field 'csrf' not set")
	}
}

// =============================================================================
// WithJWTAuth Tests
// =============================================================================

func TestWithJWTAuth(t *testing.T) {
	c := newTestCrawler()
	opt := WithJWTAuth("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...")
	err := opt(c)

	if err != nil {
		t.Fatalf("WithJWTAuth() error = %v", err)
	}
	if c.config.Auth.Type != AuthTypeJWT {
		t.Errorf("Auth.Type = %v, want jwt", c.config.Auth.Type)
	}
}

// =============================================================================
// WithBasicAuth Tests
// =============================================================================

func TestWithBasicAuth(t *testing.T) {
	c := newTestCrawler()
	opt := WithBasicAuth("admin", "secret")
	err := opt(c)

	if err != nil {
		t.Fatalf("WithBasicAuth() error = %v", err)
	}
	if c.config.Auth.Type != AuthTypeBasic {
		t.Errorf("Auth.Type = %v, want basic", c.config.Auth.Type)
	}
	if c.config.Auth.Username != "admin" {
		t.Errorf("Auth.Username = %s, want admin", c.config.Auth.Username)
	}
	if c.config.Auth.Password != "secret" {
		t.Errorf("Auth.Password = %s, want secret", c.config.Auth.Password)
	}
}

// =============================================================================
// WithAPIKeyAuth Tests
// =============================================================================

func TestWithAPIKeyAuth(t *testing.T) {
	c := newTestCrawler()
	opt := WithAPIKeyAuth("X-API-Key", "apikey123")
	err := opt(c)

	if err != nil {
		t.Fatalf("WithAPIKeyAuth() error = %v", err)
	}
	if c.config.Auth.Type != AuthTypeAPIKey {
		t.Errorf("Auth.Type = %v, want apikey", c.config.Auth.Type)
	}
	if c.config.Auth.Headers["X-API-Key"] != "apikey123" {
		t.Error("API key header not set correctly")
	}
}

// =============================================================================
// WithCookies Tests
// =============================================================================

func TestWithCookies(t *testing.T) {
	c := newTestCrawler()
	cookies := []*http.Cookie{
		{Name: "session", Value: "abc123"},
		{Name: "token", Value: "xyz789"},
	}
	opt := WithCookies(cookies)
	err := opt(c)

	if err != nil {
		t.Fatalf("WithCookies() error = %v", err)
	}
	if len(c.config.Auth.Cookies) != 2 {
		t.Errorf("Auth.Cookies length = %d, want 2", len(c.config.Auth.Cookies))
	}
	if c.config.Auth.Type != AuthTypeSession {
		t.Errorf("Auth.Type = %v, want session", c.config.Auth.Type)
	}
}

// =============================================================================
// WithCustomHeaders Tests
// =============================================================================

func TestWithCustomHeaders(t *testing.T) {
	c := newTestCrawler()
	headers := map[string]string{
		"X-Custom-1": "value1",
		"X-Custom-2": "value2",
	}
	opt := WithCustomHeaders(headers)
	err := opt(c)

	if err != nil {
		t.Fatalf("WithCustomHeaders() error = %v", err)
	}
	if c.config.CustomHeaders["X-Custom-1"] != "value1" {
		t.Error("Custom header X-Custom-1 not set")
	}
	if c.config.CustomHeaders["X-Custom-2"] != "value2" {
		t.Error("Custom header X-Custom-2 not set")
	}
}

// =============================================================================
// WithOutput Tests
// =============================================================================

func TestWithOutput(t *testing.T) {
	c := newTestCrawler()
	var buf bytes.Buffer
	opt := WithOutput(&buf)
	err := opt(c)

	if err != nil {
		t.Fatalf("WithOutput() error = %v", err)
	}
	if c.outputWriter != &buf {
		t.Error("outputWriter not set correctly")
	}
}

// =============================================================================
// WithOutputFile Tests
// =============================================================================

func TestWithOutputFile(t *testing.T) {
	c := newTestCrawler()
	opt := WithOutputFile("/tmp/output.json")
	err := opt(c)

	if err != nil {
		t.Fatalf("WithOutputFile() error = %v", err)
	}
	if c.config.Output.FilePath != "/tmp/output.json" {
		t.Errorf("Output.FilePath = %s, want /tmp/output.json", c.config.Output.FilePath)
	}
}

// =============================================================================
// WithPrettyOutput Tests
// =============================================================================

func TestWithPrettyOutput(t *testing.T) {
	c := newTestCrawler()
	opt := WithPrettyOutput(false)
	err := opt(c)

	if err != nil {
		t.Fatalf("WithPrettyOutput() error = %v", err)
	}
	if c.config.Output.Pretty {
		t.Error("Output.Pretty should be false")
	}
}

// =============================================================================
// WithStreamMode Tests
// =============================================================================

func TestWithStreamMode(t *testing.T) {
	c := newTestCrawler()
	opt := WithStreamMode(true)
	err := opt(c)

	if err != nil {
		t.Fatalf("WithStreamMode() error = %v", err)
	}
	if !c.config.Output.StreamMode {
		t.Error("Output.StreamMode should be true")
	}
}

// =============================================================================
// WithStateFile Tests
// =============================================================================

func TestWithStateFile(t *testing.T) {
	c := newTestCrawler()
	opt := WithStateFile("/tmp/state.db")
	err := opt(c)

	if err != nil {
		t.Fatalf("WithStateFile() error = %v", err)
	}
	if c.config.State.FilePath != "/tmp/state.db" {
		t.Errorf("State.FilePath = %s, want /tmp/state.db", c.config.State.FilePath)
	}
	if !c.config.State.Enabled {
		t.Error("State.Enabled should be true")
	}
}

// =============================================================================
// WithAutoSave Tests
// =============================================================================

func TestWithAutoSave(t *testing.T) {
	c := newTestCrawler()
	opt := WithAutoSave(true, 120)
	err := opt(c)

	if err != nil {
		t.Fatalf("WithAutoSave() error = %v", err)
	}
	if !c.config.State.AutoSave {
		t.Error("State.AutoSave should be true")
	}
	if c.config.State.Interval != 120 {
		t.Errorf("State.Interval = %d, want 120", c.config.State.Interval)
	}
}

// =============================================================================
// Feature Toggle Tests
// =============================================================================

func TestWithPassiveDiscovery(t *testing.T) {
	c := newTestCrawler()
	opt := WithPassiveDiscovery(false)
	opt(c)
	if c.config.PassiveAPIDiscovery {
		t.Error("PassiveAPIDiscovery should be false")
	}
}

func TestWithActiveDiscovery(t *testing.T) {
	c := newTestCrawler()
	opt := WithActiveDiscovery(false)
	opt(c)
	if c.config.ActiveAPIDiscovery {
		t.Error("ActiveAPIDiscovery should be false")
	}
}

func TestWithWebSocketDiscovery(t *testing.T) {
	c := newTestCrawler()
	opt := WithWebSocketDiscovery(false)
	opt(c)
	if c.config.WebSocketDiscovery {
		t.Error("WebSocketDiscovery should be false")
	}
}

func TestWithFormAnalysis(t *testing.T) {
	c := newTestCrawler()
	opt := WithFormAnalysis(false)
	opt(c)
	if c.config.FormAnalysis {
		t.Error("FormAnalysis should be false")
	}
}

func TestWithJSAnalysis(t *testing.T) {
	c := newTestCrawler()
	opt := WithJSAnalysis(false)
	opt(c)
	if c.config.JSAnalysis {
		t.Error("JSAnalysis should be false")
	}
}

func TestWithVerbose(t *testing.T) {
	c := newTestCrawler()
	opt := WithVerbose(true)
	opt(c)
	if !c.config.Verbose {
		t.Error("Verbose should be true")
	}
}

func TestWithDebug(t *testing.T) {
	c := newTestCrawler()
	opt := WithDebug(true)
	opt(c)
	if !c.config.Debug {
		t.Error("Debug should be true")
	}
}

// =============================================================================
// WithConfig Tests
// =============================================================================

func TestWithConfig(t *testing.T) {
	c := newTestCrawler()
	newConfig := TurboConfig()
	newConfig.Target = "https://fast.example.com"

	opt := WithConfig(newConfig)
	err := opt(c)

	if err != nil {
		t.Fatalf("WithConfig() error = %v", err)
	}
	if c.config.Target != "https://fast.example.com" {
		t.Errorf("Target = %s, want https://fast.example.com", c.config.Target)
	}
	if c.config.Workers != 200 {
		t.Errorf("Workers = %d, want 200 (turbo config)", c.config.Workers)
	}
}

// =============================================================================
// WithLogger Tests
// =============================================================================

func TestWithLogger(t *testing.T) {
	c := newTestCrawler()
	customLogger := logger.New(logger.Config{
		Level:     logger.DebugLevel,
		Pretty:    false,
		Component: "test",
	})

	opt := WithLogger(customLogger)
	err := opt(c)

	if err != nil {
		t.Fatalf("WithLogger() error = %v", err)
	}
	if c.logger != customLogger {
		t.Error("logger was not set correctly")
	}
}

func TestWithLogLevel(t *testing.T) {
	c := newTestCrawler()
	// First set up a logger
	c.logger = logger.NewDefault()

	opt := WithLogLevel(logger.DebugLevel)
	err := opt(c)

	if err != nil {
		t.Fatalf("WithLogLevel() error = %v", err)
	}
	// Logger level is set internally, can't easily verify without exposing it
}

func TestWithLogLevel_NilLogger(t *testing.T) {
	c := newTestCrawler()
	c.logger = nil

	opt := WithLogLevel(logger.DebugLevel)
	err := opt(c)

	// Should not error even with nil logger
	if err != nil {
		t.Fatalf("WithLogLevel() with nil logger error = %v", err)
	}
}

// =============================================================================
// WithMetrics Tests
// =============================================================================

func TestWithMetrics(t *testing.T) {
	c := newTestCrawler()
	customMetrics := metrics.New()

	opt := WithMetrics(customMetrics)
	err := opt(c)

	if err != nil {
		t.Fatalf("WithMetrics() error = %v", err)
	}
	if c.metrics != customMetrics {
		t.Error("metrics was not set correctly")
	}
}
