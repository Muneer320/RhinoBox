# Workflows

For files: upload → MIME/extension or explicit category → content hash → indexed file → collections/search/download/notes. Identical bytes reuse the first stored file record. Rename/move change indexed file metadata and paths; deletion removes that file record.

For JSON: validate objects → analyze structure → select SQL or NoSQL → write to the selected configured database → index an NDJSON archive → return actual inserted/archive counts and warnings. With no selected database URL, only the local archive is written. SQL schema changes select a separate inferred table. MongoDB can partially insert an unordered batch.

Data shows retained archive inventory, not live database row totals. Deleting an archive does not undo database writes. Retrying JSON is not idempotent.

For development: run the backend/frontend checks, build Compose, run `scripts/smoke_demo.py`, restart services, then verify its saved state. CI runs the same backend, frontend and database smoke checks. Keep synthetic fixtures separate from personal files. See [DEMO.md](DEMO.md).
