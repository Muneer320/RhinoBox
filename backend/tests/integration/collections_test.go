package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
	"io"
	"log/slog"
)

// TestCollectionsEndpointIntegration tests the collections endpoint end-to-end
func TestCollectionsEndpointIntegration(t *testing.T) {
	// Setup test server
	cfg := config.Config{
		Addr:           ":0",
		DataDir:        t.TempDir(),
		MaxUploadBytes: 32 * 1024 * 1024,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := api.NewServer(cfg, logger)
	t.Cleanup(func() {
		if srv != nil {
			srv.Stop()
		}
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Test empty collections
	t.Run("EmptyCollections", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/collections", nil)
		resp := httptest.NewRecorder()
		srv.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}

		var payload struct {
			Collections []map[string]any `json:"collections"`
			Count       int              `json:"total"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if payload.Count != len(payload.Collections) {
			t.Errorf("total mismatch: %d != %d", payload.Count, len(payload.Collections))
		}
		for _, collection := range payload.Collections {
			stats, ok := collection["stats"].(map[string]any)
			if !ok || stats["file_count"] != float64(0) {
				t.Errorf("empty collection has unexpected stats: %v", collection)
			}
		}
	})

	// Test collections response structure (without uploads)
	t.Run("CollectionsListStructure", func(t *testing.T) {
		// This test validates the response structure without performing uploads
		// For uploads and full end-to-end testing, see collections_e2e_test.go
		req := httptest.NewRequest(http.MethodGet, "/collections", nil)
		resp := httptest.NewRecorder()
		srv.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}

		var payload struct {
			Collections []map[string]any `json:"collections"`
			Count       int              `json:"total"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		// Verify response structure
		if payload.Count != len(payload.Collections) {
			t.Errorf("count mismatch: count=%d, collections length=%d", payload.Count, len(payload.Collections))
		}
	})
}
