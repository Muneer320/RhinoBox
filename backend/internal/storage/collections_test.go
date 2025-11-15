package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetCollectionsEmpty(t *testing.T) {
	root := t.TempDir()
	mgr, err := NewManager(root)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	collections, err := mgr.GetCollections()
	if err != nil {
		t.Fatalf("get collections: %v", err)
	}

	if len(collections) != 0 {
		t.Errorf("expected 0 collections for empty storage, got %d", len(collections))
	}
}

func TestGetCollectionsWithFiles(t *testing.T) {
	root := t.TempDir()
	mgr, err := NewManager(root)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	// Create test files in storage directories
	testFiles := []struct {
		path   string
		content string
	}{
		{"storage/images/jpg/test1.jpg", "fake image 1"},
		{"storage/images/png/test2.png", "fake image 2"},
		{"storage/videos/mp4/test3.mp4", "fake video"},
		{"storage/audio/mp3/test4.mp3", "fake audio"},
	}

	for _, tf := range testFiles {
		fullPath := filepath.Join(root, tf.path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(tf.content), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	collections, err := mgr.GetCollections()
	if err != nil {
		t.Fatalf("get collections: %v", err)
	}

	if len(collections) == 0 {
		t.Fatalf("expected at least 1 collection after creating files, got 0")
	}

	// Verify we have the expected collections
	collectionTypes := make(map[string]bool)
	for _, coll := range collections {
		collectionTypes[coll.Type] = true

		// Verify required fields
		if coll.Name == "" {
			t.Errorf("collection missing name: %+v", coll)
		}
		if coll.Description == "" {
			t.Errorf("collection missing description: %+v", coll)
		}
		if coll.Icon == "" {
			t.Errorf("collection missing icon: %+v", coll)
		}
		if coll.FileCount <= 0 {
			t.Errorf("collection should have positive file_count: %+v", coll)
		}
		if coll.TotalSize <= 0 {
			t.Errorf("collection should have positive total_size: %+v", coll)
		}
		if coll.FormattedSize == "" {
			t.Errorf("collection missing formatted_size: %+v", coll)
		}
	}

	// Check for expected collection types
	expectedTypes := []string{"images", "videos", "audio"}
	for _, expectedType := range expectedTypes {
		if !collectionTypes[expectedType] {
			t.Errorf("expected to find collection type '%s', but it was not found", expectedType)
		}
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		got := formatSize(tt.bytes)
		if got != tt.want {
			t.Errorf("formatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}


