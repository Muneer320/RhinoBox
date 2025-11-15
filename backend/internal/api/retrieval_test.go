package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestFileDownloadByHash tests downloading a file using its hash
func TestFileDownloadByHash(t *testing.T) {
	srv := newTestServer(t)

	// First, upload a file to get its hash
	uploadResp := uploadTestFile(t, srv, "test-image.jpg", "image/jpeg", []byte("fake image data for testing"))
	hash := uploadResp["hash"].(string)

	// Now download the file by hash
	req := httptest.NewRequest(http.MethodGet, "/files/download?hash="+hash, nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	// Verify response headers
	if resp.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("expected Content-Type image/jpeg, got %s", resp.Header().Get("Content-Type"))
	}

	contentDisposition := resp.Header().Get("Content-Disposition")
	if !strings.Contains(contentDisposition, "attachment") {
		t.Errorf("expected attachment in Content-Disposition, got %s", contentDisposition)
	}
	if !strings.Contains(contentDisposition, "test-image.jpg") {
		t.Errorf("expected original filename in Content-Disposition, got %s", contentDisposition)
	}

	// Verify ETag is set
	etag := resp.Header().Get("ETag")
	if etag == "" {
		t.Error("expected ETag header to be set")
	}

	// Verify Last-Modified is set
	lastModified := resp.Header().Get("Last-Modified")
	if lastModified == "" {
		t.Error("expected Last-Modified header to be set")
	}

	// Verify Accept-Ranges is set
	acceptRanges := resp.Header().Get("Accept-Ranges")
	if acceptRanges != "bytes" {
		t.Errorf("expected Accept-Ranges: bytes, got %s", acceptRanges)
	}

	// Verify custom headers
	if resp.Header().Get("X-File-Hash") != hash {
		t.Errorf("expected X-File-Hash: %s, got %s", hash, resp.Header().Get("X-File-Hash"))
	}

	// Verify content
	body := resp.Body.Bytes()
	if string(body) != "fake image data for testing" {
		t.Errorf("expected file content, got %s", string(body))
	}

	t.Logf("✓ Downloaded file by hash: %s", hash)
}

// TestFileDownloadByPath tests downloading a file using its stored path
func TestFileDownloadByPath(t *testing.T) {
	srv := newTestServer(t)

	// Upload a file
	uploadResp := uploadTestFile(t, srv, "document.pdf", "application/pdf", []byte("fake PDF content"))
	path := uploadResp["path"].(string)

	// Download by path
	req := httptest.NewRequest(http.MethodGet, "/files/download?path="+path, nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	// Verify content type
	if resp.Header().Get("Content-Type") != "application/pdf" {
		t.Errorf("expected Content-Type application/pdf, got %s", resp.Header().Get("Content-Type"))
	}

	// Verify content
	body := resp.Body.Bytes()
	if string(body) != "fake PDF content" {
		t.Errorf("expected file content, got %s", string(body))
	}

	t.Logf("✓ Downloaded file by path: %s", path)
}

// TestFileDownloadInlineDisposition tests downloading with inline disposition
func TestFileDownloadInlineDisposition(t *testing.T) {
	srv := newTestServer(t)

	uploadResp := uploadTestFile(t, srv, "photo.png", "image/png", []byte("PNG data"))
	hash := uploadResp["hash"].(string)

	req := httptest.NewRequest(http.MethodGet, "/files/download?hash="+hash+"&disposition=inline", nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	contentDisposition := resp.Header().Get("Content-Disposition")
	if !strings.Contains(contentDisposition, "inline") {
		t.Errorf("expected inline in Content-Disposition, got %s", contentDisposition)
	}

	t.Logf("✓ Inline disposition working correctly")
}

// TestFileDownloadNotFound tests 404 response for non-existent file
func TestFileDownloadNotFound(t *testing.T) {
	srv := newTestServer(t)

	// Try to download with non-existent hash
	req := httptest.NewRequest(http.MethodGet, "/files/download?hash=nonexistenthash", nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.Code)
	}

	t.Logf("✓ 404 response for non-existent file")
}

// TestFileDownloadMissingParameter tests 400 response when no parameter provided
func TestFileDownloadMissingParameter(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/files/download", nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.Code)
	}

	t.Logf("✓ 400 response for missing parameters")
}

// TestFileMetadata tests the metadata endpoint
func TestFileMetadata(t *testing.T) {
	srv := newTestServer(t)

	uploadResp := uploadTestFile(t, srv, "data.json", "application/json", []byte(`{"key": "value"}`))
	hash := uploadResp["hash"].(string)

	req := httptest.NewRequest(http.MethodGet, "/files/metadata?hash="+hash, nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var metadata map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		t.Fatalf("failed to decode metadata: %v", err)
	}

	// Verify metadata fields
	if metadata["hash"] != hash {
		t.Errorf("expected hash %s, got %v", hash, metadata["hash"])
	}
	if metadata["original_name"] != "data.json" {
		t.Errorf("expected original_name data.json, got %v", metadata["original_name"])
	}
	if metadata["mime_type"] != "application/json" {
		t.Errorf("expected mime_type application/json, got %v", metadata["mime_type"])
	}
	if metadata["size"] == nil {
		t.Error("expected size to be set")
	}
	if metadata["uploaded_at"] == nil {
		t.Error("expected uploaded_at to be set")
	}

	t.Logf("✓ Metadata retrieved successfully: %+v", metadata)
}

// TestFileMetadataByPath tests metadata retrieval by path
func TestFileMetadataByPath(t *testing.T) {
	srv := newTestServer(t)

	uploadResp := uploadTestFile(t, srv, "video.mp4", "video/mp4", []byte("fake video data"))
	path := uploadResp["path"].(string)

	req := httptest.NewRequest(http.MethodGet, "/files/metadata?path="+path, nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var metadata map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		t.Fatalf("failed to decode metadata: %v", err)
	}

	if metadata["original_name"] != "video.mp4" {
		t.Errorf("expected original_name video.mp4, got %v", metadata["original_name"])
	}

	t.Logf("✓ Metadata by path retrieved successfully")
}

// TestFileStream tests the streaming endpoint
func TestFileStream(t *testing.T) {
	srv := newTestServer(t)

	// Create a larger file for streaming
	content := bytes.Repeat([]byte("streaming content "), 1000)
	uploadResp := uploadTestFile(t, srv, "stream.mp4", "video/mp4", content)
	hash := uploadResp["hash"].(string)

	req := httptest.NewRequest(http.MethodGet, "/files/stream?hash="+hash, nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	// Verify inline disposition for streaming
	contentDisposition := resp.Header().Get("Content-Disposition")
	if !strings.Contains(contentDisposition, "inline") {
		t.Errorf("expected inline disposition for streaming, got %s", contentDisposition)
	}

	// Verify content
	body := resp.Body.Bytes()
	if !bytes.Equal(body, content) {
		t.Errorf("expected matching content")
	}

	t.Logf("✓ File streaming working correctly")
}

// TestRangeRequest tests HTTP range requests for partial content
func TestRangeRequest(t *testing.T) {
	srv := newTestServer(t)

	content := []byte("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	uploadResp := uploadTestFile(t, srv, "range-test.bin", "application/octet-stream", content)
	hash := uploadResp["hash"].(string)

	// Request bytes 10-19
	req := httptest.NewRequest(http.MethodGet, "/files/download?hash="+hash, nil)
	req.Header.Set("Range", "bytes=10-19")
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusPartialContent {
		t.Fatalf("expected 206, got %d", resp.Code)
	}

	// Verify Content-Range header
	contentRange := resp.Header().Get("Content-Range")
	expected := "bytes 10-19/36"
	if contentRange != expected {
		t.Errorf("expected Content-Range %s, got %s", expected, contentRange)
	}

	// Verify content
	body := resp.Body.String()
	expectedContent := "ABCDEFGHIJ"
	if body != expectedContent {
		t.Errorf("expected content %s, got %s", expectedContent, body)
	}

	t.Logf("✓ Range request working correctly: %s", body)
}

// TestRangeRequestOpenEnded tests open-ended range requests
func TestRangeRequestOpenEnded(t *testing.T) {
	srv := newTestServer(t)

	content := []byte("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	uploadResp := uploadTestFile(t, srv, "range-test2.bin", "application/octet-stream", content)
	hash := uploadResp["hash"].(string)

	// Request from byte 30 to end
	req := httptest.NewRequest(http.MethodGet, "/files/download?hash="+hash, nil)
	req.Header.Set("Range", "bytes=30-")
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusPartialContent {
		t.Fatalf("expected 206, got %d", resp.Code)
	}

	// Verify Content-Range header
	contentRange := resp.Header().Get("Content-Range")
	expected := "bytes 30-35/36"
	if contentRange != expected {
		t.Errorf("expected Content-Range %s, got %s", expected, contentRange)
	}

	// Verify content (last 6 characters)
	body := resp.Body.String()
	expectedContent := "UVWXYZ"
	if body != expectedContent {
		t.Errorf("expected content %s, got %s", expectedContent, body)
	}

	t.Logf("✓ Open-ended range request working correctly")
}

// TestConditionalRequestETag tests If-None-Match conditional requests
func TestConditionalRequestETag(t *testing.T) {
	srv := newTestServer(t)

	uploadResp := uploadTestFile(t, srv, "conditional.txt", "text/plain", []byte("test content"))
	hash := uploadResp["hash"].(string)

	// First request to get ETag
	req1 := httptest.NewRequest(http.MethodGet, "/files/download?hash="+hash, nil)
	resp1 := httptest.NewRecorder()
	srv.router.ServeHTTP(resp1, req1)

	if resp1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp1.Code)
	}

	etag := resp1.Header().Get("ETag")
	if etag == "" {
		t.Fatal("ETag not set in response")
	}

	// Second request with If-None-Match
	req2 := httptest.NewRequest(http.MethodGet, "/files/download?hash="+hash, nil)
	req2.Header.Set("If-None-Match", etag)
	resp2 := httptest.NewRecorder()
	srv.router.ServeHTTP(resp2, req2)

	if resp2.Code != http.StatusNotModified {
		t.Errorf("expected 304, got %d", resp2.Code)
	}

	// Body should be empty for 304
	if resp2.Body.Len() > 0 {
		t.Error("expected empty body for 304 response")
	}

	t.Logf("✓ Conditional request with ETag working correctly")
}

// TestConditionalRequestIfModifiedSince tests If-Modified-Since conditional requests
func TestConditionalRequestIfModifiedSince(t *testing.T) {
	srv := newTestServer(t)

	uploadResp := uploadTestFile(t, srv, "modified.txt", "text/plain", []byte("content"))
	hash := uploadResp["hash"].(string)

	// First request to get Last-Modified
	req1 := httptest.NewRequest(http.MethodGet, "/files/download?hash="+hash, nil)
	resp1 := httptest.NewRecorder()
	srv.router.ServeHTTP(resp1, req1)

	if resp1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp1.Code)
	}

	lastModified := resp1.Header().Get("Last-Modified")
	if lastModified == "" {
		t.Fatal("Last-Modified not set in response")
	}

	// Second request with If-Modified-Since
	req2 := httptest.NewRequest(http.MethodGet, "/files/download?hash="+hash, nil)
	req2.Header.Set("If-Modified-Since", lastModified)
	resp2 := httptest.NewRecorder()
	srv.router.ServeHTTP(resp2, req2)

	if resp2.Code != http.StatusNotModified {
		t.Errorf("expected 304, got %d", resp2.Code)
	}

	t.Logf("✓ Conditional request with If-Modified-Since working correctly")
}

// TestDownloadLogging tests that download events are logged
func TestDownloadLogging(t *testing.T) {
	srv := newTestServer(t)

	uploadResp := uploadTestFile(t, srv, "logged.txt", "text/plain", []byte("log this"))
	hash := uploadResp["hash"].(string)

	// Download the file
	req := httptest.NewRequest(http.MethodGet, "/files/download?hash="+hash, nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	// Check that download log file was created
	logPath := filepath.Join(srv.cfg.DataDir, "downloads", "download_log.ndjson")
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("download log file not created")
	} else {
		// Read and verify log content
		data, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("failed to read log file: %v", err)
		}
		if !strings.Contains(string(data), hash) {
			t.Errorf("log file does not contain hash %s", hash)
		}
		t.Logf("✓ Download logged successfully")
	}
}

// TestMultipleFileDownloads tests downloading multiple different files
func TestMultipleFileDownloads(t *testing.T) {
	srv := newTestServer(t)

	files := []struct {
		name     string
		mimeType string
		content  []byte
	}{
		{"image1.jpg", "image/jpeg", []byte("image1 data")},
		{"image2.png", "image/png", []byte("image2 data")},
		{"video.mp4", "video/mp4", []byte("video data")},
		{"doc.pdf", "application/pdf", []byte("document data")},
		{"audio.mp3", "audio/mpeg", []byte("audio data")},
	}

	for _, f := range files {
		uploadResp := uploadTestFile(t, srv, f.name, f.mimeType, f.content)
		hash := uploadResp["hash"].(string)

		// Download by hash
		req := httptest.NewRequest(http.MethodGet, "/files/download?hash="+hash, nil)
		resp := httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Errorf("failed to download %s: %d", f.name, resp.Code)
			continue
		}

		if resp.Header().Get("Content-Type") != f.mimeType {
			t.Errorf("wrong content type for %s: expected %s, got %s", 
				f.name, f.mimeType, resp.Header().Get("Content-Type"))
		}

		if !bytes.Equal(resp.Body.Bytes(), f.content) {
			t.Errorf("wrong content for %s", f.name)
		}
	}

	t.Logf("✓ Downloaded %d different files successfully", len(files))
}

// TestLargeFileDownload tests downloading a larger file
func TestLargeFileDownload(t *testing.T) {
	srv := newTestServer(t)

	// Create a 1MB file
	size := 1024 * 1024
	content := make([]byte, size)
	for i := 0; i < size; i++ {
		content[i] = byte(i % 256)
	}

	uploadResp := uploadTestFile(t, srv, "large.bin", "application/octet-stream", content)
	hash := uploadResp["hash"].(string)

	start := time.Now()
	req := httptest.NewRequest(http.MethodGet, "/files/download?hash="+hash, nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)
	duration := time.Since(start)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	if !bytes.Equal(resp.Body.Bytes(), content) {
		t.Error("large file content mismatch")
	}

	t.Logf("✓ Downloaded 1MB file in %v", duration)
}

// TestDuplicateFileDownload tests that duplicate files can be downloaded
func TestDuplicateFileDownload(t *testing.T) {
	srv := newTestServer(t)

	content := []byte("duplicate content")
	
	// Upload same file twice
	upload1 := uploadTestFile(t, srv, "file1.txt", "text/plain", content)
	upload2 := uploadTestFile(t, srv, "file2.txt", "text/plain", content)

	hash1 := upload1["hash"].(string)
	hash2 := upload2["hash"].(string)

	// Hashes should be the same for duplicate content
	if hash1 != hash2 {
		t.Errorf("expected same hash for duplicate files, got %s and %s", hash1, hash2)
	}

	// Should be able to download using the hash
	req := httptest.NewRequest(http.MethodGet, "/files/download?hash="+hash1, nil)
	resp := httptest.NewRecorder()
	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	if !bytes.Equal(resp.Body.Bytes(), content) {
		t.Error("content mismatch for duplicate file")
	}

	t.Logf("✓ Duplicate file download working correctly")
}

// Helper function to upload a test file
func uploadTestFile(t *testing.T, srv *Server, filename, mimeType string, content []byte) map[string]any {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	// Create a form part with proper Content-Type header
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	h.Set("Content-Type", mimeType)
	
	part, err := writer.CreatePart(h)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write file content: %v", err)
	}
	
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()

	srv.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("failed to upload test file: %d - %s", resp.Code, resp.Body.String())
	}

	var result struct {
		Stored []map[string]any `json:"stored"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}

	if len(result.Stored) == 0 {
		t.Fatal("no files stored in upload response")
	}

	return result.Stored[0]
}

// TestRealWorldVideoStreaming tests streaming a real-world-like video file
func TestRealWorldVideoStreaming(t *testing.T) {
	srv := newTestServer(t)

	// Simulate a video file with realistic size (10KB for testing)
	videoSize := 10 * 1024
	videoContent := make([]byte, videoSize)
	for i := 0; i < videoSize; i++ {
		videoContent[i] = byte(i % 256)
	}

	uploadResp := uploadTestFile(t, srv, "sample-video.mp4", "video/mp4", videoContent)
	hash := uploadResp["hash"].(string)

	// Test 1: Full streaming
	req1 := httptest.NewRequest(http.MethodGet, "/files/stream?hash="+hash, nil)
	resp1 := httptest.NewRecorder()
	srv.router.ServeHTTP(resp1, req1)

	if resp1.Code != http.StatusOK {
		t.Fatalf("full stream failed: %d", resp1.Code)
	}

	if resp1.Header().Get("Content-Type") != "video/mp4" {
		t.Errorf("wrong content type for video")
	}

	// Test 2: Range request for first 1KB (simulating video player seeking)
	req2 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/files/stream?hash=%s", hash), nil)
	req2.Header.Set("Range", "bytes=0-1023")
	resp2 := httptest.NewRecorder()
	srv.router.ServeHTTP(resp2, req2)

	if resp2.Code != http.StatusPartialContent {
		t.Errorf("expected 206 for range request, got %d", resp2.Code)
	}

	if resp2.Body.Len() != 1024 {
		t.Errorf("expected 1024 bytes, got %d", resp2.Body.Len())
	}

	// Test 3: Range request for last 1KB (simulating seeking to end)
	lastByteStart := videoSize - 1024
	req3 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/files/stream?hash=%s", hash), nil)
	req3.Header.Set("Range", fmt.Sprintf("bytes=%d-", lastByteStart))
	resp3 := httptest.NewRecorder()
	srv.router.ServeHTTP(resp3, req3)

	if resp3.Code != http.StatusPartialContent {
		t.Errorf("expected 206 for end range request, got %d", resp3.Code)
	}

	if resp3.Body.Len() != 1024 {
		t.Errorf("expected 1024 bytes for end range, got %d", resp3.Body.Len())
	}

	t.Logf("✓ Real-world video streaming test passed (10KB video, multiple range requests)")
}

// TestRealWorldImageGallery tests downloading multiple images as in a gallery
func TestRealWorldImageGallery(t *testing.T) {
	srv := newTestServer(t)

	// Simulate a photo gallery with different image types
	images := []struct {
		name     string
		mimeType string
		size     int
	}{
		{"photo1.jpg", "image/jpeg", 50 * 1024},
		{"photo2.png", "image/png", 30 * 1024},
		{"photo3.webp", "image/webp", 25 * 1024},
		{"photo4.gif", "image/gif", 15 * 1024},
	}

	hashes := make([]string, 0, len(images))

	// Upload all images
	for _, img := range images {
		content := make([]byte, img.size)
		for i := 0; i < img.size; i++ {
			content[i] = byte(i % 256)
		}
		uploadResp := uploadTestFile(t, srv, img.name, img.mimeType, content)
		hashes = append(hashes, uploadResp["hash"].(string))
	}

	// Download all images (simulating gallery view)
	totalBytes := int64(0)
	start := time.Now()

	for i, hash := range hashes {
		req := httptest.NewRequest(http.MethodGet, "/files/download?hash="+hash+"&disposition=inline", nil)
		resp := httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Errorf("failed to download image %d: %d", i, resp.Code)
			continue
		}

		// Verify inline disposition for gallery viewing
		if !strings.Contains(resp.Header().Get("Content-Disposition"), "inline") {
			t.Errorf("expected inline disposition for image %d", i)
		}

		totalBytes += int64(resp.Body.Len())
	}

	duration := time.Since(start)
	throughput := float64(totalBytes) / duration.Seconds() / 1024 / 1024 // MB/s

	t.Logf("✓ Downloaded %d images (%.2f KB total) in %v (%.2f MB/s)", 
		len(images), float64(totalBytes)/1024, duration, throughput)
}

// TestRealWorldDocumentDownload tests downloading various document types
func TestRealWorldDocumentDownload(t *testing.T) {
	srv := newTestServer(t)

	documents := []struct {
		name     string
		mimeType string
		content  string
	}{
		{"report.pdf", "application/pdf", "PDF document content with lots of text and data"},
		{"spreadsheet.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "Excel data"},
		{"presentation.pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation", "PowerPoint"},
		{"document.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "Word doc"},
	}

	for _, doc := range documents {
		uploadResp := uploadTestFile(t, srv, doc.name, doc.mimeType, []byte(doc.content))
		hash := uploadResp["hash"].(string)

		// Get metadata first
		metaReq := httptest.NewRequest(http.MethodGet, "/files/metadata?hash="+hash, nil)
		metaResp := httptest.NewRecorder()
		srv.router.ServeHTTP(metaResp, metaReq)

		if metaResp.Code != http.StatusOK {
			t.Errorf("failed to get metadata for %s", doc.name)
			continue
		}

		// Download document
		dlReq := httptest.NewRequest(http.MethodGet, "/files/download?hash="+hash, nil)
		dlResp := httptest.NewRecorder()
		srv.router.ServeHTTP(dlResp, dlReq)

		if dlResp.Code != http.StatusOK {
			t.Errorf("failed to download %s: %d", doc.name, dlResp.Code)
			continue
		}

		// Verify attachment disposition for documents
		if !strings.Contains(dlResp.Header().Get("Content-Disposition"), "attachment") {
			t.Errorf("expected attachment disposition for %s", doc.name)
		}

		// Verify original filename is preserved
		if !strings.Contains(dlResp.Header().Get("Content-Disposition"), doc.name) {
			t.Errorf("original filename not in Content-Disposition for %s", doc.name)
		}

		// Verify content type
		if dlResp.Header().Get("Content-Type") != doc.mimeType {
			t.Errorf("wrong content type for %s", doc.name)
		}
	}

	t.Logf("✓ Downloaded %d different document types with correct metadata", len(documents))
}

// TestRealWorldAudioStreaming tests audio file streaming
func TestRealWorldAudioStreaming(t *testing.T) {
	srv := newTestServer(t)

	// Simulate an audio file (5KB for testing)
	audioSize := 5 * 1024
	audioContent := make([]byte, audioSize)
	for i := 0; i < audioSize; i++ {
		audioContent[i] = byte(i % 256)
	}

	uploadResp := uploadTestFile(t, srv, "song.mp3", "audio/mpeg", audioContent)
	hash := uploadResp["hash"].(string)

	// Test streaming with range requests (simulating audio player)
	rangeTests := []struct {
		rangeHeader string
		expectStart int
		expectLen   int
	}{
		{"bytes=0-1023", 0, 1024},           // First 1KB
		{"bytes=1024-2047", 1024, 1024},     // Second 1KB
		{"bytes=4096-", 4096, audioSize - 4096}, // Last portion
	}

	for _, rt := range rangeTests {
		req := httptest.NewRequest(http.MethodGet, "/files/stream?hash="+hash, nil)
		req.Header.Set("Range", rt.rangeHeader)
		resp := httptest.NewRecorder()
		srv.router.ServeHTTP(resp, req)

		if resp.Code != http.StatusPartialContent {
			t.Errorf("expected 206 for range %s, got %d", rt.rangeHeader, resp.Code)
			continue
		}

		if resp.Body.Len() != rt.expectLen {
			t.Errorf("expected %d bytes for range %s, got %d", 
				rt.expectLen, rt.rangeHeader, resp.Body.Len())
		}

		// Verify content type
		if resp.Header().Get("Content-Type") != "audio/mpeg" {
			t.Errorf("wrong content type for audio")
		}
	}

	t.Logf("✓ Audio streaming with %d range requests successful", len(rangeTests))
}

// TestConcurrentDownloads tests multiple concurrent downloads
func TestConcurrentDownloads(t *testing.T) {
	srv := newTestServer(t)

	// Upload a file
	content := []byte("concurrent download test content")
	uploadResp := uploadTestFile(t, srv, "concurrent.txt", "text/plain", content)
	hash := uploadResp["hash"].(string)

	// Perform 10 concurrent downloads
	numDownloads := 10
	done := make(chan bool, numDownloads)
	errors := make(chan error, numDownloads)

	start := time.Now()
	for i := 0; i < numDownloads; i++ {
		go func(idx int) {
			req := httptest.NewRequest(http.MethodGet, "/files/download?hash="+hash, nil)
			resp := httptest.NewRecorder()
			srv.router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				errors <- fmt.Errorf("download %d failed: %d", idx, resp.Code)
				done <- false
				return
			}

			if !bytes.Equal(resp.Body.Bytes(), content) {
				errors <- fmt.Errorf("download %d: content mismatch", idx)
				done <- false
				return
			}

			done <- true
		}(i)
	}

	// Wait for all downloads to complete
	successCount := 0
	for i := 0; i < numDownloads; i++ {
		select {
		case success := <-done:
			if success {
				successCount++
			}
		case err := <-errors:
			t.Error(err)
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for concurrent downloads")
		}
	}

	duration := time.Since(start)

	if successCount != numDownloads {
		t.Errorf("expected %d successful downloads, got %d", numDownloads, successCount)
	}

	t.Logf("✓ %d concurrent downloads completed in %v", numDownloads, duration)
}
