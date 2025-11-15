package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"log/slog"
)

// TestEndToEnd_ResponseFormat tests the complete response transformation flow
func TestEndToEnd_ResponseFormat(t *testing.T) {
	logger := slog.Default()
	config := DefaultResponseConfig(logger)
	config.EnableCORS = true
	config.CORSOrigins = []string{"*"}

	mw := NewResponseMiddleware(config)

	// Test success response
	t.Run("SuccessResponse", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			data := map[string]string{"message": "test"}
			requestID := "test-req-123"
			_ = WriteSuccess(w, data, requestID)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		mw.Handler(handler).ServeHTTP(w, req)

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

		if response.RequestID != "test-req-123" {
			t.Errorf("Expected request ID 'test-req-123', got %q", response.RequestID)
		}

		// Verify headers
		if w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
			t.Errorf("Expected Content-Type 'application/json; charset=utf-8', got %q", w.Header().Get("Content-Type"))
		}

		if w.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Errorf("Expected CORS origin '*', got %q", w.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	// Test error response
	t.Run("ErrorResponse", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = WriteError(w, http.StatusBadRequest, ErrorCodeBadRequest, "Test error", nil, "test-req-456")
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		mw.Handler(handler).ServeHTTP(w, req)

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
	})

	// Test paginated response
	t.Run("PaginatedResponse", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			data := []string{"item1", "item2", "item3"}
			_ = WritePaginated(w, data, 1, 10, 25, "test-req-789")
		})

		req := httptest.NewRequest("GET", "/test?page=1&page_size=10", nil)
		w := httptest.NewRecorder()

		mw.Handler(handler).ServeHTTP(w, req)

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

		if response.Pagination.Total != 25 {
			t.Errorf("Expected total 25, got %d", response.Pagination.Total)
		}

		if response.Pagination.TotalPages != 3 {
			t.Errorf("Expected total pages 3, got %d", response.Pagination.TotalPages)
		}
	})
}

// TestEndToEnd_ResponseLogging tests response logging functionality
func TestEndToEnd_ResponseLogging(t *testing.T) {
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

	start := time.Now()
	mw.Handler(handler).ServeHTTP(w, req)
	duration := time.Since(start)

	// Verify response was logged (no errors means logging worked)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Verify reasonable duration (should be very fast)
	if duration > 100*time.Millisecond {
		t.Errorf("Response took too long: %v", duration)
	}
}

// TestEndToEnd_CORSHeaders tests CORS header handling
func TestEndToEnd_CORSHeaders(t *testing.T) {
	tests := []struct {
		name           string
		origins        []string
		requestOrigin  string
		method         string
		expectedOrigin string
		expectedStatus int
	}{
		{
			name:           "Wildcard CORS",
			origins:        []string{"*"},
			requestOrigin:  "http://example.com",
			method:         "GET",
			expectedOrigin: "*",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Specific origin match",
			origins:        []string{"http://localhost:3000"},
			requestOrigin:  "http://localhost:3000",
			method:         "GET",
			expectedOrigin: "http://localhost:3000",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "OPTIONS preflight",
			origins:        []string{"*"},
			requestOrigin:  "http://example.com",
			method:         "OPTIONS",
			expectedOrigin: "*",
			expectedStatus: http.StatusNoContent,
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

			req := httptest.NewRequest(tt.method, "/test", nil)
			if tt.requestOrigin != "" {
				req.Header.Set("Origin", tt.requestOrigin)
			}
			w := httptest.NewRecorder()

			mw.Handler(handler).ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			actualOrigin := w.Header().Get("Access-Control-Allow-Origin")
			if actualOrigin != tt.expectedOrigin {
				t.Errorf("Expected origin %q, got %q", tt.expectedOrigin, actualOrigin)
			}

			if tt.method == "OPTIONS" {
				if w.Header().Get("Access-Control-Allow-Methods") == "" {
					t.Error("Expected CORS methods header for OPTIONS")
				}
			}
		})
	}
}

// TestEndToEnd_SecurityHeaders tests security headers
func TestEndToEnd_SecurityHeaders(t *testing.T) {
	logger := slog.Default()
	config := DefaultResponseConfig(logger)

	mw := NewResponseMiddleware(config)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	mw.Handler(handler).ServeHTTP(w, req)

	// Verify security headers
	securityHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"X-XSS-Protection":       "1; mode=block",
	}

	for header, expectedValue := range securityHeaders {
		actualValue := w.Header().Get(header)
		if actualValue != expectedValue {
			t.Errorf("Expected %s header %q, got %q", header, expectedValue, actualValue)
		}
	}
}

// TestEndToEnd_ResponseConsistency tests that all responses follow the same format
func TestEndToEnd_ResponseConsistency(t *testing.T) {
	logger := slog.Default()
	config := DefaultResponseConfig(logger)

	mw := NewResponseMiddleware(config)

	testCases := []struct {
		name     string
		handler  http.HandlerFunc
		validate func(t *testing.T, body []byte)
	}{
		{
			name: "Success response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_ = WriteSuccess(w, map[string]string{"key": "value"}, "req-1")
			},
			validate: func(t *testing.T, body []byte) {
				var resp StandardResponse
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatalf("Failed to unmarshal: %v", err)
				}
				if !resp.Success {
					t.Error("Expected success=true")
				}
				if resp.Timestamp == "" {
					t.Error("Expected timestamp to be set")
				}
			},
		},
		{
			name: "Error response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_ = WriteError(w, http.StatusBadRequest, ErrorCodeBadRequest, "Error", nil, "req-2")
			},
			validate: func(t *testing.T, body []byte) {
				var resp StandardResponse
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatalf("Failed to unmarshal: %v", err)
				}
				if resp.Success {
					t.Error("Expected success=false")
				}
				if resp.Error == nil {
					t.Error("Expected error to be set")
				}
				if resp.Timestamp == "" {
					t.Error("Expected timestamp to be set")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			mw.Handler(tc.handler).ServeHTTP(w, req)

			tc.validate(t, w.Body.Bytes())
		})
	}
}

