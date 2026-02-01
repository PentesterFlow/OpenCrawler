package output

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockFlusher implements io.Writer with Flush support
type mockFlusher struct {
	bytes.Buffer
	flushed bool
}

func (m *mockFlusher) Flush() error {
	m.flushed = true
	return nil
}

// mockCloser implements io.Writer with Close support
type mockCloser struct {
	bytes.Buffer
	closed bool
}

func (m *mockCloser) Close() error {
	m.closed = true
	return nil
}

// mockWriteError simulates write errors
type mockWriteError struct {
	err error
}

func (m *mockWriteError) Write(p []byte) (n int, err error) {
	return 0, m.err
}

func TestNewJSONWriter(t *testing.T) {
	tests := []struct {
		name   string
		pretty bool
		stream bool
	}{
		{
			name:   "compact non-stream",
			pretty: false,
			stream: false,
		},
		{
			name:   "pretty non-stream",
			pretty: true,
			stream: false,
		},
		{
			name:   "compact stream",
			pretty: false,
			stream: true,
		},
		{
			name:   "pretty stream",
			pretty: true,
			stream: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			jw := NewJSONWriter(&buf, tt.pretty, tt.stream)

			if jw == nil {
				t.Fatal("NewJSONWriter returned nil")
			}
			if jw.pretty != tt.pretty {
				t.Errorf("pretty = %v, want %v", jw.pretty, tt.pretty)
			}
			if jw.stream != tt.stream {
				t.Errorf("stream = %v, want %v", jw.stream, tt.stream)
			}
			if jw.closed {
				t.Error("writer should not be closed initially")
			}
			if !jw.first {
				t.Error("first should be true initially")
			}
		})
	}
}

func TestJSONWriter_WriteResult(t *testing.T) {
	tests := []struct {
		name       string
		pretty     bool
		result     *CrawlResult
		wantFields []string
	}{
		{
			name:   "compact output",
			pretty: false,
			result: &CrawlResult{
				Target:    "https://example.com",
				StartedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
				Stats: CrawlStats{
					URLsDiscovered: 100,
					PagesCrawled:   50,
				},
			},
			wantFields: []string{"target", "started_at", "stats"},
		},
		{
			name:   "pretty output",
			pretty: true,
			result: &CrawlResult{
				Target: "https://example.com",
				Endpoints: []Endpoint{
					{URL: "https://example.com/api/users", Method: "GET"},
				},
			},
			wantFields: []string{"target", "endpoints"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			jw := NewJSONWriter(&buf, tt.pretty, false)

			err := jw.WriteResult(tt.result)
			if err != nil {
				t.Fatalf("WriteResult() error = %v", err)
			}

			output := buf.String()
			for _, field := range tt.wantFields {
				if !strings.Contains(output, field) {
					t.Errorf("output missing field %q", field)
				}
			}

			// Verify it's valid JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
				t.Errorf("output is not valid JSON: %v", err)
			}

			// Verify pretty formatting
			if tt.pretty {
				if !strings.Contains(output, "\n  ") {
					t.Error("pretty output should contain indentation")
				}
			}
		})
	}
}

func TestJSONWriter_WriteResult_Closed(t *testing.T) {
	var buf bytes.Buffer
	jw := NewJSONWriter(&buf, false, false)
	jw.Close()

	err := jw.WriteResult(&CrawlResult{Target: "https://example.com"})
	if err != nil {
		t.Errorf("WriteResult on closed writer should return nil, got %v", err)
	}
	if buf.Len() != 0 {
		t.Error("closed writer should not write anything")
	}
}

func TestJSONWriter_WriteResult_WriteError(t *testing.T) {
	errWriter := &mockWriteError{err: io.ErrShortWrite}
	jw := NewJSONWriter(errWriter, false, false)

	err := jw.WriteResult(&CrawlResult{Target: "https://example.com"})
	if err == nil {
		t.Error("expected error on write failure")
	}
}

func TestJSONWriter_WriteEndpoint_StreamMode(t *testing.T) {
	var buf bytes.Buffer
	jw := NewJSONWriter(&buf, false, true) // stream mode enabled

	endpoint := &Endpoint{
		URL:        "https://example.com/api/users",
		Method:     "GET",
		Source:     "passive",
		StatusCode: 200,
	}

	err := jw.WriteEndpoint(endpoint)
	if err != nil {
		t.Fatalf("WriteEndpoint() error = %v", err)
	}

	output := buf.String()

	// Verify it contains the stream event wrapper
	if !strings.Contains(output, `"type":"endpoint"`) {
		t.Error("stream output should contain type:endpoint")
	}
	if !strings.Contains(output, `"data"`) {
		t.Error("stream output should contain data field")
	}

	// Verify it's valid JSON
	var event StreamEvent
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}
	if event.Type != "endpoint" {
		t.Errorf("event.Type = %q, want %q", event.Type, "endpoint")
	}
}

func TestJSONWriter_WriteEndpoint_NonStreamMode(t *testing.T) {
	var buf bytes.Buffer
	jw := NewJSONWriter(&buf, false, false) // stream mode disabled

	endpoint := &Endpoint{
		URL:    "https://example.com/api/users",
		Method: "GET",
	}

	err := jw.WriteEndpoint(endpoint)
	if err != nil {
		t.Fatalf("WriteEndpoint() error = %v", err)
	}

	// In non-stream mode, nothing should be written
	if buf.Len() != 0 {
		t.Errorf("non-stream mode should not write anything, got %q", buf.String())
	}
}

func TestJSONWriter_WriteEndpoint_Closed(t *testing.T) {
	var buf bytes.Buffer
	jw := NewJSONWriter(&buf, false, true)
	jw.Close()

	err := jw.WriteEndpoint(&Endpoint{URL: "https://example.com"})
	if err != nil {
		t.Errorf("WriteEndpoint on closed writer should return nil, got %v", err)
	}
	if buf.Len() != 0 {
		t.Error("closed writer should not write anything")
	}
}

func TestJSONWriter_WriteForm_StreamMode(t *testing.T) {
	var buf bytes.Buffer
	jw := NewJSONWriter(&buf, false, true)

	form := &Form{
		URL:     "https://example.com/login",
		Action:  "/auth/login",
		Method:  "POST",
		Enctype: "application/x-www-form-urlencoded",
		Inputs: []FormInput{
			{Name: "username", Type: "text", Required: true},
			{Name: "password", Type: "password", Required: true},
		},
		HasCSRF: true,
	}

	err := jw.WriteForm(form)
	if err != nil {
		t.Fatalf("WriteForm() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"type":"form"`) {
		t.Error("stream output should contain type:form")
	}

	var event StreamEvent
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}
}

func TestJSONWriter_WriteForm_NonStreamMode(t *testing.T) {
	var buf bytes.Buffer
	jw := NewJSONWriter(&buf, false, false)

	err := jw.WriteForm(&Form{URL: "https://example.com/login"})
	if err != nil {
		t.Fatalf("WriteForm() error = %v", err)
	}

	if buf.Len() != 0 {
		t.Error("non-stream mode should not write anything")
	}
}

func TestJSONWriter_WriteWebSocket_StreamMode(t *testing.T) {
	var buf bytes.Buffer
	jw := NewJSONWriter(&buf, false, true)

	ws := &WebSocketEndpoint{
		URL:            "wss://example.com/ws/notifications",
		DiscoveredFrom: "https://example.com/dashboard",
		Protocols:      []string{"wamp", "mqtt"},
	}

	err := jw.WriteWebSocket(ws)
	if err != nil {
		t.Fatalf("WriteWebSocket() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"type":"websocket"`) {
		t.Error("stream output should contain type:websocket")
	}

	var event StreamEvent
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}
}

func TestJSONWriter_WriteWebSocket_NonStreamMode(t *testing.T) {
	var buf bytes.Buffer
	jw := NewJSONWriter(&buf, false, false)

	err := jw.WriteWebSocket(&WebSocketEndpoint{URL: "wss://example.com/ws"})
	if err != nil {
		t.Fatalf("WriteWebSocket() error = %v", err)
	}

	if buf.Len() != 0 {
		t.Error("non-stream mode should not write anything")
	}
}

func TestJSONWriter_WriteError_StreamMode(t *testing.T) {
	var buf bytes.Buffer
	jw := NewJSONWriter(&buf, false, true)

	crawlErr := &CrawlError{
		URL:       "https://example.com/broken",
		Error:     "connection refused",
		Timestamp: time.Now(),
	}

	err := jw.WriteError(crawlErr)
	if err != nil {
		t.Fatalf("WriteError() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"type":"error"`) {
		t.Error("stream output should contain type:error")
	}

	var event StreamEvent
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}
}

func TestJSONWriter_WriteError_NonStreamMode(t *testing.T) {
	var buf bytes.Buffer
	jw := NewJSONWriter(&buf, false, false)

	err := jw.WriteError(&CrawlError{URL: "https://example.com/broken"})
	if err != nil {
		t.Fatalf("WriteError() error = %v", err)
	}

	if buf.Len() != 0 {
		t.Error("non-stream mode should not write anything")
	}
}

func TestJSONWriter_WriteStreamEvent_Pretty(t *testing.T) {
	var buf bytes.Buffer
	jw := NewJSONWriter(&buf, true, true) // pretty + stream

	endpoint := &Endpoint{
		URL:    "https://example.com/api",
		Method: "GET",
	}

	err := jw.WriteEndpoint(endpoint)
	if err != nil {
		t.Fatalf("WriteEndpoint() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "\n  ") {
		t.Error("pretty stream output should contain indentation")
	}
}

func TestJSONWriter_Flush(t *testing.T) {
	t.Run("with flushable writer", func(t *testing.T) {
		flusher := &mockFlusher{}
		jw := NewJSONWriter(flusher, false, false)

		err := jw.Flush()
		if err != nil {
			t.Fatalf("Flush() error = %v", err)
		}

		if !flusher.flushed {
			t.Error("Flush() should call underlying writer's Flush")
		}
	})

	t.Run("with non-flushable writer", func(t *testing.T) {
		var buf bytes.Buffer
		jw := NewJSONWriter(&buf, false, false)

		err := jw.Flush()
		if err != nil {
			t.Fatalf("Flush() on non-flushable writer should return nil, got %v", err)
		}
	})
}

func TestJSONWriter_Close(t *testing.T) {
	t.Run("with closable writer", func(t *testing.T) {
		closer := &mockCloser{}
		jw := NewJSONWriter(closer, false, false)

		err := jw.Close()
		if err != nil {
			t.Fatalf("Close() error = %v", err)
		}

		if !closer.closed {
			t.Error("Close() should call underlying writer's Close")
		}

		if !jw.closed {
			t.Error("writer should be marked as closed")
		}
	})

	t.Run("with non-closable writer", func(t *testing.T) {
		var buf bytes.Buffer
		jw := NewJSONWriter(&buf, false, false)

		err := jw.Close()
		if err != nil {
			t.Fatalf("Close() on non-closable writer should return nil, got %v", err)
		}

		if !jw.closed {
			t.Error("writer should be marked as closed")
		}
	})
}

func TestJSONWriter_Concurrent(t *testing.T) {
	var buf bytes.Buffer
	jw := NewJSONWriter(&buf, false, true)

	var wg sync.WaitGroup
	numGoroutines := 10
	numWrites := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numWrites; j++ {
				endpoint := &Endpoint{
					URL:    "https://example.com/api",
					Method: "GET",
				}
				jw.WriteEndpoint(endpoint)
			}
		}(i)
	}

	wg.Wait()

	// Verify we got output (may be interleaved but shouldn't crash)
	if buf.Len() == 0 {
		t.Error("expected output from concurrent writes")
	}
}

func TestNewWriter(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "json format",
			config: Config{
				Format: "json",
				Pretty: true,
				Stream: false,
			},
		},
		{
			name: "default format",
			config: Config{
				Format: "",
				Pretty: false,
				Stream: true,
			},
		},
		{
			name: "unknown format defaults to json",
			config: Config{
				Format: "xml",
				Pretty: false,
				Stream: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := NewWriter(&buf, tt.config)

			if w == nil {
				t.Fatal("NewWriter returned nil")
			}

			// Verify it's a JSONWriter
			jw, ok := w.(*JSONWriter)
			if !ok {
				t.Fatal("NewWriter should return a JSONWriter")
			}

			if jw.pretty != tt.config.Pretty {
				t.Errorf("pretty = %v, want %v", jw.pretty, tt.config.Pretty)
			}
			if jw.stream != tt.config.Stream {
				t.Errorf("stream = %v, want %v", jw.stream, tt.config.Stream)
			}
		})
	}
}

func TestProgressWriter_WriteEndpoint(t *testing.T) {
	var buf bytes.Buffer
	jw := NewJSONWriter(&buf, false, true)

	var receivedStats ProgressStats
	pw := NewProgressWriter(jw, func(stats ProgressStats) {
		receivedStats = stats
	})

	endpoint := &Endpoint{URL: "https://example.com/api", Method: "GET"}
	err := pw.WriteEndpoint(endpoint)
	if err != nil {
		t.Fatalf("WriteEndpoint() error = %v", err)
	}

	if receivedStats.APIEndpoints != 1 {
		t.Errorf("APIEndpoints = %d, want 1", receivedStats.APIEndpoints)
	}
}

func TestProgressWriter_WriteForm(t *testing.T) {
	var buf bytes.Buffer
	jw := NewJSONWriter(&buf, false, true)

	var receivedStats ProgressStats
	pw := NewProgressWriter(jw, func(stats ProgressStats) {
		receivedStats = stats
	})

	form := &Form{URL: "https://example.com/login", Method: "POST"}
	err := pw.WriteForm(form)
	if err != nil {
		t.Fatalf("WriteForm() error = %v", err)
	}

	if receivedStats.FormsFound != 1 {
		t.Errorf("FormsFound = %d, want 1", receivedStats.FormsFound)
	}
}

func TestProgressWriter_WriteWebSocket(t *testing.T) {
	var buf bytes.Buffer
	jw := NewJSONWriter(&buf, false, true)

	var receivedStats ProgressStats
	pw := NewProgressWriter(jw, func(stats ProgressStats) {
		receivedStats = stats
	})

	ws := &WebSocketEndpoint{URL: "wss://example.com/ws"}
	err := pw.WriteWebSocket(ws)
	if err != nil {
		t.Fatalf("WriteWebSocket() error = %v", err)
	}

	if receivedStats.WebSockets != 1 {
		t.Errorf("WebSockets = %d, want 1", receivedStats.WebSockets)
	}
}

func TestProgressWriter_WriteError(t *testing.T) {
	var buf bytes.Buffer
	jw := NewJSONWriter(&buf, false, true)

	var receivedStats ProgressStats
	pw := NewProgressWriter(jw, func(stats ProgressStats) {
		receivedStats = stats
	})

	crawlErr := &CrawlError{URL: "https://example.com/broken", Error: "timeout"}
	err := pw.WriteError(crawlErr)
	if err != nil {
		t.Fatalf("WriteError() error = %v", err)
	}

	if receivedStats.Errors != 1 {
		t.Errorf("Errors = %d, want 1", receivedStats.Errors)
	}
}

func TestProgressWriter_NilCallback(t *testing.T) {
	var buf bytes.Buffer
	jw := NewJSONWriter(&buf, false, true)

	// Create with nil callback
	pw := NewProgressWriter(jw, nil)

	// Should not panic with nil callback
	err := pw.WriteEndpoint(&Endpoint{URL: "https://example.com"})
	if err != nil {
		t.Fatalf("WriteEndpoint() error = %v", err)
	}

	err = pw.WriteForm(&Form{URL: "https://example.com"})
	if err != nil {
		t.Fatalf("WriteForm() error = %v", err)
	}

	err = pw.WriteWebSocket(&WebSocketEndpoint{URL: "wss://example.com"})
	if err != nil {
		t.Fatalf("WriteWebSocket() error = %v", err)
	}

	err = pw.WriteError(&CrawlError{URL: "https://example.com"})
	if err != nil {
		t.Fatalf("WriteError() error = %v", err)
	}
}

func TestStreamEvent_Serialization(t *testing.T) {
	event := StreamEvent{
		Type: "endpoint",
		Data: map[string]interface{}{
			"url":    "https://example.com",
			"method": "GET",
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var parsed StreamEvent
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if parsed.Type != event.Type {
		t.Errorf("Type = %q, want %q", parsed.Type, event.Type)
	}
}

func TestCrawlResult_Serialization(t *testing.T) {
	now := time.Now()
	result := &CrawlResult{
		Target:      "https://example.com",
		StartedAt:   now,
		CompletedAt: now.Add(time.Hour),
		Stats: CrawlStats{
			URLsDiscovered:     1000,
			PagesCrawled:       500,
			FormsFound:         25,
			APIEndpoints:       150,
			WebSocketEndpoints: 5,
			ErrorCount:         10,
			Duration:           time.Hour,
			BytesTransferred:   1024 * 1024 * 100,
		},
		Endpoints: []Endpoint{
			{
				URL:        "https://example.com/api/users",
				Method:     "GET",
				Source:     "passive",
				Depth:      2,
				StatusCode: 200,
				Parameters: []Parameter{
					{Name: "page", Type: "query", Example: "1"},
				},
			},
		},
		Forms: []Form{
			{
				URL:     "https://example.com/login",
				Action:  "/auth",
				Method:  "POST",
				HasCSRF: true,
				Inputs: []FormInput{
					{Name: "username", Type: "text", Required: true},
				},
			},
		},
		WebSockets: []WebSocketEndpoint{
			{
				URL:       "wss://example.com/ws",
				Protocols: []string{"wamp"},
			},
		},
		Errors: []CrawlError{
			{URL: "https://example.com/broken", Error: "timeout"},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var parsed CrawlResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if parsed.Target != result.Target {
		t.Errorf("Target = %q, want %q", parsed.Target, result.Target)
	}
	if parsed.Stats.URLsDiscovered != result.Stats.URLsDiscovered {
		t.Errorf("URLsDiscovered = %d, want %d", parsed.Stats.URLsDiscovered, result.Stats.URLsDiscovered)
	}
	if len(parsed.Endpoints) != len(result.Endpoints) {
		t.Errorf("len(Endpoints) = %d, want %d", len(parsed.Endpoints), len(result.Endpoints))
	}
	if len(parsed.Forms) != len(result.Forms) {
		t.Errorf("len(Forms) = %d, want %d", len(parsed.Forms), len(result.Forms))
	}
}

func TestTypes_JSONTags(t *testing.T) {
	// Test that all types have proper JSON tags and serialize correctly
	t.Run("SummaryReport", func(t *testing.T) {
		report := SummaryReport{
			Target:    "https://example.com",
			StartedAt: time.Now(),
		}
		data, err := json.Marshal(report)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}
		if !strings.Contains(string(data), "target") {
			t.Error("missing target field")
		}
	})

	t.Run("Statistics", func(t *testing.T) {
		stats := Statistics{
			TotalURLs:         100,
			CrawledPages:      50,
			DiscoveredForms:   10,
			SkippedOutOfScope: 5,
		}
		data, err := json.Marshal(stats)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}
		if !strings.Contains(string(data), "total_urls") {
			t.Error("missing total_urls field")
		}
	})

	t.Run("EndpointSummary", func(t *testing.T) {
		summary := EndpointSummary{
			Total:    100,
			ByMethod: map[string]int{"GET": 50, "POST": 50},
		}
		data, err := json.Marshal(summary)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}
		if !strings.Contains(string(data), "by_method") {
			t.Error("missing by_method field")
		}
	})

	t.Run("FormSummary", func(t *testing.T) {
		summary := FormSummary{
			Total:      20,
			WithCSRF:   15,
			FileUpload: 3,
		}
		data, err := json.Marshal(summary)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}
		if !strings.Contains(string(data), "with_csrf") {
			t.Error("missing with_csrf field")
		}
	})

	t.Run("SecurityFindings", func(t *testing.T) {
		findings := SecurityFindings{
			MissingCSRF:    []string{"/form1", "/form2"},
			FileUploads:    []string{"/upload"},
			DebugEndpoints: []string{"/debug", "/trace"},
		}
		data, err := json.Marshal(findings)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}
		if !strings.Contains(string(data), "missing_csrf") {
			t.Error("missing missing_csrf field")
		}
	})

	t.Run("DetailedReport", func(t *testing.T) {
		report := DetailedReport{
			Summary: SummaryReport{Target: "https://example.com"},
			Metadata: CrawlMetadata{
				CrawlerVersion: "1.0.0",
				UserAgent:      "DAST-Crawler/1.0",
			},
		}
		data, err := json.Marshal(report)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}
		if !strings.Contains(string(data), "crawler_version") {
			t.Error("missing crawler_version field")
		}
	})
}

func TestParameter_Serialization(t *testing.T) {
	param := Parameter{
		Name:     "page",
		Type:     "query",
		Example:  "1",
		Required: true,
	}

	data, err := json.Marshal(param)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed Parameter
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.Name != param.Name {
		t.Errorf("Name = %q, want %q", parsed.Name, param.Name)
	}
	if parsed.Required != param.Required {
		t.Errorf("Required = %v, want %v", parsed.Required, param.Required)
	}
}

func TestFormInput_Serialization(t *testing.T) {
	input := FormInput{
		Name:        "email",
		Type:        "email",
		Required:    true,
		Placeholder: "user@example.com",
		Pattern:     `^[a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+\.[a-zA-Z0-9-.]+$`,
		MaxLength:   255,
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed FormInput
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.Pattern != input.Pattern {
		t.Errorf("Pattern = %q, want %q", parsed.Pattern, input.Pattern)
	}
	if parsed.MaxLength != input.MaxLength {
		t.Errorf("MaxLength = %d, want %d", parsed.MaxLength, input.MaxLength)
	}
}

func TestWebSocketMsg_Serialization(t *testing.T) {
	msg := WebSocketMsg{
		Direction: "incoming",
		Type:      "text",
		Data:      `{"event":"notification","payload":{}}`,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed WebSocketMsg
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.Direction != msg.Direction {
		t.Errorf("Direction = %q, want %q", parsed.Direction, msg.Direction)
	}
	if parsed.Data != msg.Data {
		t.Errorf("Data = %q, want %q", parsed.Data, msg.Data)
	}
}

func TestTimingInfo_Serialization(t *testing.T) {
	timing := TimingInfo{
		TotalDuration:   time.Hour,
		AveragePageTime: 500 * time.Millisecond,
		FastestPage:     100 * time.Millisecond,
		SlowestPage:     5 * time.Second,
		RequestsPerSec:  10.5,
	}

	data, err := json.Marshal(timing)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	if !strings.Contains(string(data), "requests_per_second") {
		t.Error("missing requests_per_second field")
	}
}

func TestExposedAPI_Serialization(t *testing.T) {
	api := ExposedAPI{
		URL:    "https://example.com/admin/users",
		Method: "DELETE",
		Reason: "unauthenticated access",
	}

	data, err := json.Marshal(api)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed ExposedAPI
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.Reason != api.Reason {
		t.Errorf("Reason = %q, want %q", parsed.Reason, api.Reason)
	}
}
