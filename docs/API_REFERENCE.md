# RhinoBox API Reference

Complete reference for all RhinoBox HTTP endpoints.

## Base URL

```
http://localhost:8090
```

Configure via `RHINOBOX_ADDR` environment variable (default `:8090`).

---

## Endpoints Overview

| Method | Endpoint                | Purpose                                        |
| ------ | ----------------------- | ---------------------------------------------- |
| GET    | `/healthz`              | Health check probe                             |
| POST   | `/ingest`               | **Unified ingestion** - handles all data types |
| POST   | `/ingest/media`         | Media-specific ingestion                       |
| POST   | `/ingest/json`          | JSON-specific ingestion                        |
| PATCH  | `/files/rename`         | Rename a file                                  |
| DELETE | `/files/{file_id}`      | Delete a file                                  |
| PATCH  | `/files/{file_id}/metadata` | Update file metadata                      |
| POST   | `/files/metadata/batch`  | Batch update file metadata                    |
| GET    | `/files/search`         | Search files by name                           |
| GET    | `/files/download`       | Download file by hash or path                  |
| GET    | `/files/metadata`       | Get file metadata without downloading         |
| GET    | `/files/stream`         | Stream file with range request support        |

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

## DELETE `/files/{file_id}`

**File deletion endpoint** - permanently removes a file and its metadata from the system.

### URL Parameters

| Parameter | Type   | Required | Description                    |
| --------- | ------ | -------- | ------------------------------ |
| `file_id` | string | Yes      | SHA-256 hash of the file (64 hex characters) |

### Response Schema

**Success** (HTTP 200):

```json
{
  "hash": "a1b2c3d4e5f6...",
  "original_name": "photo.jpg",
  "stored_path": "storage/images/jpg/category/a1b2c3d4e5f6_photo.jpg",
  "deleted": true,
  "deleted_at": "2025-11-15T10:30:00Z",
  "message": "deleted file photo.jpg"
}
```

### Behavior

- **Physical file deletion**: Removes the file from the filesystem
- **Metadata removal**: Removes the file entry from the metadata index
- **Audit logging**: Logs the deletion operation to `metadata/delete_log.ndjson`
- **Idempotent handling**: If the physical file is missing but metadata exists, deletion still succeeds (metadata-only cleanup)

### Error Responses

**File not found** (HTTP 404):

```json
{
  "error": "file not found: hash <hash>"
}
```

**Invalid input** (HTTP 400):

```json
{
  "error": "invalid input: hash is required"
}
```

**Missing file_id** (HTTP 400):

```json
{
  "error": "file_id is required"
}
```

**Internal server error** (HTTP 500):

```json
{
  "error": "delete failed: <error message>"
}
```

### Examples

#### Delete a File

```bash
curl -X DELETE http://localhost:8090/files/a1b2c3d4e5f6789012345678901234567890123456789012345678901234567890
```

#### PowerShell

```powershell
$hash = "a1b2c3d4e5f6789012345678901234567890123456789012345678901234567890"
Invoke-RestMethod -Uri "http://localhost:8090/files/$hash" -Method Delete
```

#### Python

```python
import requests

hash_value = "a1b2c3d4e5f6789012345678901234567890123456789012345678901234567890"
response = requests.delete(f"http://localhost:8090/files/{hash_value}")
print(response.json())
```

### Deletion Audit Log

All deletions are logged to `data/metadata/delete_log.ndjson` in newline-delimited JSON format:

```json
{
  "hash": "a1b2c3d4e5f6...",
  "original_name": "photo.jpg",
  "stored_path": "storage/images/jpg/category/a1b2c3d4e5f6_photo.jpg",
  "category": "images/jpg",
  "mime_type": "image/jpeg",
  "size": 2048576,
  "deleted_at": "2025-11-15T10:30:00Z"
}
```

### Notes

- Deletion is **permanent** and cannot be undone
- The file hash is returned from the upload/ingest endpoints
- If a file was manually deleted from the filesystem, the API will still remove the metadata entry
- Deletion operations are logged for audit purposes

---

## GET `/files/download`

Download a file by its hash or stored path.

### Query Parameters

| Parameter | Type   | Required | Description                    |
| --------- | ------ | -------- | ------------------------------ |
| `hash`    | string | No*      | SHA-256 hash of the file       |
| `path`    | string | No*      | Stored path of the file        |

\* Either `hash` or `path` must be provided.

### Response Headers

```
Content-Type: <mime-type>
Content-Length: <file-size>
Content-Disposition: attachment; filename="<original-name>"
ETag: "<file-hash>"
Last-Modified: <upload-date>
X-File-Category: <category>
X-File-Hash: <sha256>
Accept-Ranges: bytes
Cache-Control: private, max-age=3600
```

### Response

Returns the file content as binary data with appropriate MIME type.

### Examples

#### Download by Hash

```bash
curl -O -J "http://localhost:8090/files/download?hash=abc123def456..."
```

#### Download by Path

```bash
curl -O -J "http://localhost:8090/files/download?path=storage/images/jpg/vacation/abc123_photo.jpg"
```

### Error Responses

**File Not Found** (HTTP 404):

```json
{
  "error": "file not found: hash abc123..."
}
```

**Invalid Path** (HTTP 400):

```json
{
  "error": "invalid path: path traversal detected"
}
```

**Missing Parameter** (HTTP 400):

```json
{
  "error": "hash or path query parameter is required"
}
```

---

## GET `/files/metadata`

Get file metadata without downloading the file content.

### Query Parameters

| Parameter | Type   | Required | Description              |
| --------- | ------ | -------- | ------------------------ |
| `hash`    | string | Yes      | SHA-256 hash of the file |

### Response Schema

```json
{
  "hash": "abc123def456...",
  "original_name": "photo.jpg",
  "stored_path": "storage/images/jpg/vacation/abc123_photo.jpg",
  "category": "images/jpg/vacation",
  "mime_type": "image/jpeg",
  "size": 2048576,
  "uploaded_at": "2025-11-15T10:30:00Z",
  "metadata": {
    "comment": "vacation photo"
  }
}
```

### Example

```bash
curl "http://localhost:8090/files/metadata?hash=abc123def456..."
```

### Error Responses

**File Not Found** (HTTP 404):

```json
{
  "error": "file not found: hash abc123..."
}
```

**Missing Parameter** (HTTP 400):

```json
{
  "error": "hash query parameter is required"
}
```

---

## GET `/files/stream`

Stream a file with HTTP range request support. Ideal for video/audio streaming and partial downloads.

### Query Parameters

| Parameter | Type   | Required | Description                    |
| --------- | ------ | -------- | ------------------------------ |
| `hash`    | string | No*      | SHA-256 hash of the file       |
| `path`    | string | No*      | Stored path of the file        |

\* Either `hash` or `path` must be provided.

### Request Headers

| Header      | Description                                    |
| ----------- | ---------------------------------------------- |
| `Range`     | Byte range request (e.g., `bytes=0-1023`)     |
| `If-Range`  | Conditional range request using ETag           |
| `If-Modified-Since` | Conditional request using Last-Modified |

### Range Request Formats

- `bytes=0-1023` - First 1024 bytes
- `bytes=2048-4095` - Bytes 2048 to 4095
- `bytes=2048-` - From byte 2048 to end of file
- `bytes=-1024` - Last 1024 bytes (not yet supported)

### Response Headers

**Full Content (200 OK):**

```
Content-Type: <mime-type>
Content-Length: <file-size>
Content-Disposition: inline; filename="<original-name>"
ETag: "<file-hash>"
Last-Modified: <upload-date>
X-File-Category: <category>
X-File-Hash: <sha256>
Accept-Ranges: bytes
Cache-Control: private, max-age=3600
```

**Partial Content (206 Partial Content):**

```
Content-Type: <mime-type>
Content-Range: bytes <start>-<end>/<total>
Content-Length: <range-size>
ETag: "<file-hash>"
Last-Modified: <upload-date>
Accept-Ranges: bytes
```

### Examples

#### Stream Full File

```bash
curl "http://localhost:8090/files/stream?hash=abc123def456..."
```

#### Stream First 1MB

```bash
curl -H "Range: bytes=0-1048575" "http://localhost:8090/files/stream?hash=abc123def456..."
```

#### Stream Middle Range

```bash
curl -H "Range: bytes=1048576-2097151" "http://localhost:8090/files/stream?hash=abc123def456..."
```

#### Stream from Position to End

```bash
curl -H "Range: bytes=1048576-" "http://localhost:8090/files/stream?hash=abc123def456..."
```

### Error Responses

**File Not Found** (HTTP 404):

```json
{
  "error": "file not found: hash abc123..."
}
```

**Invalid Range** (HTTP 416):

```json
{
  "error": "invalid range"
}
```

**Missing Parameter** (HTTP 400):

```json
{
  "error": "hash or path query parameter is required"
}
```

### Use Cases

- **Video Streaming**: Use range requests to enable seeking in video players
- **Large File Downloads**: Resume interrupted downloads
- **Progressive Loading**: Load file chunks on demand
- **Bandwidth Optimization**: Download only needed portions

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
├── metadata/
│   ├── files.json          # File metadata index
│   ├── delete_log.ndjson   # Deletion audit log
│   └── rename_log.ndjson   # Rename audit log
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

## Testing

See `docs/JSON_TEST_PLAYBOOK.md` for JSON test fixtures and `docs/UNIFIED_INGEST.md` for comprehensive endpoint examples.

---

## Related Documentation

- [Unified Ingestion Guide](UNIFIED_INGEST.md) - Deep dive on `/ingest`
- [JSON Test Playbook](JSON_TEST_PLAYBOOK.md) - Sample payloads for testing
- [Docker Deployment](DOCKER.md) - Containerized deployment
- [Architecture Overview](ARCHITECTURE.md) - System design
