# RhinoBox API Reference

Complete reference for all RhinoBox HTTP endpoints.

## Base URL

```
http://localhost:8090
```

Configure via `RHINOBOX_ADDR` environment variable (default `:8090`).

---

## Endpoints Overview

| Method | Endpoint              | Purpose                                        |
| ------ | --------------------- | ---------------------------------------------- |
| GET    | `/healthz`            | Health check probe                             |
| POST   | `/ingest`             | **Unified ingestion** - handles all data types |
| POST   | `/ingest/media`       | Media-specific ingestion                       |
| POST   | `/ingest/json`        | JSON-specific ingestion                        |
| PATCH  | `/files/{file_id}/move` | Move/recategorize a single file              |
| PATCH  | `/files/batch/move`   | Batch move/recategorize multiple files         |

---

## GET `/healthz`

Health check endpoint for monitoring and load balancers.

### Response

```json
{
  "status": "ok",
  "time": "2025-11-15T10:30:00Z"
}
```

### Example

```bash
curl http://localhost:8090/healthz
```

---

## POST `/ingest`

**Unified intelligent ingestion endpoint** - single entry point for all data types.

Automatically routes media files, JSON data, and generic files to appropriate processing pipelines.

### Content-Type

`multipart/form-data`

### Form Fields

| Field       | Type        | Required | Description                                    |
| ----------- | ----------- | -------- | ---------------------------------------------- |
| `files`     | File(s)     | No     | One or more files (media, documents, archives) |
| `data`      | JSON string | No     | Inline JSON data (object or array)             |
| `namespace` | string      | No       | Organization/category namespace                |
| `comment`   | string      | No       | Hints for categorization or decision engine    |
| `metadata`  | JSON string | No       | Additional context (tags, source, description) |

At least one of `files` or `data` must be provided.

### Response Schema

```json
{
  "job_id": "job_1731687000000000",
  "status": "completed",
  "results": {
    "media": [
      {
        "original_name": "photo.jpg",
        "stored_path": "media/images/vacation/photo_<uuid>.jpg",
        "category": "vacation",
        "mime_type": "image/jpeg",
        "size": 2048576,
        "hash": "",
        "duplicates": false,
        "metadata": {}
      }
    ],
    "json": [
      {
        "storage_type": "sql",
        "database": "",
        "table_or_collection": "orders",
        "records_inserted": 3,
        "schema_created": true,
        "relationships_detected": [],
        "decision": {
          "engine": "sql",
          "reason": "...",
          "confidence": 1.0,
          "schema_sql": "CREATE TABLE ...",
          "table": "orders"
        },
        "batch_path": "json/sql/orders/batch_20251115.ndjson"
      }
    ],
    "files": [
      {
        "original_name": "report.pdf",
        "stored_path": "files/docs/report_<uuid>.pdf",
        "file_type": "application/pdf",
        "size": 512000,
        "hash": ""
      }
    ]
  },
  "timing": {
    "total_ms": 125,
    "processing_ms": 45,
    "json_ms": 80
  },
  "errors": []
}
```

### Routing Logic

Files are automatically routed based on MIME type (with extension fallback):

- **Media** (`image/*`, `video/*`, `audio/*`) → Media pipeline with categorization
- **JSON data** (from `data` field) → JSON decision engine (SQL vs NoSQL)
- **Generic files** (PDFs, documents, etc.) → Generic file storage

### Examples

#### Mixed Upload

```bash
curl -X POST http://localhost:8090/ingest \
  -F "files=@photo.jpg" \
  -F "files=@document.pdf" \
  -F 'data=[{"order_id":101,"total":42.50}]' \
  -F "namespace=orders" \
  -F "comment=mixed batch"
```

#### Batch Media

```bash
curl -X POST http://localhost:8090/ingest \
  -F "files=@img1.jpg" \
  -F "files=@img2.jpg" \
  -F "files=@video.mp4" \
  -F "comment=family photos"
```

#### JSON Only

```bash
curl -X POST http://localhost:8090/ingest \
  -F 'data=[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]' \
  -F "namespace=users"
```

#### PowerShell

```powershell
$form = @{
    files = Get-Item "photo.jpg"
    namespace = "gallery"
    comment = "demo"
}
Invoke-RestMethod -Uri http://localhost:8090/ingest -Method Post -Form $form
```

### Error Handling

**Partial success** (HTTP 200):

```json
{
  "job_id": "job_123",
  "status": "completed",
  "results": { ... },
  "errors": ["invalid.xyz: unsupported file type"]
}
```

**Complete failure** (HTTP 400):

```json
{
  "error": "all items failed: [file1: error, file2: error]"
}
```

---

## POST `/ingest/media`

**Media-specific ingestion endpoint** for images, videos, and audio files.

### Content-Type

`multipart/form-data`

### Form Fields

| Field      | Type    | Required | Description                           |
| ---------- | ------- | -------- | ------------------------------------- |
| `file`     | File(s) | Yes      | One or more media files               |
| `category` | string  | No       | Category hint for organization        |
| `comment`  | string  | No       | Additional context for categorization |

### Response Schema

```json
{
  "stored": [
    {
      "path": "media/images/wildlife/cat_<uuid>.jpg",
      "mime_type": "image/jpeg",
      "media_type": "images",
      "category": "wildlife",
      "comment": "demo upload",
      "original_name": "cat.jpg",
      "uploaded_at": "2025-11-15T10:30:00Z"
    }
  ]
}
```

### Categorization

- **MIME-based detection**: `image/`, `video/`, `audio/` prefixes
- **Fallback to extension**: `.jpg`, `.mp4`, `.mp3`, etc.
- **Directory structure**: `media/<media_type>/<category>/<uuid>_<original_name>`

### Examples

#### Single Upload

```bash
curl -X POST http://localhost:8090/ingest/media \
  -F "file=@cat.png" \
  -F "category=wildlife" \
  -F "comment=demo upload"
```

#### Batch Upload

```bash
curl -X POST http://localhost:8090/ingest/media \
  -F "file=@photo1.jpg" \
  -F "file=@photo2.jpg" \
  -F "file=@video.mp4" \
  -F "category=vacation" \
  -F "comment=summer trip"
```

#### PowerShell

```powershell
$form = @{
    file = Get-Item "cat.jpg"
    category = "pets"
    comment = "my cat"
}
Invoke-RestMethod -Uri http://localhost:8090/ingest/media -Method Post -Form $form
```

### Error Responses

**No files** (HTTP 400):

```json
{
  "error": "no files provided"
}
```

**Invalid multipart** (HTTP 400):

```json
{
  "error": "invalid multipart payload: ..."
}
```

---

## POST `/ingest/json`

**JSON-specific ingestion endpoint** with intelligent SQL vs NoSQL decision engine.

### Content-Type

`application/json`

### Request Schema

```json
{
  "document": {...},           // Single document (alternative to documents)
  "documents": [{...}],        // Array of documents
  "namespace": "string",       // Required: collection/table name
  "comment": "string",         // Optional: hints for decision engine
  "metadata": {...}            // Optional: additional context
}
```

**Note**: Provide either `document` or `documents`, not both.

### Response Schema

```json
{
  "decision": {
    "engine": "sql",
    "reason": "foreign keys/relationships present; schema consistency 1.00 (score 1.00)",
    "confidence": 1.0,
    "schema_sql": "CREATE TABLE IF NOT EXISTS \"orders\" (...)",
    "columns": {
      "order_id": {
        "name": "order_id",
        "type": "BIGINT",
        "required": true
      },
      ...
    },
    "summary": {
      "DocumentsAnalyzed": 3,
      "TotalFields": 6,
      "MaxDepth": 1,
      "FieldStability": 1.0,
      "TypeStability": 1.0,
      ...
    },
    "analysis": {
      "has_foreign_keys": true,
      "schema_consistency": 1.0,
      "max_nesting_depth": 1,
      ...
    },
    "table": "orders"
  },
  "batch_path": "json/sql/orders/batch_20251115T110315Z.ndjson",
  "schema_path": "json/sql/orders/schema.json",
  "documents": 3
}
```

### Decision Engine

Automatically chooses between SQL and NoSQL based on:

#### SQL Decision Factors

- Foreign key patterns (`*_id` fields)
- Stable, consistent schema across documents
- Shallow nesting (depth ≤ 2)
- High type stability (all docs use same types)
- Relational structure

#### NoSQL Decision Factors

- Deep nesting (depth > 3)
- High field variation between documents
- Array/object-heavy structure
- Schema inconsistency
- Comment hints: "flexible", "nosql", "unstructured"

### Examples

#### SQL-routed Example

```bash
curl -X POST http://localhost:8090/ingest/json \
  -H "Content-Type: application/json" \
  -d '{
    "namespace": "orders",
    "documents": [
      {"order_id": 1, "customer_id": 10, "total": 100.0},
      {"order_id": 2, "customer_id": 11, "total": 200.0}
    ]
  }'
```

#### NoSQL-routed Example

```bash
curl -X POST http://localhost:8090/ingest/json \
  -H "Content-Type: application/json" \
  -d '{
    "namespace": "activity",
    "comment": "flexible schema nosql",
    "documents": [
      {
        "user": {"id": "u1", "name": "Alice"},
        "events": [
          {"type": "click", "meta": {"device": "ios"}}
        ]
      }
    ]
  }'
```

#### PowerShell

```powershell
$payload = @{
    namespace = "inventory"
    documents = @(
        @{ sku = "A-100"; qty = 42; price = 19.99 }
        @{ sku = "B-200"; qty = 10; price = 4.25 }
    )
} | ConvertTo-Json -Depth 10

Invoke-RestMethod -Uri http://localhost:8090/ingest/json `
    -Method Post `
    -ContentType "application/json" `
    -Body $payload
```

### Storage Paths

**SQL decisions:**

- Batch: `data/json/sql/<namespace>/batch_<timestamp>.ndjson`
- Schema: `data/json/sql/<table>/schema.json`

**NoSQL decisions:**

- Batch: `data/json/nosql/<namespace>/batch_<timestamp>.ndjson`

All ingestions logged to: `data/json/ingest_log.ndjson`

### Error Responses

**No documents** (HTTP 400):

```json
{
  "error": "no JSON documents provided"
}
```

**Invalid JSON** (HTTP 400):

```json
{
  "error": "invalid JSON: ..."
}
```

---

## Configuration

Environment variables:

| Variable                 | Default  | Description                          |
| ------------------------ | -------- | ------------------------------------ |
| `RHINOBOX_ADDR`          | `:8090`  | HTTP bind address                    |
| `RHINOBOX_DATA_DIR`      | `./data` | Root directory for storage           |
| `RHINOBOX_MAX_UPLOAD_MB` | `512`    | Maximum upload size per request (MB) |

### Example

```bash
export RHINOBOX_ADDR=":8080"
export RHINOBOX_DATA_DIR="/var/rhinobox/data"
export RHINOBOX_MAX_UPLOAD_MB="1024"
go run ./cmd/rhinobox
```

---

## Storage Structure

```
data/
├── media/
│   ├── <media_type>/
│   │   └── <category>/
│   │       └── <filename>_<uuid><ext>
│   └── ingest_log.ndjson
├── json/
│   ├── sql/
│   │   └── <namespace>/
│   │       ├── batch_<timestamp>.ndjson
│   │       └── schema.json
│   ├── nosql/
│   │   └── <namespace>/
│   │       └── batch_<timestamp>.ndjson
│   └── ingest_log.ndjson
└── files/
    └── <namespace>/
        └── <filename>_<uuid><ext>
```

---

## Rate Limits

Currently no rate limiting implemented. Configure via reverse proxy (nginx, Caddy) if needed.

---

## CORS

CORS headers not configured by default. Add middleware if serving a web frontend:

```go
r.Use(cors.Handler(cors.Options{
    AllowedOrigins: []string{"https://example.com"},
    AllowedMethods: []string{"GET", "POST"},
}))
```

---

## PATCH `/files/{file_id}/move`

Move or recategorize a single file to a new location while maintaining metadata integrity.

### URL Parameters

- `file_id` - File hash or path to identify the file

### Request Body

```json
{
  "new_category": "images/jpg/vacation/2025",
  "reason": "better organization"
}
```

### Fields

- `new_category` **(required)** - Target category path (e.g., `images/png`, `documents/pdf/reports`)
- `reason` - Optional reason for the move (for audit logging)

### Response

```json
{
  "status": "success",
  "old_path": "storage/images/jpg/ffab59709a62_photo.jpg",
  "new_path": "storage/images/jpg/vacation/2025/ffab59709a62_photo.jpg",
  "old_category": "images/jpg",
  "new_category": "images/jpg/vacation/2025",
  "renamed": false,
  "metadata": {
    "hash": "ffab59709a62da7db343c05c5e11ca7a3e9d250157da0955d45c83309550ab04",
    "original_name": "photo.jpg",
    "stored_path": "storage/images/jpg/vacation/2025/ffab59709a62_photo.jpg",
    "category": "images/jpg/vacation/2025",
    "mime_type": "image/jpeg",
    "size": 2048,
    "uploaded_at": "2025-11-15T10:00:00Z",
    "metadata": {
      "move_reason": "better organization",
      "moved_at": "2025-11-15T12:30:00Z",
      "moved_from": "storage/images/jpg/ffab59709a62_photo.jpg"
    }
  }
}
```

### Behavior

- **File Identification**: Accepts either file hash or file path as `file_id`
- **Atomic Operation**: Move is transactional; rolls back on any error
- **Conflict Resolution**: If target filename exists, file is renamed with timestamp suffix
- **Metadata Preservation**: Hash, original name, upload date, and size remain unchanged
- **Move Tracking**: Adds `move_reason`, `moved_at`, and `moved_from` to metadata
- **Directory Creation**: Creates target directories if they don't exist
- **Cleanup**: Removes empty source directories after successful move
- **Logging**: Appends move operation to `media/move_log.ndjson`

### Examples

**Move by hash:**

```bash
curl -X PATCH http://localhost:8090/files/ffab59709a62/move \
  -H "Content-Type: application/json" \
  -d '{
    "new_category": "images/jpg/archive/2024",
    "reason": "yearly archival"
  }'
```

**Move with deep nesting:**

```bash
curl -X PATCH http://localhost:8090/files/abc12345/move \
  -H "Content-Type: application/json" \
  -d '{
    "new_category": "documents/pdf/clients/acme/2025/q4",
    "reason": "client organization"
  }'
```

### Error Responses

**File not found:**

```json
{
  "error": "move file: file not found"
}
```

**Invalid category:**

```json
{
  "error": "move file: new category is required"
}
```

---

## PATCH `/files/batch/move`

Move multiple files in a single atomic transaction. All moves succeed or all fail (rollback).

### Request Body

```json
{
  "files": [
    {
      "hash": "abc123",
      "new_category": "images/png/archive",
      "reason": "cleanup"
    },
    {
      "hash": "def456",
      "new_category": "videos/mp4/processed",
      "reason": "cleanup"
    }
  ]
}
```

### Fields

- `files` **(required)** - Array of move requests
  - `hash` or `path` - File identifier (at least one required)
  - `new_category` **(required)** - Target category
  - `reason` - Optional move reason

### Response

```json
{
  "status": "success",
  "success": 2,
  "failed": 0,
  "results": [
    {
      "old_path": "storage/images/jpg/abc_photo1.jpg",
      "new_path": "storage/images/png/archive/abc_photo1.jpg",
      "old_category": "images/jpg",
      "new_category": "images/png/archive",
      "renamed": false,
      "metadata": { ... }
    },
    {
      "old_path": "storage/videos/mp4/def_video.mp4",
      "new_path": "storage/videos/mp4/processed/def_video.mp4",
      "old_category": "videos/mp4",
      "new_category": "videos/mp4/processed",
      "renamed": false,
      "metadata": { ... }
    }
  ]
}
```

### Behavior

- **Atomic Transaction**: All files move successfully or none do (automatic rollback)
- **Order Preserved**: Results returned in same order as request
- **Same Features**: All single-file move features apply to each file
- **Batch Logging**: All moves logged together in `media/move_log.ndjson`

### Examples

**Batch reorganization:**

```bash
curl -X PATCH http://localhost:8090/files/batch/move \
  -H "Content-Type: application/json" \
  -d '{
    "files": [
      {"hash": "file1hash", "new_category": "archive/2024/january", "reason": "monthly archive"},
      {"hash": "file2hash", "new_category": "archive/2024/january", "reason": "monthly archive"},
      {"hash": "file3hash", "new_category": "archive/2024/january", "reason": "monthly archive"}
    ]
  }'
```

**Mixed file types:**

```bash
curl -X PATCH http://localhost:8090/files/batch/move \
  -H "Content-Type: application/json" \
  -d '{
    "files": [
      {"hash": "img1", "new_category": "project-x/images"},
      {"hash": "doc1", "new_category": "project-x/documents"},
      {"hash": "vid1", "new_category": "project-x/videos"}
    ]
  }'
```

### Error Responses

**Partial failure (all rolled back):**

```json
{
  "error": "batch move failed at file 1: file not found",
  "failed": 3,
  "errors": ["file 1: file not found"]
}
```

**Empty request:**

```json
{
  "error": "no files provided"
}
```

### Use Cases

1. **Yearly Archival**: Move all files from active categories to year-based archives
2. **Project Organization**: Group related files into project-specific categories
3. **Compliance**: Organize files by client, retention policy, or regulatory requirements
4. **Workflow Stages**: Move files through processing stages (raw → processed → archived)
5. **Correction**: Fix miscategorized files after automatic classification
6. **Integration**: Works with user-suggested routing (#15) to recategorize unrecognized types

---

## Testing

See `docs/JSON_TEST_PLAYBOOK.md` for JSON test fixtures and `docs/UNIFIED_INGEST.md` for comprehensive endpoint examples.

---

## Related Documentation

- [Unified Ingestion Guide](UNIFIED_INGEST.md) - Deep dive on `/ingest`
- [JSON Test Playbook](JSON_TEST_PLAYBOOK.md) - Sample payloads for testing
- [Docker Deployment](DOCKER.md) - Containerized deployment
- [Architecture Overview](ARCHITECTURE.md) - System design
