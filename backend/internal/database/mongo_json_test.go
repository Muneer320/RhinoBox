package database

import (
	"encoding/json"
	"testing"
)

func TestBSONJSONNumbersPreserveLargeIntegers(t *testing.T) {
	converted, err := bsonJSONValue(map[string]any{"nested": []any{json.Number("9007199254740993"), json.Number("1e3")}})
	if err != nil {
		t.Fatal(err)
	}
	values := converted.(map[string]any)["nested"].([]any)
	if values[0] != int64(9007199254740993) || values[1] != float64(1000) {
		t.Fatalf("numeric conversion changed values: %#v", values)
	}
	if _, err := bsonJSONValue(json.Number("9223372036854775808")); err == nil {
		t.Fatal("out-of-range integer must not be silently rounded")
	}
}
