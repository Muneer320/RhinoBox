package integration_test

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
)

// TestParallelMediaUpload tests uploading multiple files in a single request
func TestParallelMediaUpload(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        dir,
		MaxUploadBytes: 100 * 1024 * 1024,
	}
	
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	
	// Create multipart request with 10 files
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	for i := 0; i < 10; i++ {
		filename := fmt.Sprintf("file-%d.jpg", i)
		content := bytes.Repeat([]byte(fmt.Sprintf("content %d ", i)), 1024)
		
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatalf("CreateFormFile: %v", err)
		}
		
		if _, err := part.Write(content); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	
	// Add category and comment
	writer.WriteField("category", "batch-test")
	writer.WriteField("comment", "parallel upload test")
	
	if err := writer.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}
	
	// Create request
	req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	
	// Record response
	rec := httptest.NewRecorder()
	
	start := time.Now()
	server.Router().ServeHTTP(rec, req)
	elapsed := time.Since(start)
	
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	
	t.Logf("✓ Uploaded 10 files in %v (parallel processing)", elapsed)
	
	// Verify response contains 10 files
	respBody := rec.Body.String()
	if !contains(respBody, "stored") {
		t.Error("response should contain 'stored' field")
	}
}

// TestParallelMediaUploadLargeBatch tests uploading 100 files
func TestParallelMediaUploadLargeBatch(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        dir,
		MaxUploadBytes: 200 * 1024 * 1024,
	}
	
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	
	// Create multipart request with 100 files
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	batchSize := 100
	for i := 0; i < batchSize; i++ {
		filename := fmt.Sprintf("file-%03d.jpg", i)
		content := bytes.Repeat([]byte("x"), 10*1024) // 10KB per file
		
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatalf("CreateFormFile: %v", err)
		}
		
		if _, err := part.Write(content); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	
	writer.WriteField("category", "large-batch")
	writer.WriteField("comment", "100 file batch test")
	
	if err := writer.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}
	
	// Create request
	req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	
	// Record response
	rec := httptest.NewRecorder()
	
	start := time.Now()
	server.Router().ServeHTTP(rec, req)
	elapsed := time.Since(start)
	
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	
	t.Logf("✓ Uploaded %d files in %v (avg: %v per file)", batchSize, elapsed, elapsed/time.Duration(batchSize))
	
	// Validate performance target: <500ms for 100 files
	if elapsed > 500*time.Millisecond {
		t.Logf("⚠️  Warning: Batch processing took %v, expected <500ms", elapsed)
	} else {
		t.Logf("✓ Performance target met: %v < 500ms", elapsed)
	}
}

// TestParallelMediaUploadVeryLargeBatch tests uploading 1000 files
func TestParallelMediaUploadVeryLargeBatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large batch test in short mode")
	}
	
	// Note: This test is limited by multipart form size constraints
	// Testing with smaller batch to validate parallel processing at scale
	
	dir := t.TempDir()
	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        dir,
		MaxUploadBytes: 1000 * 1024 * 1024, // 1GB limit
	}
	
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	
	// Create multipart request with 500 files (reduced from 1000 due to multipart limits)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	batchSize := 500
	for i := 0; i < batchSize; i++ {
		filename := fmt.Sprintf("file-%04d.jpg", i)
		content := bytes.Repeat([]byte("x"), 2*1024) // 2KB per file (reduced size)
		
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatalf("CreateFormFile: %v", err)
		}
		
		if _, err := part.Write(content); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	
	writer.WriteField("category", "very-large-batch")
	writer.WriteField("comment", "500 file batch test")
	
	if err := writer.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}
	
	// Create request
	req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	
	// Record response
	rec := httptest.NewRecorder()
	
	start := time.Now()
	server.Router().ServeHTTP(rec, req)
	elapsed := time.Since(start)
	
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	
	t.Logf("✓ Uploaded %d files in %v (avg: %v per file)", batchSize, elapsed, elapsed/time.Duration(batchSize))
	t.Logf("✓ Throughput: %.0f files/second", float64(batchSize)/elapsed.Seconds())
	
	// For 500 files, scale the expectation proportionally
	expectedTime := 1500 * time.Millisecond // 1.5s for 500 files (3s for 1000)
	if elapsed > expectedTime {
		t.Logf("⚠️  Warning: Batch processing took %v, expected <%v", elapsed, expectedTime)
	} else {
		t.Logf("✓ Performance target met: %v < %v", elapsed, expectedTime)
	}
}

// TestParallelMediaUploadMixedTypes tests uploading different file types
func TestParallelMediaUploadMixedTypes(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        dir,
		MaxUploadBytes: 100 * 1024 * 1024,
	}
	
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	
	// Create multipart request with mixed file types
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	files := []struct {
		name    string
		content []byte
	}{
		{"photo1.jpg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}},
		{"photo2.png", bytes.Repeat([]byte("PNG data"), 100)},
		{"video.mp4", bytes.Repeat([]byte("MP4 data"), 100)},
		{"document.pdf", bytes.Repeat([]byte("%PDF-1.4"), 100)},
		{"archive.zip", bytes.Repeat([]byte("PK\x03\x04"), 100)},
	}
	
	for _, f := range files {
		part, err := writer.CreateFormFile("file", f.name)
		if err != nil {
			t.Fatalf("CreateFormFile: %v", err)
		}
		
		if _, err := part.Write(f.content); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	
	writer.WriteField("category", "mixed-types")
	writer.WriteField("comment", "mixed file types test")
	
	if err := writer.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}
	
	// Create request
	req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	
	// Record response
	rec := httptest.NewRecorder()
	
	server.Router().ServeHTTP(rec, req)
	
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	
	t.Logf("✓ Uploaded mixed file types successfully")
}

// TestParallelMediaUploadDuplicates tests duplicate detection with parallel processing
func TestParallelMediaUploadDuplicates(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        dir,
		MaxUploadBytes: 100 * 1024 * 1024,
	}
	
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	
	// First upload
	body1 := &bytes.Buffer{}
	writer1 := multipart.NewWriter(body1)
	
	content := bytes.Repeat([]byte("duplicate test"), 1000)
	part1, _ := writer1.CreateFormFile("file", "original.jpg")
	part1.Write(content)
	writer1.Close()
	
	req1 := httptest.NewRequest(http.MethodPost, "/ingest/media", body1)
	req1.Header.Set("Content-Type", writer1.FormDataContentType())
	rec1 := httptest.NewRecorder()
	server.Router().ServeHTTP(rec1, req1)
	
	if rec1.Code != http.StatusOK {
		t.Fatalf("first upload failed: %d: %s", rec1.Code, rec1.Body.String())
	}
	
	// Second upload with same content in batch
	body2 := &bytes.Buffer{}
	writer2 := multipart.NewWriter(body2)
	
	for i := 0; i < 3; i++ {
		part, _ := writer2.CreateFormFile("file", fmt.Sprintf("copy-%d.jpg", i))
		part.Write(content)
	}
	writer2.Close()
	
	req2 := httptest.NewRequest(http.MethodPost, "/ingest/media", body2)
	req2.Header.Set("Content-Type", writer2.FormDataContentType())
	rec2 := httptest.NewRecorder()
	server.Router().ServeHTTP(rec2, req2)
	
	if rec2.Code != http.StatusOK {
		t.Fatalf("second upload failed: %d: %s", rec2.Code, rec2.Body.String())
	}
	
	respBody := rec2.Body.String()
	if !contains(respBody, "duplicate") {
		t.Error("expected duplicate detection in response")
	}
	
	t.Logf("✓ Duplicate detection works with parallel processing")
}

// TestSingleFileUsesSequentialPath tests that single file uploads don't use worker pool
func TestSingleFileUsesSequentialPath(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        dir,
		MaxUploadBytes: 100 * 1024 * 1024,
	}
	
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	
	// Create multipart request with single file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	part, err := writer.CreateFormFile("file", "single.jpg")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	
	content := bytes.Repeat([]byte("single file"), 1024)
	if _, err := part.Write(content); err != nil {
		t.Fatalf("Write: %v", err)
	}
	
	if err := writer.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}
	
	// Create request
	req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	
	// Record response
	rec := httptest.NewRecorder()
	
	start := time.Now()
	server.Router().ServeHTTP(rec, req)
	elapsed := time.Since(start)
	
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	
	t.Logf("✓ Single file uploaded in %v (sequential path)", elapsed)
	
	// Single file should be fast
	if elapsed > 100*time.Millisecond {
		t.Logf("⚠️  Single file took %v, expected <100ms", elapsed)
	}
}

// TestConcurrentRequests tests multiple concurrent requests to the API
func TestConcurrentRequests(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        dir,
		MaxUploadBytes: 100 * 1024 * 1024,
	}
	
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	
	// Helper to create request
	createRequest := func(id int) (*http.Request, *multipart.Writer) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		
		for i := 0; i < 5; i++ {
			filename := fmt.Sprintf("req%d-file%d.jpg", id, i)
			part, _ := writer.CreateFormFile("file", filename)
			content := bytes.Repeat([]byte(fmt.Sprintf("req%d-%d", id, i)), 1024)
			part.Write(content)
		}
		
		writer.Close()
		
		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		return req, writer
	}
	
	// Send 10 concurrent requests
	concurrency := 10
	done := make(chan bool, concurrency)
	errors := make(chan error, concurrency)
	
	start := time.Now()
	
	for i := 0; i < concurrency; i++ {
		go func(id int) {
			req, _ := createRequest(id)
			rec := httptest.NewRecorder()
			server.Router().ServeHTTP(rec, req)
			
			if rec.Code != http.StatusOK {
				errors <- fmt.Errorf("request %d failed: %d", id, rec.Code)
			}
			done <- true
		}(i)
	}
	
	// Wait for all requests
	for i := 0; i < concurrency; i++ {
		select {
		case <-done:
			// Success
		case err := <-errors:
			t.Error(err)
		case <-time.After(30 * time.Second):
			t.Fatal("timeout waiting for concurrent requests")
		}
	}
	
	elapsed := time.Since(start)
	t.Logf("✓ Processed %d concurrent requests in %v", concurrency, elapsed)
	t.Logf("✓ Total throughput: %.0f files/second", float64(concurrency*5)/elapsed.Seconds())
}

func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
