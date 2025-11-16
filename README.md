# RhinoBox

[![Checks and local demo](https://github.com/Muneer320/RhinoBox/actions/workflows/ci.yml/badge.svg)](https://github.com/Muneer320/RhinoBox/actions/workflows/ci.yml)

RhinoBox is a team-built hackathon prototype for organizing files and routing JSON batches to PostgreSQL or MongoDB using structural heuristics. A Go API handles storage; a Vite frontend provides uploads, collections, notes, search, downloads and an inventory of JSON archives.

The maintained target is a **local, single-user demo**. The original team history is preserved. Throughput benchmarks and production readiness are not claimed.

## Run the complete demo

Install Docker with Compose, then run from the repository root:

```sh
docker compose -p rhinobox-demo up --build --wait --wait-timeout 180
```

Open [http://127.0.0.1:5173](http://127.0.0.1:5173). The API is available at [http://127.0.0.1:8090/healthz](http://127.0.0.1:8090/healthz). No external account or API key is needed. PostgreSQL and MongoDB use internal container networking; the web ports bind only to loopback. The included database password is a local demo default.

Stop the demo while keeping its data:

```sh
docker compose -p rhinobox-demo stop
```

Named volumes retain files, metadata and database records. Keep the same Compose project name when restarting.

## Try it

1. From **Home**, upload a small image or text file. Auto routing uses its MIME type/extension; the type selector can override that choice.
2. Open **Files**, select a collection and try download, rename, notes and deletion. Collection counts refresh after changes.
3. Open **Add New Item**, enter a flat JSON batch and submit it:

   ```json
   [{"id":1,"customer":"Ada","total":12.5},{"id":2,"customer":"Lin","total":7.5}]
   ```

4. The response identifies the actual storage backend. **Data** shows retained SQL/NoSQL-routed archives; **Files → JSON Documents** exposes downloadable NDJSON batches with normal file controls.
5. Try deeply nested JSON to exercise MongoDB routing. See [the demo walkthrough](docs/DEMO.md) for the exact examples and verification command.

## What works

- Indexed file storage with SHA-256 deduplication, MIME/extension categories and saved routing rules.
- File listing, metadata/name/date/content search, download and range streaming, rename, copy, move, versions, notes and deletion. Some advanced operations are API-only.
- JSON structure analysis, SQL/NoSQL recommendation and actual writes when the selected database is configured.
- Transactional PostgreSQL table creation/insertion; changed inferred schemas get separate tables instead of silently dropping new fields.
- MongoDB insertion that preserves int64 values and reports partial write counts.
- Indexed NDJSON archives, collection statistics, bounded persistent caching and streamed upload limits.
- Automated backend/frontend checks and a Compose smoke test that queries the real databases and checks persistence after restart.

## Storage behavior and limits

JSON responses separate `decision.engine`, `storage_backend`, `records_inserted`, `records_archived` and warnings. If the selected database URL is absent, RhinoBox archives NDJSON locally and reports zero database inserts. A configured but unreachable database fails startup.

Database writes and filesystem archival are separate operations. Archive failures after a committed write are reported as warnings. MongoDB can partially insert an unordered batch. Requests are not idempotent: retrying a committed or partially committed batch can duplicate database records. Deleting an archive file does **not** delete its PostgreSQL/MongoDB records. The Data screen describes retained archives, not a live database console or database row totals.

Identical file content reuses its first stored record, including its original name/category. JSON archive deduplication does not suppress database inserts. Legacy batches under the older root `json/` layout are not migrated automatically.

The default request/upload cap is 512 MiB; the media endpoint accepts at most 50 files per request and unified ingestion at most 100. Multipart parsing spills larger uploads to temporary files. MIME/extension classification does not interpret image or document meaning.

Authentication, tenant isolation and asynchronous ingestion are not implemented. Setting `RHINOBOX_AUTH_ENABLED=true` refuses startup. This demo is not configured for public hosting. No current throughput or stress-test performance is asserted.

## Local development without databases

Use Go and Node.js 24. Docker/CI use Go 1.27; native Windows checks also run with Go 1.25.1.

In one terminal:

```sh
cd backend
go run ./cmd/rhinobox
```

In another:

```sh
cd frontend
npm ci
npm run dev
```

Open the printed Vite URL. Its `/api` proxy forwards to the local Go server. JSON is archived as NDJSON in this mode; no database inserts are claimed.

| Setting | Default / use |
|---|---|
| `RHINOBOX_ADDR` | `127.0.0.1:8090` for native development |
| `RHINOBOX_DATA_DIR` | `./data` relative to the backend process |
| `RHINOBOX_MAX_UPLOAD_MB` | `512` |
| `RHINOBOX_MAX_REQUEST_SIZE` | `536870912` bytes; bounded by the upload limit |
| `RHINOBOX_POSTGRES_URL` | Optional dedicated PostgreSQL database |
| `RHINOBOX_MONGO_URL` | Optional MongoDB server; database `rhinobox` |
| `RHINOBOX_DB_MAX_CONNS` | `10` per database adapter |
| `VITE_API_BASE_URL` | Frontend build setting; defaults to `/api` |

## Checks

```sh
cd backend
go test -mod=readonly -p 2 -timeout 120s ./...
go vet ./...
```

```sh
cd frontend
npm ci
npm run lint
npm test
npm run build
```

With Compose running, execute from the repository root:

```sh
python scripts/smoke_demo.py --project rhinobox-demo --base-url http://127.0.0.1:5173/api --state-out .demo-state.json
docker compose -p rhinobox-demo stop rhinobox
docker compose -p rhinobox-demo restart postgres mongodb
docker compose -p rhinobox-demo up -d --force-recreate rhinobox
docker compose -p rhinobox-demo up --wait --wait-timeout 180
python scripts/smoke_demo.py --project rhinobox-demo --base-url http://127.0.0.1:5173/api --verify-state .demo-state.json
```

The smoke test creates synthetic records and disposable files; it retains synthetic database records and remaining archives for the restart check. Tests use generated fixtures, not personal Downloads folders. Vitest failures propagate to the exit status. Some legacy console warnings remain in lint. The production npm audit is clear; five moderate advisories remain in development lint dependencies.

## Documentation and contributors

- [Demo walkthrough](docs/DEMO.md)
- [API reference](docs/API_REFERENCE.md)
- [Architecture](docs/ARCHITECTURE.md), [database behavior](docs/DATABASE.md), [deployment](docs/DEPLOYMENT.md)
- [Frontend development](frontend/README.md)

RhinoBox was built collaboratively. The repository's human contributors include [@Muneer320](https://github.com/Muneer320), [@thewildofficial](https://github.com/thewildofficial) and [@ZairaAk](https://github.com/ZairaAk). Their original contributions and commit history remain intact.

Released under the [MIT license](LICENSE).
