package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	apierrors "github.com/Muneer320/RhinoBox/internal/errors"
	"github.com/Muneer320/RhinoBox/internal/storage"
)

// ErrorHandler provides centralized error handling middleware
type ErrorHandler struct {
	logger  *slog.Logger
	metrics *ErrorMetrics
}

// ErrorMetrics tracks error statistics
type ErrorMetrics struct {
	TotalErrors       int64
	ErrorsByCode      map[apierrors.ErrorCode]int64
	ErrorsByStatus    map[int]int64
	PanicsRecovered   int64
	LastErrorTime     time.Time
}

// NewErrorHandler creates a new error handler middleware
func NewErrorHandler(logger *slog.Logger) *ErrorHandler {
	return &ErrorHandler{
		logger: logger,
		metrics: &ErrorMetrics{
			ErrorsByCode:   make(map[apierrors.ErrorCode]int64),
			ErrorsByStatus: make(map[int]int64),
		},
	}
}

// Handler wraps an http.Handler with error handling and panic recovery
func (eh *ErrorHandler) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				eh.handlePanic(w, r, rec)
			}
		}()

		// Wrap the response writer to capture status code
		ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Call the next handler
		next.ServeHTTP(ww, r)

		// If status code indicates an error, log it
		if ww.statusCode >= 400 {
			eh.logError(r, ww.statusCode, nil, "handler returned error status")
		}
	})
}

// HandleError processes an error and writes an appropriate HTTP response
func (eh *ErrorHandler) HandleError(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}

	apiErr, statusCode := eh.mapError(err)
	eh.writeErrorResponse(w, r, apiErr, statusCode)
	eh.recordMetrics(apiErr.Code, statusCode)
}

// mapError maps various error types to APIError and HTTP status code
func (eh *ErrorHandler) mapError(err error) (*apierrors.APIError, int) {
	// Check if it's already an APIError
	if apiErr, ok := apierrors.AsAPIError(err); ok {
		return apiErr, eh.statusCodeFromErrorCode(apiErr.Code)
	}

	// Map storage errors
	if errors.Is(err, storage.ErrFileNotFound) {
		return apierrors.NotFound("file not found"), http.StatusNotFound
	}
	if errors.Is(err, storage.ErrInvalidPath) {
		return apierrors.BadRequest("invalid path"), http.StatusBadRequest
	}
	if errors.Is(err, storage.ErrInvalidInput) {
		return apierrors.BadRequest("invalid input"), http.StatusBadRequest
	}
	if errors.Is(err, storage.ErrInvalidFilename) {
		return apierrors.ValidationFailed("invalid filename"), http.StatusBadRequest
	}
	if errors.Is(err, storage.ErrNameConflict) {
		return apierrors.Conflict("filename conflict"), http.StatusConflict
	}
	if errors.Is(err, storage.ErrMetadataNotFound) {
		return apierrors.NotFound("metadata not found"), http.StatusNotFound
	}
	if errors.Is(err, storage.ErrMetadataTooLarge) {
		return apierrors.BadRequest("metadata exceeds size limit"), http.StatusBadRequest
	}
	if errors.Is(err, storage.ErrInvalidMetadataKey) {
		return apierrors.ValidationFailed("invalid metadata key"), http.StatusBadRequest
	}
	if errors.Is(err, storage.ErrProtectedField) {
		return apierrors.BadRequest("cannot modify protected metadata field"), http.StatusBadRequest
	}

	// Check for context errors (timeouts, cancellations)
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return apierrors.Timeout("request timeout"), http.StatusRequestTimeout
	}

	// Default to internal server error
	return apierrors.InternalServerError("an unexpected error occurred"), http.StatusInternalServerError
}

// statusCodeFromErrorCode maps ErrorCode to HTTP status code
func (eh *ErrorHandler) statusCodeFromErrorCode(code apierrors.ErrorCode) int {
	switch code {
	case apierrors.ErrorCodeBadRequest, apierrors.ErrorCodeValidationFailed:
		return http.StatusBadRequest
	case apierrors.ErrorCodeUnauthorized:
		return http.StatusUnauthorized
	case apierrors.ErrorCodeForbidden:
		return http.StatusForbidden
	case apierrors.ErrorCodeNotFound:
		return http.StatusNotFound
	case apierrors.ErrorCodeConflict:
		return http.StatusConflict
	case apierrors.ErrorCodeRequestTooLarge:
		return http.StatusRequestEntityTooLarge
	case apierrors.ErrorCodeRangeNotSatisfiable:
		return http.StatusRequestedRangeNotSatisfiable
	case apierrors.ErrorCodeTimeout:
		return http.StatusRequestTimeout
	case apierrors.ErrorCodeNotImplemented:
		return http.StatusNotImplemented
	case apierrors.ErrorCodeServiceUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// handlePanic recovers from panics and logs them properly
func (eh *ErrorHandler) handlePanic(w http.ResponseWriter, r *http.Request, rec interface{}) {
	eh.metrics.PanicsRecovered++
	eh.metrics.LastErrorTime = time.Now()

	stack := debug.Stack()

	// Log panic with full context
	eh.logger.Error("panic recovered",
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("remote_addr", r.RemoteAddr),
		slog.String("panic", fmt.Sprintf("%v", rec)),
		slog.String("stack", string(stack)),
		slog.String("request_id", r.Header.Get("X-Request-Id")),
	)

	// Create error response
	apiErr := apierrors.InternalServerError("an internal error occurred")
	eh.writeErrorResponse(w, r, apiErr, http.StatusInternalServerError)
	eh.recordMetrics(apiErr.Code, http.StatusInternalServerError)
}

// writeErrorResponse writes a standardized error response
func (eh *ErrorHandler) writeErrorResponse(w http.ResponseWriter, r *http.Request, apiErr *apierrors.APIError, statusCode int) {
	// Log the error with context
	eh.logError(r, statusCode, apiErr.Err, apiErr.Message)

	// Build response payload
	response := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    apiErr.Code,
			"message": apiErr.Message,
		},
	}

	// Add details if present
	if len(apiErr.Details) > 0 {
		response["error"].(map[string]interface{})["details"] = apiErr.Details
	}

	// Add request ID if available
	if requestID := r.Header.Get("X-Request-Id"); requestID != "" {
		response["request_id"] = requestID
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(response)
}

// logError logs an error with full context
func (eh *ErrorHandler) logError(r *http.Request, statusCode int, err error, message string) {
	args := []any{
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.Int("status", statusCode),
		slog.String("remote_addr", r.RemoteAddr),
		slog.String("user_agent", r.UserAgent()),
	}

	if requestID := r.Header.Get("X-Request-Id"); requestID != "" {
		args = append(args, slog.String("request_id", requestID))
	}

	if err != nil {
		args = append(args, slog.Any("error", err))
	}

	// Use appropriate log level based on status code
	if statusCode >= 500 {
		eh.logger.Error(message, args...)
	} else if statusCode >= 400 {
		eh.logger.Warn(message, args...)
	} else {
		eh.logger.Debug(message, args...)
	}
}

// recordMetrics updates error metrics
func (eh *ErrorHandler) recordMetrics(code apierrors.ErrorCode, statusCode int) {
	eh.metrics.TotalErrors++
	eh.metrics.ErrorsByCode[code]++
	eh.metrics.ErrorsByStatus[statusCode]++
	eh.metrics.LastErrorTime = time.Now()
}

// GetMetrics returns current error metrics
func (eh *ErrorHandler) GetMetrics() ErrorMetrics {
	return *eh.metrics
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	return rw.ResponseWriter.Write(b)
}

