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

// MoveRequest captures parameters for moving a file to a new category.
type MoveRequest struct {
	FileHash     string // Hash to identify the file
	FilePath     string // Or path to identify the file
	NewCategory  string // New category path (e.g., "images/png", "documents/pdf/reports")
	Reason       string // Reason for the move (for logging)
}

// MoveResult surfaces the outcome of a move operation.
type MoveResult struct {
	OldPath      string       `json:"old_path"`
	NewPath      string       `json:"new_path"`
	OldCategory  string       `json:"old_category"`
	NewCategory  string       `json:"new_category"`
	Metadata     FileMetadata `json:"metadata"`
	Renamed      bool         `json:"renamed"` // True if file was renamed to avoid conflict
}

// MoveFile moves a file to a new category while maintaining metadata integrity.
func (m *Manager) MoveFile(req MoveRequest) (*MoveResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find the file by hash or path
	var meta *FileMetadata
	if req.FileHash != "" {
		meta = m.index.FindByHash(req.FileHash)
	} else if req.FilePath != "" {
		meta = m.index.FindByPath(req.FilePath)
	}
	
	if meta == nil {
		return nil, errors.New("file not found")
	}

	// Parse new category
	if req.NewCategory == "" {
		return nil, errors.New("new category is required")
	}
	
	// Split category into components
	newComponents := strings.Split(req.NewCategory, "/")
	if len(newComponents) == 0 {
		return nil, errors.New("invalid category format")
	}

	// Build new directory path
	fullNewDir := filepath.Join(append([]string{m.storageRoot}, newComponents...)...)
	if err := os.MkdirAll(fullNewDir, 0o755); err != nil {
		return nil, fmt.Errorf("create target directory: %w", err)
	}

	// Get current file info
	oldAbsPath := filepath.Join(m.root, filepath.FromSlash(meta.StoredPath))
	if _, err := os.Stat(oldAbsPath); err != nil {
		return nil, fmt.Errorf("source file not found: %w", err)
	}

	// Construct new filename (keep the same name)
	filename := filepath.Base(oldAbsPath)
	newAbsPath := filepath.Join(fullNewDir, filename)
	
	// Check for naming conflict and resolve
	renamed := false
	if _, err := os.Stat(newAbsPath); err == nil {
		// File exists, add suffix to avoid conflict
		ext := filepath.Ext(filename)
		base := strings.TrimSuffix(filename, ext)
		newFilename := fmt.Sprintf("%s_moved_%d%s", base, time.Now().Unix(), ext)
		newAbsPath = filepath.Join(fullNewDir, newFilename)
		renamed = true
	}

	// Move the physical file
	if err := os.Rename(oldAbsPath, newAbsPath); err != nil {
		return nil, fmt.Errorf("move file: %w", err)
	}

	// Calculate new relative path
	newRelPath, err := filepath.Rel(m.root, newAbsPath)
	if err != nil {
		// Rollback: move file back
		_ = os.Rename(newAbsPath, oldAbsPath)
		return nil, fmt.Errorf("calculate relative path: %w", err)
	}

	// Update metadata
	oldPath := meta.StoredPath
	oldCategory := meta.Category
	meta.StoredPath = filepath.ToSlash(newRelPath)
	meta.Category = req.NewCategory

	// Add reason to metadata if provided
	if req.Reason != "" {
		if meta.Metadata == nil {
			meta.Metadata = make(map[string]string)
		}
		meta.Metadata["move_reason"] = req.Reason
		meta.Metadata["moved_at"] = time.Now().UTC().Format(time.RFC3339)
		meta.Metadata["moved_from"] = oldPath
	}

	// Update index
	if err := m.index.Update(*meta); err != nil {
		// Rollback: move file back and restore metadata
		_ = os.Rename(newAbsPath, oldAbsPath)
		return nil, fmt.Errorf("update metadata: %w", err)
	}

	// Clean up empty directories
	oldDir := filepath.Dir(oldAbsPath)
	_ = m.cleanupEmptyDirs(oldDir)

	return &MoveResult{
		OldPath:     oldPath,
		NewPath:     meta.StoredPath,
		OldCategory: oldCategory,
		NewCategory: meta.Category,
		Metadata:    *meta,
		Renamed:     renamed,
	}, nil
}

// BatchMoveRequest represents a batch move operation.
type BatchMoveRequest struct {
	Files []MoveRequest `json:"files"`
}

// BatchMoveResult contains results for all files in a batch move.
type BatchMoveResult struct {
	Results  []MoveResult `json:"results"`
	Errors   []string     `json:"errors,omitempty"`
	Success  int          `json:"success"`
	Failed   int          `json:"failed"`
}

// BatchMoveFiles moves multiple files atomically (all succeed or all fail).
func (m *Manager) BatchMoveFiles(req BatchMoveRequest) (*BatchMoveResult, error) {
	if len(req.Files) == 0 {
		return nil, errors.New("no files to move")
	}

	result := &BatchMoveResult{
		Results: make([]MoveResult, 0, len(req.Files)),
		Errors:  make([]string, 0),
	}

	// Track moves for potential rollback
	type moveRecord struct {
		oldPath string
		newPath string
		meta    FileMetadata
	}
	completed := make([]moveRecord, 0, len(req.Files))

	// Attempt all moves
	for i, fileReq := range req.Files {
		moveResult, err := m.MoveFile(fileReq)
		if err != nil {
			// Rollback all completed moves
			for j := len(completed) - 1; j >= 0; j-- {
				record := completed[j]
				oldAbs := filepath.Join(m.root, filepath.FromSlash(record.oldPath))
				newAbs := filepath.Join(m.root, filepath.FromSlash(record.newPath))
				_ = os.Rename(newAbs, oldAbs)
				// Restore old metadata
				record.meta.StoredPath = record.oldPath
				_ = m.index.Update(record.meta)
			}
			
			result.Failed = len(req.Files)
			result.Errors = append(result.Errors, fmt.Sprintf("file %d: %v", i, err))
			return result, fmt.Errorf("batch move failed at file %d: %w", i, err)
		}

		completed = append(completed, moveRecord{
			oldPath: moveResult.OldPath,
			newPath: moveResult.NewPath,
			meta:    moveResult.Metadata,
		})
		result.Results = append(result.Results, *moveResult)
		result.Success++
	}

	return result, nil
}

// cleanupEmptyDirs removes empty directories up to the storage root.
func (m *Manager) cleanupEmptyDirs(dir string) error {
	for dir != m.storageRoot && dir != m.root {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			break
		}
		if err := os.Remove(dir); err != nil {
			break
		}
		dir = filepath.Dir(dir)
	}
	return nil
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
