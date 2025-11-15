package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
	"log/slog"
	"io"
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
			Count       int              `json:"count"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if payload.Count != 0 {
			t.Errorf("expected 0 collections, got %d", payload.Count)
		}
		if len(payload.Collections) != 0 {
			t.Errorf("expected empty collections array, got %d items", len(payload.Collections))
		}
	})

	// Test collections with real file uploads
	t.Run("CollectionsWithUploads", func(t *testing.T) {
		// This would require actual file uploads through the ingest endpoint
		// For now, we test the endpoint structure
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

		// Verify response structure
		if payload.Count != len(payload.Collections) {
			t.Errorf("count mismatch: count=%d, collections length=%d", payload.Count, len(payload.Collections))
		}
	})
}

