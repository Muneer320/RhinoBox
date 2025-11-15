package middleware

import (
	"fmt"
	"regexp"
	"strings"
)

// RegisterAllSchemas registers validation schemas for all API endpoints
func RegisterAllSchemas(validator *Validator, maxUploadBytes int64) {
	// Health check - no validation needed
	// validator.RegisterSchema("GET:/healthz", &Schema{})

	// POST /ingest - Unified ingestion endpoint
	validator.RegisterSchema("POST:/ingest", &Schema{
		FileUpload: &FileUploadRule{
			Required:     false, // Files are optional (can have inline JSON)
			MaxSize:      maxUploadBytes,
			MaxFiles:     100, // Reasonable limit
			FieldName:    "files",
			AllowedTypes: []string{"image", "video", "audio", "application/json", "application/pdf", "application/octet-stream"}, // Allow all types
		},
		QueryParams: map[string]QueryParamRule{
			"namespace": {
				Required: false,
				Validate: func(value string) error {
					if len(value) > 100 {
						return fmt.Errorf("namespace too long (max 100 characters)")
					}
					// Allow alphanumeric, hyphen, underscore, dot
					matched, _ := regexp.MatchString(`^[a-zA-Z0-9_.-]+$`, value)
					if !matched {
						return fmt.Errorf("namespace contains invalid characters (only alphanumeric, underscore, hyphen, and dot allowed)")
					}
					return nil
				},
			},
			"comment": {
				Required: false,
				Validate: func(value string) error {
					if len(value) > 1000 {
						return fmt.Errorf("comment too long (max 1000 characters)")
					}
					return nil
				},
			},
		},
	})

	// POST /ingest/media - Media-specific ingestion
	validator.RegisterSchema("POST:/ingest/media", &Schema{
		FileUpload: &FileUploadRule{
			Required:     true,
			MaxSize:      maxUploadBytes,
			MaxFiles:     50,
			FieldName:    "file",
			AllowedTypes: []string{"image", "video", "audio", "application/octet-stream"}, // Allow octet-stream as fallback
		},
		QueryParams: map[string]QueryParamRule{
			"category": {
				Required: false,
				Validate: func(value string) error {
					if len(value) > 100 {
						return fmt.Errorf("category too long (max 100 characters)")
					}
					return nil
				},
			},
			"comment": {
				Required: false,
				Validate: func(value string) error {
					if len(value) > 1000 {
						return fmt.Errorf("comment too long (max 1000 characters)")
					}
					return nil
				},
			},
		},
	})

	// POST /ingest/json - JSON-specific ingestion
	validator.RegisterSchema("POST:/ingest/json", &Schema{
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

			// Validate namespace
			if namespace, exists := bodyMap["namespace"]; exists {
				if namespaceStr, ok := namespace.(string); ok {
					if len(namespaceStr) > 100 {
						errors = append(errors, ValidationError{
							Field:   "namespace",
							Message: "namespace too long (max 100 characters)",
						})
					} else {
						matched, _ := regexp.MatchString(`^[a-zA-Z0-9_.-]*$`, namespaceStr)
						if !matched {
							errors = append(errors, ValidationError{
								Field:   "namespace",
								Message: "namespace contains invalid characters",
							})
						}
					}
				} else if namespace != nil {
					errors = append(errors, ValidationError{
						Field:   "namespace",
						Message: "namespace must be a string",
					})
				}
			}

			// Validate comment
			if comment, exists := bodyMap["comment"]; exists {
				if commentStr, ok := comment.(string); ok {
					if len(commentStr) > 1000 {
						errors = append(errors, ValidationError{
							Field:   "comment",
							Message: "comment too long (max 1000 characters)",
						})
					}
				} else if comment != nil {
					errors = append(errors, ValidationError{
						Field:   "comment",
						Message: "comment must be a string",
					})
				}
			}

			// Validate documents
			hasDocument := false
			if document, exists := bodyMap["document"]; exists && document != nil {
				hasDocument = true
			}
			if documents, exists := bodyMap["documents"]; exists && documents != nil {
				if docsArray, ok := documents.([]interface{}); ok {
					if len(docsArray) == 0 {
						errors = append(errors, ValidationError{
							Field:   "documents",
							Message: "documents array cannot be empty",
						})
					} else if len(docsArray) > 10000 {
						errors = append(errors, ValidationError{
							Field:   "documents",
							Message: "too many documents (max 10000)",
						})
					}
					hasDocument = true
				} else {
					errors = append(errors, ValidationError{
						Field:   "documents",
						Message: "documents must be an array",
					})
				}
			}

			if !hasDocument {
				errors = append(errors, ValidationError{
					Field:   "document",
					Message: "either 'document' or 'documents' field is required",
				})
			}

			// Validate metadata (if present)
			if metadata, exists := bodyMap["metadata"]; exists && metadata != nil {
				if metadataMap, ok := metadata.(map[string]interface{}); ok {
					if len(metadataMap) > 100 {
						errors = append(errors, ValidationError{
							Field:   "metadata",
							Message: "too many metadata fields (max 100)",
						})
					}
				} else {
					errors = append(errors, ValidationError{
						Field:   "metadata",
						Message: "metadata must be an object",
					})
				}
			}

			return errors
		},
	})

	// PATCH /files/rename - Rename file
	validator.RegisterSchema("PATCH:/files/rename", &Schema{
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

			// Validate hash
			hash, exists := bodyMap["hash"]
			if !exists || hash == nil {
				errors = append(errors, ValidationError{
					Field:   "hash",
					Message: "hash is required",
				})
			} else if hashStr, ok := hash.(string); ok {
				if err := ValidateHash(hashStr); err != nil {
					errors = append(errors, ValidationError{
						Field:   "hash",
						Message: err.Error(),
					})
				}
			} else {
				errors = append(errors, ValidationError{
					Field:   "hash",
					Message: "hash must be a string",
				})
			}

			// Validate new_name
			newName, exists := bodyMap["new_name"]
			if !exists || newName == nil {
				errors = append(errors, ValidationError{
					Field:   "new_name",
					Message: "new_name is required",
				})
			} else if newNameStr, ok := newName.(string); ok {
				if err := ValidateFilename(newNameStr); err != nil {
					errors = append(errors, ValidationError{
						Field:   "new_name",
						Message: err.Error(),
					})
				}
			} else {
				errors = append(errors, ValidationError{
					Field:   "new_name",
					Message: "new_name must be a string",
				})
			}

			// Validate update_stored_file (optional boolean)
			if updateStoredFile, exists := bodyMap["update_stored_file"]; exists && updateStoredFile != nil {
				if _, ok := updateStoredFile.(bool); !ok {
					errors = append(errors, ValidationError{
						Field:   "update_stored_file",
						Message: "update_stored_file must be a boolean",
					})
				}
			}

			return errors
		},
	})

	// DELETE /files/{file_id} - Delete file
	validator.RegisterSchema("DELETE:/files/{file_id}", &Schema{
		PathParams: map[string]PathParamRule{
			"file_id": {
				Required: true,
				Validate: ValidateHash,
			},
		},
	})

	// PATCH /files/{file_id}/metadata - Update file metadata
	validator.RegisterSchema("PATCH:/files/{file_id}/metadata", &Schema{
		PathParams: map[string]PathParamRule{
			"file_id": {
				Required: true,
				Validate: ValidateHash,
			},
		},
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

			// Validate action
			action, exists := bodyMap["action"]
			if exists && action != nil {
				if actionStr, ok := action.(string); ok {
					allowedActions := []string{"replace", "merge", "remove"}
					valid := false
					for _, allowed := range allowedActions {
						if actionStr == allowed {
							valid = true
							break
						}
					}
					if !valid && actionStr != "" {
						errors = append(errors, ValidationError{
							Field:   "action",
							Message: fmt.Sprintf("action must be one of: %s (or empty for merge)", strings.Join(allowedActions, ", ")),
						})
					}
				} else {
					errors = append(errors, ValidationError{
						Field:   "action",
						Message: "action must be a string",
					})
				}
			}

			// Validate metadata (for replace/merge)
			actionStr := ""
			if action != nil {
				if a, ok := action.(string); ok {
					actionStr = a
				}
			}

			if actionStr == "" || actionStr == "replace" || actionStr == "merge" {
				metadata, exists := bodyMap["metadata"]
				if !exists || metadata == nil {
					errors = append(errors, ValidationError{
						Field:   "metadata",
						Message: "metadata is required for replace/merge action",
					})
				} else if metadataMap, ok := metadata.(map[string]interface{}); ok {
					if len(metadataMap) > 100 {
						errors = append(errors, ValidationError{
							Field:   "metadata",
							Message: "too many metadata fields (max 100)",
						})
					}
					// Validate metadata keys and values
					for key, value := range metadataMap {
						if len(key) == 0 {
							errors = append(errors, ValidationError{
								Field:   "metadata",
								Message: "metadata key cannot be empty",
							})
						} else if len(key) > 256 {
							errors = append(errors, ValidationError{
								Field:   "metadata",
								Message: fmt.Sprintf("metadata key '%s' too long (max 256 characters)", key),
							})
						}

						// Check for protected fields
						protectedFields := map[string]bool{
							"hash": true, "original_name": true, "stored_path": true,
							"mime_type": true, "size": true, "uploaded_at": true, "category": true,
						}
						if protectedFields[strings.ToLower(key)] {
							errors = append(errors, ValidationError{
								Field:   "metadata",
								Message: fmt.Sprintf("cannot modify protected field: %s", key),
							})
						}

						// Validate value
						if valueStr, ok := value.(string); ok {
							if len(valueStr) > 32*1024 {
								errors = append(errors, ValidationError{
									Field:   "metadata",
									Message: fmt.Sprintf("metadata value for '%s' too large (max 32KB)", key),
								})
							}
						}
					}
				} else {
					errors = append(errors, ValidationError{
						Field:   "metadata",
						Message: "metadata must be an object",
					})
				}
			}

			// Validate fields (for remove)
			if actionStr == "remove" {
				fields, exists := bodyMap["fields"]
				if !exists || fields == nil {
					errors = append(errors, ValidationError{
						Field:   "fields",
						Message: "fields is required for remove action",
					})
				} else if fieldsArray, ok := fields.([]interface{}); ok {
					if len(fieldsArray) == 0 {
						errors = append(errors, ValidationError{
							Field:   "fields",
							Message: "fields array cannot be empty",
						})
					}
					if len(fieldsArray) > 100 {
						errors = append(errors, ValidationError{
							Field:   "fields",
							Message: "too many fields to remove (max 100)",
						})
					}
					// Validate each field name
					for i, field := range fieldsArray {
						if fieldStr, ok := field.(string); ok {
							if len(fieldStr) == 0 {
								errors = append(errors, ValidationError{
									Field:   fmt.Sprintf("fields[%d]", i),
									Message: "field name cannot be empty",
								})
							}
							// Check for protected fields
							protectedFields := map[string]bool{
								"hash": true, "original_name": true, "stored_path": true,
								"mime_type": true, "size": true, "uploaded_at": true, "category": true,
							}
							if protectedFields[strings.ToLower(fieldStr)] {
								errors = append(errors, ValidationError{
									Field:   fmt.Sprintf("fields[%d]", i),
									Message: fmt.Sprintf("cannot remove protected field: %s", fieldStr),
								})
							}
						} else {
							errors = append(errors, ValidationError{
								Field:   fmt.Sprintf("fields[%d]", i),
								Message: "field name must be a string",
							})
						}
					}
				} else {
					errors = append(errors, ValidationError{
						Field:   "fields",
						Message: "fields must be an array",
					})
				}
			}

			return errors
		},
	})

	// POST /files/metadata/batch - Batch update metadata
	validator.RegisterSchema("POST:/files/metadata/batch", &Schema{
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

			// Validate updates array
			updates, exists := bodyMap["updates"]
			if !exists || updates == nil {
				errors = append(errors, ValidationError{
					Field:   "updates",
					Message: "updates array is required",
				})
				return errors
			}

			updatesArray, ok := updates.([]interface{})
			if !ok {
				errors = append(errors, ValidationError{
					Field:   "updates",
					Message: "updates must be an array",
				})
				return errors
			}

			if len(updatesArray) == 0 {
				errors = append(errors, ValidationError{
					Field:   "updates",
					Message: "updates array cannot be empty",
				})
			}

			if len(updatesArray) > 100 {
				errors = append(errors, ValidationError{
					Field:   "updates",
					Message: "too many updates (max 100)",
				})
			}

			// Validate each update (simplified - reuse metadata validation logic)
			for i, update := range updatesArray {
				if updateMap, ok := update.(map[string]interface{}); ok {
					// Validate hash
					if hash, exists := updateMap["hash"]; exists {
						if hashStr, ok := hash.(string); ok {
							if err := ValidateHash(hashStr); err != nil {
								errors = append(errors, ValidationError{
									Field:   fmt.Sprintf("updates[%d].hash", i),
									Message: err.Error(),
								})
							}
						} else {
							errors = append(errors, ValidationError{
								Field:   fmt.Sprintf("updates[%d].hash", i),
								Message: "hash must be a string",
							})
						}
					} else {
						errors = append(errors, ValidationError{
							Field:   fmt.Sprintf("updates[%d].hash", i),
							Message: "hash is required",
						})
					}
				} else {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("updates[%d]", i),
						Message: "update must be an object",
					})
				}
			}

			return errors
		},
	})

	// GET /files/search - Search files
	validator.RegisterSchema("GET:/files/search", &Schema{
		QueryParams: map[string]QueryParamRule{
			"name": {
				Required: true,
				Validate: func(value string) error {
					if len(value) == 0 {
						return fmt.Errorf("name query parameter cannot be empty")
					}
					if len(value) > 255 {
						return fmt.Errorf("name query parameter too long (max 255 characters)")
					}
					return nil
				},
			},
		},
	})

	// GET /files/download - Download file
	validator.RegisterSchema("GET:/files/download", &Schema{
		QueryParams: map[string]QueryParamRule{
			"hash": {
				Required: false,
				Validate: ValidateHash,
			},
			"path": {
				Required: false,
				Validate: func(value string) error {
					if len(value) > 1000 {
						return fmt.Errorf("path too long (max 1000 characters)")
					}
					// Basic path validation
					if strings.Contains(value, "..") {
						return fmt.Errorf("path contains invalid characters")
					}
					return nil
				},
			},
		},
		BodySchema: func(data interface{}) []ValidationError {
			// This validation is done in the Validate middleware
			// by checking query params after they're validated
			return nil
		},
	})

	// GET /files/metadata - Get file metadata
	validator.RegisterSchema("GET:/files/metadata", &Schema{
		QueryParams: map[string]QueryParamRule{
			"hash": {
				Required: true,
				Validate: ValidateHash,
			},
		},
	})

	// GET /files/stream - Stream file
	validator.RegisterSchema("GET:/files/stream", &Schema{
		QueryParams: map[string]QueryParamRule{
			"hash": {
				Required: false,
				Validate: ValidateHash,
			},
			"path": {
				Required: false,
				Validate: func(value string) error {
					if len(value) > 1000 {
						return fmt.Errorf("path too long (max 1000 characters)")
					}
					if strings.Contains(value, "..") {
						return fmt.Errorf("path contains invalid characters")
					}
					return nil
				},
			},
		},
	})
}

