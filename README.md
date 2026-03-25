## GS API

Go implementation of the Global Symbols API.

### Running the Go API locally

The simplest way to run the Go API is via Docker Compose.

**First-time setup:** copy the example compose file and add your real credentials (`docker-compose.yml` is gitignored so secrets stay local; only `docker-compose.example.yml` is in the repo):

```bash
cp docker-compose.example.yml docker-compose.yml
# Edit docker-compose.yml with your DB DSN, Directus token, etc.
```

Then:

```bash
docker compose up -d go-api
```

This uses the root `docker-compose.yml` and builds the Go API image using the root `Dockerfile`. By default it:

- Exposes the Go API on `http://localhost:8080`
- Points at the Rails API and image server on `http://host.docker.internal:3000`

### Rate limiting

Authenticated data endpoints under `/api/v2` are rate-limited in memory per API key.

- `RATE_LIMIT_PER_MINUTE` controls the per-key allowance and defaults to `100`
- a value less than or equal to `0` disables throttling
- when the limit is exceeded, the API returns `429 Too Many Requests`
- the response includes `Retry-After`, `X-RateLimit-Limit`, `X-RateLimit-Remaining`, and `X-RateLimit-Reset`

This is a temporary throttle, not a key revocation. The limiter uses a fixed 1-minute in-memory window per API key. Once a key has used its full quota inside the current minute-long window, additional requests are blocked until that window ends. After the window resets, the key can make requests again automatically with no database update required.

Because the limiter is process-local, its counters are also reset if the Go API process restarts.

### Suspicious traffic alerts

The API also watches completed requests for suspicious patterns and can raise Teams alerts without blocking traffic.

- `SUSPICIOUS_RATE_LIMIT_BREACH_THRESHOLD` default: `20`
- `SUSPICIOUS_RATE_LIMIT_BREACH_WINDOW_MS` default: `120000` (2 minutes)
- `SUSPICIOUS_UNAUTHORIZED_IP_THRESHOLD` default: `50`
- `SUSPICIOUS_UNAUTHORIZED_IP_WINDOW_MS` default: `300000` (5 minutes)
- `SUSPICIOUS_IP_REQUEST_THRESHOLD` default: `300`
- `SUSPICIOUS_IP_REQUEST_WINDOW_MS` default: `60000` (1 minute)
- `SUSPICIOUS_ALERT_COOLDOWN_MS` default: `1800000` (30 minutes)

These alerts use `TEAMS_WEBHOOK_URL`. The current detector is in-memory and process-local, so counters and alert cooldown state are cleared if the Go API process restarts.

### Test suites

There are two main Go test suites under `tests/`:

- `tests/regression`: behavior/contract tests against the Go API only  
- `tests/parity`: parity tests comparing Rails vs Go API behavior

#### 1. Regression tests (Go API only)

These tests hit the Go API directly. They verify auth behavior, validation errors, and the expected JSON contract for several successful responses.

Environment variables (required):

- `GO_API_BASE_URL` – e.g. `http://localhost:8080`
- `TEST_API_KEY` – valid API key sent as `X-Api-Key`

Example (from your host, with Go installed and the API running):

```bash
export GO_API_BASE_URL="http://localhost:8080"
export TEST_API_KEY="your-api-key-here"

go test ./tests/regression -v -count=1
```

#### 2. Parity tests (Rails vs Go)

These tests send the same requests to both the Rails API and the Go API and compare responses. They cover both successful responses and selected error cases, and the happy-path tests now require both APIs to return `200 OK` before comparing payloads.

Environment variables (required):

- `TEST_RAILS_BASE_URL` – e.g. `http://localhost:3000`
- `TEST_GO_BASE_URL` – e.g. `http://localhost:8080`
- `TEST_API_KEY` – valid API key sent as `X-Api-Key`

Example (from your host, with both APIs running):

```bash
export TEST_RAILS_BASE_URL="http://localhost:3000"
export TEST_GO_BASE_URL="http://localhost:8080"
export TEST_API_KEY="your-api-key-here"

go test ./tests/parity -v -count=1
```

### Using the Docker test container

If you do not have Go installed on your host, or you prefer an isolated test environment, you can use the test container defined in `tests/docker-compose.yml`. Copy the example first if you have not:

```bash
cp tests/docker-compose.example.yml tests/docker-compose.yml
# Set TEST_API_KEY in tests/docker-compose.yml if you use env from compose
```

This container includes the Go toolchain and mounts the repository for interactive test runs; it does **not** run tests automatically.

From the repo root:

```bash
# Build (if needed) and start the test container
docker compose -f tests/docker-compose.yml up -d tests

# Or, after changing tests/Dockerfile, force a rebuild:
docker compose -f tests/docker-compose.yml up -d --build tests
docker ps  # find the container named like tests-tests-1
docker exec -it tests-tests-1 /bin/sh
```

Inside the container:

```bash
cd /app

export GO_API_BASE_URL="http://host.docker.internal:8080"
export TEST_RAILS_BASE_URL="http://host.docker.internal:3000"
export TEST_GO_BASE_URL="http://host.docker.internal:8080"
export TEST_API_KEY="your-api-key-here"

# Run whichever suites you need:
go test ./tests/regression -v -count=1
go test ./tests/parity -v -count=1
```

Adjust the base URLs if your services are exposed differently.

