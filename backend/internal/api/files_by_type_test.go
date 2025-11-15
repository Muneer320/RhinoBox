package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleGetFilesByType(t *testing.T) {
	srv := newTestServer(t)

	// Store some test files
	files := []struct {
		name     string
		content  []byte
		mimeType string
	}{
		{"image1.jpg", []byte("image content 1"), "image/jpeg"},
		{"image2.png", []byte("image content 2"), "image/png"},
		{"video1.mp4", []byte("video content"), "video/mp4"},
	}

	for _, file := range files {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fileWriter, err := writer.CreateFormFile("file", file.name)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		fileWriter.Write(file.content)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("failed to store file %s: %d - %s", file.name, resp.Code, resp.Body.String())
		}
	}

	// Test: Get images
	req := httptest.NewRequest(http.MethodGet, "/files/type/images", nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	filesArray, ok := result["files"].([]any)
	if !ok {
		t.Fatalf("expected files array, got %T", result["files"])
	}

	if len(filesArray) != 2 {
		t.Errorf("expected 2 images, got %d", len(filesArray))
	}

	// Verify response structure
	if result["type"] != "images" {
		t.Errorf("expected type 'images', got %v", result["type"])
	}

	total, ok := result["total"].(float64)
	if !ok {
		t.Fatalf("expected total number, got %T", result["total"])
	}
	if total != 2 {
		t.Errorf("expected total 2, got %v", total)
	}
}

func TestHandleGetFilesByType_Pagination(t *testing.T) {
	srv := newTestServer(t)

	// Store 5 images with unique content to avoid deduplication
	for i := 0; i < 5; i++ {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fileWriter, err := writer.CreateFormFile("file", fmt.Sprintf("image_%d.jpg", i))
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		fileWriter.Write([]byte(fmt.Sprintf("image content %d", i)))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("failed to store file: %d", resp.Code)
		}
	}

	// Test: First page with limit
	req := httptest.NewRequest(http.MethodGet, "/files/type/images?page=1&limit=2", nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	filesArray := result["files"].([]any)
	if len(filesArray) != 2 {
		t.Errorf("expected 2 files on page 1, got %d", len(filesArray))
	}

	page, ok := result["page"].(float64)
	if !ok || page != 1 {
		t.Errorf("expected page 1, got %v", result["page"])
	}

	limit, ok := result["limit"].(float64)
	if !ok || limit != 2 {
		t.Errorf("expected limit 2, got %v", result["limit"])
	}

	totalPages, ok := result["total_pages"].(float64)
	if !ok || totalPages != 3 {
		t.Errorf("expected 3 total pages, got %v", result["total_pages"])
	}

	// Test: Second page
	req = httptest.NewRequest(http.MethodGet, "/files/type/images?page=2&limit=2", nil)
	resp = httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	filesArray = result["files"].([]any)
	if len(filesArray) != 2 {
		t.Errorf("expected 2 files on page 2, got %d", len(filesArray))
	}
}

func TestHandleGetFilesByType_CategoryFilter(t *testing.T) {
	srv := newTestServer(t)

	// Store images with different categories and unique content
	categories := []string{"vacation", "vacation", "work"}
	for i, cat := range categories {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fileWriter, err := writer.CreateFormFile("file", fmt.Sprintf("image_%d.jpg", i))
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		fileWriter.Write([]byte(fmt.Sprintf("image content %d", i)))
		writer.WriteField("category", cat)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("failed to store file: %d", resp.Code)
		}
	}

	// Test: Filter by category
	req := httptest.NewRequest(http.MethodGet, "/files/type/images?category=vacation", nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	filesArray := result["files"].([]any)
	if len(filesArray) != 2 {
		t.Errorf("expected 2 files with vacation category, got %d", len(filesArray))
	}
}

func TestHandleGetFilesByType_EmptyType(t *testing.T) {
	srv := newTestServer(t)

	// Test: Empty type parameter (should be handled by router, but test anyway)
	req := httptest.NewRequest(http.MethodGet, "/files/type/", nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	// Router should handle this, but we test the endpoint behavior
	if resp.Code == http.StatusOK {
		// If it matches, check response
		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
			if result["error"] == nil {
				t.Error("expected error for empty type")
			}
		}
	}
}

func TestHandleGetFilesByType_NonExistentType(t *testing.T) {
	srv := newTestServer(t)

	// Test: Non-existent type should return empty result
	req := httptest.NewRequest(http.MethodGet, "/files/type/nonexistent", nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	filesArray := result["files"].([]any)
	if len(filesArray) != 0 {
		t.Errorf("expected 0 files for nonexistent type, got %d", len(filesArray))
	}

	total, ok := result["total"].(float64)
	if !ok || total != 0 {
		t.Errorf("expected total 0, got %v", result["total"])
	}
}

func TestHandleGetFilesByType_ResponseFormat(t *testing.T) {
	srv := newTestServer(t)

	// Store a test file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, err := writer.CreateFormFile("file", "test.jpg")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	fileWriter.Write([]byte("image content"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("failed to store file: %d", resp.Code)
	}

	// Get the file
	req = httptest.NewRequest(http.MethodGet, "/files/type/images", nil)
	resp = httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	filesArray := result["files"].([]any)
	if len(filesArray) == 0 {
		t.Fatal("expected at least one file")
	}

	file := filesArray[0].(map[string]any)

	// Verify required fields
	requiredFields := []string{"id", "name", "path", "size", "type", "date", "hash", "url", "downloadUrl"}
	for _, field := range requiredFields {
		if _, ok := file[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}

	// Verify field types
	if _, ok := file["id"].(string); !ok {
		t.Error("id should be a string")
	}
	if _, ok := file["name"].(string); !ok {
		t.Error("name should be a string")
	}
	if _, ok := file["size"].(float64); !ok {
		t.Error("size should be a number")
	}
}

func TestHandleGetFilesByType_InvalidPagination(t *testing.T) {
	srv := newTestServer(t)

	// Test: Invalid page number (should default to 1)
	req := httptest.NewRequest(http.MethodGet, "/files/type/images?page=-1&limit=abc", nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Should default to page 1 and limit 50
	page, ok := result["page"].(float64)
	if !ok || page != 1 {
		t.Errorf("expected page to default to 1, got %v", result["page"])
	}

	limit, ok := result["limit"].(float64)
	if !ok || limit != 50 {
		t.Errorf("expected limit to default to 50, got %v", result["limit"])
	}
}

