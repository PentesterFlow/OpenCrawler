package crawler

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/PentesterFlow/OpenCrawler/internal/browser"
	"gopkg.in/yaml.v3"
)

// Config holds all crawler configuration.
type Config struct {
	// Target URL to crawl
	Target string `json:"target" yaml:"target"`

	// Number of concurrent workers
	Workers int `json:"workers" yaml:"workers"`

	// Maximum crawl depth
	MaxDepth int `json:"max_depth" yaml:"max_depth"`

	// Request timeout
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// Scope rules
	Scope ScopeRules `json:"scope" yaml:"scope"`

	// Rate limiting
	RateLimit RateLimitConfig `json:"rate_limit" yaml:"rate_limit"`

	// Browser configuration
	Browser browser.Config `json:"browser" yaml:"browser"`

	// Authentication
	Auth AuthCredentials `json:"auth" yaml:"auth"`

	// Output configuration
	Output OutputConfig `json:"output" yaml:"output"`

	// State persistence
	State StateConfig `json:"state" yaml:"state"`

	// Enable passive API discovery
	PassiveAPIDiscovery bool `json:"passive_api_discovery" yaml:"passive_api_discovery"`

	// Enable active API probing
	ActiveAPIDiscovery bool `json:"active_api_discovery" yaml:"active_api_discovery"`

	// Enable WebSocket discovery
	WebSocketDiscovery bool `json:"websocket_discovery" yaml:"websocket_discovery"`

	// Enable form analysis
	FormAnalysis bool `json:"form_analysis" yaml:"form_analysis"`

	// Enable JavaScript analysis
	JSAnalysis bool `json:"js_analysis" yaml:"js_analysis"`

	// Enable AJAX discovery (intercept XHR/Fetch, trigger events, find AJAX forms)
	AJAXDiscovery bool `json:"ajax_discovery" yaml:"ajax_discovery"`

	// Fast mode skips heavy analysis (SPA framework detection, AJAX triggering) for speed
	FastMode bool `json:"fast_mode" yaml:"fast_mode"`

	// Custom headers to include in all requests
	CustomHeaders map[string]string `json:"custom_headers" yaml:"custom_headers"`

	// Cookies to include in all requests
	Cookies map[string]string `json:"cookies" yaml:"cookies"`

	// User agents to rotate (if empty, uses default)
	UserAgents []string `json:"user_agents" yaml:"user_agents"`

	// Proxy URL
	Proxy string `json:"proxy" yaml:"proxy"`

	// Verbose logging
	Verbose bool `json:"verbose" yaml:"verbose"`

	// Debug mode
	Debug bool `json:"debug" yaml:"debug"`

	// Enhanced discovery configuration
	EnhancedDiscovery EnhancedDiscoveryConfig `json:"enhanced_discovery" yaml:"enhanced_discovery"`
}

// EnhancedDiscoveryConfig holds configuration for enhanced discovery modules.
type EnhancedDiscoveryConfig struct {
	// Enable all enhanced discovery modules
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Enable robots.txt parsing
	EnableRobots bool `json:"enable_robots" yaml:"enable_robots"`

	// Enable sitemap.xml discovery
	EnableSitemap bool `json:"enable_sitemap" yaml:"enable_sitemap"`

	// Enable JavaScript source map parsing
	EnableSourceMaps bool `json:"enable_source_maps" yaml:"enable_source_maps"`

	// Enable common path bruteforcing
	EnablePathBrute bool `json:"enable_path_brute" yaml:"enable_path_brute"`

	// Enable technology fingerprinting
	EnableFingerprint bool `json:"enable_fingerprint" yaml:"enable_fingerprint"`

	// Enable parameter discovery
	EnableParamDiscovery bool `json:"enable_param_discovery" yaml:"enable_param_discovery"`

	// Enable JavaScript extraction
	EnableJSExtract bool `json:"enable_js_extract" yaml:"enable_js_extract"`

	// Concurrency for enhanced discovery operations
	Concurrency int `json:"concurrency" yaml:"concurrency"`
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Workers:  50,
		MaxDepth: 10,
		Timeout:  30 * time.Second,
		Scope: ScopeRules{
			MaxDepth:       10,
			FollowExternal: false,
		},
		RateLimit: RateLimitConfig{
			RequestsPerSecond: 100,
			Burst:             10,
			DelayBetween:      0,
			RespectRobotsTxt:  true,
		},
		Browser: browser.DefaultConfig(),
		Auth: AuthCredentials{
			Type: AuthTypeNone,
		},
		Output: OutputConfig{
			Format:     "json",
			Pretty:     true,
			StreamMode: false,
		},
		State: StateConfig{
			Enabled:  true,
			AutoSave: true,
			Interval: 60,
		},
		PassiveAPIDiscovery: true,
		ActiveAPIDiscovery:  true,
		WebSocketDiscovery:  true,
		FormAnalysis:        true,
		JSAnalysis:          true,
		AJAXDiscovery:       true,
		Verbose:             false,
		Debug:               false,
		EnhancedDiscovery: EnhancedDiscoveryConfig{
			Enabled:              true,
			EnableRobots:         true,
			EnableSitemap:        true,
			EnableSourceMaps:     true,
			EnablePathBrute:      false, // Disabled by default (can be noisy)
			EnableFingerprint:    true,
			EnableParamDiscovery: true,
			EnableJSExtract:      true,
			Concurrency:          10,
		},
	}
}

// TurboConfig returns a configuration optimized for MAXIMUM SPEED.
// Use this when you need to crawl as fast as possible.
// Warning: This may trigger rate limiting or WAF blocks on some sites.
func TurboConfig() *Config {
	return &Config{
		Workers:  200,                // Maximum parallelism
		MaxDepth: 10,
		Timeout:  10 * time.Second,   // Short timeout
		Scope: ScopeRules{
			MaxDepth:       10,
			FollowExternal: false,
		},
		RateLimit: RateLimitConfig{
			RequestsPerSecond: 500,    // Very high rate
			Burst:             100,    // Large burst
			DelayBetween:      0,
			RespectRobotsTxt:  false,  // Ignore robots.txt for speed
		},
		Browser: browser.Config{
			PoolSize:          50,     // Large browser pool
			Headless:          true,
			Timeout:           8 * time.Second, // Fast timeout
			UserAgent:         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			ViewportWidth:     1280,
			ViewportHeight:    720,
			RecycleAfter:      100,    // Recycle less often
			IgnoreHTTPSErrors: true,
			FastMode:          true,   // Skip heavy operations
		},
		Auth: AuthCredentials{
			Type: AuthTypeNone,
		},
		Output: OutputConfig{
			Format:     "json",
			Pretty:     false,         // No pretty print (faster)
			StreamMode: true,          // Stream results
		},
		State: StateConfig{
			Enabled:  false,           // Disable state persistence
			AutoSave: false,
		},
		PassiveAPIDiscovery: true,     // Keep passive discovery (low overhead)
		ActiveAPIDiscovery:  false,    // Skip active probing
		WebSocketDiscovery:  false,    // Skip WebSocket
		FormAnalysis:        false,    // Skip form analysis
		JSAnalysis:          false,    // Skip JS analysis
		AJAXDiscovery:       false,    // Skip AJAX analysis
		FastMode:            true,     // Enable fast mode globally
		Verbose:             false,
		Debug:               false,
		EnhancedDiscovery: EnhancedDiscoveryConfig{
			Enabled:              false, // Disable all enhanced discovery
		},
	}
}

// BalancedConfig returns a configuration that balances speed with thoroughness.
func BalancedConfig() *Config {
	return &Config{
		Workers:  100,
		MaxDepth: 10,
		Timeout:  15 * time.Second,
		Scope: ScopeRules{
			MaxDepth:       10,
			FollowExternal: false,
		},
		RateLimit: RateLimitConfig{
			RequestsPerSecond: 200,
			Burst:             50,
			DelayBetween:      0,
			RespectRobotsTxt:  true,
		},
		Browser: browser.Config{
			PoolSize:          30,
			Headless:          true,
			Timeout:           12 * time.Second,
			UserAgent:         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			ViewportWidth:     1920,
			ViewportHeight:    1080,
			RecycleAfter:      50,
			IgnoreHTTPSErrors: true,
			FastMode:          false,
		},
		Auth: AuthCredentials{
			Type: AuthTypeNone,
		},
		Output: OutputConfig{
			Format:     "json",
			Pretty:     true,
			StreamMode: false,
		},
		State: StateConfig{
			Enabled:  true,
			AutoSave: true,
			Interval: 30,
		},
		PassiveAPIDiscovery: true,
		ActiveAPIDiscovery:  true,
		WebSocketDiscovery:  true,
		FormAnalysis:        true,
		JSAnalysis:          false,    // Skip expensive JS analysis
		AJAXDiscovery:       true,
		FastMode:            false,
		Verbose:             false,
		Debug:               false,
		EnhancedDiscovery: EnhancedDiscoveryConfig{
			Enabled:              true,
			EnableRobots:         true,
			EnableSitemap:        true,
			EnableSourceMaps:     false,
			EnablePathBrute:      false,
			EnableFingerprint:    true,
			EnableParamDiscovery: false,
			EnableJSExtract:      false,
			Concurrency:          20,
		},
	}
}

// LoadFromFile loads configuration from a file (JSON or YAML).
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := DefaultConfig()

	// Try YAML first, then JSON
	if err := yaml.Unmarshal(data, config); err != nil {
		if err := json.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	return config, nil
}

// SaveToFile saves configuration to a file.
func (c *Config) SaveToFile(path string) error {
	var data []byte
	var err error

	if len(path) > 5 && path[len(path)-5:] == ".json" {
		data, err = json.MarshalIndent(c, "", "  ")
	} else {
		data, err = yaml.Marshal(c)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.Target == "" {
		return fmt.Errorf("target URL is required")
	}

	if c.Workers < 1 {
		return fmt.Errorf("workers must be at least 1")
	}

	if c.MaxDepth < 1 {
		return fmt.Errorf("max depth must be at least 1")
	}

	if c.Browser.PoolSize < 1 {
		return fmt.Errorf("browser pool size must be at least 1")
	}

	if c.RateLimit.RequestsPerSecond <= 0 {
		return fmt.Errorf("rate limit must be positive")
	}

	return nil
}

// Clone creates a deep copy of the configuration.
func (c *Config) Clone() *Config {
	data, _ := json.Marshal(c)
	clone := &Config{}
	json.Unmarshal(data, clone)
	return clone
}
