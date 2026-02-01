package ratelimit

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RobotsManager manages robots.txt rules for multiple domains.
type RobotsManager struct {
	mu       sync.RWMutex
	rules    map[string]*RobotsRules
	client   *http.Client
	cache    time.Duration
}

// RobotsRules represents parsed robots.txt rules.
type RobotsRules struct {
	Disallow   []*regexp.Regexp
	Allow      []*regexp.Regexp
	CrawlDelay time.Duration
	Sitemaps   []string
	FetchedAt  time.Time
}

// NewRobotsManager creates a new robots.txt manager.
func NewRobotsManager() *RobotsManager {
	return &RobotsManager{
		rules: make(map[string]*RobotsRules),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		cache: 1 * time.Hour,
	}
}

// IsAllowed checks if a path is allowed by robots.txt.
func (m *RobotsManager) IsAllowed(domain, path, userAgent string) bool {
	m.mu.RLock()
	rules, exists := m.rules[domain]
	m.mu.RUnlock()

	if !exists {
		// Fetch robots.txt in background, allow by default
		go m.Fetch(context.Background(), domain)
		return true
	}

	// Check if cache expired
	if time.Since(rules.FetchedAt) > m.cache {
		go m.Fetch(context.Background(), domain)
	}

	return rules.IsAllowed(path)
}

// GetCrawlDelay returns the crawl delay for a domain.
func (m *RobotsManager) GetCrawlDelay(domain, userAgent string) time.Duration {
	m.mu.RLock()
	rules, exists := m.rules[domain]
	m.mu.RUnlock()

	if !exists {
		return 0
	}

	return rules.CrawlDelay
}

// Fetch fetches and parses robots.txt for a domain.
func (m *RobotsManager) Fetch(ctx context.Context, domain string) error {
	url := fmt.Sprintf("https://%s/robots.txt", domain)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "DAST-Crawler/1.0")

	resp, err := m.client.Do(req)
	if err != nil {
		// Try HTTP if HTTPS fails
		url = fmt.Sprintf("http://%s/robots.txt", domain)
		req, _ = http.NewRequestWithContext(ctx, "GET", url, nil)
		req.Header.Set("User-Agent", "DAST-Crawler/1.0")
		resp, err = m.client.Do(req)
		if err != nil {
			return err
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// No robots.txt or error - allow all
		m.mu.Lock()
		m.rules[domain] = &RobotsRules{FetchedAt: time.Now()}
		m.mu.Unlock()
		return nil
	}

	rules, err := ParseRobots(resp.Body, "DAST-Crawler")
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.rules[domain] = rules
	m.mu.Unlock()

	return nil
}

// GetSitemaps returns sitemap URLs for a domain.
func (m *RobotsManager) GetSitemaps(domain string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules, exists := m.rules[domain]
	if !exists {
		return nil
	}

	return rules.Sitemaps
}

// ParseRobots parses robots.txt content.
func ParseRobots(r io.Reader, userAgent string) (*RobotsRules, error) {
	rules := &RobotsRules{
		FetchedAt: time.Now(),
	}

	scanner := bufio.NewScanner(r)
	currentUserAgent := ""
	matchingUserAgent := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse directive
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		directive := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])

		switch directive {
		case "user-agent":
			currentUserAgent = strings.ToLower(value)
			matchingUserAgent = currentUserAgent == "*" ||
				strings.Contains(strings.ToLower(userAgent), currentUserAgent)

		case "disallow":
			if matchingUserAgent && value != "" {
				pattern := pathToRegexp(value)
				if re, err := regexp.Compile(pattern); err == nil {
					rules.Disallow = append(rules.Disallow, re)
				}
			}

		case "allow":
			if matchingUserAgent && value != "" {
				pattern := pathToRegexp(value)
				if re, err := regexp.Compile(pattern); err == nil {
					rules.Allow = append(rules.Allow, re)
				}
			}

		case "crawl-delay":
			if matchingUserAgent {
				if delay, err := strconv.ParseFloat(value, 64); err == nil {
					rules.CrawlDelay = time.Duration(delay * float64(time.Second))
				}
			}

		case "sitemap":
			rules.Sitemaps = append(rules.Sitemaps, value)
		}
	}

	return rules, scanner.Err()
}

// pathToRegexp converts a robots.txt path pattern to a regexp.
func pathToRegexp(path string) string {
	// Escape special regex characters except * and $
	pattern := regexp.QuoteMeta(path)

	// Convert * to .*
	pattern = strings.ReplaceAll(pattern, `\*`, ".*")

	// Convert $ at end to actual end-of-string
	if strings.HasSuffix(pattern, `\$`) {
		pattern = pattern[:len(pattern)-2] + "$"
	}

	// Anchor to start
	pattern = "^" + pattern

	return pattern
}

// IsAllowed checks if a path is allowed by the rules.
func (r *RobotsRules) IsAllowed(path string) bool {
	// Check allow rules first (higher priority)
	for _, re := range r.Allow {
		if re.MatchString(path) {
			return true
		}
	}

	// Check disallow rules
	for _, re := range r.Disallow {
		if re.MatchString(path) {
			return false
		}
	}

	// Default allow
	return true
}
