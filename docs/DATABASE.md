# Database Integration Guide

RhinoBox integrates with **PostgreSQL** (for SQL-routed data) and **MongoDB** (for NoSQL-routed data) with high-performance connection pooling and batch operations.

## 🎯 Performance Targets

- **PostgreSQL**: 100K+ inserts/sec using COPY protocol
- **MongoDB**: 200K+ inserts/sec using BulkWrite
- **Connection Acquisition**: <1ms from pool
- **Zero connection leaks**: Automatic connection management
- **Auto-reconnect**: On connection loss

## 🚀 Quick Start

### Using Docker Compose (Recommended)

```bash
# Start all services (PostgreSQL + MongoDB + RhinoBox)
docker-compose up -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f rhinobox

# Stop all services
docker-compose down
```

### Manual Setup

#### PostgreSQL Setup

```bash
# Using Docker
docker run -d \
  --name rhinobox-postgres \
  -e POSTGRES_USER=rhinobox \
  -e POSTGRES_PASSWORD=rhinobox_dev \
  -e POSTGRES_DB=rhinobox \
  -p 5432:5432 \
  postgres:16-alpine

# Set environment variable
export RHINOBOX_POSTGRES_URL="postgres://rhinobox:rhinobox_dev@localhost:5432/rhinobox?sslmode=disable"
```

#### MongoDB Setup

```bash
# Using Docker
docker run -d \
  --name rhinobox-mongo \
  -e MONGO_INITDB_ROOT_USERNAME=rhinobox \
  -e MONGO_INITDB_ROOT_PASSWORD=rhinobox_dev \
  -p 27017:27017 \
  mongo:7

# Set environment variable
export RHINOBOX_MONGO_URL="mongodb://rhinobox:rhinobox_dev@localhost:27017"
```

## ⚙️ Configuration

### Environment Variables

| Variable                | Default               | Description                  |
| ----------------------- | --------------------- | ---------------------------- |
| `RHINOBOX_POSTGRES_URL` | (empty - NDJSON only) | PostgreSQL connection string |
| `RHINOBOX_MONGO_URL`    | (empty - NDJSON only) | MongoDB connection string    |
| `RHINOBOX_DB_MAX_CONNS` | `100`                 | Max database connections     |

**Note**: If database URLs are not provided, RhinoBox operates in **NDJSON-only mode** (no actual database writes).

### Connection String Formats

**PostgreSQL**:

```
postgres://username:password@host:port/database?sslmode=disable
```

**MongoDB**:

```
mongodb://username:password@host:port
mongodb://username:password@host:port/?authSource=admin
```

## 📊 How It Works

### Decision Engine Integration

When JSON documents are ingested via `/ingest/json`:

1. **Schema Analysis**: Analyzer examines structure, stability, and relationships
2. **Decision**: Engine chooses SQL (PostgreSQL) or NoSQL (MongoDB)
3. **Database Write**: Documents inserted into chosen database
4. **NDJSON Backup**: Also saved to file system as backup/audit trail

### SQL Route (PostgreSQL)

**Criteria**:

- Stable, consistent schema
- Foreign key patterns (`*_id` fields)
- Shallow nesting (depth ≤ 2)
- Relational structure

**Implementation**:

- Auto-generates `CREATE TABLE` with typed columns
- Uses **COPY protocol** for bulk inserts (100K+/sec)
- Prepared statement caching (1024 statements)
- Connection pooling (4x CPU cores)

**Example**:

```json
{
  "namespace": "orders",
  "documents": [
    { "order_id": 1, "customer_id": 100, "total": 250.0 },
    { "order_id": 2, "customer_id": 101, "total": 180.5 }
  ]
}
```

→ Creates `orders` table, inserts via COPY protocol

### NoSQL Route (MongoDB)

**Criteria**:

- Deep nesting (depth > 3)
- Inconsistent schema
- Array/object-heavy structure
- Comment hints: "flexible", "nosql"

**Implementation**:

- Uses **BulkWrite** with unordered execution (200K+/sec)
- Connection pooling (100 max connections)
- Wire compression (snappy/zstd)
- Parallel write execution

**Example**:

```json
{
  "namespace": "activity_logs",
  "comment": "flexible schema nosql",
  "documents": [
    {
      "user": { "id": "u1", "name": "Alice" },
      "events": [{ "type": "click", "timestamp": "2025-11-15T10:00:00Z" }]
    }
  ]
}
```

→ Inserts into `rhinobox.activity_logs` collection

## 🧪 Testing & Benchmarks

### Run Benchmarks

```bash
# Ensure databases are running
docker-compose up -d postgres mongodb

# Run PostgreSQL benchmarks
cd backend
go test -bench=BenchmarkPostgres -benchmem ./internal/database

# Run MongoDB benchmarks
go test -bench=BenchmarkMongo -benchmem ./internal/database

# Run all benchmarks
go test -bench=. -benchmem ./internal/database
```

### Expected Results

**PostgreSQL COPY (batch=1000)**:

```
BenchmarkPostgresCopyInsert/copy_1000-12    1000    500000 ns/op    100000+ inserts/sec
```

**MongoDB BulkWrite (batch=1000)**:

```
BenchmarkMongoBulkInsert/bulk_1000-12       2000    400000 ns/op    200000+ inserts/sec
```

**Connection Acquisition**:

```
BenchmarkPostgresConnectionAcquisition-12   50000     30000 ns/op    <1ms
```

### Integration Testing

```bash
# Test with curl
curl -X POST http://localhost:8090/ingest/json \
  -H "Content-Type: application/json" \
  -d '{
    "namespace": "test_orders",
    "documents": [
      {"id": 1, "product": "Laptop", "price": 999.99},
      {"id": 2, "product": "Mouse", "price": 29.99}
    ]
  }'

# Check PostgreSQL
docker exec -it rhinobox-postgres psql -U rhinobox -d rhinobox -c "SELECT * FROM test_orders;"

# Check MongoDB
docker exec -it rhinobox-mongo mongosh -u rhinobox -p rhinobox_dev --eval "db.getSiblingDB('rhinobox').test_collection.find().pretty()"
```

## 🔧 Connection Pool Tuning

### PostgreSQL Pool Settings

**Current Configuration** (in `internal/database/postgres.go`):

```go
config.MaxConns = int32(runtime.NumCPU() * 4)  // e.g., 48 on 12-core
config.MinConns = int32(runtime.NumCPU())      // e.g., 12 on 12-core
config.MaxConnIdleTime = 5 * time.Minute
config.MaxConnLifetime = 1 * time.Hour
config.StatementCacheCapacity = 1024           // 1024 prepared statements
```

**Tuning Guidelines**:

- `MaxConns`: 2-4x CPU cores for write-heavy workloads
- `MinConns`: 1x CPU cores to keep warm connections
- `StatementCacheCapacity`: Increase if using many different schemas

### MongoDB Pool Settings

**Current Configuration** (in `internal/database/mongodb.go`):

```go
SetMaxPoolSize(100)                            // 100 connections
SetMinPoolSize(10)                             // 10 warm connections
SetCompressors([]string{"snappy", "zstd"})     // Wire compression
```

**Tuning Guidelines**:

- `MaxPoolSize`: 100-200 for high-throughput scenarios
- `MinPoolSize`: 10-20 to reduce latency spikes
- Enable compression for large documents

## 📈 Performance Optimization Tips

### PostgreSQL

1. **Use COPY for Bulk Inserts**

   - Automatically used for batches >100 documents
   - 10-100x faster than individual INSERTs
   - Bypasses query parsing, uses binary protocol

2. **Batch Size Tuning**

   - Default: 1000 documents per batch
   - Adjust in `BatchInsertJSON()` if needed
   - Larger batches = higher throughput but more memory

3. **Table Indexes**
   - Add indexes after bulk insert, not before
   - Use `CREATE INDEX CONCURRENTLY` for live tables

### MongoDB

1. **Unordered BulkWrite**

   - Already configured (`SetOrdered(false)`)
   - Allows parallel execution across shards
   - Continues on individual write failures

2. **Write Concern**

   - Default: `w:1` (acknowledged)
   - For max speed: `w:0` (unacknowledged, risky)
   - For safety: `w:majority`

3. **Sharding**
   - For 1M+ documents, consider sharding
   - Shard key should match query patterns

## 🛡️ Resilience Features

### Auto-Reconnect

Both PostgreSQL and MongoDB drivers handle reconnection automatically:

- Connection health checks every 1 minute
- Automatic retry on transient failures
- Pool replenishment on connection loss

### Graceful Shutdown

On server stop:

```go
s.Stop()  // Closes job queue, PostgreSQL, and MongoDB connections
```

### Error Handling

**Partial Failures**:

- MongoDB BulkWrite continues even if some documents fail
- Returns detailed error information per document
- Successful writes are committed

**Connection Failures**:

- Logs error and continues with NDJSON-only mode
- API remains operational
- No data loss (all writes backed up to NDJSON)

## 📁 Data Storage

### Database + NDJSON Dual Storage

**Why Both?**

- **Database**: Fast queries, transactions, indexes
- **NDJSON**: Backup, audit trail, database-independent portability

**Storage Path**:

```
data/
  json/
    sql/<namespace>/
      batch_YYYYMMDDTHHMMSSZ.ndjson
      schema.json
    nosql/<namespace>/
      batch_YYYYMMDDTHHMMSSZ.ndjson
  ingest_log.ndjson
```

### Database Schema

**PostgreSQL**:

- Tables created dynamically based on schema analysis
- Column types: `BIGINT`, `DOUBLE PRECISION`, `TEXT`, `BOOLEAN`, `TIMESTAMP`
- Auto-generates table name from namespace

**MongoDB**:

- Collections named after namespace: `rhinobox.<namespace>`
- Schema-less (documents stored as-is)
- Augmented with `_ingest_id`, `ingested_at` metadata

## 🔍 Monitoring

### Connection Pool Stats

```go
// PostgreSQL
stats := db.Stats()
fmt.Printf("Total connections: %d\n", stats.TotalConns())
fmt.Printf("Idle connections: %d\n", stats.IdleConns())
fmt.Printf("Acquired connections: %d\n", stats.AcquiredConns())
```

### Health Checks

```bash
# PostgreSQL
docker exec rhinobox-postgres pg_isready -U rhinobox

# MongoDB
docker exec rhinobox-mongo mongosh --eval "db.adminCommand('ping')"

# RhinoBox API
curl http://localhost:8090/healthz
```

## 🐛 Troubleshooting

### "PostgreSQL not available" in tests

```bash
# Ensure PostgreSQL is running
docker ps | grep postgres

# Set test environment variable
export TEST_POSTGRES_URL="postgres://rhinobox:rhinobox_dev@localhost:5432/rhinobox_test?sslmode=disable"
```

### "MongoDB not available" in tests

```bash
# Ensure MongoDB is running
docker ps | grep mongo

# Set test environment variable
export TEST_MONGO_URL="mongodb://rhinobox:rhinobox_dev@localhost:27017"
```

### High connection pool exhaustion

**Symptoms**: Slow queries, "pool exhausted" errors

**Solutions**:

1. Increase `MaxConns` in configuration
2. Check for connection leaks (should auto-release)
3. Reduce concurrent requests

### Slow insert performance

**Check**:

1. Batch size (increase for bulk operations)
2. Indexes (disable during bulk insert)
3. Network latency (use localhost/same datacenter)

## 📚 References

- [pgx Performance Guide](https://github.com/jackc/pgx)
- [MongoDB Bulk Operations](https://www.mongodb.com/docs/drivers/go/current/fundamentals/crud/write-operations/bulk/)
- [PostgreSQL COPY Protocol](https://www.postgresql.org/docs/current/sql-copy.html)
- [MongoDB Connection Pooling](https://www.mongodb.com/docs/drivers/go/current/fundamentals/connection/)

## 🔗 Related Documentation

- [API Reference](./API_REFERENCE.md) - JSON ingestion endpoints
- [Architecture](./ARCHITECTURE.md) - System design overview
- [Docker Guide](./DOCKER.md) - Container deployment
