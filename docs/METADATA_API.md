# File Metadata Update API

## Overview

The File Metadata Update API allows you to add, update, and remove metadata associated with files stored in RhinoBox. This enables rich file organization, annotation, compliance tracking, and document management workflows.

## Endpoint

```
PATCH /files/{file_id}/metadata
```

### Parameters

- `file_id` (path parameter, required): The unique identifier of the file. This can be either:
  - The file's SHA-256 hash
  - The file's stored path (for internal use)

### Request Body

The request body should be a JSON object with the following structure:

```json
{
  "action": "replace|merge|remove",
  "metadata": {
    "key1": "value1",
    "key2": "value2"
  },
  "fields": ["field1", "field2"]
}
```

#### Fields

- `action` (optional, default: "replace"): The type of operation to perform
  - `"replace"`: Replace all existing metadata with the provided metadata
  - `"merge"`: Add or update specific metadata fields, keeping existing fields
  - `"remove"`: Remove specific metadata fields (use with `fields` parameter)

- `metadata` (object): Key-value pairs of metadata to set (used with `replace` and `merge` actions)

- `fields` (array of strings): List of metadata field names to remove (used with `remove` action)

## Operations

### Replace Metadata

Replaces all existing metadata with the provided metadata. Any existing metadata not in the request will be removed.

**Request:**
```bash
curl -X PATCH http://localhost:8090/files/{file_id}/metadata \
  -H "Content-Type: application/json" \
  -d '{
    "action": "replace",
    "metadata": {
      "comment": "Updated description",
      "tags": "important,project-x,Q4-2024",
      "description": "Financial report for Q4",
      "author": "John Doe"
    }
  }'
```

**Response:**
```json
{
  "file_id": "5eaa68a1a937fa098ddaae4e15ea9e2aae4ea66ce1faeb0317994dbc4e2fc414",
  "hash": "5eaa68a1a937fa098ddaae4e15ea9e2aae4ea66ce1faeb0317994dbc4e2fc414",
  "original_name": "document.pdf",
  "stored_path": "storage/documents/pdf/5eaa68a1a937_document.pdf",
  "category": "documents/pdf",
  "mime_type": "application/pdf",
  "size": 1024,
  "uploaded_at": "2025-11-15T12:00:00Z",
  "metadata": {
    "comment": "Updated description",
    "tags": "important,project-x,Q4-2024",
    "description": "Financial report for Q4",
    "author": "John Doe"
  }
}
```

### Merge Metadata

Adds or updates specific metadata fields while preserving existing fields not mentioned in the request.

**Request:**
```bash
curl -X PATCH http://localhost:8090/files/{file_id}/metadata \
  -H "Content-Type: application/json" \
  -d '{
    "action": "merge",
    "metadata": {
      "tags": "archived",
      "status": "completed"
    }
  }'
```

**Response:**
```json
{
  "file_id": "5eaa68a1a937fa098ddaae4e15ea9e2aae4ea66ce1faeb0317994dbc4e2fc414",
  "metadata": {
    "comment": "Updated description",
    "tags": "archived",
    "description": "Financial report for Q4",
    "author": "John Doe",
    "status": "completed"
  }
}
```

### Remove Metadata

Removes specific metadata fields from the file.

**Request:**
```bash
curl -X PATCH http://localhost:8090/files/{file_id}/metadata \
  -H "Content-Type: application/json" \
  -d '{
    "action": "remove",
    "fields": ["comment", "old_field"]
  }'
```

**Response:**
```json
{
  "file_id": "5eaa68a1a937fa098ddaae4e15ea9e2aae4ea66ce1faeb0317994dbc4e2fc414",
  "metadata": {
    "tags": "archived",
    "description": "Financial report for Q4",
    "author": "John Doe",
    "status": "completed"
  }
}
```

## System Fields (Immutable)

The following fields are system-managed and cannot be modified through the metadata API:

- `hash` - SHA-256 hash of the file
- `size` - File size in bytes
- `uploaded_at` - Upload timestamp
- `mime_type` - MIME type of the file
- `original_name` - Original filename
- `stored_path` - Storage path
- `category` - File category

Attempting to modify these fields will result in a `400 Bad Request` error.

## Validation & Limits

To prevent abuse and ensure system stability, the following limits are enforced:

- **Maximum fields**: 100 metadata fields per file
- **Maximum key length**: 256 characters
- **Maximum value length**: 10KB (10,240 bytes)
- **Maximum total size**: 100KB (102,400 bytes) for all metadata combined

## Error Responses

### 400 Bad Request

Returned when the request is invalid.

**Examples:**
```json
{
  "error": "action must be 'replace', 'merge', or 'remove'"
}
```

```json
{
  "error": "cannot modify system field 'hash'"
}
```

```json
{
  "error": "metadata cannot exceed 100 fields"
}
```

```json
{
  "error": "metadata value for 'description' exceeds 10KB"
}
```

### 404 Not Found

Returned when the specified file does not exist.

```json
{
  "error": "file not found"
}
```

### 500 Internal Server Error

Returned when an unexpected server error occurs.

```json
{
  "error": "update failed: <error details>"
}
```

## Audit Logging

All metadata changes are logged to `{data_dir}/metadata/audit_log.ndjson` for compliance and tracking purposes.

**Audit Log Entry Example:**
```json
{
  "file_id": "5eaa68a1a937fa098ddaae4e15ea9e2aae4ea66ce1faeb0317994dbc4e2fc414",
  "hash": "5eaa68a1a937fa098ddaae4e15ea9e2aae4ea66ce1faeb0317994dbc4e2fc414",
  "stored_path": "storage/documents/pdf/5eaa68a1a937_document.pdf",
  "action": "merge",
  "timestamp": "2025-11-15T12:30:00Z",
  "metadata_updated": {
    "tags": "archived",
    "status": "completed"
  }
}
```

## Use Cases

### Document Management Workflow

Track document status through approval stages:

```bash
# Submit for review
curl -X PATCH http://localhost:8090/files/{file_id}/metadata \
  -H "Content-Type: application/json" \
  -d '{
    "action": "merge",
    "metadata": {
      "status": "in_review",
      "reviewer": "Alice Johnson",
      "submitted_at": "2025-11-15T12:00:00Z"
    }
  }'

# Approve document
curl -X PATCH http://localhost:8090/files/{file_id}/metadata \
  -H "Content-Type: application/json" \
  -d '{
    "action": "merge",
    "metadata": {
      "status": "approved",
      "approved_by": "Bob Smith",
      "approved_at": "2025-11-15T14:00:00Z"
    }
  }'
```

### Compliance and Classification

Add compliance metadata for regulatory requirements:

```bash
curl -X PATCH http://localhost:8090/files/{file_id}/metadata \
  -H "Content-Type: application/json" \
  -d '{
    "action": "merge",
    "metadata": {
      "classification": "confidential",
      "compliance": "SOX,GDPR",
      "retention_years": "7",
      "access_level": "finance-team-only",
      "encryption": "AES-256"
    }
  }'
```

### Project Organization

Organize files by project and team:

```bash
curl -X PATCH http://localhost:8090/files/{file_id}/metadata \
  -H "Content-Type: application/json" \
  -d '{
    "action": "replace",
    "metadata": {
      "project_name": "Project Phoenix",
      "client": "Acme Corporation",
      "department": "Engineering",
      "team": "Backend Team",
      "sprint": "Sprint 12",
      "tags": "backend,api,authentication",
      "owner": "john.doe@company.com"
    }
  }'
```

### File Archival

Archive files and clean up temporary metadata:

```bash
# Archive
curl -X PATCH http://localhost:8090/files/{file_id}/metadata \
  -H "Content-Type: application/json" \
  -d '{
    "action": "merge",
    "metadata": {
      "status": "archived",
      "archived_at": "2025-11-15T15:00:00Z",
      "archived_by": "admin@company.com"
    }
  }'

# Clean up temporary fields
curl -X PATCH http://localhost:8090/files/{file_id}/metadata \
  -H "Content-Type: application/json" \
  -d '{
    "action": "remove",
    "fields": ["temp_field", "draft_notes"]
  }'
```

## Best Practices

1. **Use Consistent Keys**: Establish a consistent naming convention for metadata keys across your organization
2. **Tag Appropriately**: Use comma-separated tags for better searchability
3. **Version Control**: Track document versions in metadata
4. **Timestamps**: Include timestamps for workflow stages (submitted_at, approved_at, etc.)
5. **Ownership**: Always include owner/assignee information
6. **Audit Trail**: Leverage the merge action to build an audit trail without losing historical metadata

## Integration with Search

Metadata fields are available for search and filtering operations. Common searchable fields include:

- Tags (multi-value)
- Description/Comment (full-text search)
- Author
- Department/Project
- Status
- Classification
- Custom key-value pairs

## Examples

### Complete Workflow Example

```bash
# 1. Upload a file
RESPONSE=$(curl -s -X POST http://localhost:8090/ingest/media \
  -F "file=@report.pdf" \
  -F "comment=Q4 Financial Report")

FILE_ID=$(echo $RESPONSE | jq -r '.stored[0].hash')

# 2. Add initial metadata
curl -X PATCH http://localhost:8090/files/$FILE_ID/metadata \
  -H "Content-Type: application/json" \
  -d '{
    "action": "replace",
    "metadata": {
      "title": "Q4 2024 Financial Report",
      "author": "Jane Smith",
      "department": "Finance",
      "tags": "finance,q4,2024,report"
    }
  }'

# 3. Update status
curl -X PATCH http://localhost:8090/files/$FILE_ID/metadata \
  -H "Content-Type: application/json" \
  -d '{
    "action": "merge",
    "metadata": {
      "status": "approved",
      "approved_by": "CFO",
      "approved_at": "2025-11-15T14:00:00Z"
    }
  }'

# 4. Add compliance metadata
curl -X PATCH http://localhost:8090/files/$FILE_ID/metadata \
  -H "Content-Type: application/json" \
  -d '{
    "action": "merge",
    "metadata": {
      "classification": "confidential",
      "retention_years": "7"
    }
  }'
```

## See Also

- [File Ingestion API](./UNIFIED_INGEST.md)
- [Search API](./SEARCH.md) (when implemented)
- [README](../README.md)
