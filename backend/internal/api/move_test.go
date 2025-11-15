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
	"testing"
)

func TestMoveFileEndpoint(t *testing.T) {
	srv := newTestServer(t)

	// First, upload a test file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, _ := writer.CreateFormFile("file", "vacation.jpg")
	fileWriter.Write([]byte("beach photo"))
	writer.WriteField("category", "photos")
	writer.WriteField("comment", "summer vacation")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("upload failed: %d %s", resp.Code, resp.Body.String())
	}

	var uploadResp struct {
		Stored []map[string]any `json:"stored"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}

	fileHash := uploadResp.Stored[0]["hash"].(string)
	originalPath := uploadResp.Stored[0]["path"].(string)
	originalCategory := uploadResp.Stored[0]["category"].(string)

	// Now move the file
	moveReq := map[string]any{
		"new_category": "images/jpg/vacation/2025",
		"reason":       "better organization",
	}
	moveBody, _ := json.Marshal(moveReq)

	req = httptest.NewRequest(http.MethodPatch, "/files/"+fileHash+"/move", bytes.NewReader(moveBody))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("move failed: %d %s", resp.Code, resp.Body.String())
	}

	var moveResp map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&moveResp); err != nil {
		t.Fatalf("decode move response: %v", err)
	}

	// Verify response
	if moveResp["status"] != "success" {
		t.Errorf("expected status success, got %v", moveResp["status"])
	}
	if moveResp["old_path"] != originalPath {
		t.Errorf("expected old_path %s, got %v", originalPath, moveResp["old_path"])
	}
	if moveResp["old_category"] != originalCategory {
		t.Errorf("expected old_category %s, got %v", originalCategory, moveResp["old_category"])
	}
	if moveResp["new_category"] != "images/jpg/vacation/2025" {
		t.Errorf("expected new_category images/jpg/vacation/2025, got %v", moveResp["new_category"])
	}

	// Verify file exists at new location
	newPath := moveResp["new_path"].(string)
	absPath := filepath.Join(srv.cfg.DataDir, filepath.FromSlash(newPath))
	if _, err := os.Stat(absPath); err != nil {
		t.Errorf("moved file not found: %v", err)
	}

	// Verify old location is empty
	oldAbsPath := filepath.Join(srv.cfg.DataDir, filepath.FromSlash(originalPath))
	if _, err := os.Stat(oldAbsPath); !os.IsNotExist(err) {
		t.Error("file still exists at old location")
	}

	// Verify move log was created
	logPath := filepath.Join(srv.cfg.DataDir, "media", "move_log.ndjson")
	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("move log not created: %v", err)
	}
}

func TestMoveFileEndpointByPath(t *testing.T) {
	srv := newTestServer(t)

	// Upload a test file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, _ := writer.CreateFormFile("file", "report.pdf")
	fileWriter.Write([]byte("quarterly report"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("upload failed: %d", resp.Code)
	}

	var uploadResp struct {
		Stored []map[string]any `json:"stored"`
	}
	json.NewDecoder(resp.Body).Decode(&uploadResp)
	fileHash := uploadResp.Stored[0]["hash"].(string)

	// Move by hash (not by path, since paths contain slashes that complicate URL routing)
	moveReq := map[string]any{
		"new_category": "documents/pdf/reports/2025/q4",
		"reason":       "quarterly archival",
	}
	moveBody, _ := json.Marshal(moveReq)

	req = httptest.NewRequest(http.MethodPatch, "/files/"+fileHash+"/move", bytes.NewReader(moveBody))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("move by hash failed: %d %s", resp.Code, resp.Body.String())
	}

	var moveResp map[string]any
	json.NewDecoder(resp.Body).Decode(&moveResp)

	if moveResp["new_category"] != "documents/pdf/reports/2025/q4" {
		t.Errorf("expected new_category documents/pdf/reports/2025/q4, got %v", moveResp["new_category"])
	}
}

func TestMoveFileEndpointNotFound(t *testing.T) {
	srv := newTestServer(t)

	moveReq := map[string]any{
		"new_category": "images/png",
		"reason":       "test",
	}
	moveBody, _ := json.Marshal(moveReq)

	req := httptest.NewRequest(http.MethodPatch, "/files/nonexistent/move", bytes.NewReader(moveBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.Code)
	}

	var errResp map[string]any
	json.NewDecoder(resp.Body).Decode(&errResp)
	
	if errResp["error"] == nil {
		t.Error("expected error in response")
	}
}

func TestMoveFileEndpointInvalidRequest(t *testing.T) {
	srv := newTestServer(t)

	// Missing new_category
	moveReq := map[string]any{
		"reason": "test",
	}
	moveBody, _ := json.Marshal(moveReq)

	req := httptest.NewRequest(http.MethodPatch, "/files/somehash/move", bytes.NewReader(moveBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.Code)
	}
}

func TestBatchMoveFilesEndpoint(t *testing.T) {
	srv := newTestServer(t)

	// Upload multiple files
	fileHashes := make([]string, 3)
	for i := 0; i < 3; i++ {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fileWriter, _ := writer.CreateFormFile("file", fmt.Sprintf("file%d.jpg", i))
		fileWriter.Write([]byte(fmt.Sprintf("content %d", i)))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)

		var uploadResp struct {
			Stored []map[string]any `json:"stored"`
		}
		json.NewDecoder(resp.Body).Decode(&uploadResp)
		fileHashes[i] = uploadResp.Stored[0]["hash"].(string)
	}

	// Batch move all files
	batchReq := map[string]any{
		"files": []map[string]any{
			{"hash": fileHashes[0], "new_category": "images/jpg/batch", "reason": "batch test"},
			{"hash": fileHashes[1], "new_category": "images/jpg/batch", "reason": "batch test"},
			{"hash": fileHashes[2], "new_category": "images/jpg/batch", "reason": "batch test"},
		},
	}
	batchBody, _ := json.Marshal(batchReq)

	req := httptest.NewRequest(http.MethodPatch, "/files/batch/move", bytes.NewReader(batchBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("batch move failed: %d %s", resp.Code, resp.Body.String())
	}

	var batchResp map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
		t.Fatalf("decode batch response: %v", err)
	}

	// Verify results
	if batchResp["status"] != "success" {
		t.Errorf("expected status success, got %v", batchResp["status"])
	}
	
	success := int(batchResp["success"].(float64))
	if success != 3 {
		t.Errorf("expected 3 successful moves, got %d", success)
	}

	failed := int(batchResp["failed"].(float64))
	if failed != 0 {
		t.Errorf("expected 0 failed moves, got %d", failed)
	}

	// Verify all files moved
	results := batchResp["results"].([]any)
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	for _, r := range results {
		result := r.(map[string]any)
		if result["new_category"] != "images/jpg/batch" {
			t.Errorf("expected new_category images/jpg/batch, got %v", result["new_category"])
		}
	}
}

func TestBatchMoveFilesEndpointPartialFailure(t *testing.T) {
	srv := newTestServer(t)

	// Upload one file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, _ := writer.CreateFormFile("file", "test.jpg")
	fileWriter.Write([]byte("test content"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	var uploadResp struct {
		Stored []map[string]any `json:"stored"`
	}
	json.NewDecoder(resp.Body).Decode(&uploadResp)
	validHash := uploadResp.Stored[0]["hash"].(string)

	// Batch move with one valid and one invalid file
	batchReq := map[string]any{
		"files": []map[string]any{
			{"hash": validHash, "new_category": "images/jpg/moved", "reason": "test"},
			{"hash": "nonexistent", "new_category": "images/jpg/moved", "reason": "test"},
		},
	}
	batchBody, _ := json.Marshal(batchReq)

	req = httptest.NewRequest(http.MethodPatch, "/files/batch/move", bytes.NewReader(batchBody))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	// Should fail because batch operations are atomic
	if resp.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.Code)
	}
}

func TestBatchMoveFilesEndpointInvalidRequest(t *testing.T) {
	srv := newTestServer(t)

	// Empty files array
	batchReq := map[string]any{
		"files": []map[string]any{},
	}
	batchBody, _ := json.Marshal(batchReq)

	req := httptest.NewRequest(http.MethodPatch, "/files/batch/move", bytes.NewReader(batchBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.Code)
	}
}

func TestMoveFileEndpointRealWorldScenario(t *testing.T) {
	srv := newTestServer(t)

	// Scenario: User uploads family photos to default location
	// then reorganizes them into year-based categories

	photos := []struct {
		name     string
		content  string
		year     string
		category string
	}{
		{"beach.jpg", "family at beach", "2023", "vacation"},
		{"birthday.jpg", "birthday party", "2023", "celebrations"},
		{"hiking.jpg", "mountain trail", "2024", "vacation"},
		{"wedding.jpg", "cousin's wedding", "2024", "celebrations"},
		{"christmas.jpg", "christmas dinner", "2024", "holidays"},
	}

	// Upload all photos
	fileData := make([]map[string]string, len(photos))
	for i, photo := range photos {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fileWriter, _ := writer.CreateFormFile("file", photo.name)
		fileWriter.Write([]byte(photo.content))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)

		var uploadResp struct {
			Stored []map[string]any `json:"stored"`
		}
		json.NewDecoder(resp.Body).Decode(&uploadResp)
		fileData[i] = map[string]string{
			"hash": uploadResp.Stored[0]["hash"].(string),
			"path": uploadResp.Stored[0]["path"].(string),
			"year": photo.year,
			"cat":  photo.category,
		}
	}

	// Move each file to organized location
	for i, photo := range photos {
		moveReq := map[string]any{
			"new_category": fmt.Sprintf("images/jpg/%s/%s", photo.year, photo.category),
			"reason":       "yearly organization",
		}
		moveBody, _ := json.Marshal(moveReq)

		req := httptest.NewRequest(http.MethodPatch, "/files/"+fileData[i]["hash"]+"/move", bytes.NewReader(moveBody))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("move photo %s failed: %d %s", photo.name, resp.Code, resp.Body.String())
		}

		var moveResp map[string]any
		json.NewDecoder(resp.Body).Decode(&moveResp)

		expectedCategory := fmt.Sprintf("images/jpg/%s/%s", photo.year, photo.category)
		if moveResp["new_category"] != expectedCategory {
			t.Errorf("photo %s: expected category %s, got %v", photo.name, expectedCategory, moveResp["new_category"])
		}

		// Verify file exists at new location
		newPath := moveResp["new_path"].(string)
		absPath := filepath.Join(srv.cfg.DataDir, filepath.FromSlash(newPath))
		data, err := os.ReadFile(absPath)
		if err != nil {
			t.Errorf("photo %s not found at new location: %v", photo.name, err)
		}
		if string(data) != photo.content {
			t.Errorf("photo %s: content mismatch", photo.name)
		}
	}

	t.Logf("✓ Successfully organized %d photos into year-based categories", len(photos))
}

func TestMoveFileEndpointMixedFileTypes(t *testing.T) {
	srv := newTestServer(t)

	// Upload various file types
	files := []struct {
		name     string
		content  string
		mime     string
		targetCat string
	}{
		{"image.jpg", "photo data", "image/jpeg", "archive/images/2025"},
		{"video.mp4", "video data", "video/mp4", "archive/videos/2025"},
		{"doc.pdf", "document data", "application/pdf", "archive/documents/2025"},
		{"audio.mp3", "audio data", "audio/mpeg", "archive/audio/2025"},
	}

	for _, file := range files {
		// Upload
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fileWriter, _ := writer.CreateFormFile("file", file.name)
		fileWriter.Write([]byte(file.content))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)

		var uploadResp struct {
			Stored []map[string]any `json:"stored"`
		}
		json.NewDecoder(resp.Body).Decode(&uploadResp)
		fileHash := uploadResp.Stored[0]["hash"].(string)

		// Move to archive
		moveReq := map[string]any{
			"new_category": file.targetCat,
			"reason":       "archival by type",
		}
		moveBody, _ := json.Marshal(moveReq)

		req = httptest.NewRequest(http.MethodPatch, "/files/"+fileHash+"/move", bytes.NewReader(moveBody))
		req.Header.Set("Content-Type", "application/json")
		resp = httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("move %s failed: %d", file.name, resp.Code)
		}

		var moveResp map[string]any
		json.NewDecoder(resp.Body).Decode(&moveResp)

		if moveResp["new_category"] != file.targetCat {
			t.Errorf("%s: expected category %s, got %v", file.name, file.targetCat, moveResp["new_category"])
		}
	}

	t.Logf("✓ Successfully moved mixed file types to archive categories")
}

func TestMoveFileEndpointPreservesHash(t *testing.T) {
	srv := newTestServer(t)

	// Upload a file
	content := []byte("important data that must not change")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, _ := writer.CreateFormFile("file", "data.bin")
	fileWriter.Write(content)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	var uploadResp struct {
		Stored []map[string]any `json:"stored"`
	}
	json.NewDecoder(resp.Body).Decode(&uploadResp)
	originalHash := uploadResp.Stored[0]["hash"].(string)

	// Move the file multiple times
	categories := []string{
		"documents/bin/temp",
		"documents/bin/backup",
		"documents/bin/archive",
	}

	for _, cat := range categories {
		moveReq := map[string]any{
			"new_category": cat,
			"reason":       "testing hash preservation",
		}
		moveBody, _ := json.Marshal(moveReq)

		req = httptest.NewRequest(http.MethodPatch, "/files/"+originalHash+"/move", bytes.NewReader(moveBody))
		req.Header.Set("Content-Type", "application/json")
		resp = httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("move to %s failed: %d", cat, resp.Code)
		}

		var moveResp map[string]any
		json.NewDecoder(resp.Body).Decode(&moveResp)

		metadata := moveResp["metadata"].(map[string]any)
		currentHash := metadata["hash"].(string)
		
		if currentHash != originalHash {
			t.Errorf("hash changed after move to %s: %s -> %s", cat, originalHash, currentHash)
		}

		// Verify file content hasn't changed
		newPath := moveResp["new_path"].(string)
		absPath := filepath.Join(srv.cfg.DataDir, filepath.FromSlash(newPath))
		data, err := os.ReadFile(absPath)
		if err != nil {
			t.Fatalf("read file after move: %v", err)
		}
		if !bytes.Equal(data, content) {
			t.Error("file content changed after move")
		}
	}

	t.Logf("✓ Hash preserved through %d moves: %s", len(categories), originalHash)
}
