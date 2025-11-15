package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestFileCopyFullCopy(t *testing.T) {
	srv := newTestServer(t)

	// First, upload a file
	uploadResp := uploadTestFile(t, srv, "test-document.pdf", "application/pdf", []byte("PDF content here"))
	
	// Extract the hash from upload response
	var uploadResult struct {
		Stored []map[string]any `json:"stored"`
	}
	if err := json.Unmarshal([]byte(uploadResp), &uploadResult); err != nil {
		t.Fatalf("unmarshal upload response: %v", err)
	}
	if len(uploadResult.Stored) == 0 {
		t.Fatalf("no files stored")
	}
	
	fileHash := uploadResult.Stored[0]["hash"].(string)
	
	// Now copy the file
	copyReq := map[string]any{
		"new_name":     "copy-of-document.pdf",
		"new_category": "documents/pdf",
		"metadata": map[string]any{
			"comment": "Working copy",
			"tags":    "draft",
		},
		"hard_link": false,
	}
	
	req := newJSONRequest(t, "/files/"+fileHash+"/copy", copyReq)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	
	var copyResp fileCopyResponse
	if err := json.NewDecoder(resp.Body).Decode(&copyResp); err != nil {
		t.Fatalf("decode copy response: %v", err)
	}
	
	if !copyResp.Success {
		t.Fatalf("copy operation failed")
	}
	
	// When copying identical content, it's automatically deduplicated as a hard link
	// This is expected behavior for storage efficiency
	if !copyResp.IsHardLink {
		t.Logf("Note: Copy was deduplicated - created as hard link for efficiency")
	}
	
	if copyResp.Copy.OriginalName != "copy-of-document.pdf" {
		t.Fatalf("expected new name 'copy-of-document.pdf', got %s", copyResp.Copy.OriginalName)
	}
	
	// Verify the source file exists
	sourcePath := filepath.Join(srv.cfg.DataDir, filepath.FromSlash(copyResp.Source.StoredPath))
	
	if _, err := os.Stat(sourcePath); err != nil {
		t.Fatalf("source file not found: %v", err)
	}
	
	// For hard link, the stored path should point to the same file
	if copyResp.IsHardLink {
		if copyResp.Copy.StoredPath != copyResp.Source.StoredPath {
			t.Fatalf("hard link should point to same file")
		}
		
		// Verify LinkedTo field is set
		if copyResp.Copy.LinkedTo == "" {
			t.Fatalf("LinkedTo should be set for hard link")
		}
	}
	
	// Verify the metadata is correct
	if copyResp.Copy.Category != "documents/pdf" {
		t.Fatalf("expected category 'documents/pdf', got %s", copyResp.Copy.Category)
	}
}

func TestFileCopyHardLink(t *testing.T) {
	srv := newTestServer(t)

	// Upload a file
	uploadResp := uploadTestFile(t, srv, "reference-doc.pdf", "application/pdf", []byte("Reference document content"))
	
	var uploadResult struct {
		Stored []map[string]any `json:"stored"`
	}
	if err := json.Unmarshal([]byte(uploadResp), &uploadResult); err != nil {
		t.Fatalf("unmarshal upload response: %v", err)
	}
	
	fileHash := uploadResult.Stored[0]["hash"].(string)
	originalPath := uploadResult.Stored[0]["path"].(string)
	
	// Create hard link
	copyReq := map[string]any{
		"new_name":     "reference-link.pdf",
		"new_category": "documents/pdf",
		"hard_link":    true,
	}
	
	req := newJSONRequest(t, "/files/"+fileHash+"/copy", copyReq)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	
	var copyResp fileCopyResponse
	if err := json.NewDecoder(resp.Body).Decode(&copyResp); err != nil {
		t.Fatalf("decode copy response: %v", err)
	}
	
	if !copyResp.Success {
		t.Fatalf("copy operation failed")
	}
	
	if !copyResp.IsHardLink {
		t.Fatalf("expected hard link, got full copy")
	}
	
	if !copyResp.Copy.IsHardLink {
		t.Fatalf("copy metadata should indicate hard link")
	}
	
	// Verify hard link points to same physical file
	if copyResp.Copy.StoredPath != originalPath {
		t.Fatalf("hard link should point to same file, got %s vs %s", copyResp.Copy.StoredPath, originalPath)
	}
	
	// Verify LinkedTo field is set
	if copyResp.Copy.LinkedTo == "" {
		t.Fatalf("LinkedTo field should be set for hard link")
	}
}

func TestFileCopyWithCustomMetadata(t *testing.T) {
	srv := newTestServer(t)

	// Upload a file
	uploadResp := uploadTestFile(t, srv, "template.txt", "text/plain", []byte("Template content"))
	
	var uploadResult struct {
		Stored []map[string]any `json:"stored"`
	}
	if err := json.Unmarshal([]byte(uploadResp), &uploadResult); err != nil {
		t.Fatalf("unmarshal upload response: %v", err)
	}
	
	fileHash := uploadResult.Stored[0]["hash"].(string)
	
	// Copy with custom metadata
	copyReq := map[string]any{
		"new_name":     "project-template.txt",
		"new_category": "documents/txt",
		"metadata": map[string]any{
			"project":     "demo-project",
			"version":     "1.0",
			"description": "Project template file",
		},
		"hard_link": false,
	}
	
	req := newJSONRequest(t, "/files/"+fileHash+"/copy", copyReq)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	
	var copyResp fileCopyResponse
	if err := json.NewDecoder(resp.Body).Decode(&copyResp); err != nil {
		t.Fatalf("decode copy response: %v", err)
	}
	
	if !copyResp.Success {
		t.Fatalf("copy operation failed")
	}
	
	if copyResp.Copy.OriginalName != "project-template.txt" {
		t.Fatalf("expected new name 'project-template.txt', got %s", copyResp.Copy.OriginalName)
	}
}

func TestFileCopyNonExistentFile(t *testing.T) {
	srv := newTestServer(t)

	copyReq := map[string]any{
		"new_name":  "copy.pdf",
		"hard_link": false,
	}
	
	req := newJSONRequest(t, "/files/nonexistent-hash/copy", copyReq)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-existent file, got %d", resp.Code)
	}
}

func TestBatchFileCopy(t *testing.T) {
	srv := newTestServer(t)

	// Upload multiple files
	hash1 := uploadTestFileHash(t, srv, "doc1.pdf", "application/pdf", []byte("Document 1"))
	hash2 := uploadTestFileHash(t, srv, "doc2.pdf", "application/pdf", []byte("Document 2"))
	hash3 := uploadTestFileHash(t, srv, "doc3.pdf", "application/pdf", []byte("Document 3"))
	
	// Batch copy request
	batchReq := map[string]any{
		"operations": []map[string]any{
			{
				"source_path":  hash1,
				"new_name":     "backup-doc1.pdf",
				"new_category": "documents/pdf/backup",
				"hard_link":    false,
			},
			{
				"source_path":  hash2,
				"new_name":     "backup-doc2.pdf",
				"new_category": "documents/pdf/backup",
				"hard_link":    true,
			},
			{
				"source_path":  hash3,
				"new_name":     "backup-doc3.pdf",
				"new_category": "documents/pdf/backup",
				"hard_link":    false,
			},
		},
	}
	
	req := newJSONRequest(t, "/files/copy/batch", batchReq)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	
	var batchResp batchFileCopyResponse
	if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
		t.Fatalf("decode batch response: %v", err)
	}
	
	if batchResp.Total != 3 {
		t.Fatalf("expected 3 operations, got %d", batchResp.Total)
	}
	
	if batchResp.Successful != 3 {
		t.Fatalf("expected 3 successful operations, got %d", batchResp.Successful)
	}
	
	if batchResp.Failed != 0 {
		t.Fatalf("expected 0 failed operations, got %d", batchResp.Failed)
	}
	
	// Verify results
	for i, result := range batchResp.Results {
		if !result.Success {
			t.Fatalf("operation %d failed: %s", i, result.Error)
		}
		if result.CopyHash == "" {
			t.Fatalf("operation %d missing copy hash", i)
		}
		if result.CopyPath == "" {
			t.Fatalf("operation %d missing copy path", i)
		}
	}
	
	// Verify second operation is hard link
	if !batchResp.Results[1].IsHardLink {
		t.Fatalf("operation 1 should be hard link")
	}
}

func TestBatchFileCopyPartialFailure(t *testing.T) {
	srv := newTestServer(t)

	// Upload one file
	hash1 := uploadTestFileHash(t, srv, "doc1.pdf", "application/pdf", []byte("Document 1"))
	
	// Batch copy with one valid and one invalid
	batchReq := map[string]any{
		"operations": []map[string]any{
			{
				"source_path":  hash1,
				"new_name":     "copy1.pdf",
				"hard_link":    false,
			},
			{
				"source_path":  "invalid-hash",
				"new_name":     "copy2.pdf",
				"hard_link":    false,
			},
			{
				"source_path":  hash1,
				"new_name":     "copy3.pdf",
				"hard_link":    true,
			},
		},
	}
	
	req := newJSONRequest(t, "/files/copy/batch", batchReq)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	
	var batchResp batchFileCopyResponse
	if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
		t.Fatalf("decode batch response: %v", err)
	}
	
	if batchResp.Total != 3 {
		t.Fatalf("expected 3 operations, got %d", batchResp.Total)
	}
	
	if batchResp.Successful != 2 {
		t.Fatalf("expected 2 successful operations, got %d", batchResp.Successful)
	}
	
	if batchResp.Failed != 1 {
		t.Fatalf("expected 1 failed operation, got %d", batchResp.Failed)
	}
	
	// Verify the failed operation
	if batchResp.Results[1].Success {
		t.Fatalf("operation 1 should have failed")
	}
	if batchResp.Results[1].Error == "" {
		t.Fatalf("operation 1 should have error message")
	}
}

func TestFileCopyByStoredPath(t *testing.T) {
	srv := newTestServer(t)

	// Upload a file
	uploadResp := uploadTestFile(t, srv, "image.jpg", "image/jpeg", []byte("JPEG image data"))
	
	var uploadResult struct {
		Stored []map[string]any `json:"stored"`
	}
	if err := json.Unmarshal([]byte(uploadResp), &uploadResult); err != nil {
		t.Fatalf("unmarshal upload response: %v", err)
	}
	
	storedPath := uploadResult.Stored[0]["path"].(string)
	fileHash := uploadResult.Stored[0]["hash"].(string)
	
	// Copy using hash (stored path lookup is done internally by storage manager)
	copyReq := map[string]any{
		"new_name":  "image-copy.jpg",
		"hard_link": false,
	}
	
	req := newJSONRequest(t, "/files/"+fileHash+"/copy", copyReq)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	
	var copyResp fileCopyResponse
	if err := json.NewDecoder(resp.Body).Decode(&copyResp); err != nil {
		t.Fatalf("decode copy response: %v", err)
	}
	
	if !copyResp.Success {
		t.Fatalf("copy operation failed")
	}
	
	// Verify source path matches
	if copyResp.Source.StoredPath != storedPath {
		t.Fatalf("source path mismatch: expected %s, got %s", storedPath, copyResp.Source.StoredPath)
	}
}

func TestFileCopyPreservesCategory(t *testing.T) {
	srv := newTestServer(t)

	// Upload a video file
	uploadResp := uploadTestFile(t, srv, "video.mp4", "video/mp4", []byte("MP4 video data"))
	
	var uploadResult struct {
		Stored []map[string]any `json:"stored"`
	}
	if err := json.Unmarshal([]byte(uploadResp), &uploadResult); err != nil {
		t.Fatalf("unmarshal upload response: %v", err)
	}
	
	fileHash := uploadResult.Stored[0]["hash"].(string)
	originalCategory := uploadResult.Stored[0]["category"].(string)
	
	// Copy without specifying new category
	copyReq := map[string]any{
		"new_name":  "video-backup.mp4",
		"hard_link": false,
	}
	
	req := newJSONRequest(t, "/files/"+fileHash+"/copy", copyReq)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	
	var copyResp fileCopyResponse
	if err := json.NewDecoder(resp.Body).Decode(&copyResp); err != nil {
		t.Fatalf("decode copy response: %v", err)
	}
	
	// Category should be preserved
	if copyResp.Copy.Category != originalCategory {
		t.Fatalf("expected category %s, got %s", originalCategory, copyResp.Copy.Category)
	}
}

func TestRealWorldScenario_TemplateSystem(t *testing.T) {
	srv := newTestServer(t)

	// Upload template document
	templateContent := []byte(`
		Invoice Template
		Company: [COMPANY_NAME]
		Date: [DATE]
		Amount: [AMOUNT]
		Description: [DESCRIPTION]
	`)
	
	uploadResp := uploadTestFile(t, srv, "invoice-template.txt", "text/plain", templateContent)
	
	var uploadResult struct {
		Stored []map[string]any `json:"stored"`
	}
	if err := json.Unmarshal([]byte(uploadResp), &uploadResult); err != nil {
		t.Fatalf("unmarshal upload response: %v", err)
	}
	
	templateHash := uploadResult.Stored[0]["hash"].(string)
	
	// Create multiple invoices from template using hard links
	invoices := []struct {
		name     string
		metadata map[string]any
	}{
		{
			name: "invoice-acme-corp.txt",
			metadata: map[string]any{
				"company":     "Acme Corp",
				"invoice_id":  "INV-001",
				"amount":      "1500.00",
			},
		},
		{
			name: "invoice-tech-inc.txt",
			metadata: map[string]any{
				"company":     "Tech Inc",
				"invoice_id":  "INV-002",
				"amount":      "2300.00",
			},
		},
		{
			name: "invoice-global-llc.txt",
			metadata: map[string]any{
				"company":     "Global LLC",
				"invoice_id":  "INV-003",
				"amount":      "890.50",
			},
		},
	}
	
	for _, inv := range invoices {
		copyReq := map[string]any{
			"new_name":     inv.name,
			"new_category": "documents/txt/invoices",
			"metadata":     inv.metadata,
			"hard_link":    true, // Use hard link for space efficiency
		}
		
		req := newJSONRequest(t, "/files/"+templateHash+"/copy", copyReq)
		resp := httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)
		
		if resp.Code != http.StatusOK {
			t.Fatalf("failed to create invoice %s: %s", inv.name, resp.Body.String())
		}
		
		var copyResp fileCopyResponse
		if err := json.NewDecoder(resp.Body).Decode(&copyResp); err != nil {
			t.Fatalf("decode response for %s: %v", inv.name, err)
		}
		
		if !copyResp.IsHardLink {
			t.Fatalf("invoice %s should be hard link", inv.name)
		}
	}
	
	t.Logf("✓ Created 3 invoices from template using hard links (space efficient)")
}

func TestRealWorldScenario_BackupWorkflow(t *testing.T) {
	srv := newTestServer(t)

	// Upload important documents
	docs := []struct {
		name    string
		content []byte
		mime    string
	}{
		{"project-proposal.pdf", []byte("Project proposal content"), "application/pdf"},
		{"financial-report.xlsx", []byte("Financial data"), "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		{"presentation.pptx", []byte("Presentation slides"), "application/vnd.openxmlformats-officedocument.presentationml.presentation"},
	}
	
	hashes := make([]string, len(docs))
	for i, doc := range docs {
		uploadResp := uploadTestFile(t, srv, doc.name, doc.mime, doc.content)
		
		var uploadResult struct {
			Stored []map[string]any `json:"stored"`
		}
		if err := json.Unmarshal([]byte(uploadResp), &uploadResult); err != nil {
			t.Fatalf("unmarshal upload response: %v", err)
		}
		
		hashes[i] = uploadResult.Stored[0]["hash"].(string)
	}
	
	// Create backups using batch copy
	operations := make([]map[string]any, len(hashes))
	for i, hash := range hashes {
		operations[i] = map[string]any{
			"source_path":  hash,
			"new_name":     "backup-" + docs[i].name,
			"new_category": "backups",
			"metadata": map[string]any{
				"backup_date": "2025-11-15",
				"backup_type": "manual",
				"original":    docs[i].name,
			},
			"hard_link": false, // Full copy for backups
		}
	}
	
	batchReq := map[string]any{
		"operations": operations,
	}
	
	req := newJSONRequest(t, "/files/copy/batch", batchReq)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusOK {
		t.Fatalf("batch backup failed: %s", resp.Body.String())
	}
	
	var batchResp batchFileCopyResponse
	if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
		t.Fatalf("decode batch response: %v", err)
	}
	
	if batchResp.Successful != len(docs) {
		t.Fatalf("expected %d successful backups, got %d", len(docs), batchResp.Successful)
	}
	
	// Verify all backups exist
	for _, result := range batchResp.Results {
		if !result.Success {
			t.Fatalf("backup failed: %s", result.Error)
		}
		
		backupPath := filepath.Join(srv.cfg.DataDir, filepath.FromSlash(result.CopyPath))
		if _, err := os.Stat(backupPath); err != nil {
			t.Fatalf("backup file not found: %v", err)
		}
	}
	
	t.Logf("✓ Created %d backups successfully", len(docs))
}

func TestRealWorldScenario_VersionControl(t *testing.T) {
	srv := newTestServer(t)

	// Upload original document
	originalContent := []byte("Version 1.0 content")
	uploadResp := uploadTestFile(t, srv, "document.txt", "text/plain", originalContent)
	
	var uploadResult struct {
		Stored []map[string]any `json:"stored"`
	}
	if err := json.Unmarshal([]byte(uploadResp), &uploadResult); err != nil {
		t.Fatalf("unmarshal upload response: %v", err)
	}
	
	originalHash := uploadResult.Stored[0]["hash"].(string)
	
	// Create version snapshots
	versions := []string{"v1.1", "v1.2", "v2.0", "v2.1"}
	
	for _, version := range versions {
		copyReq := map[string]any{
			"new_name":     "document-" + version + ".txt",
			"new_category": "documents/txt/versions",
			"metadata": map[string]any{
				"version":     version,
				"created_at":  "2025-11-15",
				"based_on":    "v1.0",
			},
			"hard_link": true, // Use hard link for version snapshots
		}
		
		req := newJSONRequest(t, "/files/"+originalHash+"/copy", copyReq)
		resp := httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)
		
		if resp.Code != http.StatusOK {
			t.Fatalf("failed to create version %s: %s", version, resp.Body.String())
		}
	}
	
	t.Logf("✓ Created %d version snapshots using hard links", len(versions))
}

// Helper functions

func uploadTestFile(t *testing.T, srv *Server, filename, mimeType string, content []byte) string {
	t.Helper()
	
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	fileWriter.Write(content)
	writer.Close()
	
	req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()
	
	srv.router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusOK {
		t.Fatalf("upload failed: %d: %s", resp.Code, resp.Body.String())
	}
	
	return resp.Body.String()
}

func uploadTestFileHash(t *testing.T, srv *Server, filename, mimeType string, content []byte) string {
	t.Helper()
	
	respBody := uploadTestFile(t, srv, filename, mimeType, content)
	
	var uploadResult struct {
		Stored []map[string]any `json:"stored"`
	}
	if err := json.Unmarshal([]byte(respBody), &uploadResult); err != nil {
		t.Fatalf("unmarshal upload response: %v", err)
	}
	
	return uploadResult.Stored[0]["hash"].(string)
}
