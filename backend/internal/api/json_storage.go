package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Muneer320/RhinoBox/internal/jsonschema"
)

// persistJSON distinguishes the routing recommendation from the storage actually used.
// Without the chosen database configured, NDJSON remains a usable local archive.
func (s *Server) persistJSON(ctx context.Context, namespace, comment string, docs []map[string]any) (JSONResult, error) {
	if len(docs) == 0 {
		return JSONResult{}, fmt.Errorf("no JSON documents provided")
	}
	analyzer := jsonschema.NewAnalyzer(4, 256)
	analyzer.AnalyzeBatch(docs)
	summary := analyzer.BuildSummary()
	analysis := jsonschema.IncorporateCommentHints(analyzer.AnalyzeStructure(docs, summary), comment)
	decision := jsonschema.DecideStorage(namespace, docs, summary, analysis)
	result := JSONResult{StorageType: decision.Engine, StorageBackend: "ndjson", TableOrCollection: decision.Table}
	if decision.Engine == "sql" {
		// Different inferred schemas get separate tables, including wider strings.
		// This avoids silently dropping fields or coercing a later batch into an old schema.
		signature, err := json.Marshal(decision.Columns)
		if err != nil {
			return JSONResult{}, err
		}
		suffix := fmt.Sprintf("%x", sha256.Sum256(signature))[:12]
		decision.Table = decision.Table[:min(40, len(decision.Table))] + "_" + suffix
		decision.Schema, decision.Columns = jsonschema.GeneratePostgresSchema(decision.Table, docs)
		columns := make([]string, 0, len(decision.Columns))
		for name := range decision.Columns {
			if name == "" || len(name) > 63 || strings.ContainsRune(name, 0) {
				return JSONResult{}, fmt.Errorf("SQL field names must contain 1–63 bytes and no NUL")
			}
			columns = append(columns, name)
		}
		if s.postgres != nil {
			created, err := s.postgres.StoreJSON(ctx, decision.Table, decision.Schema, columns, docs)
			if err != nil {
				return JSONResult{}, fmt.Errorf("PostgreSQL ingest: %w", err)
			}
			result.StorageBackend = "postgres"
			result.RecordsInserted = len(docs)
			result.SchemaCreated = created
		}
	} else if s.mongo != nil {
		inserted, err := s.mongo.InsertJSON(ctx, "rhinobox", decision.Table, docs)
		if err != nil {
			if inserted == 0 {
				return JSONResult{}, fmt.Errorf("MongoDB ingest: %w", err)
			}
			result.Warnings = append(result.Warnings, fmt.Sprintf("Only %d of %d documents were inserted; retrying the whole batch may duplicate committed documents.", inserted, len(docs)))
		}
		result.StorageBackend = "mongodb"
		result.Database = "rhinobox"
		result.RecordsInserted = inserted
	}
	result.TableOrCollection = decision.Table
	result.Decision = decision
	archive, err := s.storage.StoreJSONArchive(decision.Engine, namespace, result.StorageBackend, decision.Table, docs)
	if err != nil {
		if result.RecordsInserted == 0 {
			return JSONResult{}, fmt.Errorf("archive JSON: %w", err)
		}
		result.Warnings = append(result.Warnings, "Database write committed; NDJSON archive failed.")
		s.logger.Error("archive JSON after database commit", "err", err)
	} else {
		result.BatchPath = archive.Metadata.StoredPath
		result.ArchiveHash = archive.Metadata.Hash
		result.RecordsArchived = len(docs)
	}
	if decision.Engine == "sql" {
		schema := map[string]any{"table": decision.Table, "ddl": decision.Schema, "columns": decision.Columns}
		var err error
		result.SchemaPath, err = s.storage.WriteJSONFile(filepath.Join("json", "sql", decision.Table, "schema.json"), schema)
		if err != nil {
			result.Warnings = append(result.Warnings, "Schema sidecar could not be written.")
			s.logger.Error("write schema sidecar", "err", err)
		}
	}
	return result, nil
}
