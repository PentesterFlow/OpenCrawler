package state

import (
	"encoding/json"
	"time"
)

// CrawlStats contains statistics about the crawl.
type CrawlStats struct {
	URLsDiscovered     int
	PagesCrawled       int
	FormsFound         int
	APIEndpoints       int
	WebSocketEndpoints int
	ErrorCount         int
	Duration           time.Duration
	BytesTransferred   int64
}

// Endpoint represents a discovered endpoint.
type Endpoint struct {
	URL            string
	Method         string
	Source         string
	Depth          int
	Parameters     []Parameter
	Headers        map[string]string
	DiscoveredFrom string
	StatusCode     int
	ContentType    string
	ResponseSize   int64
	Timestamp      time.Time
}

// Parameter represents a request parameter.
type Parameter struct {
	Name     string
	Type     string
	Example  string
	Required bool
}

// Form represents an HTML form.
type Form struct {
	URL       string
	Action    string
	Method    string
	Enctype   string
	Inputs    []FormInput
	HasCSRF   bool
	Depth     int
	Timestamp time.Time
}

// FormInput represents a form input field.
type FormInput struct {
	Name        string
	Type        string
	Value       string
	Required    bool
	Placeholder string
	Pattern     string
	MaxLength   int
	MinLength   int
}

// WebSocketEndpoint represents a discovered WebSocket endpoint.
type WebSocketEndpoint struct {
	URL            string
	DiscoveredFrom string
	SampleMessages []WebSocketMsg
	Protocols      []string
	Timestamp      time.Time
}

// WebSocketMsg represents a WebSocket message.
type WebSocketMsg struct {
	Direction string
	Type      string
	Data      string
	Timestamp time.Time
}

// CrawlError represents a crawl error.
type CrawlError struct {
	URL       string
	Error     string
	Timestamp time.Time
}

// CrawlerState represents the complete state of a crawler session.
type CrawlerState struct {
	Target      string            `json:"target"`
	StartedAt   time.Time         `json:"started_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Stats       CrawlStats        `json:"stats"`
	Config      json.RawMessage   `json:"config"`
	QueueURLs   []string          `json:"queue_urls"`
	VisitedURLs []string          `json:"visited_urls"`
	Endpoints   []Endpoint        `json:"endpoints"`
	Forms       []Form            `json:"forms"`
	WebSockets  []WebSocketEndpoint `json:"websockets"`
	Errors      []CrawlError      `json:"errors"`
}
