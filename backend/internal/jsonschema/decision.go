package jsonschema

import (
	"fmt"
	"sort"
	"strings"
)

// Decision encapsulates where the JSON payload should live.
type Decision struct {
	Engine     string               `json:"engine"`
	Reason     string               `json:"reason"`
	Confidence float64              `json:"confidence"`
	Schema     string               `json:"schema_sql,omitempty"`
	Columns    map[string]ColumnDef `json:"columns,omitempty"`
	Indexes    []MongoIndex         `json:"indexes,omitempty"`
	Summary    Summary              `json:"summary"`
	Analysis   SchemaAnalysis       `json:"analysis"`
	Table      string               `json:"table"`
}

// DecideStorage picks SQL vs NoSQL and produces optional schema DDL plus metadata.
func DecideStorage(namespace string, docs []map[string]any, summary Summary, analysis SchemaAnalysis) Decision {
	score := 0.0
	sqlReasons := make([]string, 0)
	nosqlReasons := make([]string, 0)

	if analysis.HasForeignKeys || analysis.HasRelationships {
		score += 1.0
		sqlReasons = append(sqlReasons, "foreign keys/relationships present")
	}
	if analysis.RequiresJoins {
		score += 1.0
		sqlReasons = append(sqlReasons, "joins hinted")
	}
	if analysis.SchemaConsistency > 0.8 {
		score += 0.5
		sqlReasons = append(sqlReasons, fmt.Sprintf("schema consistency %.2f", analysis.SchemaConsistency))
	}
	if analysis.MaxNestingDepth <= 2 {
		score += 0.3
	}

	if analysis.MaxNestingDepth > 3 {
		score -= 1.0
		nosqlReasons = append(nosqlReasons, "deep nesting")
	}
	if analysis.SchemaConsistency < 0.5 {
		score -= 1.0
		nosqlReasons = append(nosqlReasons, "low consistency")
	}
	if ratio := float64(analysis.UniqueFieldSets) / float64(max(1, analysis.RecordCount)); ratio > 0.3 {
		score -= 0.8
		nosqlReasons = append(nosqlReasons, "high field variation")
	}
	if analysis.ExpectedWriteLoad == "high" {
		score -= 0.5
		nosqlReasons = append(nosqlReasons, "high write load hint")
	}

	engine := "nosql"
	confidence := -score
	reason := strings.Join(nosqlReasons, "; ")
	if reason == "" {
		reason = "optimized for flexible schema"
	}
	if score > 0.5 {
		engine = "sql"
		confidence = score
		reason = strings.Join(sqlReasons, "; ")
		if reason == "" {
			reason = "stable relational signals"
		}
	}
	reason = fmt.Sprintf("%s (score %.2f)", reason, score)

	table := sanitizeIdentifier(namespace)
	if table == "" {
		table = "dataset"
	}

	decision := Decision{
		Engine:     engine,
		Reason:     reason,
		Confidence: confidence,
		Summary:    summary,
		Analysis:   analysis,
		Table:      table,
	}

	if engine == "sql" {
		schema, cols := GeneratePostgresSchema(table, docs)
		decision.Schema = schema
		decision.Columns = cols
	} else {
		decision.Indexes = SuggestMongoIndexes(docs)
	}

	return decision
}

func buildCreateTable(table string, summary Summary) string {
	columns := make([]string, 0)
	for path, field := range summary.Fields {
		if strings.Contains(path, ".") || strings.Contains(path, "[]") {
			continue
		}
		columns = append(columns, fmt.Sprintf("    %s %s %s", quoteIdentifier(path), columnType(field), nullability(field)))
	}
	if len(columns) == 0 {
		columns = append(columns, "    payload JSONB NOT NULL")
	}
	sort.Strings(columns)
	columns = append(columns, "    recorded_at TIMESTAMPTZ DEFAULT NOW()")
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n%s\n);", quoteIdentifier(table), strings.Join(columns, ",\n"))
}

func columnType(field FieldSummary) string {
	switch field.DominantType {
	case Integer:
		return "BIGINT"
	case Float:
		return "DOUBLE PRECISION"
	case Boolean:
		return "BOOLEAN"
	case String:
		if field.MaxLength > 0 && field.MaxLength <= 512 {
			return fmt.Sprintf("VARCHAR(%d)", field.MaxLength)
		}
		return "TEXT"
	default:
		return "JSONB"
	}
}

func nullability(field FieldSummary) string {
	if field.NullFraction < 0.2 {
		return "NOT NULL"
	}
	return "NULL"
}

func quoteIdentifier(id string) string {
	return fmt.Sprintf("\"%s\"", sanitizeIdentifier(id))
}

func sanitizeIdentifier(id string) string {
	id = strings.ToLower(id)
	var b strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else if r == '-' || r == ' ' {
			b.WriteRune('_')
		}
	}
	return strings.Trim(b.String(), "_")
}
