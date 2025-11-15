# Async Job Queue Endpoints Documentation

## POST `/ingest/async`

**Asynchronous unified ingestion** - queues files for background processing and returns immediately with a job ID.

### Content-Type

`multipart/form-data`

### Form Fields

| Field       | Type    | Required | Description                     |
| ----------- | ------- | -------- | ------------------------------- |
| `files`     | File(s) | Yes      | One or more files to process    |
| `namespace` | string  | No       | Organization/category namespace |
| `comment`   | string  | No       | Hints for categorization        |

### Response Schema

**HTTP 202 Accepted**:

```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "queued",
  "total_items": 3,
  "check_status_url": "/jobs/550e8400-e29b-41d4-a716-446655440000",
  "created_at": "2025-11-15T10:30:00Z"
}
```

### Example

```bash
curl -X POST http://localhost:8090/ingest/async \
  -F "files=@large_file1.mp4" \
  -F "files=@large_file2.mp4" \
  -F "files=@large_file3.mp4" \
  -F "namespace=videos"
```

---

## POST `/ingest/media/async`

**Asynchronous media ingestion** - queues media files for background processing.

### Content-Type

`multipart/form-data`

### Form Fields

| Field       | Type    | Required | Description             |
| ----------- | ------- | -------- | ----------------------- |
| `file`      | File(s) | Yes      | One or more media files |
| `category`  | string  | No       | Media category          |
| `namespace` | string  | No       | Organization namespace  |

### Response Schema

**HTTP 202 Accepted**:

```json
{
  "job_id": "abc-123-def-456",
  "status": "queued",
  "total_items": 5,
  "check_status_url": "/jobs/abc-123-def-456",
  "created_at": "2025-11-15T10:30:00Z"
}
```

### Example

```bash
curl -X POST http://localhost:8090/ingest/media/async \
  -F "file=@video1.mp4" \
  -F "file=@video2.mp4" \
  -F "category=tutorials"
```

---

## POST `/ingest/json/async`

**Asynchronous JSON ingestion** - queues JSON documents for background processing with decision engine.

### Content-Type

`application/json`

### Request Schema

```json
{
  "namespace": "string",       // Required
  "comment": "string",         // Optional
  "documents": [{...}]         // Required: array of JSON documents
}
```

### Response Schema

**HTTP 202 Accepted**:

```json
{
  "job_id": "xyz-789-abc-012",
  "status": "queued",
  "total_items": 1000,
  "check_status_url": "/jobs/xyz-789-abc-012",
  "created_at": "2025-11-15T10:30:00Z"
}
```

### Example

```bash
curl -X POST http://localhost:8090/ingest/json/async \
  -H "Content-Type: application/json" \
  -d '{
    "namespace": "orders",
    "documents": [
      {"order_id": 1, "total": 100.0},
      {"order_id": 2, "total": 200.0}
    ]
  }'
```

---

## GET `/jobs`

**List all active jobs** - returns currently processing and recently completed jobs.

### Query Parameters

| Parameter | Type | Required | Default | Description                      |
| --------- | ---- | -------- | ------- | -------------------------------- |
| `limit`   | int  | No       | 100     | Maximum number of jobs to return |

### Response Schema

```json
{
  "jobs": [
    {
      "job_id": "abc-123",
      "type": "media",
      "status": "processing",
      "progress": 7,
      "total": 10,
      "progress_pct": 70.0,
      "created_at": "2025-11-15T10:30:00Z"
    },
    {
      "job_id": "def-456",
      "type": "json",
      "status": "completed",
      "progress": 100,
      "total": 100,
      "progress_pct": 100.0,
      "created_at": "2025-11-15T10:25:00Z"
    }
  ],
  "count": 2
}
```

### Example

```bash
# List all jobs
curl "http://localhost:8090/jobs"

# Limit to 50 jobs
curl "http://localhost:8090/jobs?limit=50"
```

---

## GET `/jobs/{job_id}`

**Get job status** - returns current status and progress of a specific job.

### URL Parameters

| Parameter | Type   | Required | Description                          |
| --------- | ------ | -------- | ------------------------------------ |
| `job_id`  | string | Yes      | Job ID from async ingestion response |

### Response Schema

**Processing** (HTTP 200):

```json
{
  "job_id": "abc-123",
  "type": "media",
  "status": "processing",
  "progress": 5,
  "total": 10,
  "progress_pct": 50.0,
  "created_at": "2025-11-15T10:30:00Z",
  "started_at": "2025-11-15T10:30:01Z"
}
```

**Completed** (HTTP 200):

```json
{
  "job_id": "abc-123",
  "type": "media",
  "status": "completed",
  "progress": 10,
  "total": 10,
  "progress_pct": 100.0,
  "created_at": "2025-11-15T10:30:00Z",
  "started_at": "2025-11-15T10:30:01Z",
  "completed_at": "2025-11-15T10:30:15Z",
  "duration_ms": 14000
}
```

**Failed** (HTTP 200):

```json
{
  "job_id": "abc-123",
  "type": "media",
  "status": "failed",
  "progress": 7,
  "total": 10,
  "progress_pct": 70.0,
  "error": "partial success: 7 succeeded, 3 failed",
  "created_at": "2025-11-15T10:30:00Z",
  "started_at": "2025-11-15T10:30:01Z",
  "completed_at": "2025-11-15T10:30:15Z",
  "duration_ms": 14000
}
```

**Not Found** (HTTP 404):

```json
{
  "error": "job not found"
}
```

### Job Status Values

- `queued` - Job is waiting to be processed
- `processing` - Job is currently being executed
- `completed` - Job finished successfully (all or partial success)
- `failed` - Job failed completely (all items failed)
- `cancelled` - Job was cancelled by user

### Example

```bash
# Check job status
curl "http://localhost:8090/jobs/abc-123"

# Poll for completion (bash)
while true; do
  status=$(curl -s "http://localhost:8090/jobs/abc-123" | jq -r '.status')
  if [ "$status" = "completed" ] || [ "$status" = "failed" ]; then
    break
  fi
  echo "Status: $status"
  sleep 2
done
```

---

## GET `/jobs/{job_id}/result`

**Get job results** - returns detailed results including all processed items.

### URL Parameters

| Parameter | Type   | Required | Description |
| --------- | ------ | -------- | ----------- |
| `job_id`  | string | Yes      | Job ID      |

### Response Schema

**Success** (HTTP 200):

```json
{
  "job_id": "abc-123",
  "status": "completed",
  "total": 10,
  "succeeded": 10,
  "failed": 0,
  "duration_ms": 14523,
  "results": [
    {
      "id": "item-1",
      "type": "media",
      "name": "video1.mp4",
      "size": 10485760,
      "result": {
        "stored_path": "storage/videos/mp4/default/abc123_video1.mp4",
        "hash": "abc123...",
        "category": "videos/mp4",
        "is_duplicate": false,
        "metadata": {
          "mime_type": "video/mp4",
          "size": 10485760
        }
      }
    }
  ]
}
```

**Partial Success** (HTTP 200):

```json
{
  "job_id": "abc-123",
  "status": "completed",
  "total": 10,
  "succeeded": 7,
  "failed": 3,
  "duration_ms": 14523,
  "error": "partial success: 7 succeeded, 3 failed",
  "results": [
    {
      "id": "item-1",
      "name": "video1.mp4",
      "error": "failed to store file: disk full"
    },
    {
      "id": "item-2",
      "name": "video2.mp4",
      "result": {
        "stored_path": "...",
        "hash": "..."
      }
    }
  ]
}
```

**Job Not Completed** (HTTP 409):

```json
{
  "error": "job not completed yet (status: processing)"
}
```

**Not Found** (HTTP 404):

```json
{
  "error": "job not found"
}
```

### Example

```bash
curl "http://localhost:8090/jobs/abc-123/result"
```

---

## DELETE `/jobs/{job_id}`

**Cancel a job** - attempts to cancel a queued or processing job.

### URL Parameters

| Parameter | Type   | Required | Description |
| --------- | ------ | -------- | ----------- |
| `job_id`  | string | Yes      | Job ID      |

### Response Schema

**Success** (HTTP 200):

```json
{
  "job_id": "abc-123",
  "status": "queued",
  "message": "cancellation not yet implemented"
}
```

**Already Completed** (HTTP 409):

```json
{
  "error": "cannot cancel completed job"
}
```

**Not Found** (HTTP 404):

```json
{
  "error": "job not found"
}
```

### Note

Job cancellation is currently a placeholder. Active jobs will complete processing.

### Example

```bash
curl -X DELETE "http://localhost:8090/jobs/abc-123"
```

---

## GET `/jobs/stats`

**Queue statistics** - returns current queue metrics.

### Response Schema

```json
{
  "pending": 15,
  "processing": 3,
  "completed": 245,
  "workers": 10
}
```

### Fields

- `pending` - Jobs waiting in queue
- `processing` - Jobs currently being executed
- `completed` - Jobs finished (cached in memory)
- `workers` - Number of worker threads

### Example

```bash
curl "http://localhost:8090/jobs/stats"
```

---

## Performance Characteristics

### Async Ingestion

- **Response Time**: <1ms (HTTP 202 Accepted)
- **Client Blocking**: 0 (immediate return)
- **Throughput**: 1,677 jobs/sec enqueue rate
- **Queue Capacity**: 1000 jobs buffer
- **Workers**: 10 concurrent processing threads (configurable)

### Job Processing

- **Enqueue Latency**: 596 µs/op average
- **Memory**: 7.5 KB per job
- **Persistence**: All jobs saved to disk for crash recovery
- **Resume**: Incomplete jobs auto-resume on server restart
- **Partial Success**: Continues processing even if some items fail

### Use Cases

**When to use async endpoints:**

- Batch uploads >100 files
- Large files >100MB
- Long-running processing
- Don't need immediate results
- Want to continue other work while uploading

**When to use sync endpoints:**

- Small batches <10 files
- Need immediate confirmation
- Simple upload workflows
- Testing and development
