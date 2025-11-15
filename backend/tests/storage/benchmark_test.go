package storage_test

import (
    "bytes"
    "strings"
    "testing"

    "github.com/Muneer320/RhinoBox/internal/storage"
)

func BenchmarkStoreFile(b *testing.B) {
    dir := b.TempDir()
    mgr, err := storage.NewManager(dir)
    if err != nil {
        b.Fatalf("NewManager: %v", err)
    }

    // 10MB payload
    payload := bytes.Repeat([]byte("benchmark_data_"), 1024*700)
    
    b.ResetTimer()
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        _, err := mgr.StoreFile(storage.StoreRequest{
            Reader:   bytes.NewReader(payload),
            Filename: "bench.bin",
            MimeType: "application/octet-stream",
            Size:     int64(len(payload)),
        })
        if err != nil {
            b.Fatalf("StoreFile: %v", err)
        }
    }

    b.SetBytes(int64(len(payload)))
}

func BenchmarkClassifier(b *testing.B) {
    c := storage.NewClassifier()
    
    testCases := []struct {
        mime     string
        filename string
        hint     string
    }{
        {"image/jpeg", "photo.jpg", ""},
        {"video/mp4", "clip.mp4", ""},
        {"application/pdf", "document.pdf", "reports"},
        {"", "archive.zip", "backups"},
    }

    b.ResetTimer()
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        tc := testCases[i%len(testCases)]
        _ = c.Classify(tc.mime, tc.filename, tc.hint)
    }
}

func BenchmarkFastWriter(b *testing.B) {
    dir := b.TempDir()
    
    // 1MB payload
    payload := bytes.Repeat([]byte(strings.Repeat("x", 1024)), 1024)
    
    b.ResetTimer()
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        path := dir + "/bench.bin"
        if err := storage.WriteFastFileBench(path, bytes.NewReader(payload), int64(len(payload))); err != nil {
            b.Fatalf("WriteFastFile: %v", err)
        }
    }

    b.SetBytes(int64(len(payload)))
}
