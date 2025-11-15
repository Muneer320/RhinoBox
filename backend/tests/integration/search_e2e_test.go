package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
	"github.com/Muneer320/RhinoBox/internal/storage"
	"log/slog"
)

// TestFileSearchE2E_RealWorldFiles tests the search endpoint with real files from Downloads directory
func TestFileSearchE2E_RealWorldFiles(t *testing.T) {
	downloadsDir := filepath.Join(os.Getenv("HOME"), "Downloads")
	if _, err := os.Stat(downloadsDir); os.IsNotExist(err) {
		t.Skipf("Downloads directory not found: %s", downloadsDir)
	}

	// Create test server
	cfg := config.Config{
		Addr:           ":0",
		DataDir:        t.TempDir(),
		MaxUploadBytes: 100 * 1024 * 1024, // 100MB
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Find real files in Downloads directory
	testFiles := findSearchTestFiles(t, downloadsDir, 10) // Get up to 10 files
	if len(testFiles) == 0 {
		t.Skip("no suitable test files found in Downloads directory")
	}

	t.Logf("Found %d test files to upload", len(testFiles))

	// Upload files and collect metadata
	uploadedFiles := make([]storage.FileMetadata, 0)
	for _, filePath := range testFiles {
		file, err := os.Open(filePath)
		if err != nil {
			t.Logf("skipping file %s: %v", filePath, err)
			continue
		}
		defer file.Close()

		stat, err := file.Stat()
		if err != nil {
			t.Logf("skipping file %s: %v", filePath, err)
			continue
		}

		// Upload file
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fileWriter, err := writer.CreateFormFile("file", stat.Name())
		if err != nil {
			t.Logf("failed to create form file for %s: %v", filePath, err)
			continue
		}

		if _, err := io.Copy(fileWriter, file); err != nil {
			t.Logf("failed to copy file %s: %v", filePath, err)
			continue
		}
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()
		srv.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Logf("upload failed for %s: %d: %s", filePath, resp.Code, resp.Body.String())
			continue
		}

		var uploadResp struct {
			Stored []map[string]any `json:"stored"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
			t.Logf("failed to decode upload response for %s: %v", filePath, err)
			continue
		}

		if len(uploadResp.Stored) > 0 {
			// Get metadata
			hash, _ := uploadResp.Stored[0]["hash"].(string)
			if hash != "" {
				// Retrieve full metadata
				metaReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/files/metadata?hash=%s", hash), nil)
				metaResp := httptest.NewRecorder()
				srv.Router().ServeHTTP(metaResp, metaReq)

				if metaResp.Code == http.StatusOK {
					var meta storage.FileMetadata
					if err := json.NewDecoder(metaResp.Body).Decode(&meta); err == nil {
						uploadedFiles = append(uploadedFiles, meta)
					}
				}
			}
		}
	}

	if len(uploadedFiles) == 0 {
		t.Fatal("no files were successfully uploaded")
	}

	t.Logf("Successfully uploaded %d files", len(uploadedFiles))

	// Test various search scenarios
	t.Run("search_by_extension", func(t *testing.T) {
		// Find most common extension
		extCounts := make(map[string]int)
		for _, f := range uploadedFiles {
			ext := strings.ToLower(filepath.Ext(f.OriginalName))
			if ext != "" {
				extCounts[ext]++
			}
		}

		var mostCommonExt string
		maxCount := 0
		for ext, count := range extCounts {
			if count > maxCount {
				maxCount = count
				mostCommonExt = ext
			}
		}

		if mostCommonExt == "" {
			t.Skip("no files with extensions found")
		}

		ext := strings.TrimPrefix(mostCommonExt, ".")
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/files/search?extension=%s", ext), nil)
		resp := httptest.NewRecorder()
		srv.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}

		var payload struct {
			Results []storage.FileMetadata `json:"results"`
			Count   int                    `json:"count"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if payload.Count < maxCount {
			t.Errorf("expected at least %d results for extension %s, got %d", maxCount, ext, payload.Count)
		}

		t.Logf("Found %d files with extension %s", payload.Count, ext)
	})

	t.Run("search_by_name_partial", func(t *testing.T) {
		// Use first file's name for partial search
		if len(uploadedFiles) == 0 {
			t.Skip("no files uploaded")
		}

		firstFileName := uploadedFiles[0].OriginalName
		// Extract a meaningful part of the filename (first 5-10 chars)
		searchTerm := firstFileName
		if len(searchTerm) > 10 {
			searchTerm = searchTerm[:10]
		}

		searchTermEncoded := url.QueryEscape(searchTerm)
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/files/search?name=%s", searchTermEncoded), nil)
		resp := httptest.NewRecorder()
		srv.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}

		var payload struct {
			Results []storage.FileMetadata `json:"results"`
			Count   int                    `json:"count"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if payload.Count == 0 {
			t.Errorf("expected at least 1 result for name search '%s', got 0", searchTerm)
		}

		t.Logf("Found %d files matching name '%s'", payload.Count, searchTerm)
	})

	t.Run("search_by_type", func(t *testing.T) {
		// Search for images
		req := httptest.NewRequest(http.MethodGet, "/files/search?type=image", nil)
		resp := httptest.NewRecorder()
		srv.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}

		var payload struct {
			Results []storage.FileMetadata `json:"results"`
			Count   int                    `json:"count"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		t.Logf("Found %d image files", payload.Count)

		// Search for PDFs/documents
		req2 := httptest.NewRequest(http.MethodGet, "/files/search?type=application/pdf", nil)
		resp2 := httptest.NewRecorder()
		srv.Router().ServeHTTP(resp2, req2)

		if resp2.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp2.Code, resp2.Body.String())
		}

		var payload2 struct {
			Results []storage.FileMetadata `json:"results"`
			Count   int                    `json:"count"`
		}
		if err := json.NewDecoder(resp2.Body).Decode(&payload2); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		t.Logf("Found %d PDF files", payload2.Count)
	})

	t.Run("search_by_date_range", func(t *testing.T) {
		now := time.Now()
		yesterday := now.AddDate(0, 0, -1)
		tomorrow := now.AddDate(0, 0, 1)

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/files/search?date_from=%s&date_to=%s",
			yesterday.Format("2006-01-02"), tomorrow.Format("2006-01-02")), nil)
		resp := httptest.NewRecorder()
		srv.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}

		var payload struct {
			Results []storage.FileMetadata `json:"results"`
			Count   int                    `json:"count"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if payload.Count < len(uploadedFiles) {
			t.Logf("Expected at least %d files in date range, got %d (some may have different upload times)", len(uploadedFiles), payload.Count)
		}

		t.Logf("Found %d files in date range", payload.Count)
	})

	t.Run("search_combined_filters", func(t *testing.T) {
		// Combine name and extension filters
		if len(uploadedFiles) == 0 {
			t.Skip("no files uploaded")
		}

		firstFile := uploadedFiles[0]
		namePart := firstFile.OriginalName
		if len(namePart) > 8 {
			namePart = namePart[:8]
		}
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(firstFile.OriginalName)), ".")

		if ext == "" {
			t.Skip("file has no extension")
		}

		namePartEncoded := url.QueryEscape(namePart)
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/files/search?name=%s&extension=%s", namePartEncoded, ext), nil)
		resp := httptest.NewRecorder()
		srv.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}

		var payload struct {
			Results []storage.FileMetadata `json:"results"`
			Count   int                    `json:"count"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		t.Logf("Found %d files matching combined filters (name: %s, ext: %s)", payload.Count, namePart, ext)
	})
}

// findSearchTestFiles searches for test files in the Downloads directory
func findSearchTestFiles(t *testing.T, dir string, maxFiles int) []string {
	var files []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Logf("failed to read Downloads directory: %v", err)
		return files
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if len(files) >= maxFiles {
			break
		}

		name := entry.Name()
		// Skip hidden files and system files
		if strings.HasPrefix(name, ".") {
			continue
		}

		// Prefer common file types
		ext := strings.ToLower(filepath.Ext(name))
		validExts := map[string]bool{
			".pdf":  true,
			".jpg":  true,
			".jpeg": true,
			".png":  true,
			".gif":  true,
			".txt":  true,
			".doc":  true,
			".docx": true,
			".xls":  true,
			".xlsx": true,
			".zip":  true,
		}

		if validExts[ext] || len(files) < 3 {
			// Include at least 3 files regardless of extension
			fullPath := filepath.Join(dir, name)
			stat, err := os.Stat(fullPath)
			if err != nil {
				continue
			}

			// Skip very large files (>50MB) to avoid test timeouts
			if stat.Size() > 50*1024*1024 {
				continue
			}

			files = append(files, fullPath)
		}
	}

	return files
}
