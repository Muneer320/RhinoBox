package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
)

// setupTestServer creates a test server with temporary data directory.
func setupTestServer(t *testing.T) (*httptest.Server, *api.Server) {
	tmpDir := t.TempDir()
	cfg := config.Config{
		DataDir:       tmpDir,
		Addr:          ":0",
		MaxUploadBytes: 100 * 1024 * 1024, // 100MB
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	ts := httptest.NewServer(srv.Router())
	return ts, srv
}

// uploadTestFileForCollections uploads a test file to the server for collection tests.
func uploadTestFileForCollections(t *testing.T, serverURL, filename, mimeType string, content []byte) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Create form file
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}

	if _, err := part.Write(content); err != nil {
		t.Fatalf("failed to write file content: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	req, err := http.NewRequest("POST", serverURL+"/ingest/media", &buf)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}
}

func TestCollectionsEndpoint_EmptyStorage(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/collections")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify response structure
	if _, ok := result["collections"]; !ok {
		t.Error("expected 'collections' field in response")
	}
	if _, ok := result["total"]; !ok {
		t.Error("expected 'total' field in response")
	}
	if _, ok := result["generated_at"]; !ok {
		t.Error("expected 'generated_at' field in response")
	}

	collections, ok := result["collections"].([]interface{})
	if !ok {
		t.Fatal("expected 'collections' to be an array")
	}

	if len(collections) == 0 {
		t.Error("expected at least some collection types")
	}
}

func TestCollectionsEndpoint_WithFiles(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()

	// Upload test files
	uploadTestFileForCollections(t, ts.URL, "test1.jpg", "image/jpeg", []byte("fake jpeg content"))
	uploadTestFileForCollections(t, ts.URL, "test2.mp4", "video/mp4", []byte("fake mp4 content"))
	uploadTestFileForCollections(t, ts.URL, "test3.mp3", "audio/mpeg", []byte("fake mp3 content"))

	// Get collections
	resp, err := http.Get(ts.URL + "/collections")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 200, got %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Collections []struct {
			Type        string `json:"type"`
			DisplayName string `json:"display_name"`
			Stats       struct {
				FileCount   int    `json:"file_count"`
				StorageUsed int64  `json:"storage_used"`
			} `json:"stats"`
		} `json:"collections"`
		Total int `json:"total"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Find collections with files
	imagesFound := false
	videosFound := false
	audioFound := false

	for _, c := range result.Collections {
		switch c.Type {
		case "images":
			imagesFound = true
			if c.Stats.FileCount < 1 {
				t.Errorf("expected at least 1 image file, got %d", c.Stats.FileCount)
			}
		case "videos":
			videosFound = true
			if c.Stats.FileCount < 1 {
				t.Errorf("expected at least 1 video file, got %d", c.Stats.FileCount)
			}
		case "audio":
			audioFound = true
			if c.Stats.FileCount < 1 {
				t.Errorf("expected at least 1 audio file, got %d", c.Stats.FileCount)
			}
		}
	}

	if !imagesFound {
		t.Error("expected images collection to be found")
	}
	if !videosFound {
		t.Error("expected videos collection to be found")
	}
	if !audioFound {
		t.Error("expected audio collection to be found")
	}
}

func TestCollectionStatsEndpoint_ValidType(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()

	// Upload test files
	uploadTestFileForCollections(t, ts.URL, "test1.jpg", "image/jpeg", []byte("fake jpeg content 1"))
	uploadTestFileForCollections(t, ts.URL, "test2.jpg", "image/jpeg", []byte("fake jpeg content 2"))

	// Get stats for images
	resp, err := http.Get(ts.URL + "/collections/images/stats")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 200, got %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Type        string `json:"type"`
		DisplayName string `json:"display_name"`
		Stats       struct {
			Type        string `json:"type"`
			FileCount   int    `json:"file_count"`
			StorageUsed int64  `json:"storage_used"`
		} `json:"stats"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Type != "images" {
		t.Errorf("expected type 'images', got '%s'", result.Type)
	}
	if result.DisplayName != "Images" {
		t.Errorf("expected display name 'Images', got '%s'", result.DisplayName)
	}
	if result.Stats.FileCount < 2 {
		t.Errorf("expected at least 2 files, got %d", result.Stats.FileCount)
	}
	if result.Stats.StorageUsed == 0 {
		t.Error("expected storage used to be greater than 0")
	}
}

func TestCollectionStatsEndpoint_InvalidType(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/collections/invalid_type/stats")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if _, ok := result["error"]; !ok {
		t.Error("expected 'error' field in response")
	}
}

func TestCollectionStatsEndpoint_Caching(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()

	// Upload test file
	uploadTestFileForCollections(t, ts.URL, "test.jpg", "image/jpeg", []byte("fake jpeg content"))

	// First request
	resp1, err := http.Get(ts.URL + "/collections/images/stats")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp1.Body.Close()

	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp1.StatusCode)
	}

	var result1 struct {
		Stats struct {
			FileCount int `json:"file_count"`
		} `json:"stats"`
	}
	if err := json.NewDecoder(resp1.Body).Decode(&result1); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Second request - should use cache
	resp2, err := http.Get(ts.URL + "/collections/images/stats")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp2.StatusCode)
	}

	var result2 struct {
		Stats struct {
			FileCount int `json:"file_count"`
		} `json:"stats"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&result2); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Results should be the same
	if result1.Stats.FileCount != result2.Stats.FileCount {
		t.Errorf("cached result should match original: %d != %d", 
			result1.Stats.FileCount, result2.Stats.FileCount)
	}
}

func TestCollectionsEndpoint_Performance(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()

	// Upload multiple files
	for i := 0; i < 10; i++ {
		filename := fmt.Sprintf("test%d.jpg", i)
		uploadTestFileForCollections(t, ts.URL, filename, "image/jpeg", []byte("fake jpeg content"))
	}

	// Measure response time
	start := time.Now()
	resp, err := http.Get(ts.URL + "/collections")
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	// Response should be fast (under 1 second for 10 files)
	if duration > time.Second {
		t.Errorf("response took too long: %v", duration)
	}

	t.Logf("Collections endpoint response time: %v", duration)
}

func TestCollectionStatsEndpoint_AllTypes(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()

	// Test all valid collection types
	validTypes := []string{"images", "videos", "audio", "documents", "spreadsheets", 
		"presentations", "archives", "code", "other", "json"}

	for _, typ := range validTypes {
		t.Run(typ, func(t *testing.T) {
			resp, err := http.Get(ts.URL + "/collections/" + typ + "/stats")
			if err != nil {
				t.Fatalf("failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("expected status 200 for type %s, got %d, body: %s", 
					typ, resp.StatusCode, string(body))
			}

			var result struct {
				Type        string `json:"type"`
				DisplayName string `json:"display_name"`
			}

			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if result.Type != typ {
				t.Errorf("expected type %s, got %s", typ, result.Type)
			}
		})
	}
}

