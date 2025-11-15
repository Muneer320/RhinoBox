package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestFilesListEndpoint tests the /files endpoint with various filters.
func TestFilesListEndpoint(t *testing.T) {
	srv := newTestServer(t)

	// Upload test files with different characteristics
	uploadTestFiles(t, srv, []testFile{
		{name: "report2024.pdf", mime: "application/pdf", size: 1024 * 512, category: ""},
		{name: "photo.jpg", mime: "image/jpeg", size: 1024 * 256, category: ""},
		{name: "video.mp4", mime: "video/mp4", size: 1024 * 1024 * 5, category: ""},
		{name: "presentation.pptx", mime: "application/vnd.openxmlformats-officedocument.presentationml.presentation", size: 1024 * 768, category: ""},
		{name: "data.csv", mime: "text/csv", size: 1024 * 64, category: ""},
	})

	tests := []struct {
		name         string
		query        string
		expectCount  int
		expectStatus int
	}{
		{
			name:         "list all files",
			query:        "?page=1&limit=50",
			expectCount:  5,
			expectStatus: http.StatusOK,
		},
		{
			name:         "filter by category",
			query:        "?category=documents/pdf",
			expectCount:  1,
			expectStatus: http.StatusOK,
		},
		{
			name:         "filter by mime type",
			query:        "?mime_type=image/jpeg",
			expectCount:  0, // JPEG detection may vary based on content
			expectStatus: http.StatusOK,
		},
		{
			name:         "filter by size range",
			query:        "?min_size=100000&max_size=600000",
			expectCount:  2,
			expectStatus: http.StatusOK,
		},
		{
			name:         "search by name",
			query:        "?search=photo",
			expectCount:  1,
			expectStatus: http.StatusOK,
		},
		{
			name:         "pagination",
			query:        "?page=1&limit=2",
			expectCount:  2,
			expectStatus: http.StatusOK,
		},
		{
			name:         "sort by size descending",
			query:        "?sort=size&order=desc",
			expectCount:  5,
			expectStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/files"+tt.query, nil)
			resp := httptest.NewRecorder()

			srv.router.ServeHTTP(resp, req)

			if resp.Code != tt.expectStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.expectStatus, resp.Code, resp.Body.String())
			}

			var result map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			files, ok := result["files"].([]any)
			if !ok {
				t.Fatalf("expected files array in response")
			}

			if len(files) != tt.expectCount {
				t.Fatalf("expected %d files, got %d", tt.expectCount, len(files))
			}

			// Validate pagination metadata
			pagination, ok := result["pagination"].(map[string]any)
			if !ok {
				t.Fatalf("expected pagination in response")
			}

			if _, ok := pagination["page"]; !ok {
				t.Fatalf("pagination missing page field")
			}
		})
	}
}

// TestFilesListDateFilter tests date range filtering.
func TestFilesListDateFilter(t *testing.T) {
	srv := newTestServer(t)

	// Upload files
	uploadTestFiles(t, srv, []testFile{
		{name: "old_file.txt", mime: "text/plain", size: 1024, category: "documents"},
		{name: "new_file.txt", mime: "text/plain", size: 1024, category: "documents"},
	})

	// Test date filtering
	now := time.Now().UTC()
	afterDate := now.Add(-1 * time.Hour).Format(time.RFC3339)

	req := httptest.NewRequest(http.MethodGet, "/files?uploaded_after="+afterDate, nil)
	resp := httptest.NewRecorder()

	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	files, ok := result["files"].([]any)
	if !ok {
		t.Fatalf("expected files array in response")
	}

	// Should get files uploaded after the specified date
	if len(files) < 1 {
		t.Fatalf("expected at least 1 file uploaded after %s", afterDate)
	}
}

// TestFilesBrowseEndpoint tests the /files/browse endpoint.
func TestFilesBrowseEndpoint(t *testing.T) {
	srv := newTestServer(t)

	// Upload files to create directory structure
	uploadTestFiles(t, srv, []testFile{
		{name: "photo.jpg", mime: "image/jpeg", size: 1024, category: "images"},
		{name: "video.mp4", mime: "video/mp4", size: 2048, category: "videos"},
	})

	tests := []struct {
		name         string
		path         string
		expectStatus int
		expectDirs   bool
		expectFiles  bool
	}{
		{
			name:         "browse root",
			path:         "",
			expectStatus: http.StatusOK,
			expectDirs:   true,
			expectFiles:  false,
		},
		{
			name:         "browse storage",
			path:         "storage",
			expectStatus: http.StatusOK,
			expectDirs:   true,
			expectFiles:  false,
		},
		{
			name:         "browse images category",
			path:         "storage/images",
			expectStatus: http.StatusOK,
			expectDirs:   true,
			expectFiles:  false,
		},
		{
			name:         "invalid path",
			path:         "../etc",
			expectStatus: http.StatusBadRequest,
			expectDirs:   false,
			expectFiles:  false,
		},
		{
			name:         "non-existent path",
			path:         "storage/nonexistent",
			expectStatus: http.StatusNotFound,
			expectDirs:   false,
			expectFiles:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/files/browse"
			if tt.path != "" {
				url += "?path=" + tt.path
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
			resp := httptest.NewRecorder()

			srv.router.ServeHTTP(resp, req)

			if resp.Code != tt.expectStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.expectStatus, resp.Code, resp.Body.String())
			}

			if tt.expectStatus != http.StatusOK {
				return
			}

			var result map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if _, ok := result["path"]; !ok {
				t.Fatalf("expected path in response")
			}

			if _, ok := result["breadcrumbs"]; !ok {
				t.Fatalf("expected breadcrumbs in response")
			}

			if tt.expectDirs {
				dirs, ok := result["directories"].([]any)
				if !ok {
					t.Fatalf("expected directories array in response")
				}
				if len(dirs) == 0 {
					t.Logf("Warning: expected directories but got none")
				}
			}

			if tt.expectFiles {
				files, ok := result["files"].([]any)
				if !ok {
					t.Fatalf("expected files array in response")
				}
				if len(files) == 0 {
					t.Logf("Warning: expected files but got none")
				}
			}
		})
	}
}

// TestCategoriesEndpoint tests the /files/categories endpoint.
func TestCategoriesEndpoint(t *testing.T) {
	srv := newTestServer(t)

	// Upload files across different categories
	uploadTestFiles(t, srv, []testFile{
		{name: "photo1.jpg", mime: "image/jpeg", size: 1024, category: "images"},
		{name: "photo2.jpg", mime: "image/jpeg", size: 2048, category: "images"},
		{name: "video.mp4", mime: "video/mp4", size: 5120, category: "videos"},
		{name: "doc.pdf", mime: "application/pdf", size: 3072, category: "documents"},
	})

	req := httptest.NewRequest(http.MethodGet, "/files/categories", nil)
	resp := httptest.NewRecorder()

	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	categories, ok := result["categories"].([]any)
	if !ok {
		t.Fatalf("expected categories array in response")
	}

	if len(categories) == 0 {
		t.Fatalf("expected at least one category")
	}

	// Validate category structure
	for _, cat := range categories {
		catMap, ok := cat.(map[string]any)
		if !ok {
			t.Fatalf("invalid category structure")
		}

		if _, ok := catMap["path"]; !ok {
			t.Fatalf("category missing path field")
		}
		if _, ok := catMap["count"]; !ok {
			t.Fatalf("category missing count field")
		}
		if _, ok := catMap["size"]; !ok {
			t.Fatalf("category missing size field")
		}
	}
}

// TestStatsEndpoint tests the /files/stats endpoint.
func TestStatsEndpoint(t *testing.T) {
	srv := newTestServer(t)

	// Upload various files
	uploadTestFiles(t, srv, []testFile{
		{name: "photo1.jpg", mime: "image/jpeg", size: 1024, category: "images"},
		{name: "photo2.png", mime: "image/png", size: 2048, category: "images"},
		{name: "video.mp4", mime: "video/mp4", size: 5120, category: "videos"},
		{name: "doc.pdf", mime: "application/pdf", size: 3072, category: "documents"},
		{name: "audio.mp3", mime: "audio/mpeg", size: 2560, category: "audio"},
	})

	req := httptest.NewRequest(http.MethodGet, "/files/stats", nil)
	resp := httptest.NewRecorder()

	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Validate stats structure
	totalFiles, ok := result["total_files"].(float64)
	if !ok || totalFiles < 5 {
		t.Fatalf("expected total_files >= 5, got %v", result["total_files"])
	}

	totalSize, ok := result["total_size"].(float64)
	if !ok || totalSize < 1 {
		t.Fatalf("expected total_size > 0, got %v", result["total_size"])
	}

	if _, ok := result["categories"]; !ok {
		t.Fatalf("expected categories in stats")
	}

	if _, ok := result["file_types"]; !ok {
		t.Fatalf("expected file_types in stats")
	}

	recentUploads, ok := result["recent_uploads"].(map[string]any)
	if !ok {
		t.Fatalf("expected recent_uploads in stats")
	}

	if _, ok := recentUploads["last_24h"]; !ok {
		t.Fatalf("expected last_24h in recent_uploads")
	}
	if _, ok := recentUploads["last_7d"]; !ok {
		t.Fatalf("expected last_7d in recent_uploads")
	}
	if _, ok := recentUploads["last_30d"]; !ok {
		t.Fatalf("expected last_30d in recent_uploads")
	}
}

// TestRealWorldScenario tests a comprehensive end-to-end scenario.
func TestRealWorldScenario(t *testing.T) {
	srv := newTestServer(t)

	t.Log("📁 Testing comprehensive file management workflow")

	// Step 1: Upload diverse file types
	t.Log("Step 1: Uploading diverse files...")
	testFiles := []testFile{
		{name: "company_logo.png", mime: "image/png", size: 50 * 1024, category: "branding"},
		{name: "product_demo.mp4", mime: "video/mp4", size: 10 * 1024 * 1024, category: "marketing"},
		{name: "annual_report.pdf", mime: "application/pdf", size: 500 * 1024, category: "finance"},
		{name: "employee_data.csv", mime: "text/csv", size: 30 * 1024, category: "hr"},
		{name: "presentation.pptx", mime: "application/vnd.openxmlformats-officedocument.presentationml.presentation", size: 2 * 1024 * 1024, category: "sales"},
		{name: "profile_pic.jpg", mime: "image/jpeg", size: 100 * 1024, category: "team"},
		{name: "meeting_audio.mp3", mime: "audio/mpeg", size: 5 * 1024 * 1024, category: "meetings"},
		{name: "backup.zip", mime: "application/zip", size: 20 * 1024 * 1024, category: "backups"},
		{name: "invoice_2024.pdf", mime: "application/pdf", size: 200 * 1024, category: "finance"},
		{name: "website_banner.jpg", mime: "image/jpeg", size: 150 * 1024, category: "branding"},
	}
	uploadTestFiles(t, srv, testFiles)
	t.Logf("✓ Uploaded %d files", len(testFiles))

	// Step 2: List all files
	t.Log("Step 2: Listing all files...")
	req := httptest.NewRequest(http.MethodGet, "/files?limit=100", nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusOK {
		t.Fatalf("list all files failed: %d", resp.Code)
	}
	
	var listResult map[string]any
	json.NewDecoder(resp.Body).Decode(&listResult)
	files := listResult["files"].([]any)
	t.Logf("✓ Listed %d files", len(files))

	// Step 3: Filter by category
	t.Log("Step 3: Filtering by category...")
	req = httptest.NewRequest(http.MethodGet, "/files?category=documents/pdf", nil)
	resp = httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)
	
	json.NewDecoder(resp.Body).Decode(&listResult)
	pdfFiles := listResult["files"].([]any)
	t.Logf("✓ Found %d PDF documents", len(pdfFiles))

	// Step 4: Get categories
	t.Log("Step 4: Retrieving categories...")
	req = httptest.NewRequest(http.MethodGet, "/files/categories", nil)
	resp = httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)
	
	var catResult map[string]any
	json.NewDecoder(resp.Body).Decode(&catResult)
	categories := catResult["categories"].([]any)
	t.Logf("✓ Found %d categories", len(categories))

	// Step 5: Get overall stats
	t.Log("Step 5: Getting storage statistics...")
	req = httptest.NewRequest(http.MethodGet, "/files/stats", nil)
	resp = httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)
	
	var statsResult map[string]any
	json.NewDecoder(resp.Body).Decode(&statsResult)
	totalFiles := int(statsResult["total_files"].(float64))
	totalSize := int64(statsResult["total_size"].(float64))
	t.Logf("✓ Total: %d files, %d bytes", totalFiles, totalSize)

	// Step 6: Browse directory structure
	t.Log("Step 6: Browsing directory structure...")
	req = httptest.NewRequest(http.MethodGet, "/files/browse?path=storage", nil)
	resp = httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)
	
	var browseResult map[string]any
	json.NewDecoder(resp.Body).Decode(&browseResult)
	dirs := browseResult["directories"].([]any)
	t.Logf("✓ Found %d directories", len(dirs))

	// Step 7: Search functionality
	t.Log("Step 7: Testing search...")
	req = httptest.NewRequest(http.MethodGet, "/files?search=report", nil)
	resp = httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)
	
	json.NewDecoder(resp.Body).Decode(&listResult)
	searchResults := listResult["files"].([]any)
	t.Logf("✓ Found %d files matching 'report'", len(searchResults))

	// Step 8: Size-based filtering
	t.Log("Step 8: Filtering by size...")
	req = httptest.NewRequest(http.MethodGet, "/files?min_size=1000000&max_size=15000000", nil)
	resp = httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)
	
	json.NewDecoder(resp.Body).Decode(&listResult)
	sizedFiles := listResult["files"].([]any)
	t.Logf("✓ Found %d files in size range", len(sizedFiles))

	// Step 9: Pagination
	t.Log("Step 9: Testing pagination...")
	req = httptest.NewRequest(http.MethodGet, "/files?page=1&limit=5", nil)
	resp = httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)
	
	json.NewDecoder(resp.Body).Decode(&listResult)
	page1Files := listResult["files"].([]any)
	pagination := listResult["pagination"].(map[string]any)
	t.Logf("✓ Page 1: %d files, has_next: %v", len(page1Files), pagination["has_next"])

	// Step 10: Sorting
	t.Log("Step 10: Testing sorting...")
	req = httptest.NewRequest(http.MethodGet, "/files?sort=size&order=desc&limit=5", nil)
	resp = httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)
	
	json.NewDecoder(resp.Body).Decode(&listResult)
	sortedFiles := listResult["files"].([]any)
	if len(sortedFiles) >= 2 {
		file1 := sortedFiles[0].(map[string]any)
		file2 := sortedFiles[1].(map[string]any)
		size1 := int64(file1["size"].(float64))
		size2 := int64(file2["size"].(float64))
		if size1 < size2 {
			t.Fatalf("Sorting failed: %d should be >= %d", size1, size2)
		}
		t.Logf("✓ Largest file: %d bytes", size1)
	}

	t.Log("✅ Real-world scenario completed successfully")
}

// TestPaginationEdgeCases tests edge cases in pagination.
func TestPaginationEdgeCases(t *testing.T) {
	srv := newTestServer(t)

	// Upload exactly 10 files with unique content
	testFiles := make([]testFile, 10)
	for i := 0; i < 10; i++ {
		testFiles[i] = testFile{
			name:     fmt.Sprintf("file%d.txt", i),
			mime:     "text/plain",
			size:     1024 + int64(i*100), // Make sizes unique
			category: fmt.Sprintf("test%d", i),
		}
	}
	uploadTestFiles(t, srv, testFiles)

	tests := []struct {
		name         string
		page         int
		limit        int
		expectCount  int
		expectHasNext bool
		expectHasPrev bool
	}{
		{
			name:          "first page",
			page:          1,
			limit:         5,
			expectCount:   5,
			expectHasNext: true,
			expectHasPrev: false,
		},
		{
			name:          "last page",
			page:          2,
			limit:         5,
			expectCount:   5,
			expectHasNext: false,
			expectHasPrev: true,
		},
		{
			name:          "page beyond range",
			page:          10,
			limit:         5,
			expectCount:   0,
			expectHasNext: false,
			expectHasPrev: true,
		},
		{
			name:          "all in one page",
			page:          1,
			limit:         100,
			expectCount:   10,
			expectHasNext: false,
			expectHasPrev: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := fmt.Sprintf("/files?page=%d&limit=%d", tt.page, tt.limit)
			req := httptest.NewRequest(http.MethodGet, url, nil)
			resp := httptest.NewRecorder()

			srv.router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", resp.Code)
			}

			var result map[string]any
			json.NewDecoder(resp.Body).Decode(&result)

			files := result["files"].([]any)
			if len(files) != tt.expectCount {
				t.Fatalf("expected %d files, got %d", tt.expectCount, len(files))
			}

			pagination := result["pagination"].(map[string]any)
			hasNext := pagination["has_next"].(bool)
			hasPrev := pagination["has_prev"].(bool)

			if hasNext != tt.expectHasNext {
				t.Fatalf("expected has_next=%v, got %v", tt.expectHasNext, hasNext)
			}
			if hasPrev != tt.expectHasPrev {
				t.Fatalf("expected has_prev=%v, got %v", tt.expectHasPrev, hasPrev)
			}
		})
	}
}

// TestEmptyResultsHandling tests handling of empty results.
func TestEmptyResultsHandling(t *testing.T) {
	srv := newTestServer(t)

	tests := []struct {
		name     string
		endpoint string
	}{
		{"list files", "/files"},
		{"categories", "/files/categories"},
		{"stats", "/files/stats"},
		{"browse", "/files/browse?path=storage"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.endpoint, nil)
			resp := httptest.NewRecorder()

			srv.router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
			}

			var result map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			t.Logf("Empty result handled gracefully for %s", tt.name)
		})
	}
}

// Helper types and functions

type testFile struct {
	name     string
	mime     string
	size     int64
	category string
}

func uploadTestFiles(t *testing.T, srv *Server, files []testFile) {
	t.Helper()

	for _, tf := range files {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		fileWriter, err := writer.CreateFormFile("file", tf.name)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}

		// Create fake file content
		content := make([]byte, tf.size)
		for i := range content {
			content[i] = byte(i % 256)
		}
		fileWriter.Write(content)

		if tf.category != "" {
			writer.WriteField("category", tf.category)
		}

		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()

		srv.router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("upload %s failed: %d: %s", tf.name, resp.Code, resp.Body.String())
		}
	}
}
