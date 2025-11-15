# RhinoBox Optimizations

This document catalogues the optimizations implemented or planned for RhinoBox. They span ingestion throughput, storage efficiency, database utilization, and cost management so judges can see exactly how the system meets the "intelligent + performant" requirement.

## 1. Performance Principles

- **Stream everything** – never buffer entire files or JSON batches in memory; use `io.Copy` with pooled buffers.
- **Exploit concurrency** – goroutines per upload + worker pools for async jobs keep CPUs busy and latency low.
- **Avoid unnecessary work** – dedupe by hash before writing, skip analyzer passes when metadata forces an engine.
- **Instrument first** – Prometheus metrics highlight bottlenecks; we optimize based on data, not guesses.

## 2. Ingestion Path Optimizations

| Optimization                | Description                                                               | Benefit                                     |
| --------------------------- | ------------------------------------------------------------------------- | ------------------------------------------- |
| Streaming multipart parsing | Use `multipart.Reader` and `io.MultiReader` to reuse sniff buffer         | Keeps memory flat even for multi-GB uploads |
| Zero-copy classification    | MIME detector reads 512 bytes once; `io.MultiReader` replays them to disk | Eliminates extra allocations                |
| SHA-256 during copy         | Wrap writer with `io.TeeReader` to hash data on the fly                   | No second pass for dedupe                   |
| Worker handoff via channels | Request goroutine enqueues heavy tasks (thumbnails, AV scan) instantly    | Keeps p95 latency low                       |
| Backpressure thresholds     | If queue >N jobs, API returns `429` with retry-after header               | Protects downstream systems                 |

## 3. Storage Optimizations

- **Directory hashing**: use first 2 chars of SHA to shard directories once counts exceed 10k per folder, avoiding filesystem slowdowns.
- **Sparse metadata files**: NDJSON appenders use buffered writers and fsync batching, reducing disk seeks.
- **Reference counting**: metadata index tracks how many logical references point to the same blob (content hash) to enable dedupe reclamation.
- **Lifecycle policies**: scheduled job archives cold media to cheaper storage (object store) while keeping hot set on SSD.

## 4. Database Optimizations

### 4.1 PostgreSQL

- Partial indexes on frequently queried metadata columns (category, namespace, uploader).
- JSONB GIN indexes for metadata filters.
- Batched inserts using `COPY` for high-volume SQL decisions.
- Connection pooling (pgxpool) sized to CPU count with idle timeout.

### 4.2 MongoDB

- Schema versioning to avoid collection-level migrations.
- TTL indexes for ephemeral datasets (e.g., temporary batches).
- Chunk pre-splitting for namespaces expected to shard heavily.

## 5. Decision Engine Optimizations

```go
switch {
case hints.ForceEngine != "":
    return hints.ForceEngine
case summary.StabilityScore >= 0.9 && summary.Relationships:
    return "sql"
case summary.NestedDepth > 2 || summary.ArrayFields > 4:
    return "nosql"
case summary.WriteRate > cfg.SqlWriteThreshold:
    return "nosql"
default:
    return "sql"
}
```

- Rule evaluation is O(1); metrics like `WriteRate` are updated via EWMA counters to keep responsiveness high.

## 6. Metadata & Search Optimizations

- **Bloom filter** (RedisBloom) caches known hashes to short-circuit duplicate uploads with near-zero latency.
- **Materialized views** in PostgreSQL pre-aggregate counts per namespace/category for dashboard queries.
- **Elasticsearch optional** – for demos needing text search, workers push metadata documents to Elasticsearch/OpenSearch.

## 7. Cost & Resource Efficiency

- Horizontal scale-out occurs only when CPU >70% or queue depth > threshold; prevents over-provisioning.
- Spot/preemptible workers safe because jobs are idempotent (tracked via dedupe keys).
- Object storage tiering reduces SSD footprint by >50% per sizing exercise.
- PostgreSQL autovacuum tuned with `cost_limit` to avoid I/O storms during ingest peaks.

## 8. Bottleneck Analysis & Mitigations

| Potential Bottleneck | Detection                            | Mitigation                                                             |
| -------------------- | ------------------------------------ | ---------------------------------------------------------------------- |
| Disk throughput      | Prometheus `rbx_media_write_seconds` | Switch to SSD/NVMe, shard directories, flush batching                  |
| Queue growth         | `rbx_queue_depth` metric             | Auto-scale workers, apply rate limiting                                |
| Analyzer CPU spikes  | pprof traces                         | Increase worker concurrency, reuse buffers, fallback to metadata hints |
| DB contention        | pg_stat_statements, Mongo profiler   | Partition heavy namespaces, move read queries to replicas              |

## 9. Future Optimization Ideas

- GPU-accelerated video transcoding workers for faster previews.
- WASM-based schema inference to run inside browsers before upload (client-side hints).
- Adaptive compression (Zstandard) for cold storage batches.
- Prefetch hints for CDN (signed URLs) to reduce first-byte latency.

## 10. Evidence for Hackathon Judges

- Load test scripts (Vegeta/hey) included under `scripts/` (planned) to reproduce throughput numbers.
- Grafana dashboards showing ingest rate, queue depth, decision split (SQL vs NoSQL) to prove intelligence effectiveness.
- Profiling screenshots/pprof outputs showing CPU/memory characteristics during 1k files/sec run.

By combining these optimizations, RhinoBox demonstrates it can ingest large multi-modal workloads efficiently, make intelligent storage decisions instantly, and scale without sacrificing cost or simplicity.
