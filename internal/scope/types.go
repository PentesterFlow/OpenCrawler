package scope

// ScopeRules defines crawling scope rules.
type ScopeRules struct {
	IncludePatterns []string
	ExcludePatterns []string
	AllowedDomains  []string
	MaxDepth        int
	FollowExternal  bool
}
