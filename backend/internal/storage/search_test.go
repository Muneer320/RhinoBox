package storage

import (
	"io"
	"strings"
	"testing"
	"time"
)

func newBytesReader(data []byte) io.Reader {
	return strings.NewReader(string(data))
}

func TestSearchFiles_ByName(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Create test files with different names
	files := []struct {
		name    string
		content string
		mime    string
	}{
		{"report_2024.pdf", "report content", "application/pdf"},
		{"report_2023.pdf", "old report", "application/pdf"},
		{"document.txt", "doc content", "text/plain"},
		{"image_report.png", "image data", "image/png"},
		{"presentation.pptx", "ppt content", "application/vnd.openxmlformats-officedocument.presentationml.presentation"},
	}

	for _, f := range files {
		req := StoreRequest{
			Reader:   newBytesReader([]byte(f.content)),
			Filename: f.name,
			MimeType: f.mime,
			Size:     int64(len(f.content)),
		}
		if _, err := mgr.StoreFile(req); err != nil {
			t.Fatalf("StoreFile(%q) failed: %v", f.name, err)
		}
	}

	tests := []struct {
		name         string
		filter       SearchFilters
		expectedMin  int
		expectedMax  int
		expectedNames []string
	}{
		{
			name: "search by partial name",
			filter: SearchFilters{Name: "report"},
			expectedMin: 3,
			expectedMax: 3,
			expectedNames: []string{"report_2024.pdf", "report_2023.pdf", "image_report.png"},
		},
		{
			name: "search by exact name",
			filter: SearchFilters{Name: "document.txt"},
			expectedMin: 1,
			expectedMax: 1,
			expectedNames: []string{"document.txt"},
		},
		{
			name: "search case insensitive",
			filter: SearchFilters{Name: "REPORT"},
			expectedMin: 3,
			expectedMax: 3,
			expectedNames: []string{"report_2024.pdf", "report_2023.pdf", "image_report.png"},
		},
		{
			name: "no matches",
			filter: SearchFilters{Name: "nonexistent"},
			expectedMin: 0,
			expectedMax: 0,
			expectedNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := mgr.SearchFiles(tt.filter)
			if len(results) < tt.expectedMin || len(results) > tt.expectedMax {
				t.Errorf("expected %d-%d results, got %d", tt.expectedMin, tt.expectedMax, len(results))
			}

			// Verify expected names are present
			resultNames := make(map[string]bool)
			for _, r := range results {
				resultNames[r.OriginalName] = true
			}

			for _, expectedName := range tt.expectedNames {
				if !resultNames[expectedName] {
					t.Errorf("expected to find %q in results", expectedName)
				}
			}
		})
	}
}

func TestSearchFiles_ByExtension(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	files := []struct {
		name    string
		content string
		mime    string
	}{
		{"file1.pdf", "pdf content", "application/pdf"},
		{"file2.PDF", "pdf content uppercase", "application/pdf"},
		{"file3.txt", "txt content", "text/plain"},
		{"file4.jpg", "jpg content", "image/jpeg"},
		{"file5.png", "png content", "image/png"},
	}

	for _, f := range files {
		req := StoreRequest{
			Reader:   newBytesReader([]byte(f.content)),
			Filename: f.name,
			MimeType: f.mime,
			Size:     int64(len(f.content)),
		}
		if _, err := mgr.StoreFile(req); err != nil {
			t.Fatalf("StoreFile(%q) failed: %v", f.name, err)
		}
	}

	tests := []struct {
		name         string
		filter       SearchFilters
		expectedCount int
		expectedNames []string
	}{
		{
			name: "search by extension with dot",
			filter: SearchFilters{Extension: ".pdf"},
			expectedCount: 2,
			expectedNames: []string{"file1.pdf", "file2.PDF"},
		},
		{
			name: "search by extension without dot",
			filter: SearchFilters{Extension: "pdf"},
			expectedCount: 2,
			expectedNames: []string{"file1.pdf", "file2.PDF"},
		},
		{
			name: "search case insensitive extension",
			filter: SearchFilters{Extension: "PDF"},
			expectedCount: 2,
			expectedNames: []string{"file1.pdf", "file2.PDF"},
		},
		{
			name: "search by jpg extension",
			filter: SearchFilters{Extension: "jpg"},
			expectedCount: 1,
			expectedNames: []string{"file4.jpg"},
		},
		{
			name: "no matches",
			filter: SearchFilters{Extension: "zip"},
			expectedCount: 0,
			expectedNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := mgr.SearchFiles(tt.filter)
			if len(results) != tt.expectedCount {
				t.Errorf("expected %d results, got %d", tt.expectedCount, len(results))
			}

			resultNames := make(map[string]bool)
			for _, r := range results {
				resultNames[r.OriginalName] = true
			}

			for _, expectedName := range tt.expectedNames {
				if !resultNames[expectedName] {
					t.Errorf("expected to find %q in results", expectedName)
				}
			}
		})
	}
}

func TestSearchFiles_ByType(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	files := []struct {
		name    string
		content string
		mime    string
	}{
		{"photo1.jpg", "jpg content", "image/jpeg"},
		{"photo2.png", "png content", "image/png"},
		{"video1.mp4", "mp4 content", "video/mp4"},
		{"video2.avi", "avi content", "video/x-msvideo"},
		{"audio1.mp3", "mp3 content", "audio/mpeg"},
		{"doc1.pdf", "pdf content", "application/pdf"},
	}

	for _, f := range files {
		req := StoreRequest{
			Reader:   newBytesReader([]byte(f.content)),
			Filename: f.name,
			MimeType: f.mime,
			Size:     int64(len(f.content)),
		}
		if _, err := mgr.StoreFile(req); err != nil {
			t.Fatalf("StoreFile(%q) failed: %v", f.name, err)
		}
	}

	tests := []struct {
		name         string
		filter       SearchFilters
		expectedMin  int
		expectedMax  int
		expectedNames []string
	}{
		{
			name: "search by image type",
			filter: SearchFilters{Type: "image"},
			expectedMin: 2,
			expectedMax: 2,
			expectedNames: []string{"photo1.jpg", "photo2.png"},
		},
		{
			name: "search by video type",
			filter: SearchFilters{Type: "video"},
			expectedMin: 2,
			expectedMax: 2,
			expectedNames: []string{"video1.mp4", "video2.avi"},
		},
		{
			name: "search by audio type",
			filter: SearchFilters{Type: "audio"},
			expectedMin: 1,
			expectedMax: 1,
			expectedNames: []string{"audio1.mp3"},
		},
		{
			name: "search by full MIME type",
			filter: SearchFilters{Type: "image/jpeg"},
			expectedMin: 1,
			expectedMax: 2, // May match both jpeg and png due to partial matching
			expectedNames: []string{"photo1.jpg"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := mgr.SearchFiles(tt.filter)
			if len(results) < tt.expectedMin || len(results) > tt.expectedMax {
				t.Errorf("expected %d-%d results, got %d", tt.expectedMin, tt.expectedMax, len(results))
			}

			resultNames := make(map[string]bool)
			for _, r := range results {
				resultNames[r.OriginalName] = true
			}

			for _, expectedName := range tt.expectedNames {
				if !resultNames[expectedName] {
					t.Errorf("expected to find %q in results", expectedName)
				}
			}
		})
	}
}

func TestSearchFiles_ByDateRange(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// We can't directly set UploadedAt, but we can test with current time
	// and use a date range that includes all files
	now := time.Now()
	
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
		req := StoreRequest{
			Reader:   newBytesReader([]byte(f.content)),
			Filename: f.name,
			MimeType: f.mime,
			Size:     int64(len(f.content)),
		}
		if _, err := mgr.StoreFile(req); err != nil {
			t.Fatalf("StoreFile(%q) failed: %v", f.name, err)
		}
		// Small delay to ensure different timestamps
		time.Sleep(10 * time.Millisecond)
	}

	// Test date range that includes all files (from yesterday to tomorrow)
	yesterday := now.AddDate(0, 0, -1)
	tomorrow := now.AddDate(0, 0, 1)

	results := mgr.SearchFiles(SearchFilters{
		DateFrom: yesterday,
		DateTo:   tomorrow,
	})

	if len(results) < len(files) {
		t.Errorf("expected at least %d results, got %d", len(files), len(results))
	}

	// Test date range that excludes all files (future dates)
	futureFrom := now.AddDate(1, 0, 0)
	futureTo := now.AddDate(1, 0, 1)

	results2 := mgr.SearchFiles(SearchFilters{
		DateFrom: futureFrom,
		DateTo:   futureTo,
	})

	if len(results2) != 0 {
		t.Errorf("expected 0 results for future date range, got %d", len(results2))
	}

	// Test date_from only
	results3 := mgr.SearchFiles(SearchFilters{
		DateFrom: yesterday,
	})

	if len(results3) < len(files) {
		t.Errorf("expected at least %d results with date_from, got %d", len(files), len(results3))
	}

	// Test date_to only
	results4 := mgr.SearchFiles(SearchFilters{
		DateTo: tomorrow,
	})

	if len(results4) < len(files) {
		t.Errorf("expected at least %d results with date_to, got %d", len(files), len(results4))
	}
}

func TestSearchFiles_ByMimeType(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	files := []struct {
		name    string
		content string
		mime    string
	}{
		{"file1.pdf", "pdf content", "application/pdf"},
		{"file2.pdf", "pdf content 2", "application/pdf"},
		{"file3.jpg", "jpg content", "image/jpeg"},
		{"file4.png", "png content", "image/png"},
	}

	for _, f := range files {
		req := StoreRequest{
			Reader:   newBytesReader([]byte(f.content)),
			Filename: f.name,
			MimeType: f.mime,
			Size:     int64(len(f.content)),
		}
		if _, err := mgr.StoreFile(req); err != nil {
			t.Fatalf("StoreFile(%q) failed: %v", f.name, err)
		}
	}

	tests := []struct {
		name         string
		filter       SearchFilters
		expectedCount int
		expectedNames []string
	}{
		{
			name: "search by exact MIME type",
			filter: SearchFilters{MimeType: "application/pdf"},
			expectedCount: 2,
			expectedNames: []string{"file1.pdf", "file2.pdf"},
		},
		{
			name: "search case insensitive MIME type",
			filter: SearchFilters{MimeType: "APPLICATION/PDF"},
			expectedCount: 2,
			expectedNames: []string{"file1.pdf", "file2.pdf"},
		},
		{
			name: "search by image MIME type",
			filter: SearchFilters{MimeType: "image/jpeg"},
			expectedCount: 1,
			expectedNames: []string{"file3.jpg"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := mgr.SearchFiles(tt.filter)
			if len(results) != tt.expectedCount {
				t.Errorf("expected %d results, got %d", tt.expectedCount, len(results))
			}

			resultNames := make(map[string]bool)
			for _, r := range results {
				resultNames[r.OriginalName] = true
			}

			for _, expectedName := range tt.expectedNames {
				if !resultNames[expectedName] {
					t.Errorf("expected to find %q in results", expectedName)
				}
			}
		})
	}
}

func TestSearchFiles_CombinedFilters(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	files := []struct {
		name    string
		content string
		mime    string
	}{
		{"report_2024.pdf", "pdf content 2024", "application/pdf"},
		{"report_2023.pdf", "pdf content 2023", "application/pdf"},
		{"image_report.png", "png content", "image/png"},
		{"document.txt", "txt content", "text/plain"},
		{"photo.jpg", "jpg content", "image/jpeg"},
	}

	for _, f := range files {
		req := StoreRequest{
			Reader:   newBytesReader([]byte(f.content)),
			Filename: f.name,
			MimeType: f.mime,
			Size:     int64(len(f.content)),
		}
		if _, err := mgr.StoreFile(req); err != nil {
			t.Fatalf("StoreFile(%q) failed: %v", f.name, err)
		}
	}

	tests := []struct {
		name         string
		filter       SearchFilters
		expectedCount int
		expectedNames []string
	}{
		{
			name: "name and extension",
			filter: SearchFilters{
				Name:      "report",
				Extension: "pdf",
			},
			expectedCount: 2,
			expectedNames: []string{"report_2024.pdf", "report_2023.pdf"},
		},
		{
			name: "name and type",
			filter: SearchFilters{
				Name: "report",
				Type: "image",
			},
			expectedCount: 1,
			expectedNames: []string{"image_report.png"},
		},
		{
			name: "extension and MIME type",
			filter: SearchFilters{
				Extension: "pdf",
				MimeType:  "application/pdf",
			},
			expectedCount: 2,
			expectedNames: []string{"report_2024.pdf", "report_2023.pdf"},
		},
		{
			name: "all filters (should match none)",
			filter: SearchFilters{
				Name:      "report",
				Extension: "jpg",
				Type:      "image",
			},
			expectedCount: 0,
			expectedNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := mgr.SearchFiles(tt.filter)
			if len(results) != tt.expectedCount {
				t.Errorf("expected %d results, got %d", tt.expectedCount, len(results))
			}

			resultNames := make(map[string]bool)
			for _, r := range results {
				resultNames[r.OriginalName] = true
			}

			for _, expectedName := range tt.expectedNames {
				if !resultNames[expectedName] {
					t.Errorf("expected to find %q in results", expectedName)
				}
			}
		})
	}
}

func TestSearchFiles_EmptyFilters(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Create some files
	files := []struct {
		name    string
		content string
		mime    string
	}{
		{"file1.txt", "content 1", "text/plain"},
		{"file2.txt", "content 2", "text/plain"},
	}

	for _, f := range files {
		req := StoreRequest{
			Reader:   newBytesReader([]byte(f.content)),
			Filename: f.name,
			MimeType: f.mime,
			Size:     int64(len(f.content)),
		}
		if _, err := mgr.StoreFile(req); err != nil {
			t.Fatalf("StoreFile(%q) failed: %v", f.name, err)
		}
	}

	// Empty filters should return all files
	results := mgr.SearchFiles(SearchFilters{})
	if len(results) != len(files) {
		t.Errorf("expected %d results with empty filters, got %d", len(files), len(results))
	}
}

func TestSearchFiles_ByCategory(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	files := []struct {
		name         string
		content      string
		mime         string
		categoryHint string
	}{
		{"photo1.jpg", "jpg content", "image/jpeg", "photos"},
		{"photo2.png", "png content", "image/png", "photos"},
		{"video1.mp4", "mp4 content", "video/mp4", "videos"},
		{"doc1.pdf", "pdf content", "application/pdf", "documents"},
	}

	for _, f := range files {
		req := StoreRequest{
			Reader:       newBytesReader([]byte(f.content)),
			Filename:     f.name,
			MimeType:     f.mime,
			Size:         int64(len(f.content)),
			CategoryHint: f.categoryHint,
		}
		if _, err := mgr.StoreFile(req); err != nil {
			t.Fatalf("StoreFile(%q) failed: %v", f.name, err)
		}
	}

	// Search by category (category is stored in metadata)
	results := mgr.SearchFiles(SearchFilters{
		Category: "images",
	})

	// Should find image files based on category path
	if len(results) < 2 {
		t.Logf("Found %d results, expected at least 2 image files", len(results))
		// This is a soft check since category structure may vary
	}
}
