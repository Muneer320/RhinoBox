package storage

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/storage"
)

func TestGetFileByHash(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a test file
	testContent := []byte("test file content for retrieval")
	testFilename := "test_file.txt"
	reader := bytes.NewReader(testContent)

	// Store the file
	result, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   reader,
		Filename: testFilename,
		MimeType: "text/plain",
		Size:     int64(len(testContent)),
	})
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}

	hash := result.Metadata.Hash

	// Test: Retrieve file by hash
	retrieved, err := mgr.GetFileByHash(hash)
	if err != nil {
		t.Fatalf("failed to retrieve file: %v", err)
	}
	defer retrieved.Reader.Close()

	// Verify metadata
	if retrieved.Metadata.Hash != hash {
		t.Errorf("expected hash %s, got %s", hash, retrieved.Metadata.Hash)
	}
	if retrieved.Metadata.OriginalName != testFilename {
		t.Errorf("expected filename %s, got %s", testFilename, retrieved.Metadata.OriginalName)
	}

	// Verify file content
	readContent, err := io.ReadAll(retrieved.Reader)
	if err != nil {
		t.Fatalf("failed to read retrieved file: %v", err)
	}
	if !bytes.Equal(readContent, testContent) {
		t.Errorf("file content mismatch: expected %q, got %q", string(testContent), string(readContent))
	}

	// Test: Non-existent hash
	_, err = mgr.GetFileByHash("nonexistent_hash_1234567890123456789012345678901234567890123456789012345678901234")
	if err == nil {
		t.Error("expected error for non-existent hash")
	}
	if !strings.Contains(err.Error(), "file not found") {
		t.Errorf("expected 'file not found' error, got: %v", err)
	}

	// Test: Empty hash
	_, err = mgr.GetFileByHash("")
	if err == nil {
		t.Error("expected error for empty hash")
	}
}

func TestGetFileByPath(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a test file
	testContent := []byte("test file content for path retrieval")
	testFilename := "path_test.txt"
	reader := bytes.NewReader(testContent)

	// Store the file
	result, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   reader,
		Filename: testFilename,
		MimeType: "text/plain",
		Size:     int64(len(testContent)),
	})
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}

	storedPath := result.Metadata.StoredPath

	// Test: Retrieve file by path
	retrieved, err := mgr.GetFileByPath(storedPath)
	if err != nil {
		t.Fatalf("failed to retrieve file by path: %v", err)
	}
	defer retrieved.Reader.Close()

	// Verify metadata
	if retrieved.Metadata.StoredPath != storedPath {
		t.Errorf("expected path %s, got %s", storedPath, retrieved.Metadata.StoredPath)
	}

	// Verify file content
	readContent, err := io.ReadAll(retrieved.Reader)
	if err != nil {
		t.Fatalf("failed to read retrieved file: %v", err)
	}
	if !bytes.Equal(readContent, testContent) {
		t.Errorf("file content mismatch: expected %q, got %q", string(testContent), string(readContent))
	}

	// Test: Non-existent path
	_, err = mgr.GetFileByPath("nonexistent/path/file.txt")
	if err == nil {
		t.Error("expected error for non-existent path")
	}

	// Test: Empty path
	_, err = mgr.GetFileByPath("")
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestGetFileByPath_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Test path traversal attempts
	maliciousPaths := []string{
		"../etc/passwd",
		"../../etc/passwd",
		"storage/../../../etc/passwd",
		"..\\windows\\system32",
		"storage/..\\..\\..\\etc\\passwd",
	}

	for _, path := range maliciousPaths {
		_, err := mgr.GetFileByPath(path)
		if err == nil {
			t.Errorf("expected error for path traversal attempt: %s", path)
		}
		if !strings.Contains(err.Error(), "invalid path") && !strings.Contains(err.Error(), "path traversal") {
			t.Errorf("expected path traversal error for %s, got: %v", path, err)
		}
	}
}

func TestGetFileMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a test file
	testContent := []byte("test file content for metadata")
	testFilename := "metadata_test.txt"
	reader := bytes.NewReader(testContent)

	// Store the file
	result, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   reader,
		Filename: testFilename,
		MimeType: "text/plain",
		Size:     int64(len(testContent)),
	})
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}

	hash := result.Metadata.Hash

	// Test: Get metadata
	metadata, err := mgr.GetFileMetadata(hash)
	if err != nil {
		t.Fatalf("failed to get metadata: %v", err)
	}

	// Verify metadata
	if metadata.Hash != hash {
		t.Errorf("expected hash %s, got %s", hash, metadata.Hash)
	}
	if metadata.OriginalName != testFilename {
		t.Errorf("expected filename %s, got %s", testFilename, metadata.OriginalName)
	}
	if metadata.MimeType != "text/plain" {
		t.Errorf("expected mime type text/plain, got %s", metadata.MimeType)
	}
	if metadata.Size != int64(len(testContent)) {
		t.Errorf("expected size %d, got %d", len(testContent), metadata.Size)
	}

	// Test: Non-existent hash
	_, err = mgr.GetFileMetadata("nonexistent_hash_1234567890123456789012345678901234567890123456789012345678901234")
	if err == nil {
		t.Error("expected error for non-existent hash")
	}

	// Test: Empty hash
	_, err = mgr.GetFileMetadata("")
	if err == nil {
		t.Error("expected error for empty hash")
	}
}

func TestGetFileMetadata_FileDeleted(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a test file
	testContent := []byte("test file content")
	testFilename := "deleted_test.txt"
	reader := bytes.NewReader(testContent)

	// Store the file
	result, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   reader,
		Filename: testFilename,
		MimeType: "text/plain",
		Size:     int64(len(testContent)),
	})
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}

	hash := result.Metadata.Hash

	// Delete the file from disk (but keep metadata)
	fullPath := filepath.Join(tmpDir, result.Metadata.StoredPath)
	if err := os.Remove(fullPath); err != nil {
		t.Fatalf("failed to delete file: %v", err)
	}

	// Test: Get metadata should fail because file doesn't exist on disk
	_, err = mgr.GetFileMetadata(hash)
	if err == nil {
		t.Error("expected error when file is deleted from disk")
	}
	if !strings.Contains(err.Error(), "file not found") {
		t.Errorf("expected 'file not found' error, got: %v", err)
	}
}

func TestLogDownload(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a test file
	testContent := []byte("test file content")
	testFilename := "download_test.txt"
	reader := bytes.NewReader(testContent)

	// Store the file
	result, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   reader,
		Filename: testFilename,
		MimeType: "text/plain",
		Size:     int64(len(testContent)),
	})
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}

	// Log a download
	downloadLog := storage.DownloadLog{
		Hash:         result.Metadata.Hash,
		StoredPath:   result.Metadata.StoredPath,
		OriginalName: result.Metadata.OriginalName,
		MimeType:     result.Metadata.MimeType,
		Size:         result.Metadata.Size,
		UserAgent:    "test-agent",
		IPAddress:    "127.0.0.1",
	}

	err = mgr.LogDownload(downloadLog)
	if err != nil {
		t.Fatalf("failed to log download: %v", err)
	}

	// Verify log file was created
	logPath := filepath.Join(tmpDir, "metadata", "download_log.ndjson")
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("download log file not created: %v", err)
	}

	// Verify log file content
	logContent, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(logContent), result.Metadata.Hash) {
		t.Error("log file does not contain file hash")
	}
	if !strings.Contains(string(logContent), testFilename) {
		t.Error("log file does not contain original filename")
	}
}

func TestGetFileByHash_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Store multiple files
	files := []struct {
		name    string
		content []byte
	}{
		{"file1.txt", []byte("content 1")},
		{"file2.txt", []byte("content 2")},
		{"file3.jpg", []byte("image content")},
	}

	hashes := make([]string, len(files))
	for i, file := range files {
		result, err := mgr.StoreFile(storage.StoreRequest{
			Reader:   bytes.NewReader(file.content),
			Filename: file.name,
			MimeType: "text/plain",
			Size:     int64(len(file.content)),
		})
		if err != nil {
			t.Fatalf("failed to store file %s: %v", file.name, err)
		}
		hashes[i] = result.Metadata.Hash
	}

	// Retrieve each file and verify
	for i, hash := range hashes {
		retrieved, err := mgr.GetFileByHash(hash)
		if err != nil {
			t.Fatalf("failed to retrieve file %d: %v", i, err)
		}
		defer retrieved.Reader.Close()

		readContent, err := io.ReadAll(retrieved.Reader)
		if err != nil {
			t.Fatalf("failed to read file %d: %v", i, err)
		}

		if !bytes.Equal(readContent, files[i].content) {
			t.Errorf("file %d content mismatch: expected %q, got %q", i, string(files[i].content), string(readContent))
		}
	}
}

func TestGetFileByPath_AbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Test that absolute paths are rejected
	absPath := "/etc/passwd"
	_, err = mgr.GetFileByPath(absPath)
	if err == nil {
		t.Error("expected error for absolute path")
	}
	if !strings.Contains(err.Error(), "invalid path") {
		t.Errorf("expected 'invalid path' error, got: %v", err)
	}
}

func TestGetFileByPath_NullBytes(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Test that paths with null bytes are rejected
	nullPath := "storage/images\000test.jpg"
	_, err = mgr.GetFileByPath(nullPath)
	if err == nil {
		t.Error("expected error for path with null bytes")
	}
	if !strings.Contains(err.Error(), "invalid path") {
		t.Errorf("expected 'invalid path' error, got: %v", err)
	}
}
