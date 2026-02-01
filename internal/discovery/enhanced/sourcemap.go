package enhanced

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// SourceMapParser extracts routes and endpoints from JavaScript source maps.
type SourceMapParser struct {
	client    *http.Client
	userAgent string
}

// SourceMap represents a parsed source map file.
type SourceMap struct {
	Version        int      `json:"version"`
	Sources        []string `json:"sources"`
	SourcesContent []string `json:"sourcesContent,omitempty"`
	Names          []string `json:"names"`
	Mappings       string   `json:"mappings"`
	File           string   `json:"file"`
	SourceRoot     string   `json:"sourceRoot,omitempty"`
}

// SourceMapResult contains extracted information from source maps.
type SourceMapResult struct {
	SourceMapURL   string
	Sources        []string
	Routes         []string
	APIEndpoints   []string
	Secrets        []SecretFinding
	Components     []string
	OriginalSource map[string]string // filename -> content
}

// SecretFinding represents a potential secret found in source code.
type SecretFinding struct {
	Type     string
	Value    string
	File     string
	Context  string
}

// NewSourceMapParser creates a new source map parser.
func NewSourceMapParser(userAgent string) *SourceMapParser {
	return &SourceMapParser{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		userAgent: userAgent,
	}
}

// FindSourceMaps looks for source map references in JavaScript files.
func (p *SourceMapParser) FindSourceMaps(jsURLs []string) []string {
	sourceMaps := make([]string, 0)
	seen := make(map[string]bool)

	for _, jsURL := range jsURLs {
		// Try common source map patterns
		patterns := []string{
			jsURL + ".map",
			strings.TrimSuffix(jsURL, ".js") + ".js.map",
			strings.TrimSuffix(jsURL, ".min.js") + ".min.js.map",
			strings.TrimSuffix(jsURL, ".bundle.js") + ".bundle.js.map",
		}

		for _, mapURL := range patterns {
			if !seen[mapURL] {
				seen[mapURL] = true
				if p.checkExists(mapURL) {
					sourceMaps = append(sourceMaps, mapURL)
				}
			}
		}

		// Also check for sourceMappingURL comment in the JS file
		mapURL := p.extractSourceMappingURL(jsURL)
		if mapURL != "" && !seen[mapURL] {
			seen[mapURL] = true
			sourceMaps = append(sourceMaps, mapURL)
		}
	}

	return sourceMaps
}

// checkExists checks if a URL returns 200 status.
func (p *SourceMapParser) checkExists(targetURL string) bool {
	req, err := http.NewRequest("HEAD", targetURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", p.userAgent)

	resp, err := p.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

// extractSourceMappingURL extracts sourceMappingURL from a JS file.
func (p *SourceMapParser) extractSourceMappingURL(jsURL string) string {
	req, err := http.NewRequest("GET", jsURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", p.userAgent)

	resp, err := p.client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	// Read last 500 bytes where sourceMappingURL typically is
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024)) // 1MB max
	if err != nil {
		return ""
	}

	content := string(body)

	// Look for sourceMappingURL comment
	patterns := []string{
		`//# sourceMappingURL=([^\s]+)`,
		`//@ sourceMappingURL=([^\s]+)`,
		`/\*# sourceMappingURL=([^\s]+) \*/`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(content)
		if len(matches) > 1 {
			mapPath := matches[1]
			// Resolve relative URL
			if !strings.HasPrefix(mapPath, "http") {
				baseURL, _ := url.Parse(jsURL)
				mapURL, _ := baseURL.Parse(mapPath)
				return mapURL.String()
			}
			return mapPath
		}
	}

	return ""
}

// Parse downloads and parses a source map file.
func (p *SourceMapParser) Parse(sourceMapURL string) (*SourceMapResult, error) {
	req, err := http.NewRequest("GET", sourceMapURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", p.userAgent)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var sourceMap SourceMap
	if err := json.Unmarshal(body, &sourceMap); err != nil {
		return nil, err
	}

	result := &SourceMapResult{
		SourceMapURL:   sourceMapURL,
		Sources:        sourceMap.Sources,
		Routes:         make([]string, 0),
		APIEndpoints:   make([]string, 0),
		Secrets:        make([]SecretFinding, 0),
		Components:     make([]string, 0),
		OriginalSource: make(map[string]string),
	}

	// Store source content
	for i, source := range sourceMap.Sources {
		if i < len(sourceMap.SourcesContent) {
			result.OriginalSource[source] = sourceMap.SourcesContent[i]
		}
	}

	// Extract routes and endpoints from source content
	for filename, content := range result.OriginalSource {
		p.extractFromSource(filename, content, result)
	}

	// Also extract from source names (filenames often reveal structure)
	for _, source := range sourceMap.Sources {
		p.extractFromFilename(source, result)
	}

	return result, nil
}

// extractFromSource extracts routes, endpoints, and secrets from source code.
func (p *SourceMapParser) extractFromSource(filename, content string, result *SourceMapResult) {
	seen := make(map[string]bool)

	// Route patterns for various frameworks
	routePatterns := []*regexp.Regexp{
		// React Router
		regexp.MustCompile(`path\s*[=:]\s*["']([^"']+)["']`),
		regexp.MustCompile(`Route\s+path\s*=\s*["']([^"']+)["']`),
		regexp.MustCompile(`to\s*[=:]\s*["']([^"']+)["']`),
		// Vue Router
		regexp.MustCompile(`path\s*:\s*["']([^"']+)["']`),
		// Angular
		regexp.MustCompile(`routerLink\s*=\s*["']([^"']+)["']`),
		// Generic
		regexp.MustCompile(`navigate\s*\(\s*["']([^"']+)["']`),
		regexp.MustCompile(`redirect\s*\(\s*["']([^"']+)["']`),
		regexp.MustCompile(`push\s*\(\s*["']([^"']+)["']`),
	}

	for _, re := range routePatterns {
		matches := re.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				route := match[1]
				if p.isValidRoute(route) && !seen[route] {
					seen[route] = true
					result.Routes = append(result.Routes, route)
				}
			}
		}
	}

	// API endpoint patterns
	apiPatterns := []*regexp.Regexp{
		regexp.MustCompile(`["']((?:/api/|/v\d+/)[^"']+)["']`),
		regexp.MustCompile(`fetch\s*\(\s*["']([^"']+)["']`),
		regexp.MustCompile(`axios\.[a-z]+\s*\(\s*["']([^"']+)["']`),
		regexp.MustCompile(`\$http\.[a-z]+\s*\(\s*["']([^"']+)["']`),
		regexp.MustCompile(`url\s*:\s*["']([^"']+)["']`),
		regexp.MustCompile(`endpoint\s*[=:]\s*["']([^"']+)["']`),
		regexp.MustCompile(`baseURL\s*[=:]\s*["']([^"']+)["']`),
	}

	for _, re := range apiPatterns {
		matches := re.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				endpoint := match[1]
				if p.isValidEndpoint(endpoint) && !seen[endpoint] {
					seen[endpoint] = true
					result.APIEndpoints = append(result.APIEndpoints, endpoint)
				}
			}
		}
	}

	// Secret patterns
	secretPatterns := map[string]*regexp.Regexp{
		"api_key":        regexp.MustCompile(`(?i)(api[_-]?key|apikey)\s*[=:]\s*["']([^"']{20,})["']`),
		"secret":         regexp.MustCompile(`(?i)(secret|password|passwd)\s*[=:]\s*["']([^"']{8,})["']`),
		"token":          regexp.MustCompile(`(?i)(token|auth)\s*[=:]\s*["']([^"']{20,})["']`),
		"aws_key":        regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
		"private_key":    regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA )?PRIVATE KEY-----`),
		"jwt":            regexp.MustCompile(`eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*`),
		"google_api":     regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`),
		"github_token":   regexp.MustCompile(`ghp_[0-9A-Za-z]{36}`),
		"slack_token":    regexp.MustCompile(`xox[baprs]-[0-9A-Za-z-]+`),
	}

	for secretType, re := range secretPatterns {
		matches := re.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			value := match[0]
			if len(match) > 1 {
				value = match[len(match)-1]
			}
			// Get context (surrounding text)
			idx := strings.Index(content, value)
			start := idx - 50
			if start < 0 {
				start = 0
			}
			end := idx + len(value) + 50
			if end > len(content) {
				end = len(content)
			}
			context := content[start:end]

			result.Secrets = append(result.Secrets, SecretFinding{
				Type:    secretType,
				Value:   value,
				File:    filename,
				Context: context,
			})
		}
	}

	// Component patterns (React/Vue/Angular)
	componentPatterns := []*regexp.Regexp{
		regexp.MustCompile(`class\s+(\w+)\s+extends\s+(?:React\.)?Component`),
		regexp.MustCompile(`function\s+(\w+)\s*\([^)]*\)\s*\{[^}]*return\s*\(`),
		regexp.MustCompile(`const\s+(\w+)\s*=\s*\([^)]*\)\s*=>\s*\(`),
		regexp.MustCompile(`@Component\s*\(\s*\{[^}]*selector\s*:\s*["']([^"']+)["']`),
		regexp.MustCompile(`Vue\.component\s*\(\s*["']([^"']+)["']`),
	}

	for _, re := range componentPatterns {
		matches := re.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 && !seen[match[1]] {
				seen[match[1]] = true
				result.Components = append(result.Components, match[1])
			}
		}
	}
}

// extractFromFilename extracts information from source file paths.
func (p *SourceMapParser) extractFromFilename(filename string, result *SourceMapResult) {
	// Extract route-like paths from filenames
	// e.g., src/pages/users/[id].tsx -> /users/:id
	if strings.Contains(filename, "pages/") || strings.Contains(filename, "routes/") {
		parts := strings.Split(filename, "/")
		for i, part := range parts {
			if part == "pages" || part == "routes" {
				// Build route from remaining parts
				routeParts := parts[i+1:]
				if len(routeParts) > 0 {
					route := "/" + strings.Join(routeParts, "/")
					// Clean up file extension
					route = strings.TrimSuffix(route, ".tsx")
					route = strings.TrimSuffix(route, ".ts")
					route = strings.TrimSuffix(route, ".jsx")
					route = strings.TrimSuffix(route, ".js")
					route = strings.TrimSuffix(route, ".vue")
					// Convert Next.js/Nuxt dynamic routes
					route = regexp.MustCompile(`\[([^\]]+)\]`).ReplaceAllString(route, ":$1")
					// Remove index
					route = strings.TrimSuffix(route, "/index")
					if route != "" && route != "/" {
						result.Routes = append(result.Routes, route)
					}
				}
				break
			}
		}
	}
}

// isValidRoute checks if a string looks like a valid route.
func (p *SourceMapParser) isValidRoute(route string) bool {
	if route == "" || len(route) > 200 {
		return false
	}
	if !strings.HasPrefix(route, "/") && !strings.HasPrefix(route, "#") {
		return false
	}
	// Filter out static assets
	staticExts := []string{".js", ".css", ".png", ".jpg", ".gif", ".svg", ".ico", ".woff", ".ttf"}
	for _, ext := range staticExts {
		if strings.HasSuffix(strings.ToLower(route), ext) {
			return false
		}
	}
	return true
}

// isValidEndpoint checks if a string looks like a valid API endpoint.
func (p *SourceMapParser) isValidEndpoint(endpoint string) bool {
	if endpoint == "" || len(endpoint) > 200 {
		return false
	}
	// Should start with / or be a full URL
	if !strings.HasPrefix(endpoint, "/") && !strings.HasPrefix(endpoint, "http") {
		return false
	}
	// Filter out static assets
	staticExts := []string{".js", ".css", ".png", ".jpg", ".gif", ".svg", ".ico", ".woff", ".ttf", ".html"}
	for _, ext := range staticExts {
		if strings.HasSuffix(strings.ToLower(endpoint), ext) {
			return false
		}
	}
	return true
}
