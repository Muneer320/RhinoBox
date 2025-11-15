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
)

// Manager provides a tiny abstraction over the filesystem for hackspeed storage.
type Manager struct {
	root        string
	storageRoot string
	classifier  *Classifier
	index       *MetadataIndex
	mu          sync.Mutex
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

	return &Manager{root: root, storageRoot: storageRoot, classifier: classifier, index: index}, nil
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

// FindByHash returns file metadata by hash (exposed for testing).
func (m *Manager) FindByHash(hash string) *FileMetadata {
	return m.index.FindByHash(hash)
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

// RenameFileRequest captures parameters for file renaming operation.
type RenameFileRequest struct {
	Hash              string // File hash identifier
	StoredPath        string // Alternative: stored path identifier
	NewName           string // New original filename
	UpdateStoredFile  bool   // Whether to rename the actual file on disk
}

// RenameFileResult contains the outcome of a rename operation.
type RenameFileResult struct {
	Metadata FileMetadata
	DiskRenamed bool
}

// RenameFile renames a file's metadata and optionally the stored file on disk.
func (m *Manager) RenameFile(req RenameFileRequest) (*RenameFileResult, error) {
	if req.Hash == "" && req.StoredPath == "" {
		return nil, errors.New("either hash or stored_path must be provided")
	}
	if req.NewName == "" {
		return nil, errors.New("new_name is required")
	}
	
	// Validate new filename (no path traversal)
	if strings.Contains(req.NewName, "..") || strings.Contains(req.NewName, "/") || strings.Contains(req.NewName, "\\") {
		return nil, errors.New("invalid filename: path traversal not allowed")
	}
	
	// Find the file metadata
	var meta *FileMetadata
	if req.Hash != "" {
		meta = m.index.FindByHash(req.Hash)
	} else {
		meta = m.index.FindByPath(req.StoredPath)
	}
	
	if meta == nil {
		return nil, errors.New("file not found")
	}
	
	diskRenamed := false
	newStoredPath := meta.StoredPath
	
	// If updating stored file, rename it on disk
	if req.UpdateStoredFile {
		m.mu.Lock()
		
		oldAbsPath := filepath.Join(m.root, filepath.FromSlash(meta.StoredPath))
		if _, err := os.Stat(oldAbsPath); err != nil {
			m.mu.Unlock()
			if os.IsNotExist(err) {
				return nil, errors.New("stored file not found on disk")
			}
			return nil, fmt.Errorf("stat stored file: %w", err)
		}
		
		// Build new filename preserving hash prefix
		oldDir := filepath.Dir(oldAbsPath)
		base := strings.TrimSuffix(req.NewName, filepath.Ext(req.NewName))
		if base == "" {
			base = "file"
		}
		base = sanitize(base)
		ext := strings.ToLower(filepath.Ext(req.NewName))
		newFilename := fmt.Sprintf("%s_%s%s", meta.Hash[:12], base, ext)
		newAbsPath := filepath.Join(oldDir, newFilename)
		
		// Check if new path already exists
		if _, err := os.Stat(newAbsPath); err == nil {
			m.mu.Unlock()
			return nil, errors.New("file with new name already exists")
		}
		
		// Rename the file on disk
		if err := os.Rename(oldAbsPath, newAbsPath); err != nil {
			m.mu.Unlock()
			return nil, fmt.Errorf("rename file on disk: %w", err)
		}
		
		// Update stored path
		rel, err := filepath.Rel(m.root, newAbsPath)
		if err != nil {
			m.mu.Unlock()
			return nil, fmt.Errorf("calculate relative path: %w", err)
		}
		newStoredPath = filepath.ToSlash(rel)
		diskRenamed = true
		m.mu.Unlock()
	}
	
	// Update metadata
	updatedMeta := *meta
	updatedMeta.OriginalName = req.NewName
	updatedMeta.StoredPath = newStoredPath
	
	if err := m.index.UpdateMetadata(meta.Hash, updatedMeta); err != nil {
		// If disk was renamed but metadata update failed, try to rollback
		if diskRenamed {
			oldAbsPath := filepath.Join(m.root, filepath.FromSlash(meta.StoredPath))
			newAbsPath := filepath.Join(m.root, filepath.FromSlash(newStoredPath))
			_ = os.Rename(newAbsPath, oldAbsPath) // best effort rollback
		}
		return nil, fmt.Errorf("update metadata: %w", err)
	}
	
	return &RenameFileResult{
		Metadata: updatedMeta,
		DiskRenamed: diskRenamed,
	}, nil
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
