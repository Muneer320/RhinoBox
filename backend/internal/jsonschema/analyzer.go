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
