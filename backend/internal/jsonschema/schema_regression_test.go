package jsonschema

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGeneratedSchemaPreservesInputIDAndCreatedAt(t *testing.T) {
	ddl, columns := GeneratePostgresSchema("events", []map[string]any{{"id": json.Number("17"), "created_at": "yesterday", "quote\"name": true}})
	if strings.Contains(ddl, "SERIAL") || strings.Contains(ddl, "DEFAULT NOW()") {
		t.Fatalf("generated duplicate input columns: %s", ddl)
	}
	if columns["id"].Type != "BIGINT" || columns["created_at"].Type != "VARCHAR(32)" {
		t.Fatalf("lost input types: %#v", columns)
	}
	if !strings.Contains(ddl, `"quote""name"`) {
		t.Fatalf("identifier was not quoted: %s", ddl)
	}
}
