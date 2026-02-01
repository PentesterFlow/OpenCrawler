package websocket

import (
	"encoding/json"
	"sync"
	"time"
)

// Recorder records WebSocket messages for analysis.
type Recorder struct {
	mu       sync.RWMutex
	sessions map[string]*RecordedSession
	maxMsgs  int
}

// RecordedSession contains recorded messages for a WebSocket session.
type RecordedSession struct {
	URL            string            `json:"url"`
	StartTime      time.Time         `json:"start_time"`
	EndTime        time.Time         `json:"end_time,omitempty"`
	Messages       []RecordedMessage `json:"messages"`
	TotalSent      int               `json:"total_sent"`
	TotalReceived  int               `json:"total_received"`
	Active         bool              `json:"active"`
}

// RecordedMessage represents a recorded WebSocket message.
type RecordedMessage struct {
	Timestamp time.Time `json:"timestamp"`
	Direction string    `json:"direction"` // "sent" or "received"
	Type      string    `json:"type"`      // "text", "binary", "ping", "pong"
	Data      []byte    `json:"data"`
	Size      int       `json:"size"`
}

// NewRecorder creates a new WebSocket recorder.
func NewRecorder(maxMessages int) *Recorder {
	if maxMessages <= 0 {
		maxMessages = 1000
	}
	return &Recorder{
		sessions: make(map[string]*RecordedSession),
		maxMsgs:  maxMessages,
	}
}

// StartSession starts recording a new WebSocket session.
func (r *Recorder) StartSession(url string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sessions[url] = &RecordedSession{
		URL:       url,
		StartTime: time.Now(),
		Messages:  make([]RecordedMessage, 0),
		Active:    true,
	}
}

// EndSession ends a WebSocket session.
func (r *Recorder) EndSession(url string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if session, exists := r.sessions[url]; exists {
		session.EndTime = time.Now()
		session.Active = false
	}
}

// RecordMessage records a WebSocket message.
func (r *Recorder) RecordMessage(url string, direction, msgType string, data []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, exists := r.sessions[url]
	if !exists {
		session = &RecordedSession{
			URL:       url,
			StartTime: time.Now(),
			Messages:  make([]RecordedMessage, 0),
			Active:    true,
		}
		r.sessions[url] = session
	}

	// Check message limit
	if len(session.Messages) >= r.maxMsgs {
		return
	}

	msg := RecordedMessage{
		Timestamp: time.Now(),
		Direction: direction,
		Type:      msgType,
		Data:      data,
		Size:      len(data),
	}

	session.Messages = append(session.Messages, msg)

	if direction == "sent" {
		session.TotalSent++
	} else {
		session.TotalReceived++
	}
}

// GetSession returns a recorded session.
func (r *Recorder) GetSession(url string) *RecordedSession {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if session, exists := r.sessions[url]; exists {
		// Return a copy
		copy := *session
		copy.Messages = make([]RecordedMessage, len(session.Messages))
		for i, msg := range session.Messages {
			copy.Messages[i] = msg
		}
		return &copy
	}
	return nil
}

// GetAllSessions returns all recorded sessions.
func (r *Recorder) GetAllSessions() []*RecordedSession {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sessions := make([]*RecordedSession, 0, len(r.sessions))
	for _, session := range r.sessions {
		copy := *session
		copy.Messages = make([]RecordedMessage, len(session.Messages))
		for i, msg := range session.Messages {
			copy.Messages[i] = msg
		}
		sessions = append(sessions, &copy)
	}
	return sessions
}

// GetActiveSessions returns all active sessions.
func (r *Recorder) GetActiveSessions() []*RecordedSession {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sessions := make([]*RecordedSession, 0)
	for _, session := range r.sessions {
		if session.Active {
			copy := *session
			sessions = append(sessions, &copy)
		}
	}
	return sessions
}

// Clear clears all recorded sessions.
func (r *Recorder) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions = make(map[string]*RecordedSession)
}

// Stats returns recorder statistics.
func (r *Recorder) Stats() RecorderStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := RecorderStats{
		TotalSessions:  len(r.sessions),
		ActiveSessions: 0,
		TotalMessages:  0,
	}

	for _, session := range r.sessions {
		stats.TotalMessages += len(session.Messages)
		if session.Active {
			stats.ActiveSessions++
		}
	}

	return stats
}

// RecorderStats contains recorder statistics.
type RecorderStats struct {
	TotalSessions  int `json:"total_sessions"`
	ActiveSessions int `json:"active_sessions"`
	TotalMessages  int `json:"total_messages"`
}

// ExportJSON exports all sessions as JSON.
func (r *Recorder) ExportJSON() ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return json.MarshalIndent(r.sessions, "", "  ")
}

// AnalyzePatterns analyzes message patterns in sessions.
func (r *Recorder) AnalyzePatterns(url string) *PatternAnalysis {
	r.mu.RLock()
	defer r.mu.RUnlock()

	session, exists := r.sessions[url]
	if !exists {
		return nil
	}

	analysis := &PatternAnalysis{
		URL:             url,
		MessageCount:    len(session.Messages),
		SentCount:       session.TotalSent,
		ReceivedCount:   session.TotalReceived,
		MessageTypes:    make(map[string]int),
		AverageSize:     0,
		MaxSize:         0,
		MinSize:         0,
	}

	if len(session.Messages) == 0 {
		return analysis
	}

	totalSize := 0
	analysis.MinSize = session.Messages[0].Size

	for _, msg := range session.Messages {
		analysis.MessageTypes[msg.Type]++
		totalSize += msg.Size

		if msg.Size > analysis.MaxSize {
			analysis.MaxSize = msg.Size
		}
		if msg.Size < analysis.MinSize {
			analysis.MinSize = msg.Size
		}
	}

	analysis.AverageSize = totalSize / len(session.Messages)

	// Detect JSON patterns
	for _, msg := range session.Messages {
		if isJSON(msg.Data) {
			analysis.HasJSON = true
			break
		}
	}

	return analysis
}

// PatternAnalysis contains analysis of message patterns.
type PatternAnalysis struct {
	URL           string         `json:"url"`
	MessageCount  int            `json:"message_count"`
	SentCount     int            `json:"sent_count"`
	ReceivedCount int            `json:"received_count"`
	MessageTypes  map[string]int `json:"message_types"`
	AverageSize   int            `json:"average_size"`
	MaxSize       int            `json:"max_size"`
	MinSize       int            `json:"min_size"`
	HasJSON       bool           `json:"has_json"`
}

func isJSON(data []byte) bool {
	var js json.RawMessage
	return json.Unmarshal(data, &js) == nil
}
