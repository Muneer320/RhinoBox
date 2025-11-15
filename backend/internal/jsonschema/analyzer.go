package jsonschema

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// JsonType mirrors the Python implementation to keep reasoning readable.
type JsonType string

const (
	Null    JsonType = "null"
	Boolean JsonType = "boolean"
	Integer JsonType = "integer"
	Float   JsonType = "float"
	String  JsonType = "string"
	Array   JsonType = "array"
	Object  JsonType = "object"
)

// FieldStats tracks how often a field appears and what types it carries.
type FieldStats struct {
	Path       string
	TypeCounts map[JsonType]int
	Presence   int
	Nulls      int
	MaxLen     int
}

// Summary provides a condensed overview for downstream decisions.
type Summary struct {
	DocumentsAnalyzed int
	TotalFields       int
	MaxDepth          int
	FieldStability    float64
	TypeStability     float64
	HasArrayObjects   bool
	StructureHash     string
	Fields            map[string]FieldSummary
}

// FieldSummary mirrors the Python summary payload for UI/analytics.
type FieldSummary struct {
	DominantType JsonType `json:"dominant_type"`
	TypeShare    float64  `json:"type_stability"`
	Presence     float64  `json:"presence"`
	NullFraction float64  `json:"null_fraction"`
	MaxLength    int      `json:"max_length"`
	LikelyFK     bool     `json:"is_likely_fk"`
}

// SchemaAnalysis captures the richer set of heuristics used by the decision engine.
type SchemaAnalysis struct {
	HasForeignKeys      bool     `json:"has_foreign_keys"`
	HasRelationships    bool     `json:"has_relationships"`
	RequiresJoins       bool     `json:"requires_joins"`
	HasConstraints      bool     `json:"has_constraints"`
	SchemaConsistency   float64  `json:"schema_consistency"`
	FieldCount          int      `json:"field_count"`
	MaxNestingDepth     int      `json:"max_nesting_depth"`
	ArrayFields         []string `json:"array_fields"`
	RecordCount         int      `json:"record_count"`
	UniqueFieldSets     int      `json:"unique_field_sets"`
	ExpectedWriteLoad   string   `json:"expected_write_load"`
	ExpectedReadPattern string   `json:"expected_read_pattern"`
	Comment             string   `json:"comment"`
}

// Analyzer inspects JSON docs just like the original Python helper but keeps things snappy.
type Analyzer struct {
	maxDepth          int
	maxSample         int
	docs              int
	stats             map[string]*FieldStats
	topLevelCounts    map[string]int
	observedMaxDepth  int
	arrayObjectsFound bool
}

// NewAnalyzer creates a new analyzer.
func NewAnalyzer(maxDepth, maxSample int) *Analyzer {
	return &Analyzer{
		maxDepth:       maxDepth,
		maxSample:      maxSample,
		stats:          map[string]*FieldStats{},
		topLevelCounts: map[string]int{},
	}
}

// AnalyzeBatch ingests an entire batch.
func (a *Analyzer) AnalyzeBatch(documents []map[string]any) {
	if len(documents) > a.maxSample {
		documents = documents[:a.maxSample]
	}
	for _, doc := range documents {
		a.analyze(doc)
	}
}

func (a *Analyzer) analyze(doc map[string]any) {
	if a.docs >= a.maxSample {
		return
	}
	a.docs++
	for key := range doc {
		a.topLevelCounts[key]++
	}
	flattened := flattenJSON(doc, a.maxDepth)
	for path, entry := range flattened {
		a.observedMaxDepth = max(a.observedMaxDepth, entry.depth)
		stats := a.ensureStats(path)
		stats.TypeCounts[entry.jsonType]++
		stats.Presence++
		if entry.value == nil {
			stats.Nulls++
		} else if entry.jsonType == String {
			if l := len(fmt.Sprintf("%v", entry.value)); l > stats.MaxLen {
				stats.MaxLen = l
			}
		}
		if entry.isArrayObjects {
			a.arrayObjectsFound = true
		}
	}
}

func (a *Analyzer) ensureStats(path string) *FieldStats {
	if _, ok := a.stats[path]; !ok {
		a.stats[path] = &FieldStats{Path: path, TypeCounts: map[JsonType]int{}}
	}
	return a.stats[path]
}

// BuildSummary compiles the numeric insights.
func (a *Analyzer) BuildSummary() Summary {
	fields := map[string]FieldSummary{}
	for path, stats := range a.stats {
		dominantType, typeShare := stats.dominant()
		fields[path] = FieldSummary{
			DominantType: dominantType,
			TypeShare:    typeShare,
			Presence:     float64(stats.Presence) / float64(max(1, a.docs)),
			NullFraction: float64(stats.Nulls) / float64(max(1, stats.Presence)),
			MaxLength:    stats.MaxLen,
			LikelyFK:     looksLikeForeignKey(path),
		}
	}

	return Summary{
		DocumentsAnalyzed: a.docs,
		TotalFields:       len(a.stats),
		MaxDepth:          a.observedMaxDepth,
		FieldStability:    a.fieldStability(),
		TypeStability:     a.typeStability(),
		HasArrayObjects:   a.arrayObjectsFound,
		StructureHash:     a.structureHash(),
		Fields:            fields,
	}
}

// AnalyzeStructure inspects the batch to produce SchemaAnalysis for downstream decisions.
func (a *Analyzer) AnalyzeStructure(documents []map[string]any, summary Summary) SchemaAnalysis {
	analysis := SchemaAnalysis{
		SchemaConsistency:   summary.FieldStability,
		FieldCount:          len(summary.Fields),
		MaxNestingDepth:     summary.MaxDepth,
		ArrayFields:         collectArrayFields(summary),
		RecordCount:         len(documents),
		UniqueFieldSets:     countUniqueFieldSets(documents),
		ExpectedWriteLoad:   "medium",
		ExpectedReadPattern: "mixed",
	}

	analysis.HasForeignKeys = detectForeignKeys(summary)
	analysis.HasRelationships = detectRelationships(documents)
	analysis.RequiresJoins = analysis.HasRelationships
	analysis.HasConstraints = detectConstraints(summary, analysis.RecordCount)
	return analysis
}

// IncorporateCommentHints adjusts the analysis based on free-form comment hints.
func IncorporateCommentHints(analysis SchemaAnalysis, comment string) SchemaAnalysis {
	if comment == "" {
		return analysis
	}
	lower := strings.ToLower(comment)
	if strings.Contains(lower, "sql") || strings.Contains(lower, "relational") {
		analysis.SchemaConsistency = clamp01(analysis.SchemaConsistency + 0.3)
	}
	if strings.Contains(lower, "nosql") || strings.Contains(lower, "flexible") {
		analysis.SchemaConsistency = clamp01(analysis.SchemaConsistency - 0.3)
	}
	if strings.Contains(lower, "high write") || strings.Contains(lower, "many writes") || strings.Contains(lower, "bulk ingest") {
		analysis.ExpectedWriteLoad = "high"
	}
	if strings.Contains(lower, "read heavy") || strings.Contains(lower, "analytical") {
		analysis.ExpectedReadPattern = "analytical"
	}
	if strings.Contains(lower, "transactional") || strings.Contains(lower, "oltp") {
		analysis.ExpectedReadPattern = "transactional"
	}
	if strings.Contains(lower, "join") || strings.Contains(lower, "relationship") {
		analysis.RequiresJoins = true
	}
	analysis.Comment = comment
	return analysis
}

func (a *Analyzer) fieldStability() float64 {
	var total float64
	var count float64
	for key := range a.topLevelCounts {
		total += float64(a.topLevelCounts[key]) / float64(max(1, a.docs))
		count++
	}
	if count == 0 {
		return 0
	}
	return total / count
}

func (a *Analyzer) typeStability() float64 {
	if len(a.stats) == 0 {
		return 0
	}
	var total float64
	for _, stats := range a.stats {
		_, share := stats.dominant()
		total += share
	}
	return total / float64(len(a.stats))
}

func (a *Analyzer) structureHash() string {
	keys := make([]string, 0, len(a.stats))
	for path := range a.stats {
		keys = append(keys, path)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, path := range keys {
		dominantType, _ := a.stats[path].dominant()
		b.WriteString(path)
		b.WriteByte('=')
		b.WriteString(string(dominantType))
		b.WriteByte(';')
	}
	checksum := hashBytes([]byte(b.String()))
	return fmt.Sprintf("%x", checksum)
}

func (fs *FieldStats) dominant() (JsonType, float64) {
	if len(fs.TypeCounts) == 0 {
		return Null, 1
	}
	var (
		bestType JsonType
		bestCnt  int
		acc      int
	)
	for jt, cnt := range fs.TypeCounts {
		acc += cnt
		if cnt > bestCnt {
			bestCnt = cnt
			bestType = jt
		}
	}
	if acc == 0 {
		return bestType, 0
	}
	return bestType, float64(bestCnt) / float64(acc)
}

// flattenJSON returns a depth-tagged map for analysis.
func flattenJSON(obj map[string]any, maxDepth int) map[string]flatEntry {
	result := map[string]flatEntry{}
	for key, value := range obj {
		traverse(value, key, 1, maxDepth, result)
	}
	return result
}

type flatEntry struct {
	value          any
	jsonType       JsonType
	depth          int
	isArrayObjects bool
}

func traverse(value any, path string, depth, maxDepth int, acc map[string]flatEntry) {
	jsonType := detectType(value)
	entry := flatEntry{value: value, jsonType: jsonType, depth: depth, isArrayObjects: false}
	acc[path] = entry

	if depth >= maxDepth {
		return
	}

	switch v := value.(type) {
	case map[string]any:
		for k, child := range v {
			next := path + "." + k
			traverse(child, next, depth+1, maxDepth, acc)
		}
	case []any:
		if len(v) == 0 {
			return
		}
		first := v[0]
		childPath := path + "[]"
		acc[childPath] = flatEntry{value: first, jsonType: Array, depth: depth + 1, isArrayObjects: isObject(first)}
		if isObject(first) {
			for k, child := range first.(map[string]any) {
				next := childPath + "." + k
				traverse(child, next, depth+2, maxDepth, acc)
			}
		}
	}
}

func detectType(value any) JsonType {
	switch v := value.(type) {
	case nil:
		return Null
	case bool:
		return Boolean
	case json.Number:
		if strings.Contains(v.String(), ".") {
			return Float
		}
		return Integer
	case float32, float64:
		return Float
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64:
		return Integer
	case string:
		return String
	case []any:
		return Array
	case map[string]any:
		return Object
	default:
		_ = v
		return String
	}
}

func looksLikeForeignKey(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, "_id") || strings.HasSuffix(lower, "_key") || strings.Contains(lower, "id")
}

func collectArrayFields(summary Summary) []string {
	arr := make([]string, 0)
	seen := map[string]struct{}{}
	for path, field := range summary.Fields {
		if strings.Contains(path, "[]") || field.DominantType == Array {
			name := strings.Split(strings.ReplaceAll(path, "[]", ""), ".")[0]
			if name == "" {
				name = path
			}
			if _, ok := seen[name]; !ok {
				seen[name] = struct{}{}
				arr = append(arr, name)
			}
		}
	}
	sort.Strings(arr)
	return arr
}

func countUniqueFieldSets(docs []map[string]any) int {
	if len(docs) == 0 {
		return 0
	}
	sets := map[string]struct{}{}
	for _, doc := range docs {
		keys := make([]string, 0, len(doc))
		for k := range doc {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		sets[strings.Join(keys, ",")] = struct{}{}
	}
	return len(sets)
}

func detectForeignKeys(summary Summary) bool {
	for _, field := range summary.Fields {
		if field.LikelyFK {
			return true
		}
	}
	return false
}

func detectRelationships(docs []map[string]any) bool {
	if len(docs) == 0 {
		return false
	}
	seen := map[string]map[any]int{}
	for _, doc := range docs {
		for field, value := range doc {
			if strings.HasSuffix(field, "_id") || strings.HasSuffix(field, "Id") || field == "id" {
				if seen[field] == nil {
					seen[field] = map[any]int{}
				}
				seen[field][value]++
			}
		}
	}
	for _, counts := range seen {
		for _, count := range counts {
			if count > 1 {
				return true
			}
		}
	}
	return false
}

func detectConstraints(summary Summary, total int) bool {
	if total == 0 {
		return false
	}
	for _, field := range summary.Fields {
		if field.Presence >= 0.95 && field.NullFraction <= 0.05 {
			return true
		}
	}
	return false
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func isObject(value any) bool {
	_, ok := value.(map[string]any)
	return ok
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func hashBytes(data []byte) [32]byte {
	return sha256.Sum256(data)
}
