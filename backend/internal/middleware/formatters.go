package middleware

import (
	"encoding/json"
	"net/http"
	"time"
)

// StandardResponse represents a standardized API response format
type StandardResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     *ErrorInfo  `json:"error,omitempty"`
	Timestamp string      `json:"timestamp"`
	RequestID string      `json:"request_id,omitempty"`
}

// ErrorInfo represents error information in responses
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// PaginationInfo represents pagination metadata
type PaginationInfo struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

// PaginatedResponse represents a paginated API response
type PaginatedResponse struct {
	Success    bool          `json:"success"`
	Data       interface{}   `json:"data"`
	Pagination PaginationInfo `json:"pagination"`
	Timestamp  string        `json:"timestamp"`
	RequestID  string        `json:"request_id,omitempty"`
}

// ResponseFormatter provides utilities for formatting responses
type ResponseFormatter struct{}

// NewResponseFormatter creates a new response formatter
func NewResponseFormatter() *ResponseFormatter {
	return &ResponseFormatter{}
}

// Success formats a successful response
func (rf *ResponseFormatter) Success(w http.ResponseWriter, data interface{}, requestID string) error {
	response := StandardResponse{
		Success:   true,
		Data:      data,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		RequestID: requestID,
	}
	
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	return json.NewEncoder(w).Encode(response)
}

// Error formats an error response
func (rf *ResponseFormatter) Error(w http.ResponseWriter, statusCode int, code, message string, details interface{}, requestID string) error {
	response := StandardResponse{
		Success: false,
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
			Details: details,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		RequestID: requestID,
	}
	
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	return json.NewEncoder(w).Encode(response)
}

// Paginated formats a paginated response
func (rf *ResponseFormatter) Paginated(w http.ResponseWriter, data interface{}, page, pageSize, total int, requestID string) error {
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}
	
	response := PaginatedResponse{
		Success: true,
		Data:    data,
		Pagination: PaginationInfo{
			Page:       page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: totalPages,
			HasNext:    page < totalPages,
			HasPrev:    page > 1,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		RequestID: requestID,
	}
	
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	return json.NewEncoder(w).Encode(response)
}

// WriteSuccess is a convenience function for writing success responses
func WriteSuccess(w http.ResponseWriter, data interface{}, requestID string) error {
	formatter := NewResponseFormatter()
	return formatter.Success(w, data, requestID)
}

// WriteError is a convenience function for writing error responses
func WriteError(w http.ResponseWriter, statusCode int, code, message string, details interface{}, requestID string) error {
	formatter := NewResponseFormatter()
	return formatter.Error(w, statusCode, code, message, details, requestID)
}

// WritePaginated is a convenience function for writing paginated responses
func WritePaginated(w http.ResponseWriter, data interface{}, page, pageSize, total int, requestID string) error {
	formatter := NewResponseFormatter()
	return formatter.Paginated(w, data, page, pageSize, total, requestID)
}

// ErrorCode constants for common error types
const (
	ErrorCodeBadRequest       = "BAD_REQUEST"
	ErrorCodeUnauthorized     = "UNAUTHORIZED"
	ErrorCodeForbidden        = "FORBIDDEN"
	ErrorCodeNotFound         = "NOT_FOUND"
	ErrorCodeConflict         = "CONFLICT"
	ErrorCodeValidation       = "VALIDATION_ERROR"
	ErrorCodeInternalError    = "INTERNAL_ERROR"
	ErrorCodeServiceUnavailable = "SERVICE_UNAVAILABLE"
	ErrorCodeTimeout          = "TIMEOUT"
)

// MapHTTPStatusToErrorCode maps HTTP status codes to error codes
func MapHTTPStatusToErrorCode(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest:
		return ErrorCodeBadRequest
	case http.StatusUnauthorized:
		return ErrorCodeUnauthorized
	case http.StatusForbidden:
		return ErrorCodeForbidden
	case http.StatusNotFound:
		return ErrorCodeNotFound
	case http.StatusConflict:
		return ErrorCodeConflict
	case http.StatusInternalServerError:
		return ErrorCodeInternalError
	case http.StatusServiceUnavailable:
		return ErrorCodeServiceUnavailable
	case http.StatusRequestTimeout:
		return ErrorCodeTimeout
	default:
		return ErrorCodeInternalError
	}
}

