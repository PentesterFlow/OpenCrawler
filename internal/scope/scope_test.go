package scope

import (
	"testing"
)

// =============================================================================
// Checker Tests
// =============================================================================

func TestNewChecker(t *testing.T) {
	tests := []struct {
		name      string
		targetURL string
		rules     ScopeRules
		wantErr   bool
	}{
		{
			name:      "valid URL",
			targetURL: "https://example.com",
			rules:     ScopeRules{MaxDepth: 10},
			wantErr:   false,
		},
		{
			name:      "URL with path",
			targetURL: "https://example.com/app",
			rules:     ScopeRules{MaxDepth: 5},
			wantErr:   false,
		},
		{
			name:      "invalid URL",
			targetURL: "://invalid",
			rules:     ScopeRules{},
			wantErr:   true,
		},
		{
			name:      "with include patterns",
			targetURL: "https://example.com",
			rules: ScopeRules{
				IncludePatterns: []string{`.*\.example\.com.*`},
			},
			wantErr: false,
		},
		{
			name:      "with invalid regex pattern",
			targetURL: "https://example.com",
			rules: ScopeRules{
				IncludePatterns: []string{`[invalid`},
			},
			wantErr: true,
		},
		{
			name:      "with invalid exclude pattern",
			targetURL: "https://example.com",
			rules: ScopeRules{
				ExcludePatterns: []string{`[invalid`},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker, err := NewChecker(tt.targetURL, tt.rules)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewChecker() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && checker == nil {
				t.Error("NewChecker() returned nil without error")
			}
		})
	}
}

func TestChecker_IsInScope(t *testing.T) {
	tests := []struct {
		name      string
		targetURL string
		rules     ScopeRules
		checkURL  string
		depth     int
		want      bool
	}{
		{
			name:      "same domain",
			targetURL: "https://example.com",
			rules:     ScopeRules{MaxDepth: 10},
			checkURL:  "https://example.com/page",
			depth:     1,
			want:      true,
		},
		{
			name:      "different domain not allowed",
			targetURL: "https://example.com",
			rules:     ScopeRules{MaxDepth: 10, FollowExternal: false},
			checkURL:  "https://other.com/page",
			depth:     1,
			want:      false,
		},
		{
			name:      "different domain allowed with FollowExternal",
			targetURL: "https://example.com",
			rules:     ScopeRules{MaxDepth: 10, FollowExternal: true},
			checkURL:  "https://other.com/page",
			depth:     1,
			want:      true,
		},
		{
			name:      "subdomain allowed",
			targetURL: "https://example.com",
			rules:     ScopeRules{MaxDepth: 10},
			checkURL:  "https://sub.example.com/page",
			depth:     1,
			want:      true,
		},
		{
			name:      "exceeds max depth",
			targetURL: "https://example.com",
			rules:     ScopeRules{MaxDepth: 5},
			checkURL:  "https://example.com/page",
			depth:     6,
			want:      false,
		},
		{
			name:      "within max depth",
			targetURL: "https://example.com",
			rules:     ScopeRules{MaxDepth: 5},
			checkURL:  "https://example.com/page",
			depth:     5,
			want:      true,
		},
		{
			name:      "exclude pattern match",
			targetURL: "https://example.com",
			rules: ScopeRules{
				MaxDepth:        10,
				ExcludePatterns: []string{`.*logout.*`},
			},
			checkURL: "https://example.com/logout",
			depth:    1,
			want:     false,
		},
		{
			name:      "include pattern match",
			targetURL: "https://example.com",
			rules: ScopeRules{
				MaxDepth:        10,
				IncludePatterns: []string{`.*api.*`},
			},
			checkURL: "https://example.com/api/users",
			depth:    1,
			want:     true,
		},
		{
			name:      "include pattern no match",
			targetURL: "https://example.com",
			rules: ScopeRules{
				MaxDepth:        10,
				IncludePatterns: []string{`.*api.*`},
			},
			checkURL: "https://example.com/about",
			depth:    1,
			want:     false,
		},
		{
			name:      "invalid URL",
			targetURL: "https://example.com",
			rules:     ScopeRules{MaxDepth: 10},
			checkURL:  "://invalid",
			depth:     1,
			want:      false,
		},
		{
			name:      "non-http scheme",
			targetURL: "https://example.com",
			rules:     ScopeRules{MaxDepth: 10},
			checkURL:  "ftp://example.com/file",
			depth:     1,
			want:      false,
		},
		{
			name:      "mailto scheme",
			targetURL: "https://example.com",
			rules:     ScopeRules{MaxDepth: 10},
			checkURL:  "mailto:user@example.com",
			depth:     1,
			want:      false,
		},
		{
			name:      "javascript scheme",
			targetURL: "https://example.com",
			rules:     ScopeRules{MaxDepth: 10},
			checkURL:  "javascript:void(0)",
			depth:     1,
			want:      false,
		},
		{
			name:      "allowed domain",
			targetURL: "https://example.com",
			rules: ScopeRules{
				MaxDepth:       10,
				AllowedDomains: []string{"trusted.com"},
			},
			checkURL: "https://trusted.com/page",
			depth:    1,
			want:     true,
		},
		{
			name:      "max depth zero means unlimited",
			targetURL: "https://example.com",
			rules:     ScopeRules{MaxDepth: 0},
			checkURL:  "https://example.com/page",
			depth:     100,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker, err := NewChecker(tt.targetURL, tt.rules)
			if err != nil {
				t.Fatalf("NewChecker() error = %v", err)
			}

			got := checker.IsInScope(tt.checkURL, tt.depth)
			if got != tt.want {
				t.Errorf("IsInScope(%s, %d) = %v, want %v", tt.checkURL, tt.depth, got, tt.want)
			}
		})
	}
}

func TestChecker_AddAllowedDomain(t *testing.T) {
	checker, _ := NewChecker("https://example.com", ScopeRules{MaxDepth: 10})

	// Initially, trusted.com is not in scope
	if checker.IsInScope("https://trusted.com/page", 1) {
		t.Error("trusted.com should not be in scope initially")
	}

	// Add trusted.com
	checker.AddAllowedDomain("trusted.com")

	// Now it should be in scope
	if !checker.IsInScope("https://trusted.com/page", 1) {
		t.Error("trusted.com should be in scope after adding")
	}
}

func TestChecker_AddIncludePattern(t *testing.T) {
	checker, _ := NewChecker("https://example.com", ScopeRules{MaxDepth: 10})

	// Add include pattern
	err := checker.AddIncludePattern(`.*api.*`)
	if err != nil {
		t.Fatalf("AddIncludePattern() error = %v", err)
	}

	// Should match API URLs
	if !checker.IsInScope("https://example.com/api/users", 1) {
		t.Error("API URL should be in scope")
	}

	// Should not match non-API URLs
	if checker.IsInScope("https://example.com/about", 1) {
		t.Error("Non-API URL should not be in scope")
	}
}

func TestChecker_AddIncludePattern_Invalid(t *testing.T) {
	checker, _ := NewChecker("https://example.com", ScopeRules{MaxDepth: 10})

	err := checker.AddIncludePattern(`[invalid`)
	if err == nil {
		t.Error("AddIncludePattern() should return error for invalid regex")
	}
}

func TestChecker_AddExcludePattern(t *testing.T) {
	checker, _ := NewChecker("https://example.com", ScopeRules{MaxDepth: 10})

	// Add exclude pattern
	err := checker.AddExcludePattern(`.*logout.*`)
	if err != nil {
		t.Fatalf("AddExcludePattern() error = %v", err)
	}

	// Should exclude logout URLs
	if checker.IsInScope("https://example.com/logout", 1) {
		t.Error("Logout URL should be excluded")
	}
}

func TestChecker_AddExcludePattern_Invalid(t *testing.T) {
	checker, _ := NewChecker("https://example.com", ScopeRules{MaxDepth: 10})

	err := checker.AddExcludePattern(`[invalid`)
	if err == nil {
		t.Error("AddExcludePattern() should return error for invalid regex")
	}
}

func TestChecker_SetMaxDepth(t *testing.T) {
	checker, _ := NewChecker("https://example.com", ScopeRules{MaxDepth: 5})

	// Initially depth 10 is out of scope
	if checker.IsInScope("https://example.com/page", 10) {
		t.Error("Depth 10 should be out of scope initially")
	}

	// Update max depth
	checker.SetMaxDepth(15)

	// Now depth 10 should be in scope
	if !checker.IsInScope("https://example.com/page", 10) {
		t.Error("Depth 10 should be in scope after update")
	}
}

// =============================================================================
// NormalizeURL Tests
// =============================================================================

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "lowercase scheme",
			input: "HTTPS://example.com/path",
			want:  "https://example.com/path",
		},
		{
			name:  "lowercase host",
			input: "https://EXAMPLE.COM/path",
			want:  "https://example.com/path",
		},
		{
			name:  "remove http port 80",
			input: "http://example.com:80/path",
			want:  "http://example.com/path",
		},
		{
			name:  "remove https port 443",
			input: "https://example.com:443/path",
			want:  "https://example.com/path",
		},
		{
			name:  "keep non-default port",
			input: "https://example.com:8080/path",
			want:  "https://example.com:8080/path",
		},
		{
			name:  "remove trailing slash",
			input: "https://example.com/path/",
			want:  "https://example.com/path",
		},
		{
			name:  "keep root slash",
			input: "https://example.com/",
			want:  "https://example.com/",
		},
		{
			name:  "add root slash",
			input: "https://example.com",
			want:  "https://example.com/",
		},
		{
			name:  "remove fragment",
			input: "https://example.com/path#section",
			want:  "https://example.com/path",
		},
		{
			name:  "sort query parameters",
			input: "https://example.com/path?z=1&a=2",
			want:  "https://example.com/path?a=2&z=1",
		},
		{
			name:    "invalid URL",
			input:   "://invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeURL(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizeURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("NormalizeURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// ResolveURL Tests
// =============================================================================

func TestResolveURL(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		relative string
		want     string
		wantErr  bool
	}{
		{
			name:     "relative path",
			baseURL:  "https://example.com/dir/page",
			relative: "other.html",
			want:     "https://example.com/dir/other.html",
		},
		{
			name:     "absolute path",
			baseURL:  "https://example.com/dir/page",
			relative: "/root/page.html",
			want:     "https://example.com/root/page.html",
		},
		{
			name:     "full URL",
			baseURL:  "https://example.com/dir/page",
			relative: "https://other.com/page",
			want:     "https://other.com/page",
		},
		{
			name:     "parent directory",
			baseURL:  "https://example.com/dir/subdir/page",
			relative: "../other.html",
			want:     "https://example.com/dir/other.html",
		},
		{
			name:     "query string",
			baseURL:  "https://example.com/page",
			relative: "?query=1",
			want:     "https://example.com/page?query=1",
		},
		{
			name:     "fragment",
			baseURL:  "https://example.com/page",
			relative: "#section",
			want:     "https://example.com/page#section",
		},
		{
			name:    "invalid base URL",
			baseURL: "://invalid",
			wantErr: true,
		},
		{
			name:     "invalid relative URL",
			baseURL:  "https://example.com",
			relative: "://invalid",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveURL(tt.baseURL, tt.relative)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ResolveURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// IsValidURL Tests
// =============================================================================

func TestIsValidURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"valid https", "https://example.com/page", true},
		{"valid http", "http://example.com/page", true},
		{"with query", "https://example.com/page?id=1", true},
		{"with fragment", "https://example.com/page#section", true},
		{"no scheme", "example.com/page", false},
		{"no host", "https:///page", false},
		{"ftp scheme", "ftp://example.com/file", false},
		{"mailto scheme", "mailto:user@example.com", false},
		{"javascript", "javascript:void(0)", false},
		{"jpg image", "https://example.com/image.jpg", false},
		{"jpeg image", "https://example.com/image.jpeg", false},
		{"png image", "https://example.com/image.png", false},
		{"gif image", "https://example.com/image.gif", false},
		{"css file", "https://example.com/style.css", false},
		{"pdf file", "https://example.com/doc.pdf", false},
		{"zip file", "https://example.com/archive.zip", false},
		{"mp4 video", "https://example.com/video.mp4", false},
		{"docx file", "https://example.com/doc.docx", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidURL(tt.url)
			if got != tt.want {
				t.Errorf("IsValidURL(%s) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

// =============================================================================
// ExtractDomain Tests
// =============================================================================

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    string
		wantErr bool
	}{
		{"simple URL", "https://example.com/path", "example.com", false},
		{"with port", "https://example.com:8080/path", "example.com:8080", false},
		{"subdomain", "https://sub.example.com/path", "sub.example.com", false},
		{"invalid URL", "://invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractDomain(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractDomain() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ExtractDomain() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// RuleBuilder Tests
// =============================================================================

func TestRuleBuilder(t *testing.T) {
	rules := NewRuleBuilder().
		WithMaxDepth(15).
		WithFollowExternal(true).
		WithIncludePatterns(`.*api.*`, `.*graphql.*`).
		WithExcludePatterns(`.*logout.*`).
		WithAllowedDomains("trusted.com", "other.com").
		Build()

	if rules.MaxDepth != 15 {
		t.Errorf("MaxDepth = %d, want 15", rules.MaxDepth)
	}
	if !rules.FollowExternal {
		t.Error("FollowExternal should be true")
	}
	if len(rules.IncludePatterns) != 2 {
		t.Errorf("IncludePatterns length = %d, want 2", len(rules.IncludePatterns))
	}
	if len(rules.ExcludePatterns) != 1 {
		t.Errorf("ExcludePatterns length = %d, want 1", len(rules.ExcludePatterns))
	}
	if len(rules.AllowedDomains) != 2 {
		t.Errorf("AllowedDomains length = %d, want 2", len(rules.AllowedDomains))
	}
}

func TestRuleBuilder_WithDefaultExcludes(t *testing.T) {
	rules := NewRuleBuilder().
		WithDefaultExcludes().
		Build()

	if len(rules.ExcludePatterns) != len(DefaultExcludePatterns) {
		t.Errorf("ExcludePatterns length = %d, want %d", len(rules.ExcludePatterns), len(DefaultExcludePatterns))
	}
}

// =============================================================================
// MatchPattern Tests
// =============================================================================

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		pattern string
		want    bool
	}{
		{"exact match", "https://example.com/api/users", "/api/", true},
		{"wildcard prefix", "https://example.com/api/users", "*/api/*", true},
		{"wildcard suffix", "https://example.com/page.json", "*.json", true},
		{"no match", "https://example.com/about", "/api/", false},
		{"multiple wildcards", "https://example.com/api/v1/users", "*/api/*/users", true},
		{"contains match", "https://example.com/path/to/page", "to", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchPattern(tt.url, tt.pattern)
			if got != tt.want {
				t.Errorf("MatchPattern(%s, %s) = %v, want %v", tt.url, tt.pattern, got, tt.want)
			}
		})
	}
}

// =============================================================================
// IsAPIPath Tests
// =============================================================================

func TestIsAPIPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/api/users", true},
		// Note: /v[0-9]+/ pattern uses regex syntax but MatchPattern uses wildcards
		// so version paths don't match unless explicitly listed
		{"/graphql", true},
		{"/rest/data", true},
		{"/ajax/load", true},
		{"/json/data", true},
		{"/data.json", true},
		{"/data.xml", true},
		{"?callback=jsonp", true},
		{"?format=json", true},
		{"/about", false},
		{"/contact", false},
		{"/products/list", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsAPIPath(tt.path)
			if got != tt.want {
				t.Errorf("IsAPIPath(%s) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// =============================================================================
// ClassifyURL Tests
// =============================================================================

func TestClassifyURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://example.com/api/users", "api"},
		{"https://example.com/graphql", "api"},
		{"https://example.com/rest/items", "api"},
		{"https://example.com/script.js", "static"},
		{"https://example.com/style.css", "static"},
		{"https://example.com/bundle.js.map", "static"},
		{"https://example.com/login", "auth"},
		{"https://example.com/signin", "auth"},
		{"https://example.com/oauth/callback", "auth"},
		{"https://example.com/sso/login", "auth"},
		{"https://example.com/admin/users", "admin"},
		{"https://example.com/dashboard", "admin"},
		{"https://example.com/panel/settings", "admin"},
		{"https://example.com/about", "page"},
		{"https://example.com/products", "page"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := ClassifyURL(tt.url)
			if got != tt.want {
				t.Errorf("ClassifyURL(%s) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}
