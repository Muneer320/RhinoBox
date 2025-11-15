package storage

import (
    "bufio"
    "errors"
    "io"
    "os"
    "sync"
)

const (
    fastBufferSize  = 1 << 20  // 1MB write buffer
    poolBufferSize  = 32 << 10 // 32KB reusable buffers
)

var copyBufferPool = sync.Pool{
    New: func() any {
        return make([]byte, poolBufferSize)
    },
}

type FastWriter struct {
    file    *os.File
    buf     *bufio.Writer
    written int64
}

func newFastWriter(path string, sizeHint int64) (*FastWriter, error) {
    file, _, err := openFileDirect(path)
    if err != nil {
        return nil, err
    }

    if sizeHint > 0 {
        _ = preallocateSpace(file, sizeHint)
    }

    return &FastWriter{
        file: file,
        buf:  bufio.NewWriterSize(file, fastBufferSize),
    }, nil
}

func (fw *FastWriter) Write(p []byte) (int, error) {
    n, err := fw.buf.Write(p)
    fw.written += int64(n)
    return n, err
}

func (fw *FastWriter) ReadFrom(r io.Reader) (int64, error) {
    if fw.buf == nil {
        return 0, errors.New("fast writer closed")
    }

    if rf, ok := interface{}(fw.buf).(io.ReaderFrom); ok {
        n, err := rf.ReadFrom(r)
        fw.written += n
        return n, err
    }

    n, err := copyWithPool(fw.buf, r)
    fw.written += n
    return n, err
}

func (fw *FastWriter) Close() error {
    if fw.buf != nil {
        if err := fw.buf.Flush(); err != nil {
            _ = fw.file.Close()
            return err
        }
        fw.buf = nil
    }
    if fw.file != nil {
        err := fw.file.Close()
        fw.file = nil
        return err
    }
    return nil
}

func copyWithPool(dst io.Writer, src io.Reader) (int64, error) {
    buf := copyBufferPool.Get().([]byte)
    n, err := io.CopyBuffer(dst, src, buf)
    copyBufferPool.Put(buf)
    return n, err
}

func writeFastFile(path string, reader io.Reader, sizeHint int64) error {
    writer, err := newFastWriter(path, sizeHint)
    if err != nil {
        return err
    }

    _, copyErr := writer.ReadFrom(reader)
    closeErr := writer.Close()
    if copyErr != nil {
        return copyErr
    }
    return closeErr
}

// WriteFastFileBench exports writeFastFile for benchmarking.
func WriteFastFileBench(path string, reader io.Reader, sizeHint int64) error {
    return writeFastFile(path, reader, sizeHint)
}
