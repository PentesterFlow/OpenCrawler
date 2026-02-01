package parser

import (
	"regexp"
	"strings"
)

// JSParser performs static analysis on JavaScript code.
type JSParser struct{}

// NewJSParser creates a new JavaScript parser.
func NewJSParser() *JSParser {
	return &JSParser{}
}

// JSParseResult contains the result of JavaScript analysis.
type JSParseResult struct {
	URLs         []string
	APIEndpoints []APIEndpoint
	WebSockets   []string
	Secrets      []PotentialSecret
	Routes       []Route
	Functions    []FunctionInfo
}

// APIEndpoint represents a discovered API endpoint.
type APIEndpoint struct {
	URL         string
	Method      string
	Parameters  []string
	SourceLine  int
	Context     string
}

// PotentialSecret represents a potential secret in code.
type PotentialSecret struct {
	Type   string
	Value  string
	Line   int
	Context string
}

// Route represents a client-side route.
type Route struct {
	Path      string
	Component string
}

// FunctionInfo represents a function signature.
type FunctionInfo struct {
	Name       string
	Parameters []string
	Line       int
}

// Parse analyzes JavaScript code.
func (p *JSParser) Parse(js string) *JSParseResult {
	result := &JSParseResult{
		URLs:         make([]string, 0),
		APIEndpoints: make([]APIEndpoint, 0),
		WebSockets:   make([]string, 0),
		Secrets:      make([]PotentialSecret, 0),
		Routes:       make([]Route, 0),
		Functions:    make([]FunctionInfo, 0),
	}

	// Extract URLs
	result.URLs = p.extractURLs(js)

	// Extract API endpoints
	result.APIEndpoints = p.extractAPIEndpoints(js)

	// Extract WebSocket URLs
	result.WebSockets = p.extractWebSockets(js)

	// Extract potential secrets
	result.Secrets = p.extractSecrets(js)

	// Extract routes
	result.Routes = p.extractRoutes(js)

	return result
}

// extractURLs extracts URLs from JavaScript.
func (p *JSParser) extractURLs(js string) []string {
	urls := make([]string, 0)
	seen := make(map[string]bool)

	// URL patterns
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`["']https?://[^"'\s]+["']`),
		regexp.MustCompile(`["']/api/[^"'\s]+["']`),
		regexp.MustCompile(`["']/v[0-9]+/[^"'\s]+["']`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllString(js, -1)
		for _, match := range matches {
			// Clean up the match
			url := strings.Trim(match, "\"'")
			if !seen[url] {
				seen[url] = true
				urls = append(urls, url)
			}
		}
	}

	return urls
}

// extractAPIEndpoints extracts API endpoint patterns.
func (p *JSParser) extractAPIEndpoints(js string) []APIEndpoint {
	endpoints := make([]APIEndpoint, 0)

	// Common API call patterns
	patterns := []struct {
		regex  *regexp.Regexp
		method string
	}{
		{regexp.MustCompile(`fetch\s*\(\s*["']([^"']+)["']`), "GET"},
		{regexp.MustCompile(`fetch\s*\(\s*["']([^"']+)["']\s*,\s*\{[^}]*method\s*:\s*["'](\w+)["']`), ""},
		{regexp.MustCompile(`axios\.get\s*\(\s*["']([^"']+)["']`), "GET"},
		{regexp.MustCompile(`axios\.post\s*\(\s*["']([^"']+)["']`), "POST"},
		{regexp.MustCompile(`axios\.put\s*\(\s*["']([^"']+)["']`), "PUT"},
		{regexp.MustCompile(`axios\.delete\s*\(\s*["']([^"']+)["']`), "DELETE"},
		{regexp.MustCompile(`axios\.patch\s*\(\s*["']([^"']+)["']`), "PATCH"},
		{regexp.MustCompile(`\$\.ajax\s*\(\s*\{[^}]*url\s*:\s*["']([^"']+)["']`), ""},
		{regexp.MustCompile(`\$\.get\s*\(\s*["']([^"']+)["']`), "GET"},
		{regexp.MustCompile(`\$\.post\s*\(\s*["']([^"']+)["']`), "POST"},
		{regexp.MustCompile(`XMLHttpRequest[^;]*\.open\s*\(\s*["'](\w+)["']\s*,\s*["']([^"']+)["']`), ""},
	}

	for _, p := range patterns {
		matches := p.regex.FindAllStringSubmatch(js, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				endpoint := APIEndpoint{
					URL:    match[1],
					Method: p.method,
				}

				// Extract method from match if available
				if p.method == "" && len(match) >= 3 {
					endpoint.Method = strings.ToUpper(match[2])
				}
				if endpoint.Method == "" {
					endpoint.Method = "GET"
				}

				// Extract parameters from URL
				endpoint.Parameters = extractURLParams(endpoint.URL)

				endpoints = append(endpoints, endpoint)
			}
		}
	}

	return endpoints
}

// extractURLParams extracts parameter names from a URL.
func extractURLParams(url string) []string {
	params := make([]string, 0)

	// Path parameters (e.g., :id, {id})
	pathParamPatterns := []*regexp.Regexp{
		regexp.MustCompile(`:(\w+)`),
		regexp.MustCompile(`\{(\w+)\}`),
		regexp.MustCompile(`\$\{(\w+)\}`),
	}

	for _, pattern := range pathParamPatterns {
		matches := pattern.FindAllStringSubmatch(url, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				params = append(params, match[1])
			}
		}
	}

	// Query parameters
	if idx := strings.Index(url, "?"); idx != -1 {
		query := url[idx+1:]
		parts := strings.Split(query, "&")
		for _, part := range parts {
			if eqIdx := strings.Index(part, "="); eqIdx != -1 {
				param := part[:eqIdx]
				params = append(params, param)
			}
		}
	}

	return params
}

// extractWebSockets extracts WebSocket URLs.
func (p *JSParser) extractWebSockets(js string) []string {
	wsURLs := make([]string, 0)
	seen := make(map[string]bool)

	patterns := []*regexp.Regexp{
		regexp.MustCompile(`new\s+WebSocket\s*\(\s*["']([^"']+)["']`),
		regexp.MustCompile(`["'](wss?://[^"']+)["']`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(js, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				url := match[1]
				if !seen[url] {
					seen[url] = true
					wsURLs = append(wsURLs, url)
				}
			}
		}
	}

	return wsURLs
}

// extractSecrets looks for potential secrets in code.
func (p *JSParser) extractSecrets(js string) []PotentialSecret {
	secrets := make([]PotentialSecret, 0)

	// Secret patterns
	patterns := []struct {
		regex *regexp.Regexp
		typ   string
	}{
		{regexp.MustCompile(`(?i)api[_-]?key\s*[:=]\s*["']([^"']{10,})["']`), "api_key"},
		{regexp.MustCompile(`(?i)secret[_-]?key\s*[:=]\s*["']([^"']{10,})["']`), "secret_key"},
		{regexp.MustCompile(`(?i)password\s*[:=]\s*["']([^"']{6,})["']`), "password"},
		{regexp.MustCompile(`(?i)token\s*[:=]\s*["']([^"']{10,})["']`), "token"},
		{regexp.MustCompile(`(?i)auth[_-]?token\s*[:=]\s*["']([^"']{10,})["']`), "auth_token"},
		{regexp.MustCompile(`(?i)access[_-]?token\s*[:=]\s*["']([^"']{10,})["']`), "access_token"},
		{regexp.MustCompile(`(?i)private[_-]?key\s*[:=]\s*["']([^"']{10,})["']`), "private_key"},
		{regexp.MustCompile(`(?i)client[_-]?secret\s*[:=]\s*["']([^"']{10,})["']`), "client_secret"},
		{regexp.MustCompile(`(?i)aws[_-]?secret\s*[:=]\s*["']([^"']{10,})["']`), "aws_secret"},
		{regexp.MustCompile(`AKIA[A-Z0-9]{16}`), "aws_access_key"},
		{regexp.MustCompile(`eyJ[A-Za-z0-9-_]+\.eyJ[A-Za-z0-9-_]+\.[A-Za-z0-9-_]+`), "jwt"},
	}

	lines := strings.Split(js, "\n")
	for lineNum, line := range lines {
		for _, p := range patterns {
			matches := p.regex.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				value := match[0]
				if len(match) > 1 {
					value = match[1]
				}

				// Skip common false positives
				if isLikelyFalsePositive(value) {
					continue
				}

				secrets = append(secrets, PotentialSecret{
					Type:    p.typ,
					Value:   maskSecret(value),
					Line:    lineNum + 1,
					Context: truncateContext(line, 100),
				})
			}
		}
	}

	return secrets
}

// extractRoutes extracts client-side routes.
func (p *JSParser) extractRoutes(js string) []Route {
	routes := make([]Route, 0)

	// React Router patterns
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`<Route\s+path=["']([^"']+)["']`),
		regexp.MustCompile(`path:\s*["']([^"']+)["']`),
		regexp.MustCompile(`\.route\s*\(\s*["']([^"']+)["']`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(js, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				routes = append(routes, Route{
					Path: match[1],
				})
			}
		}
	}

	return routes
}

func isLikelyFalsePositive(value string) bool {
	// Skip placeholder values
	placeholders := []string{
		"your_",
		"example",
		"xxx",
		"placeholder",
		"dummy",
		"test",
		"changeme",
		"<",
		">",
		"${",
		"{{",
	}

	lower := strings.ToLower(value)
	for _, p := range placeholders {
		if strings.Contains(lower, p) {
			return true
		}
	}

	return false
}

func maskSecret(value string) string {
	if len(value) <= 8 {
		return strings.Repeat("*", len(value))
	}
	return value[:4] + strings.Repeat("*", len(value)-8) + value[len(value)-4:]
}

func truncateContext(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
