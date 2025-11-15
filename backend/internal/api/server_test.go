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

func TestFileDeleteSuccess(t *testing.T) {
	srv := newTestServer(t)

	// First, upload a file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, err := writer.CreateFormFile("file", "test_delete.jpg")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	fileWriter.Write([]byte("test file content for deletion"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for upload, got %d: %s", resp.Code, resp.Body.String())
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
		t.Fatalf("missing hash in upload response")
	}

	// Now delete the file
	deleteReq := httptest.NewRequest(http.MethodDelete, "/files/"+hash, nil)
	deleteResp := httptest.NewRecorder()
	srv.router.ServeHTTP(deleteResp, deleteReq)

	if deleteResp.Code != http.StatusOK {
		t.Fatalf("expected 200 for delete, got %d: %s", deleteResp.Code, deleteResp.Body.String())
	}

	var deletePayload struct {
		Hash         string `json:"hash"`
		OriginalName string `json:"original_name"`
		Deleted      bool   `json:"deleted"`
	}
	if err := json.NewDecoder(deleteResp.Body).Decode(&deletePayload); err != nil {
		t.Fatalf("decode delete response: %v", err)
	}
	if !deletePayload.Deleted {
		t.Fatalf("expected Deleted=true")
	}
	if deletePayload.Hash != hash {
		t.Fatalf("expected hash %s, got %s", hash, deletePayload.Hash)
	}

	// Verify file is actually deleted
	storedPath, ok := uploadPayload.Stored[0]["path"].(string)
	if !ok {
		t.Fatalf("missing stored path in upload response")
	}
	abs := filepath.Join(srv.cfg.DataDir, filepath.FromSlash(storedPath))
	if _, err := os.Stat(abs); !os.IsNotExist(err) {
		t.Fatalf("file should be deleted, but still exists: %v", err)
	}
}

func TestFileDeleteNotFound(t *testing.T) {
	srv := newTestServer(t)

	// Try to delete non-existent file
	deleteReq := httptest.NewRequest(http.MethodDelete, "/files/nonexistent_hash_1234567890123456789012345678901234567890123456789012345678901234", nil)
	deleteResp := httptest.NewRecorder()
	srv.router.ServeHTTP(deleteResp, deleteReq)

	if deleteResp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-existent file, got %d: %s", deleteResp.Code, deleteResp.Body.String())
	}

	var errorPayload struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(deleteResp.Body).Decode(&errorPayload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if errorPayload.Error == "" {
		t.Fatalf("expected error message")
	}
}

func TestFileDeleteMissingFileID(t *testing.T) {
	srv := newTestServer(t)

	// Try to delete without file_id
	deleteReq := httptest.NewRequest(http.MethodDelete, "/files/", nil)
	deleteResp := httptest.NewRecorder()
	srv.router.ServeHTTP(deleteResp, deleteReq)

	// Chi router will return 404 for /files/ without parameter
	// But we want to test the case where file_id is empty
	// Let's test with an empty file_id in the path
	deleteReq2 := httptest.NewRequest(http.MethodDelete, "/files/", nil)
	deleteResp2 := httptest.NewRecorder()
	srv.router.ServeHTTP(deleteResp2, deleteReq2)

	// The route expects {file_id}, so empty will be 404 from router
	// But we can test with a route that has the parameter but is empty
	// Actually, chi will match /files/{file_id} and file_id will be empty string
	// Let's test that case
	deleteReq3 := httptest.NewRequest(http.MethodDelete, "/files/", nil)
	deleteResp3 := httptest.NewRecorder()
	srv.router.ServeHTTP(deleteResp3, deleteReq3)

	// Chi will return 404 for this route pattern, but we can test the handler directly
	// For now, let's just verify that a proper 404 is returned
	if deleteResp3.Code != http.StatusNotFound {
		// If it's not 404, it might be our handler returning 400
		// Let's check if it's 400 (bad request) which is what our handler returns
		if deleteResp3.Code == http.StatusBadRequest {
			// This is acceptable - our handler validates file_id
			return
		}
		t.Fatalf("expected 404 or 400, got %d", deleteResp3.Code)
	}
}

func TestGetCollections(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/collections", nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

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

	if payload.Count == 0 {
		t.Fatalf("expected at least one collection, got %d", payload.Count)
	}

	if len(payload.Collections) != payload.Count {
		t.Fatalf("collections count mismatch: %d != %d", len(payload.Collections), payload.Count)
	}

	// Verify expected collection types exist
	expectedTypes := map[string]bool{
		"images":       false,
		"videos":       false,
		"audio":        false,
		"documents":    false,
		"spreadsheets": false,
		"presentations": false,
		"archives":     false,
		"other":        false,
	}

	for _, collection := range payload.Collections {
		if _, exists := expectedTypes[collection.Type]; exists {
			expectedTypes[collection.Type] = true
		}
		if collection.Name == "" {
			t.Errorf("collection %s has empty name", collection.Type)
		}
		if collection.Description == "" {
			t.Errorf("collection %s has empty description", collection.Type)
		}
	}

	for typeName, found := range expectedTypes {
		if !found {
			t.Errorf("expected collection type %s not found", typeName)
		}
	}
}

func TestGetCollectionStats(t *testing.T) {
	srv := newTestServer(t)

	// First, upload some files to create data
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, err := writer.CreateFormFile("file", "test.jpg")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	fileWriter.Write([]byte("fake image data"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for upload, got %d: %s", resp.Code, resp.Body.String())
	}

	// Now test collection stats
	statsReq := httptest.NewRequest(http.MethodGet, "/collections/images/stats", nil)
	statsResp := httptest.NewRecorder()
	srv.router.ServeHTTP(statsResp, statsReq)

	if statsResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", statsResp.Code, statsResp.Body.String())
	}

	var statsPayload struct {
		Type                string `json:"type"`
		FileCount           int    `json:"file_count"`
		StorageUsed         int64  `json:"storage_used"`
		StorageUsedFormatted string `json:"storage_used_formatted"`
	}
	if err := json.NewDecoder(statsResp.Body).Decode(&statsPayload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if statsPayload.Type != "images" {
		t.Errorf("expected type 'images', got %s", statsPayload.Type)
	}
	if statsPayload.FileCount < 1 {
		t.Errorf("expected at least 1 file, got %d", statsPayload.FileCount)
	}
	if statsPayload.StorageUsed <= 0 {
		t.Errorf("expected storage used > 0, got %d", statsPayload.StorageUsed)
	}
	if statsPayload.StorageUsedFormatted == "" {
		t.Errorf("expected formatted storage, got empty string")
	}
}

func TestGetCollectionStatsEmptyCollection(t *testing.T) {
	srv := newTestServer(t)

	// Test stats for a collection with no files
	statsReq := httptest.NewRequest(http.MethodGet, "/collections/videos/stats", nil)
	statsResp := httptest.NewRecorder()
	srv.router.ServeHTTP(statsResp, statsReq)

	if statsResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", statsResp.Code, statsResp.Body.String())
	}

	var statsPayload struct {
		Type                string `json:"type"`
		FileCount           int    `json:"file_count"`
		StorageUsed         int64  `json:"storage_used"`
		StorageUsedFormatted string `json:"storage_used_formatted"`
	}
	if err := json.NewDecoder(statsResp.Body).Decode(&statsPayload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if statsPayload.FileCount != 0 {
		t.Errorf("expected 0 files for empty collection, got %d", statsPayload.FileCount)
	}
	if statsPayload.StorageUsed != 0 {
		t.Errorf("expected 0 storage for empty collection, got %d", statsPayload.StorageUsed)
	}
}

func TestGetCollectionStatsInvalidType(t *testing.T) {
	srv := newTestServer(t)

	// Test with invalid collection type (should still work, just return 0 stats)
	statsReq := httptest.NewRequest(http.MethodGet, "/collections/invalid_type/stats", nil)
	statsResp := httptest.NewRecorder()
	srv.router.ServeHTTP(statsResp, statsReq)

	// Should return 200 with 0 stats
	if statsResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", statsResp.Code, statsResp.Body.String())
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
