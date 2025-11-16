# API reference

Native API base: `http://127.0.0.1:8090`. The frontend accesses the same routes through `/api`. `/config` and `/api/config` both return application feature flags. Errors contain `error.code` and `error.message`; validation can also include field details.

| Method | Route | Purpose |
|---|---|---|
| GET | `/healthz`, `/config` | Health / supported feature flags |
| POST | `/ingest` | Multipart `files`, optional `namespace`, `comment`, `data`, `file_type_override` |
| POST | `/ingest/media` | Multipart `file`; at most 50 files |
| POST | `/ingest/json` | JSON body with `document` or `documents`, optional `namespace`/`comment` |
| GET | `/files`, `/files/type/{type}` | Paginated indexed files / collection |
| GET | `/files/{hash}`, `/files/metadata?hash=…` | File metadata |
| GET | `/files/search` | AND-combined name, extension, type, category, MIME, date and content filters |
| GET | `/files/download?hash=…`, `/files/stream?hash=…` | Download / range streaming; indexed path is an alternative |
| PATCH | `/files/rename` | `hash`, `new_name` |
| DELETE | `/files/{hash}` | Delete a file/archive; database records remain |
| POST or PATCH | `/files/{hash}/move` | `new_category`, optional reason |
| POST | `/files/{hash}/copy` | Copy with optional name/category/metadata |
| PATCH | `/files/{hash}/metadata` | Merge, replace or remove user metadata |
| GET / POST | `/files/{hash}/notes` | List / add a note |
| PATCH / DELETE | `/files/{hash}/notes/{note_id}` | Update / delete a note |
| GET / POST | `/files/{hash}/versions` | List / create versions |
| GET | `/collections`, `/collections/{type}/stats`, `/statistics` | Indexed archive/file statistics |
| GET / PUT / DELETE | `/routing-rules` | Saved routing rules |
| POST | `/routing-rules/suggest` | Save a MIME/extension destination rule |
| GET / POST | `/duplicates`, `/duplicates/scan` | Duplicate report / scan |

Unified responses contain `success`, `status`, `results.media`, `results.files`, `results.json` and timing. JSON results distinguish `storage_backend`, `records_inserted`, `records_archived`, `archive_hash`, `batch_path`, `schema_created` and warnings. Dedicated JSON ingestion returns these fields alongside its decision. See [database behavior](DATABASE.md).

Search supports `name`, `extension`, `type`, `category`, `mime_type`, `content_search` (or `content`), `date_from` and `date_to`. Dates accept RFC3339 or YYYY-MM-DD; date-only upper bounds include the whole UTC day. Text content search is limited to files up to 10 MiB.

Routes do not enforce authentication or tenant isolation. The async helper package is not exposed as a supported API. See [the runnable smoke check](../scripts/smoke_demo.py) for exact working request examples.
