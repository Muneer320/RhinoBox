package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
	"log/slog"
)

// TestCollectionStatsEndToEnd tests the complete flow: upload files -> get collection stats
func TestCollectionStatsEndToEnd(t *testing.T) {
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

	// Test data: files for different collections
	testFiles := []struct {
		filename string
		content  []byte
		expectedCollection string
	}{
		{"test1.jpg", []byte("fake image 1"), "images"},
		{"test2.png", []byte("fake image 2"), "images"},
		{"test3.mp4", []byte("fake video content"), "videos"},
		{"test4.mp3", []byte("fake audio content"), "audio"},
		{"test5.pdf", []byte("fake document content"), "documents"},
	}

	// Step 1: Upload files to different collections
	uploadedFiles := make(map[string][]string) // collection -> []hashes
	for _, file := range testFiles {
		hash, _ := uploadTestFile(t, srv, file.filename, file.content, "")
		if uploadedFiles[file.expectedCollection] == nil {
			uploadedFiles[file.expectedCollection] = make([]string, 0)
		}
		uploadedFiles[file.expectedCollection] = append(uploadedFiles[file.expectedCollection], hash)
	}

	// Step 2: Get stats for each collection
	for collection, hashes := range uploadedFiles {
		stats := getCollectionStats(t, srv, collection)

		// Verify stats - API returns: type, file_count, storage_used (bytes), storage_used_formatted
		typeVal, ok := stats["type"].(string)
		if !ok || typeVal != collection {
			t.Errorf("expected type %s, got %v", collection, stats["type"])
		}

		fileCountVal, ok := stats["file_count"].(float64)
		if !ok {
			t.Errorf("file_count missing or wrong type for %s", collection)
			continue
		}
		fileCount := int(fileCountVal)
		expectedCount := len(hashes)
		if fileCount != expectedCount {
			t.Errorf("expected file_count %d for %s, got %d", expectedCount, collection, fileCount)
		}

		storageBytesVal, ok := stats["storage_used"].(float64)
		if !ok {
			t.Errorf("storage_used missing or wrong type for %s", collection)
			continue
		}
		storageBytes := int64(storageBytesVal)
		if storageBytes <= 0 {
			t.Errorf("expected storage_used > 0 for %s, got %d", collection, storageBytes)
		}

		storageUsedFormatted, ok := stats["storage_used_formatted"].(string)
		if !ok || storageUsedFormatted == "" {
			t.Errorf("expected storage_used_formatted to be set for %s, got %v", collection, stats["storage_used_formatted"])
		}
	}

	// Step 3: Test empty collection
	emptyStats := getCollectionStats(t, srv, "code")
	fileCountVal, ok := emptyStats["file_count"].(float64)
	if !ok || int(fileCountVal) != 0 {
		t.Errorf("expected file_count 0 for empty collection, got %v", emptyStats["file_count"])
	}
	storageUsedVal, ok := emptyStats["storage_used"].(float64)
	if !ok || int64(storageUsedVal) != 0 {
		t.Errorf("expected storage_used 0 for empty collection, got %v", emptyStats["storage_used"])
	}
	// Verify formatted storage is still present (should be "0 B" or similar)
	storageUsedFormatted, ok := emptyStats["storage_used_formatted"].(string)
	if !ok || storageUsedFormatted == "" {
		t.Errorf("expected storage_used_formatted to be set for empty collection, got %v", emptyStats["storage_used_formatted"])
	}
}

// getCollectionStats makes a request to get collection statistics
func getCollectionStats(t *testing.T, srv *api.Server, collectionType string) map[string]interface{} {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/collections/"+collectionType+"/stats", nil)
	resp := httptest.NewRecorder()
	srv.Router().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var stats map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return stats
}

