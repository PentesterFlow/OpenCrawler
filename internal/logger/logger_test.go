package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	cfg := DefaultConfig()
	l := New(cfg)

	if l == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNewDefault(t *testing.T) {
	l := NewDefault()

	if l == nil {
		t.Fatal("NewDefault() returned nil")
	}
}

func TestNewJSON(t *testing.T) {
	l := NewJSON(InfoLevel)

	if l == nil {
		t.Fatal("NewJSON() returned nil")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Level != InfoLevel {
		t.Errorf("Level = %v, want InfoLevel", cfg.Level)
	}
	if !cfg.Pretty {
		t.Error("Pretty should be true by default")
	}
	if cfg.Output == nil {
		t.Error("Output should not be nil")
	}
}

func TestLogger_WithComponent(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  InfoLevel,
		Pretty: false,
		Output: &buf,
	})

	l = l.WithComponent("test-component")
	l.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "test-component") {
		t.Errorf("Output should contain component: %s", output)
	}
}

func TestLogger_WithField(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  InfoLevel,
		Pretty: false,
		Output: &buf,
	})

	l = l.WithField("custom_field", "custom_value")
	l.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "custom_field") {
		t.Errorf("Output should contain custom_field: %s", output)
	}
	if !strings.Contains(output, "custom_value") {
		t.Errorf("Output should contain custom_value: %s", output)
	}
}

func TestLogger_WithFields(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  InfoLevel,
		Pretty: false,
		Output: &buf,
	})

	l = l.WithFields(map[string]interface{}{
		"field1": "value1",
		"field2": 123,
	})
	l.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "field1") {
		t.Errorf("Output should contain field1: %s", output)
	}
	if !strings.Contains(output, "field2") {
		t.Errorf("Output should contain field2: %s", output)
	}
}

func TestLogger_WithURL(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  InfoLevel,
		Pretty: false,
		Output: &buf,
	})

	l = l.WithURL("https://example.com/test")
	l.Info("crawling")

	output := buf.String()
	if !strings.Contains(output, "https://example.com/test") {
		t.Errorf("Output should contain URL: %s", output)
	}
}

func TestLogger_WithWorker(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  InfoLevel,
		Pretty: false,
		Output: &buf,
	})

	l = l.WithWorker(42)
	l.Info("processing")

	output := buf.String()
	if !strings.Contains(output, "42") {
		t.Errorf("Output should contain worker ID: %s", output)
	}
}

func TestLogger_WithDepth(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  InfoLevel,
		Pretty: false,
		Output: &buf,
	})

	l = l.WithDepth(5)
	l.Info("at depth")

	output := buf.String()
	if !strings.Contains(output, "depth") {
		t.Errorf("Output should contain depth field: %s", output)
	}
}

func TestLogger_WithError(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  InfoLevel,
		Pretty: false,
		Output: &buf,
	})

	l = l.WithError(nil) // Even nil error should work
	l.Info("error context")

	// Just verify no panic
}

func TestLogger_WithDuration(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  InfoLevel,
		Pretty: false,
		Output: &buf,
	})

	l = l.WithDuration(500 * time.Millisecond)
	l.Info("completed")

	output := buf.String()
	if !strings.Contains(output, "duration") {
		t.Errorf("Output should contain duration: %s", output)
	}
}

func TestLogger_Debug(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  DebugLevel,
		Pretty: false,
		Output: &buf,
	})

	l.Debug("debug message")

	output := buf.String()
	if !strings.Contains(output, "debug message") {
		t.Errorf("Output should contain message: %s", output)
	}
	if !strings.Contains(output, "debug") {
		t.Errorf("Output should contain level debug: %s", output)
	}
}

func TestLogger_Debugf(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  DebugLevel,
		Pretty: false,
		Output: &buf,
	})

	l.Debugf("debug %s %d", "test", 123)

	output := buf.String()
	if !strings.Contains(output, "debug test 123") {
		t.Errorf("Output should contain formatted message: %s", output)
	}
}

func TestLogger_Info(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  InfoLevel,
		Pretty: false,
		Output: &buf,
	})

	l.Info("info message")

	output := buf.String()
	if !strings.Contains(output, "info message") {
		t.Errorf("Output should contain message: %s", output)
	}
}

func TestLogger_Infof(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  InfoLevel,
		Pretty: false,
		Output: &buf,
	})

	l.Infof("info %s", "formatted")

	output := buf.String()
	if !strings.Contains(output, "info formatted") {
		t.Errorf("Output should contain formatted message: %s", output)
	}
}

func TestLogger_Warn(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  WarnLevel,
		Pretty: false,
		Output: &buf,
	})

	l.Warn("warning message")

	output := buf.String()
	if !strings.Contains(output, "warning message") {
		t.Errorf("Output should contain message: %s", output)
	}
}

func TestLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  ErrorLevel,
		Pretty: false,
		Output: &buf,
	})

	l.Error("error message")

	output := buf.String()
	if !strings.Contains(output, "error message") {
		t.Errorf("Output should contain message: %s", output)
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  WarnLevel, // Only warn and above
		Pretty: false,
		Output: &buf,
	})

	l.Debug("debug")
	l.Info("info")
	l.Warn("warning")
	l.Error("error")

	output := buf.String()

	// Debug and Info should be filtered
	if strings.Contains(output, "debug") {
		t.Error("Debug should be filtered")
	}
	if strings.Contains(output, `"info"`) {
		t.Error("Info should be filtered")
	}

	// Warn and Error should be present
	if !strings.Contains(output, "warning") {
		t.Error("Warning should be present")
	}
	if !strings.Contains(output, "error") {
		t.Error("Error should be present")
	}
}

func TestLogger_RequestEvent(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  InfoLevel,
		Pretty: false,
		Output: &buf,
	})

	l.RequestEvent("GET", "https://example.com", 200, 100*time.Millisecond)

	output := buf.String()
	if !strings.Contains(output, "GET") {
		t.Errorf("Output should contain method: %s", output)
	}
	if !strings.Contains(output, "https://example.com") {
		t.Errorf("Output should contain URL: %s", output)
	}
	if !strings.Contains(output, "200") {
		t.Errorf("Output should contain status code: %s", output)
	}
}

func TestLogger_DiscoveryEvent(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  InfoLevel,
		Pretty: false,
		Output: &buf,
	})

	l.DiscoveryEvent("api", "https://example.com/api", "passive")

	output := buf.String()
	if !strings.Contains(output, "api") {
		t.Errorf("Output should contain type: %s", output)
	}
	if !strings.Contains(output, "passive") {
		t.Errorf("Output should contain source: %s", output)
	}
}

func TestLogger_StatsEvent(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  InfoLevel,
		Pretty: false,
		Output: &buf,
	})

	l.StatsEvent(map[string]interface{}{
		"pages_crawled": 100,
		"errors":        5,
	})

	output := buf.String()
	if !strings.Contains(output, "pages_crawled") {
		t.Errorf("Output should contain pages_crawled: %s", output)
	}
}

func TestLogger_SetLevel(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  DebugLevel,
		Pretty: false,
		Output: &buf,
	})

	l.Debug("should appear")
	l.SetLevel(ErrorLevel)
	l.Debug("should not appear")

	output := buf.String()
	if !strings.Contains(output, "should appear") {
		t.Error("First debug should appear")
	}
	// After SetLevel, debug won't appear (new events only)
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  Level
	}{
		{"debug", DebugLevel},
		{"info", InfoLevel},
		{"warn", WarnLevel},
		{"error", ErrorLevel},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseLevel(tt.input)
			if err != nil {
				t.Fatalf("ParseLevel() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("ParseLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGlobalLogger(t *testing.T) {
	// Test that global logger works
	l := Global()
	if l == nil {
		t.Fatal("Global() returned nil")
	}

	// Test SetGlobal
	var buf bytes.Buffer
	newLogger := New(Config{
		Level:  InfoLevel,
		Pretty: false,
		Output: &buf,
	})
	SetGlobal(newLogger)

	Info("global test")

	output := buf.String()
	if !strings.Contains(output, "global test") {
		t.Errorf("Output should contain message: %s", output)
	}

	// Reset global logger
	SetGlobal(NewDefault())
}

func TestGlobalConvenienceFunctions(t *testing.T) {
	var buf bytes.Buffer
	SetGlobal(New(Config{
		Level:  DebugLevel,
		Pretty: false,
		Output: &buf,
	}))

	Debug("debug msg")
	Debugf("debug %d", 1)
	Info("info msg")
	Infof("info %d", 2)
	Warn("warn msg")
	Warnf("warn %d", 3)
	Error("error msg")
	Errorf("error %d", 4)

	output := buf.String()
	if !strings.Contains(output, "debug msg") {
		t.Error("Missing debug msg")
	}
	if !strings.Contains(output, "info msg") {
		t.Error("Missing info msg")
	}
	if !strings.Contains(output, "warn msg") {
		t.Error("Missing warn msg")
	}
	if !strings.Contains(output, "error msg") {
		t.Error("Missing error msg")
	}

	// Reset
	SetGlobal(NewDefault())
}

func TestLogger_JSONOutput(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  InfoLevel,
		Pretty: false,
		Output: &buf,
	})

	l.Info("json test")

	// Verify output is valid JSON
	var data map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &data); err != nil {
		t.Errorf("Output is not valid JSON: %v", err)
	}

	if data["message"] != "json test" {
		t.Errorf("Message = %v, want 'json test'", data["message"])
	}
	if data["level"] != "info" {
		t.Errorf("Level = %v, want 'info'", data["level"])
	}
}

func TestLogger_CrawlEvent(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  InfoLevel,
		Pretty: false,
		Output: &buf,
	})

	l.CrawlEvent(InfoLevel, "https://example.com", 3, 5).Msg("crawling page")

	output := buf.String()
	if !strings.Contains(output, "https://example.com") {
		t.Errorf("Output should contain URL: %s", output)
	}
	if !strings.Contains(output, "3") {
		t.Errorf("Output should contain depth: %s", output)
	}
	if !strings.Contains(output, "5") {
		t.Errorf("Output should contain worker_id: %s", output)
	}
}

func TestLogger_ErrorEvent(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  ErrorLevel,
		Pretty: false,
		Output: &buf,
	})

	l.ErrorEvent(nil, "https://example.com/error", "fetch")

	output := buf.String()
	if !strings.Contains(output, "https://example.com/error") {
		t.Errorf("Output should contain URL: %s", output)
	}
	if !strings.Contains(output, "fetch") {
		t.Errorf("Output should contain operation: %s", output)
	}
}

func TestLogger_ChainedContexts(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  InfoLevel,
		Pretty: false,
		Output: &buf,
	})

	// Chain multiple contexts
	l = l.WithComponent("crawler").
		WithWorker(1).
		WithURL("https://example.com").
		WithDepth(2)

	l.Info("chained context")

	output := buf.String()
	if !strings.Contains(output, "crawler") {
		t.Errorf("Output should contain component: %s", output)
	}
	if !strings.Contains(output, "https://example.com") {
		t.Errorf("Output should contain URL: %s", output)
	}
}
