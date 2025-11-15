# Step-by-Step Guide to Fix the Collection Stats Endpoint PR

## Overview
The PR adds a collection statistics endpoint, but there were some issues that needed fixing:
1. Duplicate route definitions causing conflicts
2. Old handler function that wasn't using the correct storage method
3. Test helper function availability

## Step 1: Verify Current State

First, let's check what's currently in the code:

### Check the routes in `backend/internal/api/server.go`:
```bash
# Look at lines 53-77
grep -A 25 "func (s *Server) routes()" backend/internal/api/server.go
```

**Expected:** You should see only ONE route:
```go
r.Get("/collections/{type}/stats", s.handleGetCollectionStats)
```

**If you see TWO routes** (one with `{collection_type}` and one with `{type}`), that's the problem!

---

## Step 2: Remove Duplicate Route (IF NEEDED)

If you see duplicate routes, open `backend/internal/api/server.go` and:

1. **Find the routes() function** (around line 53)
2. **Look for these two lines:**
   ```go
   r.Get("/collections/{collection_type}/stats", s.handleCollectionStats)
   r.Get("/collections/{type}/stats", s.handleGetCollectionStats)
   ```
3. **Delete the first line** (the one with `{collection_type}`)
4. **Keep only this line:**
   ```go
   r.Get("/collections/{type}/stats", s.handleGetCollectionStats)
   ```

---

## Step 3: Remove Old Handler Function (IF NEEDED)

1. **Search for `handleCollectionStats` function:**
   ```bash
   grep -n "func (s \*Server) handleCollectionStats" backend/internal/api/server.go
   ```

2. **If it exists**, find it (around line 901-942) and **delete the entire function**.

3. **Keep only `handleGetCollectionStats`** which should look like:
   ```go
   func (s *Server) handleGetCollectionStats(w http.ResponseWriter, r *http.Request) {
       collectionType := chi.URLParam(r, "type")
       if collectionType == "" {
           httpError(w, http.StatusBadRequest, "collection type is required")
           return
       }
       
       stats, err := s.storage.GetCollectionStats(collectionType)
       if err != nil {
           httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get collection stats: %v", err))
           return
       }
       
       writeJSON(w, http.StatusOK, stats)
   }
   ```

---

## Step 4: Verify Storage Method Exists

Check that `GetCollectionStats` exists in the storage package:

```bash
grep -n "func (m \*Manager) GetCollectionStats" backend/internal/storage/collections.go
```

**Expected:** Should find the function around line 42.

---

## Step 5: Verify Test Helper Function

The test uses `uploadTestFile` which should be in `backend/tests/integration/file_retrieval_test.go`.

Check it exists:
```bash
grep -n "func uploadTestFile" backend/tests/integration/file_retrieval_test.go
```

**Expected:** Should find it around line 288 with signature:
```go
func uploadTestFile(t *testing.T, srv *api.Server, filename string, content []byte, mimeType string) (string, string)
```

---

## Step 6: Build and Test

### Build the project:
```bash
cd backend
go build ./cmd/rhinobox
```

**Expected:** Should compile without errors.

### Run the specific test:
```bash
go test ./tests/integration/... -v -run TestCollectionStatsEndToEnd
```

**Note:** On Windows, you may see a cleanup error about file locking. This is a known Windows issue and doesn't mean the test failed. Look for actual test failures in the output.

---

## Step 7: Manual Testing (Optional but Recommended)

### Start the server:
```bash
cd backend
go run ./cmd/rhinobox
```

### In another terminal, test the endpoint:
```bash
# Test empty collection
curl http://localhost:8090/collections/images/stats

# Expected response:
# {
#   "type": "images",
#   "file_count": 0,
#   "storage_used": 0,
#   "storage_used_formatted": "0 B"
# }
```

### Upload a test file and check stats:
```bash
# Upload an image
curl -X POST http://localhost:8090/ingest/media \
  -F "file=@test.jpg"

# Get stats again
curl http://localhost:8090/collections/images/stats

# Should show file_count: 1 and storage_used > 0
```

---

## Step 8: Verify Response Format

The endpoint should return JSON with these fields:
- `type` (string): The collection type (e.g., "images", "videos")
- `file_count` (number): Number of files in the collection
- `storage_used` (number): Total bytes used
- `storage_used_formatted` (string): Human-readable format (e.g., "1.23 KB")

---

## Common Issues and Solutions

### Issue 1: "404 Not Found"
**Cause:** Route not registered or wrong URL
**Solution:** 
- Check route is in `routes()` function
- Verify URL is `/collections/{type}/stats` (not `/collections/{collection_type}/stats`)

### Issue 2: "500 Internal Server Error"
**Cause:** `GetCollectionStats` method missing or error in storage
**Solution:**
- Verify `backend/internal/storage/collections.go` exists
- Check `GetCollectionStats` method is implemented

### Issue 3: Test fails with "file_count missing"
**Cause:** Response format doesn't match expected
**Solution:**
- Verify handler returns `*CollectionStats` struct directly
- Check JSON tags match: `type`, `file_count`, `storage_used`, `storage_used_formatted`

### Issue 4: Files not categorized correctly
**Cause:** File extensions not recognized
**Solution:**
- Check `backend/internal/storage/classifier.go` has mappings for your file types
- Verify files are being uploaded with correct extensions

---

## Summary Checklist

- [ ] Only ONE route exists: `/collections/{type}/stats`
- [ ] Only `handleGetCollectionStats` handler exists (not `handleCollectionStats`)
- [ ] Handler calls `s.storage.GetCollectionStats(collectionType)`
- [ ] `GetCollectionStats` method exists in `storage/collections.go`
- [ ] Response format matches: `type`, `file_count`, `storage_used`, `storage_used_formatted`
- [ ] Project builds successfully
- [ ] Test passes (or only shows Windows cleanup warning)

---

## Files Modified

1. `backend/internal/api/server.go` - Routes and handler
2. `backend/internal/storage/collections.go` - Storage method (should already exist)
3. `backend/tests/integration/collection_stats_e2e_test.go` - Test file (should already exist)

---

## Next Steps After Fixing

1. Commit your changes:
   ```bash
   git add backend/internal/api/server.go
   git commit -m "fix: remove duplicate collection stats route and use GetCollectionStats"
   ```

2. Push to your branch:
   ```bash
   git push origin feature/collection-stats-endpoint
   ```

3. Create/update PR with description of the fix

