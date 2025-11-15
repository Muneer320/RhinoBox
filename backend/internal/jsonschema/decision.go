package jsonschema

import (
	"fmt"
	"sort"
	"strings"
)

// Decision encapsulates where the JSON payload should live.
type Decision struct {
	Engine  string  `json:"engine"`
	Reason  string  `json:"reason"`
	Schema  string  `json:"schema_sql,omitempty"`
	Summary Summary `json:"summary"`
	Table   string  `json:"table"`
}

// DecideStorage picks SQL vs NoSQL and produces optional schema DDL.
func DecideStorage(namespace string, summary Summary) Decision {
	score := summary.FieldStability + summary.TypeStability
	maxDepthOK := summary.MaxDepth <= 4
	engine := "nosql"
	reason := "high variance or nested structures"
	if !summary.HasArrayObjects && maxDepthOK && score >= 1.2 {
		engine = "sql"
		reason = fmt.Sprintf("stable schema score %.2f", score)
	}

	table := sanitizeIdentifier(namespace)
	if table == "" {
		table = "dataset"
	}

	decision := Decision{
		Engine:  engine,
		Reason:  reason,
		Summary: summary,
		Table:   table,
	}

	if engine == "sql" {
		decision.Schema = buildCreateTable(table, summary)
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
