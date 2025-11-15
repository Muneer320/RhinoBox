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

// CopyFileRequest captures parameters for file copying operations.
type CopyFileRequest struct {
	SourcePath   string            // Path to source file (can be hash or stored path)
	NewName      string            // New filename for the copy
	NewCategory  string            // Optional: new category for the copy
	Metadata     map[string]string // Optional: metadata for the copy
	HardLink     bool              // If true, create hard link instead of full copy
}

// CopyFileResult surfaces the outcome of a copy operation.
type CopyFileResult struct {
	SourceMetadata FileMetadata
	CopyMetadata   FileMetadata
	IsHardLink     bool
}

// CopyFile creates a copy of an existing file with new metadata.
func (m *Manager) CopyFile(req CopyFileRequest) (*CopyFileResult, error) {
	if req.SourcePath == "" {
		return nil, errors.New("source path required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Find source file by hash or path
	var sourceMeta *FileMetadata
	sourceMeta = m.index.FindByHash(req.SourcePath)
	if sourceMeta == nil {
		sourceMeta = m.index.FindByPath(req.SourcePath)
	}
	if sourceMeta == nil {
		return nil, fmt.Errorf("source file not found: %s", req.SourcePath)
	}

	// Handle hard link creation
	if req.HardLink {
		return m.createHardLinkLocked(sourceMeta, req)
	}

	// Handle full copy
	return m.createFullCopyLocked(sourceMeta, req)
}

// createHardLinkLocked creates a hard link reference to the same physical file.
func (m *Manager) createHardLinkLocked(sourceMeta *FileMetadata, req CopyFileRequest) (*CopyFileResult, error) {
	// Determine the original file hash (if source is already a hard link)
	originalHash := sourceMeta.Hash
	if sourceMeta.IsHardLink && sourceMeta.LinkedTo != "" {
		originalHash = sourceMeta.LinkedTo
	}

	// Get original file metadata
	originalMeta := m.index.FindByHash(originalHash)
	if originalMeta == nil {
		return nil, errors.New("original file not found")
	}

	// Generate new filename
	newName := req.NewName
	if newName == "" {
		newName = sourceMeta.OriginalName
	}

	// Determine category
	newCategory := req.NewCategory
	if newCategory == "" {
		newCategory = sourceMeta.Category
	}

	// Classify and determine storage path
	components := strings.Split(newCategory, "/")
	if req.NewCategory == "" {
		components = m.classifier.Classify(sourceMeta.MimeType, newName, "")
	}

	// Generate new metadata entry with unique hash (for indexing)
	newHash := fmt.Sprintf("%s_ref_%s", originalHash[:12], uuid.NewString()[:8])
	
	// Copy metadata
	metaCopy := make(map[string]string)
	if len(sourceMeta.Metadata) > 0 {
		for k, v := range sourceMeta.Metadata {
			metaCopy[k] = v
		}
	}
	if len(req.Metadata) > 0 {
		for k, v := range req.Metadata {
			metaCopy[k] = v
		}
	}

	// The stored path for hard link points to the same file
	newMetadata := FileMetadata{
		Hash:         newHash,
		OriginalName: newName,
		StoredPath:   originalMeta.StoredPath, // Same physical file
		Category:     strings.Join(components, "/"),
		MimeType:     sourceMeta.MimeType,
		Size:         sourceMeta.Size,
		UploadedAt:   time.Now().UTC(),
		Metadata:     metaCopy,
		IsHardLink:   true,
		LinkedTo:     originalHash,
		RefCount:     0,
	}

	// Increment reference count on original file
	if err := m.index.IncrementRefCount(originalHash); err != nil {
		return nil, err
	}

	// Add new metadata entry
	if err := m.index.Add(newMetadata); err != nil {
		return nil, err
	}

	return &CopyFileResult{
		SourceMetadata: *sourceMeta,
		CopyMetadata:   newMetadata,
		IsHardLink:     true,
	}, nil
}

// createFullCopyLocked creates a physical copy of the file.
func (m *Manager) createFullCopyLocked(sourceMeta *FileMetadata, req CopyFileRequest) (*CopyFileResult, error) {
	// Open source file
	sourceFullPath := filepath.Join(m.root, filepath.FromSlash(sourceMeta.StoredPath))
	sourceFile, err := os.Open(sourceFullPath)
	if err != nil {
		return nil, fmt.Errorf("open source file: %w", err)
	}
	defer sourceFile.Close()

	// Get file info
	info, err := sourceFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat source file: %w", err)
	}

	// Generate new filename
	newName := req.NewName
	if newName == "" {
		newName = sourceMeta.OriginalName
	}

	// Determine category
	newCategory := req.NewCategory
	if newCategory == "" {
		newCategory = sourceMeta.Category
	}

	// Classify and build target directory
	components := strings.Split(newCategory, "/")
	if req.NewCategory == "" {
		components = m.classifier.Classify(sourceMeta.MimeType, newName, "")
	}
	
	fullDir := filepath.Join(append([]string{m.storageRoot}, components...)...)
	if err := os.MkdirAll(fullDir, 0o755); err != nil {
		return nil, err
	}

	// Compute hash of new copy (should be same as original for identical content)
	hasher := sha256.New()
	if _, err := io.Copy(hasher, sourceFile); err != nil {
		return nil, err
	}
	checksum := hex.EncodeToString(hasher.Sum(nil))

	// Reset file pointer for actual copy
	if _, err := sourceFile.Seek(0, 0); err != nil {
		return nil, err
	}

	// Create target file
	base := strings.TrimSuffix(newName, filepath.Ext(newName))
	if base == "" {
		base = "file"
	}
	base = sanitize(base)
	ext := strings.ToLower(filepath.Ext(newName))
	filename := fmt.Sprintf("%s_%s%s", checksum[:12], base, ext)
	finalPath := filepath.Join(fullDir, filename)

	// Copy file content
	targetFile, err := os.Create(finalPath)
	if err != nil {
		return nil, err
	}
	defer targetFile.Close()

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		_ = os.Remove(finalPath)
		return nil, err
	}

	// Build relative path
	rel, err := filepath.Rel(m.root, finalPath)
	if err != nil {
		return nil, err
	}

	// Copy metadata
	metaCopy := make(map[string]string)
	if len(sourceMeta.Metadata) > 0 {
		for k, v := range sourceMeta.Metadata {
			metaCopy[k] = v
		}
	}
	if len(req.Metadata) > 0 {
		for k, v := range req.Metadata {
			metaCopy[k] = v
		}
	}

	// Create new metadata
	newMetadata := FileMetadata{
		Hash:         checksum,
		OriginalName: newName,
		StoredPath:   filepath.ToSlash(rel),
		Category:     strings.Join(components, "/"),
		MimeType:     sourceMeta.MimeType,
		Size:         info.Size(),
		UploadedAt:   time.Now().UTC(),
		Metadata:     metaCopy,
		IsHardLink:   false,
		RefCount:     0,
	}

	// Check if this hash already exists (deduplication)
	if existing := m.index.FindByHash(checksum); existing != nil {
		// Remove the newly created file since it's a duplicate
		_ = os.Remove(finalPath)
		
		// For copy operations, we want to create a new metadata entry
		// even if the content is the same, so generate a unique hash
		newMetadata.Hash = fmt.Sprintf("%s_copy_%s", checksum[:12], uuid.NewString()[:8])
		newMetadata.StoredPath = existing.StoredPath
		newMetadata.IsHardLink = true
		newMetadata.LinkedTo = checksum
		
		// Increment reference count on the original
		if err := m.index.IncrementRefCount(checksum); err != nil {
			return nil, err
		}
	}

	// Add new metadata entry
	if err := m.index.Add(newMetadata); err != nil {
		return nil, err
	}

	return &CopyFileResult{
		SourceMetadata: *sourceMeta,
		CopyMetadata:   newMetadata,
		IsHardLink:     newMetadata.IsHardLink,
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
