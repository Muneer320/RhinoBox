package storage_test

import (
    "bytes"
    "os"
    "path/filepath"
    "testing"

    "github.com/Muneer320/RhinoBox/internal/storage"
)

func TestStoreFileCreatesMetadata(t *testing.T) {
    dir := t.TempDir()
    mgr, err := storage.NewManager(dir)
    if err != nil {
        t.Fatalf("NewManager: %v", err)
    }

    payload := bytes.Repeat([]byte("hello world"), 2048)
    res, err := mgr.StoreFile(storage.StoreRequest{
        Reader:   bytes.NewReader(payload),
        Filename: "photo.JPG",
        MimeType: "image/jpeg",
        Size:     int64(len(payload)),
        Metadata: map[string]string{"source": "unit-test"},
    })
    if err != nil {
        t.Fatalf("StoreFile: %v", err)
    }
    if res.Duplicate {
        t.Fatalf("expected new file, got duplicate")
    }
    if res.Metadata.Category != "images/jpg" {
        t.Fatalf("unexpected category %s", res.Metadata.Category)
    }
    if res.Metadata.Hash == "" {
        t.Fatalf("hash not recorded")
    }

    storedPath := filepath.Join(dir, res.Metadata.StoredPath)
    data, err := os.ReadFile(storedPath)
    if err != nil {
        t.Fatalf("stored file missing: %v", err)
    }
    if !bytes.Equal(data, payload) {
        t.Fatalf("stored payload mismatch")
    }
}

func TestStoreFileDeduplicates(t *testing.T) {
    dir := t.TempDir()
    mgr, err := storage.NewManager(dir)
    if err != nil {
        t.Fatalf("NewManager: %v", err)
    }

    payload := bytes.Repeat([]byte("abc123"), 4096)
    first, err := mgr.StoreFile(storage.StoreRequest{
        Reader:   bytes.NewReader(payload),
        Filename: "report.pdf",
        MimeType: "application/pdf",
        Size:     int64(len(payload)),
    })
    if err != nil {
        t.Fatalf("first store: %v", err)
    }
    second, err := mgr.StoreFile(storage.StoreRequest{
        Reader:   bytes.NewReader(payload),
        Filename: "report-copy.pdf",
        MimeType: "application/pdf",
        Size:     int64(len(payload)),
    })
    if err != nil {
        t.Fatalf("second store: %v", err)
    }
    if !second.Duplicate {
        t.Fatalf("expected duplicate detection")
    }
    if first.Metadata.Hash != second.Metadata.Hash {
        t.Fatalf("hash mismatch for duplicate")
    }
}
