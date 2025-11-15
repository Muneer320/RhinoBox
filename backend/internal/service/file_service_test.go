package service

import (
	"bytes"
	"errors"
	"log/slog"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Muneer320/RhinoBox/internal/storage"
)

// setupTestStorage creates a temporary storage manager for testing.
func setupTestStorage(t *testing.T) (*storage.Manager, string) {
	tmpDir, err := os.MkdirTemp("", "rhinobox-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	store, err := storage.NewManager(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create storage manager: %v", err)
	}

	return store, tmpDir
}

// cleanupTestStorage removes temporary test storage.
func cleanupTestStorage(tmpDir string) {
	os.RemoveAll(tmpDir)
}

func TestNewFileService(t *testing.T) {
	store, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	service := NewFileService(store, logger)

	if service == nil {
		t.Fatal("NewFileService returned nil")
	}
	if service.storage != store {
		t.Error("storage not set correctly")
	}
	if service.logger != logger {
		t.Error("logger not set correctly")
	}
}

func TestFileService_StoreFile(t *testing.T) {
	store, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	service := NewFileService(store, logger)

	tests := []struct {
		name        string
		req         FileStoreRequest
		expectError bool
	}{
		{
			name: "valid file",
			req: FileStoreRequest{
				Reader:       bytes.NewReader([]byte("test content")),
				Filename:     "test.txt",
				MimeType:     "text/plain",
				Size:         12,
				Metadata:     map[string]string{"comment": "test"},
				CategoryHint: "",
			},
			expectError: false,
		},
		{
			name: "invalid reader",
			req: FileStoreRequest{
				Reader:   nil,
				Filename: "test.txt",
				MimeType: "text/plain",
				Size:     0,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.StoreFile(tt.req)
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if result == nil {
					t.Fatal("result is nil")
				}
				if result.Hash == "" {
					t.Error("hash is empty")
				}
				if result.OriginalName != tt.req.Filename {
					t.Errorf("original name mismatch: got %s, want %s", result.OriginalName, tt.req.Filename)
				}
			}
		})
	}
}

func TestFileService_GetFileByHash(t *testing.T) {
	store, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	service := NewFileService(store, logger)

	// Store a file first
	storeReq := FileStoreRequest{
		Reader:   bytes.NewReader([]byte("test content")),
		Filename: "test.txt",
		MimeType: "text/plain",
		Size:     12,
	}
	stored, err := service.StoreFile(storeReq)
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}

	tests := []struct {
		name        string
		hash        string
		expectError bool
	}{
		{
			name:        "valid hash",
			hash:        stored.Hash,
			expectError: false,
		},
		{
			name:        "empty hash",
			hash:        "",
			expectError: true,
		},
		{
			name:        "non-existent hash",
			hash:        "nonexistent",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.GetFileByHash(tt.hash)
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if result == nil {
					t.Fatal("result is nil")
				}
				if result.Metadata.Hash != stored.Hash {
					t.Errorf("hash mismatch: got %s, want %s", result.Metadata.Hash, stored.Hash)
				}
				result.Reader.Close()
			}
		})
	}
}

func TestFileService_GetFileMetadata(t *testing.T) {
	store, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	service := NewFileService(store, logger)

	// Store a file first
	storeReq := FileStoreRequest{
		Reader:   bytes.NewReader([]byte("test content")),
		Filename: "test.txt",
		MimeType: "text/plain",
		Size:     12,
	}
	stored, err := service.StoreFile(storeReq)
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}

	result, err := service.GetFileMetadata(stored.Hash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Hash != stored.Hash {
		t.Errorf("hash mismatch: got %s, want %s", result.Hash, stored.Hash)
	}
	if result.OriginalName != stored.OriginalName {
		t.Errorf("original name mismatch: got %s, want %s", result.OriginalName, stored.OriginalName)
	}
}

func TestFileService_RenameFile(t *testing.T) {
	store, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	service := NewFileService(store, logger)

	// Store a file first
	storeReq := FileStoreRequest{
		Reader:   bytes.NewReader([]byte("test content")),
		Filename: "oldname.txt",
		MimeType: "text/plain",
		Size:     12,
	}
	stored, err := service.StoreFile(storeReq)
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}

	tests := []struct {
		name        string
		req         FileRenameRequest
		expectError bool
	}{
		{
			name: "valid rename",
			req: FileRenameRequest{
				Hash:             stored.Hash,
				NewName:          "newname.txt",
				UpdateStoredFile: false,
			},
			expectError: false,
		},
		{
			name: "empty hash",
			req: FileRenameRequest{
				Hash:    "",
				NewName: "newname.txt",
			},
			expectError: true,
		},
		{
			name: "empty new name",
			req: FileRenameRequest{
				Hash:    stored.Hash,
				NewName: "",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.RenameFile(tt.req)
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if result == nil {
					t.Fatal("result is nil")
				}
				if result.NewName != tt.req.NewName {
					t.Errorf("new name mismatch: got %s, want %s", result.NewName, tt.req.NewName)
				}
			}
		})
	}
}

func TestFileService_DeleteFile(t *testing.T) {
	store, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	service := NewFileService(store, logger)

	// Store a file first
	storeReq := FileStoreRequest{
		Reader:   bytes.NewReader([]byte("test content")),
		Filename: "test.txt",
		MimeType: "text/plain",
		Size:     12,
	}
	stored, err := service.StoreFile(storeReq)
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}

	req := FileDeleteRequest{Hash: stored.Hash}
	result, err := service.DeleteFile(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Deleted {
		t.Error("file not marked as deleted")
	}
	if result.Hash != stored.Hash {
		t.Errorf("hash mismatch: got %s, want %s", result.Hash, stored.Hash)
	}

	// Try to delete again - should fail
	_, err = service.DeleteFile(req)
	if err == nil {
		t.Error("expected error when deleting non-existent file")
	}
}

func TestFileService_UpdateFileMetadata(t *testing.T) {
	store, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	service := NewFileService(store, logger)

	// Store a file first
	storeReq := FileStoreRequest{
		Reader:   bytes.NewReader([]byte("test content")),
		Filename: "test.txt",
		MimeType: "text/plain",
		Size:     12,
		Metadata: map[string]string{"key1": "value1"},
	}
	stored, err := service.StoreFile(storeReq)
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}

	tests := []struct {
		name        string
		req         MetadataUpdateRequest
		expectError bool
	}{
		{
			name: "merge metadata",
			req: MetadataUpdateRequest{
				Hash:     stored.Hash,
				Action:   "merge",
				Metadata: map[string]string{"key2": "value2"},
			},
			expectError: false,
		},
		{
			name: "replace metadata",
			req: MetadataUpdateRequest{
				Hash:     stored.Hash,
				Action:   "replace",
				Metadata: map[string]string{"key3": "value3"},
			},
			expectError: false,
		},
		{
			name: "empty hash",
			req: MetadataUpdateRequest{
				Hash:   "",
				Action: "merge",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.UpdateFileMetadata(tt.req)
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if result == nil {
					t.Fatal("result is nil")
				}
				if result.Hash != stored.Hash {
					t.Errorf("hash mismatch: got %s, want %s", result.Hash, stored.Hash)
				}
			}
		})
	}
}

func TestFileService_BatchUpdateFileMetadata(t *testing.T) {
	store, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	service := NewFileService(store, logger)

	// Store files first
	var hashes []string
	for i := 0; i < 3; i++ {
		storeReq := FileStoreRequest{
			Reader:   bytes.NewReader([]byte("test content")),
			Filename: "test.txt",
			MimeType: "text/plain",
			Size:     12,
		}
		stored, err := service.StoreFile(storeReq)
		if err != nil {
			t.Fatalf("failed to store file: %v", err)
		}
		hashes = append(hashes, stored.Hash)
	}

	req := BatchMetadataUpdateRequest{
		Updates: []MetadataUpdateRequest{
			{
				Hash:     hashes[0],
				Action:   "merge",
				Metadata: map[string]string{"key1": "value1"},
			},
			{
				Hash:     hashes[1],
				Action:   "merge",
				Metadata: map[string]string{"key2": "value2"},
			},
		},
	}

	result, err := service.BatchUpdateFileMetadata(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("total mismatch: got %d, want 2", result.Total)
	}
	if result.SuccessCount != 2 {
		t.Errorf("success count mismatch: got %d, want 2", result.SuccessCount)
	}
}

func TestFileService_SearchFiles(t *testing.T) {
	store, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	service := NewFileService(store, logger)

	// Store files with different names and content (to avoid deduplication)
	files := []struct {
		name    string
		content string
	}{
		{"test1.txt", "test content 1"},
		{"test2.txt", "test content 2"},
		{"other.txt", "other content"},
	}
	for _, file := range files {
		storeReq := FileStoreRequest{
			Reader:   bytes.NewReader([]byte(file.content)),
			Filename: file.name,
			MimeType: "text/plain",
			Size:     int64(len(file.content)),
		}
		_, err := service.StoreFile(storeReq)
		if err != nil {
			t.Fatalf("failed to store file: %v", err)
		}
	}

	req := FileSearchRequest{Query: "test"}
	result, err := service.SearchFiles(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Search is case-insensitive partial match, so "test" should match "test1.txt" and "test2.txt"
	if result.Count < 2 {
		t.Logf("Search results: %+v", result.Results)
		t.Errorf("expected at least 2 results for query 'test', got %d", result.Count)
	}
}

func TestFileService_StoreFileFromMultipart(t *testing.T) {
	store, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	service := NewFileService(store, logger)

	// Create a multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	part.Write([]byte("test content"))
	writer.Close()

	// Parse the multipart form
	reader := bytes.NewReader(buf.Bytes())
	form, err := multipart.NewReader(reader, writer.Boundary()).ReadForm(1024)
	if err != nil {
		t.Fatalf("failed to parse form: %v", err)
	}
	defer form.RemoveAll()

	header := form.File["file"][0]
	result, err := service.StoreFileFromMultipart(header, "", "test comment")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}
	if result.OriginalName != "test.txt" {
		t.Errorf("original name mismatch: got %s, want test.txt", result.OriginalName)
	}
}

func TestFileService_TransformStoreResultToRecord(t *testing.T) {
	store, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	service := NewFileService(store, logger)

	result := &FileStoreResponse{
		Hash:         "testhash",
		OriginalName: "test.txt",
		StoredPath:   "storage/test.txt",
		Category:     "documents/pdf",
		MimeType:     "text/plain",
		Size:         12,
		UploadedAt:   time.Now(),
		Duplicate:    false,
	}

	record := service.TransformStoreResultToRecord(result, "test comment")
	if record == nil {
		t.Fatal("record is nil")
	}

	if record["hash"] != "testhash" {
		t.Errorf("hash mismatch: got %v, want testhash", record["hash"])
	}
	if record["comment"] != "test comment" {
		t.Errorf("comment mismatch: got %v, want test comment", record["comment"])
	}
}

func TestFileService_AppendNDJSON(t *testing.T) {
	store, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	service := NewFileService(store, logger)

	docs := []map[string]any{
		{"key1": "value1"},
		{"key2": "value2"},
	}

	path, err := service.AppendNDJSON("test.ndjson", docs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if path == "" {
		t.Error("path is empty")
	}

	// Verify file exists
	fullPath := filepath.Join(tmpDir, path)
	if _, err := os.Stat(fullPath); err != nil {
		t.Errorf("file not created: %v", err)
	}
}

func TestFileService_WriteJSONFile(t *testing.T) {
	store, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	service := NewFileService(store, logger)

	payload := map[string]any{
		"key": "value",
	}

	path, err := service.WriteJSONFile("test.json", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if path == "" {
		t.Error("path is empty")
	}

	// Verify file exists
	fullPath := filepath.Join(tmpDir, path)
	if _, err := os.Stat(fullPath); err != nil {
		t.Errorf("file not created: %v", err)
	}
}

func TestFileService_NextJSONBatchPath(t *testing.T) {
	store, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	service := NewFileService(store, logger)

	path := service.NextJSONBatchPath("sql", "test-namespace")
	if path == "" {
		t.Error("path is empty")
	}

	if !strings.Contains(path, "sql") {
		t.Errorf("path should contain 'sql': %s", path)
	}
	if !strings.Contains(path, "test-namespace") {
		t.Errorf("path should contain 'test-namespace': %s", path)
	}
}

func TestFileService_ErrorHandling(t *testing.T) {
	store, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	service := NewFileService(store, logger)

	// Test error wrapping
	_, err := service.GetFileByHash("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent hash")
	}
	if !errors.Is(err, storage.ErrFileNotFound) {
		// Check if error message contains expected text
		if !strings.Contains(err.Error(), "retrieval error") {
			t.Errorf("error should be wrapped: %v", err)
		}
	}
}

