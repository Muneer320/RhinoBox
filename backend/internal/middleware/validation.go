package middleware

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	chi "github.com/go-chi/chi/v5"
)

// ValidationError represents a validation error with field and message
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrorResponse is the standard error response format
type ValidationErrorResponse struct {
	Error   string           `json:"error"`
	Details []ValidationError `json:"details,omitempty"`
}

// Validator holds validation configuration and schemas
type Validator struct {
	logger  *slog.Logger
	schemas map[string]*Schema
}

// Schema defines validation rules for an endpoint
type Schema struct {
	// Body validation
	BodyRequired bool
	BodySchema   func(interface{}) []ValidationError

	// Query parameter validation
	QueryParams map[string]QueryParamRule

	// Path parameter validation
	PathParams map[string]PathParamRule

	// File upload validation
	FileUpload *FileUploadRule
}

// QueryParamRule defines validation for a query parameter
type QueryParamRule struct {
	Required bool
	Validate func(string) error
}

// PathParamRule defines validation for a path parameter
type PathParamRule struct {
	Required bool
	Validate func(string) error
}

// FileUploadRule defines validation for file uploads
type FileUploadRule struct {
	Required        bool
	MaxSize         int64
	AllowedTypes    []string
	AllowedExts     []string
	MaxFiles        int
	FieldName       string // form field name (e.g., "file", "files")
}

// NewValidator creates a new validator instance
func NewValidator(logger *slog.Logger) *Validator {
	return &Validator{
		logger:  logger,
		schemas: make(map[string]*Schema),
	}
}

// RegisterSchema registers a validation schema for a route
func (v *Validator) RegisterSchema(route string, schema *Schema) {
	v.schemas[route] = schema
}

// Validate is the middleware function that validates requests
func (v *Validator) Validate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := v.getRouteKey(r)
		schema, exists := v.schemas[route]
		if !exists {
			// No schema registered, proceed without validation
			next.ServeHTTP(w, r)
			return
		}

		var errors []ValidationError

		// Validate path parameters
		if schema.PathParams != nil {
			if pathErrors := v.validatePathParams(r, schema.PathParams); len(pathErrors) > 0 {
				errors = append(errors, pathErrors...)
			}
		}

		// Validate query parameters
		if schema.QueryParams != nil {
			if queryErrors := v.validateQueryParams(r, schema.QueryParams); len(queryErrors) > 0 {
				errors = append(errors, queryErrors...)
			}
			// Special case: validate that at least one of hash or path is provided for download/stream endpoints
			if route == "GET:/files/download" || route == "GET:/files/stream" {
				hash := r.URL.Query().Get("hash")
				path := r.URL.Query().Get("path")
				if hash == "" && path == "" {
					errors = append(errors, ValidationError{
						Field:   "hash",
						Message: "either 'hash' or 'path' query parameter is required",
					})
				}
			}
		}

		// Validate request body (for JSON requests)
		if schema.BodyRequired || schema.BodySchema != nil {
			if bodyErrors := v.validateBody(r, schema); len(bodyErrors) > 0 {
				errors = append(errors, bodyErrors...)
			}
		}

		// Validate file uploads (for multipart requests)
		if schema.FileUpload != nil {
			if fileErrors := v.validateFileUpload(r, schema.FileUpload); len(fileErrors) > 0 {
				errors = append(errors, fileErrors...)
			}
		}

		// If validation errors exist, return error response
		if len(errors) > 0 {
			v.writeValidationError(w, errors)
			return
		}

		// Validation passed, proceed to next handler
		next.ServeHTTP(w, r)
	})
}

// getRouteKey generates a unique key for the route
func (v *Validator) getRouteKey(r *http.Request) string {
	// Use method + path pattern (with {param} placeholders)
	route := chi.RouteContext(r.Context())
	if route != nil && route.RoutePath != "" {
		return r.Method + ":" + route.RoutePath
	}
	// Try to match against registered routes by pattern
	path := r.URL.Path
	method := r.Method
	
	// Match path patterns (e.g., /files/{file_id} matches /files/abc123)
	for routeKey := range v.schemas {
		if strings.HasPrefix(routeKey, method+":") {
			pattern := strings.TrimPrefix(routeKey, method+":")
			if v.matchRoutePattern(pattern, path) {
				return routeKey
			}
		}
	}
	
	// Fallback to method + exact path
	return method + ":" + path
}

// matchRoutePattern checks if a path matches a route pattern
func (v *Validator) matchRoutePattern(pattern, path string) bool {
	// Convert pattern like "/files/{file_id}" to regex
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")
	
	if len(patternParts) != len(pathParts) {
		return false
	}
	
	for i, patternPart := range patternParts {
		if strings.HasPrefix(patternPart, "{") && strings.HasSuffix(patternPart, "}") {
			// This is a parameter, match any non-empty value
			if pathParts[i] == "" {
				return false
			}
		} else if patternPart != pathParts[i] {
			return false
		}
	}
	
	return true
}

// validatePathParams validates path parameters
func (v *Validator) validatePathParams(r *http.Request, rules map[string]PathParamRule) []ValidationError {
	var errors []ValidationError
	route := chi.RouteContext(r.Context())

	for param, rule := range rules {
		value := ""
		if route != nil {
			value = chi.URLParam(r, param)
		}

		if rule.Required && value == "" {
			errors = append(errors, ValidationError{
				Field:   param,
				Message: fmt.Sprintf("path parameter '%s' is required", param),
			})
			continue
		}

		if value != "" && rule.Validate != nil {
			if err := rule.Validate(value); err != nil {
				errors = append(errors, ValidationError{
					Field:   param,
					Message: err.Error(),
				})
			}
		}
	}

	return errors
}

// validateQueryParams validates query parameters
func (v *Validator) validateQueryParams(r *http.Request, rules map[string]QueryParamRule) []ValidationError {
	var errors []ValidationError
	query := r.URL.Query()

	for param, rule := range rules {
		value := query.Get(param)

		if rule.Required && value == "" {
			errors = append(errors, ValidationError{
				Field:   param,
				Message: fmt.Sprintf("query parameter '%s' is required", param),
			})
			continue
		}

		if value != "" && rule.Validate != nil {
			if err := rule.Validate(value); err != nil {
				errors = append(errors, ValidationError{
					Field:   param,
					Message: err.Error(),
				})
			}
		}
	}

	return errors
}

// validateBody validates JSON request body
func (v *Validator) validateBody(r *http.Request, schema *Schema) []ValidationError {
	// Skip body validation for multipart/form-data
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		return nil
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return []ValidationError{{
			Field:   "body",
			Message: fmt.Sprintf("failed to read request body: %v", err),
		}}
	}

	// Restore body for handler
	r.Body = io.NopCloser(strings.NewReader(string(body)))

	if schema.BodyRequired && len(body) == 0 {
		return []ValidationError{{
			Field:   "body",
			Message: "request body is required",
		}}
	}

	if len(body) == 0 {
		return nil
	}

	// Parse JSON
	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return []ValidationError{{
			Field:   "body",
			Message: fmt.Sprintf("invalid JSON: %v", err),
		}}
	}

	// Apply custom validation schema if provided
	if schema.BodySchema != nil {
		return schema.BodySchema(data)
	}

	return nil
}

// validateFileUpload validates multipart file uploads
func (v *Validator) validateFileUpload(r *http.Request, rule *FileUploadRule) []ValidationError {
	var errors []ValidationError

	// Parse multipart form if not already parsed
	if r.MultipartForm == nil {
		// We need to parse it, but we can't modify MaxUploadBytes here
		// The server should have already parsed it, but if not, we'll try
		if err := r.ParseMultipartForm(rule.MaxSize); err != nil {
			return []ValidationError{{
				Field:   "file",
				Message: fmt.Sprintf("failed to parse multipart form: %v", err),
			}}
		}
	}

	fieldName := rule.FieldName
	if fieldName == "" {
		fieldName = "file" // default
	}

	files := r.MultipartForm.File[fieldName]
	if rule.Required && len(files) == 0 {
		// Also check "files" (plural) as fallback
		files = r.MultipartForm.File["files"]
		if len(files) == 0 {
			errors = append(errors, ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("file upload is required (field: %s)", fieldName),
			})
			return errors
		}
	}

	if len(files) == 0 {
		return nil // No files to validate
	}

	// Check max files
	if rule.MaxFiles > 0 && len(files) > rule.MaxFiles {
		errors = append(errors, ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("too many files: %d (max: %d)", len(files), rule.MaxFiles),
		})
	}

	// Validate each file
	for i, fileHeader := range files {
		fileErrors := v.validateSingleFile(fileHeader, rule, i)
		errors = append(errors, fileErrors...)
	}

	return errors
}

// validateSingleFile validates a single file
func (v *Validator) validateSingleFile(fileHeader *multipart.FileHeader, rule *FileUploadRule, index int) []ValidationError {
	var errors []ValidationError
	fieldPrefix := fmt.Sprintf("file[%d]", index)

	// Check file size
	if rule.MaxSize > 0 && fileHeader.Size > rule.MaxSize {
		errors = append(errors, ValidationError{
			Field:   fieldPrefix + ".size",
			Message: fmt.Sprintf("file '%s' exceeds maximum size: %d bytes (max: %d bytes)", fileHeader.Filename, fileHeader.Size, rule.MaxSize),
		})
	}

	// Check MIME type
	if len(rule.AllowedTypes) > 0 {
		mimeType := fileHeader.Header.Get("Content-Type")
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		allowed := false
		for _, allowedType := range rule.AllowedTypes {
			if mimeType == allowedType || strings.HasPrefix(mimeType, allowedType+"/") {
				allowed = true
				break
			}
		}
		if !allowed {
			errors = append(errors, ValidationError{
				Field:   fieldPrefix + ".type",
				Message: fmt.Sprintf("file '%s' has disallowed MIME type: %s", fileHeader.Filename, mimeType),
			})
		}
	}

	// Check file extension
	if len(rule.AllowedExts) > 0 {
		ext := strings.ToLower(getFileExtension(fileHeader.Filename))
		allowed := false
		for _, allowedExt := range rule.AllowedExts {
			if ext == strings.ToLower(allowedExt) {
				allowed = true
				break
			}
		}
		if !allowed {
			errors = append(errors, ValidationError{
				Field:   fieldPrefix + ".extension",
				Message: fmt.Sprintf("file '%s' has disallowed extension: %s", fileHeader.Filename, ext),
			})
		}
	}

	return errors
}

// getFileExtension extracts file extension from filename
func getFileExtension(filename string) string {
	parts := strings.Split(filename, ".")
	if len(parts) < 2 {
		return ""
	}
	return "." + parts[len(parts)-1]
}

// writeValidationError writes a validation error response
func (v *Validator) writeValidationError(w http.ResponseWriter, errors []ValidationError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)

	response := ValidationErrorResponse{
		Error:   "validation failed",
		Details: errors,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(response); err != nil {
		v.logger.Warn("failed to encode validation error", slog.Any("err", err))
	}
}

// Common validators

// ValidateNonEmpty validates that a string is not empty
func ValidateNonEmpty(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("cannot be empty")
	}
	return nil
}

// ValidateHash validates a file hash format (alphanumeric, typically SHA256)
func ValidateHash(value string) error {
	matched, _ := regexp.MatchString(`^[a-fA-F0-9]{32,64}$`, value)
	if !matched {
		return fmt.Errorf("invalid hash format (must be 32-64 hex characters)")
	}
	return nil
}

// ValidateFilename validates a filename
func ValidateFilename(value string) error {
	if len(value) == 0 {
		return fmt.Errorf("filename cannot be empty")
	}
	if len(value) > 255 {
		return fmt.Errorf("filename too long (max 255 characters)")
	}
	// Check for path traversal attempts
	if strings.Contains(value, "..") || strings.Contains(value, "/") || strings.Contains(value, "\\") {
		return fmt.Errorf("filename contains invalid characters")
	}
	return nil
}

// ValidateInt validates an integer string
func ValidateInt(value string) error {
	_, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("must be a valid integer")
	}
	return nil
}

// ValidateIntRange validates an integer within a range
func ValidateIntRange(min, max int) func(string) error {
	return func(value string) error {
		num, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("must be a valid integer")
		}
		if num < min || num > max {
			return fmt.Errorf("must be between %d and %d", min, max)
		}
		return nil
	}
}

// ValidateOneOf validates that a value is one of the allowed values
func ValidateOneOf(allowed ...string) func(string) error {
	return func(value string) error {
		for _, allowedValue := range allowed {
			if value == allowedValue {
				return nil
			}
		}
		return fmt.Errorf("must be one of: %s", strings.Join(allowed, ", "))
	}
}

