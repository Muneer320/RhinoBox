package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
	"log/slog"
)

// TestVersioningEndToEnd tests the complete versioning workflow
func TestVersioningEndToEnd(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Config{
		DataDir:        tmpDir,
		MaxUploadBytes: 10 * 1024 * 1024,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	server, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Step 1: Upload initial file
	t.Log("Step 1: Uploading initial file")
	initialContent := []byte("This is the initial version of the document.")
	body, contentType := createMultipartForm(t, "document.txt", initialContent, "Initial upload")

	req := httptest.NewRequest("POST", "/ingest/media", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("upload failed: status %d, body: %s", w.Code, w.Body.String())
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
	t.Logf("File uploaded with ID: %s", fileID)

	// Step 2: Create version 2
	t.Log("Step 2: Creating version 2")
	version2Content := []byte("This is version 2 with updated content.")
	body2, contentType2 := createMultipartForm(t, "document.txt", version2Content, "Updated financial figures")

	req2 := httptest.NewRequest("POST", fmt.Sprintf("/files/%s/versions", fileID), body2)
	req2.Header.Set("Content-Type", contentType2)
	// Add uploaded_by as form field in multipart form
	body2WithUser, contentType2WithUser := createMultipartFormWithUser(t, "document.txt", version2Content, "Updated financial figures", "user123")
	req2 = httptest.NewRequest("POST", fmt.Sprintf("/files/%s/versions", fileID), body2WithUser)
	req2.Header.Set("Content-Type", contentType2WithUser)
	w2 := httptest.NewRecorder()
	server.Router().ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("create version failed: status %d, body: %s", w2.Code, w2.Body.String())
	}

	var versionResp map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &versionResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	version := versionResp["version"].(map[string]any)
	if version["version"].(float64) != 2 {
		t.Errorf("expected version 2, got %v", version["version"])
	}
	t.Logf("Version 2 created: hash=%s", version["hash"])

	// Step 3: Create version 3
	t.Log("Step 3: Creating version 3")
	version3Content := []byte("This is version 3 with even more changes.")
	body3WithUser, contentType3WithUser := createMultipartFormWithUser(t, "document.txt", version3Content, "Final revision", "user456")
	req3 := httptest.NewRequest("POST", fmt.Sprintf("/files/%s/versions", fileID), body3WithUser)
	req3.Header.Set("Content-Type", contentType3WithUser)
	w3 := httptest.NewRecorder()
	server.Router().ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Fatalf("create version failed: status %d, body: %s", w3.Code, w3.Body.String())
	}

	// Step 4: List all versions
	t.Log("Step 4: Listing all versions")
	req4 := httptest.NewRequest("GET", fmt.Sprintf("/files/%s/versions", fileID), nil)
	w4 := httptest.NewRecorder()
	server.Router().ServeHTTP(w4, req4)

	if w4.Code != http.StatusOK {
		t.Fatalf("list versions failed: status %d", w4.Code)
	}

	var listResp map[string]any
	if err := json.Unmarshal(w4.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	versions := listResp["versions"].([]any)
	if len(versions) != 3 {
		t.Errorf("expected 3 versions, got %d", len(versions))
	}

	// Verify versions are sorted correctly (newest first)
	v1 := versions[0].(map[string]any)
	v2 := versions[1].(map[string]any)
	v3 := versions[2].(map[string]any)

	if v1["version"].(float64) != 3 {
		t.Errorf("expected first version to be 3, got %v", v1["version"])
	}
	if v2["version"].(float64) != 2 {
		t.Errorf("expected second version to be 2, got %v", v2["version"])
	}
	if v3["version"].(float64) != 1 {
		t.Errorf("expected third version to be 1, got %v", v3["version"])
	}

	// Verify current version
	if !v1["is_current"].(bool) {
		t.Error("version 3 should be current")
	}

	// Step 5: Get specific version metadata
	t.Log("Step 5: Getting version 2 metadata")
	req5 := httptest.NewRequest("GET", fmt.Sprintf("/files/%s/versions/2", fileID), nil)
	w5 := httptest.NewRecorder()
	server.Router().ServeHTTP(w5, req5)

	if w5.Code != http.StatusOK {
		t.Fatalf("get version failed: status %d", w5.Code)
	}

	var version2Resp map[string]any
	if err := json.Unmarshal(w5.Body.Bytes(), &version2Resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	version2Meta := version2Resp["version"].(map[string]any)
	if version2Meta["version"].(float64) != 2 {
		t.Errorf("expected version 2, got %v", version2Meta["version"])
	}
	if version2Meta["comment"].(string) != "Updated financial figures" {
		t.Errorf("expected comment 'Updated financial figures', got '%s'", version2Meta["comment"])
	}

	// Step 6: Download version 1
	t.Log("Step 6: Downloading version 1")
	req6 := httptest.NewRequest("GET", fmt.Sprintf("/files/%s/versions/1?download=true", fileID), nil)
	w6 := httptest.NewRecorder()
	server.Router().ServeHTTP(w6, req6)

	if w6.Code != http.StatusOK {
		t.Fatalf("download version failed: status %d", w6.Code)
	}

	downloadedContent, _ := io.ReadAll(w6.Body)
	if string(downloadedContent) != string(initialContent) {
		t.Errorf("expected content '%s', got '%s'", string(initialContent), string(downloadedContent))
	}

	// Step 7: Compare versions
	t.Log("Step 7: Comparing versions 1 and 3")
	req7 := httptest.NewRequest("GET", fmt.Sprintf("/files/%s/versions/diff?from=1&to=3", fileID), nil)
	w7 := httptest.NewRecorder()
	server.Router().ServeHTTP(w7, req7)

	if w7.Code != http.StatusOK {
		t.Fatalf("get diff failed: status %d", w7.Code)
	}

	var diffResp map[string]any
	if err := json.Unmarshal(w7.Body.Bytes(), &diffResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if diffResp["from_version"].(float64) != 1 {
		t.Errorf("expected from_version 1, got %v", diffResp["from_version"])
	}
	if diffResp["to_version"].(float64) != 3 {
		t.Errorf("expected to_version 3, got %v", diffResp["to_version"])
	}

	changes := diffResp["changes"].(map[string]any)
	if changes["size"] == nil {
		t.Error("expected size change in diff")
	}

	// Step 8: Revert to version 1
	t.Log("Step 8: Reverting to version 1")
	revertReq := map[string]any{
		"version": 1,
		"comment": "Reverting due to error in v3",
	}
	revertBody, _ := json.Marshal(revertReq)

	req8 := httptest.NewRequest("POST", fmt.Sprintf("/files/%s/revert", fileID), bytes.NewReader(revertBody))
	req8.Header.Set("Content-Type", "application/json")
	w8 := httptest.NewRecorder()
	server.Router().ServeHTTP(w8, req8)

	if w8.Code != http.StatusOK {
		t.Fatalf("revert failed: status %d, body: %s", w8.Code, w8.Body.String())
	}

	var revertResp map[string]any
	if err := json.Unmarshal(w8.Body.Bytes(), &revertResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	revertedVersion := revertResp["version"].(map[string]any)
	if revertedVersion["version"].(float64) != 1 {
		t.Errorf("expected version 1, got %v", revertedVersion["version"])
	}
	if !revertedVersion["is_current"].(bool) {
		t.Error("reverted version should be current")
	}

	// Step 9: Verify version 1 is now current
	t.Log("Step 9: Verifying version 1 is current")
	req9 := httptest.NewRequest("GET", fmt.Sprintf("/files/%s/versions", fileID), nil)
	w9 := httptest.NewRecorder()
	server.Router().ServeHTTP(w9, req9)

	var listResp2 map[string]any
	json.Unmarshal(w9.Body.Bytes(), &listResp2)
	versions2 := listResp2["versions"].([]any)

	// Find version 1
	for _, v := range versions2 {
		versionMap := v.(map[string]any)
		if versionMap["version"].(float64) == 1 {
			if !versionMap["is_current"].(bool) {
				t.Error("version 1 should be current after revert")
			}
			break
		}
	}

	t.Log("End-to-end versioning test completed successfully")
}

func createMultipartForm(t *testing.T, filename string, content []byte, comment string) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("failed to write file content: %v", err)
	}

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

func createMultipartFormWithUser(t *testing.T, filename string, content []byte, comment string, uploadedBy string) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("failed to write file content: %v", err)
	}

	if comment != "" {
		if err := writer.WriteField("comment", comment); err != nil {
			t.Fatalf("failed to write comment field: %v", err)
		}
	}

	if uploadedBy != "" {
		if err := writer.WriteField("uploaded_by", uploadedBy); err != nil {
			t.Fatalf("failed to write uploaded_by field: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	return body, writer.FormDataContentType()
}

// BenchmarkVersionOperations benchmarks version operations
func BenchmarkVersionOperations(b *testing.B) {
	tmpDir := b.TempDir()
	cfg := config.Config{
		DataDir:        tmpDir,
		MaxUploadBytes: 10 * 1024 * 1024,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))

	server, err := api.NewServer(cfg, logger)
	if err != nil {
		b.Fatalf("failed to create server: %v", err)
	}

	// Setup: Upload initial file
	initialContent := []byte("initial content")
	body, contentType := createMultipartFormBench(b, "test.txt", initialContent, "Initial")
	req := httptest.NewRequest("POST", "/ingest/media", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	var ingestResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &ingestResp)
	stored := ingestResp["stored"].([]any)
	fileID := stored[0].(map[string]any)["hash"].(string)

	b.ResetTimer()

	// Benchmark creating versions
	for i := 0; i < b.N; i++ {
		content := []byte(fmt.Sprintf("version %d content", i))
		body, contentType := createMultipartFormBench(b, "test.txt", content, fmt.Sprintf("Version %d", i))
		req := httptest.NewRequest("POST", fmt.Sprintf("/files/%s/versions", fileID), body)
		req.Header.Set("Content-Type", contentType)
		w := httptest.NewRecorder()
		server.Router().ServeHTTP(w, req)
	}
}

// BenchmarkListVersions benchmarks listing versions
func BenchmarkListVersions(b *testing.B) {
	tmpDir := b.TempDir()
	cfg := config.Config{
		DataDir:        tmpDir,
		MaxUploadBytes: 10 * 1024 * 1024,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))

	server, err := api.NewServer(cfg, logger)
	if err != nil {
		b.Fatalf("failed to create server: %v", err)
	}

	// Setup: Create file with 10 versions
	initialContent := []byte("initial")
	body, contentType := createMultipartFormBench(b, "test.txt", initialContent, "Initial")
	req := httptest.NewRequest("POST", "/ingest/media", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	var ingestResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &ingestResp)
	stored := ingestResp["stored"].([]any)
	fileID := stored[0].(map[string]any)["hash"].(string)

	// Create 10 versions
	for i := 0; i < 10; i++ {
		content := []byte(fmt.Sprintf("version %d", i))
		body, contentType := createMultipartFormBench(b, "test.txt", content, fmt.Sprintf("Version %d", i))
		req := httptest.NewRequest("POST", fmt.Sprintf("/files/%s/versions", fileID), body)
		req.Header.Set("Content-Type", contentType)
		w := httptest.NewRecorder()
		server.Router().ServeHTTP(w, req)
	}

	b.ResetTimer()

	// Benchmark listing versions
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", fmt.Sprintf("/files/%s/versions", fileID), nil)
		w := httptest.NewRecorder()
		server.Router().ServeHTTP(w, req)
	}
}

func createMultipartFormBench(b *testing.B, filename string, content []byte, comment string) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		b.Fatalf("failed to create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		b.Fatalf("failed to write file content: %v", err)
	}

	if comment != "" {
		if err := writer.WriteField("comment", comment); err != nil {
			b.Fatalf("failed to write comment field: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		b.Fatalf("failed to close writer: %v", err)
	}

	return body, writer.FormDataContentType()
}

