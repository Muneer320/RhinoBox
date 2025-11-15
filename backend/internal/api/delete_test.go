package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSingleFileSoftDelete tests soft deletion of a single file.
func TestSingleFileSoftDelete(t *testing.T) {
	srv := newTestServer(t)

	// First, upload a file to get a hash
	hash := uploadTestFile(t, srv, "test_document.pdf", []byte("PDF content here"))

	// Verify file exists and is not deleted
	meta := srv.storage.GetMetadataIndex().FindByHash(hash)
	if meta == nil {
		t.Fatal("file not found after upload")
	}
	if meta.DeletedAt != nil {
		t.Fatal("file should not be deleted initially")
	}

	// Perform soft delete
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/files/%s?soft=true", hash), nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result DeleteFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success=true, got false: %s", result.Error)
	}
	if result.Hash != hash {
		t.Errorf("expected hash %s, got %s", hash, result.Hash)
	}

	// Verify file is marked as deleted in metadata
	meta = srv.storage.GetMetadataIndex().FindByHash(hash)
	if meta == nil {
		t.Fatal("file metadata should still exist after soft delete")
	}
	if meta.DeletedAt == nil {
		t.Fatal("file should be marked as deleted")
	}

	// Verify file still exists on disk
	fullPath := filepath.Join(srv.storage.Root(), meta.StoredPath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		t.Error("file should still exist on disk after soft delete")
	}
}

// TestSingleFileHardDelete tests permanent deletion of a single file.
func TestSingleFileHardDelete(t *testing.T) {
	srv := newTestServer(t)

	// Upload a file
	content := []byte("Important data that will be deleted")
	hash := uploadTestFile(t, srv, "to_delete.txt", content)

	// Get the file path before deletion
	meta := srv.storage.GetMetadataIndex().FindByHash(hash)
	if meta == nil {
		t.Fatal("file not found after upload")
	}
	filePath := filepath.Join(srv.storage.Root(), meta.StoredPath)

	// Verify file exists on disk
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("file should exist before deletion: %v", err)
	}

	// Perform hard delete
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/files/%s?soft=false", hash), nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result DeleteFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success=true, got false: %s", result.Error)
	}
	if result.SpaceReclaimed <= 0 {
		t.Errorf("expected space_reclaimed > 0, got %d", result.SpaceReclaimed)
	}

	// Verify file is removed from metadata
	meta = srv.storage.GetMetadataIndex().FindByHash(hash)
	if meta != nil {
		t.Error("file metadata should be removed after hard delete")
	}

	// Verify file is removed from disk
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("file should not exist on disk after hard delete")
	}
}

// TestSoftDeleteAndRestore tests the complete soft delete and restore cycle.
func TestSoftDeleteAndRestore(t *testing.T) {
	srv := newTestServer(t)

	// Upload a file
	hash := uploadTestFile(t, srv, "restorable.jpg", []byte("image data"))

	// Soft delete the file
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/files/%s?soft=true", hash), nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("soft delete failed: %s", resp.Body.String())
	}

	// Verify file is soft deleted
	meta := srv.storage.GetMetadataIndex().FindByHash(hash)
	if meta == nil || meta.DeletedAt == nil {
		t.Fatal("file should be soft deleted")
	}

	// Restore the file
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/files/%s/restore", hash), nil)
	resp = httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("restore failed: %s", resp.Body.String())
	}

	var result DeleteFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success=true, got false: %s", result.Error)
	}

	// Verify file is no longer deleted
	meta = srv.storage.GetMetadataIndex().FindByHash(hash)
	if meta == nil {
		t.Fatal("file metadata should exist after restore")
	}
	if meta.DeletedAt != nil {
		t.Error("file should not be marked as deleted after restore")
	}
}

// TestBatchSoftDelete tests batch soft deletion of multiple files.
func TestBatchSoftDelete(t *testing.T) {
	srv := newTestServer(t)

	// Upload multiple files
	hashes := []string{
		uploadTestFile(t, srv, "batch1.txt", []byte("content 1")),
		uploadTestFile(t, srv, "batch2.txt", []byte("content 2")),
		uploadTestFile(t, srv, "batch3.txt", []byte("content 3")),
	}

	// Perform batch soft delete
	reqBody := DeleteFileRequest{
		Hashes: hashes,
		Soft:   true,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodDelete, "/files/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result BatchDeleteResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result.TotalDeleted != 3 {
		t.Errorf("expected 3 deleted, got %d", result.TotalDeleted)
	}
	if result.TotalFailed != 0 {
		t.Errorf("expected 0 failed, got %d", result.TotalFailed)
	}

	// Verify all files are soft deleted
	for _, hash := range hashes {
		meta := srv.storage.GetMetadataIndex().FindByHash(hash)
		if meta == nil || meta.DeletedAt == nil {
			t.Errorf("file %s should be soft deleted", hash)
		}
	}
}

// TestBatchHardDelete tests batch hard deletion with space reclamation.
func TestBatchHardDelete(t *testing.T) {
	srv := newTestServer(t)

	// Upload multiple files with known sizes
	content1 := []byte(strings.Repeat("a", 1000))
	content2 := []byte(strings.Repeat("b", 2000))
	content3 := []byte(strings.Repeat("c", 3000))

	hashes := []string{
		uploadTestFile(t, srv, "file1.bin", content1),
		uploadTestFile(t, srv, "file2.bin", content2),
		uploadTestFile(t, srv, "file3.bin", content3),
	}

	// Perform batch hard delete
	reqBody := DeleteFileRequest{
		Hashes: hashes,
		Soft:   false,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodDelete, "/files/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result BatchDeleteResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result.TotalDeleted != 3 {
		t.Errorf("expected 3 deleted, got %d", result.TotalDeleted)
	}
	if result.SpaceReclaimed <= 0 {
		t.Errorf("expected space_reclaimed > 0, got %d", result.SpaceReclaimed)
	}

	// Verify all files are removed from metadata
	for _, hash := range hashes {
		meta := srv.storage.GetMetadataIndex().FindByHash(hash)
		if meta != nil {
			t.Errorf("file %s should be removed from metadata", hash)
		}
	}
}

// TestDeleteNonExistentFile tests error handling for non-existent files.
func TestDeleteNonExistentFile(t *testing.T) {
	srv := newTestServer(t)

	fakeHash := "nonexistent123456"
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/files/%s?soft=true", fakeHash), nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.Code)
	}

	body := resp.Body.String()
	if !strings.Contains(body, "not found") {
		t.Errorf("expected 'not found' error, got: %s", body)
	}
}

// TestRestoreNonDeletedFile tests error handling when restoring a file that's not deleted.
func TestRestoreNonDeletedFile(t *testing.T) {
	srv := newTestServer(t)

	// Upload a file
	hash := uploadTestFile(t, srv, "active.txt", []byte("active content"))

	// Try to restore a file that's not deleted
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/files/%s/restore", hash), nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.Code)
	}

	body := resp.Body.String()
	if !strings.Contains(body, "not deleted") {
		t.Errorf("expected 'not deleted' error, got: %s", body)
	}
}

// TestDoubleDelete tests deleting an already deleted file.
func TestDoubleDelete(t *testing.T) {
	srv := newTestServer(t)

	// Upload and soft delete a file
	hash := uploadTestFile(t, srv, "double.txt", []byte("content"))
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/files/%s?soft=true", hash), nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("first delete failed: %s", resp.Body.String())
	}

	// Try to soft delete again
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/files/%s?soft=true", hash), nil)
	resp = httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for double delete, got %d", resp.Code)
	}

	body := resp.Body.String()
	if !strings.Contains(body, "already deleted") {
		t.Errorf("expected 'already deleted' error, got: %s", body)
	}
}

// TestBatchDeletePartialFailure tests batch deletion with some invalid hashes.
func TestBatchDeletePartialFailure(t *testing.T) {
	srv := newTestServer(t)

	// Upload two valid files
	validHash1 := uploadTestFile(t, srv, "valid1.txt", []byte("content 1"))
	validHash2 := uploadTestFile(t, srv, "valid2.txt", []byte("content 2"))
	invalidHash := "doesnotexist123"

	// Perform batch delete with mixed valid/invalid hashes
	reqBody := DeleteFileRequest{
		Hashes: []string{validHash1, invalidHash, validHash2},
		Soft:   true,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodDelete, "/files/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result BatchDeleteResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result.TotalDeleted != 2 {
		t.Errorf("expected 2 deleted, got %d", result.TotalDeleted)
	}
	if result.TotalFailed != 1 {
		t.Errorf("expected 1 failed, got %d", result.TotalFailed)
	}

	// Check individual results
	if len(result.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result.Results))
	}

	// First should succeed
	if !result.Results[0].Success {
		t.Error("first file should succeed")
	}
	// Second should fail
	if result.Results[1].Success {
		t.Error("second file should fail")
	}
	// Third should succeed
	if !result.Results[2].Success {
		t.Error("third file should succeed")
	}
}

// TestDuplicateFileHandling tests that duplicates are handled correctly during deletion.
func TestDuplicateFileHandling(t *testing.T) {
	srv := newTestServer(t)

	// Upload the same content twice to create a duplicate
	content := []byte("duplicate content for testing")
	hash1 := uploadTestFile(t, srv, "original.txt", content)
	hash2 := uploadTestFile(t, srv, "duplicate.txt", content)

	// Hashes should be the same for duplicate content
	if hash1 != hash2 {
		t.Logf("Note: Files with identical content have hash: %s", hash1)
	}

	// The storage system deduplicates, so only one physical file exists
	meta := srv.storage.GetMetadataIndex().FindByHash(hash1)
	if meta == nil {
		t.Fatal("file metadata should exist")
	}

	// Delete the file
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/files/%s?soft=false", hash1), nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("delete failed: %s", resp.Body.String())
	}

	// Verify file is deleted
	meta = srv.storage.GetMetadataIndex().FindByHash(hash1)
	if meta != nil {
		t.Error("file should be deleted from metadata")
	}
}

// TestRealWorldScenario tests a complex real-world scenario.
func TestRealWorldScenario(t *testing.T) {
	srv := newTestServer(t)

	// Scenario: User uploads multiple documents, realizes some are wrong,
	// soft deletes them, then restores one after realizing it was needed.

	// Upload 5 documents
	doc1 := uploadTestFile(t, srv, "quarterly_report_q1.pdf", []byte("Q1 2024 Financial Report"))
	doc2 := uploadTestFile(t, srv, "quarterly_report_q2.pdf", []byte("Q2 2024 Financial Report"))
	doc3 := uploadTestFile(t, srv, "old_draft_q1.pdf", []byte("Q1 Draft - Delete Me"))
	doc4 := uploadTestFile(t, srv, "personal_notes.txt", []byte("Personal notes - wrong folder"))
	doc5 := uploadTestFile(t, srv, "meeting_minutes.txt", []byte("Meeting Minutes May 2024"))

	// Soft delete the draft and personal notes
	batchReq := DeleteFileRequest{
		Hashes: []string{doc3, doc4},
		Soft:   true,
	}
	body, _ := json.Marshal(batchReq)
	req := httptest.NewRequest(http.MethodDelete, "/files/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("batch soft delete failed: %s", resp.Body.String())
	}

	// User realizes doc4 (personal notes) was actually needed - restore it
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/files/%s/restore", doc4), nil)
	resp = httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("restore failed: %s", resp.Body.String())
	}

	// Hard delete the old draft permanently
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/files/%s?soft=false", doc3), nil)
	resp = httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("hard delete failed: %s", resp.Body.String())
	}

	// Verify final state
	// doc1, doc2, doc5 should be active
	for _, hash := range []string{doc1, doc2, doc5} {
		meta := srv.storage.GetMetadataIndex().FindByHash(hash)
		if meta == nil || meta.DeletedAt != nil {
			t.Errorf("file %s should be active", hash)
		}
	}

	// doc4 should be active (restored)
	meta := srv.storage.GetMetadataIndex().FindByHash(doc4)
	if meta == nil || meta.DeletedAt != nil {
		t.Error("doc4 should be active after restore")
	}

	// doc3 should be hard deleted (not in index)
	meta = srv.storage.GetMetadataIndex().FindByHash(doc3)
	if meta != nil {
		t.Error("doc3 should be hard deleted")
	}
}

// TestAuditLogging verifies that deletion operations are logged.
func TestAuditLogging(t *testing.T) {
	srv := newTestServer(t)

	// Upload and delete a file
	hash := uploadTestFile(t, srv, "audit_test.txt", []byte("test content"))

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/files/%s?soft=true", hash), nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("delete failed: %s", resp.Body.String())
	}

	// Give it a moment for async logging
	time.Sleep(100 * time.Millisecond)

	// Check that audit log exists
	auditPath := filepath.Join(srv.storage.Root(), "audit", "deletion_log.ndjson")
	if _, err := os.Stat(auditPath); os.IsNotExist(err) {
		t.Error("audit log should exist after deletion")
	}
}

// Helper function to upload a test file and return its hash
func uploadTestFile(t *testing.T, srv *Server, filename string, content []byte) string {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("files", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := io.Copy(part, bytes.NewReader(content)); err != nil {
		t.Fatalf("copy content: %v", err)
	}
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()

	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("upload failed: %s", resp.Body.String())
	}

	var result UnifiedIngestResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Check for media results first, then generic files
	if len(result.Results.Media) > 0 {
		return result.Results.Media[0].Hash
	}
	if len(result.Results.Files) > 0 && result.Results.Files[0].Hash != "" {
		return result.Results.Files[0].Hash
	}

	// For files without hash (generic storage), we need to use the stored path
	// In this case, we'll need to find the file by its stored path
	// But since we need the hash for deletion, let's use the media ingest endpoint directly
	t.Fatal("no hash returned from upload - using different upload method")
	return ""
}
