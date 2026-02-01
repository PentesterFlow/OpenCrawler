package crawler

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// =============================================================================
// DefaultConfig Tests
// =============================================================================

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config == nil {
		t.Fatal("DefaultConfig returned nil")
	}
	if config.Workers != 50 {
		t.Errorf("Workers = %d, want 50", config.Workers)
	}
	if config.MaxDepth != 10 {
		t.Errorf("MaxDepth = %d, want 10", config.MaxDepth)
	}
	if config.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", config.Timeout)
	}
	if config.RateLimit.RequestsPerSecond != 100 {
		t.Errorf("RateLimit.RequestsPerSecond = %v, want 100", config.RateLimit.RequestsPerSecond)
	}
	if !config.PassiveAPIDiscovery {
		t.Error("PassiveAPIDiscovery should be true")
	}
	if !config.ActiveAPIDiscovery {
		t.Error("ActiveAPIDiscovery should be true")
	}
	if !config.WebSocketDiscovery {
		t.Error("WebSocketDiscovery should be true")
	}
	if !config.FormAnalysis {
		t.Error("FormAnalysis should be true")
	}
	if !config.JSAnalysis {
		t.Error("JSAnalysis should be true")
	}
	if config.Auth.Type != AuthTypeNone {
		t.Errorf("Auth.Type = %v, want none", config.Auth.Type)
	}
}

// =============================================================================
// TurboConfig Tests
// =============================================================================

func TestTurboConfig(t *testing.T) {
	config := TurboConfig()

	if config == nil {
		t.Fatal("TurboConfig returned nil")
	}
	if config.Workers != 200 {
		t.Errorf("Workers = %d, want 200", config.Workers)
	}
	if config.RateLimit.RequestsPerSecond != 500 {
		t.Errorf("RateLimit.RequestsPerSecond = %v, want 500", config.RateLimit.RequestsPerSecond)
	}
	if config.Browser.PoolSize != 50 {
		t.Errorf("Browser.PoolSize = %d, want 50", config.Browser.PoolSize)
	}
	if config.RateLimit.RespectRobotsTxt {
		t.Error("RespectRobotsTxt should be false in turbo mode")
	}
	if !config.FastMode {
		t.Error("FastMode should be true")
	}
	if config.ActiveAPIDiscovery {
		t.Error("ActiveAPIDiscovery should be false in turbo mode")
	}
	if config.FormAnalysis {
		t.Error("FormAnalysis should be false in turbo mode")
	}
	if config.State.Enabled {
		t.Error("State should be disabled in turbo mode")
	}
}

// =============================================================================
// BalancedConfig Tests
// =============================================================================

func TestBalancedConfig(t *testing.T) {
	config := BalancedConfig()

	if config == nil {
		t.Fatal("BalancedConfig returned nil")
	}
	if config.Workers != 100 {
		t.Errorf("Workers = %d, want 100", config.Workers)
	}
	if config.RateLimit.RequestsPerSecond != 200 {
		t.Errorf("RateLimit.RequestsPerSecond = %v, want 200", config.RateLimit.RequestsPerSecond)
	}
	if config.Browser.PoolSize != 30 {
		t.Errorf("Browser.PoolSize = %d, want 30", config.Browser.PoolSize)
	}
	if !config.PassiveAPIDiscovery {
		t.Error("PassiveAPIDiscovery should be true")
	}
	if !config.ActiveAPIDiscovery {
		t.Error("ActiveAPIDiscovery should be true")
	}
	if config.JSAnalysis {
		t.Error("JSAnalysis should be false in balanced mode")
	}
}

// =============================================================================
// Validate Tests
// =============================================================================

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
	}{
		{
			name: "valid config",
			modify: func(c *Config) {
				c.Target = "https://example.com"
			},
			wantErr: false,
		},
		{
			name:    "missing target",
			modify:  func(c *Config) {},
			wantErr: true,
		},
		{
			name: "invalid workers",
			modify: func(c *Config) {
				c.Target = "https://example.com"
				c.Workers = 0
			},
			wantErr: true,
		},
		{
			name: "invalid max depth",
			modify: func(c *Config) {
				c.Target = "https://example.com"
				c.MaxDepth = 0
			},
			wantErr: true,
		},
		{
			name: "invalid browser pool",
			modify: func(c *Config) {
				c.Target = "https://example.com"
				c.Browser.PoolSize = 0
			},
			wantErr: true,
		},
		{
			name: "invalid rate limit",
			modify: func(c *Config) {
				c.Target = "https://example.com"
				c.RateLimit.RequestsPerSecond = 0
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			tt.modify(config)
			err := config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// =============================================================================
// Clone Tests
// =============================================================================

func TestConfig_Clone(t *testing.T) {
	original := DefaultConfig()
	original.Target = "https://example.com"
	original.Workers = 100
	original.CustomHeaders = map[string]string{"X-Test": "value"}

	clone := original.Clone()

	// Verify clone is equal
	if clone.Target != original.Target {
		t.Errorf("Target = %s, want %s", clone.Target, original.Target)
	}
	if clone.Workers != original.Workers {
		t.Errorf("Workers = %d, want %d", clone.Workers, original.Workers)
	}

	// Verify clone is independent
	clone.Workers = 200
	if original.Workers == 200 {
		t.Error("Modifying clone affected original")
	}
}

// =============================================================================
// SaveToFile/LoadFromFile Tests
// =============================================================================

func TestConfig_SaveToFile_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "config.json")

	config := DefaultConfig()
	config.Target = "https://example.com"
	config.Workers = 75

	err := config.SaveToFile(filePath)
	if err != nil {
		t.Fatalf("SaveToFile() error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// Load and verify
	loaded, err := LoadFromFile(filePath)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}
	if loaded.Target != config.Target {
		t.Errorf("Loaded Target = %s, want %s", loaded.Target, config.Target)
	}
	if loaded.Workers != config.Workers {
		t.Errorf("Loaded Workers = %d, want %d", loaded.Workers, config.Workers)
	}
}

func TestConfig_SaveToFile_YAML(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "config.yaml")

	config := DefaultConfig()
	config.Target = "https://example.com"

	err := config.SaveToFile(filePath)
	if err != nil {
		t.Fatalf("SaveToFile() error = %v", err)
	}

	// Load and verify
	loaded, err := LoadFromFile(filePath)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}
	if loaded.Target != config.Target {
		t.Errorf("Loaded Target = %s, want %s", loaded.Target, config.Target)
	}
}

func TestLoadFromFile_NonExistent(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/path/config.json")
	if err == nil {
		t.Error("LoadFromFile() should return error for non-existent file")
	}
}

func TestLoadFromFile_InvalidContent(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "invalid.json")

	// Write invalid content
	os.WriteFile(filePath, []byte("not json or yaml"), 0644)

	_, err := LoadFromFile(filePath)
	if err == nil {
		t.Error("LoadFromFile() should return error for invalid content")
	}
}

// =============================================================================
// AuthType Tests
// =============================================================================

func TestAuthType_Constants(t *testing.T) {
	// Just verify constants exist and have expected values
	tests := []struct {
		authType AuthType
		expected string
	}{
		{AuthTypeNone, "none"},
		{AuthTypeSession, "session"},
		{AuthTypeJWT, "jwt"},
		{AuthTypeOAuth, "oauth"},
		{AuthTypeFormLogin, "form"},
		{AuthTypeAPIKey, "apikey"},
		{AuthTypeBasic, "basic"},
	}

	for _, tt := range tests {
		if string(tt.authType) != tt.expected {
			t.Errorf("%v = %s, want %s", tt.authType, string(tt.authType), tt.expected)
		}
	}
}

// =============================================================================
// EnhancedDiscoveryConfig Tests
// =============================================================================

func TestDefaultConfig_EnhancedDiscovery(t *testing.T) {
	config := DefaultConfig()

	if !config.EnhancedDiscovery.Enabled {
		t.Error("EnhancedDiscovery.Enabled should be true by default")
	}
	if !config.EnhancedDiscovery.EnableRobots {
		t.Error("EnableRobots should be true by default")
	}
	if !config.EnhancedDiscovery.EnableSitemap {
		t.Error("EnableSitemap should be true by default")
	}
	if config.EnhancedDiscovery.EnablePathBrute {
		t.Error("EnablePathBrute should be false by default")
	}
	if config.EnhancedDiscovery.Concurrency != 10 {
		t.Errorf("Concurrency = %d, want 10", config.EnhancedDiscovery.Concurrency)
	}
}
