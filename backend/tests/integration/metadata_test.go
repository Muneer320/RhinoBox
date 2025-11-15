package integration

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
	"time"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
)

// TestMetadataEndToEnd tests the complete workflow of uploading a file and updating its metadata
func TestMetadataEndToEnd(t *testing.T) {
	// Setup server
	cfg := config.Config{
		Addr:           ":0",
		DataDir:        t.TempDir(),
		MaxUploadBytes: 32 * 1024 * 1024,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	// Step 1: Upload a document
	t.Log("Step 1: Upload a financial report document")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, err := writer.CreateFormFile("file", "Q4_Financial_Report.pdf")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	fileWriter.Write([]byte("PDF content: Q4 2024 Financial Report - Revenue $10M, Expenses $7M, Profit $3M"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()
	srv.Router().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("upload failed: %d: %s", resp.Code, resp.Body.String())
	}

	var uploadResp struct {
		Stored []map[string]any `json:"stored"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}

	fileID := uploadResp.Stored[0]["hash"].(string)
	t.Logf("File uploaded with ID: %s", fileID)

	// Step 2: Add initial metadata (project and author info)
	t.Log("Step 2: Add project and author metadata")
	metaReq := map[string]any{
		"action": "replace",
		"metadata": map[string]string{
			"title":       "Q4 2024 Financial Report",
			"author":      "Jane Smith",
			"department":  "Finance",
			"project":     "Q4-Review",
			"fiscal_year": "2024",
		},
	}
	updateResp := updateMetadata(t, srv, fileID, metaReq)
	
	if updateResp["metadata"].(map[string]any)["author"] != "Jane Smith" {
		t.Error("author metadata not set correctly")
	}

	// Step 3: Move document through approval workflow
	t.Log("Step 3: Document enters review workflow")
	
	// Submitted for review
	metaReq = map[string]any{
		"action": "merge",
		"metadata": map[string]string{
			"status":      "in_review",
			"reviewer":    "Bob Johnson",
			"submitted_at": time.Now().Format(time.RFC3339),
		},
	}
	updateResp = updateMetadata(t, srv, fileID, metaReq)
	
	if updateResp["metadata"].(map[string]any)["status"] != "in_review" {
		t.Error("status not updated to in_review")
	}
	if updateResp["metadata"].(map[string]any)["author"] != "Jane Smith" {
		t.Error("original author should be preserved during merge")
	}

	// Approved
	metaReq = map[string]any{
		"action": "merge",
		"metadata": map[string]string{
			"status":      "approved",
			"approved_by": "Bob Johnson",
			"approved_at": time.Now().Format(time.RFC3339),
			"review_notes": "All numbers verified, approved for distribution",
		},
	}
	updateResp = updateMetadata(t, srv, fileID, metaReq)
	
	if updateResp["metadata"].(map[string]any)["status"] != "approved" {
		t.Error("status not updated to approved")
	}

	// Step 4: Add compliance and classification metadata
	t.Log("Step 4: Add compliance metadata")
	metaReq = map[string]any{
		"action": "merge",
		"metadata": map[string]string{
			"classification":   "confidential",
			"compliance":       "SOX,GDPR",
			"retention_years":  "7",
			"access_level":     "finance-team-only",
			"encryption":       "AES-256",
			"backup_required":  "true",
		},
	}
	updateResp = updateMetadata(t, srv, fileID, metaReq)
	
	if updateResp["metadata"].(map[string]any)["classification"] != "confidential" {
		t.Error("classification not set")
	}

	// Step 5: Add tagging for search and organization
	t.Log("Step 5: Add search tags")
	metaReq = map[string]any{
		"action": "merge",
		"metadata": map[string]string{
			"tags": "finance,q4-2024,report,annual,confidential,reviewed",
			"search_keywords": "revenue,expenses,profit,financial,quarterly",
		},
	}
	updateResp = updateMetadata(t, srv, fileID, metaReq)

	metadata := updateResp["metadata"].(map[string]any)
	if metadata["tags"] != "finance,q4-2024,report,annual,confidential,reviewed" {
		t.Error("tags not set correctly")
	}

	// Step 6: Verify all metadata is present and system fields are protected
	t.Log("Step 6: Verify metadata integrity")
	
	// Check all important fields exist
	requiredFields := []string{
		"title", "author", "department", "status", "approved_by",
		"classification", "compliance", "tags", "search_keywords",
	}
	for _, field := range requiredFields {
		if _, exists := metadata[field]; !exists {
			t.Errorf("required field '%s' missing from metadata", field)
		}
	}

	// Verify system fields are immutable
	if updateResp["hash"].(string) != fileID {
		t.Error("file hash should not change")
	}
	if updateResp["original_name"].(string) != "Q4_Financial_Report.pdf" {
		t.Error("original name should not change")
	}

	// Step 7: Test protection against system field modification
	t.Log("Step 7: Test system field protection")
	metaReq = map[string]any{
		"action": "merge",
		"metadata": map[string]string{
			"hash": "malicious_hash",
		},
	}
	
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(metaReq)
	req = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/files/%s/metadata", fileID), &buf)
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	srv.Router().ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when trying to modify system field, got %d", resp.Code)
	}

	// Step 8: Archive document and clean up temporary fields
	t.Log("Step 8: Archive document")
	metaReq = map[string]any{
		"action": "merge",
		"metadata": map[string]string{
			"status": "archived",
			"archived_at": time.Now().Format(time.RFC3339),
			"archived_by": "System",
			"archive_reason": "Fiscal year completed",
		},
	}
	updateResp = updateMetadata(t, srv, fileID, metaReq)

	// Remove temporary review fields
	metaReq = map[string]any{
		"action": "remove",
		"fields": []string{"reviewer", "review_notes"},
	}
	updateResp = updateMetadata(t, srv, fileID, metaReq)
	
	metadata = updateResp["metadata"].(map[string]any)
	if _, exists := metadata["reviewer"]; exists {
		t.Error("reviewer field should be removed")
	}
	if _, exists := metadata["review_notes"]; exists {
		t.Error("review_notes field should be removed")
	}
	if metadata["status"] != "archived" {
		t.Error("status should be archived")
	}

	// Step 9: Verify audit log
	t.Log("Step 9: Verify audit log exists")
	auditPath := filepath.Join(cfg.DataDir, "metadata", "audit_log.ndjson")
	if _, err := os.Stat(auditPath); os.IsNotExist(err) {
		t.Error("audit log should exist")
	} else {
		// Read and verify audit log has entries
		data, err := os.ReadFile(auditPath)
		if err != nil {
			t.Errorf("failed to read audit log: %v", err)
		}
		if len(data) == 0 {
			t.Error("audit log should have entries")
		}
		t.Logf("Audit log size: %d bytes", len(data))
	}

	// Step 10: Test batch update scenario
	t.Log("Step 10: Test batch metadata update")
	metaReq = map[string]any{
		"action": "merge",
		"metadata": map[string]string{
			"final_status":    "completed",
			"last_modified":   time.Now().Format(time.RFC3339),
			"version":         "1.0",
			"document_id":     "FIN-2024-Q4-001",
		},
	}
	updateResp = updateMetadata(t, srv, fileID, metaReq)
	
	metadata = updateResp["metadata"].(map[string]any)
	if len(metadata) < 10 {
		t.Errorf("expected at least 10 metadata fields, got %d", len(metadata))
	}

	t.Log("✓ All metadata operations completed successfully")
}

// TestMetadataMultiFileOrganization tests organizing multiple files with metadata
func TestMetadataMultiFileOrganization(t *testing.T) {
	// Setup
	cfg := config.Config{
		Addr:           ":0",
		DataDir:        t.TempDir(),
		MaxUploadBytes: 32 * 1024 * 1024,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	// Upload multiple project files
	projectFiles := []struct {
		name     string
		content  string
		metadata map[string]string
	}{
		{
			name:    "design_mockup.png",
			content: "PNG image data - UI mockup",
			metadata: map[string]string{
				"project":     "ProjectX",
				"type":        "design",
				"assignee":    "Alice Designer",
				"sprint":      "Sprint-12",
				"status":      "completed",
				"tags":        "ui,mockup,frontend",
			},
		},
		{
			name:    "api_spec.json",
			content: `{"endpoints": ["/users", "/posts"]}`,
			metadata: map[string]string{
				"project":     "ProjectX",
				"type":        "documentation",
				"assignee":    "Bob Backend",
				"sprint":      "Sprint-12",
				"status":      "in_progress",
				"tags":        "api,backend,specification",
			},
		},
		{
			name:    "test_results.txt",
			content: "Test Suite: PASSED (100/100)",
			metadata: map[string]string{
				"project":     "ProjectX",
				"type":        "testing",
				"assignee":    "Charlie QA",
				"sprint":      "Sprint-12",
				"status":      "completed",
				"tags":        "testing,qa,automation",
			},
		},
	}

	fileIDs := make([]string, len(projectFiles))

	// Upload and tag each file
	for i, pf := range projectFiles {
		t.Logf("Processing file: %s", pf.name)
		
		// Upload
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fileWriter, _ := writer.CreateFormFile("file", pf.name)
		fileWriter.Write([]byte(pf.content))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()
		srv.Router().ServeHTTP(resp, req)

		var uploadResp struct {
			Stored []map[string]any `json:"stored"`
		}
		json.NewDecoder(resp.Body).Decode(&uploadResp)
		fileIDs[i] = uploadResp.Stored[0]["hash"].(string)

		// Add metadata
		metaReq := map[string]any{
			"action":   "replace",
			"metadata": pf.metadata,
		}
		updateMetadata(t, srv, fileIDs[i], metaReq)
	}

	// Update project status for all files
	t.Log("Updating project status for all files")
	for _, fileID := range fileIDs {
		metaReq := map[string]any{
			"action": "merge",
			"metadata": map[string]string{
				"project_status": "review",
				"review_due":     "2024-12-31",
			},
		}
		updateMetadata(t, srv, fileID, metaReq)
	}

	t.Log("✓ Multi-file organization test completed")
}

// Helper function to update metadata
func updateMetadata(t *testing.T, srv *api.Server, fileID string, payload map[string]any) map[string]any {
	t.Helper()
	
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		t.Fatalf("encode payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/files/%s/metadata", fileID), &buf)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	srv.Router().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("metadata update failed: %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return result
}
