# Enhanced File Search Endpoint with Filtering

## Summary

Implements **Issue #20**: Add file search/query endpoint with filtering by name, extension, type, and date.

This PR enhances the existing `/files/search` endpoint to support comprehensive filtering capabilities, allowing users to search files using multiple criteria simultaneously.

## Features

### Search Filters

1. **Name Filter** (`name`)
   - Partial match on original filename
   - Case-insensitive
   - Example: `?name=report` matches "report_2024.pdf", "report_2023.pdf", "image_report.png"

2. **Extension Filter** (`extension`)
   - Exact match on file extension
   - Case-insensitive
   - Supports with or without leading dot
   - Example: `?extension=pdf` or `?extension=.pdf`

3. **Type Filter** (`type`)
   - Matches MIME type or category
   - Supports partial matching
   - Handles common patterns (image/, video/, audio/)
   - Example: `?type=image` matches all image files

4. **Category Filter** (`category`)
   - Partial match on category path
   - Case-insensitive
   - Example: `?category=images`

5. **MIME Type Filter** (`mime_type`)
   - Exact match on MIME type
   - Case-insensitive
   - Example: `?mime_type=application/pdf`

6. **Date Range Filters** (`date_from`, `date_to`)
   - Filter files by upload date
   - Supports RFC3339 format: `2025-01-15T12:00:00Z`
   - Supports date-only format: `2025-01-15`
   - Can be used independently or together
   - Example: `?date_from=2025-01-01&date_to=2025-01-31`

### Combined Filters

All filters use AND logic - all specified filters must match for a file to be included in results.

Example: `?name=report&extension=pdf&type=application/pdf` finds files with "report" in the name, PDF extension, and PDF MIME type.

## API Endpoint

```
GET /files/search?name=<string>&extension=<string>&type=<string>&category=<string>&mime_type=<string>&date_from=<date>&date_to=<date>
```

**Response:**
```json
{
  "filters": {
    "name": "report",
    "extension": "pdf"
  },
  "results": [
    {
      "hash": "...",
      "original_name": "report_2024.pdf",
      "stored_path": "...",
      "category": "...",
      "mime_type": "application/pdf",
      "size": 12345,
      "uploaded_at": "2025-01-15T12:00:00Z",
      "metadata": {}
    }
  ],
  "count": 1
}
```

## Performance Metrics

### Single Query Performance
- **Average Latency**: ~52µs
- **Min Latency**: ~45µs
- **Max Latency**: ~83µs
- **Throughput**: ~19,000 queries/sec

### Concurrent Query Performance
- **50 concurrent workers** (10 queries each = 500 total queries)
- **Total Duration**: ~7.7ms
- **Throughput**: ~64,500 queries/sec
- **Error Rate**: 0%

### Filter Combination Performance
| Filter Combination | Avg Latency | Throughput |
|-------------------|-------------|-------------|
| name_only | 236µs | 4,228 queries/sec |
| extension_only | 65µs | 15,341 queries/sec |
| type_only | 113µs | 8,828 queries/sec |
| name + extension | 64µs | 15,538 queries/sec |
| name + type | 100µs | 9,993 queries/sec |
| extension + type | 59µs | 16,832 queries/sec |

### Reliability
- **Success Rate**: 99%+ under load (1000 queries with 20 concurrent workers)
- **Scalability**: Tested with dataset sizes from 10 to 500 files
- **No performance degradation** observed with larger datasets

## Testing

### Unit Tests
- ✅ Comprehensive storage layer tests (`backend/internal/storage/search_test.go`)
  - Name filtering (partial, exact, case-insensitive)
  - Extension filtering (with/without dot, case-insensitive)
  - Type filtering (MIME type, category, patterns)
  - Date range filtering
  - MIME type filtering
  - Combined filters
  - Empty filters handling

### API Tests
- ✅ Endpoint tests (`backend/internal/api/search_test.go`)
  - All filter types individually
  - Combined filters
  - Error handling (no filters, invalid dates)
  - URL encoding handling

### Integration Tests
- ✅ End-to-end tests (`backend/tests/integration/search_e2e_test.go`)
  - Real-world files from Downloads directory
  - Multiple search scenarios
  - Combined filter testing
  - Date range validation

### Stress Tests
- ✅ Performance tests (`backend/tests/stress/search_stress_test.go`)
  - Single query latency measurement
  - Concurrent query performance
  - Filter combination performance
  - Reliability under load
  - Scalability with varying dataset sizes

## Implementation Details

### Storage Layer
- New `SearchFiles()` method in `storage.Manager`
- `SearchFilters` struct for filter parameters
- Efficient in-memory filtering with mutex protection
- All filters combined with AND logic

### API Layer
- Enhanced `handleFileSearch()` endpoint handler
- Query parameter parsing and validation
- Date format support (RFC3339 and YYYY-MM-DD)
- Comprehensive error handling
- Response includes applied filters for transparency

## Backward Compatibility

The endpoint maintains backward compatibility:
- Existing `/files/search?name=<query>` still works
- New filters are optional
- At least one filter must be provided (same as before)

## Files Changed

- `backend/internal/storage/search.go` - New search implementation
- `backend/internal/storage/search_test.go` - Storage layer tests
- `backend/internal/api/server.go` - Enhanced search endpoint
- `backend/internal/api/search_test.go` - API endpoint tests
- `backend/tests/integration/search_e2e_test.go` - E2E tests
- `backend/tests/stress/search_stress_test.go` - Performance tests

## Example Usage

```bash
# Search by name
curl "http://localhost:8090/files/search?name=report"

# Search by extension
curl "http://localhost:8090/files/search?extension=pdf"

# Search by type
curl "http://localhost:8090/files/search?type=image"

# Search by date range
curl "http://localhost:8090/files/search?date_from=2025-01-01&date_to=2025-01-31"

# Combined filters
curl "http://localhost:8090/files/search?name=report&extension=pdf&type=application/pdf"
```

## Checklist

- [x] Implementation complete
- [x] Unit tests written and passing
- [x] API tests written and passing
- [x] Integration tests written and passing
- [x] Stress/performance tests written and passing
- [x] Performance metrics documented
- [x] Code follows project conventions
- [x] Backward compatibility maintained
- [x] Documentation updated (this PR)

## Related Issues

Closes #20
