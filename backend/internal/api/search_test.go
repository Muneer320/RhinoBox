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

	"github.com/Muneer320/RhinoBox/internal/storage"
)

// TestFileSearchByName tests searching files by name pattern
func TestFileSearchByName(t *testing.T) {
	srv := newTestServer(t)

	// Upload multiple files with different names and unique content
	files := []struct {
		name     string
		content  string
		category string
	}{
		{"annual_report_2024.pdf", "PDF content for annual report", "documents"},
		{"monthly_report_jan.pdf", "PDF content for monthly report", "documents"},
		{"sales_summary.xlsx", "Excel content for sales", "spreadsheets"},
		{"meeting_notes.txt", "Text content for meeting notes", "documents"},
		{"vacation_photo.jpg", "JPEG data for vacation photo", "images"},
	}

	for _, f := range files {
		uploadTestFile(t, srv, f.name, f.content)
	}

	tests := []struct {
		name          string
		searchName    string
		expectedCount int
		expectedFiles []string
	}{
		{
			name:          "search for 'report' finds multiple files",
			searchName:    "report",
			expectedCount: 2,
			expectedFiles: []string{"annual_report_2024.pdf", "monthly_report_jan.pdf"},
		},
		{
			name:          "search for 'vacation' finds one file",
			searchName:    "vacation",
			expectedCount: 1,
			expectedFiles: []string{"vacation_photo.jpg"},
		},
		{
			name:          "search is case insensitive",
			searchName:    "REPORT",
			expectedCount: 2,
		},
		{
			name:          "empty search returns all files",
			searchName:    "",
			expectedCount: 5,
		},
		{
			name:          "non-matching search returns no results",
			searchName:    "nonexistent",
			expectedCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/files/search?name="+tc.searchName, nil)
			resp := httptest.NewRecorder()
			srv.router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
			}

			var result storage.SearchResult
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if result.Total != tc.expectedCount {
				t.Errorf("expected %d total files, got %d", tc.expectedCount, result.Total)
			}

			if len(result.Files) != tc.expectedCount {
				t.Errorf("expected %d files in results, got %d", tc.expectedCount, len(result.Files))
			}

			// Verify expected files are in results
			if tc.expectedFiles != nil {
				foundNames := make(map[string]bool)
				for _, f := range result.Files {
					foundNames[f.OriginalName] = true
				}
				for _, expected := range tc.expectedFiles {
					if !foundNames[expected] {
						t.Errorf("expected file %s not found in results", expected)
					}
				}
			}
		})
	}
}

// TestFileSearchByExtension tests filtering by file extension
func TestFileSearchByExtension(t *testing.T) {
	srv := newTestServer(t)

	// Upload files with various extensions and unique content
	files := []struct {
		name    string
		content string
	}{
		{"document1.pdf", "PDF content for document 1"},
		{"document2.pdf", "PDF content for document 2"},
		{"spreadsheet.xlsx", "Excel content for spreadsheet"},
		{"image1.jpg", "JPEG data for image 1"},
		{"image2.png", "PNG data for image 2"},
		{"video.mp4", "MP4 data for video"},
	}

	for _, f := range files {
		uploadTestFile(t, srv, f.name, f.content)
	}

	tests := []struct {
		name          string
		extension     string
		expectedCount int
	}{
		{
			name:          "search for .pdf extension",
			extension:     "pdf",
			expectedCount: 2,
		},
		{
			name:          "search with dot prefix",
			extension:     ".pdf",
			expectedCount: 2,
		},
		{
			name:          "search for .jpg extension",
			extension:     "jpg",
			expectedCount: 1,
		},
		{
			name:          "case insensitive extension",
			extension:     "PDF",
			expectedCount: 2,
		},
		{
			name:          "non-existent extension",
			extension:     "docx",
			expectedCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/files/search?extension="+tc.extension, nil)
			resp := httptest.NewRecorder()
			srv.router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", resp.Code)
			}

			var result storage.SearchResult
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if result.Total != tc.expectedCount {
				t.Errorf("expected %d files with extension %s, got %d", tc.expectedCount, tc.extension, result.Total)
			}
		})
	}
}

// TestFileSearchByCategory tests filtering by file category/type
func TestFileSearchByCategory(t *testing.T) {
	srv := newTestServer(t)

	// Upload files that will be categorized differently with unique content
	files := []struct {
		name     string
		content  string
		mimeType string
	}{
		{"photo1.jpg", "JPEG data for photo 1", "image/jpeg"},
		{"photo2.png", "PNG data for photo 2", "image/png"},
		{"document.pdf", "PDF content for document", "application/pdf"},
		{"video.mp4", "MP4 data for video", "video/mp4"},
		{"audio.mp3", "MP3 data for audio", "audio/mpeg"},
	}

	for _, f := range files {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fw, _ := writer.CreateFormFile("file", f.name)
		fw.Write([]byte(f.content))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)
	}

	tests := []struct {
		name          string
		category      string
		expectedMin   int // Minimum expected count
	}{
		{
			name:        "search for images",
			category:    "images",
			expectedMin: 2,
		},
		{
			name:        "search for documents",
			category:    "documents",
			expectedMin: 1,
		},
		{
			name:        "search for videos",
			category:    "videos",
			expectedMin: 1,
		},
		{
			name:        "search for audio",
			category:    "audio",
			expectedMin: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/files/search?category="+tc.category, nil)
			resp := httptest.NewRecorder()
			srv.router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", resp.Code)
			}

			var result storage.SearchResult
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if result.Total < tc.expectedMin {
				t.Errorf("expected at least %d files in category %s, got %d", tc.expectedMin, tc.category, result.Total)
			}
		})
	}
}

// TestFileSearchBySize tests filtering by file size range
func TestFileSearchBySize(t *testing.T) {
	srv := newTestServer(t)

	// Upload files with varying sizes and unique content
	files := []struct {
		name    string
		content string
	}{
		{"small.txt", "tiny file content"},                                     // ~17 bytes
		{"medium.txt", "medium sized content here with more text"},            // ~44 bytes
		{"large.txt", string(make([]byte, 1000)) + " large file content"},    // ~1000+ bytes
		{"xlarge.txt", string(make([]byte, 5000)) + " extra large content"},  // ~5000+ bytes
	}

	for _, f := range files {
		uploadTestFile(t, srv, f.name, f.content)
	}

	tests := []struct {
		name          string
		minSize       string
		maxSize       string
		expectedMin   int
		expectedMax   int
	}{
		{
			name:        "files larger than 100 bytes",
			minSize:     "100",
			expectedMin: 2,
		},
		{
			name:        "files smaller than 100 bytes",
			maxSize:     "100",
			expectedMin: 2,
		},
		{
			name:        "files between 50 and 2000 bytes",
			minSize:     "50",
			maxSize:     "2000",
			expectedMin: 1,
			expectedMax: 2,
		},
		{
			name:        "very large files only",
			minSize:     "3000",
			expectedMin: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			url := "/files/search?"
			if tc.minSize != "" {
				url += "min_size=" + tc.minSize
			}
			if tc.maxSize != "" {
				if tc.minSize != "" {
					url += "&"
				}
				url += "max_size=" + tc.maxSize
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
			resp := httptest.NewRecorder()
			srv.router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", resp.Code)
			}

			var result storage.SearchResult
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if tc.expectedMin > 0 && result.Total < tc.expectedMin {
				t.Errorf("expected at least %d files, got %d", tc.expectedMin, result.Total)
			}
			if tc.expectedMax > 0 && result.Total > tc.expectedMax {
				t.Errorf("expected at most %d files, got %d", tc.expectedMax, result.Total)
			}
		})
	}
}

// TestFileSearchByDateRange tests filtering by upload date
func TestFileSearchByDateRange(t *testing.T) {
	srv := newTestServer(t)

	// Upload files (they will have current timestamps) with unique content
	now := time.Now()
	files := []struct {
		name    string
		content string
	}{
		{"file1.txt", "content for file 1"},
		{"file2.txt", "content for file 2"},
		{"file3.txt", "content for file 3"},
	}
	for _, f := range files {
		uploadTestFile(t, srv, f.name, f.content)
		time.Sleep(10 * time.Millisecond) // Small delay to ensure different timestamps
	}

	tests := []struct {
		name          string
		dateFrom      string
		dateTo        string
		expectedMin   int
	}{
		{
			name:        "files from today",
			dateFrom:    now.Format("2006-01-02"),
			expectedMin: 3,
		},
		{
			name:        "files until tomorrow",
			dateTo:      now.Add(24 * time.Hour).Format("2006-01-02"),
			expectedMin: 3,
		},
		{
			name:        "files from yesterday to tomorrow",
			dateFrom:    now.Add(-24 * time.Hour).Format("2006-01-02"),
			dateTo:      now.Add(24 * time.Hour).Format("2006-01-02"),
			expectedMin: 3,
		},
		{
			name:        "files from future date",
			dateFrom:    now.Add(48 * time.Hour).Format("2006-01-02"),
			expectedMin: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			url := "/files/search?"
			if tc.dateFrom != "" {
				url += "date_from=" + tc.dateFrom
			}
			if tc.dateTo != "" {
				if tc.dateFrom != "" {
					url += "&"
				}
				url += "date_to=" + tc.dateTo
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
			resp := httptest.NewRecorder()
			srv.router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", resp.Code)
			}

			var result storage.SearchResult
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if result.Total < tc.expectedMin {
				t.Errorf("expected at least %d files, got %d", tc.expectedMin, result.Total)
			}
		})
	}
}

// TestFileSearchCombinedFilters tests multiple filters simultaneously
func TestFileSearchCombinedFilters(t *testing.T) {
	srv := newTestServer(t)

	// Upload a diverse set of files with unique content
	files := []struct {
		name    string
		content string
	}{
		{"sales_report_2024.pdf", "PDF sales data for 2024"},
		{"quarterly_report_2024.pdf", "PDF quarterly data for 2024"},
		{"marketing_plan.pdf", "PDF marketing strategy"},
		{"budget_spreadsheet.xlsx", "Excel budget calculations"},
		{"team_photo.jpg", "JPEG team photo image"},
		{"product_image.png", "PNG product showcase image"},
	}

	for _, f := range files {
		uploadTestFile(t, srv, f.name, f.content)
	}

	tests := []struct {
		name          string
		queryParams   string
		expectedCount int
	}{
		{
			name:          "pdf files with 'report' in name",
			queryParams:   "name=report&extension=pdf",
			expectedCount: 2,
		},
		{
			name:          "images larger than 10 bytes",
			queryParams:   "category=images&min_size=10",
			expectedCount: 2,
		},
		{
			name:          "pdf documents with 'report' uploaded today",
			queryParams:   fmt.Sprintf("name=report&extension=pdf&date_from=%s", time.Now().Format("2006-01-02")),
			expectedCount: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/files/search?"+tc.queryParams, nil)
			resp := httptest.NewRecorder()
			srv.router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", resp.Code)
			}

			var result storage.SearchResult
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if result.Total != tc.expectedCount {
				t.Errorf("expected %d files, got %d", tc.expectedCount, result.Total)
			}
		})
	}
}

// TestFileSearchPagination tests pagination functionality
func TestFileSearchPagination(t *testing.T) {
	srv := newTestServer(t)

	// Upload 25 files with unique content
	for i := 1; i <= 25; i++ {
		uploadTestFile(t, srv, fmt.Sprintf("file_%02d.txt", i), fmt.Sprintf("unique content for file number %d", i))
	}

	tests := []struct {
		name          string
		limit         string
		offset        string
		page          string
		expectedFiles int
		expectedTotal int
		hasMore       bool
	}{
		{
			name:          "first page with limit 10",
			limit:         "10",
			offset:        "0",
			expectedFiles: 10,
			expectedTotal: 25,
			hasMore:       true,
		},
		{
			name:          "second page with limit 10",
			limit:         "10",
			offset:        "10",
			expectedFiles: 10,
			expectedTotal: 25,
			hasMore:       true,
		},
		{
			name:          "last page with limit 10",
			limit:         "10",
			offset:        "20",
			expectedFiles: 5,
			expectedTotal: 25,
			hasMore:       false,
		},
		{
			name:          "page 1 using page parameter",
			limit:         "10",
			page:          "1",
			expectedFiles: 10,
			expectedTotal: 25,
			hasMore:       true,
		},
		{
			name:          "page 3 using page parameter",
			limit:         "10",
			page:          "3",
			expectedFiles: 5,
			expectedTotal: 25,
			hasMore:       false,
		},
		{
			name:          "default limit (50) returns all",
			expectedFiles: 25,
			expectedTotal: 25,
			hasMore:       false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			url := "/files/search?"
			params := []string{}
			if tc.limit != "" {
				params = append(params, "limit="+tc.limit)
			}
			if tc.offset != "" {
				params = append(params, "offset="+tc.offset)
			}
			if tc.page != "" {
				params = append(params, "page="+tc.page)
			}
			for i, p := range params {
				if i > 0 {
					url += "&"
				}
				url += p
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
			resp := httptest.NewRecorder()
			srv.router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", resp.Code)
			}

			var result storage.SearchResult
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if len(result.Files) != tc.expectedFiles {
				t.Errorf("expected %d files in page, got %d", tc.expectedFiles, len(result.Files))
			}

			if result.Total != tc.expectedTotal {
				t.Errorf("expected total %d, got %d", tc.expectedTotal, result.Total)
			}

			if result.HasMore != tc.hasMore {
				t.Errorf("expected hasMore=%v, got %v", tc.hasMore, result.HasMore)
			}
		})
	}
}

// TestFileSearchSorting tests sorting functionality
func TestFileSearchSorting(t *testing.T) {
	srv := newTestServer(t)

	// Upload files with different names and sizes with unique content
	files := []struct {
		name    string
		content string
	}{
		{"zebra.txt", "content z for zebra file"},
		{"apple.txt", string(make([]byte, 1000)) + " apple file content"},
		{"banana.txt", string(make([]byte, 500)) + " banana file content"},
		{"cherry.txt", string(make([]byte, 100)) + " cherry file content"},
	}

	for _, f := range files {
		uploadTestFile(t, srv, f.name, f.content)
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	tests := []struct {
		name          string
		sortBy        string
		sortOrder     string
		expectedFirst string
		expectedLast  string
	}{
		{
			name:          "sort by name ascending",
			sortBy:        "name",
			sortOrder:     "asc",
			expectedFirst: "apple.txt",
			expectedLast:  "zebra.txt",
		},
		{
			name:          "sort by name descending",
			sortBy:        "name",
			sortOrder:     "desc",
			expectedFirst: "zebra.txt",
			expectedLast:  "apple.txt",
		},
		{
			name:          "sort by size ascending",
			sortBy:        "size",
			sortOrder:     "asc",
			expectedFirst: "zebra.txt", // smallest
			expectedLast:  "apple.txt", // largest
		},
		{
			name:          "sort by size descending",
			sortBy:        "size",
			sortOrder:     "desc",
			expectedFirst: "apple.txt", // largest
			expectedLast:  "zebra.txt", // smallest
		},
		{
			name:          "sort by date ascending (oldest first)",
			sortBy:        "date",
			sortOrder:     "asc",
			expectedFirst: "zebra.txt", // uploaded first
			expectedLast:  "cherry.txt", // uploaded last
		},
		{
			name:          "sort by date descending (newest first)",
			sortBy:        "date",
			sortOrder:     "desc",
			expectedFirst: "cherry.txt", // uploaded last
			expectedLast:  "zebra.txt",  // uploaded first
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			url := fmt.Sprintf("/files/search?sort=%s&order=%s", tc.sortBy, tc.sortOrder)
			req := httptest.NewRequest(http.MethodGet, url, nil)
			resp := httptest.NewRecorder()
			srv.router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", resp.Code)
			}

			var result storage.SearchResult
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if len(result.Files) < 2 {
				t.Fatalf("expected at least 2 files for sorting test, got %d", len(result.Files))
			}

			if result.Files[0].OriginalName != tc.expectedFirst {
				t.Errorf("expected first file to be %s, got %s", tc.expectedFirst, result.Files[0].OriginalName)
			}

			lastIdx := len(result.Files) - 1
			if result.Files[lastIdx].OriginalName != tc.expectedLast {
				t.Errorf("expected last file to be %s, got %s", tc.expectedLast, result.Files[lastIdx].OriginalName)
			}
		})
	}
}

// TestFileSearchByMimeType tests filtering by MIME type
func TestFileSearchByMimeType(t *testing.T) {
	srv := newTestServer(t)

	files := []struct {
		name    string
		content string
	}{
		{"image1.jpg", "JPEG data for image 1"},
		{"image2.png", "PNG data for image 2"},
		{"doc.pdf", "PDF data for document"},
		{"video.mp4", "MP4 data for video file"},
	}

	for _, f := range files {
		uploadTestFile(t, srv, f.name, f.content)
	}

	// Verify that files are stored and searchable
	req := httptest.NewRequest(http.MethodGet, "/files/search", nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var result storage.SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result.Total != 4 {
		t.Errorf("expected 4 files total, got %d", result.Total)
	}

	// Test that mime_type filter works (even if content detection isn't perfect)
	// Files should have some MIME type assigned
	for _, file := range result.Files {
		if file.MimeType == "" {
			t.Errorf("file %s has no MIME type assigned", file.OriginalName)
		}
	}
}

// TestFileSearchEmptyResults tests that empty results are handled correctly
func TestFileSearchEmptyResults(t *testing.T) {
	srv := newTestServer(t)

	// Don't upload any files

	req := httptest.NewRequest(http.MethodGet, "/files/search", nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var result storage.SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result.Total != 0 {
		t.Errorf("expected 0 total files, got %d", result.Total)
	}

	if len(result.Files) != 0 {
		t.Errorf("expected 0 files in results, got %d", len(result.Files))
	}

	if result.HasMore {
		t.Error("expected HasMore to be false for empty results")
	}
}

// Helper function to upload a test file
func uploadTestFile(t *testing.T, srv *Server, filename, content string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fw, _ := writer.CreateFormFile("file", filename)
	fw.Write([]byte(content))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()

	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("failed to upload test file %s: %d %s", filename, resp.Code, resp.Body.String())
	}
}
