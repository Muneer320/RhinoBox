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

// TestStatisticsEndToEnd tests the complete statistics flow
func TestStatisticsEndToEnd(t *testing.T) {
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

	// Step 1: Get statistics when empty
	statsEmpty := getStatistics(t, srv)
	if statsEmpty["totalFiles"].(float64) != 0 {
		t.Errorf("expected 0 files initially, got %.0f", statsEmpty["totalFiles"].(float64))
	}
	if statsEmpty["collections"].(float64) != 0 {
		t.Errorf("expected 0 collections initially, got %.0f", statsEmpty["collections"].(float64))
	}

	// Step 2: Upload multiple files of different types
	testFiles := []struct {
		name     string
		content  []byte
		category  string
		mimeType string
	}{
		{"image1.jpg", []byte("fake jpeg content"), "images", "image/jpeg"},
		{"image2.png", []byte("fake png content longer"), "images", "image/png"},
		{"document.pdf", []byte("fake pdf document content"), "documents", "application/pdf"},
		{"video.mp4", []byte("fake video content even longer"), "videos", "video/mp4"},
	}

	var uploadedHashes []string
	for _, file := range testFiles {
		hash, _ := uploadTestFile(t, srv, file.name, file.content, file.mimeType)
		uploadedHashes = append(uploadedHashes, hash)
	}

	// Step 3: Get statistics after uploads
	statsAfter := getStatistics(t, srv)
	expectedFiles := float64(len(testFiles))
	if statsAfter["totalFiles"].(float64) != expectedFiles {
		t.Errorf("expected %d files, got %.0f", len(testFiles), statsAfter["totalFiles"].(float64))
	}

	// Verify storage is calculated correctly
	storageUsedBytes, ok := statsAfter["storageUsedBytes"].(float64)
	if !ok {
		t.Errorf("expected storageUsedBytes to be a number")
	}
	if storageUsedBytes <= 0 {
		t.Errorf("expected storageUsedBytes > 0, got %.0f", storageUsedBytes)
	}

	// Verify storage formatting
	storageUsed, ok := statsAfter["storageUsed"].(string)
	if !ok || storageUsed == "" {
		t.Errorf("expected storageUsed to be a non-empty string")
	}

	// Verify collection count
	collections, ok := statsAfter["collections"].(float64)
	if !ok {
		t.Errorf("expected collections to be a number")
	}
	if collections < 2 {
		t.Errorf("expected at least 2 collections (images, documents, videos), got %.0f", collections)
	}

	// Verify collection details
	collectionDetails, ok := statsAfter["collectionDetails"].(map[string]interface{})
	if !ok {
		t.Errorf("expected collectionDetails to be a map")
	}
	if len(collectionDetails) < 2 {
		t.Errorf("expected at least 2 collection types in details, got %d", len(collectionDetails))
	}

	// Step 4: Delete a file and verify statistics update
	deleteReq := httptest.NewRequest(http.MethodDelete, "/files/"+uploadedHashes[0], nil)
	deleteResp := httptest.NewRecorder()
	srv.Router().ServeHTTP(deleteResp, deleteReq)

	if deleteResp.Code != http.StatusOK {
		t.Fatalf("expected 200 for delete, got %d: %s", deleteResp.Code, deleteResp.Body.String())
	}

	// Step 5: Get statistics after deletion
	statsAfterDelete := getStatistics(t, srv)
	expectedFilesAfterDelete := expectedFiles - 1
	if statsAfterDelete["totalFiles"].(float64) != expectedFilesAfterDelete {
		t.Errorf("expected %.0f files after delete, got %.0f", expectedFilesAfterDelete, statsAfterDelete["totalFiles"].(float64))
	}

	// Verify storage decreased
	storageAfterDelete, ok := statsAfterDelete["storageUsedBytes"].(float64)
	if !ok {
		t.Errorf("expected storageUsedBytes to be a number after delete")
	}
	if storageAfterDelete >= storageUsedBytes {
		t.Errorf("expected storage to decrease after deletion, got %.0f >= %.0f", storageAfterDelete, storageUsedBytes)
	}
}

// TestStatisticsResponseFormat tests that the response format matches frontend expectations
func TestStatisticsResponseFormat(t *testing.T) {
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

	// Upload a file
	uploadTestFile(t, srv, "test.jpg", []byte("test content"), "image/jpeg")

	// Get statistics
	stats := getStatistics(t, srv)

	// Verify all expected fields exist (for frontend compatibility)
	requiredFields := []string{
		"totalFiles",
		"files",              // Alias
		"storageUsed",
		"storage",           // Alias
		"collections",
		"collectionCount",   // Alias
		"storageUsedBytes",
		"collectionDetails",
	}

	for _, field := range requiredFields {
		if _, ok := stats[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}

	// Verify aliases match
	if stats["totalFiles"] != stats["files"] {
		t.Errorf("files alias should match totalFiles")
	}
	if stats["storageUsed"] != stats["storage"] {
		t.Errorf("storage alias should match storageUsed")
	}
	if stats["collections"] != stats["collectionCount"] {
		t.Errorf("collectionCount alias should match collections")
	}
}

// getStatistics is a helper function to get statistics from the server
func getStatistics(t *testing.T, srv *api.Server) map[string]interface{} {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/statistics", nil)
	resp := httptest.NewRecorder()
	srv.Router().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var stats map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		t.Fatalf("decode statistics response: %v", err)
	}

	return stats
}


