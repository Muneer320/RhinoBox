package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUnifiedIngestMediaOnly(t *testing.T) {
	srv := newTestServer(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	fileWriter, _ := writer.CreateFormFile("files", "cat.jpg")
	fileWriter.Write([]byte("fake image data"))
	
	writer.WriteField("namespace", "animals")
	writer.WriteField("comment", "pets")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()

	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var response struct {
		Success bool                   `json:"success"`
		Data    UnifiedIngestResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Success {
		t.Fatalf("expected success=true")
	}
	result := response.Data

	if result.Status != "completed" {
		t.Errorf("expected status completed, got %s", result.Status)
	}
	if len(result.Results.Media) != 1 {
		t.Fatalf("expected 1 media result, got %d", len(result.Results.Media))
	}
	// Category may include the full path like "images/jpg/pets"
	if !strings.Contains(result.Results.Media[0].Category, "pets") {
		t.Errorf("expected category to contain 'pets', got %s", result.Results.Media[0].Category)
	}
}

func TestUnifiedIngestJSONOnly(t *testing.T) {
	srv := newTestServer(t)

	jsonData := `[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]`
	
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("namespace", "users")
	writer.WriteField("data", jsonData)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()

	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var response struct {
		Success bool                   `json:"success"`
		Data    UnifiedIngestResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Success {
		t.Fatalf("expected success=true")
	}
	result := response.Data

	if len(result.Results.JSON) != 1 {
		t.Fatalf("expected 1 JSON result, got %d", len(result.Results.JSON))
	}
	if result.Results.JSON[0].RecordsInserted != 2 {
		t.Errorf("expected 2 records inserted, got %d", result.Results.JSON[0].RecordsInserted)
	}
}

func TestUnifiedIngestMixed(t *testing.T) {
	srv := newTestServer(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	// Add media file
	fileWriter, _ := writer.CreateFormFile("files", "photo.png")
	fileWriter.Write([]byte("PNG fake"))
	
	// Add JSON data
	jsonData := `[{"product":"widget","qty":10}]`
	writer.WriteField("namespace", "inventory")
	writer.WriteField("data", jsonData)
	writer.WriteField("comment", "batch import")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()

	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var response struct {
		Success bool                   `json:"success"`
		Data    UnifiedIngestResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Success {
		t.Fatalf("expected success=true")
	}
	result := response.Data

	if len(result.Results.Media) != 1 {
		t.Errorf("expected 1 media result, got %d", len(result.Results.Media))
	}
	if len(result.Results.JSON) != 1 {
		t.Errorf("expected 1 JSON result, got %d", len(result.Results.JSON))
	}
	if result.Timing["total_ms"] == 0 {
		t.Error("expected timing metrics to be populated")
	}
}

func TestUnifiedIngestBatchMedia(t *testing.T) {
	srv := newTestServer(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	for i := 1; i <= 5; i++ {
		fw, _ := writer.CreateFormFile("files", fmt.Sprintf("img%d.jpg", i))
		fw.Write([]byte(fmt.Sprintf("image %d", i)))
	}
	
	writer.WriteField("comment", "batch test")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()

	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var response struct {
		Success bool                   `json:"success"`
		Data    UnifiedIngestResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Success {
		t.Fatalf("expected success=true")
	}
	result := response.Data

	if len(result.Results.Media) != 5 {
		t.Errorf("expected 5 media results, got %d", len(result.Results.Media))
	}
}

func TestUnifiedIngestGenericFile(t *testing.T) {
	srv := newTestServer(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	fileWriter, _ := writer.CreateFormFile("files", "document.pdf")
	fileWriter.Write([]byte("PDF content"))
	
	writer.WriteField("namespace", "docs")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()

	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var response struct {
		Success bool                   `json:"success"`
		Data    UnifiedIngestResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Success {
		t.Fatalf("expected success=true")
	}
	result := response.Data

	if len(result.Results.Files) != 1 {
		t.Fatalf("expected 1 generic file result, got %d", len(result.Results.Files))
	}
	if result.Results.Files[0].OriginalName != "document.pdf" {
		t.Errorf("expected document.pdf, got %s", result.Results.Files[0].OriginalName)
	}
}
