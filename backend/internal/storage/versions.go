package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileVersion represents a single version of a file.
type FileVersion struct {
	Version      int               `json:"version"`
	Hash         string            `json:"hash"`
	Size         int64             `json:"size"`
	UploadedAt   time.Time         `json:"uploaded_at"`
	UploadedBy   string            `json:"uploaded_by"`
	Comment      string            `json:"comment"`
	StoredPath   string            `json:"stored_path"`
	MimeType     string            `json:"mime_type"`
	OriginalName string            `json:"original_name"`
	IsCurrent    bool              `json:"is_current"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// VersionedFile tracks all versions of a logical file.
type VersionedFile struct {
	FileID         string         `json:"file_id"`
	CurrentVersion int            `json:"current_version"`
	Versions       []FileVersion  `json:"versions"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	Category       string         `json:"category"`
	TotalVersions  int            `json:"total_versions"`
}

// VersionIndex manages versioned files metadata.
type VersionIndex struct {
	path string
	mu   sync.RWMutex
	data map[string]*VersionedFile // fileID -> VersionedFile
}

// NewVersionIndex creates a new version index.
func NewVersionIndex(path string) (*VersionIndex, error) {
	idx := &VersionIndex{
		path: path,
		data: make(map[string]*VersionedFile),
	}
	if err := idx.load(); err != nil {
		return nil, err
	}
	return idx, nil
}

func (idx *VersionIndex) load() error {
	dir := filepath.Dir(idx.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	raw, err := os.ReadFile(idx.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	var files []*VersionedFile
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, &files); err != nil {
		return err
	}

	for _, file := range files {
		idx.data[file.FileID] = file
	}
	return nil
}

func (idx *VersionIndex) persistLocked() error {
	files := make([]*VersionedFile, 0, len(idx.data))
	for _, file := range idx.data {
		files = append(files, file)
	}

	tmp := idx.path + ".tmp"
	buf, err := json.MarshalIndent(files, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, buf, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, idx.path)
}

// GetFile retrieves a versioned file by ID.
func (idx *VersionIndex) GetFile(fileID string) (*VersionedFile, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	
	file, ok := idx.data[fileID]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", fileID)
	}
	
	// Return a copy to prevent external modification
	fileCopy := *file
	fileCopy.Versions = make([]FileVersion, len(file.Versions))
	copy(fileCopy.Versions, file.Versions)
	
	return &fileCopy, nil
}

// CreateFile creates a new versioned file with its first version.
func (idx *VersionIndex) CreateFile(fileID string, version FileVersion, category string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	
	if _, exists := idx.data[fileID]; exists {
		return fmt.Errorf("file already exists: %s", fileID)
	}
	
	version.Version = 1
	version.IsCurrent = true
	
	vf := &VersionedFile{
		FileID:         fileID,
		CurrentVersion: 1,
		Versions:       []FileVersion{version},
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
		Category:       category,
		TotalVersions:  1,
	}
	
	idx.data[fileID] = vf
	return idx.persistLocked()
}

// AddVersion adds a new version to an existing file.
func (idx *VersionIndex) AddVersion(fileID string, version FileVersion) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	
	file, ok := idx.data[fileID]
	if !ok {
		return fmt.Errorf("file not found: %s", fileID)
	}
	
	// Mark all previous versions as not current
	for i := range file.Versions {
		file.Versions[i].IsCurrent = false
	}
	
	// Add new version
	version.Version = file.CurrentVersion + 1
	version.IsCurrent = true
	file.Versions = append(file.Versions, version)
	file.CurrentVersion = version.Version
	file.UpdatedAt = time.Now().UTC()
	file.TotalVersions++
	
	return idx.persistLocked()
}

// GetVersion retrieves a specific version of a file.
func (idx *VersionIndex) GetVersion(fileID string, versionNum int) (*FileVersion, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	
	file, ok := idx.data[fileID]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", fileID)
	}
	
	for _, v := range file.Versions {
		if v.Version == versionNum {
			vCopy := v
			return &vCopy, nil
		}
	}
	
	return nil, fmt.Errorf("version %d not found", versionNum)
}

// RevertToVersion reverts a file to a specific version by creating a new version with the old content.
func (idx *VersionIndex) RevertToVersion(fileID string, targetVersion int, comment string, uploadedBy string) (*FileVersion, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	
	file, ok := idx.data[fileID]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", fileID)
	}
	
	var targetVer *FileVersion
	for i := range file.Versions {
		if file.Versions[i].Version == targetVersion {
			targetVer = &file.Versions[i]
			break
		}
	}
	
	if targetVer == nil {
		return nil, fmt.Errorf("version %d not found", targetVersion)
	}
	
	// Mark all versions as not current
	for i := range file.Versions {
		file.Versions[i].IsCurrent = false
	}
	
	// Create a new version with the old content
	newVersion := FileVersion{
		Version:      file.CurrentVersion + 1,
		Hash:         targetVer.Hash,
		Size:         targetVer.Size,
		UploadedAt:   time.Now().UTC(),
		UploadedBy:   uploadedBy,
		Comment:      comment,
		StoredPath:   targetVer.StoredPath,
		MimeType:     targetVer.MimeType,
		OriginalName: targetVer.OriginalName,
		IsCurrent:    true,
		Metadata:     targetVer.Metadata,
	}
	
	file.Versions = append(file.Versions, newVersion)
	file.CurrentVersion = newVersion.Version
	file.UpdatedAt = time.Now().UTC()
	file.TotalVersions++
	
	if err := idx.persistLocked(); err != nil {
		return nil, err
	}
	
	vCopy := newVersion
	return &vCopy, nil
}

// ListVersions returns all versions of a file.
func (idx *VersionIndex) ListVersions(fileID string) ([]FileVersion, error) {
	file, err := idx.GetFile(fileID)
	if err != nil {
		return nil, err
	}
	return file.Versions, nil
}

// FindFileByHash finds a file ID by its hash (from the main metadata index).
func (idx *VersionIndex) FindFileByHash(hash string) string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	
	for fileID, file := range idx.data {
		for _, v := range file.Versions {
			if v.Hash == hash {
				return fileID
			}
		}
	}
	return ""
}

// PruneVersions removes old versions based on retention policy.
type RetentionPolicy struct {
	KeepLastN      int           // Keep last N versions (0 = keep all)
	KeepWithinDays int           // Keep versions within N days (0 = no time limit)
	KeepMinimum    int           // Always keep at least this many versions (default: 1)
}

// ApplyRetentionPolicy prunes old versions according to the policy.
func (idx *VersionIndex) ApplyRetentionPolicy(fileID string, policy RetentionPolicy) (int, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	
	file, ok := idx.data[fileID]
	if !ok {
		return 0, fmt.Errorf("file not found: %s", fileID)
	}
	
	if policy.KeepMinimum < 1 {
		policy.KeepMinimum = 1
	}
	
	if len(file.Versions) <= policy.KeepMinimum {
		return 0, nil // Nothing to prune
	}
	
	now := time.Now().UTC()
	var versionsToKeep []FileVersion
	
	// Apply filters
	if policy.KeepLastN > 0 {
		// Keep last N versions (sorted by version number)
		startIdx := len(file.Versions) - policy.KeepLastN
		if startIdx < 0 {
			startIdx = 0
		}
		versionsToKeep = make([]FileVersion, len(file.Versions)-startIdx)
		copy(versionsToKeep, file.Versions[startIdx:])
	} else if policy.KeepWithinDays > 0 {
		// Keep versions within time window
		cutoff := now.AddDate(0, 0, -policy.KeepWithinDays)
		for _, v := range file.Versions {
			if v.UploadedAt.After(cutoff) {
				versionsToKeep = append(versionsToKeep, v)
			}
		}
	} else {
		// No pruning policy specified, keep all
		versionsToKeep = file.Versions
	}
	
	// Ensure we keep at least the minimum
	if len(versionsToKeep) < policy.KeepMinimum {
		startIdx := len(file.Versions) - policy.KeepMinimum
		if startIdx < 0 {
			startIdx = 0
		}
		versionsToKeep = make([]FileVersion, len(file.Versions)-startIdx)
		copy(versionsToKeep, file.Versions[startIdx:])
	}
	
	pruned := len(file.Versions) - len(versionsToKeep)
	if pruned > 0 {
		file.Versions = versionsToKeep
		file.TotalVersions = len(versionsToKeep)
		file.UpdatedAt = now
		
		// Update current version pointer to the latest kept version
		if len(versionsToKeep) > 0 {
			file.CurrentVersion = versionsToKeep[len(versionsToKeep)-1].Version
			// Mark the last version as current
			for i := range file.Versions {
				file.Versions[i].IsCurrent = (i == len(file.Versions)-1)
			}
		}
		
		if err := idx.persistLocked(); err != nil {
			return 0, err
		}
	}
	
	return pruned, nil
}
