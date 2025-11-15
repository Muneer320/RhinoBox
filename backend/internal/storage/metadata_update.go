package storage

import (
	"errors"
	"fmt"
	"strings"
)

// Errors for metadata operations
var (
	ErrMetadataTooLarge     = errors.New("metadata exceeds size limit")
	ErrInvalidMetadataKey   = errors.New("invalid metadata key")
	ErrProtectedField       = errors.New("cannot modify protected metadata field")
	ErrMetadataNotFound     = errors.New("metadata not found")
)

// Constants for metadata validation
const (
	MaxMetadataSize      = 64 * 1024  // 64KB total metadata size
	MaxMetadataKeyLength = 256
	MaxMetadataValueSize = 32 * 1024  // 32KB per value
	MaxMetadataFields    = 100
)

// Protected system metadata fields that cannot be modified
var protectedFields = map[string]bool{
	"hash":          true,
	"original_name": true,
	"stored_path":   true,
	"mime_type":     true,
	"size":          true,
	"uploaded_at":   true,
	"category":      true,
}

// MetadataUpdateRequest defines the metadata update operation
type MetadataUpdateRequest struct {
	Hash     string            `json:"hash"`
	Action   string            `json:"action"`   // "replace", "merge", "remove"
	Metadata map[string]string `json:"metadata"` // For replace/merge
	Fields   []string          `json:"fields"`   // For remove
}

// MetadataUpdateResult contains the result of a metadata update
type MetadataUpdateResult struct {
	Hash        string            `json:"hash"`
	OldMetadata map[string]string `json:"old_metadata"`
	NewMetadata map[string]string `json:"new_metadata"`
	Action      string            `json:"action"`
	UpdatedAt   string            `json:"updated_at"`
}

// ValidateMetadataUpdate validates a metadata update request
func ValidateMetadataUpdate(req MetadataUpdateRequest) error {
	if req.Hash == "" {
		return errors.New("hash is required")
	}

	// Validate action
	switch req.Action {
	case "", "replace", "merge":
		if req.Metadata == nil {
			return errors.New("metadata is required for replace/merge action")
		}
		return validateMetadata(req.Metadata)
	case "remove":
		if len(req.Fields) == 0 {
			return errors.New("fields is required for remove action")
		}
		return validateRemoveFields(req.Fields)
	default:
		return fmt.Errorf("invalid action: %s (must be 'replace', 'merge', or 'remove')", req.Action)
	}
}

// validateMetadata checks metadata constraints
func validateMetadata(metadata map[string]string) error {
	if len(metadata) > MaxMetadataFields {
		return fmt.Errorf("too many metadata fields: %d (max %d)", len(metadata), MaxMetadataFields)
	}

	totalSize := 0
	for key, value := range metadata {
		// Check for protected fields
		if protectedFields[strings.ToLower(key)] {
			return fmt.Errorf("%w: %s", ErrProtectedField, key)
		}

		// Validate key length
		if len(key) == 0 {
			return errors.New("metadata key cannot be empty")
		}
		if len(key) > MaxMetadataKeyLength {
			return fmt.Errorf("metadata key too long: %s (%d bytes, max %d)", key, len(key), MaxMetadataKeyLength)
		}

		// Validate key characters (alphanumeric, underscore, hyphen, dot)
		if !isValidMetadataKey(key) {
			return fmt.Errorf("%w: %s (only alphanumeric, underscore, hyphen, and dot allowed)", ErrInvalidMetadataKey, key)
		}

		// Validate value size
		valueSize := len(value)
		if valueSize > MaxMetadataValueSize {
			return fmt.Errorf("metadata value too large for key '%s': %d bytes (max %d)", key, valueSize, MaxMetadataValueSize)
		}

		totalSize += len(key) + valueSize
	}

	// Check total size
	if totalSize > MaxMetadataSize {
		return fmt.Errorf("%w: %d bytes (max %d)", ErrMetadataTooLarge, totalSize, MaxMetadataSize)
	}

	return nil
}

// validateRemoveFields validates fields to be removed
func validateRemoveFields(fields []string) error {
	if len(fields) == 0 {
		return errors.New("at least one field must be specified for removal")
	}

	for _, field := range fields {
		if field == "" {
			return errors.New("field name cannot be empty")
		}
		if protectedFields[strings.ToLower(field)] {
			return fmt.Errorf("%w: %s", ErrProtectedField, field)
		}
	}

	return nil
}

// isValidMetadataKey checks if a key contains only allowed characters
func isValidMetadataKey(key string) bool {
	for _, char := range key {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '_' || char == '-' || char == '.') {
			return false
		}
	}
	return true
}

// UpdateMetadata updates the metadata for a file by hash
func (idx *MetadataIndex) UpdateMetadata(hash string, action string, newMetadata map[string]string, removeFields []string) (*FileMetadata, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Find existing metadata
	existing, ok := idx.data[hash]
	if !ok {
		return nil, fmt.Errorf("%w: hash=%s", ErrMetadataNotFound, hash)
	}

	// Create a copy of the metadata
	var updatedMetadata map[string]string

	switch action {
	case "replace":
		// Replace all metadata
		updatedMetadata = make(map[string]string, len(newMetadata))
		for k, v := range newMetadata {
			updatedMetadata[k] = v
		}

	case "", "merge":
		// Merge: keep existing fields and add/update new ones
		updatedMetadata = make(map[string]string)
		if existing.Metadata != nil {
			for k, v := range existing.Metadata {
				updatedMetadata[k] = v
			}
		}
		for k, v := range newMetadata {
			updatedMetadata[k] = v
		}

	case "remove":
		// Remove specified fields
		updatedMetadata = make(map[string]string)
		if existing.Metadata != nil {
			for k, v := range existing.Metadata {
				updatedMetadata[k] = v
			}
		}
		for _, field := range removeFields {
			delete(updatedMetadata, field)
		}

	default:
		return nil, fmt.Errorf("invalid action: %s", action)
	}

	// Update the metadata
	existing.Metadata = updatedMetadata
	idx.data[hash] = existing

	// Persist changes
	if err := idx.persistLocked(); err != nil {
		return nil, err
	}

	clone := existing
	return &clone, nil
}

// BatchUpdateMetadata updates metadata for multiple files
func (idx *MetadataIndex) BatchUpdateMetadata(updates []MetadataUpdateRequest) ([]MetadataUpdateResult, []error) {
	results := make([]MetadataUpdateResult, len(updates))
	errs := make([]error, len(updates))

	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Track if any updates succeeded
	anySuccess := false

	for i, req := range updates {
		// Validate request
		if err := ValidateMetadataUpdate(req); err != nil {
			errs[i] = err
			continue
		}

		// Find existing metadata
		existing, ok := idx.data[req.Hash]
		if !ok {
			errs[i] = fmt.Errorf("%w: hash=%s", ErrMetadataNotFound, req.Hash)
			continue
		}

		// Store old metadata for result
		oldMetadata := make(map[string]string)
		if existing.Metadata != nil {
			for k, v := range existing.Metadata {
				oldMetadata[k] = v
			}
		}

		// Create updated metadata
		var updatedMetadata map[string]string

		switch req.Action {
		case "replace":
			updatedMetadata = make(map[string]string, len(req.Metadata))
			for k, v := range req.Metadata {
				updatedMetadata[k] = v
			}

		case "", "merge":
			updatedMetadata = make(map[string]string)
			if existing.Metadata != nil {
				for k, v := range existing.Metadata {
					updatedMetadata[k] = v
				}
			}
			for k, v := range req.Metadata {
				updatedMetadata[k] = v
			}

		case "remove":
			updatedMetadata = make(map[string]string)
			if existing.Metadata != nil {
				for k, v := range existing.Metadata {
					updatedMetadata[k] = v
				}
			}
			for _, field := range req.Fields {
				delete(updatedMetadata, field)
			}
		}

		// Update the metadata
		existing.Metadata = updatedMetadata
		idx.data[req.Hash] = existing
		anySuccess = true

		// Build result
		results[i] = MetadataUpdateResult{
			Hash:        req.Hash,
			OldMetadata: oldMetadata,
			NewMetadata: updatedMetadata,
			Action:      req.Action,
		}
	}

	// Persist changes if any updates succeeded
	if anySuccess {
		if err := idx.persistLocked(); err != nil {
			// If persist fails, mark all as errors
			for i := range errs {
				if errs[i] == nil {
					errs[i] = fmt.Errorf("persist failed: %w", err)
				}
			}
		}
	}

	return results, errs
}

// UpdateFileMetadata updates metadata for a file
func (m *Manager) UpdateFileMetadata(req MetadataUpdateRequest) (*MetadataUpdateResult, error) {
	// Validate request
	if err := ValidateMetadataUpdate(req); err != nil {
		return nil, err
	}

	// Default action to "merge" if not specified
	action := req.Action
	if action == "" {
		action = "merge"
	}

	// Update metadata in index
	updated, err := m.index.UpdateMetadata(req.Hash, action, req.Metadata, req.Fields)
	if err != nil {
		return nil, err
	}

	// Build result
	oldMetadata := make(map[string]string)
	if req.Action == "remove" {
		// For remove, reconstruct old metadata by adding back removed fields
		if updated.Metadata != nil {
			for k, v := range updated.Metadata {
				oldMetadata[k] = v
			}
		}
		for _, field := range req.Fields {
			oldMetadata[field] = ""
		}
	}

	result := &MetadataUpdateResult{
		Hash:        req.Hash,
		OldMetadata: oldMetadata,
		NewMetadata: updated.Metadata,
		Action:      action,
	}

	return result, nil
}

// BatchUpdateFileMetadata updates metadata for multiple files
func (m *Manager) BatchUpdateFileMetadata(updates []MetadataUpdateRequest) ([]MetadataUpdateResult, []error) {
	return m.index.BatchUpdateMetadata(updates)
}
