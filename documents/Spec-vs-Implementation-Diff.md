# Spec vs Implementation Diff

Generated: 2026-03-19

## Analytics middleware spec (`High-Level Middleware Implementation Guidance.md`)

| Spec requirement | Expected behavior (from spec) | Implementation in code | Status |
|---|---|---|---|
| Middleware must be **first** in chain | Analytics middleware runs before any other middleware | `middleware.RealIP` is added before `analytics.Middleware(...)` in `cmd/api/main.go` | Mismatch (ordering) |
| Capture **every** request (success + early rejections) | Log requests even when handlers reject early (401/403/404/5xx/etc.) | Captures downstream `statusCode` via `internal/analytics/response_recorder.go` + always enqueues after `next.ServeHTTP(...)` | Match |
| Data fields must exactly match Directus fields | Include fields: `created_at`, `user_email`, `api_key_id`, `user_id`, `ip_address`, `user_agent`, `method`, `path`, `query_params`, `status_code`, `response_time_ms`, `is_rate_limit_breach`, `remaining_quota`, `error_message` | Fields exist in `internal/analytics/record.go` but many are `omitempty`, so optional fields may be omitted from JSON payload when empty | Partial match (presence vs omitting) |
| Enrichment from auth/rate-limiter context | Populate `user_email` and any other available user/key fields; include `remaining_quota` | `internal/analytics/middleware.go` populates `UserEmail`/`APIKeyID`/`UserID` from `internal/auth` metadata. It never sets `RemainingQuota` | Mismatch (`remaining_quota`) |
| Asynchronous sending (never block) | Non-blocking enqueue + background worker | `internal/analytics/middleware.go` enqueues via an interface; `internal/analytics/sender.go` uses buffered channel + non-blocking send with drop logic | Match |
| Batching | Batch size (50–200 or every ~500ms) | Defaults are `ANALYTICS_BATCH_SIZE` (default 100) + flush interval `ANALYTICS_FLUSH_INTERVAL_MS` (default 500ms) in `internal/analytics/config.go` and `internal/config/config.go` | Match |
| Bulk API usage | POST batched logs as a single JSON array to `/items/api_request_logs` | `internal/analytics/sender.go` POSTs to `directusItemsEndpoint = "/items/api_request_logs"` with `json.Marshal(records)` where `records` is a slice | Match |
| Rejected requests: set `is_rate_limit_breach`, `error_message`, and correct status_code for any non-2xx | `error_message` should be derived for rejected responses | `error_message` extraction happens only when `statusCode >= 400` in `internal/analytics/middleware.go`; it does not cover 3xx | Partial mismatch (non-2xx includes 3xx) |
| Backpressure handling | If queue fills, drop oldest logs; analytics loss acceptable | `Sender.Enqueue` drains one record from `s.queue` before attempting enqueue again when full | Match (drop oldest) |
| Retry logic | Simple retry (max 2 attempts) on transient Directus failures | `defaultMaxAttempts = 2` in `internal/analytics/sender.go`; retries only for network errors and for `429` or `>=500` | Match |
| Outage resilience | After max retries drop batch; optional local file fallback; one WARN-level message per outage | No local file fallback implemented. Alerts are sent for outages (and recovery), but `log.Printf` occurs per lost batch (not just one per outage window) | Mismatch (file fallback + “one message per outage”) |
| Logging & observability | Log only errors/failures; queue depth exposed as metric if Prometheus | No Prometheus metrics integration found. Also logs lost-batch messages and analytics disabled message | Partial mismatch (metrics + logging scope) |
| Security | Service token via env only; never log sensitive query params (strip secrets) | Token is from env (`internal/config/config.go`). Query params are sanitized/redacted in `internal/analytics/middleware.go` | Match |
| Graceful shutdown | Flush remaining batched logs on shutdown | `Sender.Shutdown` closes queue and `run()` flushes pending batch when channel closes | Match |
| Configuration variables | Includes `ANALYTICS_FAILED_LOGS_PATH` optional, plus `BREACH_ALERT_THRESHOLD` optional | Config supports `DIRECTUS_URL`, `DIRECTUS_SERVICE_TOKEN`, `TEAMS_WEBHOOK_URL`, `ANALYTICS_BATCH_SIZE`, `ANALYTICS_FLUSH_INTERVAL_MS`, `ANALYTICS_MAX_QUEUE_SIZE`. No failed-logs path or breach-threshold config | Mismatch |

## Global Symbols API spec (`GLOBAL_SYMBOLS_API_SPEC.md`)

| Spec requirement | Expected behavior (from spec) | Implementation in code | Status |
|---|---|---|---|
| API key header formats accepted | Server accepts `Authorization: ApiKey <key>` and raw `Authorization: <key>`, plus `X-Api-Key` | Runtime auth supports `X-Api-Key` and `Authorization` parsing in `internal/auth/apikey.go` | Match (runtime) |
| OpenAPI/Docs security scheme | Spec describes accepted headers; docs should reflect them | OpenAPI security scheme is set to `X-Api-Key` only in `cmd/api/routes.go` | Mismatch (docs vs accepted auth) |
| Error envelope includes `{ code, error }` | For 400/401/404/... errors, response body should include both fields | Missing `query` errors use `newAPIErrorNoCode` (only `{"error": ...}`) in `cmd/api/huma_concepts.go` via `newAPIErrorNoCode` | Mismatch (`code` missing) |
| `expand` parameter on labels endpoints | `GET /v1/labels/search` and `GET /v1/labels/:id` support `expand` | No `expand` query param or `expand` logic in label operations (Huma input structs lack `expand`) | Mismatch |
| Label access control may yield 403/404 | `GET /v1/labels/:id` can return `404` or `403` depending on authorization | Label lookup in `internal/db/labels.go` filters by authoritative/published/visibility but has no caller-ability checks; no 403 path found in Go code | Mismatch (403 not representable) |
| `GET /v1/labels/search` language default | `language` is optional and table shows no default | Handler defaults `language` to `"eng"` when omitted in `cmd/api/huma_concepts.go` and `internal/handlers/labels.go` | Mismatch (defaulting language) |
| `Picto.native_format` values | Spec allows `svg` or `png` | DB queries hardcode `NativeFormat = "png"` in `internal/db/concepts.go` and `internal/db/pictos.go` (and label queries) | Potential mismatch (if svg exists) |
| Concept and picto filtering by visibility | Spec is explicit about non-archived + visibility for pictos/listing, but concept/label pictos requirements are less explicit | `internal/db/concepts.go` `conceptPictos` filters `p.archived = 0` but does not filter `p.visibility = 0` | Potential mismatch (depends on Rails behavior) |

