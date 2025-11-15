package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/storage"
)

// TestFileRenameMetadataOnly tests renaming only the metadata without touching the stored file
func TestFileRenameMetadataOnly(t *testing.T) {
	srv := newTestServer(t)

	// First, upload a file
	hash := uploadTestFile(t, srv, "original_document.pdf", "application/pdf", []byte("PDF content here"))

	// Rename metadata only
	req := fileRenameRequest{
		Hash:             hash,
		NewName:          "renamed_document.pdf",
		UpdateStoredFile: false,
	}
	
	resp := makeRenameRequest(t, srv, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !result["success"].(bool) {
		t.Error("expected success to be true")
	}

	metadata := result["metadata"].(map[string]any)
	if metadata["original_name"].(string) != "renamed_document.pdf" {
		t.Errorf("expected original_name to be renamed_document.pdf, got %s", metadata["original_name"])
	}

	if result["disk_renamed"].(bool) {
		t.Error("expected disk_renamed to be false")
	}

	// Verify the stored file path hasn't changed
	storedPath := metadata["stored_path"].(string)
	if !strings.Contains(storedPath, "original") {
		t.Errorf("expected stored path to still contain original filename sanitized, got %s", storedPath)
	}
}

// TestFileRenameFullRename tests renaming both metadata and the stored file on disk
func TestFileRenameFullRename(t *testing.T) {
	srv := newTestServer(t)

	// Upload a file
	hash := uploadTestFile(t, srv, "old_report.pdf", "application/pdf", []byte("Report data"))
	
	// Get the original stored path
	oldMetadata := srv.storage.FindByHash(hash)
	if oldMetadata == nil {
		t.Fatal("failed to find uploaded file")
	}
	oldAbsPath := filepath.Join(srv.cfg.DataDir, filepath.FromSlash(oldMetadata.StoredPath))
	
	// Verify old file exists
	if _, err := os.Stat(oldAbsPath); err != nil {
		t.Fatalf("old file should exist: %v", err)
	}

	// Full rename (metadata + disk)
	req := fileRenameRequest{
		Hash:             hash,
		NewName:          "new_report.pdf",
		UpdateStoredFile: true,
	}
	
	resp := makeRenameRequest(t, srv, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !result["disk_renamed"].(bool) {
		t.Error("expected disk_renamed to be true")
	}

	metadata := result["metadata"].(map[string]any)
	if metadata["original_name"].(string) != "new_report.pdf" {
		t.Errorf("expected original_name to be new_report.pdf, got %s", metadata["original_name"])
	}

	// Verify the new file exists and old file is gone
	newStoredPath := metadata["stored_path"].(string)
	newAbsPath := filepath.Join(srv.cfg.DataDir, filepath.FromSlash(newStoredPath))
	
	if _, err := os.Stat(newAbsPath); err != nil {
		t.Fatalf("new file should exist: %v", err)
	}
	
	if _, err := os.Stat(oldAbsPath); err == nil {
		t.Error("old file should be removed")
	}

	// Verify the new path contains the new filename
	if !strings.Contains(newStoredPath, "new") && !strings.Contains(newStoredPath, "report") {
		t.Errorf("expected stored path to contain new filename components, got %s", newStoredPath)
	}
}

// TestFileRenameByPath tests renaming a file by its stored path instead of hash
func TestFileRenameByPath(t *testing.T) {
	srv := newTestServer(t)

	// Upload a file
	hash := uploadTestFile(t, srv, "photo.jpg", "image/jpeg", []byte("JPEG data"))
	
	// Get stored path
	metadata := srv.storage.FindByHash(hash)
	if metadata == nil {
		t.Fatal("failed to find uploaded file")
	}

	// Rename by path
	req := fileRenameRequest{
		StoredPath:       metadata.StoredPath,
		NewName:          "vacation_photo.jpg",
		UpdateStoredFile: false,
	}
	
	resp := makeRenameRequest(t, srv, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	resultMetadata := result["metadata"].(map[string]any)
	if resultMetadata["original_name"].(string) != "vacation_photo.jpg" {
		t.Errorf("expected original_name to be vacation_photo.jpg, got %s", resultMetadata["original_name"])
	}
}

// TestFileRenameValidation tests various validation scenarios
func TestFileRenameValidation(t *testing.T) {
	srv := newTestServer(t)

	tests := []struct {
		name           string
		request        fileRenameRequest
		expectedStatus int
		errorContains  string
	}{
		{
			name: "missing identifier",
			request: fileRenameRequest{
				NewName: "file.txt",
			},
			expectedStatus: http.StatusBadRequest,
			errorContains:  "hash or stored_path must be provided",
		},
		{
			name: "missing new name",
			request: fileRenameRequest{
				Hash: "abc123",
			},
			expectedStatus: http.StatusBadRequest,
			errorContains:  "new_name is required",
		},
		{
			name: "path traversal attempt with ..",
			request: fileRenameRequest{
				Hash:    "abc123",
				NewName: "../../../etc/passwd",
			},
			expectedStatus: http.StatusBadRequest,
			errorContains:  "path traversal not allowed",
		},
		{
			name: "path with forward slash",
			request: fileRenameRequest{
				Hash:    "abc123",
				NewName: "dir/file.txt",
			},
			expectedStatus: http.StatusBadRequest,
			errorContains:  "path traversal not allowed",
		},
		{
			name: "path with backslash",
			request: fileRenameRequest{
				Hash:    "abc123",
				NewName: "dir\\file.txt",
			},
			expectedStatus: http.StatusBadRequest,
			errorContains:  "path traversal not allowed",
		},
		{
			name: "non-existent file",
			request: fileRenameRequest{
				Hash:    "nonexistent123456",
				NewName: "valid.txt",
			},
			expectedStatus: http.StatusNotFound,
			errorContains:  "file not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := makeRenameRequest(t, srv, tt.request)
			if resp.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d: %s", tt.expectedStatus, resp.Code, resp.Body.String())
			}

			var errResp map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
				t.Fatalf("decode error response: %v", err)
			}

			if errMsg, ok := errResp["error"].(string); ok {
				if !strings.Contains(errMsg, tt.errorContains) {
					t.Errorf("expected error to contain '%s', got '%s'", tt.errorContains, errMsg)
				}
			}
		})
	}
}

// TestFileRenameConflict tests handling of duplicate filename conflicts
func TestFileRenameConflict(t *testing.T) {
	srv := newTestServer(t)

	// Upload first file
	hash := uploadTestFile(t, srv, "file1.pdf", "application/pdf", []byte("content 1"))
	
	// Get directory to create conflicting file
	metadata := srv.storage.FindByHash(hash)
	if metadata == nil {
		t.Fatal("failed to find uploaded file")
	}
	
	absPath := filepath.Join(srv.cfg.DataDir, filepath.FromSlash(metadata.StoredPath))
	dir := filepath.Dir(absPath)
	
	// Create a file that will conflict with the rename
	conflictName := fmt.Sprintf("%s_%s.pdf", hash[:12], "conflict")
	conflictPath := filepath.Join(dir, conflictName)
	if err := os.WriteFile(conflictPath, []byte("conflict"), 0o644); err != nil {
		t.Fatalf("create conflict file: %v", err)
	}

	// Try to rename to the conflicting name
	req := fileRenameRequest{
		Hash:             hash,
		NewName:          "conflict.pdf",
		UpdateStoredFile: true,
	}
	
	resp := makeRenameRequest(t, srv, req)
	if resp.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", resp.Code, resp.Body.String())
	}
}

// TestFileRenameAuditLog tests that rename operations are logged
func TestFileRenameAuditLog(t *testing.T) {
	srv := newTestServer(t)

	// Upload and rename a file
	hash := uploadTestFile(t, srv, "audit_test.txt", "text/plain", []byte("test content"))
	
	req := fileRenameRequest{
		Hash:             hash,
		NewName:          "audited_file.txt",
		UpdateStoredFile: false,
	}
	
	resp := makeRenameRequest(t, srv, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	// Check that audit log was created
	logPath := filepath.Join(srv.cfg.DataDir, "metadata", "rename_log.ndjson")
	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("audit log should exist: %v", err)
	}

	// Read and verify log content
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}

	if len(content) == 0 {
		t.Error("audit log should not be empty")
	}

	// Parse the log entry
	var logEntry map[string]any
	if err := json.Unmarshal(content, &logEntry); err != nil {
		t.Fatalf("parse audit log: %v", err)
	}

	if logEntry["hash"].(string) != hash {
		t.Errorf("expected hash %s in log, got %s", hash, logEntry["hash"])
	}

	if logEntry["new_original_name"].(string) != "audited_file.txt" {
		t.Errorf("expected new_original_name to be audited_file.txt, got %s", logEntry["new_original_name"])
	}
}

// TestFileRenameEndToEnd tests a complete workflow with real-world data
func TestFileRenameEndToEnd(t *testing.T) {
	srv := newTestServer(t)

	// Scenario: User uploads a photo with a temporary name, then renames it
	scenarios := []struct {
		originalName string
		newName      string
		content      []byte
		mimeType     string
		fullRename   bool
	}{
		{
			originalName: "IMG_20231115_143022.jpg",
			newName:      "golden_gate_bridge_sunset.jpg",
			content:      []byte("JPEG image data"),
			mimeType:     "image/jpeg",
			fullRename:   true,
		},
		{
			originalName: "document.pdf",
			newName:      "2023_annual_report.pdf",
			content:      []byte("PDF document content"),
			mimeType:     "application/pdf",
			fullRename:   false,
		},
		{
			originalName: "Recording_001.mp3",
			newName:      "podcast_episode_42_final.mp3",
			content:      []byte("MP3 audio data"),
			mimeType:     "audio/mpeg",
			fullRename:   true,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.originalName, func(t *testing.T) {
			// Upload file with original name
			hash := uploadTestFile(t, srv, scenario.originalName, scenario.mimeType, scenario.content)

			// Verify upload
			metadata := srv.storage.FindByHash(hash)
			if metadata == nil {
				t.Fatal("file should be uploaded")
			}
			if metadata.OriginalName != scenario.originalName {
				t.Errorf("expected original name %s, got %s", scenario.originalName, metadata.OriginalName)
			}

			// Rename the file
			req := fileRenameRequest{
				Hash:             hash,
				NewName:          scenario.newName,
				UpdateStoredFile: scenario.fullRename,
			}

			resp := makeRenameRequest(t, srv, req)
			if resp.Code != http.StatusOK {
				t.Fatalf("rename failed: %d: %s", resp.Code, resp.Body.String())
			}

			var result map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			// Verify rename result
			if result["disk_renamed"].(bool) != scenario.fullRename {
				t.Errorf("expected disk_renamed to be %v, got %v", scenario.fullRename, result["disk_renamed"])
			}

			resultMetadata := result["metadata"].(map[string]any)
			if resultMetadata["original_name"].(string) != scenario.newName {
				t.Errorf("expected new name %s, got %s", scenario.newName, resultMetadata["original_name"])
			}

			// Verify file is still accessible by hash
			updatedMetadata := srv.storage.FindByHash(hash)
			if updatedMetadata == nil {
				t.Fatal("file should still be findable by hash")
			}
			if updatedMetadata.OriginalName != scenario.newName {
				t.Errorf("metadata not updated: expected %s, got %s", scenario.newName, updatedMetadata.OriginalName)
			}

			// Verify file content is still accessible
			storedPath := updatedMetadata.StoredPath
			absPath := filepath.Join(srv.cfg.DataDir, filepath.FromSlash(storedPath))
			if _, err := os.Stat(absPath); err != nil {
				t.Errorf("file should exist at %s: %v", absPath, err)
			}

			// Verify content hasn't changed
			content, err := os.ReadFile(absPath)
			if err != nil {
				t.Fatalf("read file: %v", err)
			}
			if !bytes.Equal(content, scenario.content) {
				t.Error("file content should not change after rename")
			}
		})
	}
}

// TestFileRenameSpecialCharacters tests renaming with special characters in filenames
func TestFileRenameSpecialCharacters(t *testing.T) {
	srv := newTestServer(t)

	testCases := []struct {
		name     string
		newName  string
		expected string // Expected sanitized name in stored path
	}{
		{
			name:     "spaces and special chars",
			newName:  "My Document (Final Version) v2.0.pdf",
			expected: "my-document-final-version-v2-0",
		},
		{
			name:     "unicode characters",
			newName:  "café_résumé_2023.pdf",
			expected: "caf",
		},
		{
			name:     "multiple extensions",
			newName:  "archive.tar.gz",
			expected: "archive-tar",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hash := uploadTestFile(t, srv, "original.pdf", "application/pdf", []byte("content"))

			req := fileRenameRequest{
				Hash:             hash,
				NewName:          tc.newName,
				UpdateStoredFile: true,
			}

			resp := makeRenameRequest(t, srv, req)
			if resp.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
			}

			var result map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			metadata := result["metadata"].(map[string]any)
			storedPath := metadata["stored_path"].(string)

			// The stored path should contain the sanitized version
			if !strings.Contains(storedPath, tc.expected) {
				t.Errorf("expected stored path to contain '%s', got '%s'", tc.expected, storedPath)
			}

			// But the original name should be preserved exactly
			if metadata["original_name"].(string) != tc.newName {
				t.Errorf("expected original_name to be preserved as '%s', got '%s'", tc.newName, metadata["original_name"])
			}
		})
	}
}

// Helper functions

// uploadTestFile uploads a test file and returns its hash
func uploadTestFile(t *testing.T, srv *Server, filename, mimeType string, content []byte) string {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	fileWriter, err := writer.CreateFormFile("files", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	fileWriter.Write(content)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()

	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("upload failed: %d: %s", resp.Code, resp.Body.String())
	}

	var result UnifiedIngestResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}

	// Check if it's a media file or generic file
	if len(result.Results.Media) > 0 {
		return result.Results.Media[0].Hash
	}
	
	// For generic files, we need to use StoreFile API which returns hash
	// Let's use the media endpoint for images, and directly store other files
	if strings.HasPrefix(mimeType, "image/") || strings.HasPrefix(mimeType, "video/") || strings.HasPrefix(mimeType, "audio/") {
		if len(result.Results.Media) == 0 {
			t.Fatal("expected media result from upload")
		}
		return result.Results.Media[0].Hash
	}
	
	// For non-media files, upload using StoreFile directly
	file := bytes.NewReader(content)
	storeResult, err := srv.storage.StoreFile(storage.StoreRequest{
		Reader:   file,
		Filename: filename,
		MimeType: mimeType,
		Size:     int64(len(content)),
		Metadata: map[string]string{},
	})
	if err != nil {
		t.Fatalf("store file: %v", err)
	}
	
	return storeResult.Metadata.Hash
}

// makeRenameRequest makes a rename request and returns the response
func makeRenameRequest(t *testing.T, srv *Server, req fileRenameRequest) *httptest.ResponseRecorder {
	t.Helper()

	body := &bytes.Buffer{}
	if err := json.NewEncoder(body).Encode(req); err != nil {
		t.Fatalf("encode request: %v", err)
	}

	httpReq := httptest.NewRequest(http.MethodPatch, "/files/rename", body)
	httpReq.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	srv.router.ServeHTTP(resp, httpReq)

	return resp
}
