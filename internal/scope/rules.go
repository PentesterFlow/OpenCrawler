package scope

import (
	"strings"
)

// DefaultExcludePatterns contains common patterns to exclude.
var DefaultExcludePatterns = []string{
	`.*[?&]logout.*`,
	`.*[?&]signout.*`,
	`.*[?&]exit.*`,
	`.*\/logout.*`,
	`.*\/signout.*`,
	`.*\/delete-account.*`,
	`.*\/unsubscribe.*`,
	`.*\/reset-password.*`,
	`.*\.pdf$`,
	`.*\.zip$`,
	`.*\.exe$`,
	`.*\.dmg$`,
}

// CommonAPIPatterns contains common API path patterns.
var CommonAPIPatterns = []string{
	`/api/`,
	`/v[0-9]+/`,
	`/graphql`,
	`/rest/`,
	`/rpc/`,
	`/ajax/`,
	`/json/`,
	`/xml/`,
}

// RuleBuilder helps build scope rules.
type RuleBuilder struct {
	rules ScopeRules
}

// NewRuleBuilder creates a new rule builder.
func NewRuleBuilder() *RuleBuilder {
	return &RuleBuilder{
		rules: ScopeRules{
			MaxDepth:       10,
			FollowExternal: false,
		},
	}
}

// WithIncludePatterns adds include patterns.
func (b *RuleBuilder) WithIncludePatterns(patterns ...string) *RuleBuilder {
	b.rules.IncludePatterns = append(b.rules.IncludePatterns, patterns...)
	return b
}

// WithExcludePatterns adds exclude patterns.
func (b *RuleBuilder) WithExcludePatterns(patterns ...string) *RuleBuilder {
	b.rules.ExcludePatterns = append(b.rules.ExcludePatterns, patterns...)
	return b
}

// WithDefaultExcludes adds default exclude patterns.
func (b *RuleBuilder) WithDefaultExcludes() *RuleBuilder {
	b.rules.ExcludePatterns = append(b.rules.ExcludePatterns, DefaultExcludePatterns...)
	return b
}

// WithAllowedDomains sets allowed domains.
func (b *RuleBuilder) WithAllowedDomains(domains ...string) *RuleBuilder {
	b.rules.AllowedDomains = append(b.rules.AllowedDomains, domains...)
	return b
}

// WithMaxDepth sets the maximum crawl depth.
func (b *RuleBuilder) WithMaxDepth(depth int) *RuleBuilder {
	b.rules.MaxDepth = depth
	return b
}

// WithFollowExternal enables following external links.
func (b *RuleBuilder) WithFollowExternal(follow bool) *RuleBuilder {
	b.rules.FollowExternal = follow
	return b
}

// Build returns the configured rules.
func (b *RuleBuilder) Build() ScopeRules {
	return b.rules
}

// MatchPattern checks if a URL matches a pattern.
func MatchPattern(url, pattern string) bool {
	// Simple wildcard matching
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		idx := 0
		for _, part := range parts {
			if part == "" {
				continue
			}
			newIdx := strings.Index(url[idx:], part)
			if newIdx == -1 {
				return false
			}
			idx += newIdx + len(part)
		}
		return true
	}

	return strings.Contains(url, pattern)
}

// IsAPIPath checks if a path looks like an API endpoint.
func IsAPIPath(path string) bool {
	path = strings.ToLower(path)

	for _, pattern := range CommonAPIPatterns {
		if MatchPattern(path, pattern) {
			return true
		}
	}

	// Check for common API indicators
	indicators := []string{
		".json",
		".xml",
		"callback=",
		"jsonp=",
		"format=json",
		"format=xml",
	}

	for _, ind := range indicators {
		if strings.Contains(path, ind) {
			return true
		}
	}

	return false
}

// ClassifyURL classifies a URL by its likely type.
func ClassifyURL(urlStr string) string {
	lower := strings.ToLower(urlStr)

	// API endpoints
	if IsAPIPath(lower) {
		return "api"
	}

	// Static assets
	staticExts := []string{".js", ".css", ".map"}
	for _, ext := range staticExts {
		if strings.HasSuffix(lower, ext) {
			return "static"
		}
	}

	// Authentication pages
	authIndicators := []string{"login", "signin", "auth", "oauth", "sso"}
	for _, ind := range authIndicators {
		if strings.Contains(lower, ind) {
			return "auth"
		}
	}

	// Admin pages
	adminIndicators := []string{"admin", "dashboard", "panel", "manage"}
	for _, ind := range adminIndicators {
		if strings.Contains(lower, ind) {
			return "admin"
		}
	}

	return "page"
}
