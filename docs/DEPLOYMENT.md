# Deployment Guide

This guide covers environment setup, configuration, Docker deployment, and production readiness for RhinoBox.

## Table of Contents

1. [Quick Start](#quick-start)
2. [Environment Setup](#environment-setup)
3. [Configuration](#configuration)
4. [Docker Deployment](#docker-deployment)
5. [Production Checklist](#production-checklist)
6. [Monitoring & Operations](#monitoring--operations)
7. [Troubleshooting](#troubleshooting)

---

## Quick Start

### Option 1: Docker Compose (Recommended)

```bash
# Clone repository
git clone https://github.com/Muneer320/RhinoBox.git
cd RhinoBox

# Start all services (RhinoBox + PostgreSQL + MongoDB)
docker-compose up -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f rhinobox

# Test API
curl http://localhost:8090/healthz
# Response: {"status":"ok"}

# Stop all services
docker-compose down
```

**Includes**:

- RhinoBox API (port 8090)
- PostgreSQL 16 (port 5432)
- MongoDB 7 (port 27017)
- Health checks, auto-restart, persistent volumes

### Option 2: Local Development

```bash
# Prerequisites
- Go 1.21+
- (Optional) PostgreSQL 16+
- (Optional) MongoDB 7+

# Install dependencies
cd backend
go mod download

# Run server
go run ./cmd/rhinobox

# Server starts on http://localhost:8090
```

---

## Environment Setup

### System Requirements

**Minimum (Development)**:

- CPU: 2 cores
- RAM: 2GB
- Disk: 10GB
- OS: Linux, macOS, Windows

**Recommended (Production)**:

- CPU: 8+ cores
- RAM: 8GB+
- Disk: 100GB+ SSD
- OS: Linux (Ubuntu 22.04 LTS, RHEL 9)

### Prerequisites

#### Go Installation

```bash
# Linux/macOS
wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Verify
go version
# go version go1.21.5 linux/amd64
```

#### Docker & Docker Compose

```bash
# Install Docker (Ubuntu)
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

# Install Docker Compose
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose

# Verify
docker --version
docker-compose --version
```

---

## Configuration

### Environment Variables

RhinoBox is configured via environment variables for 12-factor app compliance.

#### Core Settings

| Variable                   | Default  | Description                                    |
| -------------------------- | -------- | ---------------------------------------------- |
| `RHINOBOX_ADDR`            | `:8090`  | Server bind address (`:8090` = all interfaces) |
| `RHINOBOX_DATA_DIR`        | `./data` | Root directory for all storage                 |
| `RHINOBOX_LOG_LEVEL`       | `info`   | Log level: `debug`, `info`, `warn`, `error`    |
| `RHINOBOX_MAX_UPLOAD_SIZE` | `100MB`  | Maximum file upload size                       |

#### Database Settings

| Variable                | Default | Description                  |
| ----------------------- | ------- | ---------------------------- |
| `RHINOBOX_POSTGRES_URL` | (empty) | PostgreSQL connection string |
| `RHINOBOX_MONGO_URL`    | (empty) | MongoDB connection string    |
| `RHINOBOX_DB_MAX_CONNS` | `100`   | Max database connections     |

**Note**: If database URLs are empty, RhinoBox operates in **NDJSON-only mode** (no database writes).

#### Connection String Formats

**PostgreSQL**:

```
postgres://username:password@host:port/database?sslmode=disable
postgres://rhinobox:secret@localhost:5432/rhinobox?sslmode=require
```

**MongoDB**:

```
mongodb://username:password@host:port
mongodb://rhinobox:secret@localhost:27017/?authSource=admin
mongodb://user:pass@host1:27017,host2:27017,host3:27017/?replicaSet=rs0
```

#### Job Queue Settings

| Variable                 | Default | Description                      |
| ------------------------ | ------- | -------------------------------- |
| `RHINOBOX_QUEUE_WORKERS` | `10`    | Number of concurrent job workers |
| `RHINOBOX_QUEUE_BUFFER`  | `1000`  | Job queue buffer capacity        |

#### Cache Settings

| Variable              | Default        | Description                      |
| --------------------- | -------------- | -------------------------------- |
| `RHINOBOX_CACHE_DIR`  | `./data/cache` | BadgerDB cache directory         |
| `RHINOBOX_CACHE_SIZE` | `10000`        | LRU cache size (items)           |
| `RHINOBOX_CACHE_TTL`  | `5m`           | LRU cache TTL (e.g., `5m`, `1h`) |

### Configuration Files

#### Example: `.env` file

```bash
# Server
RHINOBOX_ADDR=:8090
RHINOBOX_DATA_DIR=/var/lib/rhinobox
RHINOBOX_LOG_LEVEL=info

# Databases
RHINOBOX_POSTGRES_URL=postgres://rhinobox:secret@postgres:5432/rhinobox?sslmode=disable
RHINOBOX_MONGO_URL=mongodb://rhinobox:secret@mongodb:27017
RHINOBOX_DB_MAX_CONNS=200

# Performance
RHINOBOX_QUEUE_WORKERS=20
RHINOBOX_MAX_UPLOAD_SIZE=500MB

# Cache
RHINOBOX_CACHE_SIZE=50000
RHINOBOX_CACHE_TTL=10m
```

#### Loading Environment

```bash
# Option 1: Export variables
export RHINOBOX_ADDR=:8090
export RHINOBOX_POSTGRES_URL="postgres://..."

# Option 2: Use .env file
set -a
source .env
set +a

# Option 3: Inline with command
RHINOBOX_ADDR=:9000 go run ./cmd/rhinobox
```

---

## Docker Deployment

### Docker Compose (Full Stack)

#### `docker-compose.yml`

```yaml
version: "3.8"

services:
  # PostgreSQL Database
  postgres:
    image: postgres:16-alpine
    container_name: rhinobox-postgres
    environment:
      POSTGRES_USER: rhinobox
      POSTGRES_PASSWORD: rhinobox_dev
      POSTGRES_DB: rhinobox
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./docker/postgres-init.sql:/docker-entrypoint-initdb.d/init.sql
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U rhinobox"]
      interval: 10s
      timeout: 5s
      retries: 5

  # MongoDB Database
  mongodb:
    image: mongo:7
    container_name: rhinobox-mongo
    environment:
      MONGO_INITDB_ROOT_USERNAME: rhinobox
      MONGO_INITDB_ROOT_PASSWORD: rhinobox_dev
    ports:
      - "27017:27017"
    volumes:
      - mongo_data:/data/db
    healthcheck:
      test: ["CMD", "mongosh", "--eval", "db.adminCommand('ping')"]
      interval: 10s
      timeout: 5s
      retries: 5

  # RhinoBox API
  rhinobox:
    build:
      context: ./backend
      dockerfile: Dockerfile
    container_name: rhinobox
    depends_on:
      postgres:
        condition: service_healthy
      mongodb:
        condition: service_healthy
    environment:
      RHINOBOX_ADDR: :8090
      RHINOBOX_DATA_DIR: /data
      RHINOBOX_POSTGRES_URL: postgres://rhinobox:rhinobox_dev@postgres:5432/rhinobox?sslmode=disable
      RHINOBOX_MONGO_URL: mongodb://rhinobox:rhinobox_dev@mongodb:27017
      RHINOBOX_DB_MAX_CONNS: 100
      RHINOBOX_QUEUE_WORKERS: 10
    ports:
      - "8090:8090"
    volumes:
      - rhinobox_data:/data
    restart: unless-stopped
    healthcheck:
      test:
        [
          "CMD",
          "wget",
          "--quiet",
          "--tries=1",
          "--spider",
          "http://localhost:8090/healthz",
        ]
      interval: 30s
      timeout: 10s
      retries: 3

volumes:
  postgres_data:
  mongo_data:
  rhinobox_data:
```

#### Commands

```bash
# Start all services
docker-compose up -d

# View logs (all services)
docker-compose logs -f

# View logs (specific service)
docker-compose logs -f rhinobox

# Check status
docker-compose ps

# Restart service
docker-compose restart rhinobox

# Stop all services
docker-compose stop

# Stop and remove containers
docker-compose down

# Stop and remove volumes (WARNING: deletes data)
docker-compose down -v
```

### Backend Only (Dockerfile)

#### `backend/Dockerfile`

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o rhinobox ./cmd/rhinobox

# Runtime stage
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /build/rhinobox .

# Create data directory
RUN mkdir -p /data

# Expose port
EXPOSE 8090

# Run as non-root user
RUN adduser -D -u 1000 rhinobox
USER rhinobox

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:8090/healthz || exit 1

# Start server
CMD ["./rhinobox"]
```

#### Build & Run

```bash
# Build image
cd backend
docker build -t rhinobox:latest .

# Run container
docker run -d \
  --name rhinobox \
  -p 8090:8090 \
  -v rhinobox_data:/data \
  -e RHINOBOX_ADDR=:8090 \
  rhinobox:latest

# View logs
docker logs -f rhinobox

# Stop container
docker stop rhinobox
docker rm rhinobox
```

---

## Production Checklist

### Security

- [ ] **Use Strong Passwords**: Generate random database passwords
- [ ] **Enable SSL/TLS**: Use `sslmode=require` for PostgreSQL
- [ ] **Firewall Rules**: Restrict database ports (5432, 27017) to application only
- [ ] **Non-Root User**: Run containers as non-root (already configured in Dockerfile)
- [ ] **Secrets Management**: Use Docker secrets or external vault (not env vars in production)
- [ ] **API Authentication**: Add authentication middleware (not included in hackathon version)
- [ ] **Rate Limiting**: Add rate limiting to prevent abuse

```go
// Example: Add authentication middleware
r.Use(func(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        apiKey := r.Header.Get("X-API-Key")
        if apiKey != os.Getenv("RHINOBOX_API_KEY") {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        next.ServeHTTP(w, r)
    })
})
```

### Performance

- [ ] **Database Connections**: Set `RHINOBOX_DB_MAX_CONNS` to 4x CPU cores
- [ ] **Worker Pool**: Set `RHINOBOX_QUEUE_WORKERS` to CPU cores or 2x CPU cores
- [ ] **Cache Size**: Increase `RHINOBOX_CACHE_SIZE` for high-volume workloads
- [ ] **Upload Limits**: Adjust `RHINOBOX_MAX_UPLOAD_SIZE` based on use case
- [ ] **PostgreSQL Tuning**: Optimize `shared_buffers`, `effective_cache_size`, `max_connections`
- [ ] **MongoDB Tuning**: Optimize `wiredTiger` cache size, compression

### Reliability

- [ ] **Health Checks**: Ensure all services have health checks (already configured)
- [ ] **Auto-Restart**: Use `restart: unless-stopped` in docker-compose (already configured)
- [ ] **Backup Strategy**: Schedule regular database backups
- [ ] **Log Rotation**: Configure log rotation to prevent disk fill
- [ ] **Monitoring**: Set up Prometheus + Grafana for metrics
- [ ] **Alerting**: Configure alerts for errors, high latency, resource exhaustion

### Backups

```bash
# PostgreSQL backup
docker exec rhinobox-postgres pg_dump -U rhinobox rhinobox > backup_$(date +%Y%m%d).sql

# PostgreSQL restore
docker exec -i rhinobox-postgres psql -U rhinobox rhinobox < backup_20251115.sql

# MongoDB backup
docker exec rhinobox-mongo mongodump --username=rhinobox --password=rhinobox_dev --out=/backup

# MongoDB restore
docker exec rhinobox-mongo mongorestore --username=rhinobox --password=rhinobox_dev /backup

# RhinoBox data backup (files + cache)
tar -czf rhinobox_data_$(date +%Y%m%d).tar.gz -C /var/lib/docker/volumes/rhinobox_rhinobox_data/_data .
```

### Scaling

- [ ] **Horizontal Scaling**: Deploy multiple RhinoBox instances behind load balancer
- [ ] **Shared Storage**: Use S3/MinIO instead of local filesystem
- [ ] **Database Replication**: Set up PostgreSQL streaming replication
- [ ] **MongoDB Sharding**: Configure MongoDB sharded cluster for >10M documents
- [ ] **Cache Cluster**: Replace BadgerDB with Redis cluster

---

## Monitoring & Operations

### Health Check

```bash
# Check API health
curl http://localhost:8090/healthz
# {"status":"ok"}

# Check with timeout
curl -m 5 http://localhost:8090/healthz || echo "UNHEALTHY"
```

### Logs

```bash
# RhinoBox logs (Docker Compose)
docker-compose logs -f rhinobox

# PostgreSQL logs
docker-compose logs -f postgres

# MongoDB logs
docker-compose logs -f mongodb

# Filter by level
docker-compose logs rhinobox | grep "level=error"

# Tail last 100 lines
docker-compose logs --tail=100 rhinobox
```

### Metrics

```bash
# Check database connections
# PostgreSQL
docker exec rhinobox-postgres psql -U rhinobox -c "SELECT count(*) FROM pg_stat_activity;"

# MongoDB
docker exec rhinobox-mongo mongosh --username rhinobox --password rhinobox_dev --eval "db.serverStatus().connections"

# Check disk usage
docker exec rhinobox df -h /data

# Check memory usage
docker stats rhinobox
```

### Database Access

```bash
# PostgreSQL shell
docker exec -it rhinobox-postgres psql -U rhinobox -d rhinobox

# List tables
\dt

# Query data
SELECT * FROM orders LIMIT 10;

# Exit
\q

# MongoDB shell
docker exec -it rhinobox-mongo mongosh -u rhinobox -p rhinobox_dev

# Switch database
use rhinobox

# List collections
show collections

# Query data
db.activity_logs.find().limit(10).pretty()

# Exit
exit
```

---

## Troubleshooting

### Common Issues

#### 1. "Connection refused" Error

**Symptom**: `dial tcp 127.0.0.1:8090: connect: connection refused`

**Cause**: RhinoBox not running or wrong port

**Solution**:

```bash
# Check if running
docker ps | grep rhinobox

# Check logs
docker logs rhinobox

# Restart
docker-compose restart rhinobox
```

#### 2. "Database not available" Warning

**Symptom**: Logs show "PostgreSQL not available, continuing with NDJSON-only mode"

**Cause**: Database not reachable or wrong credentials

**Solution**:

```bash
# Check database health
docker exec rhinobox-postgres pg_isready -U rhinobox

# Verify connection string
docker exec rhinobox env | grep POSTGRES_URL

# Test connection from RhinoBox container
docker exec rhinobox wget -qO- postgres:5432
```

#### 3. "Permission denied" Error

**Symptom**: `mkdir /data: permission denied`

**Cause**: Container running as wrong user or volume permissions

**Solution**:

```bash
# Fix volume permissions
docker-compose down
sudo chown -R 1000:1000 /var/lib/docker/volumes/rhinobox_rhinobox_data/_data
docker-compose up -d
```

#### 4. High Memory Usage

**Symptom**: RhinoBox using >2GB RAM

**Cause**: Large cache size or memory leak

**Solution**:

```bash
# Reduce cache size
export RHINOBOX_CACHE_SIZE=5000

# Restart with new config
docker-compose restart rhinobox

# Monitor memory
docker stats rhinobox
```

#### 5. Slow Performance

**Symptom**: API latency >100ms

**Cause**: Database connection pool exhaustion or disk I/O bottleneck

**Solution**:

```bash
# Increase database connections
export RHINOBOX_DB_MAX_CONNS=200

# Check disk I/O
docker exec rhinobox iostat -x 1

# Check database slow queries
# PostgreSQL
docker exec rhinobox-postgres psql -U rhinobox -c "SELECT query, calls, mean_exec_time FROM pg_stat_statements ORDER BY mean_exec_time DESC LIMIT 10;"
```

### Debug Mode

```bash
# Enable debug logging
export RHINOBOX_LOG_LEVEL=debug
docker-compose restart rhinobox

# View debug logs
docker-compose logs -f rhinobox | grep "level=debug"
```

### Performance Profiling

```bash
# CPU profiling
curl http://localhost:8090/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof

# Memory profiling
curl http://localhost:8090/debug/pprof/heap > mem.prof
go tool pprof mem.prof

# Goroutine analysis
curl http://localhost:8090/debug/pprof/goroutine?debug=2
```

---

## Production Deployment Example

### AWS EC2 Deployment

```bash
# 1. Launch EC2 instance (Ubuntu 22.04, t3.large, 8GB RAM)

# 2. Install Docker
sudo apt update
sudo apt install -y docker.io docker-compose
sudo usermod -aG docker ubuntu

# 3. Clone repository
git clone https://github.com/Muneer320/RhinoBox.git
cd RhinoBox

# 4. Configure environment
cat > .env <<EOF
RHINOBOX_ADDR=:8090
RHINOBOX_DATA_DIR=/var/lib/rhinobox
RHINOBOX_POSTGRES_URL=postgres://rhinobox:$(openssl rand -hex 16)@postgres:5432/rhinobox?sslmode=require
RHINOBOX_MONGO_URL=mongodb://rhinobox:$(openssl rand -hex 16)@mongodb:27017
RHINOBOX_DB_MAX_CONNS=200
RHINOBOX_QUEUE_WORKERS=16
EOF

# 5. Start services
docker-compose up -d

# 6. Configure firewall
sudo ufw allow 8090/tcp
sudo ufw enable

# 7. Set up SSL (Let's Encrypt)
# Use nginx reverse proxy with certbot

# 8. Configure monitoring
# Set up Prometheus + Grafana
```

---

## Summary

RhinoBox deployment is:

✅ **Simple**: Single binary or Docker Compose  
✅ **Flexible**: NDJSON-only or full database mode  
✅ **Scalable**: Horizontal scaling with load balancer  
✅ **Observable**: Health checks, logs, metrics  
✅ **Production-ready**: Security, backups, monitoring

For production deployments, follow the [Production Checklist](#production-checklist) and configure monitoring/alerting.
