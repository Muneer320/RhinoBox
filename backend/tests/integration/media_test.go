package integration_test

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

    "github.com/Muneer320/RhinoBox/internal/api"
    "github.com/Muneer320/RhinoBox/internal/config"
    "log/slog"
)

func TestMediaIngestEndToEnd(t *testing.T) {
    tmpDir := t.TempDir()
    cfg := config.Config{
        Addr:           ":8090",
        DataDir:        tmpDir,
        MaxUploadBytes: 50 * 1024 * 1024,
    }

    logger := slog.New(slog.NewTextHandler(io.Discard, nil))
    srv, err := api.NewServer(cfg, logger)
    if err != nil {
        t.Fatalf("NewServer: %v", err)
    }

    // Create test payloads
    testFiles := map[string][]byte{
        "photo.jpg":    bytes.Repeat([]byte("image_data_"), 1024),
        "video.mp4":    bytes.Repeat([]byte("video_frame_"), 2048),
        "document.pdf": bytes.Repeat([]byte("pdf_content_"), 512),
    }

    var uploadedHashes []string
    var uploadedFiles []string

    for filename, content := range testFiles {
        body := &bytes.Buffer{}
        writer := multipart.NewWriter(body)

        part, err := writer.CreateFormFile("files", filename)
        if err != nil {
            t.Fatalf("CreateFormFile: %v", err)
        }
        if _, err := part.Write(content); err != nil {
            t.Fatalf("write content: %v", err)
        }
        if err := writer.WriteField("comment", "integration test upload"); err != nil {
            t.Fatalf("write comment: %v", err)
        }
        writer.Close()

        req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
        req.Header.Set("Content-Type", writer.FormDataContentType())

        rec := httptest.NewRecorder()
        srv.Router().ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
            t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
        }

        var resp map[string]any
        if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
            t.Fatalf("parse response: %v", err)
        }

        stored, ok := resp["stored"].([]any)
        if !ok || len(stored) == 0 {
            t.Fatalf("missing stored files in response")
        }

        fileInfo := stored[0].(map[string]any)
        hash := fileInfo["hash"].(string)
        uploadedHashes = append(uploadedHashes, hash)
        uploadedFiles = append(uploadedFiles, filename)
        path := fileInfo["path"].(string)

        fullPath := filepath.Join(tmpDir, path)
        if _, err := os.Stat(fullPath); err != nil {
            t.Fatalf("file not stored at %s: %v", fullPath, err)
        }

        t.Logf("✓ Uploaded %s → %s (hash: %s)", filename, path, hash[:12])
    }

    // Test deduplication: reupload first file
    var firstFile string
    var firstContent []byte
    var firstHash string
    for i, name := range uploadedFiles {
        firstFile = name
        firstContent = testFiles[name]
        firstHash = uploadedHashes[i]
        break
    }

    body := &bytes.Buffer{}
    writer := multipart.NewWriter(body)
    part, _ := writer.CreateFormFile("files", firstFile)
    part.Write(firstContent)
    writer.Close()

    req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
    req.Header.Set("Content-Type", writer.FormDataContentType())
    rec := httptest.NewRecorder()
    srv.Router().ServeHTTP(rec, req)

    var dupResp map[string]any
    json.Unmarshal(rec.Body.Bytes(), &dupResp)
    stored := dupResp["stored"].([]any)[0].(map[string]any)

    if duplicate, ok := stored["duplicate"].(bool); !ok || !duplicate {
        t.Fatalf("expected duplicate flag for reupload")
    }
    if stored["hash"].(string) != firstHash {
        t.Fatalf("duplicate hash mismatch: got %s, want %s", stored["hash"].(string), firstHash)
    }

    t.Logf("✓ Deduplication verified")
}

func TestStorageTreeStructure(t *testing.T) {
    tmpDir := t.TempDir()
    cfg := config.Config{
        Addr:           ":8090",
        DataDir:        tmpDir,
        MaxUploadBytes: 50 * 1024 * 1024,
    }

    logger := slog.New(slog.NewTextHandler(io.Discard, nil))
    srv, err := api.NewServer(cfg, logger)
    if err != nil {
        t.Fatalf("NewServer: %v", err)
    }

    expectedDirs := []string{
        "storage/images/jpg",
        "storage/images/png",
        "storage/videos/mp4",
        "storage/documents/pdf",
        "storage/audio/mp3",
        "storage/archives/zip",
        "storage/other/unknown",
    }

    for _, dir := range expectedDirs {
        fullPath := filepath.Join(tmpDir, dir)
        if _, err := os.Stat(fullPath); os.IsNotExist(err) {
            t.Errorf("expected directory not created: %s", dir)
        }
    }

    // Upload files with various types
    testCases := []struct {
        filename string
        content  string
        mime     string
        wantDir  string
    }{
        {"pic.png", "png_data", "image/png", "storage/images/png"},
        {"clip.mp4", "video_data", "video/mp4", "storage/videos/mp4"},
        {"report.pdf", "pdf_data", "application/pdf", "storage/documents/pdf"},
    }

    for _, tc := range testCases {
        body := &bytes.Buffer{}
        writer := multipart.NewWriter(body)
        part, _ := writer.CreateFormFile("files", tc.filename)
        part.Write([]byte(tc.content))
        writer.Close()

        req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
        req.Header.Set("Content-Type", writer.FormDataContentType())
        rec := httptest.NewRecorder()
        srv.Router().ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
            t.Fatalf("upload %s failed: %s", tc.filename, rec.Body.String())
        }

        var resp map[string]any
        json.Unmarshal(rec.Body.Bytes(), &resp)
        stored := resp["stored"].([]any)[0].(map[string]any)
        path := stored["path"].(string)

        if !containsPath(path, tc.wantDir) {
            t.Errorf("file %s stored at %s, expected under %s", tc.filename, path, tc.wantDir)
        }
    }

    t.Logf("✓ Storage tree structure validated")
}

func TestRealWorldFiles(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping real-world file test in short mode")
    }

    homeDir, err := os.UserHomeDir()
    if err != nil {
        t.Skip("cannot determine home directory")
    }

    downloadsDir := filepath.Join(homeDir, "Downloads")
    if _, err := os.Stat(downloadsDir); os.IsNotExist(err) {
        t.Skip("Downloads folder not found")
    }

    tmpDir := t.TempDir()
    cfg := config.Config{
        Addr:           ":8090",
        DataDir:        tmpDir,
        MaxUploadBytes: 500 * 1024 * 1024,
    }

    logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
    srv, err := api.NewServer(cfg, logger)
    if err != nil {
        t.Fatalf("NewServer: %v", err)
    }

    entries, err := os.ReadDir(downloadsDir)
    if err != nil {
        t.Fatalf("read Downloads: %v", err)
    }

    uploadCount := 0
    maxUploads := 10

    for _, entry := range entries {
        if entry.IsDir() || uploadCount >= maxUploads {
            continue
        }

        filePath := filepath.Join(downloadsDir, entry.Name())
        info, err := entry.Info()
        if err != nil || info.Size() > 100*1024*1024 {
            continue
        }

        fileData, err := os.ReadFile(filePath)
        if err != nil {
            continue
        }

        body := &bytes.Buffer{}
        writer := multipart.NewWriter(body)
        part, _ := writer.CreateFormFile("files", entry.Name())
        part.Write(fileData)
        writer.WriteField("comment", fmt.Sprintf("Real file from Downloads: %s", entry.Name()))
        writer.Close()

        req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
        req.Header.Set("Content-Type", writer.FormDataContentType())
        rec := httptest.NewRecorder()
        srv.Router().ServeHTTP(rec, req)

        if rec.Code == http.StatusOK {
            var resp map[string]any
            json.Unmarshal(rec.Body.Bytes(), &resp)
            if stored, ok := resp["stored"].([]any); ok && len(stored) > 0 {
                fileInfo := stored[0].(map[string]any)
                t.Logf("✓ Real file uploaded: %s → %s (category: %s)",
                    entry.Name(),
                    fileInfo["path"],
                    fileInfo["category"])
                uploadCount++
            }
        }
    }

    if uploadCount == 0 {
        t.Skip("no suitable files found in Downloads for testing")
    }

    t.Logf("✓ Processed %d real files from Downloads", uploadCount)
}

func containsPath(path, dir string) bool {
    return len(path) >= len(dir) && path[:len(dir)] == dir
}
