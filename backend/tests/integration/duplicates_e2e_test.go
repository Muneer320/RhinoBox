package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
	"github.com/Muneer320/RhinoBox/internal/storage"
	"log/slog"
)

func TestDuplicateScanE2E(t *testing.T) {
	root := t.TempDir()
	cfg := config.Config{
		DataDir:      root,
		MaxUploadBytes: 100 * 1024 * 1024, // 100MB
	}
	logger := slog.Default()

	server, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Upload first file
	content1 := []byte("test file content for duplicate detection")
	req1 := httptest.NewRequest("POST", "/ingest/media", createMultipartBody("file1.txt", content1))
	req1.Header.Set("Content-Type", "multipart/form-data; boundary=testboundary")
	w1 := httptest.NewRecorder()
	server.Router().ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w1.Code, w1.Body.String())
	}

	var response1 map[string]interface{}
	if err := json.Unmarshal(w1.Body.Bytes(), &response1); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Upload same file again (should be detected as duplicate during upload)
	req2 := httptest.NewRequest("POST", "/ingest/media", createMultipartBody("file2.txt", content1))
	req2.Header.Set("Content-Type", "multipart/form-data; boundary=testboundary")
	w2 := httptest.NewRecorder()
	server.Router().ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w2.Code, w2.Body.String())
	}

	// Create a duplicate manually on disk and in metadata to test scan

	// Get hash from first upload
	stored := response1["stored"].([]interface{})
	if len(stored) == 0 {
		t.Fatal("no files in stored response")
	}
	firstFile := stored[0].(map[string]interface{})
	hash := firstFile["hash"].(string)

	// Create duplicate file on disk
	duplicatePath := filepath.Join(root, "storage", "documents", "txt", hash[:12]+"_duplicate.txt")
	if err := os.MkdirAll(filepath.Dir(duplicatePath), 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(duplicatePath, content1, 0644); err != nil {
		t.Fatalf("failed to write duplicate: %v", err)
	}

	// Add to metadata
	metadataPath := filepath.Join(root, "metadata", "files.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("failed to read metadata: %v", err)
	}

	var items []storage.FileMetadata
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	duplicateMeta := storage.FileMetadata{
		Hash:         hash,
		OriginalName: "duplicate.txt",
		StoredPath:   filepath.ToSlash(filepath.Rel(root, duplicatePath)),
		Category:     "documents/txt",
		MimeType:     "text/plain",
		Size:         int64(len(content1)),
		UploadedAt:   time.Now().UTC(),
	}
	items = append(items, duplicateMeta)

	updatedData, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	if err := os.WriteFile(metadataPath, updatedData, 0644); err != nil {
		t.Fatalf("failed to write metadata: %v", err)
	}

	// Reload server to pick up new metadata
	server, err = api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to reload server: %v", err)
	}

	// Scan for duplicates
	scanReq := httptest.NewRequest("POST", "/files/duplicates/scan", bytes.NewReader([]byte(`{"deep_scan": false, "include_metadata": true}`)))
	scanReq.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	server.Router().ServeHTTP(w3, scanReq)

	if w3.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w3.Code, w3.Body.String())
	}

	var scanResult map[string]interface{}
	if err := json.Unmarshal(w3.Body.Bytes(), &scanResult); err != nil {
		t.Fatalf("failed to parse scan result: %v", err)
	}

	if scanResult["status"] != "completed" {
		t.Errorf("expected status 'completed', got %v", scanResult["status"])
	}

	duplicatesFound := int(scanResult["duplicates_found"].(float64))
	if duplicatesFound < 1 {
		t.Errorf("expected at least 1 duplicate, got %d", duplicatesFound)
	}

	// Get duplicate report
	reportReq := httptest.NewRequest("GET", "/files/duplicates", nil)
	w4 := httptest.NewRecorder()
	server.Router().ServeHTTP(w4, reportReq)

	if w4.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w4.Code, w4.Body.String())
	}

	var report map[string]interface{}
	if err := json.Unmarshal(w4.Body.Bytes(), &report); err != nil {
		t.Fatalf("failed to parse report: %v", err)
	}

	groups := report["duplicate_groups"].([]interface{})
	if len(groups) < 1 {
		t.Error("expected at least 1 duplicate group in report")
	}
}

func TestDuplicateVerifyE2E(t *testing.T) {
	root := t.TempDir()
	cfg := config.Config{
		DataDir:      root,
		MaxUploadBytes: 100 * 1024 * 1024,
	}
	logger := slog.Default()

	server, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Upload a file
	content := []byte("test content")
	req := httptest.NewRequest("POST", "/ingest/media", createMultipartBody("test.txt", content))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=testboundary")
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	// Create orphaned file
	orphanPath := filepath.Join(root, "storage", "documents", "txt", "orphan.txt")
	if err := os.MkdirAll(filepath.Dir(orphanPath), 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(orphanPath, []byte("orphan"), 0644); err != nil {
		t.Fatalf("failed to write orphan: %v", err)
	}

	// Verify system
	verifyReq := httptest.NewRequest("POST", "/files/duplicates/verify", nil)
	w2 := httptest.NewRecorder()
	server.Router().ServeHTTP(w2, verifyReq)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w2.Code, w2.Body.String())
	}

	var verifyResult map[string]interface{}
	if err := json.Unmarshal(w2.Body.Bytes(), &verifyResult); err != nil {
		t.Fatalf("failed to parse verify result: %v", err)
	}

	orphaned := int(verifyResult["orphaned_files"].(float64))
	if orphaned < 1 {
		t.Errorf("expected at least 1 orphaned file, got %d", orphaned)
	}
}

func TestDuplicateMergeE2E(t *testing.T) {
	root := t.TempDir()
	cfg := config.Config{
		DataDir:      root,
		MaxUploadBytes: 100 * 1024 * 1024,
	}
	logger := slog.Default()

	server, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Upload file
	content := []byte("duplicate content")
	req := httptest.NewRequest("POST", "/ingest/media", createMultipartBody("file1.txt", content))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=testboundary")
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	stored := response["stored"].([]interface{})
	firstFile := stored[0].(map[string]interface{})
	hash := firstFile["hash"].(string)
	storedPath := firstFile["path"].(string)

	// Create duplicate
	duplicatePath := filepath.Join(root, "storage", "documents", "txt", hash[:12]+"_duplicate.txt")
	if err := os.MkdirAll(filepath.Dir(duplicatePath), 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(duplicatePath, content, 0644); err != nil {
		t.Fatalf("failed to write duplicate: %v", err)
	}

	// Add to metadata
	metadataPath := filepath.Join(root, "metadata", "files.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("failed to read metadata: %v", err)
	}

	var items []storage.FileMetadata
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	duplicateMeta := storage.FileMetadata{
		Hash:         hash,
		OriginalName: "duplicate.txt",
		StoredPath:   filepath.ToSlash(filepath.Rel(root, duplicatePath)),
		Category:     "documents/txt",
		MimeType:     "text/plain",
		Size:         int64(len(content)),
		UploadedAt:   time.Now().UTC(),
	}
	items = append(items, duplicateMeta)

	updatedData, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	if err := os.WriteFile(metadataPath, updatedData, 0644); err != nil {
		t.Fatalf("failed to write metadata: %v", err)
	}

	// Reload server
	server, err = api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Merge duplicates
	mergeBody := fmt.Sprintf(`{"hash": "%s", "keep": "%s", "remove_others": true}`, hash, storedPath)
	mergeReq := httptest.NewRequest("POST", "/files/duplicates/merge", bytes.NewReader([]byte(mergeBody)))
	mergeReq.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	server.Router().ServeHTTP(w2, mergeReq)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w2.Code, w2.Body.String())
	}

	// Verify duplicate is removed
	if _, err := os.Stat(duplicatePath); err == nil {
		t.Error("duplicate file should have been removed")
	}
}

func TestDuplicateStatisticsE2E(t *testing.T) {
	root := t.TempDir()
	cfg := config.Config{
		DataDir:      root,
		MaxUploadBytes: 100 * 1024 * 1024,
	}
	logger := slog.Default()

	server, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Get statistics
	statsReq := httptest.NewRequest("GET", "/files/duplicates/statistics", nil)
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, statsReq)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var stats map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatalf("failed to parse stats: %v", err)
	}

	if stats["duplicate_groups"] == nil {
		t.Error("expected duplicate_groups in statistics")
	}
}

// Helper function to create multipart form data
func createMultipartBody(filename string, content []byte) *bytes.Buffer {
	var buf bytes.Buffer
	buf.WriteString("--testboundary\r\n")
	buf.WriteString(fmt.Sprintf(`Content-Disposition: form-data; name="file"; filename="%s"`+"\r\n", filename))
	buf.WriteString("Content-Type: application/octet-stream\r\n\r\n")
	buf.Write(content)
	buf.WriteString("\r\n--testboundary--\r\n")
	return &buf
}

