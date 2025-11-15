package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// HashIndex provides content-addressed storage for deduplication
// Files with identical content share the same hash key
type HashIndex struct {
	cache *Cache
}

// NewHashIndex creates a new hash-based index
func NewHashIndex(cache *Cache) *HashIndex {
	return &HashIndex{cache: cache}
}

// ComputeHash calculates SHA-256 hash of content
func ComputeHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

// ComputeHashFromReader calculates SHA-256 hash from a reader
func ComputeHashFromReader(r io.Reader) (string, error) {
	hasher := sha256.New()
	if _, err := io.Copy(hasher, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// GetByHash retrieves content by its hash
func (h *HashIndex) GetByHash(hash string) ([]byte, bool) {
	return h.cache.Get("hash:" + hash)
}

// SetByHash stores content indexed by its hash
func (h *HashIndex) SetByHash(hash string, content []byte) error {
	return h.cache.Set("hash:"+hash, content)
}

// GetOrCompute returns cached content if hash exists, otherwise computes and stores
func (h *HashIndex) GetOrCompute(content []byte) (hash string, isDuplicate bool, err error) {
	hash = ComputeHash(content)
	
	// Check if hash already exists
	_, exists := h.GetByHash(hash)
	if exists {
		return hash, true, nil
	}
	
	// New content - store it
	if err := h.SetByHash(hash, content); err != nil {
		return "", false, err
	}
	
	return hash, false, nil
}

// MediaMetadata stores metadata about deduplicated media files
type MediaMetadata struct {
	Hash         string   `json:"hash"`
	OriginalName string   `json:"original_name"`
	ContentType  string   `json:"content_type"`
	Size         int64    `json:"size"`
	References   []string `json:"references"` // Paths that reference this hash
}

// SetMetadata stores metadata for a media file
func (h *HashIndex) SetMetadata(hash string, meta MediaMetadata) error {
	data := fmt.Sprintf("%s|%s|%s|%d|%v", 
		meta.Hash, meta.OriginalName, meta.ContentType, meta.Size, meta.References)
	return h.cache.Set("meta:"+hash, []byte(data))
}

// DeleteByHash removes content by its hash
func (h *HashIndex) DeleteByHash(hash string) error {
	return h.cache.Delete("hash:" + hash)
}
