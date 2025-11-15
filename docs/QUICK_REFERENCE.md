# RhinoBox New Features - Quick Reference

## 🔍 Enhanced Search API

### Content Search in Text Files

Search inside text files (txt, md, json, xml, js, ts, py, sh, etc.)

**Endpoint:**

```
GET /files/search?content=<search_term>
```

**Examples:**

```bash
# Search for "TODO" in all text files
curl "http://localhost:8090/files/search?content=TODO"

# Search for "database" in config files
curl "http://localhost:8090/files/search?name=config&content=database"

# Search in JSON files only
curl "http://localhost:8090/files/search?extension=json&content=password"
```

**Features:**

- ✅ Case-insensitive search
- ✅ Supports 10+ text file types
- ✅ Max 10 MB file size (performance limit)
- ✅ Line-by-line scanning
- ✅ Combines with other filters

**Supported MIME Types:**

- `text/*` (all text files)
- `application/json`
- `application/xml`
- `application/javascript`
- `application/typescript`
- `application/x-sh`
- `application/x-python`

---

## 🔄 Automatic Retry Logic

### Exponential Backoff for Failed Uploads

**Default Configuration:**

```
Max Attempts:   3
Initial Delay:  1 second
Max Delay:      30 seconds
Multiplier:     2.0x
```

**Retry Timeline:**

```
Attempt 1: Immediate (0s)
Attempt 2: After 1s delay
Attempt 3: After 2s delay
Total:     ~3 seconds
```

**What Gets Retried:**

- File upload failures
- Network timeouts
- Temporary connection issues
- Transient storage errors

**What Doesn't Get Retried:**

- Invalid file formats
- Permission errors
- Quota exceeded errors
- Context cancellation

**Integration:**
Automatic - no configuration needed! All async job processing includes retry logic by default.

---

## 📁 Organized Test Results

All test documentation moved to `backend/tests/e2e-results/`:

```
backend/tests/e2e-results/
├── README.md                            ← Start here
├── E2E_STRESS_TEST_INDEX.md             ← Navigation guide
├── E2E_STRESS_TEST_VISUAL_DASHBOARD.md  ← Visual summary
├── E2E_STRESS_TEST_SUMMARY.md           ← Executive summary
├── E2E_STRESS_TEST_REPORT.md            ← Full report
├── E2E_STRESS_TEST_DETAILED_METRICS.md  ← Technical metrics
├── stress_test_e2e.ps1                  ← Automated test script
└── stress_test_results_*.json           ← Raw data
```

---

## ⚙️ Configuration Reference

### File Upload Limits

**Environment Variable:**

```bash
RHINOBOX_MAX_UPLOAD_MB=512
```

**Docker Compose:**

```yaml
environment:
  RHINOBOX_MAX_UPLOAD_MB: "512"
```

**Code:**

```go
// internal/config/config.go
maxUploadBytes := int64(512 * 1024 * 1024) // 512 MB default
```

---

## 🐳 Docker Quick Start

### Build and Run

```bash
docker-compose up -d
```

### Services

- **RhinoBox API**: `http://localhost:8090`
- **PostgreSQL**: `localhost:5432`
- **MongoDB**: `localhost:27017`

### Check Logs

```bash
docker-compose logs -f rhinobox
```

### Health Check

```bash
curl http://localhost:8090/healthz
```

---

## 🧪 Testing

### Run Stress Test

```powershell
cd backend\tests\e2e-results
.\stress_test_e2e.ps1 -TestDir "C:\Your\Test\Directory"
```

### Test Content Search

```bash
# Upload a text file
curl -X POST http://localhost:8090/ingest \
  -F "files=@README.md"

# Search its content
curl "http://localhost:8090/files/search?content=RhinoBox"
```

### Verify Retry Logic

Check job processing logs for retry attempts:

```bash
docker-compose logs -f rhinobox | grep -i retry
```

---

## 📊 Performance Metrics

### Verified Performance

- **Upload Throughput**: 618 MB/s
- **Search Response**: 6-10ms
- **Job Completion**: 100%
- **Categorization**: 100% accuracy
- **Retry Success**: 3 attempts per failure

### Limits

- **Max Upload Size**: 512 MB (configurable)
- **Content Search**: 10 MB max file size
- **Retry Attempts**: 3 (configurable)
- **Max Retry Delay**: 30 seconds

---

## 🔧 Troubleshooting

### Search Returns No Results

✅ **FIXED** - Search now works with original filenames

### Large File Upload Fails

⚠️ Files >1 GB may timeout (working as designed)

- **Solution**: Increase `RHINOBOX_MAX_UPLOAD_MB`
- **Future**: Chunked upload support planned

### Connection Drops During Upload

✅ **FIXED** - Automatic retry with exponential backoff

---

## 📚 Documentation

- **Main README**: `README.md`
- **API Reference**: `docs/API_REFERENCE.md`
- **Architecture**: `docs/ARCHITECTURE.md`
- **Improvements**: `IMPROVEMENTS_SUMMARY.md`
- **Test Results**: `backend/tests/e2e-results/README.md`

---

## 🎯 Next Steps

1. ✅ Content search - **DONE**
2. ✅ Automatic retry - **DONE**
3. ✅ Test organization - **DONE**
4. 📋 Chunked upload - **PLANNED**
5. 📋 Resumable uploads - **PLANNED**

---

**Last Updated**: November 15, 2025  
**Version**: 1.0  
**Status**: Production Ready ✅
