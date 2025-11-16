# Technology

- Go HTTP API with chi routing and validation middleware.
- Local filesystem plus JSON metadata indexes; SHA-256 deduplication.
- Bounded Badger/LRU collection caching with persistent lookup restoration.
- PostgreSQL via pgx; MongoDB via the existing Go driver.
- Plain JavaScript, Vite and Monaco in the browser; Vitest/jsdom for unit tests.
- nginx and Docker Compose for the local integrated demo.

JSON routing is a deterministic structural heuristic. No external model/provider is required. Existing queue and bulk-helper code does not imply that asynchronous ingestion or a specific throughput target is available. Refer to the [current architecture](ARCHITECTURE.md) and [README](../README.md) for supported behavior.
