# Fronko Backend

The Fronko API server. It's a single Go binary built on the standard library's `net/http`, backed by PostgreSQL through `pgx`.

It handles:

- **Accounts.** Username/password registration and login. Sessions are JWTs carried in an HttpOnly cookie.
- **Profiles ("cards").** Each user can own several public profiles. Each one has a unique slug and a free-form JSONB `data` document.
- **Leads.** Anonymous visitors can submit their name, email and a message to a profile. The owner can list those leads.

For the full endpoint reference, see [API.md](./API.md).

---

## Contents

- [Tech stack](#tech-stack)
- [Project layout](#project-layout)
- [Getting started](#getting-started)
- [Configuration](#configuration)
- [Database](#database)
- [Request pipeline](#request-pipeline)
- [Authentication & sessions](#authentication--sessions)
- [Security measures](#security-measures)
- [Error handling conventions](#error-handling-conventions)
- [Testing](#testing)
- [Docker image](#docker-image)
- [Known limitations](#known-limitations)

---

## Tech stack

| Concern         | Choice                                                                 |
| --------------- | ---------------------------------------------------------------------- |
| Language        | Go 1.27                                                                |
| HTTP            | `net/http` with Go 1.22+ method/path patterns (`GET /api/profiles/{slug}`) |
| Database driver | [`jackc/pgx/v5`](https://github.com/jackc/pgx) (`pgxpool`)             |
| Migrations      | [`golang-migrate`](https://github.com/golang-migrate/migrate) (run by the container entrypoint) |
| Auth            | [`golang-jwt/jwt/v5`](https://github.com/golang-jwt/jwt) (HS256) and `bcrypt` |
| Rate limiting   | `golang.org/x/time/rate` (token bucket, in memory)                     |
| Tests           | `testing` and [`testify`](https://github.com/stretchr/testify)         |

## Project layout

```
backend/
├── cmd/fronko/main.go        # Entry point: config, DI wiring, routes, graceful shutdown
├── internal/
│   ├── auth/auth.go              # Password hashing (bcrypt) and JWT issue/validate
│   ├── config/config.go          # Environment variable loading and validation
│   ├── database/db.go            # pgxpool construction and connectivity check
│   ├── handlers/
│   │   ├── auth.go               # /auth/login, /auth/register, /auth/logout, /api/me/user
│   │   ├── files.go              # File library: upload (type sniffing), list, rename, delete, public redirect
│   │   ├── storage.go            # StorageService (decrypt keys → bucket client) and /api/me/storage
│   │   ├── profile.go            # Profile CRUD and public slug lookup
│   │   ├── lead.go               # Lead submission (public), paginated listing and per-profile listing (owner)
│   │   ├── respond.go            # JSON encode/decode helpers, body size cap
│   │   └── session.go            # Session cookie set/clear
│   ├── middleware/
│   │   ├── cors.go               # CORS headers + preflight for CORS_ALLOWED_ORIGINS
│   │   ├── jwt.go                # Cookie → JWT → user ID in request context
│   │   ├── origin.go             # Same-origin check for state-changing requests
│   │   └── ratelimit.go          # Per-IP token bucket limiter
│   ├── models/models.go          # User, Profile, PublicProfile, Lead, StorageSettings, File
│   ├── repository/repository.go  # All SQL; maps pg errors to ErrNotFound / ErrConflict
│   ├── secrets/secrets.go        # AES-256-GCM sealing for storage keys at rest
│   └── storage/storage.go        # S3 client (aws-sdk-go-v2), endpoint validation, SSRF-safe dialer
├── migrations/                   # golang-migrate SQL files (up/down)
├── Dockerfile                    # Multi-stage build; bundles the migrate CLI
├── entrypoint.sh                 # Runs migrations, then execs the server
└── Makefile
```

Dependencies are wired by hand in `cmd/fronko/main.go`. It goes config → pool → repository → auth service → handlers → middleware → mux. There is no DI framework.

## Getting started

### Prerequisites

- Go 1.27+
- PostgreSQL 14+ (any reachable instance)
- [`migrate` CLI](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate), built with the `postgres` tag, to apply migrations outside Docker:
  ```bash
  go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1
  ```

### Run locally

```bash
cd backend

export DATABASE_URL='postgres://fronko:password@localhost:5432/fronko?sslmode=disable'
export JWT_SECRET="$(openssl rand -base64 32)"
export COOKIE_SECURE=false   # plain http in local dev
# Optional: enables photo/brochure storage. Add STORAGE_ALLOW_PRIVATE_ENDPOINTS=true
# to connect to a local S3 server (MinIO, SeaweedFS) over http.
export SECRETS_KEY="$(openssl rand -base64 32)"

# Apply the schema
migrate -path migrations -database "$DATABASE_URL" up

# Build and run (listens on :8080 by default)
make run
```

Check that it's up:

```bash
curl localhost:8080/health   # → OK
```

Then run the frontend dev server (`cd ../frontend && npm run dev`). It proxies `/api` and `/auth` to `localhost:8080`.

### Make targets

| Target                  | What it does                                                     |
| ----------------------- | ---------------------------------------------------------------- |
| `make build`            | Builds `bin/fronko`                                          |
| `make run`              | Builds, then runs the binary                                     |
| `make test`             | Unit tests with the race detector (`go test -v -race ./...`)     |
| `make test-integration` | Repository tests against a real database (see [Testing](#testing)) |
| `make clean`            | Removes `bin/`                                                   |

## Configuration

All configuration comes from environment variables and is read once at startup by `config.Load()`.

| Variable        | Required | Default | Description |
| --------------- | :------: | ------- | ----------- |
| `DATABASE_URL`  | ✅ | none | PostgreSQL connection string (pgx format). |
| `JWT_SECRET`    | ✅ | none | HMAC key for signing session JWTs. **Must be at least 16 characters**, or the server refuses to start. Generate one with `openssl rand -base64 32`. |
| `PORT`          |    | `8080` | Port to listen on. |
| `COOKIE_SECURE` |    | `true` | Puts the `Secure` flag on the session cookie. Only the exact value `false` disables it. Set it to `false` only when serving over plain HTTP, for example in local development. |
| `TRUST_PROXY`   |    | `false` | When set to the exact value `true`, the rate limiter takes the client IP from the `X-Real-IP` header. **Enable this only behind a proxy that always sets that header**, such as the bundled nginx. Otherwise clients can spoof it to dodge rate limits. |
| `SECRETS_KEY`   |    | none | Base64 of 32 random bytes (`openssl rand -base64 32`). It encrypts users' storage keys. **Unset disables file storage**: the storage endpoints return 503, and everything else works. A malformed value stops startup. **Keep it stable and backed up**: if it changes or is lost, saved storage keys can't be decrypted and users must re-enter them. |
| `CORS_ALLOWED_ORIGINS` |    | none | Comma-separated browser origins (e.g. `https://fronko.com`) allowed to call the API cross-origin with the session cookie. Matching requests get `Access-Control-Allow-Origin` + `Allow-Credentials`, preflights are answered with 204, and the same-origin check accepts them. Leave empty when the frontend's nginx proxies the API (same-origin). In Docker it's set from `FRONTEND_URL`. |
| `STORAGE_ALLOW_PRIVATE_ENDPOINTS` | | `false` | `true` lets storage endpoints use `http` and private or loopback addresses, e.g. a local MinIO. **Development only**: in production it would let users make the server connect to internal hosts. |

### Server and pool settings

These are hard-coded:

| Setting | Value |
| ------- | ----- |
| `ReadHeaderTimeout` / `ReadTimeout` / `WriteTimeout` / `IdleTimeout` | 5s / 15s / 15s / 60s |
| Graceful shutdown window (on `SIGINT`/`SIGTERM`) | 5s |
| `pgxpool` Max / Min connections | 25 / 5 |
| `pgxpool` MaxConnLifetime / MaxConnIdleTime | 5m / 1m |
| Max request body | 64 KiB |

## Database

The schema lives in `migrations/` (001 core tables, 002 leads paging index, 003 `user_storage` and `files`).
- `user_storage` holds one row per user with encrypted key columns (`BYTEA`).
- `files` stores a random `public_id`, the `bucket` and `object_key`, the sniffed `kind` and `content_type`, the size, the original name and an optional title. The bucket is stored per file, so changing buckets later doesn't silently re-point old files.

The original schema file is `migrations/001_initial_schema.up.sql`. It uses `IF NOT EXISTS` throughout, so it's safe to run against a database whose schema was applied by hand.

```mermaid
erDiagram
    users ||--o{ profiles : owns
    profiles ||--o{ leads : receives

    users {
        bigint user_id PK
        text username "UNIQUE, case-insensitive unique index"
        text password_hash "bcrypt"
        timestamptz created_at
        timestamptz updated_at
    }
    profiles {
        bigint profile_id PK
        bigint user_id FK "ON DELETE CASCADE"
        text slug "UNIQUE, case-insensitive unique index"
        jsonb data "default {}"
        timestamptz created_at
        timestamptz updated_at
    }
    leads {
        bigint lead_id PK
        bigint profile_id FK "ON DELETE CASCADE"
        text name
        text email
        text notes "nullable"
        timestamptz created_at
    }
```

**Indexes**

| Index | Purpose |
| ----- | ------- |
| `users_username_lower_idx` (unique, `LOWER(username)`) | Case-insensitive login, and stops `Alice` and `alice` from coexisting |
| `profiles_slug_lower_idx` (unique, `LOWER(slug)`) | Case-insensitive slug routing (`/p/Faiz` = `/p/faiz`) |
| `profiles_user_id_idx` | Listing a user's profiles (Postgres does not auto-index FKs) |
| `profiles_data_gin_idx` (GIN on `data`) | Room for future JSONB queries |
| `leads_profile_id_idx` | Listing a profile's leads |
| `leads_profile_id_created_at_idx` (`profile_id, created_at DESC`) | Newest-first paging of leads (migration 002) |

**Cascades.** Deleting a user deletes their profiles, and deleting a profile deletes its leads.

**The `data` column.** The backend treats `data` as opaque. It only checks that it is a JSON object, and stores `{}` when the field is missing or `null`. The frontend owns its shape. See [`CardData`](../frontend/README.md#card-data-model).

### Adding a migration

Add a numbered pair next to the existing files:

```
migrations/002_<description>.up.sql
migrations/002_<description>.down.sql
```

The Docker entrypoint runs `migrate ... up` on every container start. Locally, run the same command by hand (see [Run locally](#run-locally)).

### Repository errors

`internal/repository` turns driver errors into two sentinel errors, so handlers never see Postgres codes:

| Postgres condition | Repository error | Typical HTTP status |
| ------------------ | ---------------- | ------------------ |
| `pgx.ErrNoRows` | `ErrNotFound` | 404 |
| `23505` unique_violation | `ErrConflict` | 409 |
| `23503` foreign_key_violation | `ErrNotFound` | 404 (e.g. a lead for a missing profile) |

`ListLeadsForUser` takes a `LeadFilter` (profile, search, since, limit/offset) and runs two queries: a `COUNT(*)` for the total and the page itself. Both join `profiles` on `user_id`, so a filter naming another user's profile simply matches nothing. Search input is escaped so `%` and `_` match literally.

Ownership checks happen in SQL: `WHERE profile_id = $1 AND user_id = $2`. So another user's profile looks exactly like a missing one (`ErrNotFound`), and the API never confirms that someone else's profile ID exists.

## Request pipeline

```
Request
  └─ CORS                             (all routes; CORS headers + preflight for CORS_ALLOWED_ORIGINS)
     └─ SameOrigin                    (rejects cross-origin POST/PUT/DELETE not in CORS_ALLOWED_ORIGINS)
          └─ ServeMux
               ├─ GET  /health
               ├─ /auth/login, /auth/register → authLimiter → handler
               ├─ /auth/logout                → handler
               ├─ GET  /api/profiles/{slug}   → handler
               ├─ POST /api/profiles/{id}/leads → leadLimiter → handler
               └─ /api/me/*                   → JWTMiddleware → protected ServeMux → handler
```

Protected routes live on their own `ServeMux`, which is mounted at `/api/me/` behind `JWTMiddleware`. Any route added under `/api/me/` is authenticated automatically. Inside a protected handler, call `middleware.UserID(r.Context())` to get the caller's ID.

## Authentication & sessions

1. `POST /auth/register` or `POST /auth/login` verifies the credentials and issues an HS256 JWT. Its claims are `sub` (the user ID), `iat` and `exp`, and it is valid for 24h (`auth.SessionTTL`).
2. The token is sent **only** as a cookie and never appears in a response body:

   | Attribute | Value |
   | --------- | ----- |
   | Name | `fronko_session` |
   | `HttpOnly` | yes. Page scripts, including any XSS, can't read it |
   | `SameSite` | `Lax` |
   | `Secure` | controlled by `COOKIE_SECURE` (default on) |
   | `Path` / `Max-Age` | `/` / 86400 |

3. `JWTMiddleware` reads the cookie, validates it, and puts the user ID into the request context. Validation pins HS256 and requires `exp`.
4. Because the SPA can't read the cookie, it calls `GET /api/me/user` to find out who is signed in.
5. `POST /auth/logout` clears the cookie. JWTs are stateless and **are not revoked server-side**, so a copied token stays valid until it expires.

If the account behind a valid token no longer exists, `GET /api/me/user` clears the cookie and returns 401.

## Security measures

| Threat | Mitigation | Where |
| ------ | ---------- | ----- |
| Token theft via XSS | JWT lives only in an HttpOnly cookie | `handlers/session.go` |
| CSRF | `SameSite=Lax`, plus `SameOrigin` middleware that rejects POST/PUT/DELETE whose `Origin` host differs from `Host` unless it's listed in `CORS_ALLOWED_ORIGINS` | `middleware/origin.go` |
| Cross-origin reads | CORS headers are only sent to origins in `CORS_ALLOWED_ORIGINS` (exact match, no wildcard) | `middleware/cors.go` |
| JWT algorithm confusion | `jwt.WithValidMethods(["HS256"])` and `WithExpirationRequired()` | `auth/auth.go` |
| Weak or empty signing key | Startup fails when `JWT_SECRET` is shorter than 16 chars | `config/config.go` |
| Username enumeration via timing | Unknown usernames still run a bcrypt compare against a dummy hash | `handlers/auth.go` |
| Brute force or lead spam | Per-IP token-bucket rate limits on auth and lead endpoints | `middleware/ratelimit.go` |
| Rate-limit bypass via spoofed headers | `X-Real-IP` is only trusted when `TRUST_PROXY=true` | `middleware/ratelimit.go` |
| bcrypt 72-byte truncation | Passwords are limited to 8–72 bytes at registration | `handlers/auth.go` |
| Oversized bodies | `http.MaxBytesReader` caps JSON bodies at 64 KiB and uploads at 21 MiB; nginx caps `/api/` at 25 MB | `handlers/respond.go`, `handlers/files.go`, `frontend/docker/default.conf.template` |
| Storage keys leaking from the database | AES-256-GCM with `SECRETS_KEY`. The AAD binds each ciphertext to its user and field. Keys are write-only in the API, and only a 4-char hint is stored in plain text | `secrets/`, `handlers/storage.go` |
| SSRF through a user-supplied endpoint | https only, no literal private addresses, and a dialer `Control` hook that rejects private, loopback, link-local, CGNAT and metadata addresses **after DNS resolution** (defeats DNS rebinding). Redirects aren't followed and env proxies are ignored | `storage/storage.go` |
| Malicious uploads (HTML/SVG posing as images) | The type is sniffed from the bytes, and only JPEG, PNG, WebP and PDF are accepted. The stored content type comes from sniffing, not the client | `handlers/files.go` |
| Enumerating other people's files | Public file IDs are 128-bit random values. Owner endpoints scope by `user_id`, and public profiles only list files their card references | `handlers/files.go`, `repository/repository.go` |
| Provider errors leaking internals | `storage.Describe` turns SDK errors into short messages; the raw error is only logged | `storage/storage.go` |
| IDOR on profiles and leads | Ownership enforced in the SQL `WHERE` clause; foreign profiles return 404 | `repository/repository.go` |
| Leaking owner info publicly | Public lookups return `PublicProfile` (id, slug, data), with no `user_id` or timestamps | `models/models.go` |
| SQL injection | All queries are parameterized | `repository/repository.go` |

### Rate limits

Defined as constants in `cmd/fronko/main.go`:

| Limiter | Applies to | Burst | Refill |
| ------- | ---------- | ----- | ------ |
| `authLimiter` | `POST /auth/login` **and** `POST /auth/register` (one shared bucket per IP) | 10 | 1 per 10s |
| `leadLimiter` | `POST /api/profiles/{id}/leads` | 5 | 1 per 15s |
| `uploadLimiter` | `POST /api/me/files` | 10 | 1 per 6s |

A rejected request gets `429 Too Many Requests` with a `Retry-After` header in seconds, and it does not use up a token. Buckets idle for more than 10 minutes are evicted every minute.

## Error handling conventions

- Every error response is JSON: `{"error": "<human-readable message>"}`. The messages are written for end users, and the frontend shows them as they are.
- Unexpected errors are logged with `log.Printf` and returned as a generic `"internal error"`. Driver details never reach the client.
- `handlers/respond.go` provides `writeJSON`, `writeError` and `decodeJSON`. Use them in new handlers so responses stay consistent.

## Testing

```bash
make test               # unit tests
make test-integration   # repository integration tests (needs a database, see below)
```

Unit tests cover:
- `middleware`: rate limiter, same-origin check, CORS.
- `secrets`: sealing round trip, tamper, wrong AAD and wrong key.
- `storage`: endpoint validation, private-address dialing, the connection probe against a fake S3 server.
- `handlers`: upload type sniffing and size limits, file-name cleaning, which file ids a public card reveals.

The integration tests (`internal/repository/repository_test.go`) sit behind the `integration` build tag. They read **`TEST_DATABASE_URL`**, not `DATABASE_URL`, because they `TRUNCATE` every table. Point them at a disposable database that already has the migrations applied:

```bash
export TEST_DATABASE_URL='postgres://fronko:password@localhost:5432/fronko_test?sslmode=disable'
migrate -path migrations -database "$TEST_DATABASE_URL" up
make test-integration
```

If `TEST_DATABASE_URL` is unset, the tests are skipped.

A throwaway database is the easiest option:

```bash
docker run -d --rm --name fronko-test-db -e POSTGRES_USER=test -e POSTGRES_PASSWORD=test \
  -e POSTGRES_DB=fronko_test -p 55999:5432 postgres:18
cat migrations/*.up.sql | docker exec -i fronko-test-db psql -q -U test -d fronko_test
TEST_DATABASE_URL='postgres://test:test@localhost:55999/fronko_test?sslmode=disable' make test-integration
docker stop fronko-test-db
```

## Docker image

`Dockerfile` is a two-stage build:

1. **Builder** (`golang:1.27-alpine`): installs the pinned `migrate` CLI (v4.19.1) and builds a static binary (`CGO_ENABLED=0`).
2. **Runtime** (`alpine:3`): ships the binary, the `migrate` CLI and `migrations/`, and runs as an unprivileged user (`uid 10001`).

`entrypoint.sh` requires `DATABASE_URL`, runs `migrate up`, then `exec`s the server. Every container start therefore brings the schema up to date before serving traffic.

```bash
docker build -t fronko-backend .
docker run --rm -p 8080:8080 \
  -e DATABASE_URL=... -e JWT_SECRET=... -e COOKIE_SECURE=false \
  fronko-backend
```

For the full stack (backend + nginx-served frontend), see `deploy/` and the [root README](../README.md).

## Known limitations

- **Rate limits are per process.** Several backend replicas each enforce their own budget. Sharing limits across replicas needs a shared store such as Redis.
- **No server-side logout.** Logging out removes the cookie but doesn't invalidate the JWT. It stays valid until `exp`, up to 24h.
- **`collect_leads` isn't enforced by the server.** The frontend hides the lead form when a card's `data.collect_leads` is `false`, but `POST /api/profiles/{id}/leads` still accepts submissions.
- **`lead_count` is only filled in by `GET /api/me/profiles`.** Create, update and single-profile responses return `0`.
- **No email or password recovery yet.** See the backlog in the root README.
- **Anyone with a file's link can open it.** `/api/files/{id}` needs no sign-in, which is how public cards show photos and brochures. IDs are 128-bit random values and only appear on cards that use them, but a link that is shared stays usable until the file is deleted.
- **Uploads are buffered in memory**, up to 20 MB per request, so they can be type-checked before writing to the bucket. The upload rate limit keeps this bounded per IP.
- **Deleting a file doesn't edit cards.** Cards that referenced it simply stop showing it; the editor shows the stale entry until removed.
- **Changing or losing `SECRETS_KEY` breaks saved storage keys.** Users have to re-enter them. Rotation isn't automated yet; the version byte in each ciphertext is there for it.
