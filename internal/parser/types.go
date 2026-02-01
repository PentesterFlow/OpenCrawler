package parser

import "time"

// Endpoint represents a discovered API endpoint.
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

// Form represents an HTML form discovered during crawling.
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

// FormInput represents an input field in a form.
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
