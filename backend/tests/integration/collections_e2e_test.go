package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
	"log/slog"
)

// TestCollectionsEndToEnd tests the complete flow: upload files -> get collections -> verify metadata
func TestCollectionsEndToEnd(t *testing.T) {
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

	// Step 1: Upload files to different collections
	testFiles := []struct {
		filename string
		content  []byte
		category string
		expectedCollection string
	}{
		{"test_image.jpg", []byte("fake jpeg image data"), "wildlife", "images"},
		{"test_video.mp4", []byte("fake mp4 video data"), "demo", "videos"},
		{"test_audio.mp3", []byte("fake mp3 audio data"), "music", "audio"},
		{"test_doc.pdf", []byte("fake pdf document data"), "documents", "documents"},
	}

	uploadedHashes := make(map[string]string)

	for _, tf := range testFiles {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fileWriter, err := writer.CreateFormFile("file", tf.filename)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		fileWriter.Write(tf.content)
		writer.WriteField("category", tf.category)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()
		srv.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200 for upload %s, got %d: %s", tf.filename, resp.Code, resp.Body.String())
		}

		var uploadPayload struct {
			Stored []map[string]any `json:"stored"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&uploadPayload); err != nil {
			t.Fatalf("decode upload response: %v", err)
		}
		if len(uploadPayload.Stored) != 1 {
			t.Fatalf("expected 1 stored item, got %d", len(uploadPayload.Stored))
		}

		hash, ok := uploadPayload.Stored[0]["hash"].(string)
		if !ok || hash == "" {
			t.Fatalf("missing hash in upload response for %s", tf.filename)
		}
		uploadedHashes[tf.expectedCollection] = hash
	}

	// Step 2: Get collections
	req := httptest.NewRequest(http.MethodGet, "/collections", nil)
	resp := httptest.NewRecorder()
	srv.Router().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var payload struct {
		Collections []map[string]any `json:"collections"`
		Count       int              `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Step 3: Verify collections
	if payload.Count < len(testFiles) {
		t.Errorf("expected at least %d collections, got %d", len(testFiles), payload.Count)
	}

	// Verify each expected collection exists and has correct metadata
	collectionsByType := make(map[string]map[string]any)
	for _, coll := range payload.Collections {
		collType, ok := coll["type"].(string)
		if !ok {
			t.Errorf("collection missing type field: %+v", coll)
			continue
		}
		collectionsByType[collType] = coll
	}

	for _, tf := range testFiles {
		coll, exists := collectionsByType[tf.expectedCollection]
		if !exists {
			t.Errorf("expected collection type '%s' not found", tf.expectedCollection)
			continue
		}

		// Verify metadata fields
		if name, ok := coll["name"].(string); !ok || name == "" {
			t.Errorf("collection '%s' missing or empty name", tf.expectedCollection)
		}
		if desc, ok := coll["description"].(string); !ok || desc == "" {
			t.Errorf("collection '%s' missing or empty description", tf.expectedCollection)
		}
		if icon, ok := coll["icon"].(string); !ok || icon == "" {
			t.Errorf("collection '%s' missing or empty icon", tf.expectedCollection)
		}
		if fileCount, ok := coll["file_count"].(float64); !ok || fileCount < 1 {
			t.Errorf("collection '%s' should have at least 1 file, got %v", tf.expectedCollection, fileCount)
		}
		if totalSize, ok := coll["total_size"].(float64); !ok || totalSize <= 0 {
			t.Errorf("collection '%s' should have positive total_size, got %v", tf.expectedCollection, totalSize)
		}
		if formattedSize, ok := coll["formatted_size"].(string); !ok || formattedSize == "" {
			t.Errorf("collection '%s' missing or empty formatted_size", tf.expectedCollection)
		}
	}

	// Step 4: Verify file counts match uploaded files
	totalFiles := 0
	for _, coll := range payload.Collections {
		if fileCount, ok := coll["file_count"].(float64); ok {
			totalFiles += int(fileCount)
		}
	}
	if totalFiles < len(testFiles) {
		t.Errorf("expected at least %d total files across collections, got %d", len(testFiles), totalFiles)
	}
}


