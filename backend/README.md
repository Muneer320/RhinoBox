# RhinoBox

RhinoBox is a hackathon-friendly, high-speed Go service that accepts any payload through one HTTP entry point and stores it intelligently in the local filesystem. Media assets are categorized and dropped into type-aware folders, while structured JSON is analyzed to decide whether it belongs in an auto-generated SQL schema or in a document-style collection.

## Features

- **Single ingestion API**: upload media (images, videos, audio, anything) and structured JSON through the same server.
- **On-the-fly media organization**: MIME-aware classifier groups assets (images/videos/audio/other) and keeps related uploads inside deterministic subdirectories. Optional `category` hints keep humans in the loop.
- **Schema-aware JSON handling**: a Go port of the MammothBox schema analyzer inspects each batch, computes stability metrics, and selects SQL vs. NoSQL storage automatically.
- **Filesystem-first**: everything lands under `./data`, making demos easy and keeping the footprint portable.

## Requirements

- Go 1.21+

## Getting Started

```pwsh
cd RhinoBox
setx RHINOBOX_ADDR ":8090" # optional override
# optional: setx RHINOBOX_DATA_DIR "C:\\tmp\\rhino-data"
go run ./cmd/rhinobox
```

The server exposes:

- `GET /healthz` — basic health probe.
- `POST /ingest/media` — multipart form upload (`file` parts, optional `category` + `comment`).
- `POST /ingest/json` — JSON body with either a single `document` or multiple `documents` plus optional metadata.
- `POST /ingest/async` — async unified ingestion (returns job ID immediately).
- `POST /ingest/media/async` — async media upload (background processing).
- `POST /ingest/json/async` — async JSON ingestion (queued processing).
- `GET /jobs` — list active and recent jobs.
- `GET /jobs/{job_id}` — check job status and progress.
- `GET /jobs/{job_id}/result` — get detailed job results.
- `DELETE /jobs/{job_id}` — cancel a job.
- `GET /jobs/stats` — queue statistics (pending, processing, completed, workers).

For full API documentation, see [API_REFERENCE.md](../docs/API_REFERENCE.md) and [ASYNC_API.md](../docs/ASYNC_API.md).

### Media Upload Example

```pwsh
curl -X POST http://localhost:8090/ingest/media `
  -F "file=@cats/sleepy.png" `
  -F "file=@videos/intro.mp4" `
  -F "category=wildlife" `
  -F "comment=demo run"
```

Response includes the resolved media folders and relative file paths under `data/media/...`.

### JSON Upload Example

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

RhinoBox will:

1. Analyze the batch for schema stability.
2. Decide SQL vs. NoSQL storage and emit a matching decision payload.
3. Append newline-delimited documents under `data/json/<engine>/<namespace>/...`.
4. For SQL decisions, generate a reusable `schema.json` with the inferred DDL.

### Configuration

- `RHINOBOX_ADDR` — HTTP bind address (default `:8090`).
- `RHINOBOX_DATA_DIR` — root for filesystem storage (default `./data`).
- `RHINOBOX_MAX_UPLOAD_MB` — multipart limit in MiB (default `512`).

### Observability

- Media ingestion log: `data/media/ingest_log.ndjson`
- JSON ingestion log: `data/json/ingest_log.ndjson`

Each log entry captures timestamps, chosen storage strategy, and any optional metadata/comments supplied during ingestion.

## Testing

Compilation + basic checks:

```pwsh
cd RhinoBox
go test ./...
```

No dedicated unit suite yet—the primary goal is to keep the hackathon demo loop tight. Plug RhinoBox behind the existing frontend when ready, or exercise the endpoints directly with `curl` or Postman.
