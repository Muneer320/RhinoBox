package jsonschema

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ColumnDef describes a generated SQL column.
type ColumnDef struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

// MongoIndex describes a suggested MongoDB index.
type MongoIndex struct {
	Fields []string `json:"fields"`
	Unique bool     `json:"unique"`
}

type columnStat struct {
	kind         columnKind
	maxLength    int
	presentCount int
	nonNullCount int
}

type columnKind int

const (
	kindUnknown columnKind = iota
	kindInteger
	kindDouble
	kindBoolean
	kindString
	kindJSONB
)

// GeneratePostgresSchema infers a CREATE TABLE statement and column metadata.
func GeneratePostgresSchema(table string, docs []map[string]any) (string, map[string]ColumnDef) {
	stats := map[string]*columnStat{}
	total := len(docs)

	for _, doc := range docs {
		for field, raw := range doc {
			stat := stats[field]
			if stat == nil {
				stat = &columnStat{}
				stats[field] = stat
			}
			kind, strlen := inferKind(raw)
			stat.kind = widenKind(stat.kind, kind)
			if strlen > stat.maxLength {
				stat.maxLength = strlen
			}
			stat.presentCount++
			if raw != nil {
				stat.nonNullCount++
			}
		}
	}

	columnDefs := map[string]ColumnDef{}
	columns := make([]string, 0, len(stats)+2)
	columns = append(columns, "    id SERIAL PRIMARY KEY")

	keys := make([]string, 0, len(stats))
	for k := range stats {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, field := range keys {
		stat := stats[field]
		sqlType := sqlTypeForStat(stat)
		required := stat.presentCount == total && stat.nonNullCount == stat.presentCount && total > 0
		columnDefs[field] = ColumnDef{Name: field, Type: sqlType, Required: required}
		nullability := "NULL"
		if required {
			nullability = "NOT NULL"
		}
		columns = append(columns, fmt.Sprintf("    %s %s %s", quoteIdentifier(field), sqlType, nullability))
	}

	columns = append(columns, "    created_at TIMESTAMPTZ DEFAULT NOW()")
	ddl := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n%s\n);", quoteIdentifier(table), strings.Join(columns, ",\n"))
	return ddl, columnDefs
}

// SuggestMongoIndexes proposes indexes based on ID-like fields.
func SuggestMongoIndexes(docs []map[string]any) []MongoIndex {
	freq := map[string]int{}
	for _, doc := range docs {
		for field := range doc {
			if looksLikeForeignKey(field) || field == "id" {
				freq[field]++
			}
		}
	}
	threshold := len(docs) / 2
	indexes := make([]MongoIndex, 0)
	for field, count := range freq {
		if len(docs) == 0 || count > threshold {
			indexes = append(indexes, MongoIndex{Fields: []string{field}})
		}
	}
	sort.Slice(indexes, func(i, j int) bool {
		return indexes[i].Fields[0] < indexes[j].Fields[0]
	})
	return indexes
}

func inferKind(value any) (columnKind, int) {
	switch v := value.(type) {
	case nil:
		return kindUnknown, 0
	case bool:
		return kindBoolean, 0
	case json.Number:
		if strings.Contains(v.String(), ".") {
			return kindDouble, 0
		}
		return kindInteger, 0
	case float32, float64:
		return kindDouble, 0
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64:
		return kindInteger, 0
	case string:
		return kindString, len(v)
	case []any, map[string]any:
		return kindJSONB, 0
	default:
		return kindJSONB, 0
	}
}

func widenKind(current, candidate columnKind) columnKind {
	if current == kindUnknown {
		return candidate
	}
	if candidate == kindUnknown {
		return current
	}
	if current == candidate {
		return current
	}
	if (current == kindInteger && candidate == kindDouble) || (current == kindDouble && candidate == kindInteger) {
		return kindDouble
	}
	if current == kindString && candidate == kindString {
		return kindString
	}
	if current == kindBoolean && candidate == kindBoolean {
		return kindBoolean
	}
	// If incompatible, fall back to JSONB.
	return kindJSONB
}

func sqlTypeForStat(stat *columnStat) string {
	switch stat.kind {
	case kindInteger:
		return "BIGINT"
	case kindDouble:
		return "DOUBLE PRECISION"
	case kindBoolean:
		return "BOOLEAN"
	case kindString:
		if stat.maxLength > 0 && stat.maxLength <= 512 {
			return fmt.Sprintf("VARCHAR(%d)", max(stat.maxLength, 32))
		}
		return "TEXT"
	default:
		return "JSONB"
	}
}
