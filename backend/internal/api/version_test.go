package api

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/config"
	"log/slog"
)

func setupTestServer(t *testing.T) (*Server, string) {
	tmpDir := t.TempDir()
	cfg := config.Config{
		DataDir:       tmpDir,
		MaxUploadBytes: 10 * 1024 * 1024, // 10MB
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	server, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	return server, tmpDir
}

func createMultipartForm(t *testing.T, filename string, content []byte, comment string) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("failed to write file content: %v", err)
	}

	// Add comment if provided
	if comment != "" {
		if err := writer.WriteField("comment", comment); err != nil {
			t.Fatalf("failed to write comment field: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	return body, writer.FormDataContentType()
}

func TestCreateVersion(t *testing.T) {
	server, _ := setupTestServer(t)

	// First, upload an initial file
	initialContent := []byte("initial file content")
	body, contentType := createMultipartForm(t, "test.txt", initialContent, "Initial version")

	req := httptest.NewRequest("POST", "/ingest/media", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var ingestResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &ingestResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	stored := ingestResp["stored"].([]any)
	if len(stored) == 0 {
		t.Fatal("no files stored")
	}

	fileData := stored[0].(map[string]any)
	fileID := fileData["hash"].(string)

	// Now create a new version
	newContent := []byte("updated file content")
	body2, contentType2 := createMultipartForm(t, "test.txt", newContent, "Updated version")

	req2 := httptest.NewRequest("POST", "/files/"+fileID+"/versions", body2)
	req2.Header.Set("Content-Type", contentType2)
	w2 := httptest.NewRecorder()
	server.Router().ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w2.Code, w2.Body.String())
	}

	var versionResp map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &versionResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if versionResp["file_id"].(string) != fileID {
		t.Errorf("expected file_id %s, got %s", fileID, versionResp["file_id"])
	}

	version := versionResp["version"].(map[string]any)
	if version["version"].(float64) != 2 {
		t.Errorf("expected version 2, got %v", version["version"])
	}
	if version["comment"].(string) != "Updated version" {
		t.Errorf("expected comment 'Updated version', got '%s'", version["comment"])
	}
}

func TestListVersions(t *testing.T) {
	server, _ := setupTestServer(t)

	// Upload initial file
	initialContent := []byte("version 1")
	body, contentType := createMultipartForm(t, "test.txt", initialContent, "Initial")

	req := httptest.NewRequest("POST", "/ingest/media", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	var ingestResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &ingestResp)
	stored := ingestResp["stored"].([]any)
	fileID := stored[0].(map[string]any)["hash"].(string)

	// Create version 2
	content2 := []byte("version 2")
	body2, contentType2 := createMultipartForm(t, "test.txt", content2, "Update 1")
	req2 := httptest.NewRequest("POST", "/files/"+fileID+"/versions", body2)
	req2.Header.Set("Content-Type", contentType2)
	w2 := httptest.NewRecorder()
	server.Router().ServeHTTP(w2, req2)

	// Create version 3
	content3 := []byte("version 3")
	body3, contentType3 := createMultipartForm(t, "test.txt", content3, "Update 2")
	req3 := httptest.NewRequest("POST", "/files/"+fileID+"/versions", body3)
	req3.Header.Set("Content-Type", contentType3)
	w3 := httptest.NewRecorder()
	server.Router().ServeHTTP(w3, req3)

	// List versions
	req4 := httptest.NewRequest("GET", "/files/"+fileID+"/versions", nil)
	w4 := httptest.NewRecorder()
	server.Router().ServeHTTP(w4, req4)

	if w4.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w4.Code, w4.Body.String())
	}

	var listResp map[string]any
	if err := json.Unmarshal(w4.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	versions := listResp["versions"].([]any)
	if len(versions) != 3 {
		t.Errorf("expected 3 versions, got %d", len(versions))
	}

	// Versions should be sorted descending (newest first)
	version1 := versions[0].(map[string]any)
	if version1["version"].(float64) != 3 {
		t.Errorf("expected first version to be 3, got %v", version1["version"])
	}
	if !version1["is_current"].(bool) {
		t.Error("version 3 should be current")
	}
}

func TestGetVersion(t *testing.T) {
	server, _ := setupTestServer(t)

	// Upload initial file
	initialContent := []byte("version 1")
	body, contentType := createMultipartForm(t, "test.txt", initialContent, "Initial")

	req := httptest.NewRequest("POST", "/ingest/media", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	var ingestResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &ingestResp)
	stored := ingestResp["stored"].([]any)
	fileID := stored[0].(map[string]any)["hash"].(string)

	// Create version 2
	content2 := []byte("version 2")
	body2, contentType2 := createMultipartForm(t, "test.txt", content2, "Update")
	req2 := httptest.NewRequest("POST", "/files/"+fileID+"/versions", body2)
	req2.Header.Set("Content-Type", contentType2)
	w2 := httptest.NewRecorder()
	server.Router().ServeHTTP(w2, req2)

	// Get version 1 metadata
	req3 := httptest.NewRequest("GET", "/files/"+fileID+"/versions/1", nil)
	w3 := httptest.NewRecorder()
	server.Router().ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w3.Code, w3.Body.String())
	}

	var versionResp map[string]any
	if err := json.Unmarshal(w3.Body.Bytes(), &versionResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	version := versionResp["version"].(map[string]any)
	if version["version"].(float64) != 1 {
		t.Errorf("expected version 1, got %v", version["version"])
	}
	if version["comment"].(string) != "Initial" {
		t.Errorf("expected comment 'Initial', got '%s'", version["comment"])
	}

	// Get version 2 as download
	req4 := httptest.NewRequest("GET", "/files/"+fileID+"/versions/2?download=true", nil)
	w4 := httptest.NewRecorder()
	server.Router().ServeHTTP(w4, req4)

	if w4.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w4.Code, w4.Body.String())
	}

	downloadedContent, _ := io.ReadAll(w4.Body)
	if string(downloadedContent) != "version 2" {
		t.Errorf("expected content 'version 2', got '%s'", string(downloadedContent))
	}
}

func TestRevertVersion(t *testing.T) {
	server, _ := setupTestServer(t)

	// Upload initial file
	initialContent := []byte("version 1")
	body, contentType := createMultipartForm(t, "test.txt", initialContent, "Initial")

	req := httptest.NewRequest("POST", "/ingest/media", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	var ingestResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &ingestResp)
	stored := ingestResp["stored"].([]any)
	fileID := stored[0].(map[string]any)["hash"].(string)

	// Create version 2
	content2 := []byte("version 2")
	body2, contentType2 := createMultipartForm(t, "test.txt", content2, "Update")
	req2 := httptest.NewRequest("POST", "/files/"+fileID+"/versions", body2)
	req2.Header.Set("Content-Type", contentType2)
	w2 := httptest.NewRecorder()
	server.Router().ServeHTTP(w2, req2)

	// Revert to version 1
	revertReq := map[string]any{
		"version": 1,
		"comment": "Reverting due to error",
	}
	revertBody, _ := json.Marshal(revertReq)

	req3 := httptest.NewRequest("POST", "/files/"+fileID+"/revert", bytes.NewReader(revertBody))
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	server.Router().ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w3.Code, w3.Body.String())
	}

	var revertResp map[string]any
	if err := json.Unmarshal(w3.Body.Bytes(), &revertResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	version := revertResp["version"].(map[string]any)
	if version["version"].(float64) != 1 {
		t.Errorf("expected version 1, got %v", version["version"])
	}
	if !version["is_current"].(bool) {
		t.Error("reverted version should be current")
	}
}

func TestVersionDiff(t *testing.T) {
	server, _ := setupTestServer(t)

	// Upload initial file
	initialContent := []byte("small")
	body, contentType := createMultipartForm(t, "test.txt", initialContent, "Initial")

	req := httptest.NewRequest("POST", "/ingest/media", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	var ingestResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &ingestResp)
	stored := ingestResp["stored"].([]any)
	fileID := stored[0].(map[string]any)["hash"].(string)

	// Create version 2 with different content
	content2 := []byte("much larger content")
	body2, contentType2 := createMultipartForm(t, "test.txt", content2, "Updated")
	req2 := httptest.NewRequest("POST", "/files/"+fileID+"/versions", body2)
	req2.Header.Set("Content-Type", contentType2)
	w2 := httptest.NewRecorder()
	server.Router().ServeHTTP(w2, req2)

	// Get diff
	req3 := httptest.NewRequest("GET", "/files/"+fileID+"/versions/diff?from=1&to=2", nil)
	w3 := httptest.NewRecorder()
	server.Router().ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w3.Code, w3.Body.String())
	}

	var diffResp map[string]any
	if err := json.Unmarshal(w3.Body.Bytes(), &diffResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if diffResp["from_version"].(float64) != 1 {
		t.Errorf("expected from_version 1, got %v", diffResp["from_version"])
	}
	if diffResp["to_version"].(float64) != 2 {
		t.Errorf("expected to_version 2, got %v", diffResp["to_version"])
	}

	changes := diffResp["changes"].(map[string]any)
	if changes["size"] == nil {
		t.Error("expected size change in diff")
	}
	if changes["comment"] == nil {
		t.Error("expected comment change in diff")
	}
}

func TestVersionErrors(t *testing.T) {
	server, _ := setupTestServer(t)

	// Test listing versions for non-existent file
	req := httptest.NewRequest("GET", "/files/non-existent/versions", nil)
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	// Test getting non-existent version
	req2 := httptest.NewRequest("GET", "/files/non-existent/versions/1", nil)
	w2 := httptest.NewRecorder()
	server.Router().ServeHTTP(w2, req2)

	if w2.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w2.Code)
	}

	// Test invalid version number
	req3 := httptest.NewRequest("GET", "/files/test/versions/invalid", nil)
	w3 := httptest.NewRecorder()
	server.Router().ServeHTTP(w3, req3)

	if w3.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w3.Code)
	}
}

