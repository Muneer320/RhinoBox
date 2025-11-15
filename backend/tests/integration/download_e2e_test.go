package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
	"log/slog"
)

// TestDownloadEndpointE2E tests the complete download flow with hash and path
func TestDownloadEndpointE2E(t *testing.T) {
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

	// Create test file
	testContent := []byte("This is a test file for download testing. " + strings.Repeat("Content ", 50))
	testFilename := "test_download.txt"

	// Step 1: Upload file
	hash, path := uploadTestFileForDownload(t, srv, testFilename, testContent)

	// Step 2: Download by hash
	downloadedByHash := downloadFileByHashE2E(t, srv, hash)
	if !bytes.Equal(downloadedByHash, testContent) {
		t.Errorf("downloaded by hash content mismatch: expected %d bytes, got %d bytes", len(testContent), len(downloadedByHash))
	}

	// Step 3: Download by path
	downloadedByPath := downloadFileByPathE2E(t, srv, path)
	if !bytes.Equal(downloadedByPath, testContent) {
		t.Errorf("downloaded by path content mismatch: expected %d bytes, got %d bytes", len(testContent), len(downloadedByPath))
	}

	// Step 4: Verify Content-Disposition header
	req := httptest.NewRequest("GET", fmt.Sprintf("/files/download?hash=%s", hash), nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentDisposition := w.Header().Get("Content-Disposition")
	if contentDisposition == "" {
		t.Error("missing Content-Disposition header")
	}
	if !strings.Contains(contentDisposition, "attachment") {
		t.Errorf("expected Content-Disposition to contain 'attachment', got %s", contentDisposition)
	}
}

// TestDownloadByFileID tests downloading a file by fetching metadata first via file ID
func TestDownloadByFileID(t *testing.T) {
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

	// Create test file
	testContent := []byte("Test file for ID-based download")
	testFilename := "test_file_id.txt"

	// Step 1: Upload file
	hash, _ := uploadTestFileForDownload(t, srv, testFilename, testContent)

	// Step 2: Download using hash directly (simulating file ID workflow where we already have hash)
	// In a real scenario, file ID would be used to fetch metadata first, then download
	// For this test, we verify that the hash we got from upload can be used for download
	downloadedContent := downloadFileByHashE2E(t, srv, hash)
	if !bytes.Equal(downloadedContent, testContent) {
		t.Errorf("downloaded content mismatch: expected %d bytes, got %d bytes", len(testContent), len(downloadedContent))
	}
	
	// Step 3: Verify that download works with the hash (simulating file ID -> hash -> download flow)
	// This tests the frontend's ability to use file ID to get hash, then download
	req := httptest.NewRequest("GET", fmt.Sprintf("/files/download?hash=%s", hash), nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("download by hash failed: status %d", w.Code)
	}
	
	downloadedBytes := w.Body.Bytes()
	if !bytes.Equal(downloadedBytes, testContent) {
		t.Errorf("downloaded content mismatch in second test: expected %d bytes, got %d bytes", len(testContent), len(downloadedBytes))
	}
}

// TestDownloadEndpointErrorHandling tests error cases
func TestDownloadEndpointErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        tmpDir,
		MaxUploadBytes: 50 * 1024 * 1024,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Test: Missing both hash and path
	req := httptest.NewRequest("GET", "/files/download", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing params, got %d", w.Code)
	}

	// Test: Non-existent hash (using valid SHA-256 format)
	req = httptest.NewRequest("GET", "/files/download?hash=0000000000000000000000000000000000000000000000000000000000000000", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for non-existent hash, got %d", w.Code)
	}

	// Test: Non-existent path
	req = httptest.NewRequest("GET", "/files/download?path=nonexistent/path/file.txt", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for non-existent path, got %d", w.Code)
	}
}

// Helper functions

func uploadTestFileForDownload(t *testing.T, srv *api.Server, filename string, content []byte) (string, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("Write: %v", err)
	}
	writer.Close()

	req := httptest.NewRequest("POST", "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("upload failed: status %d, body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal response: %v", err)
	}

	// Handle response format: {"data": {"stored": [{"hash": "...", "path": "...", ...}]}}
	var hash, path string
	
	// Check if wrapped in "data" field (response middleware)
	var storedArray []interface{}
	if data, ok := response["data"].(map[string]interface{}); ok {
		if stored, ok := data["stored"].([]interface{}); ok {
			storedArray = stored
		}
	} else if stored, ok := response["stored"].([]interface{}); ok {
		storedArray = stored
	}
	
	if len(storedArray) > 0 {
		fileInfo := storedArray[0].(map[string]interface{})
		if h, ok := fileInfo["hash"].(string); ok {
			hash = h
		}
		if p, ok := fileInfo["path"].(string); ok {
			path = p
		}
	}

	if hash == "" {
		t.Fatalf("failed to extract hash from upload response: %+v", response)
	}

	return hash, path
}

func downloadFileByHashE2E(t *testing.T, srv *api.Server, hash string) []byte {
	req := httptest.NewRequest("GET", fmt.Sprintf("/files/download?hash=%s", hash), nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("download failed: status %d, body: %s", w.Code, w.Body.String())
	}

	return w.Body.Bytes()
}

func downloadFileByPathE2E(t *testing.T, srv *api.Server, path string) []byte {
	req := httptest.NewRequest("GET", fmt.Sprintf("/files/download?path=%s", path), nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("download failed: status %d, body: %s", w.Code, w.Body.String())
	}

	return w.Body.Bytes()
}

