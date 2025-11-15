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
	"testing"
	"time"

	"log/slog"

	"github.com/Muneer320/RhinoBox/internal/config"
)

// TestMetadataUpdateReplace tests replacing all metadata
func TestMetadataUpdateReplace(t *testing.T) {
	srv, fileID := setupTestFileWithMetadata(t, map[string]string{
		"comment":    "initial comment",
		"tags":       "tag1,tag2",
		"department": "engineering",
	})

	// Replace all metadata
	req := newMetadataUpdateRequest(t, fileID, metadataUpdateRequest{
		Action: "replace",
		Metadata: map[string]string{
			"comment": "Updated financial report for Q4",
			"tags":    "important,project-x,Q4-2024",
			"author":  "John Doe",
		},
	})

	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	metadata, ok := result["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata field missing or wrong type")
	}

	// Check new metadata is present
	if metadata["comment"] != "Updated financial report for Q4" {
		t.Errorf("expected updated comment, got %v", metadata["comment"])
	}
	if metadata["author"] != "John Doe" {
		t.Errorf("expected author John Doe, got %v", metadata["author"])
	}

	// Check old metadata is gone (replace behavior)
	if _, exists := metadata["department"]; exists {
		t.Errorf("department should be removed in replace action")
	}
}

// TestMetadataUpdateMerge tests merging metadata
func TestMetadataUpdateMerge(t *testing.T) {
	srv, fileID := setupTestFileWithMetadata(t, map[string]string{
		"comment":    "initial comment",
		"department": "engineering",
		"project":    "project-alpha",
	})

	// Merge new metadata
	req := newMetadataUpdateRequest(t, fileID, metadataUpdateRequest{
		Action: "merge",
		Metadata: map[string]string{
			"comment": "updated comment",
			"tags":    "archived,2024",
			"author":  "Jane Smith",
		},
	})

	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	metadata, ok := result["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata field missing or wrong type")
	}

	// Check new and updated fields
	if metadata["comment"] != "updated comment" {
		t.Errorf("expected updated comment, got %v", metadata["comment"])
	}
	if metadata["author"] != "Jane Smith" {
		t.Errorf("expected author Jane Smith, got %v", metadata["author"])
	}
	if metadata["tags"] != "archived,2024" {
		t.Errorf("expected tags, got %v", metadata["tags"])
	}

	// Check old fields are preserved (merge behavior)
	if metadata["department"] != "engineering" {
		t.Errorf("department should be preserved in merge action, got %v", metadata["department"])
	}
	if metadata["project"] != "project-alpha" {
		t.Errorf("project should be preserved in merge action, got %v", metadata["project"])
	}
}

// TestMetadataUpdateRemove tests removing specific metadata fields
func TestMetadataUpdateRemove(t *testing.T) {
	srv, fileID := setupTestFileWithMetadata(t, map[string]string{
		"comment":     "initial comment",
		"tags":        "tag1,tag2",
		"department":  "engineering",
		"old_field":   "old_value",
		"author":      "Original Author",
	})

	// Remove specific fields
	req := newMetadataUpdateRequest(t, fileID, metadataUpdateRequest{
		Action: "remove",
		Fields: []string{"comment", "old_field"},
	})

	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	metadata, ok := result["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata field missing or wrong type")
	}

	// Check removed fields are gone
	if _, exists := metadata["comment"]; exists {
		t.Errorf("comment should be removed")
	}
	if _, exists := metadata["old_field"]; exists {
		t.Errorf("old_field should be removed")
	}

	// Check other fields are preserved
	if metadata["tags"] != "tag1,tag2" {
		t.Errorf("tags should be preserved, got %v", metadata["tags"])
	}
	if metadata["department"] != "engineering" {
		t.Errorf("department should be preserved, got %v", metadata["department"])
	}
	if metadata["author"] != "Original Author" {
		t.Errorf("author should be preserved, got %v", metadata["author"])
	}
}

// TestMetadataUpdateDefaultAction tests that default action is "replace"
func TestMetadataUpdateDefaultAction(t *testing.T) {
	srv, fileID := setupTestFileWithMetadata(t, map[string]string{
		"comment": "initial",
		"tags":    "old",
	})

	// No action specified, should default to replace
	req := newMetadataUpdateRequest(t, fileID, metadataUpdateRequest{
		Metadata: map[string]string{
			"comment": "new comment",
		},
	})

	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	metadata, ok := result["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata field missing or wrong type")
	}

	// Should have replaced (removed tags)
	if _, exists := metadata["tags"]; exists {
		t.Errorf("tags should be removed with default replace action")
	}
	if metadata["comment"] != "new comment" {
		t.Errorf("comment should be updated")
	}
}

// TestMetadataUpdateSystemFieldsImmutable tests that system fields cannot be modified
func TestMetadataUpdateSystemFieldsImmutable(t *testing.T) {
	srv, fileID := setupTestFileWithMetadata(t, map[string]string{
		"comment": "initial",
	})

	protectedFields := []string{"hash", "size", "uploaded_at", "mime_type", "original_name", "stored_path", "category"}
	for _, field := range protectedFields {
		t.Run(field, func(t *testing.T) {
			req := newMetadataUpdateRequest(t, fileID, metadataUpdateRequest{
				Action: "merge",
				Metadata: map[string]string{
					field: "hacker value",
				},
			})

			resp := httptest.NewRecorder()
			srv.router.ServeHTTP(resp, req)

			if resp.Code != http.StatusBadRequest {
				t.Errorf("expected 400 when trying to modify %s, got %d", field, resp.Code)
			}

			var errResp map[string]any
			json.NewDecoder(resp.Body).Decode(&errResp)
			if errMsg, ok := errResp["error"].(string); ok {
				if errMsg == "" || errMsg == "null" {
					t.Errorf("expected error message mentioning protected field")
				}
			}
		})
	}
}

// TestMetadataUpdateValidation tests size limits and validation
func TestMetadataUpdateValidation(t *testing.T) {
	srv, fileID := setupTestFileWithMetadata(t, map[string]string{})

	tests := []struct {
		name           string
		metadata       map[string]string
		expectError    bool
		errorSubstring string
	}{
		{
			name: "exceeds field count limit",
			metadata: func() map[string]string {
				m := make(map[string]string)
				for i := 0; i < 101; i++ {
					m[fmt.Sprintf("field_%d", i)] = "value"
				}
				return m
			}(),
			expectError:    true,
			errorSubstring: "100 fields",
		},
		{
			name: "key too long",
			metadata: map[string]string{
				string(make([]byte, 257)): "value",
			},
			expectError:    true,
			errorSubstring: "256 characters",
		},
		{
			name: "value too large",
			metadata: map[string]string{
				"bigfield": string(make([]byte, 10241)),
			},
			expectError:    true,
			errorSubstring: "10KB",
		},
		{
			name: "total size too large",
			metadata: func() map[string]string {
				m := make(map[string]string)
				// Create metadata that exceeds 100KB total
				for i := 0; i < 20; i++ {
					m[fmt.Sprintf("field_%d", i)] = string(make([]byte, 6000))
				}
				return m
			}(),
			expectError:    true,
			errorSubstring: "100KB",
		},
		{
			name: "valid metadata",
			metadata: map[string]string{
				"comment":     "This is a valid comment",
				"tags":        "important,project-x,Q4-2024",
				"description": "Financial report for Q4 with detailed analysis",
				"author":      "John Doe",
				"department":  "Finance",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newMetadataUpdateRequest(t, fileID, metadataUpdateRequest{
				Action:   "merge",
				Metadata: tt.metadata,
			})

			resp := httptest.NewRecorder()
			srv.router.ServeHTTP(resp, req)

			if tt.expectError {
				if resp.Code != http.StatusBadRequest {
					t.Errorf("expected 400, got %d", resp.Code)
				}
			} else {
				if resp.Code != http.StatusOK {
					t.Errorf("expected 200, got %d: %s", resp.Code, resp.Body.String())
				}
			}
		})
	}
}

// TestMetadataUpdateNotFound tests handling of non-existent files
func TestMetadataUpdateNotFound(t *testing.T) {
	srv := newTestServerEmpty(t)

	req := newMetadataUpdateRequest(t, "nonexistent-hash", metadataUpdateRequest{
		Action: "merge",
		Metadata: map[string]string{
			"comment": "test",
		},
	})

	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.Code)
	}
}

// TestMetadataUpdateInvalidAction tests invalid action parameter
func TestMetadataUpdateInvalidAction(t *testing.T) {
	srv, fileID := setupTestFileWithMetadata(t, map[string]string{})

	req := newMetadataUpdateRequest(t, fileID, metadataUpdateRequest{
		Action: "invalid_action",
		Metadata: map[string]string{
			"comment": "test",
		},
	})

	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.Code)
	}
}

// TestMetadataUpdateByStoredPath tests finding files by stored path
func TestMetadataUpdateByStoredPath(t *testing.T) {
	srv, hash := setupTestFileWithMetadata(t, map[string]string{
		"comment": "initial",
	})

	// Get the stored path
	fileMeta := srv.storage.Index().FindByHash(hash)
	if fileMeta == nil {
		t.Fatal("failed to find file by hash")
	}

	// For URL-safe path, we use hash as file_id since stored paths contain slashes
	// The endpoint supports both hash and stored path lookups internally,
	// but for REST API we primarily use hash as the file_id
	req := newMetadataUpdateRequest(t, hash, metadataUpdateRequest{
		Action: "merge",
		Metadata: map[string]string{
			"comment": "updated via hash lookup",
		},
	})

	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	metadata, ok := result["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata field missing")
	}

	if metadata["comment"] != "updated via hash lookup" {
		t.Errorf("expected updated comment, got %v", metadata["comment"])
	}

	// Verify that FindByStoredPath works internally
	found := srv.storage.Index().FindByStoredPath(fileMeta.StoredPath)
	if found == nil {
		t.Error("FindByStoredPath should work internally")
	}
	if found.Hash != hash {
		t.Errorf("found file hash mismatch")
	}
}

// TestMetadataUpdateRealWorldScenarios tests realistic use cases
func TestMetadataUpdateRealWorldScenarios(t *testing.T) {
	t.Run("document_workflow", func(t *testing.T) {
		srv, fileID := setupTestFileWithMetadata(t, map[string]string{
			"status": "draft",
		})

		// Scenario: Document moves through workflow stages
		stages := []struct {
			action   string
			metadata map[string]string
			expected map[string]string
		}{
			{
				action: "merge",
				metadata: map[string]string{
					"status":     "in_review",
					"reviewer":   "Alice Johnson",
					"reviewed_at": time.Now().Format(time.RFC3339),
				},
				expected: map[string]string{
					"status":     "in_review",
					"reviewer":   "Alice Johnson",
				},
			},
			{
				action: "merge",
				metadata: map[string]string{
					"status":      "approved",
					"approved_by": "Bob Smith",
					"approved_at": time.Now().Format(time.RFC3339),
				},
				expected: map[string]string{
					"status":      "approved",
					"approved_by": "Bob Smith",
					"reviewer":    "Alice Johnson", // should persist
				},
			},
			{
				action: "merge",
				metadata: map[string]string{
					"status":        "published",
					"published_by":  "Admin",
					"published_at":  time.Now().Format(time.RFC3339),
					"public_url":    "https://example.com/doc/123",
				},
				expected: map[string]string{
					"status":        "published",
					"published_by":  "Admin",
					"approved_by":   "Bob Smith", // should persist
				},
			},
		}

		for i, stage := range stages {
			req := newMetadataUpdateRequest(t, fileID, metadataUpdateRequest{
				Action:   stage.action,
				Metadata: stage.metadata,
			})

			resp := httptest.NewRecorder()
			srv.router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("stage %d: expected 200, got %d: %s", i, resp.Code, resp.Body.String())
			}

			var result map[string]any
			json.NewDecoder(resp.Body).Decode(&result)
			metadata, _ := result["metadata"].(map[string]any)

			for key, expectedVal := range stage.expected {
				if metadata[key] != expectedVal {
					t.Errorf("stage %d: expected %s=%s, got %v", i, key, expectedVal, metadata[key])
				}
			}
		}
	})

	t.Run("compliance_tagging", func(t *testing.T) {
		srv, fileID := setupTestFileWithMetadata(t, map[string]string{
			"description": "Customer financial data",
		})

		// Add compliance metadata
		req := newMetadataUpdateRequest(t, fileID, metadataUpdateRequest{
			Action: "merge",
			Metadata: map[string]string{
				"compliance_level": "PCI-DSS",
				"data_classification": "confidential",
				"retention_policy": "7years",
				"access_control": "finance-team-only",
				"encryption_status": "AES-256",
				"audit_required": "true",
			},
		})

		resp := httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}

		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		metadata, _ := result["metadata"].(map[string]any)

		if metadata["compliance_level"] != "PCI-DSS" {
			t.Errorf("compliance tagging failed")
		}
		if metadata["description"] != "Customer financial data" {
			t.Errorf("original metadata should be preserved")
		}
	})

	t.Run("collaborative_annotation", func(t *testing.T) {
		srv, fileID := setupTestFileWithMetadata(t, map[string]string{
			"uploaded_by": "User1",
		})

		// Multiple team members add annotations
		annotations := []struct {
			user     string
			metadata map[string]string
		}{
			{
				user: "Designer",
				metadata: map[string]string{
					"design_notes": "Colors need adjustment for accessibility",
					"design_status": "needs_revision",
				},
			},
			{
				user: "Developer",
				metadata: map[string]string{
					"implementation_notes": "Can be optimized further",
					"dev_status": "in_progress",
				},
			},
			{
				user: "QA",
				metadata: map[string]string{
					"qa_notes": "Tested on multiple browsers, looks good",
					"qa_status": "passed",
				},
			},
		}

		for _, ann := range annotations {
			req := newMetadataUpdateRequest(t, fileID, metadataUpdateRequest{
				Action:   "merge",
				Metadata: ann.metadata,
			})

			resp := httptest.NewRecorder()
			srv.router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("annotation by %s failed: %d", ann.user, resp.Code)
			}
		}

		// Verify all annotations are present
		fileMeta := srv.storage.Index().FindByHash(fileID)
		if fileMeta == nil {
			t.Fatal("file not found")
		}

		if fileMeta.Metadata["design_notes"] == "" {
			t.Error("design notes missing")
		}
		if fileMeta.Metadata["implementation_notes"] == "" {
			t.Error("implementation notes missing")
		}
		if fileMeta.Metadata["qa_notes"] == "" {
			t.Error("qa notes missing")
		}
	})

	t.Run("project_organization", func(t *testing.T) {
		srv, fileID := setupTestFileWithMetadata(t, map[string]string{})

		// Add comprehensive project metadata
		req := newMetadataUpdateRequest(t, fileID, metadataUpdateRequest{
			Action: "replace",
			Metadata: map[string]string{
				"project_name": "Project Phoenix",
				"project_code": "PHX-2024-001",
				"client": "Acme Corporation",
				"department": "Engineering",
				"team": "Backend Team",
				"sprint": "Sprint 12",
				"epic": "User Authentication",
				"story_id": "PHX-1234",
				"priority": "high",
				"tags": "authentication,security,backend,api",
				"owner": "john.doe@company.com",
				"assignee": "jane.smith@company.com",
				"due_date": "2024-12-31",
			},
		})

		resp := httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}

		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		metadata, _ := result["metadata"].(map[string]any)

		if metadata["project_name"] != "Project Phoenix" {
			t.Error("project metadata not properly set")
		}
		if metadata["tags"] != "authentication,security,backend,api" {
			t.Error("tags not properly set")
		}
	})

	t.Run("archival_and_cleanup", func(t *testing.T) {
		srv, fileID := setupTestFileWithMetadata(t, map[string]string{
			"status": "active",
			"tags": "project-x,Q3-2024",
			"owner": "old.owner@company.com",
			"notes": "Various notes",
			"temp_field": "temporary data",
		})

		// Archive the file and clean up temporary fields
		req := newMetadataUpdateRequest(t, fileID, metadataUpdateRequest{
			Action: "merge",
			Metadata: map[string]string{
				"status": "archived",
				"archived_at": time.Now().Format(time.RFC3339),
				"archived_by": "admin@company.com",
				"archive_reason": "Project completed",
			},
		})

		resp := httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("archive failed: %d", resp.Code)
		}

		// Remove temporary fields
		req = newMetadataUpdateRequest(t, fileID, metadataUpdateRequest{
			Action: "remove",
			Fields: []string{"temp_field", "notes"},
		})

		resp = httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("cleanup failed: %d", resp.Code)
		}

		// Verify final state
		fileMeta := srv.storage.Index().FindByHash(fileID)
		if fileMeta.Metadata["status"] != "archived" {
			t.Error("status not updated to archived")
		}
		if fileMeta.Metadata["temp_field"] != "" {
			t.Error("temporary field should be removed")
		}
		if fileMeta.Metadata["owner"] != "old.owner@company.com" {
			t.Error("owner should be preserved")
		}
	})
}

// TestMetadataUpdateAuditLog tests that changes are logged
func TestMetadataUpdateAuditLog(t *testing.T) {
	srv, fileID := setupTestFileWithMetadata(t, map[string]string{
		"comment": "initial",
	})

	req := newMetadataUpdateRequest(t, fileID, metadataUpdateRequest{
		Action: "merge",
		Metadata: map[string]string{
			"tags": "audit-test",
		},
	})

	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	// Check if audit log file exists
	auditPath := filepath.Join(srv.cfg.DataDir, "metadata", "audit_log.ndjson")
	if _, err := os.Stat(auditPath); os.IsNotExist(err) {
		t.Error("audit log file should be created")
	}
}

// Helper functions

func setupTestFileWithMetadata(t *testing.T, metadata map[string]string) (*Server, string) {
	t.Helper()
	srv := newTestServerEmpty(t)

	// Create a test file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, err := writer.CreateFormFile("file", "test_document.pdf")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	fileWriter.Write([]byte("fake pdf content for testing"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("file upload failed: %d: %s", resp.Code, resp.Body.String())
	}

	var uploadResp struct {
		Stored []map[string]any `json:"stored"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}

	hash, ok := uploadResp.Stored[0]["hash"].(string)
	if !ok {
		t.Fatalf("hash not found in upload response")
	}

	// Update with initial metadata if provided
	if len(metadata) > 0 {
		metaReq := newMetadataUpdateRequest(t, hash, metadataUpdateRequest{
			Action:   "replace",
			Metadata: metadata,
		})

		metaResp := httptest.NewRecorder()
		srv.router.ServeHTTP(metaResp, metaReq)

		if metaResp.Code != http.StatusOK {
			t.Fatalf("initial metadata setup failed: %d", metaResp.Code)
		}
	}

	return srv, hash
}

func newTestServerEmpty(t *testing.T) *Server {
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

func newMetadataUpdateRequest(t *testing.T, fileID string, payload metadataUpdateRequest) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(payload); err != nil {
		t.Fatalf("encode payload: %v", err)
	}
	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/files/%s/metadata", fileID), &buf)
	req.Header.Set("Content-Type", "application/json")
	return req
}
