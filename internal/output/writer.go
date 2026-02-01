// Package output provides output formatting for the crawler.
package output

import (
	"io"
)

// Writer defines the interface for output writers.
type Writer interface {
	// WriteResult writes the complete crawl result
	WriteResult(result *CrawlResult) error

	// WriteEndpoint writes a single endpoint (for streaming)
	WriteEndpoint(endpoint *Endpoint) error

	// WriteForm writes a single form (for streaming)
	WriteForm(form *Form) error

	// WriteWebSocket writes a single WebSocket endpoint (for streaming)
	WriteWebSocket(ws *WebSocketEndpoint) error

	// WriteError writes an error (for streaming)
	WriteError(err *CrawlError) error

	// Flush flushes any buffered output
	Flush() error

	// Close closes the writer
	Close() error
}

// Config holds output configuration.
type Config struct {
	Format   string
	Pretty   bool
	Stream   bool
	FilePath string
}

// NewWriter creates a new output writer.
func NewWriter(w io.Writer, config Config) Writer {
	switch config.Format {
	case "json":
		return NewJSONWriter(w, config.Pretty, config.Stream)
	default:
		return NewJSONWriter(w, config.Pretty, config.Stream)
	}
}
