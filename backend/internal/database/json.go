package database

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

// StoreJSON creates the inferred table and inserts a complete batch atomically.
// PostgreSQL performs JSON-to-column conversion, preserving JSON numbers and nulls.
func (db *PostgresDB) StoreJSON(ctx context.Context, table, ddl string, columns []string, docs []map[string]any) (bool, error) {
	payload, err := json.Marshal(docs)
	if err != nil {
		return false, fmt.Errorf("encode documents: %w", err)
	}
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin batch: %w", err)
	}
	defer tx.Rollback(ctx)
	// Serialize first creation of a schema shared by concurrent ingest requests.
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", table); err != nil {
		return false, err
	}
	var existed bool
	if err := tx.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", pgx.Identifier{table}.Sanitize()).Scan(&existed); err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, ddl); err != nil {
		return false, fmt.Errorf("create inferred table: %w", err)
	}
	tableSQL := pgx.Identifier{table}.Sanitize()
	columns = append([]string(nil), columns...)
	sort.Strings(columns)
	if len(columns) == 0 {
		for range docs {
			if _, err := tx.Exec(ctx, "INSERT INTO "+tableSQL+" DEFAULT VALUES"); err != nil {
				return false, fmt.Errorf("insert document: %w", err)
			}
		}
	} else {
		quoted := make([]string, len(columns))
		for i, name := range columns {
			quoted[i] = pgx.Identifier{name}.Sanitize()
		}
		list := strings.Join(quoted, ", ")
		query := "INSERT INTO " + tableSQL + " (" + list + ") SELECT " + list + " FROM jsonb_populate_recordset(NULL::" + tableSQL + ", $1::jsonb)"
		if _, err := tx.Exec(ctx, query, string(payload)); err != nil {
			return false, fmt.Errorf("insert batch: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit batch: %w", err)
	}
	return !existed, nil
}
