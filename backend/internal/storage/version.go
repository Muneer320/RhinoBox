package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

var (
	ErrVersionNotFound     = errors.New("version not found")
	ErrInvalidVersion      = errors.New("invalid version number")
	ErrVersionLimitReached = errors.New("version limit reached")
)

// VersionMetadata represents a single version of a file
type VersionMetadata struct {
	Version    int       `json:"version"`
	Hash       string    `json:"hash"`
	Size       int64     `json:"size"`
	UploadedAt time.Time `json:"uploaded_at"`
	UploadedBy string    `json:"uploaded_by"`
	Comment    string    `json:"comment"`
	IsCurrent  bool      `json:"is_current"`
}

// VersionChain tracks all versions of a logical file
type VersionChain struct {
	FileID         string            `json:"file_id"`         // Logical file ID (hash of first version)
	CurrentVersion int               `json:"current_version"` // Current version number
	Versions       []VersionMetadata `json:"versions"`        // All versions in order
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// VersionIndex manages version chains for files
type VersionIndex struct {
	path string
	mu   sync.RWMutex
	data map[string]*VersionChain // file_id -> VersionChain
}

// NewVersionIndex creates a new version index
func NewVersionIndex(path string) (*VersionIndex, error) {
	idx := &VersionIndex{
		path: path,
		data: make(map[string]*VersionChain),
	}
	if err := idx.load(); err != nil {
		return nil, err
	}
	return idx, nil
}

// load reads version chains from disk
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

	if len(raw) == 0 {
		return nil
	}

	var chains []VersionChain
	if err := json.Unmarshal(raw, &chains); err != nil {
		return err
	}

	for i := range chains {
		chain := chains[i]
		idx.data[chain.FileID] = &chain
	}
	return nil
}

// persistLocked writes version chains to disk
func (idx *VersionIndex) persistLocked() error {
	chains := make([]VersionChain, 0, len(idx.data))
	for _, chain := range idx.data {
		chains = append(chains, *chain)
	}

	tmp := idx.path + ".tmp"
	buf, err := json.MarshalIndent(chains, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, buf, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, idx.path)
}

// GetVersionChain retrieves a version chain by file ID
func (idx *VersionIndex) GetVersionChain(fileID string) (*VersionChain, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	chain, ok := idx.data[fileID]
	if !ok {
		return nil, fmt.Errorf("%w: file_id=%s", ErrFileNotFound, fileID)
	}

	// Return a deep copy
	chainCopy := *chain
	versionsCopy := make([]VersionMetadata, len(chain.Versions))
	copy(versionsCopy, chain.Versions)
	chainCopy.Versions = versionsCopy

	return &chainCopy, nil
}

// CreateVersionChain creates a new version chain for a file
func (idx *VersionIndex) CreateVersionChain(fileID string, initialHash string, initialSize int64, uploadedBy string, comment string) (*VersionChain, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Check if chain already exists
	if _, exists := idx.data[fileID]; exists {
		return nil, fmt.Errorf("version chain already exists for file_id=%s", fileID)
	}

	now := time.Now().UTC()
	chain := &VersionChain{
		FileID:         fileID,
		CurrentVersion: 1,
		Versions: []VersionMetadata{
			{
				Version:    1,
				Hash:       initialHash,
				Size:       initialSize,
				UploadedAt: now,
				UploadedBy: uploadedBy,
				Comment:    comment,
				IsCurrent:  true,
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	idx.data[fileID] = chain
	if err := idx.persistLocked(); err != nil {
		return nil, err
	}

	chainCopy := *chain
	return &chainCopy, nil
}

// AddVersion adds a new version to an existing chain
func (idx *VersionIndex) AddVersion(fileID string, hash string, size int64, uploadedBy string, comment string, maxVersions int) (*VersionMetadata, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	chain, ok := idx.data[fileID]
	if !ok {
		return nil, fmt.Errorf("%w: file_id=%s", ErrFileNotFound, fileID)
	}

	// Check version limit
	if maxVersions > 0 && len(chain.Versions) >= maxVersions {
		return nil, fmt.Errorf("%w: max versions (%d) reached", ErrVersionLimitReached, maxVersions)
	}

	// Mark current version as not current
	for i := range chain.Versions {
		chain.Versions[i].IsCurrent = false
	}

	// Create new version
	newVersion := VersionMetadata{
		Version:    chain.CurrentVersion + 1,
		Hash:       hash,
		Size:       size,
		UploadedAt: time.Now().UTC(),
		UploadedBy: uploadedBy,
		Comment:    comment,
		IsCurrent:  true,
	}

	chain.Versions = append(chain.Versions, newVersion)
	chain.CurrentVersion = newVersion.Version
	chain.UpdatedAt = time.Now().UTC()

	if err := idx.persistLocked(); err != nil {
		return nil, err
	}

	return &newVersion, nil
}

// GetVersion retrieves a specific version by file ID and version number
func (idx *VersionIndex) GetVersion(fileID string, versionNumber int) (*VersionMetadata, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	chain, ok := idx.data[fileID]
	if !ok {
		return nil, fmt.Errorf("%w: file_id=%s", ErrFileNotFound, fileID)
	}

	for i := range chain.Versions {
		if chain.Versions[i].Version == versionNumber {
			versionCopy := chain.Versions[i]
			return &versionCopy, nil
		}
	}

	return nil, fmt.Errorf("%w: version %d for file_id=%s", ErrVersionNotFound, versionNumber, fileID)
}

// ListVersions returns all versions for a file, sorted by version number (descending)
func (idx *VersionIndex) ListVersions(fileID string) ([]VersionMetadata, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	chain, ok := idx.data[fileID]
	if !ok {
		return nil, fmt.Errorf("%w: file_id=%s", ErrFileNotFound, fileID)
	}

	versions := make([]VersionMetadata, len(chain.Versions))
	copy(versions, chain.Versions)

	// Sort by version number descending (newest first)
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Version > versions[j].Version
	})

	return versions, nil
}

// RevertToVersion sets a previous version as the current version
func (idx *VersionIndex) RevertToVersion(fileID string, versionNumber int, comment string) (*VersionMetadata, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	chain, ok := idx.data[fileID]
	if !ok {
		return nil, fmt.Errorf("%w: file_id=%s", ErrFileNotFound, fileID)
	}

	// Find the version to revert to
	var targetVersion *VersionMetadata
	for i := range chain.Versions {
		if chain.Versions[i].Version == versionNumber {
			targetVersion = &chain.Versions[i]
			break
		}
	}

	if targetVersion == nil {
		return nil, fmt.Errorf("%w: version %d for file_id=%s", ErrVersionNotFound, versionNumber, fileID)
	}

	// Mark all versions as not current
	for i := range chain.Versions {
		chain.Versions[i].IsCurrent = false
	}

	// Mark target version as current
	targetVersion.IsCurrent = true
	chain.CurrentVersion = versionNumber
	chain.UpdatedAt = time.Now().UTC()

	// Update comment if provided
	if comment != "" {
		targetVersion.Comment = comment
	}

	if err := idx.persistLocked(); err != nil {
		return nil, err
	}

	versionCopy := *targetVersion
	return &versionCopy, nil
}

// GetVersionDiff returns metadata differences between two versions
func (idx *VersionIndex) GetVersionDiff(fileID string, fromVersion, toVersion int) (map[string]any, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	chain, ok := idx.data[fileID]
	if !ok {
		return nil, fmt.Errorf("%w: file_id=%s", ErrFileNotFound, fileID)
	}

	var fromMeta, toMeta *VersionMetadata
	for i := range chain.Versions {
		if chain.Versions[i].Version == fromVersion {
			fromMeta = &chain.Versions[i]
		}
		if chain.Versions[i].Version == toVersion {
			toMeta = &chain.Versions[i]
		}
	}

	if fromMeta == nil {
		return nil, fmt.Errorf("%w: version %d for file_id=%s", ErrVersionNotFound, fromVersion, fileID)
	}
	if toMeta == nil {
		return nil, fmt.Errorf("%w: version %d for file_id=%s", ErrVersionNotFound, toVersion, fileID)
	}

	diff := map[string]any{
		"from_version": fromVersion,
		"to_version":   toVersion,
		"file_id":      fileID,
		"changes":      make(map[string]any),
	}

	changes := diff["changes"].(map[string]any)

	// Compare hash
	if fromMeta.Hash != toMeta.Hash {
		changes["hash"] = map[string]string{
			"from": fromMeta.Hash,
			"to":   toMeta.Hash,
		}
	}

	// Compare size
	if fromMeta.Size != toMeta.Size {
		changes["size"] = map[string]int64{
			"from": fromMeta.Size,
			"to":   toMeta.Size,
		}
		changes["size_delta"] = toMeta.Size - fromMeta.Size
	}

	// Compare comment
	if fromMeta.Comment != toMeta.Comment {
		changes["comment"] = map[string]string{
			"from": fromMeta.Comment,
			"to":   toMeta.Comment,
		}
	}

	// Compare uploaded_by
	if fromMeta.UploadedBy != toMeta.UploadedBy {
		changes["uploaded_by"] = map[string]string{
			"from": fromMeta.UploadedBy,
			"to":   toMeta.UploadedBy,
		}
	}

	// Time difference
	timeDiff := toMeta.UploadedAt.Sub(fromMeta.UploadedAt)
	changes["time_delta_seconds"] = int64(timeDiff.Seconds())

	return diff, nil
}

