package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Muneer320/RhinoBox/internal/storage"
)

func TestFileList_Basic(t *testing.T) {
	srv := newTestServer(t)

	// Upload test files
	files := []struct {
		name    string
		content string
		mime    string
	}{
		{"test1.pdf", "pdf content", "application/pdf"},
		{"test2.jpg", "jpg content", "image/jpeg"},
		{"test3.png", "png content", "image/png"},
	}

	for _, f := range files {
		uploadTestFile(t, srv, f.name, f.content, f.mime)
	}

	// Test basic listing
	req := httptest.NewRequest(http.MethodGet, "/files", nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result struct {
		Files      []storage.FileMetadata `json:"files"`
		Pagination struct {
			Page       int  `json:"page"`
			Limit      int  `json:"limit"`
			Total      int  `json:"total"`
			TotalPages int  `json:"total_pages"`
			HasNext    bool `json:"has_next"`
			HasPrev    bool `json:"has_prev"`
		} `json:"pagination"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Pagination.Total < len(files) {
		t.Errorf("expected at least %d files, got %d", len(files), result.Pagination.Total)
	}

	if len(result.Files) == 0 {
		t.Error("expected at least one file in response")
	}
}

func TestFileList_Pagination(t *testing.T) {
	srv := newTestServer(t)

	// Upload 10 test files
	for i := 0; i < 10; i++ {
		uploadTestFile(t, srv, "file"+string(rune('0'+i))+".txt", "content", "text/plain")
	}

	// Wait a bit for files to be indexed
	time.Sleep(100 * time.Millisecond)

	// Test page 1 with limit 3
	req := httptest.NewRequest(http.MethodGet, "/files?page=1&limit=3", nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result struct {
		Files      []storage.FileMetadata `json:"files"`
		Pagination struct {
			Page       int  `json:"page"`
			Limit      int  `json:"limit"`
			Total      int  `json:"total"`
			TotalPages int  `json:"total_pages"`
			HasNext    bool `json:"has_next"`
			HasPrev    bool `json:"has_prev"`
		} `json:"pagination"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Check that we have at least 3 files total
	if result.Pagination.Total < 3 {
		t.Skipf("not enough files for pagination test (got %d, need at least 3)", result.Pagination.Total)
	}

	if len(result.Files) > result.Pagination.Limit {
		t.Errorf("expected at most %d files on page 1, got %d", result.Pagination.Limit, len(result.Files))
	}

	if result.Pagination.Total > result.Pagination.Limit && !result.Pagination.HasNext {
		t.Error("expected has_next to be true when there are more pages")
	}

	// Test page 2 only if we have enough files
	if result.Pagination.TotalPages >= 2 {
		req2 := httptest.NewRequest(http.MethodGet, "/files?page=2&limit=3", nil)
		resp2 := httptest.NewRecorder()
		srv.router.ServeHTTP(resp2, req2)

		if resp2.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp2.Code, resp2.Body.String())
		}

		var result2 struct {
			Pagination struct {
				HasNext bool `json:"has_next"`
				HasPrev bool `json:"has_prev"`
			} `json:"pagination"`
		}

		if err := json.NewDecoder(resp2.Body).Decode(&result2); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if !result2.Pagination.HasPrev {
			t.Error("expected has_prev to be true on page 2")
		}
	}
}

func TestFileList_Filtering(t *testing.T) {
	srv := newTestServer(t)

	// Upload files with different types
	uploadTestFile(t, srv, "img1.jpg", "content", "image/jpeg")
	uploadTestFile(t, srv, "img2.png", "content", "image/png")
	uploadTestFile(t, srv, "doc1.pdf", "content", "application/pdf")

	// Wait for files to be indexed
	time.Sleep(100 * time.Millisecond)

	// Test category filter
	req := httptest.NewRequest(http.MethodGet, "/files?category=images", nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result struct {
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Pagination.Total < 1 {
		t.Errorf("expected at least 1 image file, got %d", result.Pagination.Total)
	}

	// Test type filter
	req2 := httptest.NewRequest(http.MethodGet, "/files?type=image", nil)
	resp2 := httptest.NewRecorder()
	srv.router.ServeHTTP(resp2, req2)

	if resp2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp2.Code, resp2.Body.String())
	}

	var result2 struct {
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}

	if err := json.NewDecoder(resp2.Body).Decode(&result2); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result2.Pagination.Total < 1 {
		t.Errorf("expected at least 1 image file by type, got %d", result2.Pagination.Total)
	}

	// Test extension filter
	req4 := httptest.NewRequest(http.MethodGet, "/files?extension=jpg", nil)
	resp4 := httptest.NewRecorder()
	srv.router.ServeHTTP(resp4, req4)

	if resp4.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp4.Code, resp4.Body.String())
	}

	var result4 struct {
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}

	if err := json.NewDecoder(resp4.Body).Decode(&result4); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result4.Pagination.Total < 1 {
		t.Errorf("expected at least 1 JPG file, got %d", result4.Pagination.Total)
	}

	// Test name filter
	req5 := httptest.NewRequest(http.MethodGet, "/files?name=img", nil)
	resp5 := httptest.NewRecorder()
	srv.router.ServeHTTP(resp5, req5)

	if resp5.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp5.Code, resp5.Body.String())
	}

	var result5 struct {
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}

	if err := json.NewDecoder(resp5.Body).Decode(&result5); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result5.Pagination.Total < 1 {
		t.Errorf("expected at least 1 file with 'img' in name, got %d", result5.Pagination.Total)
	}
}

func TestFileList_Sorting(t *testing.T) {
	srv := newTestServer(t)

	// Upload files with different names
	uploadTestFile(t, srv, "zebra.txt", "content", "text/plain")
	uploadTestFile(t, srv, "alpha.txt", "content", "text/plain")
	uploadTestFile(t, srv, "beta.txt", "content", "text/plain")

	// Wait for files to be indexed
	time.Sleep(100 * time.Millisecond)

	// Test sort by name ascending
	req := httptest.NewRequest(http.MethodGet, "/files?sort=name&order=asc", nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result struct {
		Files []storage.FileMetadata `json:"files"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify sorting: find our test files and check order
	if len(result.Files) >= 2 {
		var alphaIdx, zebraIdx = -1, -1
		for i, f := range result.Files {
			if strings.Contains(f.OriginalName, "alpha") {
				alphaIdx = i
			}
			if strings.Contains(f.OriginalName, "zebra") {
				zebraIdx = i
			}
		}
		if alphaIdx >= 0 && zebraIdx >= 0 && alphaIdx > zebraIdx {
			t.Error("expected alpha to come before zebra when sorted ascending")
		}
	}
}

func TestFileList_DateFiltering(t *testing.T) {
	srv := newTestServer(t)

	// Upload a file
	uploadTestFile(t, srv, "test.txt", "content", "text/plain")

	// Test date_from filter (should include recent files)
	dateFrom := time.Now().UTC().Add(-24 * time.Hour).Format("2006-01-02")
	req := httptest.NewRequest(http.MethodGet, "/files?date_from="+dateFrom, nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result struct {
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Pagination.Total < 1 {
		t.Errorf("expected at least 1 file after date_from, got %d", result.Pagination.Total)
	}
}

func TestBrowseDirectory(t *testing.T) {
	srv := newTestServer(t)

	// Upload a file to create directory structure
	uploadTestFile(t, srv, "test.txt", "content", "text/plain")

	// Test browsing storage root
	req := httptest.NewRequest(http.MethodGet, "/files/browse?path=storage", nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result struct {
		Path    string `json:"path"`
		Entries []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"entries"`
		Breadcrumbs []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"breadcrumbs"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Path != "storage" {
		t.Errorf("expected path to be 'storage', got %s", result.Path)
	}

	if len(result.Breadcrumbs) == 0 {
		t.Error("expected at least one breadcrumb")
	}
}

func TestGetCategories(t *testing.T) {
	srv := newTestServer(t)

	// Upload files to different categories
	uploadTestFile(t, srv, "img1.jpg", "content", "image/jpeg")
	uploadTestFile(t, srv, "img2.jpg", "content", "image/jpeg")
	uploadTestFile(t, srv, "doc1.pdf", "content", "application/pdf")

	// Test categories endpoint
	req := httptest.NewRequest(http.MethodGet, "/files/categories", nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result struct {
		Categories []storage.CategoryInfo `json:"categories"`
		Count      int                     `json:"count"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Count < 2 {
		t.Errorf("expected at least 2 categories, got %d", result.Count)
	}

	if len(result.Categories) == 0 {
		t.Error("expected at least one category")
	}
}

func TestGetStorageStats(t *testing.T) {
	srv := newTestServer(t)

	// Upload some files
	uploadTestFile(t, srv, "test1.jpg", "content", "image/jpeg")
	uploadTestFile(t, srv, "test2.pdf", "content", "application/pdf")

	// Test stats endpoint
	req := httptest.NewRequest(http.MethodGet, "/files/stats", nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result storage.StorageStats

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.TotalFiles < 2 {
		t.Errorf("expected at least 2 total files, got %d", result.TotalFiles)
	}

	if result.TotalSize == 0 {
		t.Error("expected total size to be greater than 0")
	}

	if len(result.FileTypes) == 0 {
		t.Error("expected at least one file type")
	}
}

// Helper function to upload a test file
func uploadTestFile(t *testing.T, srv *Server, filename, content, mimeType string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	fileWriter.Write([]byte(content))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("upload failed: %d: %s", resp.Code, resp.Body.String())
	}
}
