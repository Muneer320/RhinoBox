package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/config"
)

type observedBody struct {
	io.Reader
	reads int
}

func (b *observedBody) Read(p []byte) (int, error) { b.reads++; return b.Reader.Read(p) }
func (b *observedBody) Close() error               { return nil }

func TestRequestLimitDoesNotPreReadBody(t *testing.T) {
	body := &observedBody{Reader: strings.NewReader("upload")}
	req := httptest.NewRequest(http.MethodPost, "/upload", nil)
	req.Body, req.ContentLength = body, -1
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if body.reads != 0 {
			t.Fatal("middleware buffered the upload before the handler")
		}
		data, err := io.ReadAll(r.Body)
		if err != nil || string(data) != "upload" {
			t.Fatalf("body changed: %q, %v", data, err)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	NewRequestSizeLimitMiddleware(config.SecurityConfig{MaxRequestSize: 16}, nil).Handler(handler).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status %d", recorder.Code)
	}
}

func TestChunkedUploadLimitReturns413(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(strings.Repeat("x", 1025)))
	req.ContentLength = -1
	recorder := httptest.NewRecorder()
	NewRequestSizeLimitMiddleware(config.SecurityConfig{MaxRequestSize: 1024}, nil).Handler(handler).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", recorder.Code)
	}
}
