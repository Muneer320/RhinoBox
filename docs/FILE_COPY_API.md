# File Copy/Duplication API

## Overview

The File Copy API enables efficient duplication of existing files with customizable metadata. It supports two modes:

1. **Full Copy** - Creates a physical copy of the file (automatically deduplicated if content matches)
2. **Hard Link** - Creates a reference to the same physical file (space-efficient)

## Endpoints

### Single File Copy

Create a copy of an existing file with new metadata.

**Endpoint:** `POST /files/{file_id}/copy`

**Path Parameters:**
- `file_id` (string, required) - Hash or stored path of the source file

**Request Body:**
```json
{
  "new_name": "copy-of-document.pdf",
  "new_category": "documents/pdf",
  "metadata": {
    "comment": "Working copy",
    "tags": "draft",
    "project": "demo"
  },
  "hard_link": false
}
```

**Request Fields:**
- `new_name` (string, optional) - New filename for the copy. If omitted, uses the original filename.
- `new_category` (string, optional) - Category path for the copy (e.g., "documents/pdf", "images/jpg/nature"). If omitted, preserves the original category.
- `metadata` (object, optional) - Custom metadata key-value pairs to attach to the copy.
- `hard_link` (boolean, optional, default: false) - If true, creates a hard link reference instead of a full copy.

**Response (200 OK):**
```json
{
  "success": true,
  "source": {
    "hash": "7e20bef3bdd0...",
    "original_name": "document.pdf",
    "stored_path": "storage/documents/pdf/7e20bef3bdd0_document.pdf",
    "category": "documents/pdf",
    "mime_type": "application/pdf",
    "size": 2048,
    "uploaded_at": "2025-11-15T12:00:00Z"
  },
  "copy": {
    "hash": "7e20bef3_copy_a1b2c3d4",
    "original_name": "copy-of-document.pdf",
    "stored_path": "storage/documents/pdf/7e20bef3bdd0_document.pdf",
    "category": "documents/pdf",
    "mime_type": "application/pdf",
    "size": 2048,
    "uploaded_at": "2025-11-15T12:05:00Z",
    "is_hard_link": true,
    "linked_to": "7e20bef3bdd0..."
  },
  "is_hard_link": true
}
```

**Response Fields:**
- `success` (boolean) - Indicates if the operation succeeded
- `source` (object) - Metadata of the source file
- `copy` (object) - Metadata of the newly created copy
- `is_hard_link` (boolean) - True if the copy is a hard link or was automatically deduplicated

**Error Responses:**

**400 Bad Request** - Invalid request or source file not found
```json
{
  "error": "source file not found: invalid-hash"
}
```

---

### Batch File Copy

Copy multiple files in a single request.

**Endpoint:** `POST /files/copy/batch`

**Request Body:**
```json
{
  "operations": [
    {
      "source_path": "hash1",
      "new_name": "backup-doc1.pdf",
      "new_category": "backups",
      "metadata": {
        "backup_date": "2025-11-15"
      },
      "hard_link": false
    },
    {
      "source_path": "hash2",
      "new_name": "template-copy.txt",
      "new_category": "templates",
      "hard_link": true
    }
  ]
}
```

**Request Fields:**
- `operations` (array, required) - Array of copy operations. Each operation has the same fields as the single copy request.

**Response (200 OK):**
```json
{
  "total": 2,
  "successful": 2,
  "failed": 0,
  "results": [
    {
      "index": 0,
      "source_path": "hash1",
      "success": true,
      "copy_hash": "abc123...",
      "copy_path": "storage/backups/abc123_backup-doc1.pdf",
      "is_hard_link": false
    },
    {
      "index": 1,
      "source_path": "hash2",
      "success": true,
      "copy_hash": "def456_ref_...",
      "copy_path": "storage/templates/def456_template-copy.txt",
      "is_hard_link": true
    }
  ]
}
```

**Response Fields:**
- `total` (integer) - Total number of operations
- `successful` (integer) - Number of successful operations
- `failed` (integer) - Number of failed operations
- `results` (array) - Detailed results for each operation
  - `index` (integer) - Index of the operation in the request
  - `source_path` (string) - Source file identifier
  - `success` (boolean) - Whether the operation succeeded
  - `copy_hash` (string) - Hash of the created copy (if successful)
  - `copy_path` (string) - Path to the created copy (if successful)
  - `is_hard_link` (boolean) - Whether the copy is a hard link
  - `error` (string) - Error message (if failed)

---

## Copy Modes

### Full Copy Mode

When `hard_link: false`, the system attempts to create a physical copy of the file:

- Reads the source file
- Computes hash of the content
- Creates a new file in the target location

**Automatic Deduplication:**
If the file content matches an existing file (same hash), the system automatically converts it to a hard link to save storage space. The new metadata entry is still created with your specified name and metadata.

### Hard Link Mode

When `hard_link: true`, creates a reference to the same physical file:

- No physical file duplication
- Points to the same stored file
- Separate metadata entry with custom name and metadata
- Reference counting tracks number of links
- More storage efficient

---

## Use Cases

### 1. Template System

Create multiple documents from a single template:

```bash
curl -X POST http://localhost:8090/files/{template_hash}/copy \
  -H "Content-Type: application/json" \
  -d '{
    "new_name": "invoice-acme-corp.txt",
    "new_category": "invoices",
    "metadata": {
      "company": "Acme Corp",
      "invoice_id": "INV-001",
      "amount": "1500.00"
    },
    "hard_link": true
  }'
```

### 2. Backup Workflow

Create backups of important files:

```bash
curl -X POST http://localhost:8090/files/copy/batch \
  -H "Content-Type: application/json" \
  -d '{
    "operations": [
      {
        "source_path": "doc1_hash",
        "new_name": "backup-2025-11-15-doc1.pdf",
        "new_category": "backups",
        "metadata": {
          "backup_date": "2025-11-15",
          "backup_type": "manual"
        },
        "hard_link": false
      }
    ]
  }'
```

### 3. Version Control

Create version snapshots using hard links:

```bash
curl -X POST http://localhost:8090/files/{document_hash}/copy \
  -H "Content-Type: application/json" \
  -d '{
    "new_name": "document-v1.2.txt",
    "new_category": "versions",
    "metadata": {
      "version": "1.2",
      "created_at": "2025-11-15"
    },
    "hard_link": true
  }'
```

### 4. Create Working Copies

Make editable copies while preserving originals:

```bash
curl -X POST http://localhost:8090/files/{original_hash}/copy \
  -H "Content-Type: application/json" \
  -d '{
    "new_name": "working-copy.docx",
    "metadata": {
      "status": "draft",
      "editor": "user123"
    },
    "hard_link": false
  }'
```

---

## Reference Counting

The system tracks how many metadata entries reference each physical file:

- When creating a hard link, the reference count increments
- When deleting a file with references, only the metadata is removed
- Physical file is deleted only when reference count reaches zero

This ensures:
- Storage efficiency with hard links
- Data integrity (files aren't deleted while referenced)
- Independent lifecycle management

---

## Best Practices

1. **Use Hard Links for Templates**: When you have a base template used to create many similar files, use hard links to save storage.

2. **Use Full Copy for Modifications**: When you plan to modify the copy independently, use full copy mode (or let automatic deduplication handle it).

3. **Batch Operations**: For multiple copies, use the batch endpoint to reduce network overhead.

4. **Category Organization**: Use meaningful category paths to organize copies (e.g., "backups/daily", "templates/invoices").

5. **Metadata Tracking**: Add relevant metadata to help track purpose, creation date, and relationships between copies.

---

## Limitations

1. File ID must be a valid hash or stored path of an existing file
2. Category paths are sanitized (only alphanumeric, underscore, hyphen allowed)
3. Metadata values are stored as strings
4. Batch operations process sequentially (partial failures are handled gracefully)

---

## Error Handling

The API handles errors gracefully:

- **Source not found**: Returns 400 with descriptive error
- **Invalid request**: Returns 400 with validation errors
- **Storage errors**: Returns appropriate error codes
- **Batch partial failures**: Returns success status with detailed results showing which operations failed

---

## Examples

### PowerShell Examples

**Single Copy:**
```powershell
$body = @{
    new_name = "copy-document.pdf"
    new_category = "documents/pdf"
    metadata = @{
        comment = "Working copy"
    }
    hard_link = $false
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://localhost:8090/files/{hash}/copy" `
    -Method Post `
    -ContentType "application/json" `
    -Body $body
```

**Batch Copy:**
```powershell
$body = @{
    operations = @(
        @{
            source_path = "hash1"
            new_name = "backup1.pdf"
            hard_link = $false
        },
        @{
            source_path = "hash2"
            new_name = "backup2.pdf"
            hard_link = $false
        }
    )
} | ConvertTo-Json -Depth 3

Invoke-RestMethod -Uri "http://localhost:8090/files/copy/batch" `
    -Method Post `
    -ContentType "application/json" `
    -Body $body
```

### cURL Examples

**Single Copy:**
```bash
curl -X POST http://localhost:8090/files/{hash}/copy \
  -H "Content-Type: application/json" \
  -d '{
    "new_name": "copy-document.pdf",
    "new_category": "documents/pdf",
    "metadata": {
      "comment": "Working copy"
    },
    "hard_link": false
  }'
```

**Batch Copy:**
```bash
curl -X POST http://localhost:8090/files/copy/batch \
  -H "Content-Type: application/json" \
  -d '{
    "operations": [
      {
        "source_path": "hash1",
        "new_name": "backup1.pdf",
        "hard_link": false
      },
      {
        "source_path": "hash2",
        "new_name": "backup2.pdf",
        "hard_link": false
      }
    ]
  }'
```
