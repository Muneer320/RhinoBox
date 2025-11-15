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

		// Verify stats
		if stats["collection_type"].(string) != collection {
			t.Errorf("expected collection_type %s, got %s", collection, stats["collection_type"].(string))
		}

		fileCount := int(stats["file_count"].(float64))
		expectedCount := len(hashes)
		if fileCount != expectedCount {
			t.Errorf("expected file_count %d for %s, got %d", expectedCount, collection, fileCount)
		}

		storageBytes := int64(stats["storage_bytes"].(float64))
		if storageBytes <= 0 {
			t.Errorf("expected storage_bytes > 0 for %s, got %d", collection, storageBytes)
		}

		storageUsed := stats["storage_used"].(string)
		if storageUsed == "" {
			t.Errorf("expected storage_used to be formatted for %s, got empty", collection)
		}

		lastUpdated := stats["last_updated"]
		if lastUpdated == nil {
			t.Errorf("expected last_updated to be set for %s, got nil", collection)
		}
	}

	// Step 3: Test empty collection
	emptyStats := getCollectionStats(t, srv, "code")
	if emptyStats["file_count"].(float64) != 0 {
		t.Errorf("expected file_count 0 for empty collection, got %v", emptyStats["file_count"])
	}
	if emptyStats["storage_bytes"].(float64) != 0 {
		t.Errorf("expected storage_bytes 0 for empty collection, got %v", emptyStats["storage_bytes"])
	}
	if emptyStats["last_updated"] != nil {
		t.Errorf("expected last_updated to be nil for empty collection, got %v", emptyStats["last_updated"])
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

