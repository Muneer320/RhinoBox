//go:build linux

package storage

import (
    "os"
    "syscall"
)

func openFileDirect(path string) (*os.File, bool, error) {
    file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
    if err != nil {
        return nil, false, err
    }
    return file, false, nil
}

func preallocateSpace(f *os.File, size int64) error {
    return syscall.Fallocate(int(f.Fd()), 0, 0, size)
}
