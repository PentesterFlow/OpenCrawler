// Package crawler provides the main DAST web application crawler functionality.
package crawler

import (
	"net/http"
	"time"
)

// Endpoint represents a discovered endpoint during crawling.
type Endpoint struct {
	URL            string            `json:"url"`
	Method         string            `json:"method"`
	Source         string            `json:"source"` // passive, active, html, javascript
	Depth          int               `json:"depth"`
	Parameters     []Parameter       `json:"parameters,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	DiscoveredFrom string            `json:"discovered_from,omitempty"`
	StatusCode     int               `json:"status_code,omitempty"`
	ContentType    string            `json:"content_type,omitempty"`
	ResponseSize   int64             `json:"response_size,omitempty"`
	Timestamp      time.Time         `json:"timestamp"`
}

// Parameter represents a request parameter.
type Parameter struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // query, body, header, path, cookie
	Example  string `json:"example,omitempty"`
	Required bool   `json:"required,omitempty"`
}

// Form represents an HTML form discovered during crawling.
type Form struct {
	URL       string      `json:"url"`
	Action    string      `json:"action"`
	Method    string      `json:"method"`
	Enctype   string      `json:"enctype"`
	Inputs    []FormInput `json:"inputs"`
	HasCSRF   bool        `json:"has_csrf"`
	Depth     int         `json:"depth"`
	Timestamp time.Time   `json:"timestamp"`
}

// FormInput represents an input field in a form.
type FormInput struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Value       string `json:"value,omitempty"`
	Required    bool   `json:"required"`
	Placeholder string `json:"placeholder,omitempty"`
	Pattern     string `json:"pattern,omitempty"`
	MaxLength   int    `json:"max_length,omitempty"`
	MinLength   int    `json:"min_length,omitempty"`
}

// WebSocketEndpoint represents a discovered WebSocket endpoint.
type WebSocketEndpoint struct {
	URL            string          `json:"url"`
	DiscoveredFrom string          `json:"discovered_from"`
	SampleMessages []WebSocketMsg  `json:"sample_messages,omitempty"`
	Protocols      []string        `json:"protocols,omitempty"`
	Timestamp      time.Time       `json:"timestamp"`
}

// WebSocketMsg represents a WebSocket message.
type WebSocketMsg struct {
	Direction string    `json:"direction"` // sent, received
	Type      string    `json:"type"`      // text, binary
	Data      string    `json:"data"`
	Timestamp time.Time `json:"timestamp"`
}

// CrawlResult represents the complete result of a crawl session.
type CrawlResult struct {
	Target       string              `json:"target"`
	StartedAt    time.Time           `json:"started_at"`
	CompletedAt  time.Time           `json:"completed_at,omitempty"`
	Stats        CrawlStats          `json:"stats"`
	Endpoints    []Endpoint          `json:"endpoints"`
	Forms        []Form              `json:"forms"`
	WebSockets   []WebSocketEndpoint `json:"websockets"`
	Technologies []Technology        `json:"technologies,omitempty"`
	Secrets      []SecretFinding     `json:"secrets,omitempty"`
	Errors       []CrawlError        `json:"errors,omitempty"`
}

// Technology represents a detected technology.
type Technology struct {
	Name       string `json:"name"`
	Category   string `json:"category"`
	Version    string `json:"version,omitempty"`
	Confidence int    `json:"confidence"`
	Evidence   string `json:"evidence,omitempty"`
}

// SecretFinding represents a potential secret found in source code.
type SecretFinding struct {
	Type    string `json:"type"`
	Value   string `json:"value"`
	File    string `json:"file,omitempty"`
	Context string `json:"context,omitempty"`
}

// CrawlStats contains statistics about the crawl.
type CrawlStats struct {
	URLsDiscovered     int           `json:"urls_discovered"`
	PagesCrawled       int           `json:"pages_crawled"`
	FormsFound         int           `json:"forms_found"`
	APIEndpoints       int           `json:"api_endpoints"`
	WebSocketEndpoints int           `json:"websocket_endpoints"`
	ErrorCount         int           `json:"error_count"`
	Duration           time.Duration `json:"duration"`
	BytesTransferred   int64         `json:"bytes_transferred"`
}

// CrawlError represents an error encountered during crawling.
type CrawlError struct {
	URL       string    `json:"url"`
	Error     string    `json:"error"`
	Timestamp time.Time `json:"timestamp"`
}

// QueueItem represents an item in the crawl queue.
type QueueItem struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Depth     int               `json:"depth"`
	ParentURL string            `json:"parent_url"`
	Headers   map[string]string `json:"headers,omitempty"`
	Body      []byte            `json:"body,omitempty"`
	Priority  int               `json:"priority"`
	Timestamp time.Time         `json:"timestamp"`
}

// AuthCredentials holds authentication credentials.
type AuthCredentials struct {
	Type         AuthType          `json:"type"`
	Username     string            `json:"username,omitempty"`
	Password     string            `json:"password,omitempty"`
	Token        string            `json:"token,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	Cookies      []*http.Cookie    `json:"-"`
	LoginURL     string            `json:"login_url,omitempty"`
	FormFields   map[string]string `json:"form_fields,omitempty"`
	OAuthConfig  *OAuthConfig      `json:"oauth_config,omitempty"`
}

// AuthType represents the type of authentication.
type AuthType string

const (
	AuthTypeNone      AuthType = "none"
	AuthTypeSession   AuthType = "session"
	AuthTypeJWT       AuthType = "jwt"
	AuthTypeOAuth     AuthType = "oauth"
	AuthTypeFormLogin AuthType = "form"
	AuthTypeAPIKey    AuthType = "apikey"
	AuthTypeBasic     AuthType = "basic"
)

// OAuthConfig holds OAuth 2.0 configuration.
type OAuthConfig struct {
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	AuthURL      string   `json:"auth_url"`
	TokenURL     string   `json:"token_url"`
	RedirectURL  string   `json:"redirect_url"`
	Scopes       []string `json:"scopes"`
}

// ScopeRules defines crawling scope rules.
type ScopeRules struct {
	IncludePatterns []string `json:"include_patterns"`
	ExcludePatterns []string `json:"exclude_patterns"`
	AllowedDomains  []string `json:"allowed_domains"`
	MaxDepth        int      `json:"max_depth"`
	FollowExternal  bool     `json:"follow_external"`
}

// RateLimitConfig defines rate limiting configuration.
type RateLimitConfig struct {
	RequestsPerSecond float64       `json:"requests_per_second"`
	Burst             int           `json:"burst"`
	DelayBetween      time.Duration `json:"delay_between"`
	RespectRobotsTxt  bool          `json:"respect_robots_txt"`
}

// OutputConfig defines output configuration.
type OutputConfig struct {
	Format     string `json:"format"` // json
	FilePath   string `json:"file_path"`
	Pretty     bool   `json:"pretty"`
	StreamMode bool   `json:"stream_mode"`
}

// StateConfig defines state persistence configuration.
type StateConfig struct {
	Enabled   bool   `json:"enabled"`
	FilePath  string `json:"file_path"`
	AutoSave  bool   `json:"auto_save"`
	Interval  int    `json:"interval_seconds"`
}
