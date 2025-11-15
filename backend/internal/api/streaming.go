package api

import (
	"encoding/json"
	"net/http"
)

// StreamingWriter wraps http.ResponseWriter for efficient streaming responses
type StreamingWriter struct {
	writer  http.ResponseWriter
	encoder *json.Encoder
	flusher http.Flusher
}

// NewStreamingWriter creates a new streaming response writer
func NewStreamingWriter(w http.ResponseWriter) *StreamingWriter {
	flusher, ok := w.(http.Flusher)
	if !ok {
		// If flushing not supported, use regular writer
		flusher = nil
	}

	return &StreamingWriter{
		writer:  w,
		encoder: json.NewEncoder(w),
		flusher: flusher,
	}
}

// WriteJSON writes a JSON object and flushes immediately
func (sw *StreamingWriter) WriteJSON(v any) error {
	if err := sw.encoder.Encode(v); err != nil {
		return err
	}
	if sw.flusher != nil {
		sw.flusher.Flush()
	}
	return nil
}

// SetHeaders configures headers for streaming response
func (sw *StreamingWriter) SetHeaders(contentType string) {
	sw.writer.Header().Set("Content-Type", contentType)
	sw.writer.Header().Set("Transfer-Encoding", "chunked")
	sw.writer.Header().Set("X-Content-Type-Options", "nosniff")
	// Disable buffering for streaming
	sw.writer.Header().Set("X-Accel-Buffering", "no")
}

// SetHeadersNDJSON sets headers for newline-delimited JSON streaming
func (sw *StreamingWriter) SetHeadersNDJSON() {
	sw.SetHeaders("application/x-ndjson")
}

// SetHeadersJSON sets headers for regular JSON response
func (sw *StreamingWriter) SetHeadersJSON() {
	sw.SetHeaders("application/json")
}
