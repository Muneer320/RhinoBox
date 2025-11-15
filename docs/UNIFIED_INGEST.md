# Unified Ingestion Endpoint

## Overview

The `/ingest` endpoint provides a single, intelligent entry point for all data types. It automatically routes content to appropriate processing pipelines based on MIME type and file extensions.

## Endpoint

```
POST /ingest
Content-Type: multipart/form-data
```

## Request Format

### Form Fields

| Field       | Type        | Required | Description                                      |
| ----------- | ----------- | -------- | ------------------------------------------------ |
| `files`     | File(s)     | No       | One or more files (media, documents, or generic) |
| `data`      | JSON string | No       | Inline JSON data (object or array)               |
| `namespace` | string      | No       | Organization/category namespace                  |
| `comment`   | string      | No       | Hints for categorization or decision engine      |
| `metadata`  | JSON string | No       | Additional context (tags, source, description)   |

**Note**: At least one of `files` or `data` must be provided.

## Response Format

```json
{
  "job_id": "job_1731687000000000",
  "status": "completed",
  "results": {
    "media": [
      {
        "original_name": "photo.jpg",
        "stored_path": "media/images/vacation/photo_uuid.jpg",
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
        "decision": { ... },
        "batch_path": "json/sql/orders/batch_20251115.ndjson"
      }
    ],
    "files": [
      {
        "original_name": "report.pdf",
        "stored_path": "files/docs/report_uuid.pdf",
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

## Routing Logic

### Automatic Content Detection

```
┌─────────────┐
│   Request   │
└──────┬──────┘
       │
       ▼
┌──────────────────┐
│ MIME Detection   │
│ (Header + Ext)   │
└──────┬───────────┘
       │
   ┌───┴────┬─────────┬────────────┐
   ▼        ▼         ▼            ▼
┌──────┐ ┌────┐  ┌──────┐    ┌──────────┐
│Image │ │Video│  │JSON  │    │ Generic  │
│      │ │     │  │      │    │   File   │
└──────┘ └────┘  └──────┘    └──────────┘
```

### Media Files

Detected by MIME type or extension: `.jpg`, `.png`, `.gif`, `.mp4`, `.mov`, `.mp3`, `.wav`

**Processing:**

- Automatic categorization
- Directory organization
- Metadata extraction

### JSON Data

Provided via `data` field or `.json` files.

**Processing:**

- Schema analysis
- SQL vs NoSQL decision
- Relationship detection
- Schema generation (for SQL)

### Generic Files

PDFs, documents, archives, and other file types.

**Processing:**

- Namespace-based organization
- Hash computation
- Metadata indexing

## Examples

### Single Media Upload

```bash
curl -X POST http://localhost:8090/ingest \
  -F "files=@vacation.jpg" \
  -F "comment=summer trip" \
  -F "namespace=travel"
```

### Batch Media Upload

```bash
curl -X POST http://localhost:8090/ingest \
  -F "files=@img1.jpg" \
  -F "files=@img2.jpg" \
  -F "files=@video.mp4" \
  -F "comment=family photos"
```

### JSON Data Ingestion

```bash
curl -X POST http://localhost:8090/ingest \
  -F 'data=[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]' \
  -F "namespace=users" \
  -F "comment=user batch import"
```

### Mixed Upload

```bash
curl -X POST http://localhost:8090/ingest \
  -F "files=@photo.png" \
  -F "files=@document.pdf" \
  -F 'data=[{"order_id":101,"total":42.50}]' \
  -F "namespace=orders" \
  -F "comment=order batch with attachments"
```

### PowerShell Example

```powershell
$form = @{
    files = Get-Item "photo.jpg"
    namespace = "gallery"
    comment = "demo"
}

Invoke-RestMethod -Uri http://localhost:8090/ingest `
    -Method Post `
    -Form $form
```

## Error Handling

### Partial Failures

If some files succeed and others fail, the endpoint returns HTTP 200 with errors listed:

```json
{
  "job_id": "job_123",
  "status": "completed",
  "results": {
    "media": [ ... ],
    "json": [],
    "files": []
  },
  "errors": [
    "invalid.xyz: unsupported file type"
  ]
}
```

### Complete Failure

If all items fail, returns HTTP 400:

```json
{
  "error": "all items failed: [file1.xyz: error, file2.abc: error]"
}
```

## Performance

- **Batch Processing**: Handles 1000+ files in a single request
- **Concurrent Routing**: Files processed in parallel
- **Timing Metrics**: Detailed breakdown in `timing` field
- **Memory Efficient**: Streaming upload support

## Comparison with Specialized Endpoints

| Feature          | `/ingest` | `/ingest/media` | `/ingest/json` |
| ---------------- | --------- | --------------- | -------------- |
| Media files      | ✅        | ✅              | ❌             |
| JSON data        | ✅        | ❌              | ✅             |
| Generic files    | ✅        | ❌              | ❌             |
| Mixed batches    | ✅        | ❌              | ❌             |
| Unified response | ✅        | ❌              | ❌             |

**Recommendation**: Use `/ingest` for maximum flexibility; specialized endpoints remain available for focused workflows.

## Related

- [JSON Decision Engine](JSON_TEST_PLAYBOOK.md)
- [Docker Deployment](DOCKER.md)
- Architecture documentation
