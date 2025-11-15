package jsonschema

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Muneer320/RhinoBox/internal/cache"
)

// CachedAnalyzer wraps Analyzer with schema caching
type CachedAnalyzer struct {
	schemaCache *cache.SchemaCache
	maxDepth    int
	maxSample   int
}

// NewCachedAnalyzer creates an analyzer with schema caching
func NewCachedAnalyzer(c *cache.Cache, maxDepth, maxSample int) *CachedAnalyzer {
	return &CachedAnalyzer{
		schemaCache: cache.NewSchemaCache(c, 30*time.Minute), // 30 min cache TTL
		maxDepth:    maxDepth,
		maxSample:   maxSample,
	}
}

// AnalyzeWithCache performs schema analysis with caching
func (ca *CachedAnalyzer) AnalyzeWithCache(documents []map[string]any) (*Summary, *SchemaAnalysis, error) {
	if len(documents) == 0 {
		return nil, nil, fmt.Errorf("no documents to analyze")
	}

	// Serialize documents for hash computation
	data, err := json.Marshal(documents)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal documents: %w", err)
	}

	// Compute schema hash
	schemaHash, err := cache.ComputeSchemaHash(data)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to compute schema hash: %w", err)
	}

	// Try cache first
	if decision, found := ca.schemaCache.GetDecision(schemaHash); found {
		// Cache hit - reconstruct analysis from cached decision
		analysis := &SchemaAnalysis{
			Comment: fmt.Sprintf("Cached analysis (%.0f%% confidence)", decision.Confidence*100),
		}
		
		// Perform fresh analysis for full details (summary)
		// but we can skip decision logic since we have it cached
		analyzer := NewAnalyzer(ca.maxDepth, ca.maxSample)
		analyzer.AnalyzeBatch(documents)
		summary := analyzer.BuildSummary()
		
		return &summary, analysis, nil
	}

	// Cache miss - perform full analysis
	analyzer := NewAnalyzer(ca.maxDepth, ca.maxSample)
	analyzer.AnalyzeBatch(documents)
	summary := analyzer.BuildSummary()
	analysis := analyzer.AnalyzeStructure(documents, summary)

	// Store decision in cache
	decision := cache.Decision{
		IsSQL:      false, // Will be set by decision engine
		Reason:     analysis.Comment,
		Confidence: analysis.SchemaConsistency,
	}
	if err := ca.schemaCache.SetDecision(schemaHash, decision); err != nil {
		// Log but don't fail on cache errors
		fmt.Printf("Warning: failed to cache schema analysis: %v\n", err)
	}

	return &summary, &analysis, nil
}

// InvalidateCache removes cached analysis for given documents
func (ca *CachedAnalyzer) InvalidateCache(documents []map[string]any) error {
	data, err := json.Marshal(documents)
	if err != nil {
		return err
	}

	schemaHash, err := cache.ComputeSchemaHash(data)
	if err != nil {
		return err
	}

	return ca.schemaCache.InvalidateDecision(schemaHash)
}
