package storage

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFile_FullCopy(t *testing.T) {
	tmpDir := t.TempDir()
	m, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Store an original file
	content := []byte("test file content")
	storeReq := StoreRequest{
		Reader:   bytes.NewReader(content),
		Filename: "original.txt",
		MimeType: "text/plain",
		Size:     int64(len(content)),
		Metadata: map[string]string{"comment": "original"},
	}

	storeResult, err := m.StoreFile(storeReq)
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}

	// Copy the file
	copyReq := CopyRequest{
		Hash:     storeResult.Metadata.Hash,
		NewName:  "copy.txt",
		Metadata: map[string]string{"comment": "copy"},
	}

	copyResult, err := m.CopyFile(copyReq)
	if err != nil {
		t.Fatalf("failed to copy file: %v", err)
	}

	// Verify copy result
	if copyResult.HardLink {
		t.Error("expected full copy, got hard link")
	}
	if copyResult.NewHash == copyResult.OriginalHash {
		t.Error("new hash should be different from original hash")
	}
	if copyResult.NewMeta.OriginalName != "copy.txt" {
		t.Errorf("expected new name 'copy.txt', got '%s'", copyResult.NewMeta.OriginalName)
	}
	if copyResult.NewMeta.StoredPath == copyResult.OriginalMeta.StoredPath {
		t.Error("stored paths should be different for full copy")
	}
	if copyResult.NewMeta.Metadata["comment"] != "copy" {
		t.Errorf("expected metadata comment 'copy', got '%s'", copyResult.NewMeta.Metadata["comment"])
	}

	// Verify both files exist
	originalPath := filepath.Join(tmpDir, copyResult.OriginalMeta.StoredPath)
	newPath := filepath.Join(tmpDir, copyResult.NewMeta.StoredPath)

	if _, err := os.Stat(originalPath); err != nil {
		t.Errorf("original file not found: %v", err)
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Errorf("copied file not found: %v", err)
	}

	// Verify file contents are the same
	originalContent, _ := os.ReadFile(originalPath)
	newContent, _ := os.ReadFile(newPath)
	if !bytes.Equal(originalContent, newContent) {
		t.Error("file contents should be identical")
	}
}

func TestCopyFile_HardLink(t *testing.T) {
	tmpDir := t.TempDir()
	m, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Store an original file
	content := []byte("test file content for hard link")
	storeReq := StoreRequest{
		Reader:   bytes.NewReader(content),
		Filename: "original.txt",
		MimeType: "text/plain",
		Size:     int64(len(content)),
	}

	storeResult, err := m.StoreFile(storeReq)
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}

	// Create hard link copy
	copyReq := CopyRequest{
		Hash:     storeResult.Metadata.Hash,
		NewName:  "hardlink.txt",
		HardLink: true,
	}

	copyResult, err := m.CopyFile(copyReq)
	if err != nil {
		t.Fatalf("failed to create hard link: %v", err)
	}

	// Verify hard link result
	if !copyResult.HardLink {
		t.Error("expected hard link, got full copy")
	}
	if copyResult.NewMeta.StoredPath != copyResult.OriginalMeta.StoredPath {
		t.Error("stored paths should be the same for hard link")
	}

	// Verify reference count
	originalPath := filepath.Join(tmpDir, copyResult.OriginalMeta.StoredPath)
	refCount := m.referenceIndex.GetReferenceCount(originalPath)
	if refCount != 2 {
		t.Errorf("expected 2 references, got %d", refCount)
	}

	// Verify both metadata entries exist
	originalMeta, err := m.GetFileMetadata(copyResult.OriginalHash)
	if err != nil {
		t.Errorf("failed to get original metadata: %v", err)
	}
	if originalMeta == nil {
		t.Error("original metadata not found")
	}

	newMeta, err := m.GetFileMetadata(copyResult.NewHash)
	if err != nil {
		t.Errorf("failed to get new metadata: %v", err)
	}
	if newMeta == nil {
		t.Error("new metadata not found")
	}
}

func TestCopyFile_WithNewCategory(t *testing.T) {
	tmpDir := t.TempDir()
	m, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Store an original file
	content := []byte("test content")
	storeReq := StoreRequest{
		Reader:   bytes.NewReader(content),
		Filename: "document.pdf",
		MimeType: "application/pdf",
		Size:     int64(len(content)),
	}

	storeResult, err := m.StoreFile(storeReq)
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}

	// Copy to different category
	copyReq := CopyRequest{
		Hash:        storeResult.Metadata.Hash,
		NewName:     "backup.pdf",
		NewCategory: "documents/backup",
	}

	copyResult, err := m.CopyFile(copyReq)
	if err != nil {
		t.Fatalf("failed to copy file: %v", err)
	}

	if copyResult.NewMeta.Category != "documents/backup" {
		t.Errorf("expected category 'documents/backup', got '%s'", copyResult.NewMeta.Category)
	}
}

func TestCopyFile_DefaultName(t *testing.T) {
	tmpDir := t.TempDir()
	m, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Store an original file
	content := []byte("test")
	storeReq := StoreRequest{
		Reader:   bytes.NewReader(content),
		Filename: "original.txt",
		MimeType: "text/plain",
		Size:     int64(len(content)),
	}

	storeResult, err := m.StoreFile(storeReq)
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}

	// Copy without specifying new name
	copyReq := CopyRequest{
		Hash: storeResult.Metadata.Hash,
	}

	copyResult, err := m.CopyFile(copyReq)
	if err != nil {
		t.Fatalf("failed to copy file: %v", err)
	}

	expectedName := "original_copy.txt"
	if copyResult.NewMeta.OriginalName != expectedName {
		t.Errorf("expected default name '%s', got '%s'", expectedName, copyResult.NewMeta.OriginalName)
	}
}

func TestCopyFile_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	copyReq := CopyRequest{
		Hash:    "nonexistent",
		NewName: "copy.txt",
	}

	_, err = m.CopyFile(copyReq)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
	if !errors.Is(err, ErrFileNotFound) {
		t.Errorf("expected ErrFileNotFound, got %v", err)
	}
}

func TestCopyFile_NameConflict(t *testing.T) {
	tmpDir := t.TempDir()
	m, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Store first file
	content1 := []byte("content1")
	storeReq1 := StoreRequest{
		Reader:   bytes.NewReader(content1),
		Filename: "file.txt",
		MimeType: "text/plain",
		Size:     int64(len(content1)),
	}
	storeResult1, err := m.StoreFile(storeReq1)
	if err != nil {
		t.Fatalf("failed to store first file: %v", err)
	}

	// Store second file in same category
	content2 := []byte("content2")
	storeReq2 := StoreRequest{
		Reader:       bytes.NewReader(content2),
		Filename:     "other.txt",
		MimeType:     "text/plain",
		Size:         int64(len(content2)),
		CategoryHint: storeResult1.Metadata.Category,
	}
	storeResult2, err := m.StoreFile(storeReq2)
	if err != nil {
		t.Fatalf("failed to store second file: %v", err)
	}

	// Try to copy first file with same name as second file in same category
	copyReq := CopyRequest{
		Hash:        storeResult1.Metadata.Hash,
		NewName:     storeResult2.Metadata.OriginalName,
		NewCategory: storeResult2.Metadata.Category,
	}

	_, err = m.CopyFile(copyReq)
	if err == nil {
		t.Error("expected error for name conflict")
	}
	if !errors.Is(err, ErrCopyConflict) {
		t.Errorf("expected ErrCopyConflict, got %v", err)
	}
}

func TestCopyFile_HardLinkDeletion(t *testing.T) {
	tmpDir := t.TempDir()
	m, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Store original file
	content := []byte("test content")
	storeReq := StoreRequest{
		Reader:   bytes.NewReader(content),
		Filename: "original.txt",
		MimeType: "text/plain",
		Size:     int64(len(content)),
	}
	storeResult, err := m.StoreFile(storeReq)
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}

	// Create hard link
	copyReq := CopyRequest{
		Hash:     storeResult.Metadata.Hash,
		NewName:  "hardlink.txt",
		HardLink: true,
	}
	copyResult, err := m.CopyFile(copyReq)
	if err != nil {
		t.Fatalf("failed to create hard link: %v", err)
	}

	originalPath := filepath.Join(tmpDir, copyResult.OriginalMeta.StoredPath)

	// Delete the original - file should still exist
	deleteReq := DeleteRequest{Hash: copyResult.OriginalHash}
	deleteResult, err := m.DeleteFile(deleteReq)
	if err != nil {
		t.Fatalf("failed to delete original: %v", err)
	}
	if !deleteResult.Deleted {
		t.Error("expected deletion to succeed")
	}

	// File should still exist because hard link reference remains
	if _, err := os.Stat(originalPath); err != nil {
		t.Errorf("file should still exist after deleting one hard link: %v", err)
	}

	// Verify reference count decreased
	refCount := m.referenceIndex.GetReferenceCount(originalPath)
	if refCount != 1 {
		t.Errorf("expected 1 reference after deletion, got %d", refCount)
	}

	// Delete the copy - now file should be deleted
	deleteReq2 := DeleteRequest{Hash: copyResult.NewHash}
	_, err = m.DeleteFile(deleteReq2)
	if err != nil {
		t.Fatalf("failed to delete copy: %v", err)
	}

	// File should now be deleted
	if _, err := os.Stat(originalPath); err == nil {
		t.Error("file should be deleted after removing last reference")
	}
}

func TestReferenceIndex(t *testing.T) {
	tmpDir := t.TempDir()
	refPath := filepath.Join(tmpDir, "references.json")

	idx, err := NewReferenceIndex(refPath)
	if err != nil {
		t.Fatalf("failed to create reference index: %v", err)
	}

	// Add references
	physicalPath := "/path/to/file"
	hash1 := "hash1"
	hash2 := "hash2"

	if err := idx.AddReference(physicalPath, hash1); err != nil {
		t.Fatalf("failed to add reference: %v", err)
	}
	if err := idx.AddReference(physicalPath, hash2); err != nil {
		t.Fatalf("failed to add reference: %v", err)
	}

	// Check reference count
	count := idx.GetReferenceCount(physicalPath)
	if count != 2 {
		t.Errorf("expected 2 references, got %d", count)
	}

	// Get references
	refs := idx.GetReferences(physicalPath)
	if len(refs) != 2 {
		t.Errorf("expected 2 references, got %d", len(refs))
	}

	// Remove one reference
	if err := idx.RemoveReference(physicalPath, hash1); err != nil {
		t.Fatalf("failed to remove reference: %v", err)
	}

	count = idx.GetReferenceCount(physicalPath)
	if count != 1 {
		t.Errorf("expected 1 reference after removal, got %d", count)
	}

	// Remove last reference
	if err := idx.RemoveReference(physicalPath, hash2); err != nil {
		t.Fatalf("failed to remove reference: %v", err)
	}

	count = idx.GetReferenceCount(physicalPath)
	if count != 0 {
		t.Errorf("expected 0 references after removal, got %d", count)
	}
}

func TestCopyFile_MetadataMerge(t *testing.T) {
	tmpDir := t.TempDir()
	m, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Store file with metadata
	content := []byte("test")
	storeReq := StoreRequest{
		Reader:   bytes.NewReader(content),
		Filename: "file.txt",
		MimeType: "text/plain",
		Size:     int64(len(content)),
		Metadata: map[string]string{
			"comment": "original",
			"tag":     "important",
		},
	}
	storeResult, err := m.StoreFile(storeReq)
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}

	// Copy with new metadata
	copyReq := CopyRequest{
		Hash: storeResult.Metadata.Hash,
		Metadata: map[string]string{
			"comment": "copy",
			"status":  "draft",
		},
	}
	copyResult, err := m.CopyFile(copyReq)
	if err != nil {
		t.Fatalf("failed to copy file: %v", err)
	}

	// Verify metadata merge
	if copyResult.NewMeta.Metadata["comment"] != "copy" {
		t.Errorf("expected comment 'copy', got '%s'", copyResult.NewMeta.Metadata["comment"])
	}
	if copyResult.NewMeta.Metadata["tag"] != "important" {
		t.Errorf("expected tag 'important', got '%s'", copyResult.NewMeta.Metadata["tag"])
	}
	if copyResult.NewMeta.Metadata["status"] != "draft" {
		t.Errorf("expected status 'draft', got '%s'", copyResult.NewMeta.Metadata["status"])
	}
}

