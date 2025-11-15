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
	rulesMgr       *RoutingRulesManager
	index          *MetadataIndex
	versionIndex   *VersionIndex
	hashIndex      *cache.HashIndex
	referenceIndex *ReferenceIndex
	mu             sync.Mutex
	scanState      scanState
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

	// Initialize routing rules manager
	rulesMgr, err := NewRoutingRulesManager(root)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize routing rules manager: %w", err)
	}

	versionIndex, err := NewVersionIndex(filepath.Join(root, "metadata", "versions.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize version index: %w", err)
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
		root:           root,
		storageRoot:    storageRoot,
		classifier:     classifier,
		rulesMgr:       rulesMgr,
		index:          index,
		versionIndex:   versionIndex,
		hashIndex:      hashIndex,
		referenceIndex: nil, // Lazily initialized when needed
	}, nil
}

// Root returns the configured base directory.
func (m *Manager) Root() string {
	return m.root
}

// RoutingRules returns the routing rules manager.
func (m *Manager) RoutingRules() *RoutingRulesManager {
	return m.rulesMgr
}

// Classifier returns the file classifier.
func (m *Manager) Classifier() *Classifier {
	return m.classifier
}

// StoreFile writes a file to the organized storage tree and records metadata for deduplication.
func (m *Manager) StoreFile(req StoreRequest) (*StoreResult, error) {
	if req.Reader == nil {
		return nil, errors.New("store file: nil reader")
	}

	components := m.classifier.ClassifyWithRules(req.MimeType, req.Filename, req.CategoryHint, m.rulesMgr)
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

// VersionRequest contains parameters for creating a new version
type VersionRequest struct {
	FileID      string
	Reader      io.Reader
	Filename    string
	MimeType    string
	Size        int64
	Comment     string
	UploadedBy  string
	MaxVersions int // 0 = unlimited
}

// VersionResult contains the result of a version operation
type VersionResult struct {
	Version   VersionMetadata `json:"version"`
	FileID    string          `json:"file_id"`
	IsNewFile bool            `json:"is_new_file"` // True if this is the first version
}

// CreateVersion creates a new version of a file
func (m *Manager) CreateVersion(req VersionRequest) (*VersionResult, error) {
	if req.FileID == "" {
		return nil, errors.New("file_id is required")
	}
	if req.Reader == nil {
		return nil, errors.New("reader is required")
	}

	// Store the new file version
	storeReq := StoreRequest{
		Reader:       req.Reader,
		Filename:     req.Filename,
		MimeType:     req.MimeType,
		Size:         req.Size,
		CategoryHint: "",
	}
	if req.Comment != "" {
		storeReq.Metadata = map[string]string{"comment": req.Comment}
	}

	storeResult, err := m.StoreFile(storeReq)
	if err != nil {
		return nil, fmt.Errorf("store file: %w", err)
	}

	newHash := storeResult.Metadata.Hash
	newSize := storeResult.Metadata.Size

	// Check if version chain exists for this file_id
	chain, err := m.versionIndex.GetVersionChain(req.FileID)
	isNewFile := false

	if err != nil {
		// Version chain doesn't exist - create one
		// Check if file_id exists in metadata (it's a hash of an existing file)
		m.mu.Lock()
		existingMeta := m.index.FindByHash(req.FileID)
		m.mu.Unlock()

		if existingMeta != nil {
			// File exists - create chain with original file as version 1, then add new version as version 2
			// Extract comment from metadata if available
			originalComment := ""
			if existingMeta.Metadata != nil {
				if comment, ok := existingMeta.Metadata["comment"]; ok {
					originalComment = comment
				}
			}
			_, err = m.versionIndex.CreateVersionChain(req.FileID, existingMeta.Hash, existingMeta.Size, "", originalComment)
			if err != nil {
				return nil, fmt.Errorf("create version chain: %w", err)
			}
			// Add the new version
			version, err := m.versionIndex.AddVersion(req.FileID, newHash, newSize, req.UploadedBy, req.Comment, req.MaxVersions)
			if err != nil {
				return nil, fmt.Errorf("add version: %w", err)
			}
			return &VersionResult{
				Version:   *version,
				FileID:    req.FileID,
				IsNewFile: false,
			}, nil
		} else {
			// File doesn't exist - this is the first version, use file_id as logical ID
			_, err = m.versionIndex.CreateVersionChain(req.FileID, newHash, newSize, req.UploadedBy, req.Comment)
			if err != nil {
				return nil, fmt.Errorf("create version chain: %w", err)
			}
			isNewFile = true
			chain, _ = m.versionIndex.GetVersionChain(req.FileID)
		}
	} else {
		// Version chain exists - add new version
		version, err := m.versionIndex.AddVersion(req.FileID, newHash, newSize, req.UploadedBy, req.Comment, req.MaxVersions)
		if err != nil {
			return nil, fmt.Errorf("add version: %w", err)
		}
		return &VersionResult{
			Version:   *version,
			FileID:    req.FileID,
			IsNewFile: false,
		}, nil
	}

	// Return first version (only reached for new files)
	if chain != nil && len(chain.Versions) > 0 {
		return &VersionResult{
			Version:   chain.Versions[0],
			FileID:    chain.FileID,
			IsNewFile: isNewFile,
		}, nil
	}

	return nil, errors.New("failed to create version")
}

// ListVersions returns all versions for a file
func (m *Manager) ListVersions(fileID string) ([]VersionMetadata, error) {
	return m.versionIndex.ListVersions(fileID)
}

// GetVersion retrieves a specific version
func (m *Manager) GetVersion(fileID string, versionNumber int) (*VersionMetadata, error) {
	return m.versionIndex.GetVersion(fileID, versionNumber)
}

// GetVersionFile retrieves the file content for a specific version
func (m *Manager) GetVersionFile(fileID string, versionNumber int) (*FileRetrievalResult, error) {
	version, err := m.versionIndex.GetVersion(fileID, versionNumber)
	if err != nil {
		return nil, err
	}

	// Retrieve file by hash
	return m.GetFileByHash(version.Hash)
}

// RevertVersion reverts a file to a previous version
func (m *Manager) RevertVersion(fileID string, versionNumber int, comment string) (*VersionMetadata, error) {
	return m.versionIndex.RevertToVersion(fileID, versionNumber, comment)
}

// GetVersionDiff returns differences between two versions
func (m *Manager) GetVersionDiff(fileID string, fromVersion, toVersion int) (map[string]any, error) {
	return m.versionIndex.GetVersionDiff(fileID, fromVersion, toVersion)
}
