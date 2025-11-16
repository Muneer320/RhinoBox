package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// StoreJSONArchive stores each batch through the normal file index, making its
// NDJSON available to collections, downloads, notes, rename and deletion.
func (m *Manager) StoreJSONArchive(engine, namespace, backend, table string, docs []map[string]any) (*StoreResult, error) {
	dir := filepath.Join(m.storageRoot, ".tmp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	file, err := os.CreateTemp(dir, "json-archive-*")
	if err != nil {
		return nil, err
	}
	defer func() { file.Close(); _ = os.Remove(file.Name()) }()
	encoder := json.NewEncoder(file)
	for _, doc := range docs {
		if err := encoder.Encode(doc); err != nil {
			return nil, err
		}
	}
	if _, err := file.Seek(0, 0); err != nil {
		return nil, err
	}
	return m.StoreFile(StoreRequest{
		Reader:           file,
		Filename:         fmt.Sprintf("%s_%s.ndjson", firstNonEmptySlug(namespace), time.Now().UTC().Format("20060102T150405.000000000Z")),
		MimeType:         "application/x-ndjson",
		CategoryOverride: "json/" + engine,
		Metadata:         map[string]string{"namespace": namespace, "engine": engine, "storage_backend": backend, "table": table, "record_count": strconv.Itoa(len(docs))},
	})
}

func firstNonEmptySlug(namespace string) string {
	if slug := sanitize(namespace); slug != "" {
		return slug
	}
	return "default"
}
