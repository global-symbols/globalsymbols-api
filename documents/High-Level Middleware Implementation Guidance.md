**High-Level Implementation Guidance**  
**API Analytics Middleware for Go Backend**

**Document Purpose**  
This document provides a clear, high-level specification and guidance for implementing the API request logging middleware. It is designed to be used directly with Cursor (or any code-aware editor) so you can prompt the AI with sections from this doc and let it generate the code. No code examples are included here — only requirements, flow, and decisions.

### 1. Objective
Add a single, lightweight middleware that captures **every** incoming API request (successful or rejected) and asynchronously sends the data to the existing Directus collection `api_request_logs`. The goal is complete visibility into API usage, user activity, and rate-limit breaches without impacting API performance or latency.

### 2. Scope
- Only the Go backend (the API layer).
- Middleware must be the **first** in the middleware chain.
- Data destination: Directus REST endpoint `/items/api_request_logs`.
- No changes to Rails, database schema, or Directus configuration.
- No new external dependencies beyond what you already use.

### 3. Functional Requirements

| Requirement | Description |
|-------------|-------------|
| Capture every request | Middleware must run on 100% of requests, including early rejections (rate-limit 429, auth 401/403, 404, 5xx, etc.). |
| Data fields to log | Exactly match the `api_request_logs` collection fields: `created_at`, `user_email`, `api_key_id`, `user_id`, `ip_address`, `user_agent`, `method`, `path`, `query_params`, `status_code`, `response_time_ms`, `is_rate_limit_breach`, `remaining_quota`, `error_message`. |
| Enrichment | At request time, pull `user_email` (and any other available user/key fields) from your existing auth/rate-limiter context. |
| Asynchronous sending | Never block the request handler. Use non-blocking pattern (goroutine + channel recommended). |
| Batching | Group logs (e.g. 50–200 items or every 500ms) before sending to Directus for efficiency. |
| Bulk API usage | Send batched logs as a single JSON array to Directus `/items` endpoint (Directus native bulk create). |
| Rejected requests | Automatically set `is_rate_limit_breach`, `error_message`, and correct `status_code` for any non-2xx response. |

### 4. Non-Functional Requirements

| Requirement | Target |
|-------------|--------|
| Performance impact | Zero measurable increase in p95/p99 response time. |
| Backpressure handling | If the internal queue fills, drop oldest logs (or log warning) — analytics loss is acceptable. |
| Retry logic | Simple retry (max 2 attempts) on transient Directus failures. No infinite retries. |
| Logging & observability | Log only errors/failures to your existing logger (stdout or structured). Expose queue depth as a metric if you use Prometheus. |
| Configuration | All settings (Directus URL, service token, batch size, flush interval) via environment variables or Viper/config struct. |
| Graceful shutdown | Flush any remaining batched logs on server shutdown. |
| Security | Service token stored in env only. Never log sensitive query params (strip tokens/secrets before sending). |

### 5. Data Flow Summary
1. Request arrives → Analytics middleware (first).
2. Start timer.
3. Call `c.Next()` (or equivalent).
4. After handler finishes, build log struct from context + response data.
5. Push struct to internal channel (non-blocking).
6. Background worker(s) consume channel, batch, and POST to Directus.
7. On success → discard. On failure → follow outage handling below.

### 6. Outage Resilience
During Directus outages (maintenance, network issues, crashes, or rate-limiting):

- After max retries, drop the batch (do not block or slow the API).
- Optional (recommended): Write failed batches to a local file (e.g. `/tmp/failed_analytics_logs.jsonl` or a configurable path) for manual replay later.
- Log a single WARN-level message per outage (include timestamp and batch size) — avoid spamming logs.
- Monitoring: If you use Prometheus/Grafana, expose a counter for “dropped batches due to outage” so stakeholders can see any gaps.
- Philosophy: Analytics is best-effort. Short gaps during outages are acceptable; permanent loss is not required.

### 7. Error Handling & Resilience
- Network/Directus errors → follow outage resilience rules above.
- Invalid JSON or schema errors → drop the batch and log sample.
- Rate-limit on Directus side → apply brief back-off on retries.

### 8. Configuration Variables (expected)
- `DIRECTUS_URL`
- `DIRECTUS_SERVICE_TOKEN`
- `ANALYTICS_BATCH_SIZE` (default 100)
- `ANALYTICS_FLUSH_INTERVAL_MS` (default 500)
- `ANALYTICS_MAX_QUEUE_SIZE` (default 10_000)
- `ANALYTICS_FAILED_LOGS_PATH` (optional, for outage file fallback)
- `TEAMS_WEBHOOK_URL` (for outage alerts)
- `BREACH_ALERT_THRESHOLD` (optional, if breach alerts are enabled later)

### 9. Testing Approach (high-level)
- Unit: Test log struct building and enrichment.
- Integration: Mock Directus endpoint, verify batch POST format and async behaviour.
- Load: Simulate 500+ req/s with mixed 2xx/4xx/429 responses; confirm no latency regression and queue stays stable.
- Edge cases: Early aborts, missing auth context, simulated Directus outage (return 5xx or connection error), server shutdown with pending batch and failed-logs file.

### 10. Implementation Order Recommendation (for Cursor)
1. Define the log struct and config.
2. Create the middleware skeleton (attach to router).
3. Implement the non-blocking channel + background worker.
4. Add batching + HTTP client.
5. Add enrichment from auth context.
6. Add outage resilience (file fallback + logging).
7. Add graceful shutdown handling.
8. Add tests and metrics (optional).

### 11. Alerting Strategy
**Directus Outage Alerting (chosen Tier 1)**  
When batches are dropped after max retries, send a single notification to a Microsoft Teams channel using an **Incoming Webhook** URL.  
- One clean message per outage (timestamp + batch size + error summary).  
- Configuration: `TEAMS_WEBHOOK_URL` (set once in your Teams channel → Workflows).  
- This is the simplest, zero-extra-service option and matches your preference exactly.

**Rate Limit Breach Alerting**  
(Left optional for now — we can add later if you want Tier 1 per-user thresholds or daily digest.)
