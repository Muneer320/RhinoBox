package errors

import (
	"errors"
	"testing"
)

func TestAPIError_Error(t *testing.T) {
	err := NewAPIError(ErrorCodeBadRequest, "test error")
	if err.Error() != "test error" {
		t.Errorf("expected 'test error', got %s", err.Error())
	}

	wrapped := WrapAPIError(ErrorCodeNotFound, "wrapped", errors.New("original"))
	if wrapped.Error() == "" {
		t.Error("wrapped error should have message")
	}
}

func TestAPIError_Unwrap(t *testing.T) {
	original := errors.New("original error")
	wrapped := WrapAPIError(ErrorCodeBadRequest, "wrapped", original)

	unwrapped := wrapped.Unwrap()
	if unwrapped != original {
		t.Errorf("expected unwrapped error to be original, got %v", unwrapped)
	}
}

func TestAPIError_WithDetails(t *testing.T) {
	err := NewAPIError(ErrorCodeBadRequest, "test")
	err.WithDetails("key1", "value1")
	err.WithDetails("key2", 42)

	if len(err.Details) != 2 {
		t.Errorf("expected 2 details, got %d", len(err.Details))
	}

	if err.Details["key1"] != "value1" {
		t.Errorf("expected key1=value1, got %v", err.Details["key1"])
	}

	if err.Details["key2"] != 42 {
		t.Errorf("expected key2=42, got %v", err.Details["key2"])
	}
}

func TestIsAPIError(t *testing.T) {
	apiErr := NewAPIError(ErrorCodeBadRequest, "test")
	if !IsAPIError(apiErr) {
		t.Error("expected IsAPIError to return true for APIError")
	}

	regularErr := errors.New("regular error")
	if IsAPIError(regularErr) {
		t.Error("expected IsAPIError to return false for regular error")
	}
}

func TestAsAPIError(t *testing.T) {
	apiErr := NewAPIError(ErrorCodeBadRequest, "test")
	wrapped := WrapAPIError(ErrorCodeNotFound, "wrapped", apiErr)

	extracted, ok := AsAPIError(wrapped)
	if !ok {
		t.Error("expected AsAPIError to extract APIError from wrapped error")
	}

	if extracted.Code != ErrorCodeNotFound {
		t.Errorf("expected code %s, got %s", ErrorCodeNotFound, extracted.Code)
	}
}

func TestErrorConstructors(t *testing.T) {
	tests := []struct {
		name string
		fn   func(string) *APIError
		code ErrorCode
	}{
		{"BadRequest", BadRequest, ErrorCodeBadRequest},
		{"NotFound", NotFound, ErrorCodeNotFound},
		{"InternalServerError", InternalServerError, ErrorCodeInternalServerError},
		{"Conflict", Conflict, ErrorCodeConflict},
		{"ValidationFailed", ValidationFailed, ErrorCodeValidationFailed},
		{"Timeout", Timeout, ErrorCodeTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn("test message")
			if err.Code != tt.code {
				t.Errorf("expected code %s, got %s", tt.code, err.Code)
			}
			if err.Message != "test message" {
				t.Errorf("expected message 'test message', got %s", err.Message)
			}
		})
	}
}

func TestErrorConstructorsf(t *testing.T) {
	err := BadRequestf("error: %s", "test")
	if err.Code != ErrorCodeBadRequest {
		t.Errorf("expected code %s, got %s", ErrorCodeBadRequest, err.Code)
	}
	if err.Message != "error: test" {
		t.Errorf("expected message 'error: test', got %s", err.Message)
	}

	err2 := NotFoundf("not found: %d", 404)
	if err2.Code != ErrorCodeNotFound {
		t.Errorf("expected code %s, got %s", ErrorCodeNotFound, err2.Code)
	}
	if err2.Message != "not found: 404" {
		t.Errorf("expected message 'not found: 404', got %s", err2.Message)
	}
}

