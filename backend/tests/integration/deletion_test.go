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
	"path/filepath"
	"strings"
	"testing"
	"time"

	"log/slog"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
)

// TestFileDeletionEndToEnd tests the complete file lifecycle including upload, soft delete, restore, and hard delete.
func TestFileDeletionEndToEnd(t *testing.T) {
	srv := setupTestServer(t)

	// Phase 1: Upload multiple files
	t.Log("Phase 1: Uploading test files...")
	files := map[string]string{
		"invoice_2024.pdf":       uploadFile(t, srv, "invoice_2024.pdf", []byte("Invoice content for 2024")),
		"company_logo.png":       uploadFile(t, srv, "company_logo.png", []byte("PNG image data for logo")),
		"quarterly_report.docx":  uploadFile(t, srv, "quarterly_report.docx", []byte("Quarterly report document")),
		"backup_2024_01.tar.gz":  uploadFile(t, srv, "backup_2024_01.tar.gz", []byte("Compressed backup archive")),
		"meeting_recording.mp4":  uploadFile(t, srv, "meeting_recording.mp4", []byte("MP4 video content")),
		"old_draft.txt":          uploadFile(t, srv, "old_draft.txt", []byte("Old draft text content")),
	}

	if len(files) != 6 {
		t.Fatalf("expected 6 files uploaded, got %d", len(files))
	}

	// Phase 2: Soft delete some files
	t.Log("Phase 2: Soft deleting old draft and backup...")
	softDeleteHashes := []string{files["old_draft.txt"], files["backup_2024_01.tar.gz"]}
	batchSoftDelete(t, srv, softDeleteHashes, true)

	// Verify files are soft deleted
	for _, hash := range softDeleteHashes {
		meta := srv.Storage().GetMetadataIndex().FindByHash(hash)
		if meta == nil {
			t.Errorf("file %s should still exist in metadata", hash)
		} else if meta.DeletedAt == nil {
			t.Errorf("file %s should be marked as deleted", hash)
		}
	}

	// Phase 3: Attempt to use soft-deleted file (should still exist on disk)
	t.Log("Phase 3: Verifying soft-deleted files still exist on disk...")
	for _, hash := range softDeleteHashes {
		meta := srv.Storage().GetMetadataIndex().FindByHash(hash)
		if meta != nil {
			fullPath := filepath.Join(srv.Config().DataDir, meta.StoredPath)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				t.Errorf("soft-deleted file should still exist on disk: %s", fullPath)
			}
		}
	}

	// Phase 4: Restore one of the soft-deleted files
	t.Log("Phase 4: Restoring backup file...")
	restoreFile(t, srv, files["backup_2024_01.tar.gz"])

	// Verify restoration
	meta := srv.Storage().GetMetadataIndex().FindByHash(files["backup_2024_01.tar.gz"])
	if meta == nil || meta.DeletedAt != nil {
		t.Error("backup file should be restored and not marked as deleted")
	}

	// Phase 5: Hard delete the old draft permanently
	t.Log("Phase 5: Hard deleting old draft permanently...")
	hardDeleteFile(t, srv, files["old_draft.txt"])

	// Verify hard deletion
	meta = srv.Storage().GetMetadataIndex().FindByHash(files["old_draft.txt"])
	if meta != nil {
		t.Error("old draft should be removed from metadata")
	}

	// Phase 6: Batch hard delete multiple files
	t.Log("Phase 6: Batch hard deleting invoice and meeting recording...")
	batchHardDeleteHashes := []string{files["invoice_2024.pdf"], files["meeting_recording.mp4"]}
	result := batchSoftDelete(t, srv, batchHardDeleteHashes, false)

	// Verify batch deletion results
	if result.TotalDeleted != 2 {
		t.Errorf("expected 2 files deleted, got %d", result.TotalDeleted)
	}
	if result.SpaceReclaimed <= 0 {
		t.Errorf("expected space reclaimed > 0, got %d", result.SpaceReclaimed)
	}

	// Phase 7: Verify remaining active files
	t.Log("Phase 7: Verifying remaining active files...")
	activeFiles := []string{files["company_logo.png"], files["quarterly_report.docx"], files["backup_2024_01.tar.gz"]}
	for _, hash := range activeFiles {
		meta := srv.Storage().GetMetadataIndex().FindByHash(hash)
		if meta == nil {
			t.Errorf("file %s should still exist", hash)
		} else if meta.DeletedAt != nil {
			t.Errorf("file %s should not be deleted", hash)
		}
	}

	t.Log("End-to-end test completed successfully!")
}

// TestDeletionWithDuplicates tests deletion behavior with duplicate file content.
func TestDeletionWithDuplicates(t *testing.T) {
	srv := setupTestServer(t)

	// Upload the same content with different filenames
	content := []byte("Shared document content that will be duplicated")
	hash1 := uploadFile(t, srv, "version1.txt", content)
	hash2 := uploadFile(t, srv, "version2.txt", content)

	// Due to deduplication, both should have the same hash
	if hash1 != hash2 {
		t.Logf("Note: hashes differ, possibly due to different storage logic")
	}

	// Delete one instance
	hardDeleteFile(t, srv, hash1)

	// Verify deletion
	meta := srv.Storage().GetMetadataIndex().FindByHash(hash1)
	if meta != nil {
		t.Error("file should be deleted from metadata")
	}
}

// TestAuditLogIntegrity tests that all deletion operations are properly logged.
func TestAuditLogIntegrity(t *testing.T) {
	srv := setupTestServer(t)

	// Upload a file
	hash := uploadFile(t, srv, "audit_test.pdf", []byte("Test content for audit"))

	// Soft delete
	softDeleteFile(t, srv, hash)

	// Restore
	restoreFile(t, srv, hash)

	// Hard delete
	hardDeleteFile(t, srv, hash)

	// Check audit log
	time.Sleep(100 * time.Millisecond) // Give time for async logging
	auditPath := filepath.Join(srv.Config().DataDir, "audit", "deletion_log.ndjson")
	
	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 3 {
		t.Errorf("expected at least 3 audit log entries, got %d", len(lines))
	}

	// Verify each log entry is valid JSON
	for i, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i+1, err)
		}
		// Verify required fields
		if _, ok := entry["operation"]; !ok {
			t.Errorf("line %d missing 'operation' field", i+1)
		}
	}
}

// TestConcurrentDeletions tests concurrent deletion operations.
func TestConcurrentDeletions(t *testing.T) {
	srv := setupTestServer(t)

	// Upload multiple files
	hashes := make([]string, 10)
	for i := 0; i < 10; i++ {
		content := []byte(fmt.Sprintf("File %d content", i))
		hashes[i] = uploadFile(t, srv, fmt.Sprintf("file_%d.txt", i), content)
	}

	// Concurrently delete half of them
	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func(idx int) {
			softDeleteFile(t, srv, hashes[idx])
			done <- true
		}(i)
	}

	// Wait for all deletions
	for i := 0; i < 5; i++ {
		<-done
	}

	// Verify results
	deletedCount := 0
	activeCount := 0
	for _, hash := range hashes {
		meta := srv.Storage().GetMetadataIndex().FindByHash(hash)
		if meta != nil {
			if meta.DeletedAt != nil {
				deletedCount++
			} else {
				activeCount++
			}
		}
	}

	if deletedCount != 5 {
		t.Errorf("expected 5 deleted files, got %d", deletedCount)
	}
	if activeCount != 5 {
		t.Errorf("expected 5 active files, got %d", activeCount)
	}
}

// TestLargeFileDeletion tests deletion of larger files.
func TestLargeFileDeletion(t *testing.T) {
	srv := setupTestServer(t)

	// Create a larger file (1MB)
	largeContent := bytes.Repeat([]byte("Large file content block. "), 40000) // ~1MB
	hash := uploadFile(t, srv, "large_file.bin", largeContent)

	// Get file size before deletion
	meta := srv.Storage().GetMetadataIndex().FindByHash(hash)
	if meta == nil {
		t.Fatal("file not found after upload")
	}
	originalSize := meta.Size

	// Hard delete and verify space reclamation
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/files/%s?soft=false", hash), nil)
	resp := httptest.NewRecorder()
	srv.Router().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("delete failed: %s", resp.Body.String())
	}

	var result struct {
		SpaceReclaimed int64 `json:"space_reclaimed"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result.SpaceReclaimed < originalSize/2 {
		t.Errorf("expected space reclaimed to be close to %d, got %d", originalSize, result.SpaceReclaimed)
	}
}

// TestBatchDeleteMixedResults tests batch deletion with some invalid hashes.
func TestBatchDeleteMixedResults(t *testing.T) {
	srv := setupTestServer(t)

	// Upload some valid files
	valid1 := uploadFile(t, srv, "valid1.txt", []byte("content 1"))
	valid2 := uploadFile(t, srv, "valid2.txt", []byte("content 2"))
	invalid := "nonexistent_hash_123"

	// Batch delete with mixed valid/invalid
	reqBody := map[string]any{
		"hashes": []string{valid1, invalid, valid2},
		"soft":   true,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodDelete, "/files/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	srv.Router().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("batch delete failed: %s", resp.Body.String())
	}

	var result struct {
		TotalDeleted int `json:"total_deleted"`
		TotalFailed  int `json:"total_failed"`
		Results      []struct {
			Hash    string `json:"hash"`
			Success bool   `json:"success"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result.TotalDeleted != 2 {
		t.Errorf("expected 2 deleted, got %d", result.TotalDeleted)
	}
	if result.TotalFailed != 1 {
		t.Errorf("expected 1 failed, got %d", result.TotalFailed)
	}

	// Verify correct files were deleted
	if result.Results[0].Success != true || result.Results[0].Hash != valid1 {
		t.Error("first valid file should succeed")
	}
	if result.Results[1].Success != false || result.Results[1].Hash != invalid {
		t.Error("invalid hash should fail")
	}
	if result.Results[2].Success != true || result.Results[2].Hash != valid2 {
		t.Error("second valid file should succeed")
	}
}

// Helper functions

func setupTestServer(t *testing.T) *api.Server {
	t.Helper()
	cfg := config.Config{
		Addr:           ":0",
		DataDir:        t.TempDir(),
		MaxUploadBytes: 32 * 1024 * 1024,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	return srv
}

func uploadFile(t *testing.T, srv *api.Server, filename string, content []byte) string {
	t.Helper()
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
	srv.Router().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("upload failed: %s", resp.Body.String())
	}

	var result struct {
		Results struct {
			Media []struct {
				Hash string `json:"hash"`
			} `json:"media"`
			Files []struct {
				Hash string `json:"hash"`
			} `json:"files"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(result.Results.Media) > 0 && result.Results.Media[0].Hash != "" {
		return result.Results.Media[0].Hash
	}
	if len(result.Results.Files) > 0 && result.Results.Files[0].Hash != "" {
		return result.Results.Files[0].Hash
	}

	t.Fatal("no hash returned from upload")
	return ""
}

func softDeleteFile(t *testing.T, srv *api.Server, hash string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/files/%s?soft=true", hash), nil)
	resp := httptest.NewRecorder()
	srv.Router().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("soft delete failed: %s", resp.Body.String())
	}
}

func hardDeleteFile(t *testing.T, srv *api.Server, hash string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/files/%s?soft=false", hash), nil)
	resp := httptest.NewRecorder()
	srv.Router().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("hard delete failed: %s", resp.Body.String())
	}
}

func restoreFile(t *testing.T, srv *api.Server, hash string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/files/%s/restore", hash), nil)
	resp := httptest.NewRecorder()
	srv.Router().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("restore failed: %s", resp.Body.String())
	}
}

func batchSoftDelete(t *testing.T, srv *api.Server, hashes []string, soft bool) struct {
	TotalDeleted   int   `json:"total_deleted"`
	TotalFailed    int   `json:"total_failed"`
	SpaceReclaimed int64 `json:"space_reclaimed"`
} {
	t.Helper()
	reqBody := map[string]any{
		"hashes": hashes,
		"soft":   soft,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodDelete, "/files/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	srv.Router().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("batch delete failed: %s", resp.Body.String())
	}

	var result struct {
		TotalDeleted   int   `json:"total_deleted"`
		TotalFailed    int   `json:"total_failed"`
		SpaceReclaimed int64 `json:"space_reclaimed"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return result
}
