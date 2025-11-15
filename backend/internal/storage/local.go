package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/Muneer320/RhinoBox/internal/cache"
)

// Manager provides a tiny abstraction over the filesystem for hackspeed storage.
type Manager struct {
	root           string
	storageRoot    string
	classifier     *Classifier
	index          *MetadataIndex
	hashIndex      *cache.HashIndex
	referenceIndex *ReferenceIndex
	mu             sync.Mutex
}

// StoreRequest captures parameters for the high-throughput storage path.
type StoreRequest struct {
	Reader       io.Reader
	Filename     string
	MimeType     string
	Size         int64
	Metadata     map[string]string
	CategoryHint string
}

// StoreResult surfaces the outcome of a storage operation.
type StoreResult struct {
	Metadata  FileMetadata
	Duplicate bool
}

// NewManager bootstraps the directory structure RhinoBox expects.
func NewManager(root string) (*Manager, error) {
	mediaDir := filepath.Join(root, "media")
	jsonDir := filepath.Join(root, "json")
	if err := os.MkdirAll(mediaDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(jsonDir, 0o755); err != nil {
		return nil, err
	}

	storageRoot := filepath.Join(root, "storage")
	if err := ensureStorageTree(storageRoot); err != nil {
		return nil, err
	}

	classifier := NewClassifier()
	index, err := NewMetadataIndex(filepath.Join(root, "metadata", "files.json"))
	if err != nil {
		return nil, err
	}

	// Initialize cache for deduplication
	cacheConfig := cache.DefaultConfig()
	cacheConfig.L3Path = filepath.Join(root, "cache")
	cacheInstance, err := cache.New(cacheConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cache: %w", err)
	}

	hashIndex := cache.NewHashIndex(cacheInstance)

	return &Manager{
		root:        root,
		storageRoot: storageRoot,
		classifier:  classifier,
		index:       index,
		hashIndex:   hashIndex,
	}, nil
}

// Root returns the configured base directory.
func (m *Manager) Root() string {
	return m.root
}

// StoreFile writes a file to the organized storage tree and records metadata for deduplication.
func (m *Manager) StoreFile(req StoreRequest) (*StoreResult, error) {
	if req.Reader == nil {
		return nil, errors.New("store file: nil reader")
	}

	components := m.classifier.Classify(req.MimeType, req.Filename, req.CategoryHint)
	fullDir := filepath.Join(append([]string{m.storageRoot}, components...)...)
	if err := os.MkdirAll(fullDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(m.storageRoot, ".tmp"), 0o755); err != nil {
		return nil, err
	}

	base := strings.TrimSuffix(req.Filename, filepath.Ext(req.Filename))
	if base == "" {
		base = "file"
	}
	base = sanitize(base)
	ext := strings.ToLower(filepath.Ext(req.Filename))
	if ext == "" {
		ext = ""
	}

	hasher := sha256.New()
	tee := io.TeeReader(req.Reader, hasher)
	tmpPath := filepath.Join(m.storageRoot, ".tmp", fmt.Sprintf("tmp_%s", uuid.NewString()))
	if err := writeFastFile(tmpPath, tee, req.Size); err != nil {
		_ = os.Remove(tmpPath)
		return nil, err
	}
	checksum := hex.EncodeToString(hasher.Sum(nil))

	m.mu.Lock()
	if existing := m.index.FindByHash(checksum); existing != nil {
		m.mu.Unlock()
		_ = os.Remove(tmpPath)
		return &StoreResult{Metadata: *existing, Duplicate: true}, nil
	}

	filename := fmt.Sprintf("%s_%s%s", checksum[:12], base, ext)
	finalPath := filepath.Join(fullDir, filename)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		m.mu.Unlock()
		_ = os.Remove(tmpPath)
		return nil, err
	}

	rel, err := filepath.Rel(m.root, finalPath)
	if err != nil {
		m.mu.Unlock()
		return nil, err
	}
	info, err := os.Stat(finalPath)
	if err != nil {
		m.mu.Unlock()
		return nil, err
	}

	metaCopy := map[string]string(nil)
	if len(req.Metadata) > 0 {
		metaCopy = make(map[string]string, len(req.Metadata))
		for k, v := range req.Metadata {
			metaCopy[k] = v
		}
	}

	metadata := FileMetadata{
		Hash:         checksum,
		OriginalName: req.Filename,
		StoredPath:   filepath.ToSlash(rel),
		Category:     strings.Join(components, "/"),
		MimeType:     req.MimeType,
		Size:         info.Size(),
		UploadedAt:   time.Now().UTC(),
		Metadata:     metaCopy,
	}
	if err := m.index.Add(metadata); err != nil {
		m.mu.Unlock()
		return nil, err
	}
	m.mu.Unlock()

	return &StoreResult{Metadata: metadata, Duplicate: false}, nil
}

// StoreMedia streams the reader contents into the categorized folder and returns the relative path.
func (m *Manager) StoreMedia(subdirs []string, originalName string, reader io.Reader) (string, error) {
	dirParts := append([]string{m.root, "media"}, subdirs...)
	dir := filepath.Join(dirParts...)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	ext := filepath.Ext(originalName)
	base := strings.TrimSuffix(originalName, ext)
	if base == "" {
		base = "asset"
	}
	base = sanitize(base)
	filename := fmt.Sprintf("%s_%s%s", base, uuid.NewString(), strings.ToLower(ext))
	fullPath := filepath.Join(dir, filename)

	file, err := os.Create(fullPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		return "", err
	}

	rel := filepath.ToSlash(filepath.Join(append([]string{"media"}, append(subdirs, filename)...)...))
	return rel, nil
}

// WriteJSONFile writes the JSON payload with indentation for readability.
func (m *Manager) WriteJSONFile(relPath string, payload any) (string, error) {
	abs := filepath.Join(m.root, relPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(abs, data, 0o644); err != nil {
		return "", err
	}

	return filepath.ToSlash(relPath), nil
}

// AppendNDJSON appends newline-delimited JSON documents atomically.
func (m *Manager) AppendNDJSON(relPath string, docs []map[string]any) (string, error) {
	abs := filepath.Join(m.root, relPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	file, err := os.OpenFile(abs, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return "", err
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	for _, doc := range docs {
		if err := enc.Encode(doc); err != nil {
			return "", err
		}
	}

	return filepath.ToSlash(relPath), nil
}

// NextJSONBatchPath returns a timestamped file path for new JSON batches.
func (m *Manager) NextJSONBatchPath(engine, namespace string) string {
	slug := sanitize(namespace)
	if slug == "" {
		slug = "default"
	}
	ts := time.Now().UTC().Format("20060102T150405Z")
	return filepath.ToSlash(filepath.Join("json", engine, slug, fmt.Sprintf("batch_%s.ndjson", ts)))
}

var invalidChars = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func sanitize(input string) string {
	lower := strings.ToLower(input)
	lower = invalidChars.ReplaceAllString(lower, "-")
	return strings.Trim(lower, "-")
}

func ensureStorageTree(root string) error {
	for category, subdirs := range storageLayout {
		if len(subdirs) == 0 {
			if err := os.MkdirAll(filepath.Join(root, category), 0o755); err != nil {
				return err
			}
			continue
		}
		for _, sd := range subdirs {
			if err := os.MkdirAll(filepath.Join(root, category, sd), 0o755); err != nil {
				return err
			}
		}
	}
	return os.MkdirAll(filepath.Join(root, "other", "unknown"), 0o755)
}

var storageLayout = map[string][]string{
	"images":        {"jpg", "png", "gif", "svg", "webp", "bmp"},
	"videos":        {"mp4", "avi", "mov", "mkv", "webm", "flv"},
	"audio":         {"mp3", "wav", "flac", "ogg"},
	"documents":     {"pdf", "doc", "docx", "txt", "rtf", "md", "epub", "mobi"},
	"spreadsheets":  {"xls", "xlsx", "csv"},
	"presentations": {"ppt", "pptx"},
	"archives":      {"zip", "tar", "gz", "rar"},
	"code":          {"py", "js", "go", "java", "cpp"},
	"other":         {"unknown"},
}
