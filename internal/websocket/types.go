package websocket

import "time"

// WebSocketEndpoint represents a discovered WebSocket endpoint.
type WebSocketEndpoint struct {
	URL            string
	DiscoveredFrom string
	SampleMessages []WebSocketMsg
	Protocols      []string
	Timestamp      time.Time
}

// WebSocketMsg represents a WebSocket message.
type WebSocketMsg struct {
	Direction string
	Type      string
	Data      string
	Timestamp time.Time
}
