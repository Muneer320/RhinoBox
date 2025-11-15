package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/config"
	"github.com/Muneer320/RhinoBox/internal/storage"
)

func TestVersionAPI_UploadVersion(t *testing.T) {
	srv := newTestServer(t)

	// First, create a versioned file
	fileID := createVersionedFile(t, srv, "initial_content_v1", "report.pdf")

	// Upload a new version
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, _ := writer.CreateFormFile("file", "report.pdf")
	fileWriter.Write([]byte("updated_content_v2"))
	writer.WriteField("comment", "Updated financial figures")
	writer.WriteField("uploaded_by", "user456")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/files/%s/versions/", fileID), body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()

	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result["version"].(float64) != 2 {
		t.Fatalf("expected version 2, got %v", result["version"])
	}
	if result["uploaded_by"].(string) != "user456" {
		t.Fatalf("expected uploaded_by user456, got %v", result["uploaded_by"])
	}
	if result["comment"].(string) != "Updated financial figures" {
		t.Fatalf("expected comment, got %v", result["comment"])
	}
	if result["is_current"].(bool) != true {
		t.Fatal("expected is_current to be true")
	}
}

func TestVersionAPI_ListVersions(t *testing.T) {
	srv := newTestServer(t)

	// Create a file with 3 versions
	fileID := createVersionedFile(t, srv, "content_v1", "document.txt")
	uploadVersion(t, srv, fileID, "content_v2", "user2", "Version 2")
	uploadVersion(t, srv, fileID, "content_v3", "user3", "Version 3")

	// List all versions
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/files/%s/versions/", fileID), nil)
	resp := httptest.NewRecorder()

	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result["current_version"].(float64) != 3 {
		t.Fatalf("expected current_version 3, got %v", result["current_version"])
	}
	if result["total_versions"].(float64) != 3 {
		t.Fatalf("expected total_versions 3, got %v", result["total_versions"])
	}

	versions := result["versions"].([]any)
	if len(versions) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(versions))
	}

	// Verify only last version is current
	for i, v := range versions {
		ver := v.(map[string]any)
		isCurrent := ver["is_current"].(bool)
		if i == len(versions)-1 {
			if !isCurrent {
				t.Fatal("last version should be current")
			}
		} else {
			if isCurrent {
				t.Fatalf("version %d should not be current", i+1)
			}
		}
	}
}

func TestVersionAPI_GetVersion(t *testing.T) {
	srv := newTestServer(t)

	// Create a file with 2 versions
	fileID := createVersionedFile(t, srv, "content_v1", "file.txt")
	uploadVersion(t, srv, fileID, "content_v2", "user2", "Updated content")

	// Get version 1
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/files/%s/versions/1", fileID), nil)
	resp := httptest.NewRecorder()

	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result["version"].(float64) != 1 {
		t.Fatalf("expected version 1, got %v", result["version"])
	}
	if result["is_current"].(bool) != false {
		t.Fatal("version 1 should not be current")
	}

	// Get version 2
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/files/%s/versions/2", fileID), nil)
	resp = httptest.NewRecorder()

	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result["version"].(float64) != 2 {
		t.Fatalf("expected version 2, got %v", result["version"])
	}
	if result["is_current"].(bool) != true {
		t.Fatal("version 2 should be current")
	}
}

func TestVersionAPI_GetVersionDownload(t *testing.T) {
	srv := newTestServer(t)

	content := []byte("test content for download")
	fileID := createVersionedFile(t, srv, string(content), "test.txt")

	// Download version 1
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/files/%s/versions/1?download=true", fileID), nil)
	resp := httptest.NewRecorder()

	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	downloadedContent := resp.Body.Bytes()
	if !bytes.Equal(downloadedContent, content) {
		t.Fatalf("downloaded content mismatch: expected %s, got %s", content, downloadedContent)
	}

	contentDisposition := resp.Header().Get("Content-Disposition")
	if contentDisposition == "" {
		t.Fatal("expected Content-Disposition header")
	}
}

func TestVersionAPI_RevertVersion(t *testing.T) {
	srv := newTestServer(t)

	// Create a file with 3 versions
	fileID := createVersionedFile(t, srv, "content_v1", "doc.txt")
	uploadVersion(t, srv, fileID, "content_v2", "user2", "Version 2")
	uploadVersion(t, srv, fileID, "content_v3_buggy", "user3", "Version 3 with bugs")

	// Revert to version 2
	reqBody := map[string]any{
		"version":     2,
		"comment":     "Reverting due to bugs in v3",
		"uploaded_by": "admin",
	}
	reqJSON, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/files/%s/revert", fileID), bytes.NewReader(reqJSON))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result["new_version"].(float64) != 4 {
		t.Fatalf("expected new_version 4, got %v", result["new_version"])
	}
	if result["reverted_to"].(float64) != 2 {
		t.Fatalf("expected reverted_to 2, got %v", result["reverted_to"])
	}
	if result["uploaded_by"].(string) != "admin" {
		t.Fatalf("expected uploaded_by admin, got %v", result["uploaded_by"])
	}

	// Verify the file now has 4 versions
	listReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/files/%s/versions/", fileID), nil)
	listResp := httptest.NewRecorder()
	srv.router.ServeHTTP(listResp, listReq)

	var listResult map[string]any
	json.NewDecoder(listResp.Body).Decode(&listResult)

	if listResult["total_versions"].(float64) != 4 {
		t.Fatalf("expected 4 versions after revert, got %v", listResult["total_versions"])
	}
	if listResult["current_version"].(float64) != 4 {
		t.Fatalf("expected current_version 4, got %v", listResult["current_version"])
	}
}

func TestVersionAPI_CompareVersions(t *testing.T) {
	srv := newTestServer(t)

	// Create a file with different versions
	fileID := createVersionedFile(t, srv, "small content", "file.txt")
	uploadVersion(t, srv, fileID, "much larger content here", "user2", "Expanded content")

	// Compare versions 1 and 2
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/files/%s/versions/diff?from=1&to=2", fileID), nil)
	resp := httptest.NewRecorder()

	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	differences := result["differences"].(map[string]any)
	
	// Content should have changed (different hashes)
	if differences["content_changed"].(bool) != true {
		t.Fatal("expected content_changed to be true")
	}

	// Size should have changed
	if differences["size_changed"].(bool) != true {
		t.Fatal("expected size_changed to be true")
	}

	sizeDelta := differences["size_delta"].(float64)
	if sizeDelta <= 0 {
		t.Fatalf("expected positive size_delta, got %v", sizeDelta)
	}
}

func TestVersionAPI_ErrorCases(t *testing.T) {
	srv := newTestServer(t)

	// Test 1: Upload version to non-existent file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, _ := writer.CreateFormFile("file", "test.txt")
	fileWriter.Write([]byte("content"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/files/non-existent-id/versions/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()

	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Code)
	}

	// Test 2: Get non-existent version
	fileID := createVersionedFile(t, srv, "content", "file.txt")
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/files/%s/versions/99", fileID), nil)
	resp = httptest.NewRecorder()

	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Code)
	}

	// Test 3: Revert to non-existent version
	reqBody := map[string]any{"version": 99, "uploaded_by": "user"}
	reqJSON, _ := json.Marshal(reqBody)
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/files/%s/revert", fileID), bytes.NewReader(reqJSON))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()

	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.Code)
	}

	// Test 4: Compare with invalid version numbers
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/files/%s/versions/diff?from=invalid&to=2", fileID), nil)
	resp = httptest.NewRecorder()

	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

// Helper functions

func createVersionedFile(t *testing.T, srv *Server, content, filename string) string {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, _ := writer.CreateFormFile("file", filename)
	fileWriter.Write([]byte(content))
	writer.WriteField("comment", "Initial version")
	writer.WriteField("uploaded_by", "user123")
	writer.WriteField("versioned", "true")
	writer.Close()

	// Use the ingest endpoint to create a versioned file
	// Since we don't have that endpoint, we'll use storage directly
	result, err := srv.storage.StoreFile(storage.StoreRequest{
		Reader:     bytes.NewReader([]byte(content)),
		Filename:   filename,
		MimeType:   "text/plain",
		Size:       int64(len(content)),
		Versioned:  true,
		UploadedBy: "user123",
		Metadata:   map[string]string{"comment": "Initial version"},
	})
	if err != nil {
		t.Fatalf("create versioned file: %v", err)
	}

	return result.FileID
}

func uploadVersion(t *testing.T, srv *Server, fileID, content, uploadedBy, comment string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, _ := writer.CreateFormFile("file", "file.txt")
	fileWriter.Write([]byte(content))
	writer.WriteField("comment", comment)
	writer.WriteField("uploaded_by", uploadedBy)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/files/%s/versions/", fileID), body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()

	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("upload version failed: %d: %s", resp.Code, resp.Body.String())
	}
}

func newTestServerForVersions(t *testing.T) *Server {
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
