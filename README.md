# RhinoBox

**Intelligent Multi-Modal Storage System** - A Go-based service that automatically categorizes and stores media files and JSON documents with intelligent routing.

## 🎯 Key Features

- **Unified Ingest Endpoint**: Single `/ingest` endpoint handles images, videos, audio, JSON, and generic files
- **Intelligent JSON Routing**: Automatically decides between SQL (relational) vs NoSQL (document) storage based on schema analysis
- **Smart Media Classification**: MIME-based categorization with content-based deduplication
- **Parallel Processing**: Optimized concurrent file handling with worker pools
- **Production Ready**: Graceful shutdown, structured logging, health checks

## 🏗️ Architecture

```
backend/
  cmd/rhinobox/main.go          # Application entrypoint
  internal/
    api/                         # HTTP handlers and routing
      server.go                  # Chi router, middleware
      ingest.go                  # Unified ingest endpoint
    media/                       # Media classification and processing
      categorizer.go             # MIME type detection
      processor.go               # Parallel upload handling
    jsonschema/                  # JSON analysis and decision engine
      analyzer.go                # Schema structure analysis
      decision.go                # SQL vs NoSQL decision logic
    storage/                     # File persistence layer
      local.go                   # Filesystem operations
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

| Method | Endpoint        | Description                                   |
| ------ | --------------- | --------------------------------------------- |
| GET    | `/healthz`      | Health check endpoint                         |
| POST   | `/ingest`       | Unified endpoint for all file types           |
| POST   | `/ingest/media` | Media-specific upload (images, videos, audio) |
| POST   | `/ingest/json`  | JSON document ingestion with decision engine  |

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
  metadata/
    files.ndjson          # Ingestion logs
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
- **[Docker Guide](docs/DOCKER.md)** - Container deployment instructions

## 🔑 Key Implementation Details

### Intelligent JSON Decision Engine

- Analyzes schema structure (depth, field stability, relationships)
- **SQL route**: Flat schemas with stable fields, generates PostgreSQL DDL
- **NoSQL route**: Nested/dynamic schemas, optimized for document stores
- Confidence scoring and detailed decision reasoning

### Smart Media Processing

- MIME-based classification with fallback detection
- Content-based deduplication using SHA-256 hashing
- Parallel worker pools for concurrent uploads
- Automatic directory organization by type and category

### Production Features

- Structured logging with `slog`
- Graceful shutdown handling
- Request timeout and size limits
- Health check endpoint for monitoring
