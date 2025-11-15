package integration_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
)

func TestMetadataUpdateEndToEnd(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Config{
		DataDir:        tmpDir,
		Addr:           ":0",
		MaxUploadBytes: 10 << 20,
	}

	srv, err := api.NewServer(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	// Step 1: Upload a file with initial metadata
	fileContent := "test file content"
	uploadResp := uploadTestFile(t, srv, "test.txt", fileContent, "initial comment")
	
	hash := uploadResp["hash"].(string)
	if hash == "" {
		t.Fatal("uploaded file has no hash")
	}

	// Step 2: Update metadata using merge action
	mergeReq := map[string]interface{}{
		"action": "merge",
		"metadata": map[string]string{
			"author":      "John Doe",
			"department":  "Engineering",
			"project":     "RhinoBox",
		},
	}

	mergeResp := updateMetadata(t, srv, hash, mergeReq, http.StatusOK)
	
	newMeta := mergeResp["new_metadata"].(map[string]interface{})
	if len(newMeta) != 4 { // initial comment + 3 new fields
		t.Errorf("expected 4 metadata fields after merge, got %d", len(newMeta))
	}

	if newMeta["comment"] != "initial comment" {
		t.Errorf("original comment should be preserved in merge")
	}

	if newMeta["author"] != "John Doe" {
		t.Errorf("author field not set correctly")
	}

	// Step 3: Update metadata using replace action
	replaceReq := map[string]interface{}{
		"action": "replace",
		"metadata": map[string]string{
			"status": "archived",
			"year":   "2024",
		},
	}

	replaceResp := updateMetadata(t, srv, hash, replaceReq, http.StatusOK)
	
	newMeta = replaceResp["new_metadata"].(map[string]interface{})
	if len(newMeta) != 2 {
		t.Errorf("expected 2 metadata fields after replace, got %d", len(newMeta))
	}

	if _, exists := newMeta["comment"]; exists {
		t.Errorf("comment should be removed after replace")
	}

	if newMeta["status"] != "archived" {
		t.Errorf("status field not set correctly")
	}

	// Step 4: Add more fields with merge
	mergeReq2 := map[string]interface{}{
		"action": "merge",
		"metadata": map[string]string{
			"tags": "important,q4-2024",
		},
	}

	mergeResp2 := updateMetadata(t, srv, hash, mergeReq2, http.StatusOK)
	
	newMeta = mergeResp2["new_metadata"].(map[string]interface{})
	if len(newMeta) != 3 { // status, year, tags
		t.Errorf("expected 3 metadata fields, got %d", len(newMeta))
	}

	// Step 5: Remove specific fields
	removeReq := map[string]interface{}{
		"action": "remove",
		"fields": []string{"year"},
	}

	removeResp := updateMetadata(t, srv, hash, removeReq, http.StatusOK)
	
	newMeta = removeResp["new_metadata"].(map[string]interface{})
	if len(newMeta) != 2 { // status, tags
		t.Errorf("expected 2 metadata fields after remove, got %d", len(newMeta))
	}

	if _, exists := newMeta["year"]; exists {
		t.Errorf("year should be removed")
	}
}

func TestMetadataUpdateValidation(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Config{
		DataDir:        tmpDir,
		Addr:           ":0",
		MaxUploadBytes: 10 << 20,
	}

	srv, err := api.NewServer(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	// Upload a test file
	uploadResp := uploadTestFile(t, srv, "test.txt", "content", "")
	hash := uploadResp["hash"].(string)

	tests := []struct {
		name           string
		hash           string
		req            map[string]interface{}
		expectedStatus int
		errorContains  string
	}{
		{
			name: "protected field - hash",
			hash: hash,
			req: map[string]interface{}{
				"action":   "merge",
				"metadata": map[string]string{"hash": "newvalue"},
			},
			expectedStatus: http.StatusBadRequest,
			errorContains:  "protected",
		},
		{
			name: "protected field - size",
			hash: hash,
			req: map[string]interface{}{
				"action":   "merge",
				"metadata": map[string]string{"size": "100"},
			},
			expectedStatus: http.StatusBadRequest,
			errorContains:  "protected",
		},
		{
			name: "invalid key with spaces",
			hash: hash,
			req: map[string]interface{}{
				"action":   "merge",
				"metadata": map[string]string{"key with spaces": "value"},
			},
			expectedStatus: http.StatusBadRequest,
			errorContains:  "invalid metadata key",
		},
		{
			name: "nonexistent file",
			hash: "nonexistent123",
			req: map[string]interface{}{
				"action":   "merge",
				"metadata": map[string]string{"key": "value"},
			},
			expectedStatus: http.StatusNotFound,
			errorContains:  "not found",
		},
		{
			name: "remove protected field",
			hash: hash,
			req: map[string]interface{}{
				"action": "remove",
				"fields": []string{"uploaded_at"},
			},
			expectedStatus: http.StatusBadRequest,
			errorContains:  "protected",
		},
		{
			name: "invalid action",
			hash: hash,
			req: map[string]interface{}{
				"action":   "invalid_action",
				"metadata": map[string]string{"key": "value"},
			},
			expectedStatus: http.StatusBadRequest,
			errorContains:  "invalid action",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := updateMetadata(t, srv, tt.hash, tt.req, tt.expectedStatus)
			
			if tt.expectedStatus != http.StatusOK {
				errorMsg, ok := resp["error"].(string)
				if !ok {
					t.Fatalf("expected error message in response")
				}
				
				if !strings.Contains(strings.ToLower(errorMsg), strings.ToLower(tt.errorContains)) {
					t.Errorf("error message %q should contain %q", errorMsg, tt.errorContains)
				}
			}
		})
	}
}

func TestBatchMetadataUpdateEndToEnd(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Config{
		DataDir:        tmpDir,
		Addr:           ":0",
		MaxUploadBytes: 10 << 20,
	}

	srv, err := api.NewServer(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	// Upload multiple files
	hashes := make([]string, 3)
	for i := 0; i < 3; i++ {
		uploadResp := uploadTestFile(t, srv, "test.txt", "content", "")
		hashes[i] = uploadResp["hash"].(string)
	}

	// Batch update
	batchReq := map[string]interface{}{
		"updates": []map[string]interface{}{
			{
				"hash":   hashes[0],
				"action": "merge",
				"metadata": map[string]string{
					"batch_field": "value1",
				},
			},
			{
				"hash":   hashes[1],
				"action": "merge",
				"metadata": map[string]string{
					"batch_field": "value2",
				},
			},
			{
				"hash":   "nonexistent",
				"action": "merge",
				"metadata": map[string]string{
					"batch_field": "value3",
				},
			},
		},
	}

	body, _ := json.Marshal(batchReq)
	req := httptest.NewRequest("POST", "/files/metadata/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("batch update failed with status %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response error: %v", err)
	}

	successCount := int(resp["success_count"].(float64))
	failureCount := int(resp["failure_count"].(float64))

	if successCount != 2 {
		t.Errorf("expected 2 successes, got %d", successCount)
	}

	if failureCount != 1 {
		t.Errorf("expected 1 failure, got %d", failureCount)
	}

	results := resp["results"].([]interface{})
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Check first two succeeded
	for i := 0; i < 2; i++ {
		result := results[i].(map[string]interface{})
		if !result["success"].(bool) {
			t.Errorf("result %d should succeed", i)
		}
	}

	// Check third failed
	result := results[2].(map[string]interface{})
	if result["success"].(bool) {
		t.Errorf("result 2 should fail")
	}
}

func TestMetadataUpdateDefaultAction(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Config{
		DataDir:        tmpDir,
		Addr:           ":0",
		MaxUploadBytes: 10 << 20,
	}

	srv, err := api.NewServer(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	// Upload a file
	uploadResp := uploadTestFile(t, srv, "test.txt", "content", "original")
	hash := uploadResp["hash"].(string)

	// Update without specifying action (should default to merge)
	req := map[string]interface{}{
		"metadata": map[string]string{
			"new_field": "new_value",
		},
	}

	resp := updateMetadata(t, srv, hash, req, http.StatusOK)
	
	newMeta := resp["new_metadata"].(map[string]interface{})
	
	// Should have both original comment and new field
	if len(newMeta) != 2 {
		t.Errorf("expected 2 metadata fields (default merge), got %d", len(newMeta))
	}

	if newMeta["comment"] != "original" {
		t.Errorf("original comment should be preserved with default merge action")
	}

	if newMeta["new_field"] != "new_value" {
		t.Errorf("new field should be added")
	}
}

func TestMetadataUpdateLargePayload(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Config{
		DataDir:        tmpDir,
		Addr:           ":0",
		MaxUploadBytes: 10 << 20,
	}

	srv, err := api.NewServer(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	// Upload a file
	uploadResp := uploadTestFile(t, srv, "test.txt", "content", "")
	hash := uploadResp["hash"].(string)

	// Try to update with oversized metadata
	req := map[string]interface{}{
		"action": "merge",
		"metadata": map[string]string{
			"large_field": strings.Repeat("a", 33*1024), // 33KB > 32KB limit
		},
	}

	resp := updateMetadata(t, srv, hash, req, http.StatusBadRequest)
	
	errorMsg := resp["error"].(string)
	if !strings.Contains(strings.ToLower(errorMsg), "too large") {
		t.Errorf("error should mention size limit, got: %s", errorMsg)
	}
}

func TestMetadataUpdateSpecialCharacters(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Config{
		DataDir:        tmpDir,
		Addr:           ":0",
		MaxUploadBytes: 10 << 20,
	}

	srv, err := api.NewServer(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	// Upload a file
	uploadResp := uploadTestFile(t, srv, "test.txt", "content", "")
	hash := uploadResp["hash"].(string)

	// Update with valid special characters in keys
	req := map[string]interface{}{
		"action": "merge",
		"metadata": map[string]string{
			"key_with_underscore": "value1",
			"key-with-dash":       "value2",
			"key.with.dots":       "value3",
			"MixedCaseKey123":     "value4",
		},
	}

	resp := updateMetadata(t, srv, hash, req, http.StatusOK)
	
	newMeta := resp["new_metadata"].(map[string]interface{})
	if len(newMeta) != 4 {
		t.Errorf("expected 4 metadata fields, got %d", len(newMeta))
	}

	// All keys should be present
	expectedKeys := []string{"key_with_underscore", "key-with-dash", "key.with.dots", "MixedCaseKey123"}
	for _, key := range expectedKeys {
		if _, exists := newMeta[key]; !exists {
			t.Errorf("key %s should exist", key)
		}
	}
}

// Helper functions

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func uploadTestFile(t *testing.T, srv *api.Server, filename, content, comment string) map[string]interface{} {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("files", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("write content: %v", err)
	}
	if comment != "" {
		if err := writer.WriteField("comment", comment); err != nil {
			t.Fatalf("write comment: %v", err)
		}
	}
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("upload failed: status %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	stored, ok := resp["stored"].([]interface{})
	if !ok || len(stored) == 0 {
		t.Fatalf("missing stored files in response")
	}

	fileInfo := stored[0].(map[string]interface{})
	return fileInfo
}

func updateMetadata(t *testing.T, srv *api.Server, hash string, reqBody map[string]interface{}, expectedStatus int) map[string]interface{} {
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request error: %v", err)
	}

	req := httptest.NewRequest("PATCH", "/files/"+hash+"/metadata", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d: %s", expectedStatus, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response error: %v", err)
	}

	return resp
}
