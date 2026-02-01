// Package discovery provides API discovery mechanisms.
package discovery

import "time"

// Endpoint represents a discovered API endpoint.
type Endpoint struct {
	URL            string
	Method         string
	Source         string // passive, active, active_method_probe, active_graphql
	Parameters     []Parameter
	Headers        map[string]string
	DiscoveredFrom string
	StatusCode     int
	ContentType    string
	Timestamp      time.Time
}

// Parameter represents a request parameter.
type Parameter struct {
	Name    string
	Type    string // query, body, header, path, cookie
	Example string
}
