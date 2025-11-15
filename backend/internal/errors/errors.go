package errors

import (
	"errors"
	"fmt"
)

// ErrorCode represents a standardized error code for frontend consumption
type ErrorCode string

const (
	// Client errors (4xx)
	ErrorCodeBadRequest          ErrorCode = "BAD_REQUEST"
	ErrorCodeUnauthorized        ErrorCode = "UNAUTHORIZED"
	ErrorCodeForbidden           ErrorCode = "FORBIDDEN"
	ErrorCodeNotFound            ErrorCode = "NOT_FOUND"
	ErrorCodeConflict            ErrorCode = "CONFLICT"
	ErrorCodeValidationFailed    ErrorCode = "VALIDATION_FAILED"
	ErrorCodeRequestTooLarge     ErrorCode = "REQUEST_TOO_LARGE"
	ErrorCodeRangeNotSatisfiable ErrorCode = "RANGE_NOT_SATISFIABLE"
	ErrorCodeTimeout             ErrorCode = "TIMEOUT"

	// Server errors (5xx)
	ErrorCodeInternalServerError ErrorCode = "INTERNAL_SERVER_ERROR"
	ErrorCodeServiceUnavailable   ErrorCode = "SERVICE_UNAILABLE"
	ErrorCodeNotImplemented       ErrorCode = "NOT_IMPLEMENTED"
)

// APIError represents a structured error response
type APIError struct {
	Code    ErrorCode              `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
	Err     error                  `json:"-"` // Original error, not serialized
}

// Error implements the error interface
func (e *APIError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap returns the underlying error
func (e *APIError) Unwrap() error {
	return e.Err
}

// WithDetails adds additional context to the error
func (e *APIError) WithDetails(key string, value interface{}) *APIError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// NewAPIError creates a new APIError
func NewAPIError(code ErrorCode, message string) *APIError {
	return &APIError{
		Code:    code,
		Message: message,
		Details: make(map[string]interface{}),
	}
}

// WrapAPIError wraps an existing error with an APIError
func WrapAPIError(code ErrorCode, message string, err error) *APIError {
	return &APIError{
		Code:    code,
		Message: message,
		Err:     err,
		Details: make(map[string]interface{}),
	}
}

// IsAPIError checks if an error is an APIError
func IsAPIError(err error) bool {
	_, ok := err.(*APIError)
	return ok
}

// AsAPIError extracts an APIError from an error chain
func AsAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	ok := errors.As(err, &apiErr)
	return apiErr, ok
}

// Common error constructors
func BadRequest(message string) *APIError {
	return NewAPIError(ErrorCodeBadRequest, message)
}

func BadRequestf(format string, args ...interface{}) *APIError {
	return NewAPIError(ErrorCodeBadRequest, fmt.Sprintf(format, args...))
}

func NotFound(message string) *APIError {
	return NewAPIError(ErrorCodeNotFound, message)
}

func NotFoundf(format string, args ...interface{}) *APIError {
	return NewAPIError(ErrorCodeNotFound, fmt.Sprintf(format, args...))
}

func InternalServerError(message string) *APIError {
	return NewAPIError(ErrorCodeInternalServerError, message)
}

func InternalServerErrorf(format string, args ...interface{}) *APIError {
	return NewAPIError(ErrorCodeInternalServerError, fmt.Sprintf(format, args...))
}

func Conflict(message string) *APIError {
	return NewAPIError(ErrorCodeConflict, message)
}

func ValidationFailed(message string) *APIError {
	return NewAPIError(ErrorCodeValidationFailed, message)
}

func Timeout(message string) *APIError {
	return NewAPIError(ErrorCodeTimeout, message)
}

