package parser

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
)

// APIParser extracts API information from various sources.
type APIParser struct{}

// NewAPIParser creates a new API parser.
func NewAPIParser() *APIParser {
	return &APIParser{}
}

// APIInfo represents discovered API information.
type APIInfo struct {
	Endpoints []Endpoint
	BaseURL   string
	Version   string
	Type      APIType // REST, GraphQL, SOAP, etc.
}

// APIType represents the type of API.
type APIType string

const (
	APITypeREST    APIType = "rest"
	APITypeGraphQL APIType = "graphql"
	APITypeSOAP    APIType = "soap"
	APITypeRPC     APIType = "rpc"
	APITypeUnknown APIType = "unknown"
)

// ParseFromResponse extracts API information from a response.
func (p *APIParser) ParseFromResponse(url, contentType string, body []byte) *APIInfo {
	info := &APIInfo{
		Endpoints: make([]Endpoint, 0),
	}

	// Detect API type from content type
	if strings.Contains(contentType, "application/json") {
		info.Type = APITypeREST
		p.parseJSONResponse(body, info)
	} else if strings.Contains(contentType, "application/xml") ||
		strings.Contains(contentType, "text/xml") {
		info.Type = APITypeSOAP
	} else if strings.Contains(contentType, "application/graphql") {
		info.Type = APITypeGraphQL
	}

	// Try to detect version from URL
	info.Version = p.detectVersion(url)
	info.BaseURL = p.extractBaseURL(url)

	return info
}

// parseJSONResponse extracts API information from JSON.
func (p *APIParser) parseJSONResponse(body []byte, info *APIInfo) {
	// Try to parse as JSON
	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return
	}

	// Look for common API response patterns
	p.extractLinksFromJSON(data, info)
}

// extractLinksFromJSON recursively extracts URLs from JSON.
func (p *APIParser) extractLinksFromJSON(data interface{}, info *APIInfo) {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			// Check for HATEOAS links
			if key == "_links" || key == "links" {
				p.extractHATEOASLinks(value, info)
			}
			// Check for URL fields
			if isURLField(key) {
				if urlStr, ok := value.(string); ok {
					if isValidAPIURL(urlStr) {
						info.Endpoints = append(info.Endpoints, Endpoint{
							URL:    urlStr,
							Source: "json_response",
						})
					}
				}
			}
			// Recurse
			p.extractLinksFromJSON(value, info)
		}
	case []interface{}:
		for _, item := range v {
			p.extractLinksFromJSON(item, info)
		}
	}
}

// extractHATEOASLinks extracts HATEOAS-style links.
func (p *APIParser) extractHATEOASLinks(data interface{}, info *APIInfo) {
	switch v := data.(type) {
	case map[string]interface{}:
		for _, link := range v {
			if linkMap, ok := link.(map[string]interface{}); ok {
				if href, ok := linkMap["href"].(string); ok {
					method := "GET"
					if m, ok := linkMap["method"].(string); ok {
						method = strings.ToUpper(m)
					}
					info.Endpoints = append(info.Endpoints, Endpoint{
						URL:    href,
						Method: method,
						Source: "hateoas",
					})
				}
			}
		}
	case []interface{}:
		for _, item := range v {
			if linkMap, ok := item.(map[string]interface{}); ok {
				if href, ok := linkMap["href"].(string); ok {
					method := "GET"
					if m, ok := linkMap["method"].(string); ok {
						method = strings.ToUpper(m)
					}
					info.Endpoints = append(info.Endpoints, Endpoint{
						URL:    href,
						Method: method,
						Source: "hateoas",
					})
				}
			}
		}
	}
}

// detectVersion detects API version from URL.
func (p *APIParser) detectVersion(urlStr string) string {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`/v(\d+(?:\.\d+)?)/`),
		regexp.MustCompile(`/api/v(\d+(?:\.\d+)?)/`),
		regexp.MustCompile(`version=(\d+(?:\.\d+)?)`),
	}

	for _, pattern := range patterns {
		if matches := pattern.FindStringSubmatch(urlStr); len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}

// extractBaseURL extracts the base API URL.
func (p *APIParser) extractBaseURL(urlStr string) string {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}

	// Look for API path prefix
	path := parsed.Path
	apiPrefixes := []string{"/api/", "/v1/", "/v2/", "/v3/", "/rest/"}

	for _, prefix := range apiPrefixes {
		if idx := strings.Index(path, prefix); idx != -1 {
			parsed.Path = path[:idx+len(prefix)-1]
			parsed.RawQuery = ""
			return parsed.String()
		}
	}

	return ""
}

// ParseOpenAPISpec parses an OpenAPI/Swagger specification.
func (p *APIParser) ParseOpenAPISpec(spec []byte) (*APIInfo, error) {
	info := &APIInfo{
		Endpoints: make([]Endpoint, 0),
		Type:      APITypeREST,
	}

	var data map[string]interface{}
	if err := json.Unmarshal(spec, &data); err != nil {
		return nil, err
	}

	// Extract base info
	if swagger, ok := data["swagger"].(string); ok {
		info.Version = swagger
	} else if openapi, ok := data["openapi"].(string); ok {
		info.Version = openapi
	}

	// Extract servers/host
	if servers, ok := data["servers"].([]interface{}); ok && len(servers) > 0 {
		if server, ok := servers[0].(map[string]interface{}); ok {
			if urlStr, ok := server["url"].(string); ok {
				info.BaseURL = urlStr
			}
		}
	} else if host, ok := data["host"].(string); ok {
		scheme := "https"
		if schemes, ok := data["schemes"].([]interface{}); ok && len(schemes) > 0 {
			if s, ok := schemes[0].(string); ok {
				scheme = s
			}
		}
		basePath := ""
		if bp, ok := data["basePath"].(string); ok {
			basePath = bp
		}
		info.BaseURL = scheme + "://" + host + basePath
	}

	// Extract paths
	if paths, ok := data["paths"].(map[string]interface{}); ok {
		for path, methods := range paths {
			if methodMap, ok := methods.(map[string]interface{}); ok {
				for method := range methodMap {
					method = strings.ToUpper(method)
					if isHTTPMethod(method) {
						endpoint := Endpoint{
							URL:    info.BaseURL + path,
							Method: method,
							Source: "openapi",
						}

						// Extract parameters
						if methodData, ok := methodMap[strings.ToLower(method)].(map[string]interface{}); ok {
							if params, ok := methodData["parameters"].([]interface{}); ok {
								for _, param := range params {
									if paramMap, ok := param.(map[string]interface{}); ok {
										if name, ok := paramMap["name"].(string); ok {
											paramIn := "query"
											if in, ok := paramMap["in"].(string); ok {
												paramIn = in
											}
											endpoint.Parameters = append(endpoint.Parameters, Parameter{
												Name: name,
												Type: paramIn,
											})
										}
									}
								}
							}
						}

						info.Endpoints = append(info.Endpoints, endpoint)
					}
				}
			}
		}
	}

	return info, nil
}

// ParseGraphQLSchema parses a GraphQL schema.
func (p *APIParser) ParseGraphQLSchema(schema string) *APIInfo {
	info := &APIInfo{
		Endpoints: make([]Endpoint, 0),
		Type:      APITypeGraphQL,
	}

	// Extract query types
	queryPattern := regexp.MustCompile(`type\s+Query\s*\{([^}]+)\}`)
	if matches := queryPattern.FindStringSubmatch(schema); len(matches) > 1 {
		fields := parseGraphQLFields(matches[1])
		for _, field := range fields {
			info.Endpoints = append(info.Endpoints, Endpoint{
				URL:    "/graphql",
				Method: "POST",
				Source: "graphql_query",
				Parameters: []Parameter{
					{Name: "query", Type: "body", Example: field},
				},
			})
		}
	}

	// Extract mutation types
	mutationPattern := regexp.MustCompile(`type\s+Mutation\s*\{([^}]+)\}`)
	if matches := mutationPattern.FindStringSubmatch(schema); len(matches) > 1 {
		fields := parseGraphQLFields(matches[1])
		for _, field := range fields {
			info.Endpoints = append(info.Endpoints, Endpoint{
				URL:    "/graphql",
				Method: "POST",
				Source: "graphql_mutation",
				Parameters: []Parameter{
					{Name: "mutation", Type: "body", Example: field},
				},
			})
		}
	}

	return info
}

func parseGraphQLFields(block string) []string {
	fields := make([]string, 0)
	lines := strings.Split(block, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Extract field name
		if idx := strings.Index(line, "("); idx != -1 {
			fields = append(fields, strings.TrimSpace(line[:idx]))
		} else if idx := strings.Index(line, ":"); idx != -1 {
			fields = append(fields, strings.TrimSpace(line[:idx]))
		}
	}

	return fields
}

func isURLField(key string) bool {
	urlFields := []string{
		"url", "href", "link", "uri", "endpoint",
		"path", "resource", "location", "redirect",
	}
	lower := strings.ToLower(key)
	for _, field := range urlFields {
		if strings.Contains(lower, field) {
			return true
		}
	}
	return false
}

func isValidAPIURL(s string) bool {
	if s == "" {
		return false
	}
	// Check for absolute or relative URLs
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "/") {
		return true
	}
	return false
}

func isHTTPMethod(method string) bool {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	for _, m := range methods {
		if method == m {
			return true
		}
	}
	return false
}
