package output

import (
	"net/http"
	"time"
)

// CrawlResult represents the complete result of a crawl session.
type CrawlResult struct {
	Target      string              `json:"target"`
	StartedAt   time.Time           `json:"started_at"`
	CompletedAt time.Time           `json:"completed_at,omitempty"`
	Stats       CrawlStats          `json:"stats"`
	Endpoints   []Endpoint          `json:"endpoints"`
	Forms       []Form              `json:"forms"`
	WebSockets  []WebSocketEndpoint `json:"websockets"`
	Errors      []CrawlError        `json:"errors,omitempty"`
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

// Endpoint represents a discovered endpoint during crawling.
type Endpoint struct {
	URL            string            `json:"url"`
	Method         string            `json:"method"`
	Source         string            `json:"source"`
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
	Type     string `json:"type"`
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
	URL            string         `json:"url"`
	DiscoveredFrom string         `json:"discovered_from"`
	SampleMessages []WebSocketMsg `json:"sample_messages,omitempty"`
	Protocols      []string       `json:"protocols,omitempty"`
	Timestamp      time.Time      `json:"timestamp"`
}

// WebSocketMsg represents a WebSocket message.
type WebSocketMsg struct {
	Direction string    `json:"direction"`
	Type      string    `json:"type"`
	Data      string    `json:"data"`
	Timestamp time.Time `json:"timestamp"`
}

// CrawlError represents an error encountered during crawling.
type CrawlError struct {
	URL       string    `json:"url"`
	Error     string    `json:"error"`
	Timestamp time.Time `json:"timestamp"`
}

// AuthCredentials holds authentication credentials (used for state).
type AuthCredentials struct {
	Type        string            `json:"type"`
	Username    string            `json:"username,omitempty"`
	Password    string            `json:"password,omitempty"`
	Token       string            `json:"token,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Cookies     []*http.Cookie    `json:"-"`
	LoginURL    string            `json:"login_url,omitempty"`
	FormFields  map[string]string `json:"form_fields,omitempty"`
}
