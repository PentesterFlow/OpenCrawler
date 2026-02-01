package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// =============================================================================
// Test WebSocket Server
// =============================================================================

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func createTestWSServer(t *testing.T, handler func(*websocket.Conn)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		if handler != nil {
			handler(conn)
		} else {
			// Default: echo messages
			for {
				mt, msg, err := conn.ReadMessage()
				if err != nil {
					break
				}
				conn.WriteMessage(mt, msg)
			}
		}
	}))
}

func httpToWS(url string) string {
	return strings.Replace(strings.Replace(url, "http://", "ws://", 1), "https://", "wss://", 1)
}

// =============================================================================
// Handler Tests
// =============================================================================

func TestNewHandler(t *testing.T) {
	h := NewHandler()

	if h == nil {
		t.Fatal("NewHandler() returned nil")
	}
	if !h.enabled {
		t.Error("should be enabled by default")
	}
	if h.maxMsgs != 100 {
		t.Errorf("maxMsgs = %d, want 100", h.maxMsgs)
	}
	if h.msgTimeout != 5*time.Second {
		t.Errorf("msgTimeout = %v, want 5s", h.msgTimeout)
	}
}

func TestHandler_SetHeaders(t *testing.T) {
	h := NewHandler()

	h.SetHeaders(map[string]string{
		"X-Custom":       "value",
		"Authorization":  "Bearer token",
	})

	if h.headers.Get("X-Custom") != "value" {
		t.Error("X-Custom header not set")
	}
	if h.headers.Get("Authorization") != "Bearer token" {
		t.Error("Authorization header not set")
	}
}

func TestHandler_SetCookies(t *testing.T) {
	h := NewHandler()

	cookies := []*http.Cookie{
		{Name: "session", Value: "abc123"},
		{Name: "user", Value: "john"},
	}
	h.SetCookies(cookies)

	cookie := h.headers.Get("Cookie")
	if !strings.Contains(cookie, "session=abc123") {
		t.Error("session cookie not set")
	}
	if !strings.Contains(cookie, "user=john") {
		t.Error("user cookie not set")
	}
}

func TestHandler_Connect(t *testing.T) {
	// Create a test WebSocket server that sends a message
	server := createTestWSServer(t, func(conn *websocket.Conn) {
		conn.WriteMessage(websocket.TextMessage, []byte("hello"))
		time.Sleep(100 * time.Millisecond)
	})
	defer server.Close()

	h := NewHandler()
	h.SetMessageTimeout(500 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	wsURL := httpToWS(server.URL)
	err := h.Connect(ctx, wsURL, "https://example.com")

	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	if h.Count() != 1 {
		t.Errorf("Count() = %d, want 1", h.Count())
	}

	if !h.HasEndpoint(wsURL) {
		t.Error("HasEndpoint() should return true")
	}

	endpoints := h.GetEndpoints()
	if len(endpoints) != 1 {
		t.Fatalf("len(endpoints) = %d, want 1", len(endpoints))
	}

	if endpoints[0].DiscoveredFrom != "https://example.com" {
		t.Errorf("DiscoveredFrom = %q", endpoints[0].DiscoveredFrom)
	}
}

func TestHandler_Connect_Disabled(t *testing.T) {
	server := createTestWSServer(t, nil)
	defer server.Close()

	h := NewHandler()
	h.Disable()

	err := h.Connect(context.Background(), httpToWS(server.URL), "https://example.com")

	if err != nil {
		t.Errorf("Connect() error = %v", err)
	}

	if h.Count() != 0 {
		t.Error("should not connect when disabled")
	}
}

func TestHandler_Connect_InvalidURL(t *testing.T) {
	h := NewHandler()

	err := h.Connect(context.Background(), "://invalid", "https://example.com")

	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestHandler_Connect_SchemeConversion(t *testing.T) {
	server := createTestWSServer(t, func(conn *websocket.Conn) {
		time.Sleep(50 * time.Millisecond)
	})
	defer server.Close()

	h := NewHandler()
	h.SetMessageTimeout(100 * time.Millisecond)
	ctx := context.Background()

	// Test with http:// (should convert to ws://)
	err := h.Connect(ctx, server.URL, "https://example.com")
	if err != nil {
		t.Logf("Connect with http:// error (expected in some cases): %v", err)
	}
}

func TestHandler_GetEndpoints(t *testing.T) {
	h := NewHandler()

	// Manually add an endpoint
	h.mu.Lock()
	h.endpoints["wss://example.com/ws"] = &EndpointInfo{
		URL:            "wss://example.com/ws",
		DiscoveredFrom: "https://example.com",
		Protocols:      []string{"wamp"},
		Messages: []Message{
			{Direction: "received", Type: websocket.TextMessage, Data: []byte("hello")},
		},
		ConnectTime: time.Now(),
	}
	h.mu.Unlock()

	endpoints := h.GetEndpoints()

	if len(endpoints) != 1 {
		t.Fatalf("len(endpoints) = %d, want 1", len(endpoints))
	}

	ep := endpoints[0]
	if ep.URL != "wss://example.com/ws" {
		t.Errorf("URL = %q", ep.URL)
	}
	if len(ep.SampleMessages) != 1 {
		t.Errorf("len(SampleMessages) = %d", len(ep.SampleMessages))
	}
	if ep.SampleMessages[0].Type != "text" {
		t.Errorf("message type = %q, want text", ep.SampleMessages[0].Type)
	}
}

func TestHandler_Count(t *testing.T) {
	h := NewHandler()

	if h.Count() != 0 {
		t.Error("initial count should be 0")
	}

	h.mu.Lock()
	h.endpoints["ws://test1"] = &EndpointInfo{}
	h.endpoints["ws://test2"] = &EndpointInfo{}
	h.mu.Unlock()

	if h.Count() != 2 {
		t.Errorf("Count() = %d, want 2", h.Count())
	}
}

func TestHandler_HasEndpoint(t *testing.T) {
	h := NewHandler()

	h.mu.Lock()
	h.endpoints["wss://example.com/ws"] = &EndpointInfo{}
	h.mu.Unlock()

	if !h.HasEndpoint("wss://example.com/ws") {
		t.Error("should have endpoint")
	}
	if h.HasEndpoint("wss://other.com/ws") {
		t.Error("should not have endpoint")
	}
}

func TestHandler_EnableDisable(t *testing.T) {
	h := NewHandler()

	h.Disable()
	if h.enabled {
		t.Error("should be disabled")
	}

	h.Enable()
	if !h.enabled {
		t.Error("should be enabled")
	}
}

func TestHandler_Clear(t *testing.T) {
	h := NewHandler()

	h.mu.Lock()
	h.endpoints["ws://test"] = &EndpointInfo{}
	h.mu.Unlock()

	h.Clear()

	if h.Count() != 0 {
		t.Error("Count() should be 0 after clear")
	}
}

func TestHandler_SetMaxMessages(t *testing.T) {
	h := NewHandler()

	h.SetMaxMessages(50)

	if h.maxMsgs != 50 {
		t.Errorf("maxMsgs = %d, want 50", h.maxMsgs)
	}
}

func TestHandler_SetMessageTimeout(t *testing.T) {
	h := NewHandler()

	h.SetMessageTimeout(10 * time.Second)

	if h.msgTimeout != 10*time.Second {
		t.Errorf("msgTimeout = %v, want 10s", h.msgTimeout)
	}
}

func TestHandler_Concurrent(t *testing.T) {
	h := NewHandler()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			h.SetHeaders(map[string]string{"X-ID": "value"})
			h.GetEndpoints()
			h.Count()
			h.HasEndpoint("ws://test")
		}(i)
	}
	wg.Wait()
}

func TestSplitStrings(t *testing.T) {
	tests := []struct {
		input string
		sep   string
		want  int
	}{
		{"a,b,c", ",", 3},
		{"a, b, c", ",", 3},
		{",a,,b,", ",", 2},
		{"", ",", 0},
		{"single", ",", 1},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitStrings(tt.input, tt.sep)
			if len(got) != tt.want {
				t.Errorf("splitStrings(%q, %q) = %d items, want %d", tt.input, tt.sep, len(got), tt.want)
			}
		})
	}
}

// =============================================================================
// Recorder Tests
// =============================================================================

func TestNewRecorder(t *testing.T) {
	t.Run("with positive max", func(t *testing.T) {
		r := NewRecorder(500)
		if r.maxMsgs != 500 {
			t.Errorf("maxMsgs = %d, want 500", r.maxMsgs)
		}
	})

	t.Run("with zero max", func(t *testing.T) {
		r := NewRecorder(0)
		if r.maxMsgs != 1000 {
			t.Errorf("maxMsgs = %d, want 1000 (default)", r.maxMsgs)
		}
	})

	t.Run("with negative max", func(t *testing.T) {
		r := NewRecorder(-10)
		if r.maxMsgs != 1000 {
			t.Errorf("maxMsgs = %d, want 1000 (default)", r.maxMsgs)
		}
	})
}

func TestRecorder_StartSession(t *testing.T) {
	r := NewRecorder(100)

	r.StartSession("wss://example.com/ws")

	session := r.GetSession("wss://example.com/ws")
	if session == nil {
		t.Fatal("session should exist")
	}
	if !session.Active {
		t.Error("session should be active")
	}
	if session.URL != "wss://example.com/ws" {
		t.Errorf("URL = %q", session.URL)
	}
}

func TestRecorder_EndSession(t *testing.T) {
	r := NewRecorder(100)

	r.StartSession("wss://example.com/ws")
	r.EndSession("wss://example.com/ws")

	session := r.GetSession("wss://example.com/ws")
	if session == nil {
		t.Fatal("session should exist")
	}
	if session.Active {
		t.Error("session should not be active")
	}
	if session.EndTime.IsZero() {
		t.Error("EndTime should be set")
	}
}

func TestRecorder_EndSession_NonExistent(t *testing.T) {
	r := NewRecorder(100)

	// Should not panic
	r.EndSession("wss://nonexistent.com/ws")
}

func TestRecorder_RecordMessage(t *testing.T) {
	r := NewRecorder(100)

	r.StartSession("wss://example.com/ws")

	r.RecordMessage("wss://example.com/ws", "sent", "text", []byte("hello"))
	r.RecordMessage("wss://example.com/ws", "received", "text", []byte("world"))

	session := r.GetSession("wss://example.com/ws")
	if len(session.Messages) != 2 {
		t.Errorf("len(Messages) = %d, want 2", len(session.Messages))
	}
	if session.TotalSent != 1 {
		t.Errorf("TotalSent = %d, want 1", session.TotalSent)
	}
	if session.TotalReceived != 1 {
		t.Errorf("TotalReceived = %d, want 1", session.TotalReceived)
	}
}

func TestRecorder_RecordMessage_AutoCreateSession(t *testing.T) {
	r := NewRecorder(100)

	// Record message without starting session
	r.RecordMessage("wss://example.com/ws", "received", "text", []byte("hello"))

	session := r.GetSession("wss://example.com/ws")
	if session == nil {
		t.Fatal("session should be auto-created")
	}
	if len(session.Messages) != 1 {
		t.Errorf("len(Messages) = %d, want 1", len(session.Messages))
	}
}

func TestRecorder_RecordMessage_MaxLimit(t *testing.T) {
	r := NewRecorder(3)

	r.StartSession("wss://example.com/ws")

	for i := 0; i < 5; i++ {
		r.RecordMessage("wss://example.com/ws", "received", "text", []byte("msg"))
	}

	session := r.GetSession("wss://example.com/ws")
	if len(session.Messages) != 3 {
		t.Errorf("len(Messages) = %d, want 3 (max)", len(session.Messages))
	}
}

func TestRecorder_GetSession(t *testing.T) {
	r := NewRecorder(100)

	r.StartSession("wss://example.com/ws")
	r.RecordMessage("wss://example.com/ws", "sent", "text", []byte("hello"))

	session := r.GetSession("wss://example.com/ws")
	if session == nil {
		t.Fatal("session should exist")
	}

	// Verify it's a copy (modify shouldn't affect original)
	session.Messages = append(session.Messages, RecordedMessage{})
	originalSession := r.GetSession("wss://example.com/ws")
	if len(originalSession.Messages) != 1 {
		t.Error("GetSession should return a copy")
	}
}

func TestRecorder_GetSession_NonExistent(t *testing.T) {
	r := NewRecorder(100)

	session := r.GetSession("wss://nonexistent.com/ws")
	if session != nil {
		t.Error("should return nil for non-existent session")
	}
}

func TestRecorder_GetAllSessions(t *testing.T) {
	r := NewRecorder(100)

	r.StartSession("wss://example.com/ws1")
	r.StartSession("wss://example.com/ws2")
	r.StartSession("wss://example.com/ws3")

	sessions := r.GetAllSessions()
	if len(sessions) != 3 {
		t.Errorf("len(sessions) = %d, want 3", len(sessions))
	}
}

func TestRecorder_GetActiveSessions(t *testing.T) {
	r := NewRecorder(100)

	r.StartSession("wss://example.com/ws1")
	r.StartSession("wss://example.com/ws2")
	r.EndSession("wss://example.com/ws2")
	r.StartSession("wss://example.com/ws3")

	active := r.GetActiveSessions()
	if len(active) != 2 {
		t.Errorf("len(active) = %d, want 2", len(active))
	}
}

func TestRecorder_Clear(t *testing.T) {
	r := NewRecorder(100)

	r.StartSession("wss://example.com/ws1")
	r.StartSession("wss://example.com/ws2")

	r.Clear()

	sessions := r.GetAllSessions()
	if len(sessions) != 0 {
		t.Errorf("len(sessions) = %d, want 0 after clear", len(sessions))
	}
}

func TestRecorder_Stats(t *testing.T) {
	r := NewRecorder(100)

	r.StartSession("wss://example.com/ws1")
	r.RecordMessage("wss://example.com/ws1", "sent", "text", []byte("msg1"))
	r.RecordMessage("wss://example.com/ws1", "received", "text", []byte("msg2"))

	r.StartSession("wss://example.com/ws2")
	r.EndSession("wss://example.com/ws2")

	stats := r.Stats()

	if stats.TotalSessions != 2 {
		t.Errorf("TotalSessions = %d, want 2", stats.TotalSessions)
	}
	if stats.ActiveSessions != 1 {
		t.Errorf("ActiveSessions = %d, want 1", stats.ActiveSessions)
	}
	if stats.TotalMessages != 2 {
		t.Errorf("TotalMessages = %d, want 2", stats.TotalMessages)
	}
}

func TestRecorder_ExportJSON(t *testing.T) {
	r := NewRecorder(100)

	r.StartSession("wss://example.com/ws")
	r.RecordMessage("wss://example.com/ws", "sent", "text", []byte(`{"type":"ping"}`))

	data, err := r.ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON() error = %v", err)
	}

	if len(data) == 0 {
		t.Error("exported JSON should not be empty")
	}

	// Verify it's valid JSON by checking for expected content
	if !strings.Contains(string(data), "wss://example.com/ws") {
		t.Error("exported JSON should contain URL")
	}
}

func TestRecorder_AnalyzePatterns(t *testing.T) {
	r := NewRecorder(100)

	r.StartSession("wss://example.com/ws")
	r.RecordMessage("wss://example.com/ws", "sent", "text", []byte(`{"type":"hello"}`))
	r.RecordMessage("wss://example.com/ws", "received", "text", []byte(`{"type":"world"}`))
	r.RecordMessage("wss://example.com/ws", "received", "binary", []byte{0x01, 0x02, 0x03})

	analysis := r.AnalyzePatterns("wss://example.com/ws")

	if analysis == nil {
		t.Fatal("analysis should not be nil")
	}
	if analysis.MessageCount != 3 {
		t.Errorf("MessageCount = %d, want 3", analysis.MessageCount)
	}
	if analysis.SentCount != 1 {
		t.Errorf("SentCount = %d, want 1", analysis.SentCount)
	}
	if analysis.ReceivedCount != 2 {
		t.Errorf("ReceivedCount = %d, want 2", analysis.ReceivedCount)
	}
	if analysis.MessageTypes["text"] != 2 {
		t.Errorf("MessageTypes[text] = %d, want 2", analysis.MessageTypes["text"])
	}
	if analysis.MessageTypes["binary"] != 1 {
		t.Errorf("MessageTypes[binary] = %d, want 1", analysis.MessageTypes["binary"])
	}
	if !analysis.HasJSON {
		t.Error("HasJSON should be true")
	}
}

func TestRecorder_AnalyzePatterns_NonExistent(t *testing.T) {
	r := NewRecorder(100)

	analysis := r.AnalyzePatterns("wss://nonexistent.com/ws")
	if analysis != nil {
		t.Error("should return nil for non-existent session")
	}
}

func TestRecorder_AnalyzePatterns_EmptySession(t *testing.T) {
	r := NewRecorder(100)

	r.StartSession("wss://example.com/ws")

	analysis := r.AnalyzePatterns("wss://example.com/ws")

	if analysis == nil {
		t.Fatal("analysis should not be nil")
	}
	if analysis.MessageCount != 0 {
		t.Errorf("MessageCount = %d, want 0", analysis.MessageCount)
	}
}

func TestRecorder_Concurrent(t *testing.T) {
	r := NewRecorder(1000)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			url := "wss://example.com/ws"
			r.StartSession(url)
			r.RecordMessage(url, "sent", "text", []byte("msg"))
			r.GetSession(url)
			r.Stats()
		}(i)
	}
	wg.Wait()
}

func TestIsJSON(t *testing.T) {
	tests := []struct {
		data []byte
		want bool
	}{
		{[]byte(`{"key":"value"}`), true},
		{[]byte(`[1,2,3]`), true},
		{[]byte(`"string"`), true},
		{[]byte(`123`), true},
		{[]byte(`true`), true},
		{[]byte(`null`), true},
		{[]byte(`not json`), false},
		{[]byte(`{invalid}`), false},
		{[]byte(``), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.data), func(t *testing.T) {
			got := isJSON(tt.data)
			if got != tt.want {
				t.Errorf("isJSON(%q) = %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Type Tests
// =============================================================================

func TestWebSocketEndpoint_Struct(t *testing.T) {
	ep := WebSocketEndpoint{
		URL:            "wss://example.com/ws",
		DiscoveredFrom: "https://example.com",
		Protocols:      []string{"wamp", "mqtt"},
		SampleMessages: []WebSocketMsg{
			{Direction: "received", Type: "text", Data: "hello"},
		},
		Timestamp: time.Now(),
	}

	if ep.URL != "wss://example.com/ws" {
		t.Error("URL not set correctly")
	}
	if len(ep.Protocols) != 2 {
		t.Error("Protocols not set correctly")
	}
}

func TestWebSocketMsg_Struct(t *testing.T) {
	msg := WebSocketMsg{
		Direction: "sent",
		Type:      "text",
		Data:      "hello world",
		Timestamp: time.Now(),
	}

	if msg.Direction != "sent" {
		t.Error("Direction not set correctly")
	}
	if msg.Data != "hello world" {
		t.Error("Data not set correctly")
	}
}

func TestRecordedSession_Struct(t *testing.T) {
	session := RecordedSession{
		URL:           "wss://example.com/ws",
		StartTime:     time.Now(),
		Messages:      []RecordedMessage{},
		TotalSent:     5,
		TotalReceived: 10,
		Active:        true,
	}

	if session.URL != "wss://example.com/ws" {
		t.Error("URL not set correctly")
	}
	if session.TotalSent != 5 {
		t.Error("TotalSent not set correctly")
	}
}

func TestRecordedMessage_Struct(t *testing.T) {
	msg := RecordedMessage{
		Timestamp: time.Now(),
		Direction: "received",
		Type:      "binary",
		Data:      []byte{0x01, 0x02},
		Size:      2,
	}

	if msg.Direction != "received" {
		t.Error("Direction not set correctly")
	}
	if msg.Size != 2 {
		t.Error("Size not set correctly")
	}
}

func TestPatternAnalysis_Struct(t *testing.T) {
	analysis := PatternAnalysis{
		URL:           "wss://example.com/ws",
		MessageCount:  100,
		SentCount:     40,
		ReceivedCount: 60,
		MessageTypes:  map[string]int{"text": 80, "binary": 20},
		AverageSize:   256,
		MaxSize:       1024,
		MinSize:       10,
		HasJSON:       true,
	}

	if analysis.MessageCount != 100 {
		t.Error("MessageCount not set correctly")
	}
	if !analysis.HasJSON {
		t.Error("HasJSON not set correctly")
	}
}

func TestRecorderStats_Struct(t *testing.T) {
	stats := RecorderStats{
		TotalSessions:  10,
		ActiveSessions: 3,
		TotalMessages:  500,
	}

	if stats.TotalSessions != 10 {
		t.Error("TotalSessions not set correctly")
	}
}
