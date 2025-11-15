package integration

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
	"log/slog"
	"io"
)

// TestCollectionsEndToEnd tests the complete flow of collections API
func TestCollectionsEndToEnd(t *testing.T) {
	// Setup test server
	cfg := config.Config{
		Addr:           ":0",
		DataDir:        t.TempDir(),
		MaxUploadBytes: 32 * 1024 * 1024,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Step 1: Get all collections
	t.Run("GetAllCollections", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/collections", nil)
		resp := httptest.NewRecorder()
		srv.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}

		var payload struct {
			Collections []struct {
				Type        string `json:"type"`
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"collections"`
			Count int `json:"count"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if payload.Count < 8 {
			t.Errorf("expected at least 8 collections, got %d", payload.Count)
		}
	})

	// Step 2: Upload files to different collections
	t.Run("UploadFilesToCollections", func(t *testing.T) {
		files := []struct {
			filename string
			mimeType string
			data     []byte
			collection string
		}{
			{"test.jpg", "image/jpeg", []byte("fake image data"), "images"},
			{"test.mp4", "video/mp4", []byte("fake video data"), "videos"},
			{"test.mp3", "audio/mpeg", []byte("fake audio data"), "audio"},
			{"test.pdf", "application/pdf", []byte("fake pdf data"), "documents"},
		}

		for _, file := range files {
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			fileWriter, err := writer.CreateFormFile("file", file.filename)
			if err != nil {
				t.Fatalf("create form file: %v", err)
			}
			fileWriter.Write(file.data)
			writer.Close()

			req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			resp := httptest.NewRecorder()
			srv.Router().ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("failed to upload %s: got %d: %s", file.filename, resp.Code, resp.Body.String())
			}
		}
	})

	// Step 3: Get stats for each collection
	t.Run("GetCollectionStats", func(t *testing.T) {
		collections := []string{"images", "videos", "audio", "documents", "spreadsheets", "presentations", "archives", "other"}

		for _, collectionType := range collections {
			req := httptest.NewRequest(http.MethodGet, "/collections/"+collectionType+"/stats", nil)
			resp := httptest.NewRecorder()
			srv.Router().ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("expected 200 for %s stats, got %d: %s", collectionType, resp.Code, resp.Body.String())
			}

			var stats struct {
				Type                string `json:"type"`
				FileCount           int    `json:"file_count"`
				StorageUsed         int64  `json:"storage_used"`
				StorageUsedFormatted string `json:"storage_used_formatted"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
				t.Fatalf("decode stats for %s: %v", collectionType, err)
			}

			if stats.Type != collectionType {
				t.Errorf("expected type %s, got %s", collectionType, stats.Type)
			}

			// Verify stats are non-negative
			if stats.FileCount < 0 {
				t.Errorf("file count should be non-negative, got %d", stats.FileCount)
			}
			if stats.StorageUsed < 0 {
				t.Errorf("storage used should be non-negative, got %d", stats.StorageUsed)
			}
			if stats.StorageUsedFormatted == "" {
				t.Errorf("storage formatted should not be empty")
			}
		}
	})

	// Step 4: Verify stats reflect uploaded files
	t.Run("VerifyStatsReflectUploads", func(t *testing.T) {
		imagesStatsReq := httptest.NewRequest(http.MethodGet, "/collections/images/stats", nil)
		imagesStatsResp := httptest.NewRecorder()
		srv.Router().ServeHTTP(imagesStatsResp, imagesStatsReq)

		if imagesStatsResp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", imagesStatsResp.Code)
		}

		var stats struct {
			FileCount int `json:"file_count"`
		}
		if err := json.NewDecoder(imagesStatsResp.Body).Decode(&stats); err != nil {
			t.Fatalf("decode stats: %v", err)
		}

		if stats.FileCount < 1 {
			t.Errorf("expected at least 1 image file, got %d", stats.FileCount)
		}
	})
}

// TestCollectionsPerformance tests performance of collections endpoints
func TestCollectionsPerformance(t *testing.T) {
	cfg := config.Config{
		Addr:           ":0",
		DataDir:        t.TempDir(),
		MaxUploadBytes: 32 * 1024 * 1024,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Benchmark getting collections
	t.Run("BenchmarkGetCollections", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/collections", nil)
		
		for i := 0; i < 100; i++ {
			resp := httptest.NewRecorder()
			srv.Router().ServeHTTP(resp, req)
			if resp.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", resp.Code)
			}
		}
	})

	// Benchmark getting stats
	t.Run("BenchmarkGetStats", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/collections/images/stats", nil)
		
		for i := 0; i < 100; i++ {
			resp := httptest.NewRecorder()
			srv.Router().ServeHTTP(resp, req)
			if resp.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", resp.Code)
			}
		}
	})
}

