## RhinoBox

RhinoBox solves **Problem Statement 2: Intelligent Multi-Modal Storage** by exposing a single Go 1.21 service that ingests both media files and JSON documents, categorizes them automatically, and writes everything to a hackathon-friendly filesystem layout.

---

### Architecture At A Glance

- `cmd/rhinobox/main.go` loads config, wires dependencies, and runs an HTTP server with signal-aware shutdown.
- `internal/api/server.go` hosts the Chi router plus the `/healthz`, `/ingest/media`, and `/ingest/json` handlers.
- `internal/media` sniffs MIME types + filenames to classify uploads and organize directories.
- `internal/jsonschema` reuses MammothBox heuristics to summarize document batches and pick SQL vs NoSQL.
- `internal/storage` bootstraps the directory tree, writes media streams, and appends NDJSON logs under `RHINOBOX_DATA_DIR` (default `./data`).

```
backend/
	cmd/rhinobox/main.go        # entrypoint
	internal/api/server.go      # HTTP routes + handlers
	internal/media/...          # MIME classification
	internal/jsonschema/...     # analyzer + storage decision
	internal/storage/local.go   # filesystem persistence
```

### End-to-End Flows

**Media uploads**

1. `/ingest/media` receives a multipart request with any number of `file` parts plus optional `category` and `comment` fields.
2. `Categorizer.Classify` derives a coarse media type (image/video/audio/other) and infers a category slug (or uses the hint).
3. `storage.StoreMedia` sanitizes names, applies a UUID suffix, creates directories under `data/media/<media_type>/<category>/`, streams file contents, and returns the relative path.
4. Every stored asset is appended as NDJSON to `data/media/ingest_log.ndjson` for traceability.

**JSON ingestion**

1. `/ingest/json` accepts either a single `document` or a `documents` array plus `namespace`, optional `comment`, and `metadata`.
2. `jsonschema.Analyzer` flattens fields (depth 4), tracks type stability, and builds a summary.
3. `jsonschema.DecideStorage` emits `Decision{Engine: "sql"|"nosql", Reason, Table, Schema}`.
4. `storage.AppendNDJSON` stores the batch under `data/json/<engine>/<namespace>/batch_*.ndjson` and for SQL decisions writes `schema.json` alongside the inferred DDL.
5. A log entry is appended to `data/json/ingest_log.ndjson` capturing the decision, namespace, and optional metadata.

### HTTP Surface

| Method | Path                     | Description                                                                                                            |
| ------ | ------------------------ | ---------------------------------------------------------------------------------------------------------------------- |
| GET    | `/healthz`               | Returns `{status:"ok", time:...}` for probes                                                                           |
| POST   | `/ingest`                | **Unified endpoint**: handles media, JSON, and generic files in single or mixed batches (see `docs/UNIFIED_INGEST.md`) |
| POST   | `/ingest/media`          | Multipart upload with one or more `file` parts, optional `category` + `comment`                                        |
| POST   | `/ingest/json`           | JSON body containing `document` or `documents`, `namespace`, optional `comment`, `metadata`                            |
| POST   | `/files/{file_id}/copy`  | Copy/duplicate a file with new metadata (see `docs/FILE_COPY_API.md`)                                                  |
| POST   | `/files/copy/batch`      | Batch copy multiple files in one request                                                                               |

### Sample Requests

**Unified Ingestion (Media + JSON)**

```pwsh
curl -X POST http://localhost:8090/ingest `
	-F "files=@photo.jpg" `
	-F "files=@document.pdf" `
	-F 'data=[{"order_id":101,"total":42.50}]' `
	-F "namespace=orders" `
	-F "comment=mixed batch"
```

**Media Only**

```pwsh
curl -X POST http://localhost:8090/ingest/media `
	-F "file=@samples/cat.png" `
	-F "file=@samples/demo.mp4" `
	-F "category=wildlife" `
	-F "comment=demo run"
```

**JSON Only**

```pwsh
curl -X POST http://localhost:8090/ingest/json `
	-H "Content-Type: application/json" `
	-d '{
		"namespace": "inventory",
		"comment": "evening batch",
		"documents": [
			{"sku": "A-100", "qty": 42, "price": 19.99},
			{"sku": "B-200", "qty": 10, "price": 4.25}
		]
	}'
```

### Configuration

- `RHINOBOX_ADDR` (default `:8090`) — HTTP bind address
- `RHINOBOX_DATA_DIR` (default `./data`) — root directory for media + JSON outputs
- `RHINOBOX_MAX_UPLOAD_MB` (default `512`) — multipart size cap; server converts to bytes internally

### Storage & Logs

```
data/
	media/
		<media_type>/<category>/<uuid>_<original>
		ingest_log.ndjson
	json/
		sql|nosql/<namespace>/batch_*.ndjson
		sql/<table>/schema.json
		ingest_log.ndjson
```

Every ingestion is captured as newline-delimited JSON for easy replay and analytics.

### Running & Testing

```pwsh
Set-Location backend
go run ./cmd/rhinobox

# optional compilation check
go test ./...
```

**Dockerized run** (more in `docs/DOCKER.md`):

```pwsh
Set-Location backend
docker build -t rhinobox-backend .
docker run --rm -it -p 8080:8090 -v ..\\rhino-data:/data rhinobox-backend
```

### Operational Notes

- Chi middleware provides structured logging and panic recovery out of the box.
- `http.Transport` is tuned with `MaxIdleConnsPerHost = 32` for smoother concurrent uploads.
- Graceful shutdown is handled via `context.WithCancel` around SIGINT/SIGTERM.
- All writes stay inside `RHINOBOX_DATA_DIR`, making it easy to mount/ship during demos.

### Roadmap / Next Steps

1. Back SQL decisions with a lightweight SQLite table writer and wire a document database (DuckDB/Badger) for NoSQL to satisfy end-to-end persistence.
2. Enhance analyzer to infer relationships/foreign keys when multiple collections arrive together.
3. Add automated integration tests (media + JSON fixtures) and a Postman/newman collection for hand-off validation.
4. Harden container image (health checks, distroless base) or add a Taskfile for repeatable local workflows.

Need another artifact (sample frontend hooks, deployment manifest, or load test script)? Open an issue, and RhinoBox will keep evolving.
