package database

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresDB wraps a pgx connection pool with optimized batch operations
type PostgresDB struct {
	pool *pgxpool.Pool
}

// NewPostgresDB creates a new PostgreSQL connection pool with aggressive settings
// for high-throughput workloads (100K+ inserts/sec target)
func NewPostgresDB(ctx context.Context, connString string) (*PostgresDB, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Aggressive pooling for high throughput
	// MaxConns = 4x CPU cores for parallel INSERT/COPY operations
	config.MaxConns = int32(runtime.NumCPU() * 4)
	config.MinConns = int32(runtime.NumCPU())
	config.MaxConnIdleTime = 5 * time.Minute
	config.MaxConnLifetime = 1 * time.Hour
	config.HealthCheckPeriod = 1 * time.Minute

	// Statement cache for prepared statements (1024 cached statements)
	// Reduces parse overhead for repeated INSERT patterns
	config.ConnConfig.StatementCacheCapacity = 1024

	// Connect with timeout
	config.ConnConfig.ConnectTimeout = 10 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	// Verify connectivity
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}

	return &PostgresDB{pool: pool}, nil
}

// Close gracefully shuts down the connection pool
func (db *PostgresDB) Close() {
	db.pool.Close()
}

// Ping checks if the database is reachable
func (db *PostgresDB) Ping(ctx context.Context) error {
	return db.pool.Ping(ctx)
}

// Stats returns connection pool statistics
func (db *PostgresDB) Stats() *pgxpool.Stat {
	return db.pool.Stat()
}

// CreateTableFromSchema creates a PostgreSQL table from a schema definition
// Schema format: map[columnName]string where string is the SQL type (e.g., "BIGINT", "TEXT")
func (db *PostgresDB) CreateTableFromSchema(ctx context.Context, tableName string, schema map[string]string) error {
	// Build CREATE TABLE statement
	var columns []string
	for colName, colType := range schema {
		columns = append(columns, fmt.Sprintf("%q %s", colName, colType))
	}

	query := fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %q (%s)",
		tableName,
		strings.Join(columns, ", "),
	)

	_, err := db.pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("create table %s: %w", tableName, err)
	}

	return nil
}

// BatchInsertJSON inserts multiple JSON documents into a table with automatic batching
// Uses COPY for large batches (>100 docs) and multi-value INSERT for smaller batches
func (db *PostgresDB) BatchInsertJSON(ctx context.Context, table string, docs []map[string]any) error {
	if len(docs) == 0 {
		return nil
	}

	const batchSize = 1000

	for i := 0; i < len(docs); i += batchSize {
		end := min(i+batchSize, len(docs))
		batch := docs[i:end]

		// Use COPY for massive inserts (10-100x faster than INSERT)
		// COPY achieves near-disk-write-speed performance
		if len(batch) > 100 {
			if err := db.copyInsert(ctx, table, batch); err != nil {
				return fmt.Errorf("copy insert: %w", err)
			}
		} else {
			// Use multi-value INSERT for smaller batches
			if err := db.multiInsert(ctx, table, batch); err != nil {
				return fmt.Errorf("multi insert: %w", err)
			}
		}
	}

	return nil
}

// copyInsert uses PostgreSQL COPY protocol for ultra-fast bulk inserts
// Achieves 100K+ inserts/sec by bypassing query parsing and using binary protocol
func (db *PostgresDB) copyInsert(ctx context.Context, table string, docs []map[string]any) error {
	if len(docs) == 0 {
		return nil
	}

	// Extract column names from first document
	var columns []string
	for col := range docs[0] {
		columns = append(columns, col)
	}

	// Acquire a dedicated connection for COPY
	conn, err := db.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	// Build rows for COPY
	rows := make([][]any, len(docs))
	for i, doc := range docs {
		row := make([]any, len(columns))
		for j, col := range columns {
			row[j] = doc[col]
		}
		rows[i] = row
	}

	// Use COPY FROM for bulk insert
	_, err = conn.Conn().CopyFrom(
		ctx,
		pgx.Identifier{table},
		columns,
		pgx.CopyFromRows(rows),
	)

	if err != nil {
		return fmt.Errorf("copy from: %w", err)
	}

	return nil
}

// multiInsert uses multi-value INSERT for smaller batches
// INSERT INTO table (col1, col2) VALUES (v1, v2), (v3, v4), ...
func (db *PostgresDB) multiInsert(ctx context.Context, table string, docs []map[string]any) error {
	if len(docs) == 0 {
		return nil
	}

	// Extract column names from first document
	var columns []string
	for col := range docs[0] {
		columns = append(columns, col)
	}

	// Build multi-value INSERT statement
	var placeholders []string
	var values []any

	for i, doc := range docs {
		var rowPlaceholders []string
		for j, col := range columns {
			placeholder := fmt.Sprintf("$%d", i*len(columns)+j+1)
			rowPlaceholders = append(rowPlaceholders, placeholder)
			values = append(values, doc[col])
		}
		placeholders = append(placeholders, fmt.Sprintf("(%s)", strings.Join(rowPlaceholders, ", ")))
	}

	query := fmt.Sprintf(
		"INSERT INTO %q (%s) VALUES %s",
		table,
		strings.Join(quoteIdentifiers(columns), ", "),
		strings.Join(placeholders, ", "),
	)

	_, err := db.pool.Exec(ctx, query, values...)
	if err != nil {
		return fmt.Errorf("exec insert: %w", err)
	}

	return nil
}

// ExecuteSQL executes arbitrary SQL (for schema creation, DDL, etc.)
func (db *PostgresDB) ExecuteSQL(ctx context.Context, query string, args ...any) error {
	_, err := db.pool.Exec(ctx, query, args...)
	return err
}

// Query executes a SELECT query and returns rows
func (db *PostgresDB) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	return db.pool.Query(ctx, query, args...)
}

// quoteIdentifiers quotes SQL identifiers
func quoteIdentifiers(ids []string) []string {
	quoted := make([]string, len(ids))
	for i, id := range ids {
		quoted[i] = fmt.Sprintf("%q", id)
	}
	return quoted
}
