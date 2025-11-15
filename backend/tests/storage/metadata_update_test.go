package storage

import (
"encoding/json"
"os"
"path/filepath"
"strings"
"testing"

"github.com/Muneer320/RhinoBox/internal/storage"
)

func TestValidateMetadataUpdate(t *testing.T) {
	tests := []struct {
		name    string
		req     storage.MetadataUpdateRequest
		wantErr bool
		errType error
	}{
		{
			name: "valid merge request",
			req: storage.MetadataUpdateRequest{
				Hash:     "abc123",
				Action:   "merge",
				Metadata: map[string]string{"key": "value"},
			},
			wantErr: false,
		},
		{
			name: "valid replace request",
			req: storage.MetadataUpdateRequest{
				Hash:     "abc123",
				Action:   "replace",
				Metadata: map[string]string{"key": "value"},
			},
			wantErr: false,
		},
		{
			name: "valid remove request",
			req: storage.MetadataUpdateRequest{
				Hash:   "abc123",
				Action: "remove",
				Fields: []string{"field1", "field2"},
			},
			wantErr: false,
		},
		{
			name: "missing hash",
			req: storage.MetadataUpdateRequest{
				Action:   "merge",
				Metadata: map[string]string{"key": "value"},
			},
			wantErr: true,
		},
		{
			name: "invalid action",
			req: storage.MetadataUpdateRequest{
				Hash:     "abc123",
				Action:   "invalid",
				Metadata: map[string]string{"key": "value"},
			},
			wantErr: true,
		},
		{
			name: "protected field - hash",
			req: storage.MetadataUpdateRequest{
				Hash:     "abc123",
				Action:   "merge",
				Metadata: map[string]string{"hash": "value"},
			},
			wantErr: true,
			errType: storage.ErrProtectedField,
		},
		{
			name: "protected field - size",
			req: storage.MetadataUpdateRequest{
				Hash:     "abc123",
				Action:   "merge",
				Metadata: map[string]string{"size": "value"},
			},
			wantErr: true,
			errType: storage.ErrProtectedField,
		},
		{
			name: "too many fields",
			req: storage.MetadataUpdateRequest{
				Hash:     "abc123",
				Action:   "merge",
				Metadata: generateLargeMetadata(101),
			},
			wantErr: true,
		},
		{
			name: "metadata value too large",
			req: storage.MetadataUpdateRequest{
				Hash:   "abc123",
				Action: "merge",
				Metadata: map[string]string{
					"key": strings.Repeat("a", 33*1024),
				},
			},
			wantErr: true,
		},
		{
			name: "invalid key characters",
			req: storage.MetadataUpdateRequest{
				Hash:   "abc123",
				Action: "merge",
				Metadata: map[string]string{
					"key with spaces": "value",
				},
			},
			wantErr: true,
			errType: storage.ErrInvalidMetadataKey,
		},
		{
			name: "valid key with special chars",
			req: storage.MetadataUpdateRequest{
				Hash:   "abc123",
				Action: "merge",
				Metadata: map[string]string{
					"key_with-dash.dot": "value",
				},
			},
			wantErr: false,
		},
		{
			name: "remove protected field",
			req: storage.MetadataUpdateRequest{
				Hash:   "abc123",
				Action: "remove",
				Fields: []string{"hash"},
			},
			wantErr: true,
			errType: storage.ErrProtectedField,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := storage.ValidateMetadataUpdate(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMetadataUpdate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.errType != nil && err != nil && !strings.Contains(err.Error(), tt.errType.Error()) {
				t.Errorf("ValidateMetadataUpdate() error = %v, want error type %v", err, tt.errType)
			}
		})
	}
}

func TestMetadataUpdateMerge(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Store a test file with initial metadata
	content := strings.NewReader("test content")
	result, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   content,
		Filename: "test.txt",
		MimeType: "text/plain",
		Size:     12,
		Metadata: map[string]string{
			"comment": "initial comment",
			"tag1":    "value1",
		},
	})
	if err != nil {
		t.Fatalf("StoreFile() error = %v", err)
	}

	// Test merge operation
	req := storage.MetadataUpdateRequest{
		Hash:   result.Metadata.Hash,
		Action: "merge",
		Metadata: map[string]string{
			"tag2":    "value2",
			"comment": "updated comment",
		},
	}

	updateResult, err := mgr.UpdateFileMetadata(req)
	if err != nil {
		t.Fatalf("UpdateFileMetadata() error = %v", err)
	}

	// Verify old and new metadata
	if updateResult.Hash != result.Metadata.Hash {
		t.Errorf("Hash mismatch: got %v, want %v", updateResult.Hash, result.Metadata.Hash)
	}

	// Check that merge kept old fields and added new ones
	if len(updateResult.NewMetadata) != 3 {
		t.Errorf("NewMetadata length = %v, want 3", len(updateResult.NewMetadata))
	}

	if updateResult.NewMetadata["comment"] != "updated comment" {
		t.Errorf("comment = %v, want 'updated comment'", updateResult.NewMetadata["comment"])
	}

	if updateResult.NewMetadata["tag1"] != "value1" {
		t.Errorf("tag1 = %v, want 'value1'", updateResult.NewMetadata["tag1"])
	}

	if updateResult.NewMetadata["tag2"] != "value2" {
		t.Errorf("tag2 = %v, want 'value2'", updateResult.NewMetadata["tag2"])
	}
}

func TestMetadataUpdateReplace(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Store a test file with initial metadata
	content := strings.NewReader("test content")
	result, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   content,
		Filename: "test.txt",
		MimeType: "text/plain",
		Size:     12,
		Metadata: map[string]string{
			"comment": "initial comment",
			"tag1":    "value1",
		},
	})
	if err != nil {
		t.Fatalf("StoreFile() error = %v", err)
	}

	// Test replace operation
	req := storage.MetadataUpdateRequest{
		Hash:   result.Metadata.Hash,
		Action: "replace",
		Metadata: map[string]string{
			"tag2": "value2",
		},
	}

	updateResult, err := mgr.UpdateFileMetadata(req)
	if err != nil {
		t.Fatalf("UpdateFileMetadata() error = %v", err)
	}

	// Check that replace removed old fields
	if len(updateResult.NewMetadata) != 1 {
		t.Errorf("NewMetadata length = %v, want 1", len(updateResult.NewMetadata))
	}

	if updateResult.NewMetadata["tag2"] != "value2" {
		t.Errorf("tag2 = %v, want 'value2'", updateResult.NewMetadata["tag2"])
	}

	// Old field should not exist
	if _, exists := updateResult.NewMetadata["comment"]; exists {
		t.Errorf("comment should not exist after replace")
	}

	if _, exists := updateResult.NewMetadata["tag1"]; exists {
		t.Errorf("tag1 should not exist after replace")
	}
}

func TestMetadataUpdateRemove(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Store a test file with initial metadata
	content := strings.NewReader("test content")
	result, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   content,
		Filename: "test.txt",
		MimeType: "text/plain",
		Size:     12,
		Metadata: map[string]string{
			"comment": "initial comment",
			"tag1":    "value1",
			"tag2":    "value2",
		},
	})
	if err != nil {
		t.Fatalf("StoreFile() error = %v", err)
	}

	// Test remove operation
	req := storage.MetadataUpdateRequest{
		Hash:   result.Metadata.Hash,
		Action: "remove",
		Fields: []string{"comment", "tag1"},
	}

	updateResult, err := mgr.UpdateFileMetadata(req)
	if err != nil {
		t.Fatalf("UpdateFileMetadata() error = %v", err)
	}

	// Check that remove deleted specified fields
	if len(updateResult.NewMetadata) != 1 {
		t.Errorf("NewMetadata length = %v, want 1", len(updateResult.NewMetadata))
	}

	if updateResult.NewMetadata["tag2"] != "value2" {
		t.Errorf("tag2 = %v, want 'value2'", updateResult.NewMetadata["tag2"])
	}

	// Removed fields should not exist
	if _, exists := updateResult.NewMetadata["comment"]; exists {
		t.Errorf("comment should be removed")
	}

	if _, exists := updateResult.NewMetadata["tag1"]; exists {
		t.Errorf("tag1 should be removed")
	}
}

func TestMetadataUpdatePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Store a test file
	content := strings.NewReader("test content")
	result, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   content,
		Filename: "test.txt",
		MimeType: "text/plain",
		Size:     12,
		Metadata: map[string]string{
			"initial": "value",
		},
	})
	if err != nil {
		t.Fatalf("StoreFile() error = %v", err)
	}

	// Update metadata
	req := storage.MetadataUpdateRequest{
		Hash:   result.Metadata.Hash,
		Action: "merge",
		Metadata: map[string]string{
			"updated": "newvalue",
		},
	}

	_, err = mgr.UpdateFileMetadata(req)
	if err != nil {
		t.Fatalf("UpdateFileMetadata() error = %v", err)
	}

	// Retrieve and verify metadata persisted
	metadataPath := filepath.Join(tmpDir, "metadata", "files.json")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Fatalf("metadata file does not exist")
	}

	// Read the metadata file directly to verify persistence
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("failed to read metadata file: %v", err)
	}

	var items []storage.FileMetadata
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}

	// Find our file in the metadata
	var found *storage.FileMetadata
	for _, item := range items {
		if item.Hash == result.Metadata.Hash {
			found = &item
			break
		}
	}

	if found == nil {
		t.Fatalf("metadata not found after reload")
	}

	if found.Metadata["initial"] != "value" {
		t.Errorf("initial metadata not persisted correctly")
	}

	if found.Metadata["updated"] != "newvalue" {
		t.Errorf("updated metadata not persisted correctly")
	}
}

func TestMetadataUpdateNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	req := storage.MetadataUpdateRequest{
		Hash:     "nonexistent",
		Action:   "merge",
		Metadata: map[string]string{"key": "value"},
	}

	_, err = mgr.UpdateFileMetadata(req)
	if err == nil {
		t.Fatalf("UpdateFileMetadata() should return error for nonexistent hash")
	}

	if !strings.Contains(err.Error(), "metadata not found") {
		t.Errorf("error should contain 'metadata not found', got: %v", err)
	}
}

func TestBatchMetadataUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Store multiple test files
	hashes := make([]string, 3)
	for i := 0; i < 3; i++ {
		content := strings.NewReader("test content")
		result, err := mgr.StoreFile(storage.StoreRequest{
			Reader:   content,
			Filename: "test.txt",
			MimeType: "text/plain",
			Size:     12,
			Metadata: map[string]string{
				"initial": "value",
			},
		})
		if err != nil {
			t.Fatalf("StoreFile() error = %v", err)
		}
		hashes[i] = result.Metadata.Hash
	}

	// Batch update with mix of success and failure
	updates := []storage.MetadataUpdateRequest{
		{
			Hash:   hashes[0],
			Action: "merge",
			Metadata: map[string]string{
				"batch": "update1",
			},
		},
		{
			Hash:   hashes[1],
			Action: "replace",
			Metadata: map[string]string{
				"batch": "update2",
			},
		},
		{
			Hash:   "nonexistent",
			Action: "merge",
			Metadata: map[string]string{
				"batch": "update3",
			},
		},
	}

	results, errs := mgr.BatchUpdateFileMetadata(updates)

	// Check results
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	if len(errs) != 3 {
		t.Fatalf("expected 3 error entries, got %d", len(errs))
	}

	// First two should succeed
	if errs[0] != nil {
		t.Errorf("first update should succeed, got error: %v", errs[0])
	}

	if errs[1] != nil {
		t.Errorf("second update should succeed, got error: %v", errs[1])
	}

	// Third should fail
	if errs[2] == nil {
		t.Errorf("third update should fail for nonexistent hash")
	}

	// Verify first update (merge)
	if len(results[0].NewMetadata) != 2 {
		t.Errorf("first result should have 2 metadata fields, got %d", len(results[0].NewMetadata))
	}

	// Verify second update (replace)
	if len(results[1].NewMetadata) != 1 {
		t.Errorf("second result should have 1 metadata field, got %d", len(results[1].NewMetadata))
	}
}

func TestMetadataSizeLimits(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Store a test file
	content := strings.NewReader("test content")
	result, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   content,
		Filename: "test.txt",
		MimeType: "text/plain",
		Size:     12,
	})
	if err != nil {
		t.Fatalf("StoreFile() error = %v", err)
	}

	// Test metadata value size limit
	req := storage.MetadataUpdateRequest{
		Hash:   result.Metadata.Hash,
		Action: "merge",
		Metadata: map[string]string{
			"large": strings.Repeat("a", 33*1024), // 33KB > 32KB limit
		},
	}

	_, err = mgr.UpdateFileMetadata(req)
	if err == nil {
		t.Fatalf("UpdateFileMetadata() should fail for oversized value")
	}

	// Test total metadata size limit
	largeMeta := make(map[string]string)
	for i := 0; i < 10; i++ {
		largeMeta[string(rune('a'+i))] = strings.Repeat("x", 10*1024) // 10KB each
	}

	req = storage.MetadataUpdateRequest{
		Hash:     result.Metadata.Hash,
		Action:   "merge",
		Metadata: largeMeta,
	}

	_, err = mgr.UpdateFileMetadata(req)
	if err == nil {
		t.Fatalf("UpdateFileMetadata() should fail for total size exceeding limit")
	}
}

func TestConcurrentMetadataUpdates(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Store a test file
	content := strings.NewReader("test content")
	result, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   content,
		Filename: "test.txt",
		MimeType: "text/plain",
		Size:     12,
	})
	if err != nil {
		t.Fatalf("StoreFile() error = %v", err)
	}

	// Perform concurrent updates
	const numUpdates = 10
	done := make(chan bool, numUpdates)
	errs := make([]error, numUpdates)

	for i := 0; i < numUpdates; i++ {
		go func(idx int) {
			req := storage.MetadataUpdateRequest{
				Hash:   result.Metadata.Hash,
				Action: "merge",
				Metadata: map[string]string{
					string(rune('a' + idx)): "value",
				},
			}
			_, err := mgr.UpdateFileMetadata(req)
			errs[idx] = err
			done <- true
		}(i)
	}

	// Wait for all updates
	for i := 0; i < numUpdates; i++ {
		<-done
	}

	// Check no errors occurred
	for i, err := range errs {
		if err != nil {
			t.Errorf("concurrent update %d failed: %v", i, err)
		}
	}

	// Verify all updates persisted
	metadataPath := filepath.Join(tmpDir, "metadata", "files.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("failed to read metadata file: %v", err)
	}

	var items []storage.FileMetadata
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}

	// Find our file
	var stored *storage.FileMetadata
	for _, item := range items {
		if item.Hash == result.Metadata.Hash {
			stored = &item
			break
		}
	}

	if stored == nil {
		t.Fatalf("metadata not found")
	}

	if len(stored.Metadata) != numUpdates {
		t.Errorf("expected %d metadata fields, got %d", numUpdates, len(stored.Metadata))
	}
}

// Helper function to generate large metadata
func generateLargeMetadata(count int) map[string]string {
	metadata := make(map[string]string, count)
	for i := 0; i < count; i++ {
		metadata[string(rune('a'+i%26))+string(rune('0'+i/26))] = "value"
	}
	return metadata
}
