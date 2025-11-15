package storage_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/storage"
)

func TestDeleteFileRemovesFileAndMetadata(t *testing.T) {
	dir := t.TempDir()
	mgr, err := storage.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Store a file first
	payload := bytes.Repeat([]byte("test content for deletion"), 1024)
	res, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   bytes.NewReader(payload),
		Filename: "test_file.jpg",
		MimeType: "image/jpeg",
		Size:     int64(len(payload)),
		Metadata: map[string]string{"source": "unit-test"},
	})
	if err != nil {
		t.Fatalf("StoreFile: %v", err)
	}

	hash := res.Metadata.Hash
	storedPath := filepath.Join(dir, res.Metadata.StoredPath)

	// Verify file exists
	if _, err := os.Stat(storedPath); err != nil {
		t.Fatalf("stored file should exist: %v", err)
	}

	// Verify metadata exists
	if mgr.FindByOriginalName("test_file.jpg") == nil || len(mgr.FindByOriginalName("test_file.jpg")) == 0 {
		t.Fatalf("metadata should exist before deletion")
	}

	// Delete the file
	deleteReq := storage.DeleteRequest{Hash: hash}
	result, err := mgr.DeleteFile(deleteReq)
	if err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}

	// Verify deletion result
	if !result.Deleted {
		t.Fatalf("expected Deleted=true")
	}
	if result.Hash != hash {
		t.Fatalf("expected hash %s, got %s", hash, result.Hash)
	}
	if result.OriginalName != "test_file.jpg" {
		t.Fatalf("expected original_name test_file.jpg, got %s", result.OriginalName)
	}

	// Verify file is removed from filesystem
	if _, err := os.Stat(storedPath); !os.IsNotExist(err) {
		t.Fatalf("stored file should be deleted: %v", err)
	}

	// Verify metadata is removed
	if found := mgr.FindByOriginalName("test_file.jpg"); len(found) > 0 {
		t.Fatalf("metadata should be removed after deletion")
	}
}

func TestDeleteFileNotFound(t *testing.T) {
	dir := t.TempDir()
	mgr, err := storage.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Try to delete non-existent file
	deleteReq := storage.DeleteRequest{Hash: "nonexistent_hash_1234567890123456789012345678901234567890123456789012345678901234"}
	_, err = mgr.DeleteFile(deleteReq)
	if err == nil {
		t.Fatalf("expected error for non-existent file")
	}
	if !errors.Is(err, storage.ErrFileNotFound) {
		t.Fatalf("expected ErrFileNotFound, got: %v", err)
	}
}

func TestDeleteFileWithEmptyHash(t *testing.T) {
	dir := t.TempDir()
	mgr, err := storage.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Try to delete with empty hash
	deleteReq := storage.DeleteRequest{Hash: ""}
	_, err = mgr.DeleteFile(deleteReq)
	if err == nil {
		t.Fatalf("expected error for empty hash")
	}
	if !errors.Is(err, storage.ErrFileNotFound) {
		t.Fatalf("expected ErrFileNotFound, got: %v", err)
	}
}

func TestDeleteFileHandlesMissingPhysicalFile(t *testing.T) {
	dir := t.TempDir()
	mgr, err := storage.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Store a file
	payload := bytes.Repeat([]byte("test content"), 512)
	res, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   bytes.NewReader(payload),
		Filename: "test.jpg",
		MimeType: "image/jpeg",
		Size:     int64(len(payload)),
	})
	if err != nil {
		t.Fatalf("StoreFile: %v", err)
	}

	hash := res.Metadata.Hash
	storedPath := filepath.Join(dir, res.Metadata.StoredPath)

	// Manually delete the physical file (simulating external deletion)
	if err := os.Remove(storedPath); err != nil {
		t.Fatalf("failed to manually remove file: %v", err)
	}

	// Delete should still succeed (metadata-only deletion)
	deleteReq := storage.DeleteRequest{Hash: hash}
	result, err := mgr.DeleteFile(deleteReq)
	if err != nil {
		t.Fatalf("DeleteFile should succeed even if physical file is missing: %v", err)
	}

	if !result.Deleted {
		t.Fatalf("expected Deleted=true")
	}

	// Verify metadata is removed
	if found := mgr.FindByOriginalName("test.jpg"); len(found) > 0 {
		t.Fatalf("metadata should be removed after deletion")
	}
}

func TestDeleteFileCreatesAuditLog(t *testing.T) {
	dir := t.TempDir()
	mgr, err := storage.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Store a file
	payload := bytes.Repeat([]byte("audit test"), 256)
	res, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   bytes.NewReader(payload),
		Filename: "audit_test.jpg",
		MimeType: "image/jpeg",
		Size:     int64(len(payload)),
	})
	if err != nil {
		t.Fatalf("StoreFile: %v", err)
	}

	// Delete the file
	deleteReq := storage.DeleteRequest{Hash: res.Metadata.Hash}
	_, err = mgr.DeleteFile(deleteReq)
	if err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}

	// Verify audit log exists
	logPath := filepath.Join(dir, "metadata", "delete_log.ndjson")
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("delete log should exist: %v", err)
	}

	// Verify log contains entry
	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read delete log: %v", err)
	}
	if len(logData) == 0 {
		t.Fatalf("delete log should contain entry")
	}
	if !bytes.Contains(logData, []byte(res.Metadata.Hash)) {
		t.Fatalf("delete log should contain file hash")
	}
}

func TestDeleteFileMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	mgr, err := storage.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Store multiple files
	var hashes []string
	for i := 0; i < 5; i++ {
		payload := bytes.Repeat([]byte{byte(i)}, 512)
		res, err := mgr.StoreFile(storage.StoreRequest{
			Reader:   bytes.NewReader(payload),
			Filename: "test_file_" + string(rune('0'+i)) + ".jpg",
			MimeType: "image/jpeg",
			Size:     int64(len(payload)),
		})
		if err != nil {
			t.Fatalf("StoreFile %d: %v", i, err)
		}
		hashes = append(hashes, res.Metadata.Hash)
	}

	// Delete one file
	deleteReq := storage.DeleteRequest{Hash: hashes[2]}
	result, err := mgr.DeleteFile(deleteReq)
	if err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	if !result.Deleted {
		t.Fatalf("expected Deleted=true")
	}

	// Verify other files still exist
	for i := range hashes {
		if i == 2 {
			// This one should be deleted
			if found := mgr.FindByOriginalName("test_file_2.jpg"); len(found) > 0 {
				t.Fatalf("file 2 should be deleted")
			}
		} else {
			// These should still exist
			if found := mgr.FindByOriginalName("test_file_" + string(rune('0'+i)) + ".jpg"); len(found) == 0 {
				t.Fatalf("file %d should still exist", i)
			}
		}
	}
}
