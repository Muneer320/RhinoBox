package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResponseWriter(t *testing.T) {
	w := httptest.NewRecorder()
	rw := NewResponseWriter(w)

	// Test initial state
	if rw.StatusCode() != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rw.StatusCode())
	}

	// Test WriteHeader
	rw.WriteHeader(http.StatusNotFound)
	if rw.StatusCode() != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rw.StatusCode())
	}

	// Test Write
	body := []byte("test body")
	n, err := rw.Write(body)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(body) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(body), n)
	}

	// Test Body
	if string(rw.Body()) != string(body) {
		t.Errorf("Expected body %q, got %q", string(body), string(rw.Body()))
	}

	// Test Duration
	duration := rw.Duration()
	if duration <= 0 {
		t.Errorf("Expected positive duration, got %v", duration)
	}
}

func TestResponseMiddleware_CommonHeaders(t *testing.T) {
	logger := slog.Default()
	config := DefaultResponseConfig(logger)
	config.EnableCORS = true
	config.CORSOrigins = []string{"*"}

	mw := NewResponseMiddleware(config)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	mw.Handler(handler).ServeHTTP(w, req)

	// Check Content-Type
	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Expected Content-Type to contain 'application/json', got %q", contentType)
	}

	// Check CORS headers
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("Expected CORS origin '*', got %q", w.Header().Get("Access-Control-Allow-Origin"))
	}

	// Check security headers
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("Expected X-Content-Type-Options 'nosniff', got %q", w.Header().Get("X-Content-Type-Options"))
	}

	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("Expected X-Frame-Options 'DENY', got %q", w.Header().Get("X-Frame-Options"))
	}
}

func TestResponseMiddleware_CORS_Preflight(t *testing.T) {
	logger := slog.Default()
	config := DefaultResponseConfig(logger)
	config.EnableCORS = true
	config.CORSOrigins = []string{"http://localhost:3000"}

	mw := NewResponseMiddleware(config)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()

	mw.Handler(handler).ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status %d for OPTIONS, got %d", http.StatusNoContent, w.Code)
	}
}

func TestResponseFormatter_Success(t *testing.T) {
	formatter := NewResponseFormatter()
	w := httptest.NewRecorder()

	data := map[string]string{"message": "test"}
	err := formatter.Success(w, data, "req-123")
	if err != nil {
		t.Fatalf("Success() failed: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response StandardResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success=true")
	}

	if response.Data == nil {
		t.Error("Expected data to be set")
	}

	if response.RequestID != "req-123" {
		t.Errorf("Expected request ID 'req-123', got %q", response.RequestID)
	}

	if response.Timestamp == "" {
		t.Error("Expected timestamp to be set")
	}
}

func TestResponseFormatter_Error(t *testing.T) {
	formatter := NewResponseFormatter()
	w := httptest.NewRecorder()

	err := formatter.Error(w, http.StatusBadRequest, ErrorCodeBadRequest, "Invalid input", nil, "req-456")
	if err != nil {
		t.Fatalf("Error() failed: %v", err)
	}

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var response StandardResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Success {
		t.Error("Expected success=false")
	}

	if response.Error == nil {
		t.Fatal("Expected error to be set")
	}

	if response.Error.Code != ErrorCodeBadRequest {
		t.Errorf("Expected error code %q, got %q", ErrorCodeBadRequest, response.Error.Code)
	}

	if response.Error.Message != "Invalid input" {
		t.Errorf("Expected error message 'Invalid input', got %q", response.Error.Message)
	}
}

func TestResponseFormatter_Paginated(t *testing.T) {
	formatter := NewResponseFormatter()
	w := httptest.NewRecorder()

	data := []string{"item1", "item2", "item3"}
	err := formatter.Paginated(w, data, 1, 10, 25, "req-789")
	if err != nil {
		t.Fatalf("Paginated() failed: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response PaginatedResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success=true")
	}

	if response.Pagination.Page != 1 {
		t.Errorf("Expected page 1, got %d", response.Pagination.Page)
	}

	if response.Pagination.PageSize != 10 {
		t.Errorf("Expected page size 10, got %d", response.Pagination.PageSize)
	}

	if response.Pagination.Total != 25 {
		t.Errorf("Expected total 25, got %d", response.Pagination.Total)
	}

	if response.Pagination.TotalPages != 3 {
		t.Errorf("Expected total pages 3, got %d", response.Pagination.TotalPages)
	}

	if !response.Pagination.HasNext {
		t.Error("Expected HasNext=true")
	}

	if response.Pagination.HasPrev {
		t.Error("Expected HasPrev=false")
	}
}

func TestMapHTTPStatusToErrorCode(t *testing.T) {
	tests := []struct {
		statusCode int
		expected   string
	}{
		{http.StatusBadRequest, ErrorCodeBadRequest},
		{http.StatusUnauthorized, ErrorCodeUnauthorized},
		{http.StatusForbidden, ErrorCodeForbidden},
		{http.StatusNotFound, ErrorCodeNotFound},
		{http.StatusConflict, ErrorCodeConflict},
		{http.StatusInternalServerError, ErrorCodeInternalError},
		{http.StatusServiceUnavailable, ErrorCodeServiceUnavailable},
		{http.StatusRequestTimeout, ErrorCodeTimeout},
		{999, ErrorCodeInternalError}, // Unknown status
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.statusCode), func(t *testing.T) {
			result := MapHTTPStatusToErrorCode(tt.statusCode)
			if result != tt.expected {
				t.Errorf("Expected %q for status %d, got %q", tt.expected, tt.statusCode, result)
			}
		})
	}
}

func TestResponseMiddleware_Logging(t *testing.T) {
	logger := slog.Default()
	config := DefaultResponseConfig(logger)
	config.EnableLogging = true

	mw := NewResponseMiddleware(config)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	req := httptest.NewRequest("GET", "/test?param=value", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()

	mw.Handler(handler).ServeHTTP(w, req)

	// Logging should not cause errors
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestResponseMiddleware_LoggingDisabled(t *testing.T) {
	logger := slog.Default()
	config := DefaultResponseConfig(logger)
	config.EnableLogging = false

	mw := NewResponseMiddleware(config)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	mw.Handler(handler).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestGetAllowedOrigin(t *testing.T) {
	tests := []struct {
		name           string
		origins        []string
		requestOrigin  string
		expectedOrigin string
	}{
		{
			name:           "Wildcard allows all",
			origins:        []string{"*"},
			requestOrigin:  "http://example.com",
			expectedOrigin: "*",
		},
		{
			name:           "Exact match",
			origins:        []string{"http://localhost:3000"},
			requestOrigin:  "http://localhost:3000",
			expectedOrigin: "http://localhost:3000",
		},
		{
			name:           "No match returns first",
			origins:        []string{"http://localhost:3000", "http://localhost:3001"},
			requestOrigin:  "http://example.com",
			expectedOrigin: "http://localhost:3000",
		},
		{
			name:           "Empty origins",
			origins:        []string{},
			requestOrigin:  "http://example.com",
			expectedOrigin: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.Default()
			config := DefaultResponseConfig(logger)
			config.EnableCORS = true
			config.CORSOrigins = tt.origins

			mw := NewResponseMiddleware(config)
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			if tt.requestOrigin != "" {
				req.Header.Set("Origin", tt.requestOrigin)
			}
			w := httptest.NewRecorder()

			mw.Handler(handler).ServeHTTP(w, req)

			actualOrigin := w.Header().Get("Access-Control-Allow-Origin")
			if tt.expectedOrigin == "" {
				// If no origin expected, header should be empty or not set
				if actualOrigin != "" && actualOrigin != tt.expectedOrigin {
					t.Errorf("Expected no origin header or empty, got %q", actualOrigin)
				}
			} else if actualOrigin != tt.expectedOrigin {
				t.Errorf("Expected origin %q, got %q", tt.expectedOrigin, actualOrigin)
			}
		})
	}
}

func TestPaginatedResponse_EdgeCases(t *testing.T) {
	formatter := NewResponseFormatter()
	w := httptest.NewRecorder()

	// Test with zero total
	err := formatter.Paginated(w, []string{}, 1, 10, 0, "req-1")
	if err != nil {
		t.Fatalf("Paginated() failed: %v", err)
	}

	var response PaginatedResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Pagination.TotalPages != 1 {
		t.Errorf("Expected total pages 1 for zero total, got %d", response.Pagination.TotalPages)
	}

	// Test last page
	w2 := httptest.NewRecorder()
	err = formatter.Paginated(w2, []string{"item"}, 3, 10, 25, "req-2")
	if err != nil {
		t.Fatalf("Paginated() failed: %v", err)
	}

	var response2 PaginatedResponse
	if err := json.NewDecoder(w2.Body).Decode(&response2); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response2.Pagination.HasNext {
		t.Error("Expected HasNext=false on last page")
	}

	if !response2.Pagination.HasPrev {
		t.Error("Expected HasPrev=true on last page")
	}
}

func BenchmarkResponseFormatter_Success(b *testing.B) {
	formatter := NewResponseFormatter()
	data := map[string]string{"message": "test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		_ = formatter.Success(w, data, "req-123")
	}
}

func BenchmarkResponseFormatter_Error(b *testing.B) {
	formatter := NewResponseFormatter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		_ = formatter.Error(w, http.StatusBadRequest, ErrorCodeBadRequest, "Error message", nil, "req-123")
	}
}

func BenchmarkResponseFormatter_Paginated(b *testing.B) {
	formatter := NewResponseFormatter()
	data := make([]string, 100)
	for i := range data {
		data[i] = "item"
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		_ = formatter.Paginated(w, data, 1, 10, 100, "req-123")
	}
}

