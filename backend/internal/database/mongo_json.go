package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/mongo/options"
)

// InsertJSON returns the number of committed documents even on a partial failure.
func (db *MongoDB) InsertJSON(ctx context.Context, database, collection string, docs []map[string]any) (int, error) {
	values := make([]any, len(docs))
	for i, doc := range docs {
		value, err := bsonJSONValue(doc)
		if err != nil {
			return 0, err
		}
		values[i] = value
	}
	result, err := db.client.Database(database).Collection(collection).InsertMany(ctx, values, options.InsertMany().SetOrdered(false))
	count := 0
	if result != nil {
		count = len(result.InsertedIDs)
	}
	return count, err
}

func bsonJSONValue(value any) (any, error) {
	switch v := value.(type) {
	case json.Number:
		if strings.ContainsAny(v.String(), ".eE") {
			number, err := v.Float64()
			if err != nil {
				return nil, fmt.Errorf("JSON number cannot be represented as BSON double: %w", err)
			}
			return number, nil
		}
		number, err := v.Int64()
		if err != nil {
			return nil, fmt.Errorf("JSON integer is outside BSON int64 range: %w", err)
		}
		return number, nil
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			converted, err := bsonJSONValue(item)
			if err != nil {
				return nil, err
			}
			out[key] = converted
		}
		return out, nil
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			converted, err := bsonJSONValue(item)
			if err != nil {
				return nil, err
			}
			out[i] = converted
		}
		return out, nil
	default:
		return value, nil
	}
}
