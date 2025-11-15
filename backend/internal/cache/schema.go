package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// SchemaCache provides caching for JSON schema analysis results
// Maps schema structure hash → routing decision (SQL/NoSQL)
type SchemaCache struct {
	cache *Cache
	ttl   time.Duration
}

// Decision represents a schema analysis decision
type Decision struct {
	IsSQL       bool      `json:"is_sql"`
	Reason      string    `json:"reason"`
	Confidence  float64   `json:"confidence"`
	AnalyzedAt  time.Time `json:"analyzed_at"`
	SchemaHash  string    `json:"schema_hash"`
}

// NewSchemaCache creates a new schema cache
func NewSchemaCache(cache *Cache, ttl time.Duration) *SchemaCache {
	return &SchemaCache{
		cache: cache,
		ttl:   ttl,
	}
}

// GetDecision retrieves cached decision for a schema
func (s *SchemaCache) GetDecision(schemaHash string) (*Decision, bool) {
	data, found := s.cache.Get("schema:" + schemaHash)
	if !found {
		return nil, false
	}

	var decision Decision
	if err := json.Unmarshal(data, &decision); err != nil {
		return nil, false
	}

	// Check if decision has expired
	if time.Since(decision.AnalyzedAt) > s.ttl {
		return nil, false
	}

	return &decision, true
}

// SetDecision stores a schema analysis decision
func (s *SchemaCache) SetDecision(schemaHash string, decision Decision) error {
	decision.AnalyzedAt = time.Now()
	decision.SchemaHash = schemaHash

	data, err := json.Marshal(decision)
	if err != nil {
		return err
	}

	return s.cache.Set("schema:"+schemaHash, data)
}

// ComputeSchemaHash creates a hash from JSON schema structure
func ComputeSchemaHash(schemaData []byte) (string, error) {
	// Parse JSON to normalize structure
	var jsonData interface{}
	if err := json.Unmarshal(schemaData, &jsonData); err != nil {
		return "", err
	}

	// Re-marshal to get consistent formatting
	normalized, err := json.Marshal(jsonData)
	if err != nil {
		return "", err
	}

	// Compute hash
	hash := sha256.Sum256(normalized)
	return hex.EncodeToString(hash[:]), nil
}

// InvalidateDecision removes a cached decision
func (s *SchemaCache) InvalidateDecision(schemaHash string) error {
	return s.cache.Delete("schema:" + schemaHash)
}

// GetOrAnalyze retrieves cached decision or runs analysis function
func (s *SchemaCache) GetOrAnalyze(schemaData []byte, analyzeFn func([]byte) (Decision, error)) (Decision, error) {
	// Compute schema hash
	schemaHash, err := ComputeSchemaHash(schemaData)
	if err != nil {
		return Decision{}, fmt.Errorf("failed to compute schema hash: %w", err)
	}

	// Try cache first
	if decision, found := s.GetDecision(schemaHash); found {
		return *decision, nil
	}

	// Cache miss - run analysis
	decision, err := analyzeFn(schemaData)
	if err != nil {
		return Decision{}, err
	}

	// Store in cache
	if err := s.SetDecision(schemaHash, decision); err != nil {
		// Log but don't fail on cache write errors
		fmt.Printf("Warning: failed to cache schema decision: %v\n", err)
	}

	return decision, nil
}
