package output

import (
	"encoding/json"
	"io"
	"sync"
)

// JSONWriter writes output in JSON format.
type JSONWriter struct {
	mu       sync.Mutex
	writer   io.Writer
	pretty   bool
	stream   bool
	encoder  *json.Encoder
	first    bool
	closed   bool
}

// NewJSONWriter creates a new JSON writer.
func NewJSONWriter(w io.Writer, pretty, stream bool) *JSONWriter {
	jw := &JSONWriter{
		writer: w,
		pretty: pretty,
		stream: stream,
		first:  true,
	}

	jw.encoder = json.NewEncoder(w)
	if pretty {
		jw.encoder.SetIndent("", "  ")
	}

	return jw
}

// WriteResult writes the complete crawl result.
func (j *JSONWriter) WriteResult(result *CrawlResult) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.closed {
		return nil
	}

	var data []byte
	var err error

	if j.pretty {
		data, err = json.MarshalIndent(result, "", "  ")
	} else {
		data, err = json.Marshal(result)
	}

	if err != nil {
		return err
	}

	_, err = j.writer.Write(data)
	if err != nil {
		return err
	}

	// Add newline
	_, err = j.writer.Write([]byte("\n"))
	return err
}

// WriteEndpoint writes a single endpoint in streaming mode.
func (j *JSONWriter) WriteEndpoint(endpoint *Endpoint) error {
	if !j.stream {
		return nil
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	if j.closed {
		return nil
	}

	wrapper := StreamEvent{
		Type: "endpoint",
		Data: endpoint,
	}

	return j.writeStreamEvent(wrapper)
}

// WriteForm writes a single form in streaming mode.
func (j *JSONWriter) WriteForm(form *Form) error {
	if !j.stream {
		return nil
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	if j.closed {
		return nil
	}

	wrapper := StreamEvent{
		Type: "form",
		Data: form,
	}

	return j.writeStreamEvent(wrapper)
}

// WriteWebSocket writes a single WebSocket endpoint in streaming mode.
func (j *JSONWriter) WriteWebSocket(ws *WebSocketEndpoint) error {
	if !j.stream {
		return nil
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	if j.closed {
		return nil
	}

	wrapper := StreamEvent{
		Type: "websocket",
		Data: ws,
	}

	return j.writeStreamEvent(wrapper)
}

// WriteError writes an error in streaming mode.
func (j *JSONWriter) WriteError(err *CrawlError) error {
	if !j.stream {
		return nil
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	if j.closed {
		return nil
	}

	wrapper := StreamEvent{
		Type: "error",
		Data: err,
	}

	return j.writeStreamEvent(wrapper)
}

// writeStreamEvent writes a stream event.
func (j *JSONWriter) writeStreamEvent(event StreamEvent) error {
	var data []byte
	var err error

	if j.pretty {
		data, err = json.MarshalIndent(event, "", "  ")
	} else {
		data, err = json.Marshal(event)
	}

	if err != nil {
		return err
	}

	_, err = j.writer.Write(data)
	if err != nil {
		return err
	}

	_, err = j.writer.Write([]byte("\n"))
	return err
}

// Flush flushes the writer.
func (j *JSONWriter) Flush() error {
	j.mu.Lock()
	defer j.mu.Unlock()

	if flusher, ok := j.writer.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}

// Close closes the writer.
func (j *JSONWriter) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.closed = true

	if closer, ok := j.writer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// StreamEvent represents a streaming output event.
type StreamEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// ProgressWriter wraps a writer and provides progress updates.
type ProgressWriter struct {
	Writer
	onProgress func(stats ProgressStats)
}

// ProgressStats contains progress statistics.
type ProgressStats struct {
	URLsDiscovered int
	PagesCrawled   int
	FormsFound     int
	APIEndpoints   int
	WebSockets     int
	Errors         int
}

// NewProgressWriter creates a writer that reports progress.
func NewProgressWriter(w Writer, onProgress func(ProgressStats)) *ProgressWriter {
	return &ProgressWriter{
		Writer:     w,
		onProgress: onProgress,
	}
}

// WriteEndpoint writes an endpoint and updates progress.
func (p *ProgressWriter) WriteEndpoint(endpoint *Endpoint) error {
	if p.onProgress != nil {
		p.onProgress(ProgressStats{APIEndpoints: 1})
	}
	return p.Writer.WriteEndpoint(endpoint)
}

// WriteForm writes a form and updates progress.
func (p *ProgressWriter) WriteForm(form *Form) error {
	if p.onProgress != nil {
		p.onProgress(ProgressStats{FormsFound: 1})
	}
	return p.Writer.WriteForm(form)
}

// WriteWebSocket writes a WebSocket and updates progress.
func (p *ProgressWriter) WriteWebSocket(ws *WebSocketEndpoint) error {
	if p.onProgress != nil {
		p.onProgress(ProgressStats{WebSockets: 1})
	}
	return p.Writer.WriteWebSocket(ws)
}

// WriteError writes an error and updates progress.
func (p *ProgressWriter) WriteError(err *CrawlError) error {
	if p.onProgress != nil {
		p.onProgress(ProgressStats{Errors: 1})
	}
	return p.Writer.WriteError(err)
}
