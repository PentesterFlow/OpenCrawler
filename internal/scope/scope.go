// Package scope provides URL scope checking for the crawler.
package scope

import (
	"net/url"
	"regexp"
	"strings"
	"sync"
)

// Checker validates URLs against scope rules.
type Checker struct {
	mu              sync.RWMutex
	rules           ScopeRules
	targetDomain    string
	includeRegexps  []*regexp.Regexp
	excludeRegexps  []*regexp.Regexp
	allowedDomains  map[string]struct{}
}

// NewChecker creates a new scope checker.
func NewChecker(targetURL string, rules ScopeRules) (*Checker, error) {
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	c := &Checker{
		rules:          rules,
		targetDomain:   parsed.Host,
		allowedDomains: make(map[string]struct{}),
	}

	// Compile include patterns
	for _, pattern := range rules.IncludePatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		c.includeRegexps = append(c.includeRegexps, re)
	}

	// Compile exclude patterns
	for _, pattern := range rules.ExcludePatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		c.excludeRegexps = append(c.excludeRegexps, re)
	}

	// Build allowed domains set
	c.allowedDomains[strings.ToLower(parsed.Host)] = struct{}{}
	for _, domain := range rules.AllowedDomains {
		c.allowedDomains[strings.ToLower(domain)] = struct{}{}
	}

	return c, nil
}

// IsInScope checks if a URL is within the crawling scope.
func (c *Checker) IsInScope(urlStr string, depth int) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Check depth
	if c.rules.MaxDepth > 0 && depth > c.rules.MaxDepth {
		return false
	}

	parsed, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// Check scheme
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}

	// Check domain
	if !c.isDomainAllowed(parsed.Host) {
		if !c.rules.FollowExternal {
			return false
		}
	}

	// Check exclude patterns first (higher priority)
	for _, re := range c.excludeRegexps {
		if re.MatchString(urlStr) {
			return false
		}
	}

	// Check include patterns (if any defined)
	if len(c.includeRegexps) > 0 {
		matched := false
		for _, re := range c.includeRegexps {
			if re.MatchString(urlStr) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// isDomainAllowed checks if a domain is allowed.
func (c *Checker) isDomainAllowed(host string) bool {
	host = strings.ToLower(host)

	// Direct match
	if _, ok := c.allowedDomains[host]; ok {
		return true
	}

	// Check for subdomain match
	for domain := range c.allowedDomains {
		if strings.HasSuffix(host, "."+domain) {
			return true
		}
	}

	return false
}

// AddAllowedDomain adds a domain to the allowed list.
func (c *Checker) AddAllowedDomain(domain string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.allowedDomains[strings.ToLower(domain)] = struct{}{}
}

// AddIncludePattern adds an include pattern.
func (c *Checker) AddIncludePattern(pattern string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.includeRegexps = append(c.includeRegexps, re)
	c.rules.IncludePatterns = append(c.rules.IncludePatterns, pattern)
	return nil
}

// AddExcludePattern adds an exclude pattern.
func (c *Checker) AddExcludePattern(pattern string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.excludeRegexps = append(c.excludeRegexps, re)
	c.rules.ExcludePatterns = append(c.rules.ExcludePatterns, pattern)
	return nil
}

// SetMaxDepth sets the maximum crawl depth.
func (c *Checker) SetMaxDepth(depth int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rules.MaxDepth = depth
}

// NormalizeURL normalizes a URL for deduplication.
func NormalizeURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	// Normalize scheme
	parsed.Scheme = strings.ToLower(parsed.Scheme)

	// Normalize host
	parsed.Host = strings.ToLower(parsed.Host)

	// Remove default ports
	if (parsed.Scheme == "http" && strings.HasSuffix(parsed.Host, ":80")) ||
		(parsed.Scheme == "https" && strings.HasSuffix(parsed.Host, ":443")) {
		parsed.Host = parsed.Host[:strings.LastIndex(parsed.Host, ":")]
	}

	// Remove fragment
	parsed.Fragment = ""

	// Remove trailing slash from path (except for root)
	if parsed.Path != "/" && strings.HasSuffix(parsed.Path, "/") {
		parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	}

	// Empty path should be /
	if parsed.Path == "" {
		parsed.Path = "/"
	}

	// Sort query parameters for consistent comparison
	if parsed.RawQuery != "" {
		values := parsed.Query()
		parsed.RawQuery = values.Encode()
	}

	return parsed.String(), nil
}

// ResolveURL resolves a relative URL against a base URL.
func ResolveURL(baseURL, relativeURL string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	ref, err := url.Parse(relativeURL)
	if err != nil {
		return "", err
	}

	resolved := base.ResolveReference(ref)
	return resolved.String(), nil
}

// IsValidURL checks if a URL is valid for crawling.
func IsValidURL(urlStr string) bool {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// Must have a scheme
	if parsed.Scheme == "" {
		return false
	}

	// Only http/https
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}

	// Must have a host
	if parsed.Host == "" {
		return false
	}

	// Skip common non-page extensions
	path := strings.ToLower(parsed.Path)
	skipExtensions := []string{
		".jpg", ".jpeg", ".png", ".gif", ".ico", ".svg", ".webp",
		".css", ".woff", ".woff2", ".ttf", ".eot",
		".pdf", ".zip", ".tar", ".gz", ".rar",
		".mp3", ".mp4", ".wav", ".avi", ".mov",
		".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
	}

	for _, ext := range skipExtensions {
		if strings.HasSuffix(path, ext) {
			return false
		}
	}

	return true
}

// ExtractDomain extracts the domain from a URL.
func ExtractDomain(urlStr string) (string, error) {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}
	return parsed.Host, nil
}
