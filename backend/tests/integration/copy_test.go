package integration

import (
	"bytes"
	"encoding/json"
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
)

func TestFileCopyIntegration(t *testing.T) {
	// Setup test server
	tmpDir := t.TempDir()
	cfg := config.Config{
		Addr:           ":0",
		DataDir:        tmpDir,
		MaxUploadBytes: 32 * 1024 * 1024,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	// Test Scenario: Complete workflow
	// 1. Upload a document
	// 2. Create a working copy
	// 3. Create hard link references
	// 4. Create batch backups

	t.Run("upload original document", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fw, _ := writer.CreateFormFile("file", "original-report.pdf")
		fw.Write([]byte("Original quarterly report Q4 2025"))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()

		srv.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("upload failed: %d: %s", resp.Code, resp.Body.String())
		}

		var result struct {
			Stored []map[string]any `json:"stored"`
		}
		json.NewDecoder(resp.Body).Decode(&result)
		
		originalHash := result.Stored[0]["hash"].(string)
		originalPath := result.Stored[0]["path"].(string)
		
		t.Logf("✓ Uploaded original document: %s (hash: %s)", originalPath, originalHash[:12])

		// Verify file exists
		fullPath := filepath.Join(tmpDir, filepath.FromSlash(originalPath))
		if _, err := os.Stat(fullPath); err != nil {
			t.Fatalf("original file not found: %v", err)
		}

		// 2. Create working copy
		t.Run("create working copy", func(t *testing.T) {
			copyReq := map[string]any{
				"new_name":     "working-report-v1.pdf",
				"new_category": "documents/pdf/working",
				"metadata": map[string]any{
					"status": "draft",
					"editor": "john.doe",
				},
				"hard_link": false,
			}

			reqBody, _ := json.Marshal(copyReq)
			req := httptest.NewRequest(http.MethodPost, "/files/"+originalHash+"/copy", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()

			srv.Router().ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("copy failed: %d: %s", resp.Code, resp.Body.String())
			}

			var copyResult struct {
				Success bool              `json:"success"`
				Copy    map[string]any    `json:"copy"`
			}
			json.NewDecoder(resp.Body).Decode(&copyResult)

			if !copyResult.Success {
				t.Fatal("copy operation failed")
			}

			copyHash := copyResult.Copy["hash"].(string)
			copyPath := copyResult.Copy["stored_path"].(string)

			t.Logf("✓ Created working copy: %s (hash: %s)", copyPath, copyHash[:12])
		})

		// 3. Create hard link references
		t.Run("create hard link references", func(t *testing.T) {
			references := []struct {
				name     string
				category string
				metadata map[string]any
			}{
				{
					name:     "reference-finance.pdf",
					category: "documents/pdf/finance",
					metadata: map[string]any{
						"department": "finance",
						"access":     "read-only",
					},
				},
				{
					name:     "reference-executive.pdf",
					category: "documents/pdf/executive",
					metadata: map[string]any{
						"department": "executive",
						"access":     "read-only",
					},
				},
			}

			for _, ref := range references {
				copyReq := map[string]any{
					"new_name":     ref.name,
					"new_category": ref.category,
					"metadata":     ref.metadata,
					"hard_link":    true,
				}

				reqBody, _ := json.Marshal(copyReq)
				req := httptest.NewRequest(http.MethodPost, "/files/"+originalHash+"/copy", bytes.NewReader(reqBody))
				req.Header.Set("Content-Type", "application/json")
				resp := httptest.NewRecorder()

				srv.Router().ServeHTTP(resp, req)

				if resp.Code != http.StatusOK {
					t.Fatalf("hard link creation failed for %s: %d: %s", ref.name, resp.Code, resp.Body.String())
				}

				var copyResult struct {
					Success    bool           `json:"success"`
					IsHardLink bool           `json:"is_hard_link"`
					Copy       map[string]any `json:"copy"`
				}
				json.NewDecoder(resp.Body).Decode(&copyResult)

				if !copyResult.IsHardLink {
					t.Fatalf("expected hard link for %s", ref.name)
				}

				t.Logf("✓ Created hard link reference: %s", ref.name)
			}
		})

		// 4. Create batch backups
		t.Run("create batch backups", func(t *testing.T) {
			batchReq := map[string]any{
				"operations": []map[string]any{
					{
						"source_path":  originalHash,
						"new_name":     "backup-daily-2025-11-15.pdf",
						"new_category": "backups/daily",
						"metadata": map[string]any{
							"backup_type": "daily",
							"backup_date": "2025-11-15",
						},
						"hard_link": false,
					},
					{
						"source_path":  originalHash,
						"new_name":     "backup-weekly-2025-W46.pdf",
						"new_category": "backups/weekly",
						"metadata": map[string]any{
							"backup_type": "weekly",
							"backup_week": "2025-W46",
						},
						"hard_link": false,
					},
					{
						"source_path":  originalHash,
						"new_name":     "backup-monthly-2025-11.pdf",
						"new_category": "backups/monthly",
						"metadata": map[string]any{
							"backup_type": "monthly",
							"backup_month": "2025-11",
						},
						"hard_link": false,
					},
				},
			}

			reqBody, _ := json.Marshal(batchReq)
			req := httptest.NewRequest(http.MethodPost, "/files/copy/batch", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()

			srv.Router().ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("batch copy failed: %d: %s", resp.Code, resp.Body.String())
			}

			var batchResult struct {
				Total      int              `json:"total"`
				Successful int              `json:"successful"`
				Failed     int              `json:"failed"`
				Results    []map[string]any `json:"results"`
			}
			json.NewDecoder(resp.Body).Decode(&batchResult)

			if batchResult.Total != 3 {
				t.Fatalf("expected 3 operations, got %d", batchResult.Total)
			}

			if batchResult.Successful != 3 {
				t.Fatalf("expected 3 successful operations, got %d", batchResult.Successful)
			}

			for _, result := range batchResult.Results {
				success := result["success"].(bool)
				if !success {
					t.Fatalf("backup operation failed: %v", result["error"])
				}
			}

			t.Logf("✓ Created 3 backups (daily, weekly, monthly)")
		})

		// Verify all copies exist in storage
		t.Run("verify storage structure", func(t *testing.T) {
			// Check metadata file exists and is valid
			metadataPath := filepath.Join(tmpDir, "metadata", "files.json")
			if _, err := os.Stat(metadataPath); err != nil {
				t.Fatalf("metadata file not found: %v", err)
			}

			// Read and parse metadata
			data, _ := os.ReadFile(metadataPath)
			var metadata []map[string]any
			if err := json.Unmarshal(data, &metadata); err != nil {
				t.Fatalf("invalid metadata JSON: %v", err)
			}

			// We should have multiple entries (original + copies)
			if len(metadata) < 5 {
				t.Fatalf("expected at least 5 metadata entries, got %d", len(metadata))
			}

			t.Logf("✓ Verified storage structure: %d total file entries", len(metadata))
		})

		t.Logf("✓ Integration test complete: end-to-end file copy workflow successful")
	})
}

func TestFileCopyRealWorldData(t *testing.T) {
	// Setup test server
	tmpDir := t.TempDir()
	cfg := config.Config{
		Addr:           ":0",
		DataDir:        tmpDir,
		MaxUploadBytes: 32 * 1024 * 1024,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	// Test with realistic file sizes and content
	testCases := []struct {
		name        string
		content     []byte
		filename    string
		mimeType    string
		description string
	}{
		{
			name:        "small_text",
			content:     []byte("Lorem ipsum dolor sit amet"),
			filename:    "note.txt",
			mimeType:    "text/plain",
			description: "Small text file",
		},
		{
			name:        "medium_document",
			content:     bytes.Repeat([]byte("Document content. "), 1000),
			filename:    "report.pdf",
			mimeType:    "application/pdf",
			description: "Medium-sized document",
		},
		{
			name:        "large_image",
			content:     bytes.Repeat([]byte{0xFF, 0xD8, 0xFF, 0xE0}, 50000),
			filename:    "photo.jpg",
			mimeType:    "image/jpeg",
			description: "Large image file",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Upload file
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			fw, _ := writer.CreateFormFile("file", tc.filename)
			fw.Write(tc.content)
			writer.Close()

			req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			resp := httptest.NewRecorder()

			srv.Router().ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("upload failed: %d", resp.Code)
			}

			var result struct {
				Stored []map[string]any `json:"stored"`
			}
			json.NewDecoder(resp.Body).Decode(&result)
			fileHash := result.Stored[0]["hash"].(string)

			// Create copy
			copyReq := map[string]any{
				"new_name":  "copy-" + tc.filename,
				"hard_link": false,
			}

			reqBody, _ := json.Marshal(copyReq)
			req = httptest.NewRequest(http.MethodPost, "/files/"+fileHash+"/copy", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			resp = httptest.NewRecorder()

			srv.Router().ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("copy failed: %d: %s", resp.Code, resp.Body.String())
			}

			t.Logf("✓ %s (%d bytes): uploaded and copied successfully", tc.description, len(tc.content))
		})
	}
}
