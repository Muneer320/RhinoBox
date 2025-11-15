package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apierrors "github.com/Muneer320/RhinoBox/internal/errors"
	"github.com/Muneer320/RhinoBox/internal/storage"
	"log/slog"
)

func TestErrorHandler_HandleError(t *testing.T) {
	logger := slog.Default()
	handler := NewErrorHandler(logger)

	tests := []struct {
		name           string
		err            error
		expectedCode   int
		expectedStatus int
	}{
		{
			name:           "APIError BadRequest",
			err:            apierrors.BadRequest("invalid input"),
			expectedCode:   http.StatusBadRequest,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "APIError NotFound",
			err:            apierrors.NotFound("file not found"),
			expectedCode:   http.StatusNotFound,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Storage ErrFileNotFound",
			err:            storage.ErrFileNotFound,
			expectedCode:   http.StatusNotFound,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Storage ErrInvalidPath",
			err:            storage.ErrInvalidPath,
			expectedCode:   http.StatusBadRequest,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Storage ErrInvalidInput",
			err:            storage.ErrInvalidInput,
			expectedCode:   http.StatusBadRequest,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Storage ErrNameConflict",
			err:            storage.ErrNameConflict,
			expectedCode:   http.StatusConflict,
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "Context timeout",
			err:            context.DeadlineExceeded,
			expectedCode:   http.StatusRequestTimeout,
			expectedStatus: http.StatusRequestTimeout,
		},
		{
			name:           "Unknown error",
			err:            errors.New("unknown error"),
			expectedCode:   http.StatusInternalServerError,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("X-Request-Id", "test-request-id")
			w := httptest.NewRecorder()

			handler.HandleError(w, req, tt.err)

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

			if code != string(apierrors.ErrorCode(tt.name)) && code != string(apierrors.ErrorCodeBadRequest) {
				// Check if it's a valid error code
				validCodes := []string{
					string(apierrors.ErrorCodeBadRequest),
					string(apierrors.ErrorCodeNotFound),
					string(apierrors.ErrorCodeConflict),
					string(apierrors.ErrorCodeTimeout),
					string(apierrors.ErrorCodeInternalServerError),
				}
				found := false
				for _, validCode := range validCodes {
					if code == validCode {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("unexpected error code: %s", code)
				}
			}
		})
	}
}

func TestErrorHandler_PanicRecovery(t *testing.T) {
	logger := slog.Default()
	handler := NewErrorHandler(logger)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Create a handler that panics
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	// Wrap with error handler
	wrapped := handler.Handler(panicHandler)
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
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

	if code != string(apierrors.ErrorCodeInternalServerError) {
		t.Errorf("expected code %s, got %s", apierrors.ErrorCodeInternalServerError, code)
	}

	// Check metrics
	metrics := handler.GetMetrics()
	if metrics.PanicsRecovered != 1 {
		t.Errorf("expected 1 panic recovered, got %d", metrics.PanicsRecovered)
	}
}

func TestErrorHandler_Metrics(t *testing.T) {
	logger := slog.Default()
	handler := NewErrorHandler(logger)

	req := httptest.NewRequest("GET", "/test", nil)

	// Generate some errors
	errors := []error{
		apierrors.BadRequest("error 1"),
		apierrors.NotFound("error 2"),
		apierrors.BadRequest("error 3"),
	}

	for _, err := range errors {
		w := httptest.NewRecorder()
		handler.HandleError(w, req, err)
	}

	metrics := handler.GetMetrics()
	if metrics.TotalErrors != 3 {
		t.Errorf("expected 3 total errors, got %d", metrics.TotalErrors)
	}

	if metrics.ErrorsByCode[apierrors.ErrorCodeBadRequest] != 2 {
		t.Errorf("expected 2 BadRequest errors, got %d", metrics.ErrorsByCode[apierrors.ErrorCodeBadRequest])
	}

	if metrics.ErrorsByCode[apierrors.ErrorCodeNotFound] != 1 {
		t.Errorf("expected 1 NotFound error, got %d", metrics.ErrorsByCode[apierrors.ErrorCodeNotFound])
	}
}

func TestErrorHandler_StatusCodeMapping(t *testing.T) {
	logger := slog.Default()
	handler := NewErrorHandler(logger)

	tests := []struct {
		code           apierrors.ErrorCode
		expectedStatus int
	}{
		{apierrors.ErrorCodeBadRequest, http.StatusBadRequest},
		{apierrors.ErrorCodeNotFound, http.StatusNotFound},
		{apierrors.ErrorCodeConflict, http.StatusConflict},
		{apierrors.ErrorCodeTimeout, http.StatusRequestTimeout},
		{apierrors.ErrorCodeInternalServerError, http.StatusInternalServerError},
		{apierrors.ErrorCodeUnauthorized, http.StatusUnauthorized},
		{apierrors.ErrorCodeForbidden, http.StatusForbidden},
		{apierrors.ErrorCodeRequestTooLarge, http.StatusRequestEntityTooLarge},
		{apierrors.ErrorCodeRangeNotSatisfiable, http.StatusRequestedRangeNotSatisfiable},
		{apierrors.ErrorCodeNotImplemented, http.StatusNotImplemented},
		{apierrors.ErrorCodeServiceUnavailable, http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			status := handler.statusCodeFromErrorCode(tt.code)
			if status != tt.expectedStatus {
				t.Errorf("expected status %d for code %s, got %d", tt.expectedStatus, tt.code, status)
			}
		})
	}
}

func TestErrorHandler_ErrorResponseFormat(t *testing.T) {
	logger := slog.Default()
	handler := NewErrorHandler(logger)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-Id", "test-123")
	w := httptest.NewRecorder()

	apiErr := apierrors.BadRequest("test error").WithDetails("field", "value")
	handler.HandleError(w, req, apiErr)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Check error structure
	errorObj, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error object in response")
	}

	if errorObj["code"] != string(apierrors.ErrorCodeBadRequest) {
		t.Errorf("expected code %s, got %v", apierrors.ErrorCodeBadRequest, errorObj["code"])
	}

	if errorObj["message"] != "test error" {
		t.Errorf("expected message 'test error', got %v", errorObj["message"])
	}

	// Check details
	details, ok := errorObj["details"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected details in error object")
	}

	if details["field"] != "value" {
		t.Errorf("expected detail field=value, got %v", details["field"])
	}

	// Check request ID
	if response["request_id"] != "test-123" {
		t.Errorf("expected request_id 'test-123', got %v", response["request_id"])
	}
}

func TestErrorHandler_Logging(t *testing.T) {
	logger := slog.Default()
	handler := NewErrorHandler(logger)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()

	// Test that logging doesn't panic
	handler.HandleError(w, req, apierrors.BadRequest("test"))

	// Should complete without panic
	if w.Code == 0 {
		t.Error("handler did not write response")
	}
}

func TestErrorHandler_ContextErrors(t *testing.T) {
	logger := slog.Default()
	handler := NewErrorHandler(logger)

	tests := []struct {
		name         string
		err          error
		expectedCode int
	}{
		{
			name:         "DeadlineExceeded",
			err:          context.DeadlineExceeded,
			expectedCode: http.StatusRequestTimeout,
		},
		{
			name:         "Canceled",
			err:          context.Canceled,
			expectedCode: http.StatusRequestTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			handler.HandleError(w, req, tt.err)

			if w.Code != tt.expectedCode {
				t.Errorf("expected status %d, got %d", tt.expectedCode, w.Code)
			}
		})
	}
}

func TestErrorHandler_LastErrorTime(t *testing.T) {
	logger := slog.Default()
	handler := NewErrorHandler(logger)

	initialTime := handler.GetMetrics().LastErrorTime

	// Wait a bit to ensure time difference
	time.Sleep(10 * time.Millisecond)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.HandleError(w, req, apierrors.BadRequest("test"))

	metrics := handler.GetMetrics()
	if !metrics.LastErrorTime.After(initialTime) {
		t.Error("LastErrorTime should be updated after handling error")
	}
}

