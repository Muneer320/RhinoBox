package service

import (
	"bytes"
	"io"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/storage"
)

// TestFileService_Integration tests FileService with a real storage manager
func TestFileService_Integration(t *testing.T) {
	tempDir := t.TempDir()
	store, err := storage.NewManager(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage manager: %v", err)
	}

	service := NewFileService(store)

	// Test StoreFile
	req := storage.StoreRequest{
		Reader:   io.NopCloser(io.Reader(bytes.NewReader([]byte("test content")))),
		Filename: "test.jpg",
		MimeType: "image/jpeg",
		Size:     12,
	}

	result, err := service.StoreFile(req)
	if err != nil {
		t.Fatalf("unexpected error storing file: %v", err)
	}
	if result.Metadata.Hash == "" {
		t.Fatalf("expected hash to be set")
	}

	// Test GetFileMetadata
	metadata, err := service.GetFileMetadata(result.Metadata.Hash)
	if err != nil {
		t.Fatalf("unexpected error getting metadata: %v", err)
	}
	if metadata.Hash != result.Metadata.Hash {
		t.Fatalf("expected hash %s, got %s", result.Metadata.Hash, metadata.Hash)
	}

	// Test SearchFiles
	results := service.SearchFiles("test")
	if len(results) == 0 {
		t.Fatalf("expected at least one result")
	}

	// Test DeleteFile
	deleteReq := storage.DeleteRequest{
		Hash: result.Metadata.Hash,
	}
	deleteResult, err := service.DeleteFile(deleteReq)
	if err != nil {
		t.Fatalf("unexpected error deleting file: %v", err)
	}
	if !deleteResult.Deleted {
		t.Fatalf("expected Deleted=true")
	}

	// Verify file is deleted
	_, err = service.GetFileMetadata(result.Metadata.Hash)
	if err == nil {
		t.Fatalf("expected error after deletion")
	}
}

