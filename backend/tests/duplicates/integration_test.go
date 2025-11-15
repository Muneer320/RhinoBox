package duplicates_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
	"github.com/Muneer320/RhinoBox/internal/duplicates"
)

// TestDuplicateDetectionEndToEnd tests the complete duplicate detection workflow
func TestDuplicateDetectionEndToEnd(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        tmpDir,
		MaxUploadBytes: 50 * 1024 * 1024,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Step 1: Upload various files including duplicates
	t.Log("=== Step 1: Uploading files ===")
	
	testFiles := map[string][]byte{
		"document1.pdf": bytes.Repeat([]byte("PDF content "), 100),
		"photo1.jpg":    bytes.Repeat([]byte("JPEG data "), 200),
		"video1.mp4":    bytes.Repeat([]byte("MP4 video "), 500),
		"audio1.mp3":    bytes.Repeat([]byte("MP3 audio "), 150),
		"report.txt":    []byte("This is a text report with some content"),
	}

	uploadedFiles := make(map[string]map[string]any)
	for filename, content := range testFiles {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("files", filename)
		part.Write(content)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Upload %s failed: %d - %s", filename, rec.Code, rec.Body.String())
		}

		var resp map[string]any
		json.Unmarshal(rec.Body.Bytes(), &resp)
		stored := resp["stored"].([]any)[0].(map[string]any)
		uploadedFiles[filename] = stored
		t.Logf("✓ Uploaded %s (hash: %s)", filename, stored["hash"].(string)[:12])
	}

	// Step 2: Try to upload duplicate files
	t.Log("\n=== Step 2: Testing deduplication on upload ===")
	
	duplicateAttempts := []struct {
		original string
		newName  string
	}{
		{"document1.pdf", "document_copy.pdf"},
		{"photo1.jpg", "photo_backup.jpg"},
	}

	for _, dup := range duplicateAttempts {
		content := testFiles[dup.original]
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("files", dup.newName)
		part.Write(content)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Duplicate upload failed: %d - %s", rec.Code, rec.Body.String())
		}

		var resp map[string]any
		json.Unmarshal(rec.Body.Bytes(), &resp)
		stored := resp["stored"].([]any)[0].(map[string]any)
		
		if duplicate, ok := stored["duplicate"].(bool); ok && duplicate {
			t.Logf("✓ Duplicate detected: %s -> %s", dup.newName, dup.original)
		} else {
			t.Logf("Note: %s not marked as duplicate (dedup working at storage level)", dup.newName)
		}
	}

	// Step 3: Scan for duplicates
	t.Log("\n=== Step 3: Scanning for duplicates ===")
	
	scanReq := httptest.NewRequest(http.MethodPost, "/files/duplicates/scan", 
		bytes.NewBufferString(`{"deep_scan": true, "include_metadata": true}`))
	scanReq.Header.Set("Content-Type", "application/json")
	scanRec := httptest.NewRecorder()
	srv.Router().ServeHTTP(scanRec, scanReq)

	if scanRec.Code != http.StatusOK {
		t.Fatalf("Scan failed: %d - %s", scanRec.Code, scanRec.Body.String())
	}

	var scanResult duplicates.ScanResult
	json.Unmarshal(scanRec.Body.Bytes(), &scanResult)
	
	t.Logf("✓ Scan completed:")
	t.Logf("  - Total files: %d", scanResult.TotalFiles)
	t.Logf("  - Duplicates found: %d", scanResult.DuplicatesFound)
	t.Logf("  - Storage wasted: %d bytes", scanResult.StorageWasted)
	t.Logf("  - Status: %s", scanResult.Status)
	t.Logf("  - Scan ID: %s", scanResult.ScanID)

	if scanResult.TotalFiles < len(testFiles) {
		t.Errorf("Expected at least %d files, got %d", len(testFiles), scanResult.TotalFiles)
	}

	// Step 4: Get duplicate report
	t.Log("\n=== Step 4: Getting duplicate report ===")
	
	listReq := httptest.NewRequest(http.MethodGet, "/files/duplicates", nil)
	listRec := httptest.NewRecorder()
	srv.Router().ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("List duplicates failed: %d - %s", listRec.Code, listRec.Body.String())
	}

	var listResult map[string]any
	json.Unmarshal(listRec.Body.Bytes(), &listResult)
	
	t.Logf("✓ Duplicate report retrieved:")
	t.Logf("  - Total groups: %v", listResult["total_groups"])
	t.Logf("  - Total duplicates: %v", listResult["total_duplicates"])
	t.Logf("  - Storage wasted: %v bytes", listResult["storage_wasted"])

	// Step 5: Verify system integrity
	t.Log("\n=== Step 5: Verifying system integrity ===")
	
	verifyReq := httptest.NewRequest(http.MethodPost, "/files/duplicates/verify", nil)
	verifyRec := httptest.NewRecorder()
	srv.Router().ServeHTTP(verifyRec, verifyReq)

	if verifyRec.Code != http.StatusOK {
		t.Fatalf("Verify failed: %d - %s", verifyRec.Code, verifyRec.Body.String())
	}

	var verifyResult duplicates.VerifyResult
	json.Unmarshal(verifyRec.Body.Bytes(), &verifyResult)
	
	t.Logf("✓ Verification completed:")
	t.Logf("  - Metadata index count: %d", verifyResult.MetadataIndexCount)
	t.Logf("  - Physical files count: %d", verifyResult.PhysicalFilesCount)
	t.Logf("  - Hash mismatches: %d", verifyResult.HashMismatches)
	t.Logf("  - Orphaned files: %d", verifyResult.OrphanedFiles)
	t.Logf("  - Missing files: %d", verifyResult.MissingFiles)

	if len(verifyResult.Issues) > 0 {
		t.Logf("  - Issues found: %d", len(verifyResult.Issues))
		for _, issue := range verifyResult.Issues {
			t.Logf("    * %s: %s", issue.Type, issue.Message)
		}
	}

	if verifyResult.HashMismatches > 0 {
		t.Error("Found hash mismatches - data corruption detected!")
	}
}

// TestRealWorldScenarios tests various real-world duplicate scenarios
func TestRealWorldScenarios(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        tmpDir,
		MaxUploadBytes: 100 * 1024 * 1024,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Scenario 1: Multiple users upload the same presentation
	t.Log("\n=== Scenario 1: Multiple users upload same file ===")
	
	presentation := bytes.Repeat([]byte("PowerPoint slide "), 1000)
	users := []string{"alice", "bob", "charlie"}
	
	for _, user := range users {
		filename := fmt.Sprintf("%s_presentation.pptx", user)
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("files", filename)
		part.Write(presentation)
		writer.WriteField("comment", fmt.Sprintf("Uploaded by %s", user))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Upload by %s failed: %s", user, rec.Body.String())
		}
		
		t.Logf("✓ %s uploaded presentation", user)
	}

	// Scenario 2: Photo backups with identical content
	t.Log("\n=== Scenario 2: Photo backups ===")
	
	photo := bytes.Repeat([]byte("JPEG photo data "), 2000)
	photoNames := []string{"vacation.jpg", "vacation_backup.jpg", "vacation_final.jpg"}
	
	for _, name := range photoNames {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("files", name)
		part.Write(photo)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)

		if rec.Code == http.StatusOK {
			t.Logf("✓ Uploaded %s", name)
		}
	}

	// Scenario 3: Different file types with same content (edge case)
	t.Log("\n=== Scenario 3: Different extensions, same content ===")
	
	sameContent := []byte("This is the exact same text content in different files")
	extensions := []string{".txt", ".md", ".log"}
	
	for _, ext := range extensions {
		filename := fmt.Sprintf("readme%s", ext)
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("files", filename)
		part.Write(sameContent)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)

		if rec.Code == http.StatusOK {
			t.Logf("✓ Uploaded %s", filename)
		}
	}

	// Run comprehensive scan
	t.Log("\n=== Running comprehensive scan ===")
	
	scanReq := httptest.NewRequest(http.MethodPost, "/files/duplicates/scan",
		bytes.NewBufferString(`{"deep_scan": true, "include_metadata": true}`))
	scanReq.Header.Set("Content-Type", "application/json")
	scanRec := httptest.NewRecorder()
	srv.Router().ServeHTTP(scanRec, scanReq)

	if scanRec.Code != http.StatusOK {
		t.Fatalf("Final scan failed: %s", scanRec.Body.String())
	}

	var finalScan duplicates.ScanResult
	json.Unmarshal(scanRec.Body.Bytes(), &finalScan)
	
	t.Logf("\n=== Final Statistics ===")
	t.Logf("Total files in system: %d", finalScan.TotalFiles)
	t.Logf("Duplicate instances: %d", finalScan.DuplicatesFound)
	t.Logf("Storage wasted: %d bytes", finalScan.StorageWasted)
	
	if finalScan.StorageWasted > 0 {
		efficiency := float64(finalScan.StorageWasted) / float64(finalScan.TotalFiles) * 100
		t.Logf("Deduplication efficiency: %.2f%% storage saved", efficiency)
	}
}

// TestLargeScaleDuplicateDetection tests duplicate detection with many files
func TestLargeScaleDuplicateDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large scale test in short mode")
	}

	tmpDir := t.TempDir()
	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        tmpDir,
		MaxUploadBytes: 200 * 1024 * 1024,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	t.Log("=== Large Scale Test: Uploading 100+ files ===")

	// Create 50 unique files
	uniqueFiles := 50
	for i := 0; i < uniqueFiles; i++ {
		content := bytes.Repeat([]byte(fmt.Sprintf("File %d content ", i)), 50)
		filename := fmt.Sprintf("file_%03d.dat", i)
		
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("files", filename)
		part.Write(content)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Upload failed for %s", filename)
		}
	}

	t.Logf("✓ Uploaded %d unique files", uniqueFiles)

	// Scan the system
	scanReq := httptest.NewRequest(http.MethodPost, "/files/duplicates/scan",
		bytes.NewBufferString(`{"deep_scan": false, "include_metadata": false}`))
	scanReq.Header.Set("Content-Type", "application/json")
	scanRec := httptest.NewRecorder()
	srv.Router().ServeHTTP(scanRec, scanReq)

	if scanRec.Code != http.StatusOK {
		t.Fatalf("Scan failed: %s", scanRec.Body.String())
	}

	var scanResult duplicates.ScanResult
	json.Unmarshal(scanRec.Body.Bytes(), &scanResult)
	
	t.Logf("✓ Large scale scan completed in %v", 
		scanResult.CompletedAt.Sub(scanResult.StartedAt))
	t.Logf("  - Total files: %d", scanResult.TotalFiles)
	t.Logf("  - Duplicates: %d", scanResult.DuplicatesFound)

	if scanResult.TotalFiles != uniqueFiles {
		t.Errorf("Expected %d files, got %d", uniqueFiles, scanResult.TotalFiles)
	}
}

// TestOrphanedFileDetection tests detection of files not in metadata index
func TestOrphanedFileDetection(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        tmpDir,
		MaxUploadBytes: 50 * 1024 * 1024,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Upload a normal file
	t.Log("=== Testing orphaned file detection ===")
	
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("files", "normal.txt")
	part.Write([]byte("normal file content"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Normal upload failed: %s", rec.Body.String())
	}
	t.Log("✓ Uploaded normal file")

	// Create orphaned files (bypass the API)
	orphanPath := filepath.Join(tmpDir, "storage", "documents", "txt", "orphan.txt")
	os.MkdirAll(filepath.Dir(orphanPath), 0755)
	os.WriteFile(orphanPath, []byte("orphaned content"), 0644)
	t.Log("✓ Created orphaned file")

	// Verify to detect orphan
	verifyReq := httptest.NewRequest(http.MethodPost, "/files/duplicates/verify", nil)
	verifyRec := httptest.NewRecorder()
	srv.Router().ServeHTTP(verifyRec, verifyReq)

	if verifyRec.Code != http.StatusOK {
		t.Fatalf("Verify failed: %s", verifyRec.Body.String())
	}

	var verifyResult duplicates.VerifyResult
	json.Unmarshal(verifyRec.Body.Bytes(), &verifyResult)
	
	t.Logf("✓ Verification results:")
	t.Logf("  - Metadata entries: %d", verifyResult.MetadataIndexCount)
	t.Logf("  - Physical files: %d", verifyResult.PhysicalFilesCount)
	t.Logf("  - Orphaned files: %d", verifyResult.OrphanedFiles)

	if verifyResult.OrphanedFiles != 1 {
		t.Errorf("Expected 1 orphaned file, got %d", verifyResult.OrphanedFiles)
	}

	// Check issues
	foundOrphan := false
	for _, issue := range verifyResult.Issues {
		if issue.Type == "orphaned_file" {
			foundOrphan = true
			t.Logf("✓ Detected orphan: %s", issue.Path)
		}
	}
	if !foundOrphan {
		t.Error("Orphaned file not reported in issues")
	}
}
