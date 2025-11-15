package api

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"log/slog"

	"github.com/Muneer320/RhinoBox/internal/config"
	"github.com/Muneer320/RhinoBox/internal/jsonschema"
)

func TestMediaIngestStoresFile(t *testing.T) {
	srv := newTestServer(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, err := writer.CreateFormFile("file", "cat.jpg")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	fileWriter.Write([]byte("fake image"))
	writer.WriteField("category", "wildlife")
	writer.WriteField("comment", "demo upload")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()

	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var payload struct {
		Stored []map[string]any `json:"stored"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Stored) != 1 {
		t.Fatalf("expected 1 stored item, got %d", len(payload.Stored))
	}
	pathVal, ok := payload.Stored[0]["path"].(string)
	if !ok || pathVal == "" {
		t.Fatalf("missing stored path in response: %+v", payload.Stored[0])
	}
	abs := filepath.Join(srv.cfg.DataDir, filepath.FromSlash(pathVal))
	if _, err := os.Stat(abs); err != nil {
		t.Fatalf("stored file missing: %v", err)
	}
}

func TestMediaIngestRequiresFile(t *testing.T) {
	srv := newTestServer(t)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("category", "empty")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()

	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestJSONIngestDecidesSQL(t *testing.T) {
	srv := newTestServer(t)
	docs := []map[string]any{
		{"id": 1, "user_id": 10, "amount": 100.0},
		{"id": 2, "user_id": 11, "amount": 200.0},
	}
	payload := map[string]any{
		"namespace": "orders",
		"documents": docs,
	}

	req := newJSONRequest(t, "/ingest/json", payload)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var body struct {
		Decision    jsonschema.Decision `json:"decision"`
		BatchPath   string              `json:"batch_path"`
		SchemaPath  string              `json:"schema_path"`
		DocumentCnt int                 `json:"documents"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Decision.Engine != "sql" {
		t.Fatalf("expected sql decision, got %s", body.Decision.Engine)
	}
	if body.SchemaPath == "" {
		t.Fatalf("expected schema path to be set")
	}
	abs := filepath.Join(srv.cfg.DataDir, filepath.FromSlash(body.SchemaPath))
	if _, err := os.Stat(abs); err != nil {
		t.Fatalf("schema file missing: %v", err)
	}
}

func TestJSONIngestDecidesNoSQL(t *testing.T) {
	srv := newTestServer(t)
	docs := []map[string]any{
		{
			"user": map[string]any{"id": "u1", "name": "Alice"},
			"events": []any{
				map[string]any{"type": "click", "meta": map[string]any{"at": "2025-11-15"}},
			},
		},
		{
			"user": map[string]any{"id": "u2"},
			"events": []any{
				map[string]any{"type": "view", "meta": map[string]any{"device": "mobile"}},
				map[string]any{"type": "purchase", "amount": 42, "items": []any{"book", "pen"}},
			},
		},
	}
	payload := map[string]any{
		"namespace": "activity",
		"documents": docs,
		"comment":   "flexible schema nosql high write",
	}

	req := newJSONRequest(t, "/ingest/json", payload)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var body struct {
		Decision   jsonschema.Decision `json:"decision"`
		SchemaPath string              `json:"schema_path"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Decision.Engine != "nosql" {
		t.Fatalf("expected nosql decision, got %s", body.Decision.Engine)
	}
	if body.SchemaPath != "" {
		t.Fatalf("expected no schema path for nosql, got %s", body.SchemaPath)
	}
}

func TestJSONIngestRequiresDocuments(t *testing.T) {
	srv := newTestServer(t)
	payload := map[string]any{
		"namespace": "empty",
	}
	req := newJSONRequest(t, "/ingest/json", payload)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := config.Config{
		Addr:           ":0",
		DataDir:        t.TempDir(),
		MaxUploadBytes: 32 * 1024 * 1024,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return srv
}

func newJSONRequest(t *testing.T, path string, payload any) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(payload); err != nil {
		t.Fatalf("encode payload: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	return req
}
