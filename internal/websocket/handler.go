// Package websocket provides WebSocket handling for the crawler.
package websocket

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Handler manages WebSocket connections and message recording.
type Handler struct {
	mu         sync.RWMutex
	endpoints  map[string]*EndpointInfo
	dialer     *websocket.Dialer
	headers    http.Header
	maxMsgs    int
	msgTimeout time.Duration
	enabled    bool
}

// EndpointInfo contains information about a WebSocket endpoint.
type EndpointInfo struct {
	URL            string
	DiscoveredFrom string
	Protocols      []string
	Messages       []Message
	Connected      bool
	ConnectTime    time.Time
	Error          error
}

// Message represents a WebSocket message.
type Message struct {
	Direction string    // "sent" or "received"
	Type      int       // websocket.TextMessage, websocket.BinaryMessage, etc.
	Data      []byte
	Timestamp time.Time
}

// NewHandler creates a new WebSocket handler.
func NewHandler() *Handler {
	return &Handler{
		endpoints: make(map[string]*EndpointInfo),
		dialer: &websocket.Dialer{
			HandshakeTimeout: 10 * time.Second,
		},
		headers:    make(http.Header),
		maxMsgs:    100,
		msgTimeout: 5 * time.Second,
		enabled:    true,
	}
}

// SetHeaders sets custom headers for WebSocket connections.
func (h *Handler) SetHeaders(headers map[string]string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for k, v := range headers {
		h.headers.Set(k, v)
	}
}

// SetCookies sets cookies for WebSocket connections.
func (h *Handler) SetCookies(cookies []*http.Cookie) {
	h.mu.Lock()
	defer h.mu.Unlock()

	cookieStrs := make([]string, 0, len(cookies))
	for _, c := range cookies {
		cookieStrs = append(cookieStrs, c.Name+"="+c.Value)
	}

	if len(cookieStrs) > 0 {
		h.headers.Set("Cookie", strings.Join(cookieStrs, "; "))
	}
}

// Connect attempts to connect to a WebSocket endpoint.
func (h *Handler) Connect(ctx context.Context, wsURL, sourceURL string) error {
	if !h.enabled {
		return nil
	}

	h.mu.Lock()
	info := &EndpointInfo{
		URL:            wsURL,
		DiscoveredFrom: sourceURL,
		Messages:       make([]Message, 0),
		ConnectTime:    time.Now(),
	}
	h.endpoints[wsURL] = info
	headers := h.headers.Clone()
	h.mu.Unlock()

	// Parse URL to determine protocol
	parsed, err := url.Parse(wsURL)
	if err != nil {
		info.Error = err
		return err
	}

	// Ensure ws/wss scheme
	switch parsed.Scheme {
	case "ws", "wss":
		// OK
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	default:
		parsed.Scheme = "wss"
	}

	// Connect
	conn, resp, err := h.dialer.DialContext(ctx, parsed.String(), headers)
	if err != nil {
		info.Error = err
		return err
	}
	defer conn.Close()

	if resp != nil {
		// Extract protocols from response
		if protocols := resp.Header.Get("Sec-WebSocket-Protocol"); protocols != "" {
			info.Protocols = splitStrings(protocols, ",")
		}
	}

	info.Connected = true

	// Record messages for a short time
	return h.recordMessages(ctx, conn, info)
}

func splitStrings(s, sep string) []string {
	result := make([]string, 0)
	for _, part := range strings.Split(s, sep) {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// recordMessages records messages from a WebSocket connection.
func (h *Handler) recordMessages(ctx context.Context, conn *websocket.Conn, info *EndpointInfo) error {
	// Set read deadline
	conn.SetReadDeadline(time.Now().Add(h.msgTimeout))

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, h.msgTimeout)
	defer cancel()

	done := make(chan struct{})

	go func() {
		defer close(done)

		for {
			select {
			case <-timeoutCtx.Done():
				return
			default:
			}

			msgType, data, err := conn.ReadMessage()
			if err != nil {
				return
			}

			h.mu.Lock()
			if len(info.Messages) < h.maxMsgs {
				info.Messages = append(info.Messages, Message{
					Direction: "received",
					Type:      msgType,
					Data:      data,
					Timestamp: time.Now(),
				})
			}
			h.mu.Unlock()

			// Reset deadline for next message
			conn.SetReadDeadline(time.Now().Add(h.msgTimeout))
		}
	}()

	<-done
	return nil
}

// SendMessage sends a message and records the response.
func (h *Handler) SendMessage(ctx context.Context, wsURL string, msgType int, data []byte) error {
	h.mu.RLock()
	info, exists := h.endpoints[wsURL]
	headers := h.headers.Clone()
	h.mu.RUnlock()

	if !exists {
		return nil
	}

	// Connect if not connected
	conn, _, err := h.dialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Send message
	if err := conn.WriteMessage(msgType, data); err != nil {
		return err
	}

	h.mu.Lock()
	if len(info.Messages) < h.maxMsgs {
		info.Messages = append(info.Messages, Message{
			Direction: "sent",
			Type:      msgType,
			Data:      data,
			Timestamp: time.Now(),
		})
	}
	h.mu.Unlock()

	// Wait for response
	conn.SetReadDeadline(time.Now().Add(h.msgTimeout))
	respType, respData, err := conn.ReadMessage()
	if err == nil {
		h.mu.Lock()
		if len(info.Messages) < h.maxMsgs {
			info.Messages = append(info.Messages, Message{
				Direction: "received",
				Type:      respType,
				Data:      respData,
				Timestamp: time.Now(),
			})
		}
		h.mu.Unlock()
	}

	return nil
}

// GetEndpoints returns all discovered WebSocket endpoints.
func (h *Handler) GetEndpoints() []WebSocketEndpoint {
	h.mu.RLock()
	defer h.mu.RUnlock()

	endpoints := make([]WebSocketEndpoint, 0, len(h.endpoints))
	for _, info := range h.endpoints {
		ep := WebSocketEndpoint{
			URL:            info.URL,
			DiscoveredFrom: info.DiscoveredFrom,
			Protocols:      info.Protocols,
			Timestamp:      info.ConnectTime,
		}

		// Convert messages
		for _, msg := range info.Messages {
			typeName := "unknown"
			switch msg.Type {
			case websocket.TextMessage:
				typeName = "text"
			case websocket.BinaryMessage:
				typeName = "binary"
			}

			ep.SampleMessages = append(ep.SampleMessages, WebSocketMsg{
				Direction: msg.Direction,
				Type:      typeName,
				Data:      string(msg.Data),
				Timestamp: msg.Timestamp,
			})
		}

		endpoints = append(endpoints, ep)
	}

	return endpoints
}

// Count returns the number of discovered endpoints.
func (h *Handler) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.endpoints)
}

// HasEndpoint checks if an endpoint has been discovered.
func (h *Handler) HasEndpoint(wsURL string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, exists := h.endpoints[wsURL]
	return exists
}

// Enable enables WebSocket handling.
func (h *Handler) Enable() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.enabled = true
}

// Disable disables WebSocket handling.
func (h *Handler) Disable() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.enabled = false
}

// Clear clears all discovered endpoints.
func (h *Handler) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.endpoints = make(map[string]*EndpointInfo)
}

// SetMaxMessages sets the maximum number of messages to record per endpoint.
func (h *Handler) SetMaxMessages(max int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.maxMsgs = max
}

// SetMessageTimeout sets the timeout for receiving messages.
func (h *Handler) SetMessageTimeout(timeout time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.msgTimeout = timeout
}
