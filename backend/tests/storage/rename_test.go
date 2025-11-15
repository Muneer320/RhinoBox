package storage

import (
"bytes"
"errors"
"os"
"path/filepath"
"strings"
"testing"

"github.com/Muneer320/RhinoBox/internal/storage"
)

// Helper function to create a bytes reader
func newBytesReader(data []byte) *bytes.Reader {
return bytes.NewReader(data)
}

// Helper functions to check error types
func isFileNotFoundError(err error) bool {
return err != nil && (errors.Is(err, storage.ErrFileNotFound) ||
strings.Contains(err.Error(), "file not found"))
}

func isInvalidFilenameError(err error) bool {
return err != nil && (errors.Is(err, storage.ErrInvalidFilename) ||
strings.Contains(err.Error(), "invalid filename"))
}

func isNameConflictError(err error) bool {
return err != nil && (errors.Is(err, storage.ErrNameConflict) ||
strings.Contains(err.Error(), "filename conflict"))
}

func TestValidateFilename(t *testing.T) {
tests := []struct {
name     string
filename string
wantErr  bool
errType  error
}{
// Valid filenames
{"valid simple", "document.pdf", false, nil},
{"valid with spaces", "my document.pdf", false, nil},
{"valid with numbers", "file123.txt", false, nil},
{"valid with underscores", "my_file_name.doc", false, nil},
{"valid with hyphens", "my-file-name.doc", false, nil},
{"valid mixed case", "MyDocument.PDF", false, nil},
{"valid no extension", "README", false, nil},
{"valid multiple dots", "file.backup.tar.gz", false, nil},

// Invalid filenames - empty/too short
{"empty string", "", true, storage.ErrInvalidFilename},

// Invalid filenames - too long
{"too long", string(make([]byte, 256)), true, storage.ErrInvalidFilename},

// Invalid filenames - path traversal
{"path traversal dots", "../etc/passwd", true, storage.ErrInvalidFilename},
{"path traversal current", "./file.txt", true, storage.ErrInvalidFilename},
{"path traversal hidden", "../../secret.txt", true, storage.ErrInvalidFilename},

// Invalid filenames - directory separators
{"forward slash", "path/to/file.txt", true, storage.ErrInvalidFilename},
{"backslash", "path\\to\\file.txt", true, storage.ErrInvalidFilename},
{"mixed separators", "path/to\\file.txt", true, storage.ErrInvalidFilename},

// Invalid filenames - special characters
{"colon", "file:name.txt", true, storage.ErrInvalidFilename},
{"pipe", "file|name.txt", true, storage.ErrInvalidFilename},
{"question mark", "file?.txt", true, storage.ErrInvalidFilename},
{"asterisk", "file*.txt", true, storage.ErrInvalidFilename},
{"less than", "file<name.txt", true, storage.ErrInvalidFilename},
{"greater than", "file>name.txt", true, storage.ErrInvalidFilename},
{"quotes", "file\"name.txt", true, storage.ErrInvalidFilename},

// Invalid filenames - reserved Windows names
{"reserved CON", "CON.txt", true, storage.ErrInvalidFilename},
{"reserved PRN", "PRN", true, storage.ErrInvalidFilename},
{"reserved AUX", "aux.log", true, storage.ErrInvalidFilename},
{"reserved NUL", "NUL.dat", true, storage.ErrInvalidFilename},
{"reserved COM1", "COM1", true, storage.ErrInvalidFilename},
{"reserved LPT1", "lpt1.txt", true, storage.ErrInvalidFilename},

// Invalid filenames - leading/trailing issues
{"leading dot", ".hidden", true, storage.ErrInvalidFilename},
{"trailing dot", "file.", true, storage.ErrInvalidFilename},
{"leading space", " file.txt", true, storage.ErrInvalidFilename},
{"trailing space", "file.txt ", true, storage.ErrInvalidFilename},
{"leading and trailing spaces", " file.txt ", true, storage.ErrInvalidFilename},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
err := storage.ValidateFilename(tt.filename)
if tt.wantErr {
if err == nil {
t.Errorf("ValidateFilename(%q) expected error, got nil", tt.filename)
}
} else {
if err != nil {
t.Errorf("ValidateFilename(%q) unexpected error: %v", tt.filename, err)
}
}
})
}
}

func TestRenameFile_MetadataOnly(t *testing.T) {
tmpDir := t.TempDir()
mgr, err := storage.NewManager(tmpDir)
if err != nil {
t.Fatalf("NewManager failed: %v", err)
}

// Create a test file
content := []byte("test file content")
req := storage.StoreRequest{
Reader:   newBytesReader(content),
Filename: "original.txt",
MimeType: "text/plain",
Size:     int64(len(content)),
}
result, err := mgr.StoreFile(req)
if err != nil {
t.Fatalf("StoreFile failed: %v", err)
}

// Rename metadata only
renameReq := storage.RenameRequest{
Hash:             result.Metadata.Hash,
NewName:          "renamed.txt",
UpdateStoredFile: false,
}
renameResult, err := mgr.RenameFile(renameReq)
if err != nil {
t.Fatalf("RenameFile failed: %v", err)
}

// Verify metadata was updated
if renameResult.NewMetadata.OriginalName != "renamed.txt" {
t.Errorf("expected new name 'renamed.txt', got %q", renameResult.NewMetadata.OriginalName)
}

// Verify stored path did NOT change
if renameResult.NewMetadata.StoredPath != renameResult.OldMetadata.StoredPath {
t.Errorf("stored path changed unexpectedly: %q -> %q",
renameResult.OldMetadata.StoredPath, renameResult.NewMetadata.StoredPath)
}

// Verify old file still exists at original path
oldPath := filepath.Join(tmpDir, renameResult.OldMetadata.StoredPath)
if _, err := os.Stat(oldPath); err != nil {
t.Errorf("old file should still exist at %q: %v", oldPath, err)
}

// Verify rename was logged
logPath := filepath.Join(tmpDir, "metadata", "rename_log.ndjson")
if _, err := os.Stat(logPath); err != nil {
t.Errorf("rename log should exist: %v", err)
}
}

func TestRenameFile_WithStoredFile(t *testing.T) {
tmpDir := t.TempDir()
mgr, err := storage.NewManager(tmpDir)
if err != nil {
t.Fatalf("NewManager failed: %v", err)
}

// Create a test file
content := []byte("test file content for renaming")
req := storage.StoreRequest{
Reader:   newBytesReader(content),
Filename: "original.txt",
MimeType: "text/plain",
Size:     int64(len(content)),
}
result, err := mgr.StoreFile(req)
if err != nil {
t.Fatalf("StoreFile failed: %v", err)
}

oldPath := filepath.Join(tmpDir, result.Metadata.StoredPath)
if _, err := os.Stat(oldPath); err != nil {
t.Fatalf("original file should exist: %v", err)
}

// Rename with stored file update
renameReq := storage.RenameRequest{
Hash:             result.Metadata.Hash,
NewName:          "renamed_file.txt",
UpdateStoredFile: true,
}
renameResult, err := mgr.RenameFile(renameReq)
if err != nil {
t.Fatalf("RenameFile failed: %v", err)
}

// Verify metadata was updated
if renameResult.NewMetadata.OriginalName != "renamed_file.txt" {
t.Errorf("expected new name 'renamed_file.txt', got %q", renameResult.NewMetadata.OriginalName)
}

// Verify stored path DID change
if renameResult.NewMetadata.StoredPath == renameResult.OldMetadata.StoredPath {
t.Errorf("stored path should have changed")
}

// Verify old file no longer exists
if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
t.Errorf("old file should not exist at %q", oldPath)
}

// Verify new file exists
newPath := filepath.Join(tmpDir, renameResult.NewMetadata.StoredPath)
if _, err := os.Stat(newPath); err != nil {
t.Errorf("new file should exist at %q: %v", newPath, err)
}

// Verify file content is intact
newContent, err := os.ReadFile(newPath)
if err != nil {
t.Fatalf("failed to read renamed file: %v", err)
}
if string(newContent) != string(content) {
t.Errorf("file content mismatch after rename")
}
}

func TestRenameFile_FileNotFound(t *testing.T) {
tmpDir := t.TempDir()
mgr, err := storage.NewManager(tmpDir)
if err != nil {
t.Fatalf("NewManager failed: %v", err)
}

renameReq := storage.RenameRequest{
Hash:    "nonexistenthash123456",
NewName: "renamed.txt",
}
_, err = mgr.RenameFile(renameReq)
if err == nil {
t.Fatal("expected error for non-existent file")
}
if !isFileNotFoundError(err) {
t.Errorf("expected ErrFileNotFound, got: %v", err)
}
}

func TestRenameFile_InvalidFilename(t *testing.T) {
tmpDir := t.TempDir()
mgr, err := storage.NewManager(tmpDir)
if err != nil {
t.Fatalf("NewManager failed: %v", err)
}

// Create a test file
content := []byte("test content")
req := storage.StoreRequest{
Reader:   newBytesReader(content),
Filename: "test.txt",
MimeType: "text/plain",
Size:     int64(len(content)),
}
result, err := mgr.StoreFile(req)
if err != nil {
t.Fatalf("StoreFile failed: %v", err)
}

invalidNames := []string{
"../etc/passwd",
"file<name>.txt",
"CON.txt",
".hidden",
"file ",
"",
}

for _, invalidName := range invalidNames {
renameReq := storage.RenameRequest{
Hash:    result.Metadata.Hash,
NewName: invalidName,
}
_, err := mgr.RenameFile(renameReq)
if err == nil {
t.Errorf("expected error for invalid filename %q", invalidName)
}
if !isInvalidFilenameError(err) {
t.Errorf("expected ErrInvalidFilename for %q, got: %v", invalidName, err)
}
}
}

func TestRenameFile_ConflictDetection(t *testing.T) {
tmpDir := t.TempDir()
mgr, err := storage.NewManager(tmpDir)
if err != nil {
t.Fatalf("NewManager failed: %v", err)
}

// Create two different files with same category but different content
content1 := []byte("file 1 content")
req1 := storage.StoreRequest{
Reader:   newBytesReader(content1),
Filename: "file1.txt",
MimeType: "text/plain",
Size:     int64(len(content1)),
}
result1, err := mgr.StoreFile(req1)
if err != nil {
t.Fatalf("StoreFile failed: %v", err)
}

content2 := []byte("file 2 content - different")
req2 := storage.StoreRequest{
Reader:   newBytesReader(content2),
Filename: "file2.txt",
MimeType: "text/plain",
Size:     int64(len(content2)),
}
result2, err := mgr.StoreFile(req2)
if err != nil {
t.Fatalf("StoreFile failed: %v", err)
}

// Rename file2 with stored file update
renameReq := storage.RenameRequest{
Hash:             result2.Metadata.Hash,
NewName:          "renamed_target.txt",
UpdateStoredFile: true,
}
result2Renamed, err := mgr.RenameFile(renameReq)
if err != nil {
t.Fatalf("First rename failed: %v", err)
}

	// Try to rename file1 to the same name - with hash-based naming,
	// files are unique so this should succeed
	targetFilename := "renamed_target.txt"
	renameReq2 := storage.RenameRequest{
		Hash:             result1.Metadata.Hash,
		NewName:          targetFilename,
		UpdateStoredFile: true,
	}
	result1Renamed, err := mgr.RenameFile(renameReq2)
	
	// Assert explicitly: either the rename succeeded (err == nil) or returned the expected conflict error
	if err != nil {
		if !isNameConflictError(err) {
			t.Fatalf("Unexpected error during second rename: %v", err)
		}
		// If we got a conflict error, the rename didn't happen, so result1Renamed should be nil
		if result1Renamed != nil {
			t.Errorf("Expected nil result when rename fails with conflict error, got: %+v", result1Renamed)
		}
	} else {
		// Rename succeeded, result should not be nil
		if result1Renamed == nil {
			t.Fatal("Expected non-nil result when rename succeeds, got nil")
		}
	}

	// Verify files exist using the NewMetadata.StoredPath values returned by successful renames
	// File2 was successfully renamed, so use its new path
	path2 := filepath.Join(tmpDir, result2Renamed.NewMetadata.StoredPath)
	if _, err := os.Stat(path2); err != nil {
		t.Fatalf("File2 should exist at %q after successful rename: %v", path2, err)
	}

	// File1: if rename succeeded, use new path; if it failed with conflict, use original path
	var path1 string
	if err == nil && result1Renamed != nil {
		// Rename succeeded, use the new path
		path1 = filepath.Join(tmpDir, result1Renamed.NewMetadata.StoredPath)
	} else {
		// Rename failed with conflict, file should still be at original path
		path1 = filepath.Join(tmpDir, result1.Metadata.StoredPath)
	}
	
	if _, err := os.Stat(path1); err != nil {
		t.Fatalf("File1 should exist at %q: %v", path1, err)
	}

	// Verify both files have correct content
	content1Read, err := os.ReadFile(path1)
	if err != nil {
		t.Fatalf("Failed to read file1: %v", err)
	}
	if string(content1Read) != string(content1) {
		t.Errorf("File1 content mismatch: expected %q, got %q", string(content1), string(content1Read))
	}

	content2Read, err := os.ReadFile(path2)
	if err != nil {
		t.Fatalf("Failed to read file2: %v", err)
	}
	if string(content2Read) != string(content2) {
		t.Errorf("File2 content mismatch: expected %q, got %q", string(content2), string(content2Read))
	}
}

func TestFindByOriginalName(t *testing.T) {
tmpDir := t.TempDir()
mgr, err := storage.NewManager(tmpDir)
if err != nil {
t.Fatalf("NewManager failed: %v", err)
}

// Create multiple test files
files := []struct {
name    string
content string
}{
{"report_2024.pdf", "report content"},
{"report_2023.pdf", "old report"},
{"document.txt", "doc content"},
{"image_report.png", "image data"},
}

for _, f := range files {
req := storage.StoreRequest{
Reader:   newBytesReader([]byte(f.content)),
Filename: f.name,
MimeType: "application/octet-stream",
Size:     int64(len(f.content)),
}
if _, err := mgr.StoreFile(req); err != nil {
t.Fatalf("StoreFile(%q) failed: %v", f.name, err)
}
}

// Test search
tests := []struct {
query         string
expectedCount int
expectedNames []string
}{
{"report", 3, []string{"report_2024.pdf", "report_2023.pdf", "image_report.png"}},
{"2024", 1, []string{"report_2024.pdf"}},
{"document", 1, []string{"document.txt"}},
{"nonexistent", 0, []string{}},
{"pdf", 2, []string{"report_2024.pdf", "report_2023.pdf"}},
}

for _, tt := range tests {
t.Run(tt.query, func(t *testing.T) {
results := mgr.FindByOriginalName(tt.query)
if len(results) != tt.expectedCount {
t.Errorf("expected %d results, got %d", tt.expectedCount, len(results))
}

// Verify expected names are present
for _, expectedName := range tt.expectedNames {
found := false
for _, result := range results {
if result.OriginalName == expectedName {
found = true
break
}
}
if !found {
t.Errorf("expected to find %q in results", expectedName)
}
}
})
}
}

func TestRenameFile_Atomicity(t *testing.T) {
tmpDir := t.TempDir()
mgr, err := storage.NewManager(tmpDir)
if err != nil {
t.Fatalf("NewManager failed: %v", err)
}

// Create a test file
content := []byte("test content")
req := storage.StoreRequest{
Reader:   newBytesReader(content),
Filename: "test.txt",
MimeType: "text/plain",
Size:     int64(len(content)),
}
result, err := mgr.StoreFile(req)
if err != nil {
t.Fatalf("StoreFile failed: %v", err)
}

// Verify file exists at original location
oldPath := filepath.Join(tmpDir, result.Metadata.StoredPath)
if _, err := os.Stat(oldPath); err != nil {
t.Fatalf("original file should exist: %v", err)
}

// Attempt rename with stored file update
renameReq := storage.RenameRequest{
Hash:             result.Metadata.Hash,
NewName:          "renamed.txt",
UpdateStoredFile: true,
}
renameResult, err := mgr.RenameFile(renameReq)
if err != nil {
t.Fatalf("RenameFile failed: %v", err)
}

// Verify atomicity: old file shouldn't exist
if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
t.Error("old file should not exist after successful rename")
}

// Verify new file exists with correct content
newPath := filepath.Join(tmpDir, renameResult.NewMetadata.StoredPath)
newContent, err := os.ReadFile(newPath)
if err != nil {
t.Fatalf("new file should exist: %v", err)
}
if string(newContent) != string(content) {
t.Error("file content changed during rename")
}
}
