# RhinoBox

**Intelligent Multi-Modal Storage System** - A Go-based service that automatically categorizes and stores media files and JSON documents with intelligent routing.

## 🎯 Key Features

- **Unified Ingest Endpoint**: Single `/ingest` endpoint handles images, videos, audio, JSON, and generic files
- **Intelligent JSON Routing**: Automatically decides between SQL (relational) vs NoSQL (document) storage based on schema analysis
- **Multi-Level Caching**: LRU + Bloom filters + BadgerDB for 3.6M ops/sec with 100% hit rate
- **Content Deduplication**: SHA-256 based deduplication saves storage and processing time
- **Smart Media Classification**: MIME-based categorization with content-based deduplication
- **Advanced Search**: Full-text search across filenames with fuzzy matching and filtering
- **File Management**: List, search, download, stream, delete, and update file metadata
- **Parallel Processing**: Optimized concurrent file handling with worker pools
- **Production Ready**: HTTP/2, graceful shutdown, structured logging, health checks

## 🏗️ Architecture

```
backend/
  cmd/rhinobox/main.go          # Application entrypoint with HTTP/2
  internal/
    api/                         # HTTP handlers and routing
      server.go                  # Chi router, middleware, streaming
      ingest.go                  # Unified ingest endpoint
    cache/                       # Multi-level caching system
      cache.go                   # LRU + Bloom + BadgerDB
      dedup.go                   # Content-addressed deduplication
      schema.go                  # Schema decision caching
    media/                       # Media classification and processing
      categorizer.go             # MIME type detection
      processor.go               # Parallel upload handling
    jsonschema/                  # JSON analysis and decision engine
      analyzer.go                # Schema structure analysis
      decision.go                # SQL vs NoSQL decision logic
      cached_analyzer.go         # Cache-aware analyzer
    storage/                     # File persistence layer
      local.go                   # Filesystem operations
      search.go                  # Full-text search with fuzzy matching
      listing.go                 # File listing with pagination
      delete.go                  # File deletion with validation
    config/                      # Configuration management
```

## 🚀 Quick Start

### Running Locally

```bash
cd backend
go run ./cmd/rhinobox
```

Server starts on `http://localhost:8090`

### Docker

```bash
cd backend
docker build -t rhinobox .
docker run -p 8090:8090 -v ./data:/data rhinobox
```

## 📡 API Endpoints

| Method | Endpoint                     | Description                                    |
| ------ | ---------------------------- | ---------------------------------------------- |
| GET    | `/healthz`                   | Health check endpoint                          |
| POST   | `/ingest`                    | Unified endpoint for all file types            |
| POST   | `/ingest/media`              | Media-specific upload (images, videos, audio)  |
| POST   | `/ingest/json`               | JSON document ingestion with decision engine   |
| GET    | `/files/search`              | Search files by name with fuzzy matching       |
| GET    | `/files/list`                | List files with pagination and filtering       |
| GET    | `/files/download`            | Download file by hash or path                  |
| GET    | `/files/stream`              | Stream file with range request support         |
| GET    | `/files/metadata`            | Get file metadata without downloading          |
| DELETE | `/files/{file_id}`           | Delete file by ID                              |
| PATCH  | `/files/{file_id}/metadata`  | Update file metadata                           |
| PATCH  | `/files/rename`              | Rename a file                                  |
| POST   | `/files/metadata/batch`      | Batch update file metadata                     |

**See `docs/API_REFERENCE.md` for detailed API documentation.**

## 💡 Example Usage

### Unified Endpoint (Mixed Batch)

```bash
curl -X POST http://localhost:8090/ingest \
  -F "files=@photo.jpg" \
  -F "files=@data.json" \
  -F "namespace=demo" \
  -F "comment=mixed upload test"
```

### Media Upload

```bash
curl -X POST http://localhost:8090/ingest/media \
  -F "file=@image.png" \
  -F "file=@video.mp4" \
  -F "category=demo"
```

### JSON with Decision Engine

```bash
curl -X POST http://localhost:8090/ingest/json \
  -H "Content-Type: application/json" \
  -d '{
    "namespace": "products",
    "comment": "product catalog",
    "documents": [
      {"id": 1, "name": "Laptop", "price": 999.99}
    ]
  }'
```

### Search Files

```bash
# Search by name with fuzzy matching
curl "http://localhost:8090/files/search?query=photo&limit=10"

# Filter by category and MIME type
curl "http://localhost:8090/files/search?query=vacation&category=images&mime_type=image/jpeg"
```

### List Files

```bash
# List all files with pagination
curl "http://localhost:8090/files/list?page=1&page_size=50"

# List with filtering and sorting
curl "http://localhost:8090/files/list?category=videos&sort_by=size&sort_order=desc"
```

### Download & Stream

```bash
# Download file by hash
curl "http://localhost:8090/files/download?hash=abc123..." -o file.jpg

# Stream with range support
curl "http://localhost:8090/files/stream?path=media/videos/demo/video.mp4" \
  -H "Range: bytes=0-1023"
```

## ⚙️ Configuration

| Variable                 | Default  | Description              |
| ------------------------ | -------- | ------------------------ |
| `RHINOBOX_ADDR`          | `:8090`  | HTTP server bind address |
| `RHINOBOX_DATA_DIR`      | `./data` | Storage root directory   |
| `RHINOBOX_MAX_UPLOAD_MB` | `512`    | Maximum upload size (MB) |

## 📁 Storage Structure

```
data/
  storage/
    images/png/<category>/<hash>_<filename>
    videos/mp4/<category>/<hash>_<filename>
    audio/mp3/<category>/<hash>_<filename>
  json/
    sql/<namespace>/
      batch_YYYYMMDDTHHMMSSZ.ndjson
      schema.json
    nosql/<namespace>/
      batch_YYYYMMDDTHHMMSSZ.ndjson
  cache/                  # BadgerDB persistent cache (L3)
    MANIFEST
    *.sst                 # SST files for fast lookups
    *.vlog                # Value logs
  metadata/
    files.json            # File metadata index with search data
```

## 🧪 Testing

```bash
cd backend
go test ./...                    # Unit tests
go test -run Integration ./...   # Integration tests
```

## 📚 Documentation

- **[API Reference](docs/API_REFERENCE.md)** - Complete API documentation with request/response schemas
- **[Architecture](docs/ARCHITECTURE.md)** - System design and component overview
- **[Cache Implementation](backend/docs/CACHE_IMPLEMENTATION.md)** - Multi-level caching system details
- **[Docker Guide](docs/DOCKER.md)** - Container deployment instructions

## 🔑 Key Implementation Details

### Multi-Level Intelligent Cache

- **L1 (LRU)**: In-memory cache with 10K items, 5-min TTL - 231 ns/op reads
- **L2 (Bloom Filter)**: 1M items at 0.01% false positive rate - fast negative lookups
- **L3 (BadgerDB)**: Persistent on-disk storage - survives restarts
- **Performance**: 3.6M ops/sec throughput, 100% hit rate, <1µs latency
- **Deduplication**: SHA-256 content-addressed storage saves bandwidth
- **Schema Caching**: 30-min TTL for JSON routing decisions

### Intelligent JSON Decision Engine

- Analyzes schema structure (depth, field stability, relationships)
- **SQL route**: Flat schemas with stable fields, generates PostgreSQL DDL
- **NoSQL route**: Nested/dynamic schemas, optimized for document stores
- Confidence scoring and detailed decision reasoning
- Cache-aware analyzer for instant repeated schema analysis

### Smart Media Processing

- MIME-based classification with fallback detection
- Content-based deduplication using SHA-256 hashing
- Parallel worker pools for concurrent uploads
- Automatic directory organization by type and category
- Metadata tracking with search and listing capabilities

### Advanced Search & Listing

- Full-text search across filenames with fuzzy matching
- Filter by category, MIME type, date range, size range
- Pagination with configurable page size
- Sort by name, date, size (ascending/descending)
- Metadata-based filtering and querying

### Production Features

- HTTP/2 with 1000 concurrent streams and optimized timeouts
- Structured logging with custom lightweight middleware
- Graceful shutdown with 10-second timeout
- Gzip compression (level 5) for responses
- Request timeout and size limits
- Health check endpoint for monitoring
- TCP keepalive (30s) for connection stability
