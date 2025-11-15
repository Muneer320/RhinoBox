package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
	"log/slog"
)

// TestFileMoveEndToEnd tests the complete flow: upload -> move -> verify
func TestFileMoveEndToEnd(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        tmpDir,
		MaxUploadBytes: 100 * 1024 * 1024, // 100MB
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Step 1: Upload a file
	testContent := []byte("This is a test file for move testing. " + strings.Repeat("Content ", 50))
	testFilename := "test_move.txt"
	uploadHash, _ := uploadTestFile(t, srv, testFilename, testContent, "text/plain")

	// Step 2: Get initial metadata
	initialMetadata := getFileMetadata(t, srv, uploadHash)
	initialCategory := initialMetadata["category"].(string)
	if initialCategory == "" {
		t.Fatal("initial category should not be empty")
	}

	// Step 3: Move file to new category
	newCategory := "documents/archived"
	moveReq := map[string]interface{}{
		"new_category": newCategory,
		"reason":       "test move operation",
	}

	reqBody, _ := json.Marshal(moveReq)
	req := httptest.NewRequest("PATCH", fmt.Sprintf("/files/%s/move", uploadHash), bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("move request failed with status %d: %s", w.Code, w.Body.String())
	}

	var moveResult map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &moveResult); err != nil {
		t.Fatalf("failed to parse move result: %v", err)
	}

	if !moveResult["moved"].(bool) {
		t.Error("expected file to be moved")
	}

	if moveResult["old_category"].(string) != initialCategory {
		t.Errorf("expected old category %s, got %s", initialCategory, moveResult["old_category"].(string))
	}

	if moveResult["new_category"].(string) != newCategory {
		t.Errorf("expected new category %s, got %s", newCategory, moveResult["new_category"].(string))
	}

	// Step 4: Verify metadata was updated
	updatedMetadata := getFileMetadata(t, srv, uploadHash)
	if updatedMetadata["category"].(string) != newCategory {
		t.Errorf("expected category %s, got %s", newCategory, updatedMetadata["category"].(string))
	}

	// Step 5: Verify file can still be downloaded
	downloadedContent := downloadFileByHash(t, srv, uploadHash)
	if !bytes.Equal(downloadedContent, testContent) {
		t.Errorf("downloaded content mismatch after move")
	}

	// Step 6: Verify old path no longer works (if path changed)
	if moveResult["old_path"].(string) != moveResult["new_path"].(string) {
		oldPath := moveResult["old_path"].(string)
		req = httptest.NewRequest("GET", fmt.Sprintf("/files/download?path=%s", oldPath), nil)
		w = httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)
		if w.Code == http.StatusOK {
			t.Error("old path should not work after move")
		}
	}
}

// TestBatchFileMoveEndToEnd tests batch move operations
func TestBatchFileMoveEndToEnd(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        tmpDir,
		MaxUploadBytes: 100 * 1024 * 1024,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Upload multiple files
	hashes := make([]string, 3)
	for i := 0; i < 3; i++ {
		content := []byte(fmt.Sprintf("test file %d content", i))
		filename := fmt.Sprintf("test%d.txt", i)
		hash, _ := uploadTestFile(t, srv, filename, content, "text/plain")
		hashes[i] = hash
	}

	// Batch move files
	batchReq := map[string]interface{}{
		"files": []map[string]interface{}{
			{"hash": hashes[0], "new_category": "documents/archived", "reason": "archive"},
			{"hash": hashes[1], "new_category": "documents/important", "reason": "organize"},
			{"hash": hashes[2], "new_category": "documents/temp", "reason": "temporary"},
		},
	}

	reqBody, _ := json.Marshal(batchReq)
	req := httptest.NewRequest("PATCH", "/files/batch/move", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("batch move request failed with status %d: %s", w.Code, w.Body.String())
	}

	var batchResult map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &batchResult); err != nil {
		t.Fatalf("failed to parse batch result: %v", err)
	}

	if int(batchResult["total"].(float64)) != 3 {
		t.Errorf("expected total 3, got %v", batchResult["total"])
	}

	if int(batchResult["success_count"].(float64)) != 3 {
		t.Errorf("expected 3 successes, got %v", batchResult["success_count"])
	}

	// Verify each file was moved
	results := batchResult["results"].([]interface{})
	for i, result := range results {
		resultMap := result.(map[string]interface{})
		if !resultMap["moved"].(bool) {
			t.Errorf("file %d was not moved", i)
		}
	}
}

// TestFileMoveInvalidCategory tests move with invalid category
func TestFileMoveInvalidCategory(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        tmpDir,
		MaxUploadBytes: 100 * 1024 * 1024,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Upload a file
	hash, _ := uploadTestFile(t, srv, "test.txt", []byte("test"), "text/plain")

	// Try to move with invalid category (path traversal)
	moveReq := map[string]interface{}{
		"new_category": "../invalid",
	}

	reqBody, _ := json.Marshal(moveReq)
	req := httptest.NewRequest("PATCH", fmt.Sprintf("/files/%s/move", hash), bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid category, got %d", w.Code)
	}
}

// TestFileMoveNotFound tests move with nonexistent file
func TestFileMoveNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        tmpDir,
		MaxUploadBytes: 100 * 1024 * 1024,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	moveReq := map[string]interface{}{
		"new_category": "documents/pdf",
	}

	reqBody, _ := json.Marshal(moveReq)
	req := httptest.NewRequest("PATCH", "/files/nonexistent/move", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent file, got %d", w.Code)
	}
}

