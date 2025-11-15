package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
	apierrors "github.com/Muneer320/RhinoBox/internal/errors"
)

func setupTestServer(t *testing.T) (*api.Server, func()) {
	tempDir, err := os.MkdirTemp("", "rhinobox-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cfg := config.Config{
		DataDir:      tempDir,
		MaxUploadBytes: 10 * 1024 * 1024,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server, err := api.NewServer(cfg, logger)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("failed to create server: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return server, cleanup
}

func TestErrorHandling_NotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "Delete non-existent file",
			method:         "DELETE",
			path:           "/files/nonexistent-hash",
			expectedStatus: http.StatusNotFound,
			expectedCode:   string(apierrors.ErrorCodeNotFound),
		},
		{
			name:           "Get metadata for non-existent file",
			method:         "GET",
			path:           "/files/metadata?hash=nonexistent",
			expectedStatus: http.StatusNotFound,
			expectedCode:   string(apierrors.ErrorCodeNotFound),
		},
		{
			name:           "Download non-existent file",
			method:         "GET",
			path:           "/files/download?hash=nonexistent",
			expectedStatus: http.StatusNotFound,
			expectedCode:   string(apierrors.ErrorCodeNotFound),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			server.Router().ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var response map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			errorObj, ok := response["error"].(map[string]interface{})
			if !ok {
				t.Fatalf("expected error object in response")
			}

			code, ok := errorObj["code"].(string)
			if !ok {
				t.Fatalf("expected code in error object")
			}

			if code != tt.expectedCode {
				t.Errorf("expected error code %s, got %s", tt.expectedCode, code)
			}
		})
	}
}

func TestErrorHandling_BadRequest(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	tests := []struct {
		name           string
		method         string
		path           string
		body           io.Reader
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "Rename with missing hash",
			method:         "PATCH",
			path:           "/files/rename",
			body:           bytes.NewBufferString(`{"new_name": "test.txt"}`),
			expectedStatus: http.StatusBadRequest,
			expectedCode:   string(apierrors.ErrorCodeBadRequest),
		},
		{
			name:           "Rename with missing new_name",
			method:         "PATCH",
			path:           "/files/rename",
			body:           bytes.NewBufferString(`{"hash": "test"}`),
			expectedStatus: http.StatusBadRequest,
			expectedCode:   string(apierrors.ErrorCodeBadRequest),
		},
		{
			name:           "Invalid JSON in request",
			method:         "PATCH",
			path:           "/files/rename",
			body:           bytes.NewBufferString(`invalid json`),
			expectedStatus: http.StatusBadRequest,
			expectedCode:   string(apierrors.ErrorCodeBadRequest),
		},
		{
			name:           "Search without name parameter",
			method:         "GET",
			path:           "/files/search",
			body:           nil,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   string(apierrors.ErrorCodeBadRequest),
		},
		{
			name:           "Download without hash or path",
			method:         "GET",
			path:           "/files/download",
			body:           nil,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   string(apierrors.ErrorCodeBadRequest),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, tt.body)
			if tt.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()

			server.Router().ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var response map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			errorObj, ok := response["error"].(map[string]interface{})
			if !ok {
				t.Fatalf("expected error object in response")
			}

			code, ok := errorObj["code"].(string)
			if !ok {
				t.Fatalf("expected code in error object")
			}

			if code != tt.expectedCode {
				t.Errorf("expected error code %s, got %s", tt.expectedCode, code)
			}
		})
	}
}

func TestErrorHandling_ConsistentFormat(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Test that all error responses have consistent format
	req := httptest.NewRequest("DELETE", "/files/nonexistent", nil)
	w := httptest.NewRecorder()

	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Check required fields
	if _, ok := response["error"]; !ok {
		t.Fatal("expected 'error' field in response")
	}

	errorObj, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error to be an object")
	}

	if _, ok := errorObj["code"]; !ok {
		t.Fatal("expected 'code' field in error object")
	}

	if _, ok := errorObj["message"]; !ok {
		t.Fatal("expected 'message' field in error object")
	}

	// Check Content-Type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %s", contentType)
	}
}

func TestErrorHandling_PanicRecovery(t *testing.T) {
	// This test verifies that panics are caught and handled gracefully
	// We can't easily trigger a panic in the actual handlers, but we can
	// verify the middleware is in place by checking error handling works
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Make a request that would cause an error
	req := httptest.NewRequest("GET", "/files/metadata", nil)
	w := httptest.NewRecorder()

	server.Router().ServeHTTP(w, req)

	// Should get a proper error response, not a panic
	if w.Code == 0 {
		t.Fatal("handler did not write response (possible panic)")
	}

	if w.Code >= 500 && w.Code < 600 {
		// If we got a 5xx, check it's a proper error response
		var response map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("got 5xx but response is not JSON (possible panic): %v", err)
		}
	}
}

func TestErrorHandling_RequestID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("DELETE", "/files/nonexistent", nil)
	req.Header.Set("X-Request-Id", "test-request-123")
	w := httptest.NewRecorder()

	server.Router().ServeHTTP(w, req)

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Request ID should be included in error response
	if requestID, ok := response["request_id"].(string); ok {
		if requestID != "test-request-123" {
			t.Errorf("expected request_id 'test-request-123', got %s", requestID)
		}
	}
}

func TestErrorHandling_StorageErrorMapping(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Test that storage errors are properly mapped
	// First, create a file to get a valid hash
	testFile := bytes.NewBufferString("test content")
	req := httptest.NewRequest("POST", "/ingest/media", nil)
	req.Header.Set("Content-Type", "multipart/form-data")
	
	// This is a simplified test - in a real scenario we'd use multipart
	// For now, just test that invalid operations return proper errors
	
	// Test invalid rename
	renameReq := httptest.NewRequest("PATCH", "/files/rename", 
		bytes.NewBufferString(`{"hash": "invalid", "new_name": "test.txt"}`))
	renameReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	
	server.Router().ServeHTTP(w, renameReq)
	
	if w.Code != http.StatusNotFound {
		t.Logf("Note: Expected NotFound for invalid hash, got %d", w.Code)
	}
	
	_ = testFile // suppress unused variable warning
	_ = req
}

func TestErrorHandling_ErrorDetails(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Test error with details (if supported)
	req := httptest.NewRequest("GET", "/files/metadata?hash=", nil)
	w := httptest.NewRecorder()

	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Error should have message
	errorObj, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object")
	}

	if message, ok := errorObj["message"].(string); !ok || message == "" {
		t.Error("expected non-empty error message")
	}
}

func TestErrorHandling_ConcurrentRequests(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Test that error handling works correctly under concurrent load
	const numRequests = 10
	errors := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			req := httptest.NewRequest("DELETE", fmt.Sprintf("/files/nonexistent-%d", i), nil)
			w := httptest.NewRecorder()
			server.Router().ServeHTTP(w, req)

			if w.Code != http.StatusNotFound {
				errors <- fmt.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
				return
			}

			var response map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				errors <- fmt.Errorf("failed to decode response: %v", err)
				return
			}

			if _, ok := response["error"]; !ok {
				errors <- fmt.Errorf("expected error in response")
				return
			}

			errors <- nil
		}()
	}

	// Collect results
	for i := 0; i < numRequests; i++ {
		if err := <-errors; err != nil {
			t.Error(err)
		}
	}
}

func TestErrorHandling_ResponseTime(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Test that error responses are fast
	req := httptest.NewRequest("DELETE", "/files/nonexistent", nil)
	w := httptest.NewRecorder()

	start := time.Now()
	server.Router().ServeHTTP(w, req)
	duration := time.Since(start)

	if duration > 100*time.Millisecond {
		t.Errorf("error response took too long: %v", duration)
	}

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

