package enhanced

import (
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// JSExtractor extracts URLs and endpoints from JavaScript files.
type JSExtractor struct {
	client      *http.Client
	userAgent   string
	concurrency int
}

// JSExtractionResult contains extracted data from JS files.
type JSExtractionResult struct {
	SourceURL    string
	URLs         []string
	APIEndpoints []string
	Routes       []string
	Subdomains   []string
	Secrets      []SecretFinding
	Comments     []string
}

// NewJSExtractor creates a new JavaScript extractor.
func NewJSExtractor(userAgent string, concurrency int) *JSExtractor {
	if concurrency <= 0 {
		concurrency = 5
	}
	return &JSExtractor{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		userAgent:   userAgent,
		concurrency: concurrency,
	}
}

// ExtractFromURLs fetches and extracts from multiple JS URLs.
func (e *JSExtractor) ExtractFromURLs(jsURLs []string) []JSExtractionResult {
	results := make([]JSExtractionResult, 0)
	resultChan := make(chan JSExtractionResult, len(jsURLs))
	urlChan := make(chan string, len(jsURLs))

	var wg sync.WaitGroup
	for i := 0; i < e.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for jsURL := range urlChan {
				if result := e.extractFromURL(jsURL); result != nil {
					resultChan <- *result
				}
			}
		}()
	}

	go func() {
		for _, jsURL := range jsURLs {
			urlChan <- jsURL
		}
		close(urlChan)
	}()

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for result := range resultChan {
		results = append(results, result)
	}

	return results
}

// extractFromURL fetches and extracts from a single JS URL.
func (e *JSExtractor) extractFromURL(jsURL string) *JSExtractionResult {
	req, err := http.NewRequest("GET", jsURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", e.userAgent)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil
	}

	// Limit to 10MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil
	}

	content := string(body)
	baseURL, _ := url.Parse(jsURL)

	return e.ExtractFromContent(content, baseURL)
}

// ExtractFromContent extracts URLs and endpoints from JS content.
func (e *JSExtractor) ExtractFromContent(content string, baseURL *url.URL) *JSExtractionResult {
	result := &JSExtractionResult{
		URLs:         make([]string, 0),
		APIEndpoints: make([]string, 0),
		Routes:       make([]string, 0),
		Subdomains:   make([]string, 0),
		Secrets:      make([]SecretFinding, 0),
		Comments:     make([]string, 0),
	}

	if baseURL != nil {
		result.SourceURL = baseURL.String()
	}

	seen := make(map[string]bool)

	// Extract full URLs
	urlPatterns := []*regexp.Regexp{
		regexp.MustCompile(`https?://[a-zA-Z0-9][a-zA-Z0-9.-]*[a-zA-Z0-9](?:/[^\s"'<>()]*)?`),
		regexp.MustCompile(`//[a-zA-Z0-9][a-zA-Z0-9.-]*[a-zA-Z0-9](?:/[^\s"'<>()]*)?`),
	}

	for _, re := range urlPatterns {
		matches := re.FindAllString(content, -1)
		for _, match := range matches {
			// Clean up the URL
			match = cleanURL(match)
			if match != "" && !seen[match] && isValidExtractedURL(match) {
				seen[match] = true
				result.URLs = append(result.URLs, match)

				// Check if it's an API endpoint
				if isAPIEndpoint(match) {
					result.APIEndpoints = append(result.APIEndpoints, match)
				}

				// Extract subdomains
				if subdomain := extractSubdomain(match, baseURL); subdomain != "" {
					if !seen["subdomain:"+subdomain] {
						seen["subdomain:"+subdomain] = true
						result.Subdomains = append(result.Subdomains, subdomain)
					}
				}
			}
		}
	}

	// Extract relative paths
	pathPatterns := []*regexp.Regexp{
		// Paths in strings
		regexp.MustCompile(`["'](/[a-zA-Z0-9/_.-]+)["']`),
		// API paths
		regexp.MustCompile(`["'](/api/[a-zA-Z0-9/_.-]+)["']`),
		regexp.MustCompile(`["'](/v[0-9]+/[a-zA-Z0-9/_.-]+)["']`),
		// Route definitions
		regexp.MustCompile(`path\s*:\s*["']([^"']+)["']`),
		regexp.MustCompile(`route\s*:\s*["']([^"']+)["']`),
	}

	for _, re := range pathPatterns {
		matches := re.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				path := match[1]
				if !seen[path] && isValidPath(path) {
					seen[path] = true

					// Resolve to full URL if base URL available
					if baseURL != nil {
						fullURL, _ := baseURL.Parse(path)
						if fullURL != nil {
							result.URLs = append(result.URLs, fullURL.String())
						}
					}

					// Check if it's an API endpoint
					if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/v1/") ||
						strings.HasPrefix(path, "/v2/") || strings.HasPrefix(path, "/graphql") {
						result.APIEndpoints = append(result.APIEndpoints, path)
					}

					// Check if it's a route
					if isRoute(path) {
						result.Routes = append(result.Routes, path)
					}
				}
			}
		}
	}

	// Extract React/Vue/Angular routes
	routePatterns := []*regexp.Regexp{
		// React Router
		regexp.MustCompile(`<Route[^>]+path=["']([^"']+)["']`),
		regexp.MustCompile(`navigate\s*\(\s*["']([^"']+)["']`),
		regexp.MustCompile(`push\s*\(\s*["']([^"']+)["']`),
		// Vue Router
		regexp.MustCompile(`path\s*:\s*["']([^"']+)["']`),
		regexp.MustCompile(`name\s*:\s*["']([^"']+)["']`),
		// Angular
		regexp.MustCompile(`routerLink=["']([^"']+)["']`),
		// AngularJS
		regexp.MustCompile(`\.when\s*\(\s*["']([^"']+)["']`),
		regexp.MustCompile(`\.state\s*\(\s*["']([^"']+)["']`),
		regexp.MustCompile(`templateUrl\s*:\s*["']([^"']+)["']`),
	}

	for _, re := range routePatterns {
		matches := re.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				route := match[1]
				if !seen["route:"+route] && isValidRoute(route) {
					seen["route:"+route] = true
					result.Routes = append(result.Routes, route)
				}
			}
		}
	}

	// Extract secrets
	secretPatterns := map[string]*regexp.Regexp{
		"api_key":        regexp.MustCompile(`(?i)(?:api[_-]?key|apikey)\s*[=:]\s*["']([a-zA-Z0-9_-]{20,})["']`),
		"secret":         regexp.MustCompile(`(?i)(?:secret|password)\s*[=:]\s*["']([^"']{8,})["']`),
		"token":          regexp.MustCompile(`(?i)(?:token|bearer)\s*[=:]\s*["']([a-zA-Z0-9_.-]{20,})["']`),
		"aws_key":        regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
		"aws_secret":     regexp.MustCompile(`(?i)aws[_-]?secret[_-]?access[_-]?key\s*[=:]\s*["']([a-zA-Z0-9/+=]{40})["']`),
		"private_key":    regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA )?PRIVATE KEY-----`),
		"jwt":            regexp.MustCompile(`eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*`),
		"google_api":     regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`),
		"github_token":   regexp.MustCompile(`ghp_[0-9A-Za-z]{36}`),
		"firebase":       regexp.MustCompile(`(?i)firebase[_-]?(?:api[_-]?)?key\s*[=:]\s*["']([a-zA-Z0-9_-]+)["']`),
		"stripe_key":     regexp.MustCompile(`(?:sk|pk)_(?:test|live)_[0-9a-zA-Z]{24,}`),
		"slack_webhook":  regexp.MustCompile(`https://hooks\.slack\.com/services/[A-Z0-9]+/[A-Z0-9]+/[a-zA-Z0-9]+`),
		"twilio":         regexp.MustCompile(`SK[0-9a-fA-F]{32}`),
		"sendgrid":       regexp.MustCompile(`SG\.[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+`),
		"mailchimp":      regexp.MustCompile(`[0-9a-f]{32}-us[0-9]{1,2}`),
	}

	for secretType, re := range secretPatterns {
		matches := re.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			value := match[0]
			if len(match) > 1 {
				value = match[1]
			}
			key := secretType + ":" + value
			if !seen[key] {
				seen[key] = true

				// Get context
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
					File:    result.SourceURL,
					Context: context,
				})
			}
		}
	}

	// Extract comments (might contain useful info)
	commentPatterns := []*regexp.Regexp{
		regexp.MustCompile(`//\s*TODO:?\s*(.+)`),
		regexp.MustCompile(`//\s*FIXME:?\s*(.+)`),
		regexp.MustCompile(`//\s*HACK:?\s*(.+)`),
		regexp.MustCompile(`//\s*XXX:?\s*(.+)`),
		regexp.MustCompile(`//\s*BUG:?\s*(.+)`),
		regexp.MustCompile(`/\*\s*TODO:?\s*([^*]+)\*/`),
	}

	for _, re := range commentPatterns {
		matches := re.FindAllStringSubmatch(content, 10) // Limit to 10
		for _, match := range matches {
			if len(match) > 1 {
				comment := strings.TrimSpace(match[1])
				if len(comment) > 10 && len(comment) < 200 {
					result.Comments = append(result.Comments, comment)
				}
			}
		}
	}

	return result
}

// Helper functions

func cleanURL(rawURL string) string {
	// Remove trailing punctuation
	rawURL = strings.TrimRight(rawURL, ".,;:!?'\")}]>")

	// Remove any escape sequences
	rawURL = strings.ReplaceAll(rawURL, "\\", "")

	return rawURL
}

func isValidExtractedURL(rawURL string) bool {
	// Filter out data URLs, javascript:, etc
	if strings.HasPrefix(rawURL, "data:") ||
		strings.HasPrefix(rawURL, "javascript:") ||
		strings.HasPrefix(rawURL, "mailto:") ||
		strings.HasPrefix(rawURL, "tel:") {
		return false
	}

	// Filter out likely false positives
	if strings.Contains(rawURL, "example.com") ||
		strings.Contains(rawURL, "localhost") ||
		strings.Contains(rawURL, "127.0.0.1") ||
		strings.Contains(rawURL, "0.0.0.0") {
		return false
	}

	// Should have at least some path structure
	if len(rawURL) < 10 {
		return false
	}

	return true
}

func isAPIEndpoint(rawURL string) bool {
	apiIndicators := []string{
		"/api/", "/v1/", "/v2/", "/v3/", "/rest/",
		"/graphql", "/query", "/mutation",
		".json", ".xml",
	}

	urlLower := strings.ToLower(rawURL)
	for _, indicator := range apiIndicators {
		if strings.Contains(urlLower, indicator) {
			return true
		}
	}
	return false
}

func extractSubdomain(rawURL string, baseURL *url.URL) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	host := parsed.Hostname()
	if host == "" {
		return ""
	}

	// If we have a base URL, check if it's a subdomain of the same domain
	if baseURL != nil {
		baseHost := baseURL.Hostname()
		baseParts := strings.Split(baseHost, ".")
		hostParts := strings.Split(host, ".")

		// Check if they share the same root domain
		if len(baseParts) >= 2 && len(hostParts) >= 2 {
			baseRoot := strings.Join(baseParts[len(baseParts)-2:], ".")
			hostRoot := strings.Join(hostParts[len(hostParts)-2:], ".")

			if baseRoot == hostRoot && host != baseHost {
				return host
			}
		}
	}

	return ""
}

func isValidPath(path string) bool {
	if len(path) < 2 || len(path) > 200 {
		return false
	}

	if !strings.HasPrefix(path, "/") {
		return false
	}

	// Filter out static assets
	staticExts := []string{
		".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".svg",
		".ico", ".woff", ".woff2", ".ttf", ".eot", ".map",
	}
	for _, ext := range staticExts {
		if strings.HasSuffix(strings.ToLower(path), ext) {
			return false
		}
	}

	return true
}

func isRoute(path string) bool {
	// Routes typically don't have file extensions
	if strings.Contains(path, ".") {
		return false
	}

	// Should look like a path
	if !strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "#") {
		return false
	}

	return true
}

func isValidRoute(route string) bool {
	if len(route) < 1 || len(route) > 100 {
		return false
	}

	// Should start with / or #
	if !strings.HasPrefix(route, "/") && !strings.HasPrefix(route, "#") {
		return false
	}

	return true
}
