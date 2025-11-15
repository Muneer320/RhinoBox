package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/config"
	"github.com/Muneer320/RhinoBox/internal/storage"
	"log/slog"
)

func TestHandleFileCopy(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Config{DataDir: tmpDir}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	server, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// First, store a file
	storeContent := []byte("test file content for copy")
	storeReq := storage.StoreRequest{
		Reader:   bytes.NewReader(storeContent),
		Filename: "original.txt",
		MimeType: "text/plain",
		Size:     int64(len(storeContent)),
	}
	storeResult, err := server.storage.StoreFile(storeReq)
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}

	// Test copy with new name
	copyBody := map[string]any{
		"new_name": "copy.txt",
		"metadata": map[string]string{
			"comment": "working copy",
		},
	}
	body, _ := json.Marshal(copyBody)

	req := httptest.NewRequest("POST", "/files/"+storeResult.Metadata.Hash+"/copy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if result["new_hash"] == nil {
		t.Error("response missing new_hash")
	}
	if result["hard_link"] == true {
		t.Error("expected full copy, got hard link")
	}

	// Test copy with hard link
	copyBody2 := map[string]any{
		"new_name": "hardlink.txt",
		"hard_link": true,
	}
	body2, _ := json.Marshal(copyBody2)

	req2 := httptest.NewRequest("POST", "/files/"+storeResult.Metadata.Hash+"/copy", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()

	server.router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w2.Code, w2.Body.String())
	}

	var result2 map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &result2); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if result2["hard_link"] != true {
		t.Error("expected hard link, got full copy")
	}
}

func TestHandleFileCopy_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Config{DataDir: tmpDir}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	server, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	copyBody := map[string]any{
		"new_name": "copy.txt",
	}
	body, _ := json.Marshal(copyBody)

	req := httptest.NewRequest("POST", "/files/nonexistent/copy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleBatchFileCopy(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Config{DataDir: tmpDir}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	server, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Store two files
	content1 := []byte("file 1 content")
	storeReq1 := storage.StoreRequest{
		Reader:   bytes.NewReader(content1),
		Filename: "file1.txt",
		MimeType: "text/plain",
		Size:     int64(len(content1)),
	}
	result1, err := server.storage.StoreFile(storeReq1)
	if err != nil {
		t.Fatalf("failed to store file 1: %v", err)
	}

	content2 := []byte("file 2 content")
	storeReq2 := storage.StoreRequest{
		Reader:   bytes.NewReader(content2),
		Filename: "file2.txt",
		MimeType: "text/plain",
		Size:     int64(len(content2)),
	}
	result2, err := server.storage.StoreFile(storeReq2)
	if err != nil {
		t.Fatalf("failed to store file 2: %v", err)
	}

	// Batch copy
	batchBody := map[string]any{
		"copies": []map[string]any{
			{
				"hash":     result1.Metadata.Hash,
				"new_name": "copy1.txt",
			},
			{
				"hash":     result2.Metadata.Hash,
				"new_name": "copy2.txt",
				"hard_link": true,
			},
		},
	}
	body, _ := json.Marshal(batchBody)

	req := httptest.NewRequest("POST", "/files/copy/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if result["total"] != float64(2) {
		t.Errorf("expected total 2, got %v", result["total"])
	}
	if result["success_count"] != float64(2) {
		t.Errorf("expected success_count 2, got %v", result["success_count"])
	}
}

func TestHandleBatchFileCopy_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Config{DataDir: tmpDir}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	server, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	batchBody := map[string]any{
		"copies": []map[string]any{},
	}
	body, _ := json.Marshal(batchBody)

	req := httptest.NewRequest("POST", "/files/copy/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleBatchFileCopy_TooMany(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Config{DataDir: tmpDir}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	server, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Create 101 copy requests
	copies := make([]map[string]any, 101)
	for i := 0; i < 101; i++ {
		copies[i] = map[string]any{
			"hash":     "test",
			"new_name": "copy.txt",
		}
	}

	batchBody := map[string]any{
		"copies": copies,
	}
	body, _ := json.Marshal(batchBody)

	req := httptest.NewRequest("POST", "/files/copy/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

