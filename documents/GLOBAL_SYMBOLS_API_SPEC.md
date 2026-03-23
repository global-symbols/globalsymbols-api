# Global Symbols API v1 — Specification

This document describes the **Global Symbols API v1** under `app/api/global_symbols`. It is intended for re-implementing this API in Go or another language.

**Base URL (relative):** `/api/v1`  
**Full base URL example:** `https://<host>/api/v1`

**Format:** JSON for request and response bodies.

**Versioning:** API version is in the path (`/v1`). Vendor: `globalsymbols`.

---

## 1. Authentication

### 1.1 API Key (all endpoints except `/user`)

All v1 endpoints **except** the `/user` resource require a valid, active API key.

**Accepted methods:**

| Method | Description |
|--------|-------------|
| **Authorization header** | `Authorization: ApiKey <your_key>` or raw `Authorization: <your_key>` |
| **X-Api-Key header** | `X-Api-Key: <your_key>` |

**Behavior:**

- If the key is missing or invalid/inactive, respond with **401 Unauthorized** and a JSON error body (see [Error response](#9-error-response)).
- Error message: *"A valid, active API key is required. Provide it in the Authorization header as \"ApiKey <key>\" or in the X-Api-Key header."*
- The server may update a "last used" timestamp for the key on each request.

### 1.2 OAuth2 (User endpoint only)

The **`/user`** resource uses OAuth2 (e.g. Doorkeeper-style) with:

- **Scope:** `profile`
- **401** when no access token is presented.
- **403** when the token is valid but lacks the required scope.

Re-implementing the User endpoints in Go would require an OAuth2 provider (or equivalent) and is optional if only the symbol/label/concept/language data is needed.

---

## 2. Common behavior

- **Content-Type:** Assume `application/json` where a body is sent.
- **404 Not Found:** When a resource is not found (e.g. invalid ID), return the standard [Error response](#9-error-response) with `code: 404` and a message such as *"Couldn't find &lt;Model&gt; with id &lt;id&gt;"*.
- **Swagger:** The Ruby app exposes Swagger at a path that includes `swagger_doc`; that path may be excluded from API-key checks so the docs can load without a key.

---

## 3. Endpoints

### 3.1 Concepts

#### GET `/v1/concepts/suggest`

Returns concepts matching a text query, with pictos for each concept. Results are ordered by match type (exact word, then word prefix, then containing word, etc.) and then by `subject`.

**Query parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `query` | string | Yes | — | Concept name to search for. Server may lowercase and normalize (e.g. spaces → underscores for matching). |
| `symbolset` | string | No | — | Slug of a **published** symbolset to restrict search. If omitted, search all symbolsets. Must be a published symbolset slug. |
| `language` | string | No | `eng` | ISO 639 language code (format depends on `language_iso_format`). |
| `language_iso_format` | string | No | `639-3` | One of: `639-1`, `639-2b`, `639-2t`, `639-3`. |
| `limit` | integer | No | 10 | Number of results. Allowed range: 1–100. |

**Defaults and examples:**

- If `language` is provided and `language_iso_format` is omitted, the server assumes `language_iso_format=639-3`.

Examples:

```bash
# ISO 639-3 (default)
GET /v1/concepts/suggest?query=dog&language=eng

# ISO 639-1 (explicit)
GET /v1/concepts/suggest?query=dog&language=en&language_iso_format=639-1
```

**Response:** `200 OK` — JSON array of [Concept](#concept) objects.

**Validation:**

- If `symbolset` is provided and is not a published symbolset slug → **400** (or equivalent error).
- If `language` / `language_iso_format` are invalid or language not found → server error or 400 as appropriate.

**Search behavior (for parity):**

- Query is lowercased; spaces can be normalized to underscores for ordering.
- Matching: concepts whose `subject` contains the (normalized) query.
- Ordering priority: exact match → subject starting with `query_` → subject ending with `_query` → subject containing `_query_` → subject starting with query → subject containing query; then by `subject`.
- When `symbolset` is set, only concepts that have at least one picto in that symbolset are returned, and only pictos from that symbolset are included in each concept.

---

#### GET `/v1/concepts/:id`

Returns a single concept by ID with its associated pictos.

**Path parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | integer | Yes | Concept ID. |

**Response:** `200 OK` — Single [Concept](#concept) object.

**Errors:** `404` if concept not found.

---

### 3.2 Labels

#### GET `/v1/labels/search`

Returns labels matching the query, each with its associated picto. Only authoritative labels from non-archived pictos in published symbolsets are considered.

**Query parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `query` | string | Yes | — | Label text to search for. Leading/trailing whitespace may be stripped. |
| `symbolset` | string | No | — | Slug of a published symbolset to restrict search. |
| `language` | string | No | — | ISO 639 language code (format per `language_iso_format`). Use Languages endpoint for valid values. |
| `language_iso_format` | string | No | `639-3` | One of: `639-1`, `639-2b`, `639-2t`, `639-3`. |
| `limit` | integer | No | 10 | Number of results. Allowed range: 1–100. |
| `include_preview` | boolean | No | `false` | When `true`, include `picto.preview_data_url` as an inline 64x64 PNG data URL for each result when preview generation succeeds. |
| `expand` | string | No | — | Space-separated list of field paths to expand (e.g. nested `picto.symbolset`). Coerced to array by splitting on whitespace. |

**Defaults and examples:**

- If `language` is provided and `language_iso_format` is omitted, the server assumes `language_iso_format=639-3`.

Examples:

```bash
# ISO 639-3 (default)
GET /v1/labels/search?query=dog&language=eng

# ISO 639-1 (explicit)
GET /v1/labels/search?query=dog&language=en&language_iso_format=639-1
```

**Response:** `200 OK` — JSON array of label-search objects. The shape matches [Label](#label), except that when `include_preview=true` the nested `picto` object may also include:

- `preview_data_url` — string data URL in the form `data:image/png;base64,...`, containing a 64x64 PNG preview for the picto.

**Search behavior (for parity):**

- Same kind of text matching and ordering as concepts: exact match, then prefix/suffix/contains, then order by `text`.
- Only labels that are “authoritative” and whose picto belongs to a published, non-archived symbolset; optionally filtered by `symbolset` slug.

---

#### GET `/v1/labels/:id`

Returns a single label by ID with its associated picto. Access may be restricted by authorization (e.g. ability); if the label exists but the caller is not allowed to see it, return **404** or **403** as appropriate.

**Path parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | integer | Yes | Label ID. |

**Query parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `expand` | string | No | — | Space-separated field paths to expand. |

**Response:** `200 OK` — Single [Label](#label) object.

**Errors:** `404` if label not found or not accessible.

---

### 3.3 Languages

#### GET `/v1/languages/active`

Returns all active languages.

**Query parameters:** None.

**Response:** `200 OK` — JSON array of [Language](#language) objects.

---

### 3.4 Symbolsets

#### GET `/v1/symbolsets`

Returns all **published** symbol sets, ordered by featured level (nulls last) then by name.

**Query parameters:** None.

**Response:** `200 OK` — JSON array of [Symbolset](#symbolset) objects.

---

### 3.5 Pictos

#### GET `/v1/pictos`

Returns a paginated list of pictos for a given symbolset. Can be used in two modes: full list or delta (changes since a timestamp).

**Query parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `symbolset` | string | Yes | — | Slug of a **published** symbolset. Must be from the published set. |
| `page` | integer | No | 1 | Page number (≥ 1). |
| `per_page` | integer | No | 100 | Items per page (clamped to 1–100). |
| `since` | string | No | — | ISO 8601 timestamp. If provided, **delta mode**: return only pictos created or updated after this time, plus a `deletions` array of IDs that were archived since then. |

**Delta mode (when `since` is set):**

- Only pictos with `updated_at > since` are considered.
- Response includes:
  - `items`: paginated list of **non-archived**, visible (e.g. visibility “everybody”) pictos that changed since `since`.
  - `total`: total count of changes (additions/updates/archivals) before pagination.
  - `deletions`: array of picto IDs that were archived since `since`.
  - `last_updated`: ISO 8601 timestamp of the most recent `updated_at` among changed pictos (optional field, only in delta mode).

**Full mode (no `since`):**

- Return all non-archived, visible pictos in the symbolset.
- Response: `items` (paginated), `total` (total count). No `deletions` or `last_updated`.

**Validation:**

- Invalid `since` (not valid ISO 8601) → **400** with body like: *"Invalid 'since' timestamp. Use ISO 8601 format (e.g., 2026-01-01T00:00:00Z)."*

**Response:** `200 OK` — [PagedPictosResponse](#pagedpictosresponse).

**Errors:** `404` if symbolset slug is not found or not published.

---

### 3.6 User (OAuth2)

These endpoints require OAuth2 access token with scope `profile`. They are not protected by the API key alone.

#### GET `/v1/user`

Returns the authenticated user’s profile.

**Response:** `200 OK` — Single [User](#user) object.

**Errors:** `401` if not authenticated; `403` if scope insufficient.

---

#### PATCH `/v1/user`

Updates the authenticated user. Only provided fields are updated.

**Body (JSON):**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `default_hair_colour` | string | No | Colour in hex format. |
| `default_skin_colour` | string | No | Colour in hex format. |

**Response:** `200 OK` — Updated [User](#user) object.

**Errors:** `401`/`403` as for GET `/v1/user`.

---

## 4. Data types (entities)

### Concept

| Field | Type | Description |
|-------|------|-------------|
| `id` | integer | Concept ID. |
| `subject` | string | Concept subject (e.g. "computer"). |
| `coding_framework` | [CodingFramework](#codingframework) | Coding framework. |
| `language` | [Language](#language) | Language of the concept. |
| `pictos_count` | integer | Count of published pictos linked to this concept (possibly filtered by symbolset in suggest). |
| `pictos` | array of [Picto](#picto) | Pictos for this concept (filtered by symbolset when applicable). |
| `api_uri` | string | External API URI (e.g. ConceptNet). |
| `www_uri` | string | External web URI. |

---

### Label

| Field | Type | Description |
|-------|------|-------------|
| `id` | integer | Label ID. |
| `text` | string | Label text. |
| `text_diacritised` | string \| null | Diacritised version (e.g. for Arabic). |
| `description` | string \| null | Optional description. |
| `language` | string | ISO 639-3 code (e.g. "eng"). |
| `picto` | [Picto](#picto) | Associated picto. May be expanded (e.g. with `symbolset`) when `expand` is used. |

---

### Language

| Field | Type | Description |
|-------|------|-------------|
| `id` | integer | Language ID. |
| `name` | string | Language name (e.g. "English"). |
| `scope` | string | One of: `I`, `M`, `S` (see ISO 639-3 scope). |
| `category` | string | One of: `L`, `E`, `C`, `A`, `H`, `S` (see ISO 639-3 types). |
| `iso639_1` | string \| null | Two-letter code. |
| `iso639_2b` | string \| null | ISO 639-2 bibliographic. |
| `iso639_2t` | string \| null | ISO 639-2 terminological. |
| `iso639_3` | string | Three-letter code. |

---

### Symbolset

| Field | Type | Description |
|-------|------|-------------|
| `id` | integer | Symbolset ID. |
| `slug` | string | URL slug (e.g. "arasaac"). |
| `name` | string | Display name. |
| `publisher` | string | Publisher name. |
| `publisher_url` | string | Publisher URL. |
| `status` | string | One of: `published`, `draft`. |
| `licence` | [Licence](#licence) | Licence. |
| `featured_level` | integer \| null | Optional featured ordering. |
| `logo_url` | string \| null | Full URL for the symbolset logo (CarrierWave path `…/uploads/{APP_ENV}/symbolset/logo/{id}/{file}`). Omitted when absent. |

---

### Licence

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Licence name. |
| `url` | string \| null | Licence URL. |
| `version` | string \| null | Version. |
| `properties` | string \| null | Short properties (e.g. "by-nc-sa"). |

---

### Picto (full)

Used in Concept and Label responses.

| Field | Type | Description |
|-------|------|-------------|
| `id` | integer | Picto ID. |
| `symbolset_id` | integer | Symbolset ID. |
| `part_of_speech` | string | One of: `noun`, `verb`, `adjective`, `adverb`, `pronoun`, `preposition`, `conjunction`, `interjection`, `article`, `modifier`. |
| `image_url` | string | Full URL to download image (e.g. PNG). |
| `native_format` | string | One of: `svg`, `png`. |
| `adaptable` | boolean \| null | Whether the picto is adaptable. |
| `symbolset` | [Symbolset](#symbolset) \| absent | Present only when expanded (e.g. via `expand=picto.symbolset`). |

---

### PictoSummary

Used in the list of pictos returned by GET `/v1/pictos`.

| Field | Type | Description |
|-------|------|-------------|
| `id` | integer | Picto ID. |
| `part_of_speech` | string | Same values as Picto. |
| `image_url` | string | Full URL to image. |
| `native_format` | string | `svg` or `png`. |
| `labels` | array of [LabelSummary](#labelsummary) | Authoritative labels only, in all available languages. |

---

### LabelSummary

| Field | Type | Description |
|-------|------|-------------|
| `language` | string | ISO 639-3 code. |
| `text` | string | Label text. |
| `text_diacritised` | string \| null | Diacritised version if present. |

---

### PagedPictosResponse

| Field | Type | Description |
|-------|------|-------------|
| `items` | array of [PictoSummary](#pictosummary) | Page of pictos. |
| `total` | integer | Total count (before pagination). |
| `deletions` | array of integers | (Delta mode only.) IDs of pictos archived since `since`. |
| `last_updated` | string \| null | (Delta mode only.) ISO 8601 timestamp of latest change. |

---

### CodingFramework

| Field | Type | Description |
|-------|------|-------------|
| `id` | integer | Framework ID. |
| `name` | string | e.g. "ConceptNet". |
| `structure` | string | One of: `linked_data`, `legacy`. |

---

### User

| Field | Type | Description |
|-------|------|-------------|
| `id` | integer | User ID. |
| `prename` | string | First name. |
| `surname` | string | Surname. |
| `default_hair_colour` | string \| null | Hex colour. |
| `default_skin_colour` | string \| null | Hex colour. |

---

### Error response

Used for 400, 401, 404, etc.

| Field | Type | Description |
|-------|------|-------------|
| `code` | integer | HTTP-style code (e.g. 401, 404). |
| `error` | string | Human-readable message. |

**Example:**

```json
{
  "code": 401,
  "error": "A valid, active API key is required. Provide it in the Authorization header as \"ApiKey <key>\" or in the X-Api-Key header."
}
```

---

## 5. Summary table

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/v1/concepts/suggest` | API Key | Search concepts by query. |
| GET | `/v1/concepts/:id` | API Key | Get concept by ID. |
| GET | `/v1/labels/search` | API Key | Search labels by query. |
| GET | `/v1/labels/:id` | API Key | Get label by ID. |
| GET | `/v1/languages/active` | API Key | List active languages. |
| GET | `/v1/symbolsets` | API Key | List published symbolsets. |
| GET | `/v1/pictos` | API Key | List pictos for a symbolset (full or delta). |
| GET | `/v1/user` | OAuth2 `profile` | Current user profile. |
| PATCH | `/v1/user` | OAuth2 `profile` | Update current user. |

---

## 6. Notes for Go re-implementation

1. **Slugs and published state:** Only **published** symbolsets are accepted for `symbolset` query/path params and for listing; reject or 404 for draft/unpublished slugs.
2. **Authoritative labels:** Label search and picto label lists use only “authoritative” labels (e.g. a source flag or equivalent in your schema).
3. **Picto visibility:** Pictos listed in GET `/v1/pictos` are non-archived and have visibility “everybody” (or equivalent).
4. **Expand:** The `expand` parameter is a space-separated list of field paths; the server may optionally embed nested entities (e.g. `picto.symbolset`) when requested.
5. **Pagination:** For GET `/v1/pictos`, `page` and `per_page` are 1-based; `per_page` is clamped between 1 and 100.
6. **Delta sync:** The `since` parameter and `deletions` / `last_updated` fields support clients that do incremental sync of a symbolset.

This spec reflects the behavior of the Ruby Grape API under `app/api/global_symbols` and is intended to be the single source of truth for a Go (or other) re-implementation of that API surface.
