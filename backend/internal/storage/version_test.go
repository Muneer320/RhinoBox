package storage

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestVersionIndex(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "versions.json")

	idx, err := NewVersionIndex(indexPath)
	if err != nil {
		t.Fatalf("failed to create version index: %v", err)
	}

	fileID := "test-file-123"
	hash1 := "hash1"
	hash2 := "hash2"
	hash3 := "hash3"

	// Test creating a version chain
	chain, err := idx.CreateVersionChain(fileID, hash1, 1000, "user1", "Initial version")
	if err != nil {
		t.Fatalf("failed to create version chain: %v", err)
	}

	if chain.FileID != fileID {
		t.Errorf("expected file_id %s, got %s", fileID, chain.FileID)
	}
	if chain.CurrentVersion != 1 {
		t.Errorf("expected current_version 1, got %d", chain.CurrentVersion)
	}
	if len(chain.Versions) != 1 {
		t.Errorf("expected 1 version, got %d", len(chain.Versions))
	}

	// Test adding versions
	version2, err := idx.AddVersion(fileID, hash2, 2000, "user2", "Updated version", 0)
	if err != nil {
		t.Fatalf("failed to add version: %v", err)
	}

	if version2.Version != 2 {
		t.Errorf("expected version 2, got %d", version2.Version)
	}
	if version2.Hash != hash2 {
		t.Errorf("expected hash %s, got %s", hash2, version2.Hash)
	}
	if !version2.IsCurrent {
		t.Error("version 2 should be current")
	}

	version3, err := idx.AddVersion(fileID, hash3, 3000, "user3", "Another update", 0)
	if err != nil {
		t.Fatalf("failed to add version: %v", err)
	}

	if version3.Version != 3 {
		t.Errorf("expected version 3, got %d", version3.Version)
	}

	// Test listing versions
	versions, err := idx.ListVersions(fileID)
	if err != nil {
		t.Fatalf("failed to list versions: %v", err)
	}

	if len(versions) != 3 {
		t.Errorf("expected 3 versions, got %d", len(versions))
	}

	// Versions should be sorted descending (newest first)
	if versions[0].Version != 3 {
		t.Errorf("expected first version to be 3, got %d", versions[0].Version)
	}
	if versions[2].Version != 1 {
		t.Errorf("expected last version to be 1, got %d", versions[2].Version)
	}

	// Test getting specific version
	version, err := idx.GetVersion(fileID, 2)
	if err != nil {
		t.Fatalf("failed to get version: %v", err)
	}

	if version.Version != 2 {
		t.Errorf("expected version 2, got %d", version.Version)
	}
	if version.Hash != hash2 {
		t.Errorf("expected hash %s, got %s", hash2, version.Hash)
	}

	// Test reverting to a previous version
	reverted, err := idx.RevertToVersion(fileID, 1, "Reverting due to error")
	if err != nil {
		t.Fatalf("failed to revert version: %v", err)
	}

	if reverted.Version != 1 {
		t.Errorf("expected version 1, got %d", reverted.Version)
	}
	if !reverted.IsCurrent {
		t.Error("reverted version should be current")
	}

	// Verify current version was updated
	chain, _ = idx.GetVersionChain(fileID)
	if chain.CurrentVersion != 1 {
		t.Errorf("expected current_version 1, got %d", chain.CurrentVersion)
	}

	// Test version diff
	diff, err := idx.GetVersionDiff(fileID, 1, 3)
	if err != nil {
		t.Fatalf("failed to get version diff: %v", err)
	}

	if diff["from_version"].(int) != 1 {
		t.Errorf("expected from_version 1, got %v", diff["from_version"])
	}
	if diff["to_version"].(int) != 3 {
		t.Errorf("expected to_version 3, got %v", diff["to_version"])
	}

	changes := diff["changes"].(map[string]any)
	if changes["size"] == nil {
		t.Error("expected size change in diff")
	}
}

func TestVersionIndexPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "versions.json")

	// Create index and add data
	idx1, err := NewVersionIndex(indexPath)
	if err != nil {
		t.Fatalf("failed to create version index: %v", err)
	}

	fileID := "test-file-456"
	_, err = idx1.CreateVersionChain(fileID, "hash1", 1000, "user1", "Initial")
	if err != nil {
		t.Fatalf("failed to create version chain: %v", err)
	}

	_, err = idx1.AddVersion(fileID, "hash2", 2000, "user2", "Update", 0)
	if err != nil {
		t.Fatalf("failed to add version: %v", err)
	}

	// Create new index instance (simulates restart)
	idx2, err := NewVersionIndex(indexPath)
	if err != nil {
		t.Fatalf("failed to create version index: %v", err)
	}

	// Verify data persisted
	chain, err := idx2.GetVersionChain(fileID)
	if err != nil {
		t.Fatalf("failed to get version chain: %v", err)
	}

	if len(chain.Versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(chain.Versions))
	}
	if chain.CurrentVersion != 2 {
		t.Errorf("expected current_version 2, got %d", chain.CurrentVersion)
	}
}

func TestVersionIndexMaxVersions(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "versions.json")

	idx, err := NewVersionIndex(indexPath)
	if err != nil {
		t.Fatalf("failed to create version index: %v", err)
	}

	fileID := "test-file-789"
	_, err = idx.CreateVersionChain(fileID, "hash1", 1000, "user1", "Initial")
	if err != nil {
		t.Fatalf("failed to create version chain: %v", err)
	}

	// Add versions up to limit
	maxVersions := 3
	for i := 2; i <= maxVersions; i++ {
		hash := "hash" + string(rune('0'+i))
		_, err = idx.AddVersion(fileID, hash, int64(i*1000), "user", "Update", maxVersions)
		if err != nil {
			t.Fatalf("failed to add version %d: %v", i, err)
		}
	}

	// Try to add one more - should fail
	_, err = idx.AddVersion(fileID, "hash4", 4000, "user", "Update", maxVersions)
	if err == nil {
		t.Error("expected error when exceeding max versions")
	}
	if err != nil && err.Error() != "version limit reached: max versions (3) reached" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestManagerCreateVersion(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// First, store an initial file
	initialContent := bytes.NewReader([]byte("initial content"))
	storeReq := StoreRequest{
		Reader:   initialContent,
		Filename: "test.txt",
		MimeType:  "text/plain",
		Size:     15,
	}

	storeResult, err := manager.StoreFile(storeReq)
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}

	fileID := storeResult.Metadata.Hash

	// Create a new version
	newContent := bytes.NewReader([]byte("updated content"))
	versionReq := VersionRequest{
		FileID:     fileID,
		Reader:     newContent,
		Filename:   "test.txt",
		MimeType:   "text/plain",
		Size:       15,
		Comment:    "Updated version",
		UploadedBy: "user1",
	}

	result, err := manager.CreateVersion(versionReq)
	if err != nil {
		t.Fatalf("failed to create version: %v", err)
	}

	if result.FileID != fileID {
		t.Errorf("expected file_id %s, got %s", fileID, result.FileID)
	}
	if result.Version.Version != 2 {
		t.Errorf("expected version 2, got %d", result.Version.Version)
	}
	if result.Version.Comment != "Updated version" {
		t.Errorf("expected comment 'Updated version', got '%s'", result.Version.Comment)
	}
	if result.IsNewFile {
		t.Error("expected IsNewFile to be false")
	}

	// List versions
	versions, err := manager.ListVersions(fileID)
	if err != nil {
		t.Fatalf("failed to list versions: %v", err)
	}

	if len(versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(versions))
	}

	// Verify version 1 exists
	version1, err := manager.GetVersion(fileID, 1)
	if err != nil {
		t.Fatalf("failed to get version 1: %v", err)
	}

	if version1.Hash != fileID {
		t.Errorf("expected version 1 hash %s, got %s", fileID, version1.Hash)
	}

	// Verify version 2 exists
	version2, err := manager.GetVersion(fileID, 2)
	if err != nil {
		t.Fatalf("failed to get version 2: %v", err)
	}

	if version2.Version != 2 {
		t.Errorf("expected version 2, got %d", version2.Version)
	}
}

func TestManagerRevertVersion(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Store initial file
	initialContent := bytes.NewReader([]byte("version 1"))
	storeReq := StoreRequest{
		Reader:   initialContent,
		Filename: "test.txt",
		MimeType: "text/plain",
		Size:     9,
	}

	storeResult, err := manager.StoreFile(storeReq)
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}

	fileID := storeResult.Metadata.Hash
	hash1 := fileID

	// Create version 2
	version2Content := bytes.NewReader([]byte("version 2"))
	versionReq := VersionRequest{
		FileID:     fileID,
		Reader:     version2Content,
		Filename:   "test.txt",
		MimeType:   "text/plain",
		Size:       9,
		UploadedBy: "user1",
	}

	_, err = manager.CreateVersion(versionReq)
	if err != nil {
		t.Fatalf("failed to create version: %v", err)
	}

	// Revert to version 1
	reverted, err := manager.RevertVersion(fileID, 1, "Reverting to v1")
	if err != nil {
		t.Fatalf("failed to revert version: %v", err)
	}

	if reverted.Version != 1 {
		t.Errorf("expected version 1, got %d", reverted.Version)
	}
	if !reverted.IsCurrent {
		t.Error("reverted version should be current")
	}
	if reverted.Hash != hash1 {
		t.Errorf("expected hash %s, got %s", hash1, reverted.Hash)
	}
}

func TestManagerVersionDiff(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Store initial file
	initialContent := bytes.NewReader([]byte("small"))
	storeReq := StoreRequest{
		Reader:   initialContent,
		Filename: "test.txt",
		MimeType: "text/plain",
		Size:     5,
	}

	storeResult, err := manager.StoreFile(storeReq)
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}

	fileID := storeResult.Metadata.Hash

	// Create version 2 with different size
	version2Content := bytes.NewReader([]byte("much larger content"))
	versionReq := VersionRequest{
		FileID:     fileID,
		Reader:     version2Content,
		Filename:   "test.txt",
		MimeType:   "text/plain",
		Size:       19,
		Comment:    "Updated with more content",
		UploadedBy: "user2",
	}

	_, err = manager.CreateVersion(versionReq)
	if err != nil {
		t.Fatalf("failed to create version: %v", err)
	}

	// Get diff
	diff, err := manager.GetVersionDiff(fileID, 1, 2)
	if err != nil {
		t.Fatalf("failed to get version diff: %v", err)
	}

	if diff["from_version"].(int) != 1 {
		t.Errorf("expected from_version 1, got %v", diff["from_version"])
	}
	if diff["to_version"].(int) != 2 {
		t.Errorf("expected to_version 2, got %v", diff["to_version"])
	}

	changes := diff["changes"].(map[string]any)
	if changes["size"] == nil {
		t.Error("expected size change in diff")
	}
	if changes["comment"] == nil {
		t.Error("expected comment change in diff")
	}
}

func TestVersionIndexErrors(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "versions.json")

	idx, err := NewVersionIndex(indexPath)
	if err != nil {
		t.Fatalf("failed to create version index: %v", err)
	}

	// Test getting non-existent version chain
	_, err = idx.GetVersionChain("non-existent")
	if err == nil {
		t.Error("expected error for non-existent version chain")
	}

	// Test getting non-existent version
	_, err = idx.GetVersion("non-existent", 1)
	if err == nil {
		t.Error("expected error for non-existent version")
	}

	// Test adding version to non-existent chain
	_, err = idx.AddVersion("non-existent", "hash", 1000, "user", "comment", 0)
	if err == nil {
		t.Error("expected error when adding version to non-existent chain")
	}
}

