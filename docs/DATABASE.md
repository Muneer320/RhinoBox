# JSON storage

`POST /ingest/json` and unified ingestion share the same persistence path. Responses identify the recommendation (`decision.engine`), actual backend, inserted count, archived count, destination and optional warnings. No database URL means NDJSON-only mode for that engine; a configured but unreachable database fails startup.

For SQL, the schema generator quotes original field names, infers column types and avoids adding a second `id` or `created_at` when the input supplies them. A hash of the inferred column definitions selects a table version, so changed types/fields do not silently overwrite an earlier schema. Creation and insertion share a PostgreSQL transaction and a per-table advisory lock. `schema_created` is true only when that transaction creates a new table.

MongoDB stores routed documents in database `rhinobox`. Integral JSON numbers are converted to BSON int64; integers outside that range are rejected instead of rounded. Fractional/exponent numbers use BSON doubles. Unordered insertion can commit a subset: the response reports the inserted count and a warning. Inspect warnings before deciding to retry.

Each accepted batch also produces an indexed NDJSON archive. Archive deduplication does not make database writes idempotent. Deleting its file does not remove database rows/documents. The filesystem and databases are separate stores, so an archive failure after a committed database write is reported without pretending the database rolled back.

Use dedicated local databases. Compose retains PostgreSQL, MongoDB and file data in named volumes. Pool size defaults to 10. There is no automatic migration of older root `json/` batches and no verified throughput guarantee.
