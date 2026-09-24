# Expert Listing API

A property listings REST API built for Expert Listing Limited, a Nigerian proptech platform.

**Live API:** https://expertlisting-kccx.onrender.com  
**Swagger UI:** https://expertlisting-kccx.onrender.com/swagger/

## Contents

1. [Overview](#overview)
2. [Requirements](#requirements)
3. [Technology choices](#technology-choices)
4. [Architecture](#architecture)
5. [PostgreSQL setup](#postgresql-setup)
6. [Environment variables](#environment-variables)
7. [Running migrations](#running-migrations)
8. [Running locally](#running-locally)
9. [Running tests](#running-tests)
10. [API endpoints](#api-endpoints)
11. [Swagger / OpenAPI documentation](#swagger--openapi-documentation)
12. [Search and filtering](#search-and-filtering)
13. [Geospatial search](#geospatial-search)
14. [Pagination](#pagination)
15. [Error handling](#error-handling)
16. [Database indexes](#database-indexes)
17. [Design decisions](#design-decisions)
18. [Trade-offs](#trade-offs)
19. [What I would improve with more time](#what-i-would-improve-with-more-time)

---

## Overview

Implements CRUD operations for property listings, full-text filtering by type,
price, and bedroom count, geospatial radius search using the Haversine formula
computed in PostgreSQL, and standard pagination.

---

## Requirements

- Go 1.22 or later
- PostgreSQL 14 or later

---

## Technology choices

| Choice | Reason |
|---|---|
| `net/http` | Standard library. The routing requirements here are simple; no framework overhead is needed. Go 1.22's enhanced mux supports method+path patterns natively. |
| `pgx / pgxpool` | Idiomatic, high-performance PostgreSQL driver. Avoids the abstraction overhead of an ORM, keeps SQL explicit and reviewable. |
| `pgxpool.Pool` | Connection pooling at the application level. Reuses connections across requests rather than opening a new connection per request. |
| `logrus` | Structured JSON logging with level control. Easy to wire into a log aggregator in production. |
| `godotenv` | Loads `.env` for local development without affecting production where variables come from the environment directly. |

No web framework, no ORM, no Docker.

---

## Architecture

```
expert-listing/
├── cmd/api/        Server entry point — wires components together, starts HTTP server
├── internal/
│   ├── config/     Environment variable loading
│   ├── db/         PostgreSQL connection pool setup
│   ├── model/      Domain types shared between handler and repository layers
│   ├── respond/    JSON response helpers (consistent format across all handlers)
│   ├── repository/ SQL operations against PostgreSQL
│   └── handler/    HTTP handlers and input validation
├── docs/           Embedded OpenAPI specification
├── migrations/     Plain SQL migration files
└── tests/          Handler unit tests (httptest) and repository integration tests
```

The handler layer depends on a `ListingStore` interface, which the repository
satisfies. This allows handler unit tests to run without a real database using
a simple in-memory mock.

---

## PostgreSQL setup

### Install PostgreSQL (Ubuntu / Debian)

```bash
sudo apt update
sudo apt install postgresql postgresql-contrib
sudo systemctl start postgresql
```

### macOS (Homebrew)

```bash
brew install postgresql@16
brew services start postgresql@16
```

### Create a database

```bash
sudo -u postgres psql
```

```sql
CREATE USER expert_listing_user WITH PASSWORD 'your_password';
CREATE DATABASE expert_listing OWNER expert_listing_user;
GRANT ALL PRIVILEGES ON DATABASE expert_listing TO expert_listing_user;
\q
```

---

## Environment variables

Copy `.env.example` to `.env` and fill in your values:

```bash
cp .env.example .env
```

| Variable | Description | Default |
|---|---|---|
| `DATABASE_URL` | PostgreSQL connection string | **required** |
| `PORT` | HTTP server port | `8080` |
| `APP_ENV` | `development` or `production` | `development` |

**Example `DATABASE_URL`:**

```
postgres://expert_listing_user:your_password@localhost:5432/expert_listing?sslmode=disable
```

---

## Running migrations

Migrations are plain SQL files. Apply them with `psql`:

```bash
psql "$DATABASE_URL" -f migrations/001_create_listings.up.sql
```

To roll back:

```bash
psql "$DATABASE_URL" -f migrations/001_create_listings.down.sql
```

---

## Running locally

```bash
# 1. Install dependencies
go mod download

# 2. Apply migrations (see above)

# 3. Start the server
go run ./cmd/api
```

The server starts on port `8080` by default (or the value of `PORT`).

---

## Running tests

### Unit tests (no database required)

```bash
go test ./tests/... -run TestCreate -v
go test ./tests/... -run TestGetByID -v
go test ./tests/... -run TestUpdate -v
go test ./tests/... -run TestDelete -v
go test ./tests/... -run TestList -v
```

Or run all tests (integration tests are skipped automatically when
`TEST_DATABASE_URL` is not set):

```bash
go test ./...
```

### Integration tests (PostgreSQL required)

Create a test database and apply the migration, then:

```bash
TEST_DATABASE_URL="postgres://user:pass@localhost:5432/expert_listing_test?sslmode=disable" \
  go test ./tests/... -v
```

Integration tests clean up their own data and do not drop the schema between
runs.

---

## API endpoints

All endpoints are under the `/api/v1` prefix.

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/v1/listings` | Create a listing |
| `GET` | `/api/v1/listings` | List and search listings |
| `GET` | `/api/v1/listings/{id}` | Get a listing by ID |
| `PATCH` | `/api/v1/listings/{id}` | Partially update a listing |
| `DELETE` | `/api/v1/listings/{id}` | Delete a listing |

### Create a listing

```bash
curl -X POST https://expertlisting-kccx.onrender.com/api/v1/listings \
  -H 'Content-Type: application/json' \
  -d '{
    "title":     "3 Bedroom Flat in Lekki Phase 1",
    "price":     2500000,
    "type":      "rent",
    "bedrooms":  3,
    "latitude":  6.4281,
    "longitude": 3.4219,
    "agent_id":  "550e8400-e29b-41d4-a716-446655440000"
  }'
```

**Response 201:**

```json
{
  "data": {
    "id": "b5a2c0e1-...",
    "title": "3 Bedroom Flat in Lekki Phase 1",
    "price": 2500000,
    "type": "rent",
    "bedrooms": 3,
    "latitude": 6.4281,
    "longitude": 3.4219,
    "agent_id": "550e8400-...",
    "created_at": "2026-09-24T10:00:00Z",
    "updated_at": "2026-09-24T10:00:00Z"
  }
}
```

### Get a listing

```bash
curl https://expertlisting-kccx.onrender.com/api/v1/listings/b5a2c0e1-...
```

### Update a listing (partial)

```bash
curl -X PATCH https://expertlisting-kccx.onrender.com/api/v1/listings/b5a2c0e1-... \
  -H 'Content-Type: application/json' \
  -d '{"price": 2800000, "type": "sale"}'
```

### Delete a listing

```bash
curl -X DELETE https://expertlisting-kccx.onrender.com/api/v1/listings/b5a2c0e1-...
```

---

## Swagger / OpenAPI documentation

Start the server and open the Swagger UI in a browser:

```
https://expertlisting-kccx.onrender.com/swagger/
```

The raw OpenAPI YAML specification is also available at:

```
https://expertlisting-kccx.onrender.com/swagger/openapi.yaml
```

The spec is embedded in the binary at build time from `docs/openapi.yaml`.

---

## Search and filtering

All parameters are optional and can be combined freely.

| Parameter | Type | Description |
|---|---|---|
| `type` | string | Property type: `rent`, `sale`, or `shortlet` |
| `min_price` | number | Minimum price (inclusive) |
| `max_price` | number | Maximum price (inclusive) |
| `bedrooms` | integer | Exact bedroom count |
| `lat` | number | Latitude of search origin (requires `lng` and `radius`) |
| `lng` | number | Longitude of search origin (requires `lat` and `radius`) |
| `radius` | number | Search radius in kilometres (requires `lat` and `lng`) |
| `page` | integer | Page number (default `1`) |
| `limit` | integer | Results per page, 1–100 (default `20`) |

**Examples:**

```bash
# Filter by type and price
GET /api/v1/listings?type=rent&min_price=500000&max_price=3000000

# Filter by bedrooms
GET /api/v1/listings?bedrooms=3

# Geographic search within 10 km of Lagos Island
GET /api/v1/listings?lat=6.454&lng=3.394&radius=10

# Combined
GET /api/v1/listings?type=rent&bedrooms=2&lat=6.5244&lng=3.3792&radius=10&page=1&limit=20
```

---

## Geospatial search

When `lat`, `lng`, and `radius` are supplied together, the API filters listings
within the given radius using the **Haversine formula** computed directly in
PostgreSQL:

```sql
6371.0 * acos(LEAST(1.0,
    cos(radians($lat)) * cos(radians(latitude))
    * cos(radians(longitude) - radians($lng))
    + sin(radians($lat)) * sin(radians(latitude))
))
```

`LEAST(1.0, ...)` guards against floating-point values marginally above 1.0
that would cause `acos` to return `NaN`.

**Why Haversine instead of PostGIS?**

The Haversine formula is mathematically correct to within about 0.3% for
distances under a few hundred kilometres — accurate enough for a property
search radius. It runs without any additional PostgreSQL extension, which keeps
the setup simple for this assessment.

**Filtering at the database level:** distance is computed in PostgreSQL inside
the `WHERE` clause. Only rows within the radius are returned; the Go
application never receives records that need to be discarded.

**Ordering:** geo results are sorted by `distance_km ASC` so the closest
listings appear first. The computed distance is returned in each listing as
`distance_km`.

**Upgrade path to PostGIS:** when the dataset grows significantly, PostGIS
`geography` columns and a `GIST` spatial index would reduce the full-table scan
to a fast index-assisted lookup. The application-level query would change from
the Haversine arithmetic to `ST_DWithin(location, ST_MakePoint($lng,$lat)::geography, $radius_metres)`, and results would be ordered by
`ST_Distance(...)`. The column would need a one-time migration.

---

## Pagination

Pagination is applied at the database level using `LIMIT` and `OFFSET` so only
the requested page is transferred from PostgreSQL to the application.

Default page size is 20. Maximum page size is 100.

**Response shape:**

```json
{
  "data": [...],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 125,
    "total_pages": 7
  }
}
```

Results without a geo filter are ordered by `created_at DESC` (newest first).
Geo results are ordered by `distance_km ASC`.

---

## Error handling

All errors follow a consistent JSON structure:

```json
{
  "error": {
    "code":    "VALIDATION_ERROR",
    "message": "validation failed",
    "details": {
      "type":  "must be one of: rent, sale, shortlet",
      "price": "must be >= 0"
    }
  }
}
```

| HTTP status | Code | Meaning |
|---|---|---|
| 400 | `VALIDATION_ERROR` | One or more request fields failed validation |
| 400 | `INVALID_JSON` | Request body is not valid JSON |
| 400 | `BAD_REQUEST` | Request is structurally valid but semantically wrong (e.g., no fields to update) |
| 400 | `INVALID_ID` | Path parameter is not a valid UUID |
| 404 | `NOT_FOUND` | Listing does not exist |
| 500 | `INTERNAL_ERROR` | Unexpected server error — no internal details are exposed |

---

## Database indexes

```sql
CREATE INDEX idx_listings_type       ON listings (type);
CREATE INDEX idx_listings_price      ON listings (price);
CREATE INDEX idx_listings_bedrooms   ON listings (bedrooms);
CREATE INDEX idx_listings_agent_id   ON listings (agent_id);
CREATE INDEX idx_listings_created_at ON listings (created_at DESC);
```

Single-column indexes were chosen deliberately. At this data scale the query
planner can intersect multiple single-column indexes efficiently. Composite
indexes (e.g. `(type, price)`) would only improve queries that always filter on
both columns together, which is not guaranteed here. Composite indexes become
worthwhile once real production query patterns are established.

The geo filter uses a full-table scan of the `latitude`/`longitude` columns.
This is acceptable for a moderate dataset. At larger scale a PostGIS `GIST`
index on a geography column would allow index-assisted radial filtering.

---

## Design decisions

**Why a `ListingStore` interface in the handler layer?**  
The interface exists specifically to enable handler unit tests to run without a
database. `*repository.ListingRepository` satisfies it implicitly. If testing
were not a concern, the handler would depend on the concrete type directly.

**Why `NUMERIC(15, 2)` for price?**  
Nigerian property prices are quoted in Naira and can reach tens of millions.
`NUMERIC` preserves exact decimal precision, which is appropriate for currency
even though we scan to `float64` in Go for simplicity.

**Why `DOUBLE PRECISION` for latitude/longitude?**  
The Haversine formula involves trigonometric functions; single-precision floats
accumulate significant error at the precision levels needed for
kilometre-accuracy geolocation.

**Why `updated_at` is set in SQL and also via a trigger?**  
The `UPDATE … SET updated_at = NOW()` in the query ensures the `RETURNING`
clause reflects the current timestamp immediately. The trigger is a safety net
in case a future developer runs a raw `UPDATE` that omits the field.

---

## Trade-offs

- **No PostGIS** — Haversine in SQL is accurate enough for a take-home but
  would not handle polar coordinates or very large datasets efficiently.
- **No authentication** — endpoints are public as specified. In production,
  agent mutations should be gated behind an API key or JWT.
- **Haversine full scan** — without a spatial index, the geo filter scans every
  listing. Acceptable at the assessment scale; PostGIS GIST would fix it.
- **Single `go test` package** — handler and repository tests are in the same
  package for simplicity. Splitting them into separate packages would enforce
  stronger API boundaries.

---

## What I would improve with more time

1. **PostGIS integration** — `geography` column + `GIST` index for efficient
   large-scale geospatial queries.
2. **Composite indexes** — profile query patterns from production traffic and
   add composite indexes where they consistently speed up common filters.
3. **Cursor-based pagination** — `OFFSET`-based pagination degrades on large
   tables; cursor pagination using `created_at` + `id` would scale better.
4. **Agent validation** — verify that `agent_id` references a real agent in a
   separate `agents` table rather than accepting any UUID.
5. **Request ID middleware** — attach a unique request ID to every response and
   log entry for traceability.
6. **Rate limiting** — protect the listing search endpoint from abuse.
7. **Integration test isolation** — use a dedicated schema per test run or
   `pgx` transactions that roll back, rather than deleting by ID.
