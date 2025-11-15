# Duplicate Detection and Management API

This document describes the duplicate file detection and management system in RhinoBox.

## Overview

RhinoBox includes a comprehensive duplicate detection and management system that:
- Scans for duplicate files based on SHA256 hash
- Verifies system integrity (metadata index vs physical files)
- Detects orphaned files (on disk but not in index)
- Detects missing files (in index but not on disk)
- Provides duplicate merge/cleanup operations
- Reports storage waste from duplicates

## API Endpoints

### 1. Scan for Duplicates

Performs a duplicate scan across all files in the metadata index.

**Endpoint:** `POST /files/duplicates/scan`

**Request Body:**
```json
{
  "deep_scan": true,
  "include_metadata": true
}
```

**Parameters:**
- `deep_scan` (boolean, optional): Whether to perform deep hash verification. Default: false
- `include_metadata` (boolean, optional): Whether to include detailed duplicate groups in response. Default: true

**Response:**
```json
{
  "scan_id": "scan-abc123",
  "total_files": 100,
  "duplicates_found": 5,
  "storage_wasted": 524288,
  "status": "completed",
  "started_at": "2024-01-01T00:00:00Z",
  "completed_at": "2024-01-01T00:00:01Z",
  "groups": [
    {
      "hash": "abc123def456...",
      "count": 2,
      "size": 1048576,
      "total_wasted": 1048576,
      "files": [
        {
          "hash": "abc123def456...",
          "original_name": "vacation.jpg",
          "stored_path": "storage/images/jpg/abc123_photo.jpg",
          "category": "images/jpg",
          "mime_type": "image/jpeg",
          "size": 1048576,
          "uploaded_at": "2024-01-01T00:00:00Z"
        }
      ]
    }
  ]
}
```

**Example:**
```bash
curl -X POST http://localhost:8090/files/duplicates/scan \
  -H "Content-Type: application/json" \
  -d '{"deep_scan": true, "include_metadata": true}'
```

---

### 2. Get Duplicate Report

Returns a detailed report of all duplicate file groups.

**Endpoint:** `GET /files/duplicates`

**Response:**
```json
{
  "duplicate_groups": [
    {
      "hash": "abc123def456...",
      "count": 3,
      "size": 1048576,
      "total_wasted": 2097152,
      "files": [
        {
          "hash": "abc123def456...",
          "original_name": "vacation.jpg",
          "stored_path": "storage/images/jpg/abc123_photo.jpg",
          "category": "images/jpg",
          "mime_type": "image/jpeg",
          "size": 1048576,
          "uploaded_at": "2024-01-01T00:00:00Z"
        }
      ]
    }
  ],
  "total_groups": 1,
  "total_duplicates": 2,
  "storage_wasted": 2097152
}
```

**Example:**
```bash
curl -X GET http://localhost:8090/files/duplicates
```

---

### 3. Verify System Integrity

Performs comprehensive integrity checks on the storage system.

**Endpoint:** `POST /files/duplicates/verify`

**Response:**
```json
{
  "metadata_index_count": 1000,
  "physical_files_count": 1002,
  "hash_mismatches": 0,
  "orphaned_files": 2,
  "missing_files": 0,
  "issues": [
    {
      "type": "orphaned_file",
      "path": "storage/images/jpg/orphan.jpg",
      "message": "File exists on disk but not in metadata index"
    },
    {
      "type": "missing_file",
      "path": "storage/documents/pdf/missing.pdf",
      "hash": "def456abc789...",
      "message": "File in metadata index but not found on disk"
    },
    {
      "type": "hash_mismatch",
      "path": "storage/videos/mp4/corrupted.mp4",
      "hash": "123abc456def...",
      "message": "Stored hash 123abc456def does not match actual hash 789def123abc"
    }
  ]
}
```

**Issue Types:**
- `orphaned_file`: File exists on disk but not in metadata index
- `missing_file`: File in index but not found on disk
- `hash_mismatch`: Stored hash doesn't match actual file hash (data corruption)

**Example:**
```bash
curl -X POST http://localhost:8090/files/duplicates/verify
```

---

### 4. Merge/Cleanup Duplicates

Removes duplicate files, keeping only one copy.

**Endpoint:** `POST /files/duplicates/merge`

**Request Body:**
```json
{
  "hash": "abc123def456...",
  "keep": "storage/images/jpg/abc123_photo.jpg",
  "remove_others": true
}
```

**Parameters:**
- `hash` (string, required): Hash of the duplicate group
- `keep` (string, required): Path of the file to keep
- `remove_others` (boolean, required): Whether to actually remove duplicates (false = dry run)

**Response:**
```json
{
  "hash": "abc123def456...",
  "kept_file": "storage/images/jpg/abc123_photo.jpg",
  "removed_files": [
    "storage/images/jpg/abc123_vacation.jpg",
    "storage/images/jpg/abc123_trip.jpg"
  ],
  "space_reclaimed": 2097152
}
```

**Example (Dry Run):**
```bash
curl -X POST http://localhost:8090/files/duplicates/merge \
  -H "Content-Type: application/json" \
  -d '{
    "hash": "abc123def456",
    "keep": "storage/images/jpg/abc123_photo.jpg",
    "remove_others": false
  }'
```

**Example (Actual Cleanup):**
```bash
curl -X POST http://localhost:8090/files/duplicates/merge \
  -H "Content-Type: application/json" \
  -d '{
    "hash": "abc123def456",
    "keep": "storage/images/jpg/abc123_photo.jpg",
    "remove_others": true
  }'
```

---

## Use Cases

### 1. System Audit
Verify that the deduplication system is working correctly:
```bash
# Scan for duplicates
curl -X POST http://localhost:8090/files/duplicates/scan \
  -H "Content-Type: application/json" \
  -d '{"include_metadata": false}'

# Verify integrity
curl -X POST http://localhost:8090/files/duplicates/verify
```

### 2. Find and Report Duplicates
Get a comprehensive report of all duplicate files:
```bash
curl -X GET http://localhost:8090/files/duplicates | jq '.'
```

### 3. Cleanup Orphaned Files
Find files that exist on disk but aren't tracked:
```bash
# Verify to find orphans
curl -X POST http://localhost:8090/files/duplicates/verify | \
  jq '.issues[] | select(.type == "orphaned_file")'
```

### 4. Storage Optimization
Calculate storage waste and identify cleanup opportunities:
```bash
# Get storage waste statistics
curl -X GET http://localhost:8090/files/duplicates | \
  jq '{total_groups, total_duplicates, storage_wasted}'
```

### 5. Data Integrity Check
Verify file hashes haven't changed (detect corruption):
```bash
curl -X POST http://localhost:8090/files/duplicates/verify | \
  jq '.issues[] | select(.type == "hash_mismatch")'
```

---

## Integration with Existing Features

### Deduplication on Upload
RhinoBox automatically prevents duplicate uploads:
- When a file is uploaded, its SHA256 hash is computed
- The system checks if a file with the same hash exists
- If found, the upload is marked as duplicate and no file is stored
- The duplicate endpoint `/files/duplicates` will show 0 duplicates if deduplication is working

### File Storage Structure
Files are organized in a hierarchical structure:
```
data/
  storage/
    images/jpg/
    images/png/
    videos/mp4/
    audio/mp3/
    documents/pdf/
    documents/txt/
  metadata/
    files.json
```

---

## Error Handling

All endpoints return appropriate HTTP status codes:
- `200 OK`: Successful operation
- `400 Bad Request`: Invalid request parameters
- `500 Internal Server Error`: Server-side error

Error responses have the format:
```json
{
  "error": "descriptive error message"
}
```

---

## Performance Considerations

- **Scan operation**: O(n) where n is the number of files in metadata index
- **Verify operation**: O(n) where n is the number of physical files (involves disk I/O)
- **Hash verification**: Can be expensive for large files; use `deep_scan: false` for faster scans
- **Large datasets**: Tested with 100+ files, performs efficiently

---

## Notes

1. **Deduplication Works**: The system prevents duplicates at upload time, so typically you'll see 0 duplicates
2. **Verification is Important**: Use verify to detect orphaned files, missing files, or data corruption
3. **Merge is Destructive**: Use `remove_others: false` first to see what would be removed
4. **Hash Algorithm**: Uses SHA256 for file hashing (collision-resistant)
5. **Concurrency Safe**: All operations are thread-safe with proper locking
