package output

import (
	"time"
)

// SummaryReport contains a summary of the crawl.
type SummaryReport struct {
	Target      string        `json:"target"`
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt time.Time     `json:"completed_at"`
	Duration    time.Duration `json:"duration"`
	Statistics  Statistics    `json:"statistics"`
}

// Statistics contains crawl statistics.
type Statistics struct {
	TotalURLs          int `json:"total_urls"`
	CrawledPages       int `json:"crawled_pages"`
	DiscoveredForms    int `json:"discovered_forms"`
	DiscoveredAPIs     int `json:"discovered_apis"`
	DiscoveredWS       int `json:"discovered_websockets"`
	Errors             int `json:"errors"`
	SkippedOutOfScope  int `json:"skipped_out_of_scope"`
	SkippedDuplicate   int `json:"skipped_duplicate"`
}

// EndpointSummary contains a summary of discovered endpoints.
type EndpointSummary struct {
	Total       int                 `json:"total"`
	ByMethod    map[string]int      `json:"by_method"`
	BySource    map[string]int      `json:"by_source"`
	ByStatus    map[int]int         `json:"by_status"`
	TopPaths    []PathCount         `json:"top_paths"`
}

// PathCount represents a path and its occurrence count.
type PathCount struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
}

// FormSummary contains a summary of discovered forms.
type FormSummary struct {
	Total        int            `json:"total"`
	ByMethod     map[string]int `json:"by_method"`
	ByType       map[string]int `json:"by_type"`
	WithCSRF     int            `json:"with_csrf"`
	WithCaptcha  int            `json:"with_captcha"`
	FileUpload   int            `json:"file_upload"`
}

// SecurityFindings contains security-relevant findings.
type SecurityFindings struct {
	ExposedAPIs      []ExposedAPI     `json:"exposed_apis,omitempty"`
	MissingCSRF      []string         `json:"missing_csrf,omitempty"`
	FileUploads      []string         `json:"file_uploads,omitempty"`
	PotentialSecrets []string         `json:"potential_secrets,omitempty"`
	DebugEndpoints   []string         `json:"debug_endpoints,omitempty"`
}

// ExposedAPI represents a potentially exposed API endpoint.
type ExposedAPI struct {
	URL         string `json:"url"`
	Method      string `json:"method"`
	Reason      string `json:"reason"`
}

// CrawlMetadata contains metadata about the crawl.
type CrawlMetadata struct {
	CrawlerVersion string            `json:"crawler_version"`
	UserAgent      string            `json:"user_agent"`
	Config         map[string]interface{} `json:"config"`
	Environment    map[string]string `json:"environment,omitempty"`
}

// TimingInfo contains timing information.
type TimingInfo struct {
	TotalDuration   time.Duration `json:"total_duration"`
	AveragePageTime time.Duration `json:"average_page_time"`
	FastestPage     time.Duration `json:"fastest_page"`
	SlowestPage     time.Duration `json:"slowest_page"`
	RequestsPerSec  float64       `json:"requests_per_second"`
}

// DetailedReport contains a detailed crawl report.
type DetailedReport struct {
	Summary   SummaryReport     `json:"summary"`
	Endpoints EndpointSummary   `json:"endpoints"`
	Forms     FormSummary       `json:"forms"`
	Security  SecurityFindings  `json:"security"`
	Metadata  CrawlMetadata     `json:"metadata"`
	Timing    TimingInfo        `json:"timing"`
}
