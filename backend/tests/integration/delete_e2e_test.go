package integration_test

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
	"testing"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
	"log/slog"
)

// TestFileDeletionEndToEndWithRealWorldData tests the complete file deletion flow
// using real-world files from the user's Downloads directory.
func TestFileDeletionEndToEndWithRealWorldData(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-world file test in short mode")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	downloadsDir := filepath.Join(homeDir, "Downloads")
	if _, err := os.Stat(downloadsDir); os.IsNotExist(err) {
		t.Skip("Downloads folder not found")
	}

	tmpDir := t.TempDir()
	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        tmpDir,
		MaxUploadBytes: 500 * 1024 * 1024,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Step 1: Find and upload real-world files from Downloads
	entries, err := os.ReadDir(downloadsDir)
	if err != nil {
		t.Fatalf("read Downloads: %v", err)
	}

	type uploadedFile struct {
		OriginalName string
		Hash         string
		StoredPath   string
		FullPath     string
		Size         int64
	}

	var uploadedFiles []uploadedFile
	maxUploads := 5 // Limit to 5 files for testing

	for _, entry := range entries {
		if entry.IsDir() || len(uploadedFiles) >= maxUploads {
			continue
		}

		// Skip hidden files and very large files
		if entry.Name()[0] == '.' {
			continue
		}

		filePath := filepath.Join(downloadsDir, entry.Name())
		info, err := entry.Info()
		if err != nil || info.Size() > 50*1024*1024 || info.Size() == 0 {
			continue
		}

		fileData, err := os.ReadFile(filePath)
		if err != nil {
			t.Logf("Skipping %s: %v", entry.Name(), err)
			continue
		}

		// Upload the file
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", entry.Name())
		if err != nil {
			t.Logf("Skipping %s: failed to create form file: %v", entry.Name(), err)
			continue
		}
		if _, err := part.Write(fileData); err != nil {
			t.Logf("Skipping %s: failed to write content: %v", entry.Name(), err)
			continue
		}
		if err := writer.WriteField("comment", fmt.Sprintf("E2E test file: %s", entry.Name())); err != nil {
			t.Logf("Skipping %s: failed to write comment: %v", entry.Name(), err)
			continue
		}
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Logf("Skipping %s: upload failed with status %d: %s", entry.Name(), rec.Code, rec.Body.String())
			continue
		}

		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Logf("Skipping %s: failed to parse response: %v", entry.Name(), err)
			continue
		}

		stored, ok := resp["stored"].([]any)
		if !ok || len(stored) == 0 {
			t.Logf("Skipping %s: no stored files in response", entry.Name())
			continue
		}

		fileInfo := stored[0].(map[string]any)
		hash, ok := fileInfo["hash"].(string)
		if !ok || hash == "" {
			t.Logf("Skipping %s: missing hash in response", entry.Name())
			continue
		}

		storedPath, ok := fileInfo["path"].(string)
		if !ok {
			t.Logf("Skipping %s: missing path in response", entry.Name())
			continue
		}

		fullPath := filepath.Join(tmpDir, filepath.FromSlash(storedPath))

		uploadedFiles = append(uploadedFiles, uploadedFile{
			OriginalName: entry.Name(),
			Hash:         hash,
			StoredPath:   storedPath,
			FullPath:     fullPath,
			Size:         info.Size(),
		})

		t.Logf("✓ Uploaded real file: %s (hash: %s, size: %d bytes, path: %s)",
			entry.Name(), hash[:12], info.Size(), storedPath)
	}

	if len(uploadedFiles) == 0 {
		t.Skip("no suitable files found in Downloads for testing")
	}

	t.Logf("✓ Successfully uploaded %d real-world files for deletion testing", len(uploadedFiles))

	// Step 2: Verify all files exist before deletion
	for _, uf := range uploadedFiles {
		if _, err := os.Stat(uf.FullPath); os.IsNotExist(err) {
			t.Fatalf("uploaded file should exist before deletion: %s", uf.FullPath)
		}
	}

	// Step 3: Delete files one by one and verify
	for i, uf := range uploadedFiles {
		t.Logf("Testing deletion of file %d/%d: %s", i+1, len(uploadedFiles), uf.OriginalName)

		// Delete the file via API
		deleteReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/files/%s", uf.Hash), nil)
		deleteResp := httptest.NewRecorder()
		srv.Router().ServeHTTP(deleteResp, deleteReq)

		if deleteResp.Code != http.StatusOK {
			t.Fatalf("expected 200 for delete, got %d: %s", deleteResp.Code, deleteResp.Body.String())
		}

		var deleteResult map[string]any
		if err := json.Unmarshal(deleteResp.Bytes(), &deleteResult); err != nil {
			t.Fatalf("failed to parse delete response: %v", err)
		}

		// Verify delete response
		if deleted, ok := deleteResult["deleted"].(bool); !ok || !deleted {
			t.Fatalf("expected deleted=true in response")
		}
		if hash, ok := deleteResult["hash"].(string); !ok || hash != uf.Hash {
			t.Fatalf("expected hash %s in response, got %s", uf.Hash, hash)
		}
		if originalName, ok := deleteResult["original_name"].(string); !ok || originalName != uf.OriginalName {
			t.Fatalf("expected original_name %s in response, got %s", uf.OriginalName, originalName)
		}

		// Verify file is physically deleted
		if _, err := os.Stat(uf.FullPath); !os.IsNotExist(err) {
			t.Fatalf("file should be deleted from filesystem: %s", uf.FullPath)
		}

		// Verify file cannot be deleted again (should return 404)
		deleteReq2 := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/files/%s", uf.Hash), nil)
		deleteResp2 := httptest.NewRecorder()
		srv.Router().ServeHTTP(deleteResp2, deleteReq2)

		if deleteResp2.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for already-deleted file, got %d: %s", deleteResp2.Code, deleteResp2.Body.String())
		}

		t.Logf("✓ Successfully deleted and verified: %s", uf.OriginalName)
	}

	// Step 4: Verify all files are deleted
	for _, uf := range uploadedFiles {
		if _, err := os.Stat(uf.FullPath); !os.IsNotExist(err) {
			t.Errorf("file should be deleted: %s", uf.FullPath)
		}
	}

	// Step 5: Test deletion of non-existent file
	t.Logf("Testing deletion of non-existent file...")
	nonExistentHash := "nonexistent_hash_1234567890123456789012345678901234567890123456789012345678901234"
	deleteReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/files/%s", nonExistentHash), nil)
	deleteResp := httptest.NewRecorder()
	srv.Router().ServeHTTP(deleteResp, deleteReq)

	if deleteResp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-existent file, got %d: %s", deleteResp.Code, deleteResp.Body.String())
	}

	var errorResp map[string]any
	if err := json.Unmarshal(deleteResp.Bytes(), &errorResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}
	if errorMsg, ok := errorResp["error"].(string); !ok || errorMsg == "" {
		t.Fatalf("expected error message in response")
	}

	t.Logf("✓ Non-existent file deletion correctly returns 404")

	// Step 6: Verify deletion log exists
	logPath := filepath.Join(tmpDir, "metadata", "delete_log.ndjson")
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Fatalf("deletion log should exist: %s", logPath)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read deletion log: %v", err)
	}

	// Verify log contains entries for all deleted files
	for _, uf := range uploadedFiles {
		if !bytes.Contains(logData, []byte(uf.Hash)) {
			t.Errorf("deletion log should contain hash for %s", uf.OriginalName)
		}
	}

	t.Logf("✓ Deletion audit log verified with %d entries", len(uploadedFiles))

	t.Logf("✓ End-to-end deletion test completed successfully with %d real-world files", len(uploadedFiles))
}

// TestFileDeletionBatchWithRealWorldData tests batch deletion scenarios
func TestFileDeletionBatchWithRealWorldData(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-world file test in short mode")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	downloadsDir := filepath.Join(homeDir, "Downloads")
	if _, err := os.Stat(downloadsDir); os.IsNotExist(err) {
		t.Skip("Downloads folder not found")
	}

	tmpDir := t.TempDir()
	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        tmpDir,
		MaxUploadBytes: 500 * 1024 * 1024,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Upload multiple files
	entries, err := os.ReadDir(downloadsDir)
	if err != nil {
		t.Fatalf("read Downloads: %v", err)
	}

	var hashes []string
	uploadCount := 0
	maxUploads := 3

	for _, entry := range entries {
		if entry.IsDir() || uploadCount >= maxUploads {
			continue
		}

		if entry.Name()[0] == '.' {
			continue
		}

		filePath := filepath.Join(downloadsDir, entry.Name())
		info, err := entry.Info()
		if err != nil || info.Size() > 10*1024*1024 || info.Size() == 0 {
			continue
		}

		fileData, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", entry.Name())
		part.Write(fileData)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)

		if rec.Code == http.StatusOK {
			var resp map[string]any
			json.Unmarshal(rec.Bytes(), &resp)
			if stored, ok := resp["stored"].([]any); ok && len(stored) > 0 {
				fileInfo := stored[0].(map[string]any)
				if hash, ok := fileInfo["hash"].(string); ok && hash != "" {
					hashes = append(hashes, hash)
					uploadCount++
				}
			}
		}
	}

	if len(hashes) == 0 {
		t.Skip("no files uploaded for batch deletion test")
	}

	// Delete all files in sequence
	for i, hash := range hashes {
		deleteReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/files/%s", hash), nil)
		deleteResp := httptest.NewRecorder()
		srv.Router().ServeHTTP(deleteResp, deleteReq)

		if deleteResp.Code != http.StatusOK {
			t.Fatalf("delete %d failed: %d %s", i, deleteResp.Code, deleteResp.Body.String())
		}
	}

	t.Logf("✓ Batch deletion of %d files completed successfully", len(hashes))
}
