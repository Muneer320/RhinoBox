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

// TestFilesByTypeEndToEnd tests the complete flow: upload files -> retrieve by type -> verify pagination
func TestFilesByTypeEndToEnd(t *testing.T) {
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

	// Step 1: Upload files of different types
	imageHashes := make([]string, 0)
	videoHashes := make([]string, 0)

	// Upload 3 images
	for i := 0; i < 3; i++ {
		filename := fmt.Sprintf("test_image_%d.jpg", i)
		content := []byte(fmt.Sprintf("fake image content %d", i))
		hash, _ := uploadTestFile(t, srv, filename, content, "image/jpeg")
		imageHashes = append(imageHashes, hash)
	}

	// Upload 2 videos
	for i := 0; i < 2; i++ {
		filename := fmt.Sprintf("test_video_%d.mp4", i)
		content := []byte(fmt.Sprintf("fake video content %d", i))
		hash, _ := uploadTestFile(t, srv, filename, content, "video/mp4")
		videoHashes = append(videoHashes, hash)
	}

	// Step 2: Retrieve images
	resp := getFilesByType(t, srv, "images", 1, 10, "")
	if resp["type"].(string) != "images" {
		t.Errorf("expected type 'images', got %s", resp["type"].(string))
	}

	files := resp["files"].([]any)
	if len(files) != 3 {
		t.Errorf("expected 3 images, got %d", len(files))
	}

	total := int(resp["total"].(float64))
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}

	// Verify all returned files are images (check category instead of mime type as it's more reliable)
	for _, fileAny := range files {
		file := fileAny.(map[string]any)
		category := file["category"].(string)
		if !strings.HasPrefix(category, "images/") && category != "images" {
			t.Errorf("expected image category, got %s", category)
		}

		// Verify required fields
		requiredFields := []string{"id", "name", "path", "size", "type", "date", "hash", "url"}
		for _, field := range requiredFields {
			if _, ok := file[field]; !ok {
				t.Errorf("missing required field: %s", field)
			}
		}
	}

	// Step 3: Retrieve videos
	resp = getFilesByType(t, srv, "videos", 1, 10, "")
	if resp["type"].(string) != "videos" {
		t.Errorf("expected type 'videos', got %s", resp["type"].(string))
	}

	files = resp["files"].([]any)
	if len(files) != 2 {
		t.Errorf("expected 2 videos, got %d", len(files))
	}

	// Step 4: Test pagination
	resp = getFilesByType(t, srv, "images", 1, 2, "")
	files = resp["files"].([]any)
	if len(files) != 2 {
		t.Errorf("expected 2 files on page 1, got %d", len(files))
	}

	totalPages := int(resp["total_pages"].(float64))
	if totalPages != 2 {
		t.Errorf("expected 2 total pages, got %d", totalPages)
	}

	// Get second page
	resp = getFilesByType(t, srv, "images", 2, 2, "")
	files = resp["files"].([]any)
	if len(files) != 1 {
		t.Errorf("expected 1 file on page 2, got %d", len(files))
	}
}

// TestFilesByTypeWithCategory tests filtering by category
func TestFilesByTypeWithCategory(t *testing.T) {
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

	// Upload images with different categories and unique content
	categories := []string{"vacation", "vacation", "work"}
	for i, cat := range categories {
		filename := fmt.Sprintf("image_%d.jpg", i)
		content := []byte(fmt.Sprintf("image content %d", i))
		uploadTestFileWithCategory(t, srv, filename, content, "image/jpeg", cat)
	}

	// Filter by vacation category
	resp := getFilesByType(t, srv, "images", 1, 10, "vacation")
	files := resp["files"].([]any)
	if len(files) != 2 {
		t.Errorf("expected 2 files with vacation category, got %d", len(files))
	}
}

// TestFilesByTypeNonExistent tests behavior with non-existent type
func TestFilesByTypeNonExistent(t *testing.T) {
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

	// Request non-existent type
	resp := getFilesByType(t, srv, "nonexistent", 1, 10, "")
	files := resp["files"].([]any)
	if len(files) != 0 {
		t.Errorf("expected 0 files for nonexistent type, got %d", len(files))
	}

	total := int(resp["total"].(float64))
	if total != 0 {
		t.Errorf("expected total 0, got %d", total)
	}
}

// Helper function to get files by type
func getFilesByType(t *testing.T, srv *api.Server, fileType string, page, limit int, category string) map[string]any {
	t.Helper()

	url := fmt.Sprintf("/files/type/%s?page=%d&limit=%d", fileType, page, limit)
	if category != "" {
		url += fmt.Sprintf("&category=%s", category)
	}

	req := httptest.NewRequest(http.MethodGet, url, nil)
	resp := httptest.NewRecorder()
	srv.Router().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return result
}

// Helper function to upload file with category
func uploadTestFileWithCategory(t *testing.T, srv *api.Server, filename string, content []byte, mimeType, category string) (string, string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	fileWriter.Write(content)
	if category != "" {
		writer.WriteField("category", category)
	}
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()
	srv.Router().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("upload failed: %d - %s", resp.Code, resp.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	stored := result["stored"].([]any)
	if len(stored) == 0 {
		t.Fatal("no files stored")
	}

	fileData := stored[0].(map[string]any)
	hash := fileData["hash"].(string)
	path := fileData["path"].(string)

	return hash, path
}

