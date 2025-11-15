package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
	"log/slog"
)

// TestFileRetrievalEndToEnd tests the complete flow: upload -> retrieve -> download
func TestFileRetrievalEndToEnd(t *testing.T) {
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
	testContent := []byte("This is a test file for retrieval testing. " + strings.Repeat("Content ", 100))
	testFilename := "test_retrieval.txt"

	// Step 1: Upload file
	uploadHash, _ := uploadTestFile(t, srv, testFilename, testContent, "text/plain")

	// Step 2: Get metadata
	metadata := getFileMetadata(t, srv, uploadHash)
	if metadata["hash"].(string) != uploadHash {
		t.Errorf("expected hash %s, got %s", uploadHash, metadata["hash"].(string))
	}
	if metadata["original_name"].(string) != testFilename {
		t.Errorf("expected filename %s, got %s", testFilename, metadata["original_name"].(string))
	}

	// Step 3: Download by hash
	downloadedContent := downloadFileByHash(t, srv, uploadHash)
	if !bytes.Equal(downloadedContent, testContent) {
		t.Errorf("downloaded content mismatch: expected %d bytes, got %d bytes", len(testContent), len(downloadedContent))
	}

	// Step 4: Download by path (re-upload to get path)
	_, uploadPath := uploadTestFile(t, srv, testFilename+"_path", testContent, "text/plain")
	downloadedByPath := downloadFileByPath(t, srv, uploadPath)
	if !bytes.Equal(downloadedByPath, testContent) {
		t.Errorf("downloaded by path content mismatch")
	}

	// Step 5: Stream with range request
	partialContent := streamFileRange(t, srv, uploadHash, 0, 50)
	expectedPartial := testContent[0:51]
	if !bytes.Equal(partialContent, expectedPartial) {
		t.Errorf("partial content mismatch: expected %d bytes, got %d bytes", len(expectedPartial), len(partialContent))
	}

	// Step 6: Get file by ID (new endpoint)
	fileInfo := getFileByID(t, srv, uploadHash)
	if fileInfo["hash"].(string) != uploadHash {
		t.Errorf("expected hash %s, got %s", uploadHash, fileInfo["hash"].(string))
	}
	if fileInfo["original_name"].(string) != testFilename {
		t.Errorf("expected filename %s, got %s", testFilename, fileInfo["original_name"].(string))
	}
	// Verify download_url and stream_url are present
	if _, ok := fileInfo["download_url"]; !ok {
		t.Error("missing download_url in response")
	}
	if _, ok := fileInfo["stream_url"]; !ok {
		t.Error("missing stream_url in response")
	}
}

// TestFileRetrievalWithRealWorldFiles tests with actual files from Downloads directory
func TestFileRetrievalWithRealWorldFiles(t *testing.T) {
	downloadsDir := filepath.Join(os.Getenv("HOME"), "Downloads")
	if _, err := os.Stat(downloadsDir); os.IsNotExist(err) {
		t.Skip("Downloads directory not found, skipping real-world file test")
	}

	tmpDir := t.TempDir()
	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        tmpDir,
		MaxUploadBytes: 500 * 1024 * 1024, // 500MB
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Find a small test file (less than 1MB) from Downloads
	testFiles := findTestFiles(t, downloadsDir, 1024*1024) // 1MB max
	if len(testFiles) == 0 {
		t.Skip("No suitable test files found in Downloads directory")
	}

	for _, testFile := range testFiles[:min(3, len(testFiles))] { // Test up to 3 files
		t.Run(filepath.Base(testFile), func(t *testing.T) {
			// Read the file
			originalContent, err := os.ReadFile(testFile)
			if err != nil {
				t.Fatalf("failed to read test file: %v", err)
			}

			filename := filepath.Base(testFile)

			// Upload the file
			hash, _ := uploadRealFile(t, srv, testFile, filename)

			// Get metadata
			metadata := getFileMetadata(t, srv, hash)
			if metadata["size"].(float64) != float64(len(originalContent)) {
				t.Errorf("size mismatch: expected %d, got %.0f", len(originalContent), metadata["size"].(float64))
			}

			// Download and verify content
			downloaded := downloadFileByHash(t, srv, hash)
			if !bytes.Equal(downloaded, originalContent) {
				t.Errorf("content mismatch for file %s", filename)
			}

			// Test range request (first 1KB)
			if len(originalContent) > 1024 {
				partial := streamFileRange(t, srv, hash, 0, 1023)
				expected := originalContent[0:1024]
				if !bytes.Equal(partial, expected) {
					t.Errorf("partial content mismatch for file %s", filename)
				}
			}
		})
	}
}

// TestFileRetrievalNotFound tests error handling for non-existent files
func TestFileRetrievalNotFound(t *testing.T) {
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

	// Test non-existent hash
	req := httptest.NewRequest("GET", "/files/download?hash=nonexistent_hash_1234567890123456789012345678901234567890123456789012345678901234", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	// Test non-existent path
	req = httptest.NewRequest("GET", "/files/download?path=nonexistent/path/file.txt", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	// Test metadata for non-existent file
	req = httptest.NewRequest("GET", "/files/metadata?hash=nonexistent_hash_1234567890123456789012345678901234567890123456789012345678901234", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	// Test get file by ID for non-existent file
	req = httptest.NewRequest("GET", "/files/nonexistent_hash_1234567890123456789012345678901234567890123456789012345678901234", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for get file by ID, got %d", w.Code)
	}
}

// TestFileRetrievalPathTraversal tests security against path traversal attacks
func TestFileRetrievalPathTraversal(t *testing.T) {
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

	maliciousPaths := []string{
		"../etc/passwd",
		"../../etc/passwd",
		"storage/../../../etc/passwd",
		"..\\windows\\system32",
		"/etc/passwd",
	}

	for _, path := range maliciousPaths {
		req := httptest.NewRequest("GET", fmt.Sprintf("/files/download?path=%s", path), nil)
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
			t.Errorf("expected 400 or 404 for path %s, got %d", path, w.Code)
		}
	}
}

// TestFileStreamRangeRequests tests various range request scenarios
func TestFileStreamRangeRequests(t *testing.T) {
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

	// Create a larger test file (10KB)
	testContent := bytes.Repeat([]byte("A"), 10*1024)
	hash, _ := uploadTestFile(t, srv, "range_test.txt", testContent, "text/plain")

	// Test: Full range (0-10239)
	fullRange := streamFileWithRangeHeader(t, srv, hash, "bytes=0-10239")
	if len(fullRange) != 10*1024 {
		t.Errorf("full range: expected %d bytes, got %d", 10*1024, len(fullRange))
	}

	// Test: First 1KB (0-1023)
	firstKB := streamFileWithRangeHeader(t, srv, hash, "bytes=0-1023")
	if len(firstKB) != 1024 {
		t.Errorf("first KB: expected 1024 bytes, got %d", len(firstKB))
	}
	if !bytes.Equal(firstKB, testContent[0:1024]) {
		t.Error("first KB content mismatch")
	}

	// Test: Middle range (2048-4095)
	middleRange := streamFileWithRangeHeader(t, srv, hash, "bytes=2048-4095")
	if len(middleRange) != 2048 {
		t.Errorf("middle range: expected 2048 bytes, got %d", len(middleRange))
	}
	if !bytes.Equal(middleRange, testContent[2048:4096]) {
		t.Error("middle range content mismatch")
	}

	// Test: Open-ended range (2048-)
	openEnded := streamFileWithRangeHeader(t, srv, hash, "bytes=2048-")
	if len(openEnded) != 10*1024-2048 {
		t.Errorf("open-ended range: expected %d bytes, got %d", 10*1024-2048, len(openEnded))
	}
	if !bytes.Equal(openEnded, testContent[2048:]) {
		t.Error("open-ended range content mismatch")
	}

	// Test: Invalid range (start > end)
	req := httptest.NewRequest("GET", fmt.Sprintf("/files/stream?hash=%s", hash), nil)
	req.Header.Set("Range", "bytes=5000-1000")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("expected 416 for invalid range, got %d", w.Code)
	}

	// Test: Range beyond file size
	req = httptest.NewRequest("GET", fmt.Sprintf("/files/stream?hash=%s", hash), nil)
	req.Header.Set("Range", "bytes=20000-30000")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("expected 416 for range beyond file size, got %d", w.Code)
	}
}

// Helper functions

func uploadTestFile(t *testing.T, srv *api.Server, filename string, content []byte, mimeType string) (string, string) {
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

	stored := response["stored"].([]interface{})
	if len(stored) == 0 {
		t.Fatal("no files stored")
	}

	fileInfo := stored[0].(map[string]interface{})
	hash := fileInfo["hash"].(string)
	path := fileInfo["path"].(string)

	return hash, path
}

func uploadRealFile(t *testing.T, srv *api.Server, filePath, filename string) (string, string) {
	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Open file: %v", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		t.Fatalf("Copy: %v", err)
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

	stored := response["stored"].([]interface{})
	if len(stored) == 0 {
		t.Fatal("no files stored")
	}

	fileInfo := stored[0].(map[string]interface{})
	hash := fileInfo["hash"].(string)
	path := fileInfo["path"].(string)

	return hash, path
}

func getFileMetadata(t *testing.T, srv *api.Server, hash string) map[string]interface{} {
	req := httptest.NewRequest("GET", fmt.Sprintf("/files/metadata?hash=%s", hash), nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("get metadata failed: status %d, body: %s", w.Code, w.Body.String())
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &metadata); err != nil {
		t.Fatalf("Unmarshal metadata: %v", err)
	}

	return metadata
}

func downloadFileByHash(t *testing.T, srv *api.Server, hash string) []byte {
	req := httptest.NewRequest("GET", fmt.Sprintf("/files/download?hash=%s", hash), nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("download failed: status %d, body: %s", w.Code, w.Body.String())
	}

	return w.Body.Bytes()
}

func downloadFileByPath(t *testing.T, srv *api.Server, path string) []byte {
	req := httptest.NewRequest("GET", fmt.Sprintf("/files/download?path=%s", path), nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("download failed: status %d, body: %s", w.Code, w.Body.String())
	}

	return w.Body.Bytes()
}

func streamFileRange(t *testing.T, srv *api.Server, hash string, start, end int64) []byte {
	return streamFileWithRangeHeader(t, srv, hash, fmt.Sprintf("bytes=%d-%d", start, end))
}

func streamFileWithRangeHeader(t *testing.T, srv *api.Server, hash, rangeHeader string) []byte {
	req := httptest.NewRequest("GET", fmt.Sprintf("/files/stream?hash=%s", hash), nil)
	req.Header.Set("Range", rangeHeader)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusPartialContent && w.Code != http.StatusOK {
		t.Fatalf("stream failed: status %d, body: %s", w.Code, w.Body.String())
	}

	return w.Body.Bytes()
}

func findTestFiles(t *testing.T, dir string, maxSize int64) []string {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.Size() > 0 && info.Size() <= maxSize {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}

	return files
}

func getFileByID(t *testing.T, srv *api.Server, fileID string) map[string]interface{} {
	req := httptest.NewRequest("GET", fmt.Sprintf("/files/%s", fileID), nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("get file by ID failed: status %d, body: %s", w.Code, w.Body.String())
	}

	var fileInfo map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &fileInfo); err != nil {
		t.Fatalf("Unmarshal file info: %v", err)
	}

	return fileInfo
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
