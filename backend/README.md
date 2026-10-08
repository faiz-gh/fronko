# Fronko Backend

The Fronko API server. It's a single Go binary built on the standard library's `net/http`, backed by PostgreSQL through `pgx`. One process serves the API, runs background jobs from a queue in Postgres, and runs periodic tasks.

It handles:

- **Accounts and organisations.** Username/password registration and login, emailed codes, roles (owner, admin, member), teams, and SAML single sign-on. Sessions are JWTs carried in an HttpOnly cookie.
- **Cards.** Each organisation has any number of public profiles, each with a slug unique within the organisation and a free-form JSONB `data` document. A card's public link is `/p/{org handle}/{slug}`.
- **Leads.** Anonymous visitors send their name, email, an optional mobile number and a message to a card. Leads are listed, searched and exported, and synced to CRMs and automation tools.
- **Files, branding, analytics and feedback.** A file library in the organisation's own S3 bucket, organisation branding, cookieless card analytics, and product feedback.
- **Integrations.** Lead sync (webhooks, Zapier, Make, n8n, HubSpot), booking pages on cards, a SCIM 2.0 server and SAML 2.0 sign-in.
- **Platform admin.** Separate admin accounts (created with `./fronko admin create`) get usage totals per organisation, trends, a feedback inbox, organisation suspension and an audit log.

For the endpoints, see [API.md](./API.md). For how the code is put together, see [Architecture](../docs/architecture.md).

---

## Contents

- [Tech stack](#tech-stack)
- [Project layout](#project-layout)
- [Getting started](#getting-started)
- [Configuration](#configuration)
- [Database](#database)
- [Authentication & sessions](#authentication--sessions)
- [Platform admin](#platform-admin)
- [Card analytics](#card-analytics)
- [Integrations](#integrations)
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
| HTTP            | `net/http` with Go 1.22+ method/path patterns (`GET /api/profiles/{org}/{slug}`) |
| Database driver | [`jackc/pgx/v5`](https://github.com/jackc/pgx) (`pgxpool`)             |
| Migrations      | [`golang-migrate`](https://github.com/golang-migrate/migrate) (run by the container entrypoint) |
| Auth            | [`golang-jwt/jwt/v5`](https://github.com/golang-jwt/jwt) (HS256) and `bcrypt` |
| Background work | A job queue and periodic tasks in Postgres (`internal/platform/jobs`)  |
| Integrations    | `golang.org/x/oauth2`, [`crewjam/saml`](https://github.com/crewjam/saml) |
| Rate limiting   | `golang.org/x/time/rate` (token bucket, in memory)                     |
| Tests           | `testing` and [`testify`](https://github.com/stretchr/testify)         |
| Lint            | `gofmt`, `go vet`, [`golangci-lint`](https://golangci-lint.run/) v2 (`.golangci.yml`) |

## Project layout

```
backend/
├── cmd/fronko/main.go        # subcommands: serve (default), admin, seed
├── internal/
│   ├── server/               # composition root: builds every module, runs HTTP, job workers and tasks
│   ├── app/                  # Module interfaces and route groups (Routes)
│   ├── platform/             # infrastructure, no business rules
│   │   ├── config/           # environment variables
│   │   ├── database/         # pool, Querier, MapError → ErrNotFound / ErrConflict
│   │   ├── httpx/            # JSON helpers, path ids, paging
│   │   ├── events/           # in-process domain event bus
│   │   ├── jobs/             # Postgres job queue, worker, periodic tasks
│   │   ├── mail/             # SMTP sender (implicit TLS / STARTTLS), log fallback, templates
│   │   ├── netguard/         # SSRF-safe HTTP client for user-supplied addresses
│   │   ├── ratelimit/        # per-IP token buckets
│   │   ├── secrets/          # AES-256-GCM sealing (SECRETS_KEY)
│   │   ├── storage/          # S3 client and endpoint validation
│   │   └── web/              # CORS and the same-origin check
│   ├── auth/                 # passwords, JWTs, email codes, session middleware, Principal, Scope, VisibleTo
│   ├── users/                # users, email codes, username and email rules
│   ├── account/              # register, login, logout, email verification and change, passwords
│   ├── orgs/                 # organisation settings, handle, user management
│   ├── teams/  branding/
│   ├── files/                # file library, card file references, storage settings
│   ├── cards/                # cards (profiles), public lookup, vCard
│   ├── leads/                # lead form, listing; publishes leads.Created
│   ├── analytics/            # event recording, reports, retention
│   ├── feedback/  platformadmin/
│   ├── integrations/         # catalog, connections, OAuth, lead dispatch, activity log
│   │   ├── providers/        # all.go (registry list), comingsoon.go, webhook/, hubspot/, calendar/
│   │   ├── sso/              # SAML service provider, email domains, SSO policy
│   │   └── directory/        # SCIM 2.0 server
│   ├── devseed/              # `fronko seed`: demo data (development only)
│   └── integrationtest/      # tests against a real database (build tag `integration`)
├── migrations/               # golang-migrate SQL files (up/down)
├── Dockerfile                # multi-stage build; bundles the migrate CLI
├── entrypoint.sh             # runs migrations, then execs the server
├── .golangci.yml
└── Makefile
```

Each feature package owns its handlers, its SQL (`store.go`) and its types, and registers its routes through `app.Routes`. `internal/server/server.go` wires everything by hand; there is no DI framework. [Architecture](../docs/architecture.md) explains the module contract, the route groups, events and jobs, and [Backend modules](../docs/backend-modules.md) shows how to add one.

## Getting started

### Prerequisites

- Go 1.27+
- PostgreSQL 14+ (any reachable instance)
- [`migrate` CLI](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate), built with the `postgres` tag, to apply migrations outside Docker:
  ```bash
  go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1
  ```
- Optional: [`golangci-lint`](https://golangci-lint.run/welcome/install/) v2, for `make lint`

### Run locally

```bash
cd backend

export DATABASE_URL='postgres://fronko:password@localhost:5432/fronko?sslmode=disable'
export JWT_SECRET="$(openssl rand -base64 32)"
export COOKIE_SECURE=false            # plain http in local dev
export FRONKO_ENV=development         # lets integrations reach local receivers; enables `seed`
export PUBLIC_URL=http://localhost:5173
# Optional: enables photo/brochure storage and integrations with secrets. Add
# STORAGE_ALLOW_PRIVATE_ENDPOINTS=true to connect to a local S3 server (MinIO, SeaweedFS) over http.
export SECRETS_KEY="$(openssl rand -base64 32)"

# Apply the schema
migrate -path migrations -database "$DATABASE_URL" up

# Optional: demo organisations, cards, leads and analytics
make seed

# Build and run (listens on :8080 by default)
make run
```

Check that it's up:

```bash
curl localhost:8080/health   # → OK
```

Then run the frontend dev server (`cd ../frontend && npm run dev`). It proxies `/api`, `/auth` and `/scim` to `localhost:8080`. The demo accounts are listed in [CONTRIBUTING](../CONTRIBUTING.md#demo-data).

### Make targets

| Target                  | What it does                                                     |
| ----------------------- | ---------------------------------------------------------------- |
| `make build`            | Builds `bin/fronko`                                              |
| `make run`              | Builds, then runs the server                                     |
| `make test`             | Unit tests with the race detector (`go test -v -race ./...`)     |
| `make test-integration` | Database tests against a disposable database (see [Testing](#testing)) |
| `make lint`             | `golangci-lint run ./...`                                        |
| `make seed`             | Demo data in `DATABASE_URL` (`FRONKO_ENV=development`); `ARGS=--reset` empties the database first |
| `make clean`            | Removes `bin/`                                                   |

### Subcommands

| Command | What it does |
| ------- | ------------ |
| `fronko` or `fronko serve` | Runs the server |
| `fronko admin create --email …` | Creates a platform admin (prompts for a password) |
| `fronko admin set-password --email …` | Replaces a platform admin's password and signs them out everywhere |
| `fronko seed [--reset]` | Demo data. Refused unless `FRONKO_ENV=development`, and on a database that already has organisations unless `--reset` |

## Configuration

All configuration comes from environment variables, read once at startup by `config.Load()`. A malformed value stops startup.

| Variable        | Required | Default | Description |
| --------------- | :------: | ------- | ----------- |
| `DATABASE_URL`  | ✅ | none | PostgreSQL connection string (pgx format). |
| `JWT_SECRET`    | ✅ | none | HMAC key for signing session JWTs; also the root for the keys that sign OAuth and SAML state and hash email codes. **At least 16 characters**, or the server refuses to start. Generate one with `openssl rand -base64 32`. |
| `PORT`          |    | `8080` | Port to listen on. |
| `COOKIE_SECURE` |    | `true` | Puts the `Secure` flag on the session cookie. Only the exact value `false` disables it, for plain HTTP in local development. |
| `TRUST_PROXY`   |    | `false` | When `true`, the rate limiter and analytics take the client IP from `X-Real-IP`. **Enable this only behind a proxy that always sets that header**, such as the bundled nginx. |
| `SECRETS_KEY`   |    | none | Base64 of 32 random bytes (`openssl rand -base64 32`). Encrypts storage keys and integration secrets. **Unset disables file storage** (the storage endpoints return 503) **and integrations that store a secret** (shown as unavailable). **Keep it stable and backed up**: if it changes or is lost, saved keys and secrets can't be decrypted. |
| `PUBLIC_URL`    |    | the first `CORS_ALLOWED_ORIGINS` entry | Where people reach the site, such as `https://cards.example.com`: an absolute `http(s)` URL without credentials, a query or a fragment; a trailing slash is dropped. Only needed when it isn't the first allowed origin (in Docker, the first `FRONTEND_URL`). Card links in lead payloads, the shared SSO sign-in link, and where people land after signing in or authorising are built from it; so are the OAuth, SAML and SCIM addresses, unless `PUBLIC_API_URL` says the API is elsewhere. **With neither set, HubSpot, SAML and SCIM are unavailable.** |
| `PUBLIC_API_URL` |   | `PUBLIC_URL` | Where the API is reached when it has its own domain, such as `https://api.cards.example.com`. Same rules as `PUBLIC_URL`. The OAuth redirect URL, SAML ACS and metadata URLs and the SCIM base URL are built on it, because the browser's session cookie (and other servers' requests) go to the API. Pages people land on (after signing in or authorising, the shared sign-in link, card links) stay on `PUBLIC_URL`. The production Compose file sets it from `BACKEND_URL`. |
| `JOB_WORKERS`   |    | `2` | Background jobs this instance runs at once. `0` queues jobs without running them, for instances that should only serve requests. |
| `FRONKO_ENV`    |    | `production` | `production` or `development`. Development lets integrations call private and local addresses (a webhook receiver on your machine) and allows `fronko seed`. **Never use development in production.** |
| `CORS_ALLOWED_ORIGINS` |    | none | Comma-separated browser origins allowed to call the API cross-origin with the session cookie. Leave empty when the frontend's nginx proxies the API (same-origin). In Docker it's set from `FRONTEND_URL`. |
| `SMTP_HOST`     |    | none | Outgoing mail server. **Unset logs each email (with its code) to stdout instead**, which is only useful in development. |
| `SMTP_PORT`     |    | `587` | `465` uses implicit TLS; any other port upgrades with STARTTLS when offered. |
| `SMTP_USERNAME` / `SMTP_PASSWORD` | | none | SMTP credentials. Auth is skipped when the username is empty. Credentials are never sent over an unencrypted connection (except to localhost). |
| `SMTP_FROM`     | with `SMTP_HOST` | none | Sender, e.g. `Fronko <no-reply@fronko.app>`. |
| `FEEDBACK_NOTIFY_EMAIL` | | none | Emailed for each piece of product feedback; also the Reply-To on admin replies and suspension emails. |
| `ANALYTICS_RETENTION_DAYS` | | `395` | Days of card analytics events to keep (1–3650). |
| `STORAGE_ALLOW_PRIVATE_ENDPOINTS` | | `false` | `true` lets storage endpoints use `http` and private addresses, e.g. a local MinIO. **Development only.** |

### Server and pool settings

These are hard-coded:

| Setting | Value |
| ------- | ----- |
| `ReadHeaderTimeout` / `ReadTimeout` / `WriteTimeout` / `IdleTimeout` | 5s / 15s / 15s / 60s |
| Graceful shutdown (on `SIGINT`/`SIGTERM`) | 5s for HTTP, then up to 10s for running jobs and tasks (jobs cut short go back in the queue) |
| `pgxpool` Max / Min connections | 25 / 5 |
| `pgxpool` MaxConnLifetime / MaxConnIdleTime | 5m / 1m |
| Max JSON request body | 64 KiB |
| Job timeout / lease / retention | 5 min / 10 min / 30 days |
| Outbound integration requests | 20s timeout |

## Database

The schema lives in `migrations/`:

| Migration | Contents |
| --------- | -------- |
| `001_baseline` | Everything before integrations: organisations, users, email codes, teams, cards, leads, files, branding, analytics, platform admin, feedback and usage snapshots. Earlier migrations 001–013 were squashed into it ([ADR 0005](../docs/adr/0005-squashed-baseline-migration.md)) |
| `002_jobs` | The background job queue |
| `003_integrations` | `integration_connections` and `integration_activity` |
| `004_identity` | `integration_tokens` (SCIM), `org_domains`, SSO and SCIM columns on `users` and `teams` |

The SQL files are commented and are the reference for each table. In outline:

- **Organisations and users.** Every user belongs to one `organizations` row with a `role`: `owner` (exactly one per organisation, enforced by a partial unique index), `admin` or `member`. Users have a case-insensitively unique username and email, `email_verified_at`, a `session_version` copied into each JWT (bumping it revokes every session), `must_change_password` while an organisation-chosen password is in use, an optional `storage_quota_bytes`, `suspended_at`, `created_by` and `last_login_at`. `password_hash` is `NULL` for people who only sign in with SSO. `full_name`, `external_id`, `external_username` and `provisioned_by` (`scim` or `sso`) record what an identity provider knows about them. `organizations.handle` is the organisation's part of every card link.
- **Email codes.** `email_codes` holds at most one live code per user and purpose (`verify_email`, `reset_password`, `change_email`): an HMAC of the code, an attempt counter and an expiry. For `change_email`, `email` holds the new address until it's confirmed.
- **Teams.** `teams` (names unique per organisation, ignoring case; optional `external_id` for SCIM groups) and `team_members` (`role` `lead` or `member`; a user can be in many teams).
- **Cards.** `profiles` belong to the organisation (`org_id`); `user_id` is the creator and `assigned_user_id` the one person working on it. Slugs are unique per organisation, case-insensitively. `data` is opaque JSONB owned by the frontend; the backend only reads the keys that name library files.
- **Leads** belong to a card and record `assigned_user_id` (copied from the card when they arrive, so they stay with that person after a reassignment) and `source` (`nfc`, `qr`, `link`). Phone numbers are stored as a digit-only dial code and number, both or neither.
- **Files.** `files` have an `area`: `personal` (counts toward the owner's quota), `org`, `shared` or `team`. They store a random 128-bit `public_id`, the bucket and object key, the sniffed type, a `purpose`, dimensions or page count, and an optional preview. `file_grants` and `file_team_grants` give extra access. `file_refs` records which card slot uses which file; it's rewritten whenever a card is saved (`files.SyncRefs`, in the same transaction). `user_storage` holds the organisation's bucket settings (on the owner's row), with encrypted keys.
- **Branding.** `organizations.logo_file`, `logo_policy` and the `signature` JSONB.
- **Analytics.** `card_events` (one row per thing a visitor did on a public card) and `analytics_salts` (one random salt per UTC day).
- **Platform admin.** `platform_admins`, `admin_audit_log`, `feedback`, `feedback_replies`, and daily `org_usage_snapshots` and `platform_usage_snapshots`.
- **Jobs.** `jobs`: kind, payload, organisation, dedupe key, status (`queued`, `running`, `done`, `dead`), schedule and retry state.
- **Integrations.** `integration_connections` (organisation-wide when `user_id` is `NULL`, personal otherwise; `config` JSONB, `secrets` sealed as one blob, `status`, failure tracking), `integration_activity` (90 days of log), `integration_tokens` (SHA-256 hashes of SCIM tokens) and `org_domains` (verification token; a domain is unique among verified domains).

**Scopes.** Reads that depend on who's asking take an `auth.Scope{OrgID, UserID, Admin}` (from `auth.ScopeOf(r)`). Admins see the whole organisation. Cards and leads go through `auth.VisibleTo(col, n)`: a member sees what's assigned to them, and a team lead also sees (and can edit) what's assigned to people in the teams they lead. Files go through `fileVisible` and `fileEditable` in `files/store.go`. Keep new queries going through a scope so a member can never widen what they see.

**Cascades.** Deleting an organisation deletes everything in it. Deleting a card deletes its leads. Users are never deleted with a bare `DELETE`: `DeleteOrgUser` first moves their personal files to the organisation and passes cards and files they created to the owner.

### Adding a migration

Add the next numbered pair next to the existing files (the next number is 005):

```
migrations/005_<description>.up.sql
migrations/005_<description>.down.sql
```

Never edit a migration that has been merged. The Docker entrypoint runs `migrate ... up` on every container start; locally, run the same command by hand.

A database created before the squash (version 13 in `schema_migrations`) can't be migrated and has to be reset; see [Resetting the database](../docs/operations/resetting-the-database.md).

### Store errors

`database.MapError` turns driver errors into two sentinel errors, so handlers never see Postgres codes:

| Postgres condition | Error | Typical HTTP status |
| ------------------ | ----- | ------------------- |
| `pgx.ErrNoRows` | `database.ErrNotFound` | 404 |
| `23505` unique_violation | `database.ErrConflict` (wraps the driver error, so `database.IsConstraint` can name the index) | 409 |
| `23503` foreign_key_violation | `database.ErrNotFound` | 404 (e.g. a lead for a missing card) |

Ownership checks happen in SQL, so another organisation's card looks exactly like a missing one, and the API never confirms that someone else's ID exists.

## Authentication & sessions

1. `POST /auth/register` or `POST /auth/login` (username or email) verifies the credentials and issues an HS256 JWT with `sub` (user ID), `sv` (`session_version`), `iat` and `exp`, valid for 24h. A SAML sign-in issues the same token.
2. The token is sent **only** as a cookie and never appears in a response body:

   | Attribute | Value |
   | --------- | ----- |
   | Name | `fronko_session` |
   | `HttpOnly` | yes |
   | `SameSite` | `Lax` |
   | `Secure` | `COOKIE_SECURE` (default on) |
   | `Path` / `Max-Age` | `/` / 86400 |

3. `auth.JWTMiddleware` validates the cookie (HS256 pinned, `exp` required, no `platform-admin` audience) and loads the user's session state in one query: `session_version`, verification, organisation, role, teams, suspension, `must_change_password` and whether the organisation is suspended. A suspended organisation is rejected first (`401 org_suspended` with the reason), then a stale `sv`, then a deleted or suspended account. The `Principal` goes into the request context.
4. `auth.RequireVerified` answers `403 email_unverified` until the email is verified, and `auth.RequirePasswordSet` answers `403 password_change_required` while the user still has an organisation-set temporary password. The route groups in `app.Routes` apply them (see [Architecture](../docs/architecture.md#route-groups)).
5. The SPA can't read the cookie, so it calls `GET /api/me/user` to learn who is signed in.
6. Changing or resetting a password, an admin setting a temporary password, and suspending a user all bump `session_version`, signing out every existing session.
7. `POST /auth/logout` only clears the cookie.

**Single sign-on.** When an organisation has an active SAML connection, people without a password (created by SSO or SCIM) can't use password sign-in or reset, and with "Require single sign-on" on nobody but the owner can. Those requests answer `403 {"code":"sso_required","sso_url":"/auth/sso/{handle}"}`; login only says so after the password checks out, so it reveals nothing. See [SAML single sign-on](../docs/integrations/saml.md).

### Email codes

Codes are 6 random digits, stored as an HMAC-SHA256 keyed with `JWT_SECRET` and bound to the user and purpose. They expire after 15 minutes and allow 5 guesses, each spent atomically **before** comparing (`UseEmailCodeAttempt`). A new code can be requested once every 60 seconds and replaces the old one. Reset codes are only sent to verified addresses.

**Changing a verified email** takes the current password, then a code sent to the new address; the old address keeps working until it's confirmed, and then gets a notice with the new one masked.

**Users created by an organisation** (`POST /api/org/users`) get a welcome email with their username and a temporary password; `must_change_password` locks the account to verification and choosing a new password until it's replaced. Users created over SCIM get the same email, or, when the organisation has SSO, an invitation to sign in with SSO and no password.

Mail is sent in the background (30s timeout, errors logged), so response timing doesn't reveal whether an address has an account.

## Platform admin

Platform admins run the server itself. They live in `platform_admins`, sign in at `POST /auth/admin/login` (the SPA page is `/admin/login`), and get their own cookie, `fronko_admin` (HttpOnly, `SameSite=Strict`, 8h). Their JWT carries `aud: "platform-admin"`; `ValidateJWT` rejects that audience and `ValidateAdminJWT` requires it, so an admin token never works as a user session or the reverse.

Accounts are made from the command line (`./fronko admin create --email …`); there's no sign-up page.

- **Privacy boundary.** Usage comes from one aggregate query (`orgUsageQuery` in `platformadmin/store.go`). It never selects card data, lead columns, file names, team names or member identities; the owner's email and feedback senders' emails are the only personal data the admin API returns.
- **Trends.** The `usage snapshot` task upserts today's row in both snapshot tables every hour.
- **Suspending an organisation** sets `suspended_at` and the reason and bumps every member's `session_version` in one transaction. Sign-in and every API call then answer `org_suspended`, public cards answer `410`, leads and files `404`, and SCIM and SSO refuse. The owner is emailed.
- **Audit log.** Admin sign-ins, suspensions, reinstatements, feedback replies and status changes each add an `admin_audit_log` row.

## Card analytics

**Recording.** The public card batches events and sends them with `navigator.sendBeacon` to `POST /api/profiles/{org}/{slug}/events`. `AnalyticsHandler.Collect` keeps only the types a browser may report (contact saves and sent forms are recorded by the server in the vCard and lead handlers, so they can't be faked), cleans each event, and hands the batch to `EventRecorder.Record`, which:

1. drops bots (empty or bot-like user agent),
2. drops requests carrying a session from the card's own organisation, so previews don't count,
3. hashes the visitor: `SHA-256(daily salt ‖ client IP ‖ user agent ‖ profile id)`,
4. inserts the batch in one statement, taking the organisation and current assignee from the card.

Raw IP addresses are never stored. Because each salt is deleted after a day, a hash can't be linked to an address, or to the same person on another day or card. `Collect` always answers `204`.

**Retention.** The hourly `analytics retention` task deletes events older than `ANALYTICS_RETENTION_DAYS` and every salt but today's and yesterday's.

**Reporting.** `/api/me/analytics/*` limits events exactly like leads (`eventScope`), groups visit-level numbers by session, and computes days and hours in Postgres with `AT TIME ZONE`. Legacy zone names Postgres doesn't know (`Asia/Calcutta`) fall back to a POSIX offset (`posixZone`).

## Integrations

`internal/integrations` connects organisations and people to outside services. Each provider is described by a manifest and registered in `providers/all.go`; the core handles connections, sealed secrets, OAuth, lead dispatch, retries and the activity log.

- **Lead sync.** `leads` publishes `leads.Created` inside the transaction that stores the lead. The integrations module subscribes and, in that same transaction, queues one `integrations.push_lead` job per active lead sync connection (the organisation's, plus the card holder's own). Workers deliver them with retries; three final failures in a row mark the connection `error`.
- **Calendar booking.** People connect any number of booking pages (personal or organisation). A card chooses one in its data (`booking_connection_id`). `cards` resolves it through `BookingFinder` (wired in `server.go`), which shows it only if it's enabled, active and belongs to the card's holder or the organisation, and otherwise shows none. Single-card responses also list `booking_options`; saving a page that isn't among them is refused unless the card already had it. Connections report `used_by_cards`.
- **SAML SSO** (`integrations/sso`) serves `/auth/sso/{handle}`, `/auth/sso/discover`, `/auth/saml/{id}/metadata` and `/auth/saml/{id}/acs`, the email domains API, and the policy `account` uses to block passwords.
- **SCIM** (`integrations/directory`) serves `/scim/v2/*`, authenticated with the directory connection's bearer token.

Guides: [overview](../docs/integrations/overview.md), [writing a provider](../docs/integrations/writing-a-provider.md), and one per provider in [`docs/integrations/`](../docs/integrations/).

## Security measures

| Threat | Mitigation | Where |
| ------ | ---------- | ----- |
| Token theft via XSS | JWT lives only in an HttpOnly cookie | `auth/cookie.go` |
| CSRF | `SameSite=Lax`, plus a same-origin check that rejects POST/PUT/PATCH/DELETE whose `Origin` host differs from `Host` unless listed in `CORS_ALLOWED_ORIGINS`. Only `External` routes (SCIM, the SAML ACS) skip it, and they never act on the session cookie alone | `platform/web/origin.go`, `app/routes.go` |
| Cross-origin reads | CORS headers only for origins in `CORS_ALLOWED_ORIGINS` (exact match) | `platform/web/cors.go` |
| A route mounted without auth by mistake | Signed-in routes must live under `/api/<area>/`, and the first route in an area mounts the whole area behind the session chain | `app/routes.go` |
| Admin and user sessions crossing over | Separate cookies, an `aud` claim each validator checks, separate tables, `SameSite=Strict` admin cookie | `auth/auth.go`, `platformadmin/middleware.go` |
| Admin data revealing organisations' content | Admin endpoints read only aggregates; an integration test checks no card data, leads or member names appear | `platformadmin/store.go` |
| JWT algorithm confusion | `jwt.WithValidMethods(["HS256"])` and `WithExpirationRequired()` | `auth/auth.go` |
| Weak signing key | Startup fails when `JWT_SECRET` is shorter than 16 chars | `platform/config/config.go` |
| Username enumeration via timing | Unknown usernames (and SSO-only accounts) still run a bcrypt compare against a dummy hash | `account/handler.go` |
| Account enumeration via password reset | Forgot-password always answers 204 and sends mail asynchronously | `account/account.go` |
| Guessing email codes | 5 attempts per code (counted atomically), 15-minute expiry, per-IP rate limit, 60s resend cooldown | `users/codes.go`, `users/store.go` |
| Codes leaking from the database | Only an HMAC of each code is stored | `auth/otp.go` |
| Stolen sessions surviving a password change | `session_version` in the JWT, checked on every request | `auth/session.go` |
| Email header injection | Recipients parsed with `net/mail`; CR/LF rejected | `platform/mail/mail.go` |
| Brute force, lead spam | Per-IP token buckets (see below) | `platform/ratelimit/` |
| Rate-limit bypass via spoofed headers | `X-Real-IP` only trusted when `TRUST_PROXY=true` | `platform/ratelimit/ratelimit.go` |
| bcrypt 72-byte truncation | Passwords limited to 8–72 bytes | `users/validate.go` |
| Oversized bodies | JSON bodies capped at 64 KiB, uploads at about 21.3 MiB, SAML responses at 1 MiB; nginx caps `/api/` at 25 MB | `platform/httpx/respond.go`, `files/handler.go`, `integrations/sso/handler.go` |
| Secrets leaking from the database | AES-256-GCM with `SECRETS_KEY`; the AAD binds each ciphertext to its owner (`storage:<user>:…`, `integration_connection:<id>`). Secrets are write-only in the API | `platform/secrets/`, `files/service.go`, `integrations/connections.go` |
| SSRF through user-supplied addresses (storage endpoints, webhooks, booking links, SAML metadata, OAuth) | https only, no literal private addresses, and a dialer hook that rejects private, loopback, link-local, CGNAT and metadata addresses **after DNS resolution**. Redirects aren't followed and proxy variables are ignored | `platform/netguard/`, `platform/storage/` |
| Forged webhook deliveries | Optional HMAC-SHA256 signature over timestamp and body (`X-Fronko-Signature`) | `integrations/providers/webhook/` |
| OAuth CSRF and code injection | HMAC-signed `state` bound to the user and connection, 10-minute expiry, nonce (and PKCE verifier) in a short-lived cookie | `integrations/oauth.go` |
| SAML response forgery or replay | Signed responses checked for audience, destination and time; SP-initiated responses must answer the request in this browser's signed cookie; IdP-initiated off by default; XML round-trip validation | `integrations/sso/` |
| Stolen SCIM tokens | Stored as SHA-256 only, shown once, revoked on rotation or when the connection is paused; the owner can't be deprovisioned | `integrations/linked.go`, `integrations/directory/` |
| Claiming someone else's email domain | DNS TXT proof, and a domain can be verified by one organisation only | `integrations/sso/domains.go` |
| Malicious uploads | Type sniffed from the bytes; only JPEG, PNG, WebP and PDF accepted | `files/handler.go` |
| Enumerating other people's files | 128-bit random public IDs; signed-in endpoints go through `fileVisible`/`fileEditable`; public cards only list files they reference | `files/` |
| IDOR on cards, leads and connections | Ownership enforced in SQL; foreign IDs return 404 | each module's `store.go`, `integrations/connections.go` |
| Leaking owner info publicly | Public lookups return `PublicProfile`, without `user_id` or timestamps | `cards/card.go` |
| SQL injection | All queries are parameterised | every `store.go` |
| Tracking card visitors | No cookies, no stored IPs, a daily salt deleted after a day, aggregates only | `analytics/` |
| Inflated analytics | Bots and the card's own organisation skipped; saves and forms only recorded server-side; events validated, batches capped at 20 and rate limited | `analytics/handler.go`, `analytics/recorder.go` |

### Rate limits

Per client IP, in memory:

| Limiter | Applies to | Burst | Refill | Defined in |
| ------- | ---------- | ----- | ------ | ---------- |
| auth | Login, register, forgot/reset password, email verify/resend/change, password change, platform admin login, SSO discover, the SAML ACS, domain verification and SCIM token rotation (one shared bucket) | 10 | 1 per 10s | `server/server.go` |
| lead | `POST /api/profiles/{id}/leads` | 5 | 1 per 15s | `leads/module.go` |
| events | `POST /api/profiles/{org}/{slug}/events` | 30 | 1 per 2s | `analytics/module.go` |
| upload | `POST /api/me/files` | 10 | 1 per 6s | `files/module.go` |
| feedback | `POST /api/me/feedback` | 5 | 1 per 12 min | `feedback/module.go` |
| integration tests | `POST /api/integrations/connections/{id}/test` | 5 | 1 per 10s | `integrations/module.go` |
| SCIM | `/scim/v2/*` | 200 | 1 per 20ms | `integrations/directory/module.go` |

A rejected request gets `429 Too Many Requests` with `Retry-After` in seconds and doesn't use up a token. Buckets idle for 10 minutes are evicted.

## Error handling conventions

- Every error response is JSON: `{"error": "<message>"}`, sometimes with a machine-readable `code` (`email_unverified`, `sso_required` …) or, for integration settings, the `field` at fault. Messages are written for end users; the frontend shows them as they are.
- Unexpected errors are logged and returned as a generic `"internal error"` (`httpx.Internal`). Driver details never reach the client.
- Use `httpx.WriteJSON`, `httpx.WriteError`, `httpx.DecodeJSON` and `httpx.PathID` in handlers so responses stay consistent.
- SCIM endpoints answer SCIM error bodies (`urn:ietf:params:scim:api:messages:2.0:Error`) instead.

## Testing

```bash
make test               # unit tests, with the race detector
make test-integration   # database tests (needs a disposable database, see below)
make lint               # golangci-lint
```

**Unit tests** sit next to the code. They cover, among others: JWTs, sessions and email codes (`auth`), mail building and header injection (`platform/mail`), rate limiting, CORS and the same-origin check (`platform/web`, `platform/ratelimit`), sealing (`platform/secrets`), the SSRF guard (`platform/netguard`), storage validation, the event bus and job backoff, configuration parsing, uploads and file areas, vCards, lead validation, analytics (event cleaning, visitor hashes, filters, time zones), integration settings validation, every provider (webhook signatures and retries, the presets' host checks, HubSpot's OAuth and contacts API, calendar links), SCIM parsing and filters, and SAML metadata and attribute handling.

**Integration tests** (`internal/integrationtest/`) sit behind the `integration` build tag and run every module's store against a real PostgreSQL: users and codes, organisations, cards and leads with scopes, files, admin aggregates (and that nothing private appears in them), analytics, the job queue (claiming, retries, reclaiming, dedupe), integrations (connections, dispatch inside the lead transaction, activity) and identity (SCIM provisioning, domains, SSO users). They read **`TEST_DATABASE_URL`**, not `DATABASE_URL`, because they `TRUNCATE` every table, and are skipped when it's unset.

A throwaway database is the easiest option:

```bash
docker run -d --rm --name fronko-test-db -e POSTGRES_USER=test -e POSTGRES_PASSWORD=test \
  -e POSTGRES_DB=fronko_test -p 55999:5432 postgres:18
export TEST_DATABASE_URL='postgres://test:test@localhost:55999/fronko_test?sslmode=disable'
migrate -path migrations -database "$TEST_DATABASE_URL" up
make test-integration
docker stop fronko-test-db
```

CI (`.github/workflows/ci.yml`) runs `gofmt`, `go vet`, `golangci-lint`, the unit tests, and the integration tests against a Postgres service.

## Docker image

`Dockerfile` is a two-stage build:

1. **Builder** (`golang:1.27-alpine`): installs the pinned `migrate` CLI (v4.19.1) and builds a static binary (`CGO_ENABLED=0`).
2. **Runtime** (`alpine:3`): ships the binary, the `migrate` CLI and `migrations/`, and runs as an unprivileged user (`uid 10001`).

`entrypoint.sh` requires `DATABASE_URL`, runs `migrate up`, then `exec`s the server, so every container start brings the schema up to date before serving traffic.

```bash
docker build -t fronko-backend .
docker run --rm -p 8080:8080 \
  -e DATABASE_URL=... -e JWT_SECRET=... -e COOKIE_SECURE=false \
  fronko-backend
```

For the full stack, see `deploy/` and the [root README](../README.md).

## Known limitations

- **Rate limits are per process.** Several replicas each enforce their own budget; sharing them needs a store such as Redis. (The job queue and periodic tasks are already safe with several replicas.)
- **No per-session logout.** Logging out removes the cookie but doesn't invalidate that JWT; it stays valid until `exp` (up to 24h) unless the password is changed or reset.
- **An email change can't be undone from the old address.** Recovering a hijacked account needs the server operator.
- **`collect_leads` isn't enforced by the server.** The frontend hides the lead form when it's off, but the endpoint still accepts submissions.
- **`lead_count` is only filled in by `GET /api/me/profiles`.**
- **Anyone with a file's link can open it.** `/api/files/{id}` needs no sign-in; IDs are 128-bit random and only appear on cards that use them.
- **Uploads are buffered in memory**, up to 20 MB per request, so they can be type-checked first.
- **Deleting a file doesn't edit cards.** Cards that referenced it stop showing it.
- **Unique visitors are per day**, by design, and analytics are only as complete as the browser allows (ad blockers, beacons on close).
- **Admin sign-in has no second factor yet.**
- **Changing or losing `SECRETS_KEY` breaks saved secrets.** Storage keys and integration secrets have to be re-entered and SAML connections re-created. Rotation isn't automated yet.
- **Leads aren't replayed.** Leads that arrive while a lead sync connection is paused or in error aren't sent when it comes back. Lead deliveries are at least once, so a receiver can see a rare duplicate.
- **SAML.** Single logout isn't supported, and each organisation has one SSO connection.
- **SCIM.** No bulk operations, sorting or ETags; roles aren't provisioned (everyone is a member).
