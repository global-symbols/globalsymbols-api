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

