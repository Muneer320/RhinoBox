package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	chi "github.com/go-chi/chi/v5"
	"log/slog"
)

func TestValidator_ValidatePathParams(t *testing.T) {
	logger := slog.Default()
	validator := NewValidator(logger)

	// Register a test schema
	validator.RegisterSchema("GET:/test/{id}", &Schema{
		PathParams: map[string]PathParamRule{
			"id": {
				Required: true,
				Validate: ValidateNonEmpty,
			},
		},
	})

	// Test valid path param
	req := httptest.NewRequest("GET", "/test/123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "123")
	rctx.RoutePath = "/test/{id}"
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	handler := validator.Validate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	// Test missing path param
	req2 := httptest.NewRequest("GET", "/test/", nil)
	rctx2 := chi.NewRouteContext()
	rctx2.RoutePath = "/test/{id}"
	req2 = req2.WithContext(context.WithValue(req2.Context(), chi.RouteCtxKey, rctx2))

	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr2.Code)
	}

	var errorResp ValidationErrorResponse
	if err := json.Unmarshal(rr2.Body.Bytes(), &errorResp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}

	if errorResp.Error != "validation failed" {
		t.Errorf("expected error message 'validation failed', got '%s'", errorResp.Error)
	}
}

func TestValidator_ValidateQueryParams(t *testing.T) {
	logger := slog.Default()
	validator := NewValidator(logger)

	validator.RegisterSchema("GET:/test", &Schema{
		QueryParams: map[string]QueryParamRule{
			"name": {
				Required: true,
				Validate: ValidateNonEmpty,
			},
			"limit": {
				Required: false,
				Validate: ValidateIntRange(1, 100),
			},
		},
	})

	// Test valid query params
	req := httptest.NewRequest("GET", "/test?name=test&limit=10", nil)
	rr := httptest.NewRecorder()
	handler := validator.Validate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	// Test missing required param
	req2 := httptest.NewRequest("GET", "/test", nil)
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr2.Code)
	}

	// Test invalid limit
	req3 := httptest.NewRequest("GET", "/test?name=test&limit=200", nil)
	rr3 := httptest.NewRecorder()
	handler.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr3.Code)
	}
}

func TestValidator_ValidateBody(t *testing.T) {
	logger := slog.Default()
	validator := NewValidator(logger)

	validator.RegisterSchema("POST:/test", &Schema{
		BodyRequired: true,
		BodySchema: func(data interface{}) []ValidationError {
			var errors []ValidationError
			bodyMap, ok := data.(map[string]interface{})
			if !ok {
				return []ValidationError{{
					Field:   "body",
					Message: "request body must be a JSON object",
				}}
			}

			if name, exists := bodyMap["name"]; !exists || name == nil {
				errors = append(errors, ValidationError{
					Field:   "name",
					Message: "name is required",
				})
			}

			return errors
		},
	})

	// Test valid body
	validBody := `{"name": "test"}`
	req := httptest.NewRequest("POST", "/test", strings.NewReader(validBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler := validator.Validate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	// Test missing body
	req2 := httptest.NewRequest("POST", "/test", nil)
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr2.Code)
	}

	// Test invalid JSON
	req3 := httptest.NewRequest("POST", "/test", strings.NewReader("invalid json"))
	req3.Header.Set("Content-Type", "application/json")
	rr3 := httptest.NewRecorder()
	handler.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr3.Code)
	}

	// Test missing required field
	req4 := httptest.NewRequest("POST", "/test", strings.NewReader(`{}`))
	req4.Header.Set("Content-Type", "application/json")
	rr4 := httptest.NewRecorder()
	handler.ServeHTTP(rr4, req4)

	if rr4.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr4.Code)
	}
}

func TestValidator_ValidateFileUpload(t *testing.T) {
	logger := slog.Default()
	validator := NewValidator(logger)

	validator.RegisterSchema("POST:/upload", &Schema{
		FileUpload: &FileUploadRule{
			Required:     true,
			MaxSize:      1024 * 1024, // 1MB
			MaxFiles:     5,
			FieldName:    "file",
			AllowedTypes: []string{"image/jpeg", "image/png"},
			AllowedExts:  []string{".jpg", ".jpeg", ".png"},
		},
	})

	// Test valid file upload
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.jpg")
	part.Write([]byte("fake image data"))
	writer.Close()

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	handler := validator.Validate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Parse multipart form first
	req.ParseMultipartForm(10 << 20)
	handler.ServeHTTP(rr, req)

	// Note: File validation happens after parsing, so we need to check the actual file
	// For this test, we'll just verify the middleware doesn't crash
	if rr.Code == http.StatusInternalServerError {
		t.Errorf("unexpected internal server error: %s", rr.Body.String())
	}
}

func TestValidateHash(t *testing.T) {
	tests := []struct {
		name    string
		hash    string
		wantErr bool
	}{
		{"valid SHA256", "a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456", false},
		{"valid MD5", "1234567890abcdef1234567890abcdef", false},
		{"too short", "abc123", true},
		{"invalid chars", "ghijklmnopqrstuvwxyz1234567890abcdef", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHash(tt.hash)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHash() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFilename(t *testing.T) {
	tests := []struct {
		name    string
		filename string
		wantErr bool
	}{
		{"valid", "test.txt", false},
		{"valid with spaces", "my file.txt", false},
		{"empty", "", true},
		{"too long", strings.Repeat("a", 256), true},
		{"path traversal", "../../etc/passwd", true},
		{"with slash", "path/to/file.txt", true},
		{"with backslash", "path\\to\\file.txt", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilename(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFilename() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateIntRange(t *testing.T) {
	validator := ValidateIntRange(1, 100)

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid min", "1", false},
		{"valid max", "100", false},
		{"valid middle", "50", false},
		{"too small", "0", true},
		{"too large", "101", true},
		{"not a number", "abc", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateIntRange() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateOneOf(t *testing.T) {
	validator := ValidateOneOf("red", "green", "blue")

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid red", "red", false},
		{"valid green", "green", false},
		{"valid blue", "blue", false},
		{"invalid", "yellow", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateOneOf() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMatchRoutePattern(t *testing.T) {
	logger := slog.Default()
	validator := NewValidator(logger)

	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{"exact match", "/test", "/test", true},
		{"with param", "/files/{file_id}", "/files/abc123", true},
		{"multiple params", "/users/{user_id}/files/{file_id}", "/users/1/files/2", true},
		{"no match", "/test", "/other", false},
		{"wrong length", "/test/{id}", "/test", false},
		{"empty param", "/test/{id}", "/test/", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validator.matchRoutePattern(tt.pattern, tt.path)
			if got != tt.want {
				t.Errorf("matchRoutePattern() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidator_NoSchema(t *testing.T) {
	logger := slog.Default()
	validator := NewValidator(logger)

	// Request to route without schema
	req := httptest.NewRequest("GET", "/no-schema", nil)
	rr := httptest.NewRecorder()
	handler := validator.Validate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	if rr.Body.String() != "ok" {
		t.Errorf("expected body 'ok', got '%s'", rr.Body.String())
	}
}

func TestValidator_ErrorResponseFormat(t *testing.T) {
	logger := slog.Default()
	validator := NewValidator(logger)

	validator.RegisterSchema("POST:/test", &Schema{
		BodyRequired: true,
	})

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler := validator.Validate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var errorResp ValidationErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &errorResp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}

	if errorResp.Error != "validation failed" {
		t.Errorf("expected error message 'validation failed', got '%s'", errorResp.Error)
	}

	if len(errorResp.Details) == 0 {
		t.Error("expected validation error details, got none")
	}
}

// Test file upload validation with actual multipart data
func TestValidator_FileUploadValidation(t *testing.T) {
	logger := slog.Default()
	validator := NewValidator(logger)

	validator.RegisterSchema("POST:/upload", &Schema{
		FileUpload: &FileUploadRule{
			Required:  true,
			MaxSize:   100, // Very small for testing
			MaxFiles:  1,
			FieldName: "file",
		},
	})

	// Create multipart form with file that's too large
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "large.txt")
	largeData := make([]byte, 200) // Larger than MaxSize
	part.Write(largeData)
	writer.Close()

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	
	// Parse multipart form
	err := req.ParseMultipartForm(10 << 20)
	if err != nil {
		t.Fatalf("failed to parse multipart form: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := validator.Validate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rr, req)

	// Should fail validation due to file size
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for oversized file, got %d", rr.Code)
	}
}

// Test that body is restored after validation
func TestValidator_BodyRestoration(t *testing.T) {
	logger := slog.Default()
	validator := NewValidator(logger)

	originalBody := `{"name": "test"}`
	validator.RegisterSchema("POST:/test", &Schema{
		BodyRequired: true,
		BodySchema: func(data interface{}) []ValidationError {
			return nil // Pass validation
		},
	})

	req := httptest.NewRequest("POST", "/test", strings.NewReader(originalBody))
	req.Header.Set("Content-Type", "application/json")

	var capturedBody []byte
	handler := validator.Validate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if string(capturedBody) != originalBody {
		t.Errorf("body was not restored correctly. Expected: %s, Got: %s", originalBody, string(capturedBody))
	}
}

