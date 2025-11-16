package cache

import (
	"bytes"
	"io/fs"
	"path/filepath"
	"testing"
)

func TestReopenFindsPersistedEntries(t *testing.T) {
	cfg := DefaultConfig()
	cfg.L3Path = t.TempDir()
	first, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })
	want := []byte("persisted metadata")
	if err := first.Set("persistent-key", want); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
	second, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })
	got, ok := second.Get("persistent-key")
	if !ok || !bytes.Equal(got, want) {
		t.Fatalf("persisted entry missing after reopening: found=%v", ok)
	}
}

func TestEmptyCacheUsesBoundedDiskSpace(t *testing.T) {
	cfg := DefaultConfig()
	cfg.L3Path = t.TempDir()
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	var total int64
	err = filepath.WalkDir(cfg.L3Path, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if total > 64<<20 {
		t.Fatalf("empty metadata cache reserved %d bytes; expected at most 64 MiB", total)
	}
}
