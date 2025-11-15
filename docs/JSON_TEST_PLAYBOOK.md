# JSON Test Playbook

This playbook packages representative JSON batches you can replay against `/ingest/json` to validate the SQL vs NoSQL decision logic, schema generation, and artifact writing paths.

## How to Run

1. Start the backend (adjust flags as needed):
   ```pwsh
   cd backend
   go run ./cmd/rhinobox
   ```
2. In another terminal, post any sample payload using cURL (swap the file path as needed):
   ```pwsh
   curl -X POST http://localhost:8080/ingest/json \
     -H "Content-Type: application/json" \
     --data-binary "@backend/testdata/json/sql_orders.json"
   ```

All payload files already include the required `namespace`, `documents`, and optional `comment` fields, so they can be replayed as-is.

## SQL-leaning payloads

| File                                               | Namespace           | Primary shape                 | Why it should score SQL                                           |
| -------------------------------------------------- | ------------------- | ----------------------------- | ----------------------------------------------------------------- |
| `backend/testdata/json/sql_orders.json`            | `orders_sql`        | Flat ecommerce orders         | Stable columns, scalar fields, clear primary/foreign keys         |
| `backend/testdata/json/sql_payroll.json`           | `payroll_batches`   | Finance/payroll with decimals | Identical schema per row, mix of numeric + boolean columns        |
| `backend/testdata/json/sql_sensor_metrics.json`    | `sensor_metrics`    | Time-series metrics           | Narrow schema, high cardinality IDs, predictable timestamped rows |
| `backend/testdata/json/sql_inventory_batches.json` | `inventory_batches` | Inventory snapshots           | Repeatable snapshot records ideal for warehouse tables            |

## NoSQL-leaning payloads

| File                                                | Namespace           | Primary shape                    | Why it should score NoSQL                                    |
| --------------------------------------------------- | ------------------- | -------------------------------- | ------------------------------------------------------------ |
| `backend/testdata/json/nosql_activity_stream.json`  | `activity_stream`   | Users with nested event arrays   | Deeply nested arrays, polymorphic event metadata             |
| `backend/testdata/json/nosql_user_profiles.json`    | `profiles_flexible` | Profiles with optional blobs     | Documents diverge heavily per profile, many optional fields  |
| `backend/testdata/json/nosql_iot_unstructured.json` | `iot_unstructured`  | Device telemetry blobs           | Device-specific payloads, arrays/objects differ per document |
| `backend/testdata/json/nosql_chat_threads.json`     | `chat_threads`      | Chat threads with nested replies | Recursive arrays, attachment lists, non-tabular structure    |

## Tips

- Keep `MAX_UPLOAD_BYTES` in `config.yaml` high enough; these payloads are tiny but real runs may include thousands of docs.
- Mix and match payloads to batch decision stress tests (e.g., send SQL files with `comment` mentioning "analytics" to reinforce the ranking).
- After each request, inspect the response `decision`, `schema_path`, and `batch_path` to confirm the engine routed as expected and schema artifacts were generated only for SQL decisions.
