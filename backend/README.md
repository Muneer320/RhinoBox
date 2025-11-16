# RhinoBox backend

The Go API supports the local file manager and JSON routing demo. Run `go run ./cmd/rhinobox` from this directory. It binds to `127.0.0.1:8090` and stores data under `./data` by default. Use Go 1.25 or newer; Docker and CI use Go 1.27.

Files are indexed by content hash and support collections, search, download, rename, move, notes and deletion. JSON batches are analyzed using structural heuristics. Configured PostgreSQL/MongoDB adapters receive the selected batches; otherwise that engine uses an indexed NDJSON archive and reports zero database inserts. Responses distinguish inserted and archived counts and surface partial-write warnings.

For the complete browser/database demo, run Compose from the repository root. See the [README](../README.md), [walkthrough](../docs/DEMO.md), [API reference](../docs/API_REFERENCE.md) and [database behavior](../docs/DATABASE.md).

Run checks here:

```sh
go test -mod=readonly -p 2 -timeout 120s ./...
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
```

Tests generate their own fixtures. The current integrated smoke check is `scripts/smoke_demo.py` at the repository root; it independently queries both databases and checks retained archives after restart. Earlier stress-test artifacts are historical, do not establish current performance and are not part of the supported demo instructions.

This is a local, single-user prototype. Authentication, tenant isolation and asynchronous ingestion routes are not implemented. Queue helper code does not imply a public job API. Concurrent API processes must not share a data directory. Database writes and filesystem archives are separate operations; retries are not idempotent.
