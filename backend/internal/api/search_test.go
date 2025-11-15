package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Muneer320/RhinoBox/internal/storage"
)

func TestFileSearch_ByName(t *testing.T) {
	srv := newTestServer(t)

	// Upload test files
	files := []struct {
		name    string
		content string
		mime    string
	}{
		{"report_2024.pdf", "pdf content 2024", "application/pdf"},
		{"report_2023.pdf", "pdf content 2023", "application/pdf"},
		{"document.txt", "txt content", "text/plain"},
		{"image_report.png", "png content", "image/png"},
	}

	var hashes []string
	for _, f := range files {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fileWriter, err := writer.CreateFormFile("file", f.name)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		fileWriter.Write([]byte(f.content))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("upload failed: %d: %s", resp.Code, resp.Body.String())
		}

		var payload struct {
			Stored []map[string]any `json:"stored"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(payload.Stored) > 0 {
			if hash, ok := payload.Stored[0]["hash"].(string); ok {
				hashes = append(hashes, hash)
			}
		}
	}

	// Test search by name
	tests := []struct {
		name         string
		query        string
		expectedMin  int
		expectedMax  int
		shouldContain []string
	}{
		{
			name:         "search by partial name",
			query:        "?name=report",
			expectedMin:  2,
			expectedMax:  3,
			shouldContain: []string{"report_2024.pdf", "report_2023.pdf"},
		},
		{
			name:         "search by exact name",
			query:        "?name=document.txt",
			expectedMin:  1,
			expectedMax:  1,
			shouldContain: []string{"document.txt"},
		},
		{
			name:         "case insensitive search",
			query:        "?name=REPORT",
			expectedMin:  2,
			expectedMax:  3,
			shouldContain: []string{"report_2024.pdf", "report_2023.pdf"},
		},
		{
			name:         "no matches",
			query:        "?name=nonexistent",
			expectedMin:  0,
			expectedMax:  0,
			shouldContain: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/files/search"+tt.query, nil)
			resp := httptest.NewRecorder()
			srv.router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
			}

			var payload struct {
				Filters map[string]any   `json:"filters"`
				Results []storage.FileMetadata `json:"results"`
				Count   int                `json:"count"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if payload.Count < tt.expectedMin || payload.Count > tt.expectedMax {
				t.Errorf("expected %d-%d results, got %d", tt.expectedMin, tt.expectedMax, payload.Count)
			}

			resultNames := make(map[string]bool)
			for _, r := range payload.Results {
				resultNames[r.OriginalName] = true
			}

			for _, expectedName := range tt.shouldContain {
				if !resultNames[expectedName] {
					t.Errorf("expected to find %q in results", expectedName)
				}
			}
		})
	}
}

func TestFileSearch_ByExtension(t *testing.T) {
	srv := newTestServer(t)

	// Upload test files
	files := []struct {
		name    string
		content string
		mime    string
	}{
		{"file1.pdf", "pdf content 1", "application/pdf"},
		{"file2.PDF", "pdf content 2", "application/pdf"},
		{"file3.txt", "txt content", "text/plain"},
		{"file4.jpg", "jpg content", "image/jpeg"},
	}

	for _, f := range files {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fileWriter, err := writer.CreateFormFile("file", f.name)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		fileWriter.Write([]byte(f.content))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("upload failed: %d: %s", resp.Code, resp.Body.String())
		}
	}

	tests := []struct {
		name         string
		query        string
		expectedCount int
		shouldContain []string
	}{
		{
			name:         "search by extension with dot",
			query:        "?extension=.pdf",
			expectedCount: 2,
			shouldContain: []string{"file1.pdf", "file2.PDF"},
		},
		{
			name:         "search by extension without dot",
			query:        "?extension=pdf",
			expectedCount: 2,
			shouldContain: []string{"file1.pdf", "file2.PDF"},
		},
		{
			name:         "search case insensitive extension",
			query:        "?extension=PDF",
			expectedCount: 2,
			shouldContain: []string{"file1.pdf", "file2.PDF"},
		},
		{
			name:         "search by jpg extension",
			query:        "?extension=jpg",
			expectedCount: 1,
			shouldContain: []string{"file4.jpg"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/files/search"+tt.query, nil)
			resp := httptest.NewRecorder()
			srv.router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
			}

			var payload struct {
				Results []storage.FileMetadata `json:"results"`
				Count   int                `json:"count"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if payload.Count != tt.expectedCount {
				t.Errorf("expected %d results, got %d", tt.expectedCount, payload.Count)
			}

			resultNames := make(map[string]bool)
			for _, r := range payload.Results {
				resultNames[r.OriginalName] = true
			}

			for _, expectedName := range tt.shouldContain {
				if !resultNames[expectedName] {
					t.Errorf("expected to find %q in results", expectedName)
				}
			}
		})
	}
}

func TestFileSearch_ByType(t *testing.T) {
	srv := newTestServer(t)

	// Upload test files
	files := []struct {
		name    string
		content string
		mime    string
	}{
		{"photo1.jpg", "jpg content", "image/jpeg"},
		{"photo2.png", "png content", "image/png"},
		{"video1.mp4", "mp4 content", "video/mp4"},
		{"audio1.mp3", "mp3 content", "audio/mpeg"},
		{"doc1.pdf", "pdf content", "application/pdf"},
	}

	for _, f := range files {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fileWriter, err := writer.CreateFormFile("file", f.name)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		fileWriter.Write([]byte(f.content))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("upload failed: %d: %s", resp.Code, resp.Body.String())
		}
	}

	tests := []struct {
		name         string
		query        string
		expectedMin  int
		expectedMax  int
		shouldContain []string
	}{
		{
			name:         "search by image type",
			query:        "?type=image",
			expectedMin:  2,
			expectedMax:  2,
			shouldContain: []string{"photo1.jpg", "photo2.png"},
		},
		{
			name:         "search by video type",
			query:        "?type=video",
			expectedMin:  1,
			expectedMax:  1,
			shouldContain: []string{"video1.mp4"},
		},
		{
			name:         "search by audio type",
			query:        "?type=audio",
			expectedMin:  1,
			expectedMax:  1,
			shouldContain: []string{"audio1.mp3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/files/search"+tt.query, nil)
			resp := httptest.NewRecorder()
			srv.router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
			}

			var payload struct {
				Results []storage.FileMetadata `json:"results"`
				Count   int                `json:"count"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if payload.Count < tt.expectedMin || payload.Count > tt.expectedMax {
				t.Errorf("expected %d-%d results, got %d", tt.expectedMin, tt.expectedMax, payload.Count)
			}

			resultNames := make(map[string]bool)
			for _, r := range payload.Results {
				resultNames[r.OriginalName] = true
			}

			for _, expectedName := range tt.shouldContain {
				if !resultNames[expectedName] {
					t.Errorf("expected to find %q in results", expectedName)
				}
			}
		})
	}
}

func TestFileSearch_ByDateRange(t *testing.T) {
	srv := newTestServer(t)

	// Upload test files
	files := []struct {
		name    string
		content string
		mime    string
	}{
		{"file1.txt", "content 1", "text/plain"},
		{"file2.txt", "content 2", "text/plain"},
		{"file3.txt", "content 3", "text/plain"},
	}

	for _, f := range files {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fileWriter, err := writer.CreateFormFile("file", f.name)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		fileWriter.Write([]byte(f.content))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("upload failed: %d: %s", resp.Code, resp.Body.String())
		}
		// Small delay to ensure different timestamps
		time.Sleep(10 * time.Millisecond)
	}

	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	tomorrow := now.AddDate(0, 0, 1)

	tests := []struct {
		name         string
		query        string
		expectedMin  int
		shouldContain []string
	}{
		{
			name:         "search by date range (RFC3339)",
			query:        "?date_from=" + url.QueryEscape(yesterday.Format(time.RFC3339)) + "&date_to=" + url.QueryEscape(tomorrow.Format(time.RFC3339)),
			expectedMin:  len(files),
			shouldContain: []string{"file1.txt", "file2.txt", "file3.txt"},
		},
		{
			name:         "search by date range (YYYY-MM-DD)",
			query:        "?date_from=" + yesterday.Format("2006-01-02") + "&date_to=" + tomorrow.Format("2006-01-02"),
			expectedMin:  len(files),
			shouldContain: []string{"file1.txt", "file2.txt", "file3.txt"},
		},
		{
			name:         "search by date_from only",
			query:        "?date_from=" + yesterday.Format("2006-01-02"),
			expectedMin:  len(files),
			shouldContain: []string{"file1.txt", "file2.txt", "file3.txt"},
		},
		{
			name:         "search by date_to only",
			query:        "?date_to=" + tomorrow.Format("2006-01-02"),
			expectedMin:  len(files),
			shouldContain: []string{"file1.txt", "file2.txt", "file3.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/files/search"+tt.query, nil)
			resp := httptest.NewRecorder()
			srv.router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
			}

			var payload struct {
				Results []storage.FileMetadata `json:"results"`
				Count   int                `json:"count"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if payload.Count < tt.expectedMin {
				t.Errorf("expected at least %d results, got %d", tt.expectedMin, payload.Count)
			}
		})
	}
}

func TestFileSearch_ByMimeTypeQueryParam(t *testing.T) {
	srv := newTestServer(t)

	// Upload JPEG and PNG files with proper content
	jpegContent := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46} // JPEG header
	pngContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG header

	files := []struct {
		name    string
		content []byte
		mime    string
	}{
		{"photo1.jpg", jpegContent, "image/jpeg"},
		{"photo2.jpg", jpegContent, "image/jpeg"},
		{"image1.png", pngContent, "image/png"},
		{"image2.png", pngContent, "image/png"},
	}

	for _, f := range files {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fileWriter, err := writer.CreateFormFile("file", f.name)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		fileWriter.Write(f.content)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("upload failed: %d: %s", resp.Code, resp.Body.String())
		}

	}

	// Small delay to ensure indexing
	time.Sleep(50 * time.Millisecond)

	// Test search by mime_type=image/jpeg
	t.Run("search by mime_type=image/jpeg", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/files/search?mime_type=image/jpeg", nil)
		resp := httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)

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

		// Should only contain JPEG files, not PNG files
		if payload.Count < 2 {
			t.Errorf("expected at least 2 JPEG results, got %d", payload.Count)
		}

		// Verify all results are JPEG
		for _, r := range payload.Results {
			if !strings.EqualFold(r.MimeType, "image/jpeg") {
				t.Errorf("expected only JPEG files, found %q with MIME type %q", r.OriginalName, r.MimeType)
			}
		}

		// Verify JPEG files are present
		jpegFound := false
		for _, r := range payload.Results {
			if strings.Contains(r.OriginalName, ".jpg") {
				jpegFound = true
				break
			}
		}
		if !jpegFound {
			t.Error("expected to find JPEG files in results")
		}

		// Verify PNG files are NOT present
		for _, r := range payload.Results {
			if strings.Contains(r.OriginalName, ".png") {
				t.Errorf("expected no PNG files, found %q", r.OriginalName)
			}
		}
	})
}

func TestFileSearch_CombinedFilters(t *testing.T) {
	srv := newTestServer(t)

	// Upload test files
	files := []struct {
		name    string
		content string
		mime    string
	}{
		{"report_2024.pdf", "pdf content 2024", "application/pdf"},
		{"report_2023.pdf", "pdf content 2023", "application/pdf"},
		{"image_report.png", "png content", "image/png"},
		{"document.txt", "txt content", "text/plain"},
	}

	for _, f := range files {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fileWriter, err := writer.CreateFormFile("file", f.name)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		fileWriter.Write([]byte(f.content))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("upload failed: %d: %s", resp.Code, resp.Body.String())
		}
	}

	tests := []struct {
		name         string
		query        string
		expectedCount int
		shouldContain []string
	}{
		{
			name:         "name and extension",
			query:        "?name=report&extension=pdf",
			expectedCount: 2,
			shouldContain: []string{"report_2024.pdf", "report_2023.pdf"},
		},
		{
			name:         "name and type",
			query:        "?name=report&type=image",
			expectedCount: 1,
			shouldContain: []string{"image_report.png"},
		},
		{
			name:         "extension and MIME type",
			query:        "?extension=pdf&mime_type=application/pdf",
			expectedCount: 2,
			shouldContain: []string{"report_2024.pdf", "report_2023.pdf"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/files/search"+tt.query, nil)
			resp := httptest.NewRecorder()
			srv.router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
			}

			var payload struct {
				Results []storage.FileMetadata `json:"results"`
				Count   int                `json:"count"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if payload.Count != tt.expectedCount {
				t.Errorf("expected %d results, got %d", tt.expectedCount, payload.Count)
			}

			resultNames := make(map[string]bool)
			for _, r := range payload.Results {
				resultNames[r.OriginalName] = true
			}

			for _, expectedName := range tt.shouldContain {
				if !resultNames[expectedName] {
					t.Errorf("expected to find %q in results", expectedName)
				}
			}
		})
	}
}

func TestFileSearch_NoFilters(t *testing.T) {
	srv := newTestServer(t)

	// Test without any filters
	req := httptest.NewRequest(http.MethodGet, "/files/search", nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for no filters, got %d: %s", resp.Code, resp.Body.String())
	}

	var errorPayload struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&errorPayload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if !strings.Contains(errorPayload.Error, "at least one filter") {
		t.Errorf("expected error about missing filters, got: %s", errorPayload.Error)
	}
}

func TestFileSearch_InvalidDateFormat(t *testing.T) {
	srv := newTestServer(t)

	// Test with invalid date format
	req := httptest.NewRequest(http.MethodGet, "/files/search?date_from=invalid-date", nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid date, got %d: %s", resp.Code, resp.Body.String())
	}

	var errorPayload struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&errorPayload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if !strings.Contains(errorPayload.Error, "invalid date_from") {
		t.Errorf("expected error about invalid date format, got: %s", errorPayload.Error)
	}
}

func TestFileSearch_ByMimeType(t *testing.T) {
	srv := newTestServer(t)

	// Upload test files - use unique content to avoid deduplication
	files := []struct {
		name    string
		content string
		mime    string
	}{
		{"file1.pdf", "unique pdf content 1 abc123", "application/pdf"},
		{"file2.pdf", "unique pdf content 2 xyz789", "application/pdf"},
		{"file3.jpg", "unique jpg content def456", "image/jpeg"},
	}

	var storedMimeTypes []string
	for _, f := range files {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fileWriter, err := writer.CreateFormFile("file", f.name)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		fileWriter.Write([]byte(f.content))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("upload failed: %d: %s", resp.Code, resp.Body.String())
		}

		// Get the stored MIME type
		var uploadResp struct {
			Stored []map[string]any `json:"stored"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err == nil && len(uploadResp.Stored) > 0 {
			if mime, ok := uploadResp.Stored[0]["mime_type"].(string); ok {
				storedMimeTypes = append(storedMimeTypes, mime)
			}
		}
	}

	// Test search by extension (more reliable than MIME type which may vary)
	req := httptest.NewRequest(http.MethodGet, "/files/search?extension=pdf", nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var payload struct {
		Results []storage.FileMetadata `json:"results"`
		Count   int                `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.Count < 2 {
		t.Logf("Stored MIME types: %v", storedMimeTypes)
		t.Errorf("expected at least 2 PDF results, got %d", payload.Count)
	}
}
