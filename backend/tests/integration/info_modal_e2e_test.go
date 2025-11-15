package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
	"log/slog"
)

// TestInfoModalMetadataEndpoint tests the metadata endpoint used by the info modal
func TestInfoModalMetadataEndpoint(t *testing.T) {
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

	// Step 1: Upload a test file
	testContent := []byte("This is a test file for info modal metadata testing. " + strings.Repeat("Content ", 50))
	testFilename := "info_modal_test.txt"
	uploadHash, _ := uploadTestFile(t, srv, testFilename, testContent, "text/plain")

	// Step 2: Test metadata endpoint with valid hash
	t.Run("ValidHash", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/files/metadata?hash=%s", uploadHash), nil)
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
		}

		var metadata map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &metadata); err != nil {
			t.Fatalf("Unmarshal metadata: %v", err)
		}

		// Verify required fields for info modal
		requiredFields := []string{"hash", "original_name", "stored_path", "category", "mime_type", "size", "uploaded_at"}
		for _, field := range requiredFields {
			if _, ok := metadata[field]; !ok {
				t.Errorf("missing required field: %s", field)
			}
		}

		// Verify values
		if metadata["hash"].(string) != uploadHash {
			t.Errorf("expected hash %s, got %s", uploadHash, metadata["hash"].(string))
		}
		if metadata["original_name"].(string) != testFilename {
			t.Errorf("expected filename %s, got %s", testFilename, metadata["original_name"].(string))
		}
		if metadata["size"].(float64) != float64(len(testContent)) {
			t.Errorf("expected size %d, got %v", len(testContent), metadata["size"])
		}
	})

	// Step 3: Test metadata endpoint with missing hash
	t.Run("MissingHash", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/files/metadata", nil)
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d, body: %s", w.Code, w.Body.String())
		}
	})

	// Step 4: Test metadata endpoint with invalid hash
	t.Run("InvalidHash", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/files/metadata?hash=invalid_hash_123", nil)
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d, body: %s", w.Code, w.Body.String())
		}
	})

	// Step 5: Test metadata endpoint with non-existent hash
	t.Run("NonExistentHash", func(t *testing.T) {
		nonExistentHash := "a1b2c3d4e5f6789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234"
		req := httptest.NewRequest("GET", fmt.Sprintf("/files/metadata?hash=%s", nonExistentHash), nil)
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d, body: %s", w.Code, w.Body.String())
		}
	})
}

// TestInfoModalMetadataCompleteData tests that metadata contains all necessary data for display
func TestInfoModalMetadataCompleteData(t *testing.T) {
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

	// Upload different file types to test various metadata scenarios
	testCases := []struct {
		name        string
		filename    string
		content     []byte
		mimeType    string
		expectSize  int
		expectCategory string
	}{
		{
			name:        "TextFile",
			filename:    "test.txt",
			content:     []byte("Simple text content"),
			mimeType:    "text/plain",
			expectSize:  18,
			expectCategory: "documents/txt",
		},
		{
			name:        "JSONFile",
			filename:    "data.json",
			content:     []byte(`{"key": "value"}`),
			mimeType:    "application/json",
			expectSize:  17,
			expectCategory: "code/json",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hash, _ := uploadTestFile(t, srv, tc.filename, tc.content, tc.mimeType)

			req := httptest.NewRequest("GET", fmt.Sprintf("/files/metadata?hash=%s", hash), nil)
			w := httptest.NewRecorder()
			srv.Router().ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d", w.Code)
			}

			var metadata map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &metadata); err != nil {
				t.Fatalf("Unmarshal metadata: %v", err)
			}

			// Verify all fields needed for info modal display
			if metadata["hash"].(string) != hash {
				t.Errorf("hash mismatch")
			}
			if metadata["original_name"].(string) != tc.filename {
				t.Errorf("filename mismatch: expected %s, got %s", tc.filename, metadata["original_name"].(string))
			}
			if int(metadata["size"].(float64)) != tc.expectSize {
				t.Errorf("size mismatch: expected %d, got %v", tc.expectSize, metadata["size"])
			}
			if metadata["mime_type"].(string) != tc.mimeType {
				t.Errorf("mime_type mismatch: expected %s, got %s", tc.mimeType, metadata["mime_type"].(string))
			}
			if !strings.Contains(metadata["category"].(string), tc.expectCategory) {
				t.Errorf("category mismatch: expected to contain %s, got %s", tc.expectCategory, metadata["category"].(string))
			}
			if metadata["stored_path"].(string) == "" {
				t.Error("stored_path is empty")
			}
			if metadata["uploaded_at"] == nil {
				t.Error("uploaded_at is missing")
			}
		})
	}
}

