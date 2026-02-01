package state

import (
	"crypto/md5"
	"encoding/hex"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// HashAwareDeduplicator provides URL deduplication that properly handles hash-based SPAs.
type HashAwareDeduplicator struct {
	mu            sync.RWMutex
	visitedURLs   map[string]bool   // Normalized URL -> visited
	contentHashes map[string]string // Normalized URL -> content hash
	urlToContent  map[string]string // Maps URL to its content hash for duplicate detection
	maxSize       int
}

// NewHashAwareDeduplicator creates a new hash-aware deduplicator.
func NewHashAwareDeduplicator(maxSize int) *HashAwareDeduplicator {
	return &HashAwareDeduplicator{
		visitedURLs:   make(map[string]bool),
		contentHashes: make(map[string]string),
		urlToContent:  make(map[string]string),
		maxSize:       maxSize,
	}
}

// NormalizeURL normalizes a URL for deduplication.
func (d *HashAwareDeduplicator) NormalizeURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	// Normalize scheme
	parsed.Scheme = strings.ToLower(parsed.Scheme)

	// Normalize host
	parsed.Host = strings.ToLower(parsed.Host)

	// Remove default ports
	if (parsed.Scheme == "http" && strings.HasSuffix(parsed.Host, ":80")) ||
		(parsed.Scheme == "https" && strings.HasSuffix(parsed.Host, ":443")) {
		parsed.Host = strings.TrimSuffix(parsed.Host, ":80")
		parsed.Host = strings.TrimSuffix(parsed.Host, ":443")
	}

	// Normalize path
	parsed.Path = normalizePath(parsed.Path)

	// Sort query parameters
	if parsed.RawQuery != "" {
		parsed.RawQuery = normalizeQuery(parsed.RawQuery)
	}

	// Normalize fragment (hash) for SPAs
	if parsed.Fragment != "" {
		parsed.Fragment = normalizeFragment(parsed.Fragment)
	}

	return parsed.String()
}

// normalizePath normalizes a URL path.
func normalizePath(path string) string {
	if path == "" {
		return "/"
	}

	// Remove trailing slash (except for root)
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}

	// Remove duplicate slashes
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}

	// Resolve . and ..
	parts := strings.Split(path, "/")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case ".":
			continue
		case "..":
			if len(result) > 0 {
				result = result[:len(result)-1]
			}
		default:
			result = append(result, part)
		}
	}

	return strings.Join(result, "/")
}

// normalizeQuery normalizes query parameters.
func normalizeQuery(query string) string {
	params, err := url.ParseQuery(query)
	if err != nil {
		return query
	}

	// Remove tracking and cache-busting parameters
	removeParams := []string{
		"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content",
		"_ga", "gclid", "fbclid", "ref", "source",
		"_", "timestamp", "t", "nocache", "cache",
		"PHPSESSID", "jsessionid", "sid", "session_id",
	}

	for _, param := range removeParams {
		params.Del(param)
		params.Del(strings.ToUpper(param))
	}

	// Sort parameters
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Rebuild query
	var parts []string
	for _, k := range keys {
		for _, v := range params[k] {
			parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}

	return strings.Join(parts, "&")
}

// normalizeFragment normalizes the fragment (hash) for SPA routing.
func normalizeFragment(fragment string) string {
	// Remove hashbang prefix
	fragment = strings.TrimPrefix(fragment, "!")

	// If it looks like a route, normalize it
	if strings.HasPrefix(fragment, "/") || isLikelyRoute(fragment) {
		// Handle query params in fragment
		if idx := strings.Index(fragment, "?"); idx != -1 {
			path := fragment[:idx]
			query := fragment[idx+1:]

			// Normalize the path part
			path = normalizePath(path)

			// Normalize and filter query params
			query = normalizeFragmentQuery(query)

			if query != "" {
				return path + "?" + query
			}
			return path
		}

		return normalizePath(fragment)
	}

	// For non-route fragments, check if it's UI state (should be ignored)
	if isUIStateFragment(fragment) {
		return "" // Ignore UI state fragments
	}

	return fragment
}

// normalizeFragmentQuery normalizes query parameters within a fragment.
func normalizeFragmentQuery(query string) string {
	params, err := url.ParseQuery(query)
	if err != nil {
		return query
	}

	// Remove UI state parameters
	uiStateParams := []string{
		"modal", "popup", "dialog", "overlay", "drawer",
		"scroll", "scrollTop", "scrollY", "scrollX",
		"tab", "panel", "accordion", "section",
		"expanded", "collapsed", "open", "closed",
		"highlight", "focus", "selected", "active",
		"view", "layout", "display",
	}

	for _, param := range uiStateParams {
		params.Del(param)
	}

	if len(params) == 0 {
		return ""
	}

	// Sort and rebuild
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		for _, v := range params[k] {
			parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}

	return strings.Join(parts, "&")
}

// isLikelyRoute checks if a fragment looks like a route.
func isLikelyRoute(fragment string) bool {
	// Starts with /
	if strings.HasPrefix(fragment, "/") {
		return true
	}

	// Contains path-like structure
	if strings.Contains(fragment, "/") && !strings.Contains(fragment, " ") {
		return true
	}

	// Looks like a state name (e.g., "users.profile")
	if regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9._-]+$`).MatchString(fragment) {
		return true
	}

	return false
}

// isUIStateFragment checks if a fragment is UI state (not a route).
func isUIStateFragment(fragment string) bool {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`^modal[-=]`),
		regexp.MustCompile(`^popup[-=]`),
		regexp.MustCompile(`^tab[-=]`),
		regexp.MustCompile(`^panel[-=]`),
		regexp.MustCompile(`^section[-=]`),
		regexp.MustCompile(`^scroll[-=]?\d*$`),
		regexp.MustCompile(`^page[-=]?\d+$`),
		regexp.MustCompile(`^offset[-=]?\d+$`),
		regexp.MustCompile(`^[a-z]+-\d+$`),  // element-123 style anchors
		regexp.MustCompile(`^\d+$`),          // Just a number
		regexp.MustCompile(`^[a-f0-9]{32}$`), // MD5 hash
		regexp.MustCompile(`^[a-f0-9]{40}$`), // SHA1 hash
	}

	fragmentLower := strings.ToLower(fragment)
	for _, pattern := range patterns {
		if pattern.MatchString(fragmentLower) {
			return true
		}
	}

	return false
}

// HasVisited checks if a URL has been visited.
func (d *HashAwareDeduplicator) HasVisited(rawURL string) bool {
	normalized := d.NormalizeURL(rawURL)

	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.visitedURLs[normalized]
}

// MarkVisited marks a URL as visited.
func (d *HashAwareDeduplicator) MarkVisited(rawURL string) {
	normalized := d.NormalizeURL(rawURL)

	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.visitedURLs) < d.maxSize {
		d.visitedURLs[normalized] = true
	}
}

// SetContentHash sets the content hash for a URL.
func (d *HashAwareDeduplicator) SetContentHash(rawURL string, contentHash string) {
	normalized := d.NormalizeURL(rawURL)

	d.mu.Lock()
	defer d.mu.Unlock()

	d.contentHashes[normalized] = contentHash
	d.urlToContent[normalized] = contentHash
}

// HasDuplicateContent checks if a URL has the same content as another visited URL.
func (d *HashAwareDeduplicator) HasDuplicateContent(rawURL string, contentHash string) (bool, string) {
	normalized := d.NormalizeURL(rawURL)

	d.mu.RLock()
	defer d.mu.RUnlock()

	// Check if we've seen this content hash before
	for url, hash := range d.contentHashes {
		if hash == contentHash && url != normalized {
			return true, url
		}
	}

	return false, ""
}

// GetContentHash returns the content hash for a URL if available.
func (d *HashAwareDeduplicator) GetContentHash(rawURL string) (string, bool) {
	normalized := d.NormalizeURL(rawURL)

	d.mu.RLock()
	defer d.mu.RUnlock()

	hash, exists := d.contentHashes[normalized]
	return hash, exists
}

// ShouldSkipFragment returns true if this fragment should be skipped.
func (d *HashAwareDeduplicator) ShouldSkipFragment(fragment string) bool {
	return isUIStateFragment(fragment)
}

// ExtractRoutingFragment extracts the routing-relevant part of a fragment.
func (d *HashAwareDeduplicator) ExtractRoutingFragment(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	fragment := parsed.Fragment
	if fragment == "" {
		return ""
	}

	// Remove hashbang
	fragment = strings.TrimPrefix(fragment, "!")

	// If it's UI state, return empty
	if isUIStateFragment(fragment) {
		return ""
	}

	// Normalize
	return normalizeFragment(fragment)
}

// Stats returns deduplicator statistics.
func (d *HashAwareDeduplicator) Stats() map[string]int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return map[string]int{
		"visited_urls":   len(d.visitedURLs),
		"content_hashes": len(d.contentHashes),
	}
}

// GetAll returns all visited URLs.
func (d *HashAwareDeduplicator) GetAll() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	urls := make([]string, 0, len(d.visitedURLs))
	for url := range d.visitedURLs {
		urls = append(urls, url)
	}
	return urls
}

// ComputeContentHash computes an MD5 hash of content.
func ComputeContentHash(content string) string {
	hash := md5.Sum([]byte(content))
	return hex.EncodeToString(hash[:])
}

// AddBatch adds multiple URLs as visited.
func (d *HashAwareDeduplicator) AddBatch(urls []string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, rawURL := range urls {
		normalized := d.NormalizeURL(rawURL)
		if len(d.visitedURLs) < d.maxSize {
			d.visitedURLs[normalized] = true
		}
	}
}
