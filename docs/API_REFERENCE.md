# RhinoBox API Reference

Complete reference for all RhinoBox HTTP endpoints.

## Base URL

```
http://localhost:8090
```

Configure via `RHINOBOX_ADDR` environment variable (default `:8090`).

## Documentation Sections

- **[Async API Documentation](./ASYNC_API.md)** - Detailed guide for asynchronous job queue endpoints
- **[Synchronous Endpoints](#synchronous-endpoints)** - Traditional request-response APIs (below)

---

## Endpoints Overview

| Method | Endpoint                    | Purpose                                           |
| ------ | --------------------------- | ------------------------------------------------- |
| GET    | `/healthz`                  | Health check probe                                |
| POST   | `/ingest`                   | **Unified ingestion** - handles all data types    |
| POST   | `/ingest/media`             | Media-specific ingestion                          |
| POST   | `/ingest/json`              | JSON-specific ingestion                           |
| POST   | `/ingest/async`             | **Async unified ingestion** - returns job ID      |
| POST   | `/ingest/media/async`       | **Async media ingestion** - background processing |
| POST   | `/ingest/json/async`        | **Async JSON ingestion** - queued processing      |
| GET    | `/jobs`                     | List all active and recent jobs                   |
| GET    | `/jobs/{job_id}`            | Get job status with progress                      |
| GET    | `/jobs/{job_id}/result`     | Get detailed job results                          |
| DELETE | `/jobs/{job_id}`            | Cancel a job                                      |
| GET    | `/jobs/stats`               | Queue statistics                                  |
| PATCH  | `/files/rename`             | Rename a file                                     |
| DELETE | `/files/{file_id}`          | Delete a file                                     |
| PATCH  | `/files/{file_id}/metadata` | Update file metadata                              |
| POST   | `/files/metadata/batch`     | Batch update file metadata                        |
| GET    | `/files/search`             | Search files with content search support          |
| GET    | `/files`                    | List files with pagination and filtering          |
| GET    | `/files/browse`             | Browse directory structure                        |
| GET    | `/files/categories`         | Get all file categories                           |
| GET    | `/files/stats`              | Get storage statistics                            |
| GET    | `/files/download`           | Download file by hash or path                     |
| GET    | `/files/metadata`           | Get file metadata without downloading             |
| GET    | `/files/stream`             | Stream file with range request support            |
| POST   | `/files/{file_id}/copy`     | Copy a file                                       |
| POST   | `/files/copy/batch`         | Batch copy files                                  |
| PATCH  | `/files/{file_id}/move`     | Move a file to different category                 |
| PATCH  | `/files/batch/move`         | Batch move files                                  |
| GET    | `/files/type/{type}`        | Get files by type (images, videos, documents)     |
| GET    | `/files/{file_id}/notes`    | Get notes/comments for a file                     |
| POST   | `/files/{file_id}/notes`    | Add a note to a file                              |
| PATCH  | `/files/{file_id}/notes/{note_id}` | Update a note                             |
| DELETE | `/files/{file_id}/notes/{note_id}` | Delete a note                             |
| GET    | `/statistics`               | Get overall storage statistics                    |
| GET    | `/collections`              | Get all file collections                          |
| GET    | `/collections/{type}/stats` | Get statistics for a specific collection          |
| GET    | `/api/config`               | Get API configuration                             |

---

## Synchronous Endpoints

All endpoints below return results immediately (blocking). For asynchronous processing of large batches, see [Async API Documentation](./ASYNC_API.md).

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
| `files`     | File(s)     | No       | One or more files (media, documents, archives) |
| `data`      | JSON string | No       | Inline JSON data (object or array)             |
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

| Parameter | Type   | Required | Description                                  |
| --------- | ------ | -------- | -------------------------------------------- |
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

## GET `/files/search`

**Advanced file search** - Search files by metadata and optionally by content inside text files.

### Query Parameters

| Parameter   | Type   | Required | Description                                                |
| ----------- | ------ | -------- | ---------------------------------------------------------- |
| `name`      | string | No\*     | Partial match on filename (case-insensitive)               |
| `extension` | string | No\*     | Exact match on file extension (e.g., "jpg", ".jpg")        |
| `type`      | string | No\*     | Match on MIME type or category (supports partial match)    |
| `category`  | string | No\*     | Match on category path (case-insensitive partial match)    |
| `mime_type` | string | No\*     | Exact match on MIME type (case-insensitive)                |
| `content`   | string | No\*     | **Search inside text files** (case-insensitive, 10MB max)  |
| `date_from` | string | No\*     | Files uploaded on/after this date (RFC3339 or YYYY-MM-DD)  |
| `date_to`   | string | No\*     | Files uploaded on/before this date (RFC3339 or YYYY-MM-DD) |

\* At least one filter parameter is required.

### Content Search Support

The `content` parameter searches inside text files with these MIME types:

- `text/*` (text/plain, text/html, text/markdown, etc.)
- `application/json`
- `application/xml`
- `application/javascript`
- `application/typescript`
- `application/x-sh`
- `application/x-python`

**Limitations:**

- Maximum file size: 10 MB
- Case-insensitive search
- Line-by-line scanning (1MB max line size)

### Response Schema

```json
{
  "filters": {
    "name": "config",
    "content": "database"
  },
  "results": [
    {
      "hash": "abc123def456...",
      "original_name": "config.json",
      "stored_path": "storage/code/json/app/abc123_config.json",
      "category": "code/json/app",
      "mime_type": "application/json",
      "size": 2048,
      "uploaded_at": "2025-11-15T10:30:00Z"
    }
  ],
  "count": 1
}
```

### Examples

#### Search by Filename

```bash
curl "http://localhost:8090/files/search?name=vacation"
```

#### Search by Extension

```bash
curl "http://localhost:8090/files/search?extension=jpg"
```

#### Search by Type (MIME prefix)

```bash
curl "http://localhost:8090/files/search?type=image"
```

#### Content Search in Text Files

```bash
# Search for "TODO" in all text files
curl "http://localhost:8090/files/search?content=TODO"

# Search for "database" in JSON files
curl "http://localhost:8090/files/search?extension=json&content=database"

# Search in config files
curl "http://localhost:8090/files/search?name=config&content=password"
```

#### Date Range Search

```bash
# Files from specific date
curl "http://localhost:8090/files/search?date_from=2025-11-15&name=report"

# Files in date range
curl "http://localhost:8090/files/search?date_from=2025-11-01&date_to=2025-11-30&type=image"
```

#### Combined Filters

```bash
curl "http://localhost:8090/files/search?category=docs&extension=md&content=architecture"
```

### Error Responses

**No Filters** (HTTP 400):

```json
{
  "error": "at least one filter parameter is required (name, extension, type, category, mime_type, content, date_from, date_to)"
}
```

**Invalid Date Format** (HTTP 400):

```json
{
  "error": "invalid date_from format: ... (use RFC3339 or YYYY-MM-DD)"
}
```

---

## GET `/files/download`

Download a file by its hash or stored path.

### Query Parameters

| Parameter | Type   | Required | Description              |
| --------- | ------ | -------- | ------------------------ |
| `hash`    | string | No\*     | SHA-256 hash of the file |
| `path`    | string | No\*     | Stored path of the file  |

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

| Parameter | Type   | Required | Description              |
| --------- | ------ | -------- | ------------------------ |
| `hash`    | string | No\*     | SHA-256 hash of the file |
| `path`    | string | No\*     | Stored path of the file  |

\* Either `hash` or `path` must be provided.

### Request Headers

| Header              | Description                               |
| ------------------- | ----------------------------------------- |
| `Range`             | Byte range request (e.g., `bytes=0-1023`) |
| `If-Range`          | Conditional range request using ETag      |
| `If-Modified-Since` | Conditional request using Last-Modified   |

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

## GET `/files/type/{type}`

Get all files of a specific type (collection).

### Path Parameters

| Parameter | Type   | Description                                    |
| --------- | ------ | ---------------------------------------------- |
| `type`    | string | File type: `images`, `videos`, `audio`, `documents`, `archives`, `other` |

### Query Parameters

| Parameter   | Type   | Description                    |
| ----------- | ------ | ------------------------------ |
| `page`      | int    | Page number (default: 1)       |
| `page_size` | int    | Items per page (default: 50)   |

### Response

```json
{
  "files": [
    {
      "hash": "abc123...",
      "original_name": "vacation.jpg",
      "stored_path": "storage/images/vacation.jpg",
      "category": "images",
      "mime_type": "image/jpeg",
      "size": 1048576,
      "uploaded_at": "2025-11-15T10:30:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "page_size": 50,
    "total_items": 150,
    "total_pages": 3
  }
}
```

### Example

```bash
curl "http://localhost:8090/files/type/images?page=1&page_size=20"
```

---

## GET `/files/{file_id}/notes`

Get all notes/comments for a specific file.

### Path Parameters

| Parameter | Type   | Description |
| --------- | ------ | ----------- |
| `file_id` | string | File hash   |

### Response

```json
{
  "notes": [
    {
      "id": "note_123",
      "file_id": "abc123...",
      "text": "Great photo from the trip!",
      "author": "user@example.com",
      "created_at": "2025-11-15T10:30:00Z",
      "updated_at": "2025-11-15T10:30:00Z"
    }
  ]
}
```

### Example

```bash
curl "http://localhost:8090/files/abc123.../notes"
```

---

## POST `/files/{file_id}/notes`

Add a new note to a file.

### Path Parameters

| Parameter | Type   | Description |
| --------- | ------ | ----------- |
| `file_id` | string | File hash   |

### Request Body

```json
{
  "text": "Remember to edit this later",
  "author": "user@example.com"
}
```

### Response

```json
{
  "id": "note_456",
  "file_id": "abc123...",
  "text": "Remember to edit this later",
  "author": "user@example.com",
  "created_at": "2025-11-15T11:00:00Z",
  "updated_at": "2025-11-15T11:00:00Z"
}
```

### Example

```bash
curl -X POST "http://localhost:8090/files/abc123.../notes" \
  -H "Content-Type: application/json" \
  -d '{"text": "Great shot!", "author": "john@example.com"}'
```

---

## PATCH `/files/{file_id}/notes/{note_id}`

Update an existing note.

### Path Parameters

| Parameter | Type   | Description |
| --------- | ------ | ----------- |
| `file_id` | string | File hash   |
| `note_id` | string | Note ID     |

### Request Body

```json
{
  "text": "Updated note text"
}
```

### Response

```json
{
  "id": "note_456",
  "file_id": "abc123...",
  "text": "Updated note text",
  "author": "user@example.com",
  "created_at": "2025-11-15T11:00:00Z",
  "updated_at": "2025-11-15T11:30:00Z"
}
```

---

## DELETE `/files/{file_id}/notes/{note_id}`

Delete a note from a file.

### Path Parameters

| Parameter | Type   | Description |
| --------- | ------ | ----------- |
| `file_id` | string | File hash   |
| `note_id` | string | Note ID     |

### Response

```json
{
  "message": "Note deleted successfully"
}
```

### Example

```bash
curl -X DELETE "http://localhost:8090/files/abc123.../notes/note_456"
```

---

## GET `/statistics`

Get overall storage and file statistics.

### Response

```json
{
  "total_files": 1250,
  "total_size": 5368709120,
  "total_size_formatted": "5.0 GB",
  "by_category": {
    "images": {"count": 450, "size": 2147483648},
    "videos": {"count": 120, "size": 2147483648},
    "documents": {"count": 680, "size": 1073741824}
  },
  "recent_uploads": 25,
  "storage_used_percent": 45.5
}
```

### Example

```bash
curl "http://localhost:8090/statistics"
```

---

## GET `/collections`

Get all file collections with metadata and statistics.

### Response

```json
{
  "collections": [
    {
      "type": "images",
      "name": "Images",
      "description": "Photo and image files",
      "count": 450,
      "size": 2147483648,
      "size_formatted": "2.0 GB"
    },
    {
      "type": "videos",
      "name": "Videos",
      "description": "Video files",
      "count": 120,
      "size": 2147483648,
      "size_formatted": "2.0 GB"
    }
  ]
}
```

### Example

```bash
curl "http://localhost:8090/collections"
```

---

## GET `/collections/{type}/stats`

Get detailed statistics for a specific collection.

### Path Parameters

| Parameter | Type   | Description                    |
| --------- | ------ | ------------------------------ |
| `type`    | string | Collection type (e.g., `images`) |

### Response

```json
{
  "type": "images",
  "file_count": 450,
  "total_size": 2147483648,
  "storage_used_formatted": "2.0 GB",
  "avg_file_size": 4771853,
  "largest_file": {
    "name": "panorama.jpg",
    "size": 15728640
  },
  "recent_files": 15,
  "file_types": {
    "jpg": 320,
    "png": 100,
    "gif": 30
  }
}
```

### Example

```bash
curl "http://localhost:8090/collections/images/stats"
```

---

## GET `/api/config`

Get API configuration and feature flags.

### Response

```json
{
  "features": {
    "auth_enabled": false,
    "async_processing": true,
    "content_search": true
  },
  "limits": {
    "max_upload_size": 536870912,
    "max_batch_size": 100
  },
  "version": "1.0.0"
}
```

### Example

```bash
curl "http://localhost:8090/api/config"
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
