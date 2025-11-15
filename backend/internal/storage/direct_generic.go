//go:build !linux

package storage

import (
    "os"
)

func openFileDirect(path string) (*os.File, bool, error) {
    file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
    return file, false, err
}

func preallocateSpace(f *os.File, size int64) error {
    return nil
}
