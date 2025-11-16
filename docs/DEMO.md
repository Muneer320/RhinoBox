# Local demo walkthrough

Start the stack with `docker compose -p rhinobox-demo up --build --wait --wait-timeout 180`, then open http://127.0.0.1:5173.

## File flow

Upload a small PNG and a text file from Home. Open Files, then Images or Documents. Download the original file; rename it; open Notes and add a note. Reload the page and confirm the note remains. Delete only your disposable test file and confirm its collection count updates.

## JSON flow

Use Add New Item and the JSON editor. A flat batch such as `[ {"id":1,"name":"Ada"}, {"id":2,"name":"Lin"} ]` selects SQL. A nested batch such as `[ {"user":{"profile":{"preferences":{"theme":"dark"}}},"event":"opened"} ]` selects NoSQL.

With Compose, these routes write to PostgreSQL or MongoDB. Without the selected database configured, the response says `ndjson`, with zero inserts. Files → JSON Documents lists the corresponding NDJSON archives. Data shows retained archive inventory grouped by destination/namespace; its counts are archive counts, not database row totals.

Deleting or renaming an archive changes the file record only. It does not undo database inserts. Repeating JSON ingestion appends database records; requests are not idempotent. Partial MongoDB writes and archive failures produce warnings.

## Automated verification

From the root run `python scripts/smoke_demo.py --project rhinobox-demo --base-url http://127.0.0.1:5173/api --state-out .demo-state.json`. This checks the web proxy, SQL creation/reuse/schema changes/rollback, MongoDB int64 and partial insertion, archive listing/download/rename, and disposable file upload/search/move/notes/delete.

Stop the API with `docker compose -p rhinobox-demo stop rhinobox`, restart databases with `docker compose -p rhinobox-demo restart postgres mongodb`, then recreate the API using `docker compose -p rhinobox-demo up -d --force-recreate rhinobox`. Wait using `docker compose -p rhinobox-demo up --wait --wait-timeout 180`, then run `python scripts/smoke_demo.py --project rhinobox-demo --base-url http://127.0.0.1:5173/api --verify-state .demo-state.json`. Verification independently queries the retained database records and downloads indexed archives through nginx. Its dynamic resolver follows the API container's current address. Keep the same project name and volumes.

Stop services with `docker compose -p rhinobox-demo stop`; data is retained. This is a local prototype demonstration, not a production or throughput benchmark.
