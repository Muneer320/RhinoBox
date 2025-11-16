package jsonschema

import (
	"encoding/json"
	"testing"
)

func TestDecideStorageSQLVsNoSQL(t *testing.T) {
	tests := []struct {
		name     string
		docs     []map[string]any
		expected string
	}{
		{
			name: "relational_data",
			docs: []map[string]any{
				{"id": jsonNumber("1"), "user_id": jsonNumber("10"), "amount": jsonNumber("100")},
				{"id": jsonNumber("2"), "user_id": jsonNumber("11"), "amount": jsonNumber("200")},
			},
			expected: "sql",
		},
		{
			name: "flexible_schema",
			docs: []map[string]any{
				{"name": "John", "age": jsonNumber("30")},
				{"name": "Jane", "city": "NYC", "hobbies": []any{"reading"}},
			},
			expected: "nosql",
		},
		{
			name: "simple_consistent_json",
			docs: []map[string]any{
				{"name": "Alice", "age": jsonNumber("25"), "email": "alice@example.com"},
				{"name": "Bob", "age": jsonNumber("30"), "email": "bob@example.com"},
				{"name": "Charlie", "age": jsonNumber("35"), "email": "charlie@example.com"},
			},
			expected: "sql",
		},
		{
			name: "simple_single_document",
			docs: []map[string]any{
				{"id": jsonNumber("1"), "title": "Test", "status": "active"},
			},
			expected: "sql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			an := NewAnalyzer(4, 256)
			an.AnalyzeBatch(tt.docs)
			summary := an.BuildSummary()
			analysis := an.AnalyzeStructure(tt.docs, summary)
			decision := DecideStorage("test", tt.docs, summary, analysis)
			if decision.Engine != tt.expected {
				t.Fatalf("expected %s, got %s", tt.expected, decision.Engine)
			}
		})
	}
}

func jsonNumber(v string) any {
	return json.Number(v)
}
