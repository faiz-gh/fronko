# Fronko API Reference

Every endpoint is served by the Go backend. In deployment, the browser reaches them through the frontend's nginx (or the Vite dev proxy) on the **same origin** as the SPA, so no CORS is configured.

- **Content type.** Requests and responses use `application/json`.
- **Auth.** Protected endpoints (`/api/me/*`) need the `fronko_session` cookie, which login or register sets. Browsers send it automatically. With `curl`, use a cookie jar (`-c`/`-b`).
- **Errors.** Every error body has the shape `{"error": "<message>"}`, and the message is safe to show to users.
- **Timestamps.** RFC 3339 strings, for example `"2026-10-03T12:34:56.789Z"`.
- **IDs.** 64-bit integers.

## Endpoint summary

| Method | Path | Auth | Rate limited | Description |
| ------ | ---- | :--: | :----------: | ----------- |
| `GET`    | [`/health`](#get-health) | | | Liveness check |
| `POST`   | [`/auth/register`](#post-authregister) | | ✅ auth | Create an account and sign in |
| `POST`   | [`/auth/login`](#post-authlogin) | | ✅ auth | Sign in |
| `POST`   | [`/auth/logout`](#post-authlogout) | | | Clear the session cookie |
| `GET`    | [`/api/me/user`](#get-apimeuser) | ✅ | | Current user |
| `GET`    | [`/api/me/profiles`](#get-apimeprofiles) | ✅ | | List my profiles |
| `POST`   | [`/api/me/profiles`](#post-apimeprofiles) | ✅ | | Create a profile |
| `GET`    | [`/api/me/profiles/{id}`](#get-apimeprofilesid) | ✅ | | Get one of my profiles |
| `PUT`    | [`/api/me/profiles/{id}`](#put-apimeprofilesid) | ✅ | | Update a profile |
| `DELETE` | [`/api/me/profiles/{id}`](#delete-apimeprofilesid) | ✅ | | Delete a profile and its leads |
| `GET`    | [`/api/me/profiles/{id}/leads`](#get-apimeprofilesidleads) | ✅ | | List a profile's leads (unpaginated) |
| `GET`    | [`/api/me/leads`](#get-apimeleads) | ✅ | | Paginated leads across all my profiles, with filters |
| `GET`    | [`/api/me/storage`](#get-apimestorage) | ✅ | | My storage connection (never returns keys) |
| `PUT`    | [`/api/me/storage`](#put-apimestorage) | ✅ | | Check, then save storage settings |
| `POST`   | [`/api/me/storage/test`](#post-apimestoragetest) | ✅ | | Check storage settings without saving |
| `DELETE` | [`/api/me/storage`](#delete-apimestorage) | ✅ | | Forget my storage keys |
| `GET`    | [`/api/me/files`](#get-apimefiles) | ✅ | | Paginated file library |
| `POST`   | [`/api/me/files`](#post-apimefiles) | ✅ | ✅ upload | Upload a photo or PDF |
| `PATCH`  | [`/api/me/files/{id}`](#patch-apimefilesid) | ✅ | | Rename a file |
| `DELETE` | [`/api/me/files/{id}`](#delete-apimefilesid) | ✅ | | Delete a file from the bucket and library |
| `GET`    | [`/api/profiles/{slug}`](#get-apiprofilesslug) | | | Public profile by slug |
| `GET`    | [`/api/files/{id}`](#get-apifilesid) | | | Redirect to a file (short-lived signed URL) |
| `POST`   | [`/api/profiles/{id}/leads`](#post-apiprofilesidleads) | | ✅ lead | Submit a lead to a profile |

## Cross-cutting behaviour

| Status | When |
| ------ | ---- |
| `400` | Body isn't valid JSON (`"invalid request body"`), the body is over 64 KiB, a path ID isn't an integer (`"invalid profile ID"`), or validation failed |
| `401` | `/api/me/*` without a cookie (`"not signed in"`) or with an invalid or expired token (`"session expired, please sign in again"`) |
| `403` | A `POST`/`PUT`/`DELETE` whose `Origin` header names a different host (`"cross-origin request rejected"`). Requests without `Origin`, such as curl, are allowed |
| `429` | Rate limit exceeded (`"too many requests, please try again shortly"`), with a `Retry-After: <seconds>` header |
| `500` | Unexpected server error (`"internal error"` or a specific "failed to …" message) |

**Rate limits** are per client IP:

- **auth**: burst of 10, then 1 request per 10s. `login` and `register` share one bucket.
- **lead**: burst of 5, then 1 request per 15s.

## Objects

### User

```json
{ "id": 1, "username": "faiz" }
```

### Profile (owner view)

```json
{
  "id": 42,
  "user_id": 1,
  "slug": "faiz",
  "data": { "name": "Faiz", "title": "Engineer", "links": [] },
  "created_at": "2026-10-03T12:00:00Z",
  "updated_at": "2026-10-03T12:00:00Z",
  "lead_count": 3
}
```

`lead_count` is only filled in by `GET /api/me/profiles`. Every other endpoint returns `0`.

### PublicProfile (visitor view)

```json
{
  "id": 42,
  "slug": "faiz",
  "data": { "name": "Faiz", "avatar_file": "90meIH31WrEH0xe9ymyzDA", "documents": [] },
  "files": [{ "id": "90meIH31WrEH0xe9ymyzDA", "kind": "image", "name": "me.png", "size_bytes": 48211 }]
}
```

The owner's ID and the timestamps are left out on purpose. `files` lists only the library files the card itself references (`data.avatar_file` and `data.documents[].file`) that the owner still has. Nothing else from the owner's library is revealed.

### File

```json
{
  "id": "r749f-c4WN5WAPhaxOzysg",
  "kind": "pdf",
  "content_type": "application/pdf",
  "size_bytes": 411,
  "name": "partnership.pdf",
  "title": "Partnership brochure",
  "created_at": "2026-10-03T20:53:27Z"
}
```

`id` is a random 22-character public ID (128 bits), the only file identifier the API exposes. `kind` is `image` or `pdf`.

### Lead

```json
{
  "id": 7,
  "profile_id": 42,
  "name": "Jane Doe",
  "email": "jane@example.com",
  "notes": "Great talk!",
  "created_at": "2026-10-03T12:10:00Z"
}
```

### `data`

`data` is a free-form JSON **object** that the frontend owns. The backend only checks that it is an object, and stores `{}` when it's missing or `null`. For the shape the web app uses, see [`CardData` in the frontend docs](../frontend/README.md#card-data-model).

---

## Health

### `GET /health`

Returns `200` with the plain-text body `OK`. It doesn't touch the database.

---

## Auth

### `POST /auth/register`

Creates an account and signs it in.

**Request**

```json
{ "username": "faiz", "password": "correct horse battery" }
```

| Field | Rules |
| ----- | ----- |
| `username` | Trimmed. 3–32 chars of `[a-zA-Z0-9_.-]`. Unique **case-insensitively** |
| `password` | 8–72 bytes (bcrypt's limit) |

**Responses**

| Status | Body |
| ------ | ---- |
| `201` | [`User`](#user). Sets the `fronko_session` cookie |
| `400` | `"username must be 3-32 characters: letters, numbers, '.', '_' or '-'"` or `"password must be 8-72 characters"` |
| `409` | `"username already taken"` |

### `POST /auth/login`

**Request**

```json
{ "username": "faiz", "password": "correct horse battery" }
```

The username match is case-insensitive.

**Responses**

| Status | Body |
| ------ | ---- |
| `200` | [`User`](#user). Sets the `fronko_session` cookie |
| `401` | `"invalid username or password"`. The same message and similar timing whether or not the user exists |

### `POST /auth/logout`

Clears the session cookie. It works without a session. The JWT isn't revoked server-side.

**Response:** `204 No Content`

### `GET /api/me/user` 🔒

Returns the signed-in user. The SPA calls this at startup to restore the session, because it can't read the HttpOnly cookie.

| Status | Body |
| ------ | ---- |
| `200` | [`User`](#user) |
| `401` | Not signed in or session expired. If the token is valid but the account was deleted, the cookie is cleared and the body is `"account no longer exists"` |

---

## Profiles (owner) 🔒

Every endpoint in this section needs a session, and the caller can only act on profiles they own. **Another user's profile returns `404`, the same as a missing one.**

### Slug rules

These rules apply to both create and update. The slug is trimmed and lowercased, must be **3–48 characters**, and must match `^[a-z0-9]+(?:-[a-z0-9]+)*$`: lowercase letters and digits separated by single hyphens, with no leading or trailing hyphen. Slugs are globally unique, case-insensitively.

Breaking either rule returns `400` with `"slug must be 3-48 characters: lowercase letters, numbers and hyphens"`. If `data` is present but isn't a JSON object, the response is `400` with `"data must be a JSON object"`.

### `GET /api/me/profiles`

Lists the caller's profiles, newest first, each with its `lead_count`.

**Response:** `200`, an array of [`Profile`](#profile-owner-view). It's `[]` when there are none, never `null`.

### `POST /api/me/profiles`

**Request**

```json
{ "slug": "faiz", "data": { "name": "Faiz" } }
```

`data` is optional and defaults to `{}`.

| Status | Body |
| ------ | ---- |
| `201` | The created [`Profile`](#profile-owner-view) |
| `400` | Slug or data validation failed |
| `409` | `"that slug is already taken"` |

### `GET /api/me/profiles/{id}`

| Status | Body |
| ------ | ---- |
| `200` | [`Profile`](#profile-owner-view) |
| `404` | `"profile not found"` |

### `PUT /api/me/profiles/{id}`

Replaces both `slug` and `data`. This is a full update, not a patch: send the complete `data` object.

**Request:** same shape as [create](#post-apimeprofiles).

| Status | Body |
| ------ | ---- |
| `200` | The updated [`Profile`](#profile-owner-view) |
| `400` | Validation failed |
| `404` | `"profile not found"` |
| `409` | `"that slug is already taken"` |

Changing a slug breaks every NFC card or QR code that points at the old URL.

### `DELETE /api/me/profiles/{id}`

Deletes the profile and, through the `ON DELETE CASCADE` foreign key, all of its leads.

| Status | Body |
| ------ | ---- |
| `204` | No content |
| `404` | `"profile not found"` |

### `GET /api/me/profiles/{id}/leads`

Lists **all** of a profile's leads, newest first, with no pagination. The web app uses [`GET /api/me/leads`](#get-apimeleads) instead. This endpoint remains for simple integrations.

| Status | Body |
| ------ | ---- |
| `200` | Array of [`Lead`](#lead) (`[]` when empty) |
| `404` | `"profile not found"` |

### `GET /api/me/leads`

Returns one page of leads across all of the caller's profiles, newest first (ties broken by newest ID). The web app's Leads page, the editor's Leads tab and the overview all use it.

**Query parameters** (all optional)

| Param | Default | Rules |
| ----- | ------- | ----- |
| `profile_id` | none | Only this profile's leads. Another user's profile ID returns an empty page, never their leads |
| `q` | none | Trimmed, at most 100 chars. Case-insensitive substring match on name, email or notes. `%` and `_` match literally |
| `since` | none | RFC 3339 timestamp (`2026-10-01T00:00:00Z`). Only leads received at or after it |
| `page` | `1` | 1-based |
| `page_size` | `25` | 1–100 |

**Response:** `200`

```json
{
  "leads": [ { "id": 7, "profile_id": 42, "name": "Jane Doe", "email": "jane@example.com", "notes": "", "created_at": "2026-10-03T12:10:00Z" } ],
  "total": 67,
  "page": 1,
  "page_size": 25
}
```

`total` counts every lead matching the filters, not just this page, so the page count is `ceil(total / page_size)`. A page past the end returns `"leads": []` with the real `total`. To fetch everything (e.g. for an export), request `page_size=100` and walk pages until you have `total` leads.

| Status | Body |
| ------ | ---- |
| `400` | `"invalid page"`, `"invalid page_size"`, `"invalid profile_id"`, `"search is too long"` or `"since must be an RFC 3339 timestamp"` |

---

## Storage 🔒

Each user connects **their own** S3-compatible bucket (Cloudflare R2, Backblaze B2, AWS S3, MinIO, ...). Uploaded photos and PDFs live there; the bucket can stay private.

- Every endpoint here returns **503** `"file storage is not enabled on this server"` when the server has no `SECRETS_KEY`.
- Keys are **write-only**. They're encrypted before storage and never returned. `access_key_hint` (the last 4 characters of the key ID) is the only trace you get back.

### `GET /api/me/storage`

```json
{
  "enabled": true,
  "configured": true,
  "provider": "r2",
  "endpoint": "https://abc123.r2.cloudflarestorage.com",
  "region": "auto",
  "bucket": "my-fronko-files",
  "path_style": false,
  "access_key_hint": "9F2A",
  "verified_at": "2026-10-03T20:53:03Z",
  "file_count": 5
}
```

When nothing is connected, it returns `{"enabled": true, "configured": false, "path_style": false, "file_count": 0}`.

### `PUT /api/me/storage`

**Request**

```json
{
  "provider": "r2",
  "endpoint": "https://abc123.r2.cloudflarestorage.com",
  "region": "auto",
  "bucket": "my-fronko-files",
  "path_style": false,
  "access_key_id": "…",
  "secret_access_key": "…"
}
```

| Field | Rules |
| ----- | ----- |
| `provider` | `r2`, `b2`, `s3`, `minio` or `other` |
| `endpoint` | `https://host[:port]` with no path, query or credentials. Private, loopback and link-local addresses are refused, both as literals and when the name resolves to one at connect time (unless the server sets `STORAGE_ALLOW_PRIVATE_ENDPOINTS`) |
| `bucket` | 3–63 chars: lowercase letters, digits, `.`, `-` |
| `region` | `[a-z0-9-]`. Defaults to `auto` for R2 and `us-east-1` otherwise |
| `access_key_id`, `secret_access_key` | Up to 256 chars. **May be blank when updating** to keep the saved values |

Before saving, the server **checks the settings live** by writing and deleting a small `.fronko-probe` object. It calls HeadBucket first for a clear "bucket not found", but a 403 there is ignored: keys limited to objects (R2 "Object Read & Write", AWS policies without `s3:ListBucket`) are enough. Settings that fail are not saved.

| Status | Body |
| ------ | ---- |
| `200` | The new status (as in `GET`) |
| `400` | Validation failure, or `"couldn't use this bucket: <reason>"`. The reason is summarised, e.g. access denied, bucket not found, wrong region or unreachable endpoint |

### `POST /api/me/storage/test`

Runs the same checks as `PUT` without saving. Returns `200 {"ok": true}` or `400` with the reason.

### `DELETE /api/me/storage`

Deletes the saved keys (`204`). Nothing is removed from the bucket, but files can't be shown until storage is connected again.

---

## Files 🔒

### `GET /api/me/files`

Paginated library, newest first. Query: `kind` (`image` or `pdf`, optional), `page` (1-based) and `page_size` (default 24, max 100).

**Response:** `200 {"files": [File], "total": 5, "page": 1, "page_size": 24}`

### `POST /api/me/files`

`multipart/form-data` with a `file` part and an optional `title` part (up to 120 chars). Rate limited per IP: burst of 10, then 1 every 6s.

The type is decided by **sniffing the bytes**, never from the file name or the client's `Content-Type`:

| Accepted | Max size |
| -------- | -------- |
| JPEG, PNG, WebP images | 5 MB |
| PDF | 20 MB |

The object is written to the user's bucket as `fronko/<user_id>/<file id>.<ext>`.

| Status | Body |
| ------ | ---- |
| `201` | [`File`](#file) |
| `409` | `"connect your storage in Settings first"` |
| `413` | Over the size limit. Requests over 25 MB are stopped by nginx first |
| `415` | `"only JPEG, PNG or WebP images and PDF files can be uploaded"` |
| `502` | `"couldn't save to your storage: <reason>"` |

### `PATCH /api/me/files/{id}`

`{"title": "Spring price list"}`. Returns the updated [`File`](#file), or `404` for a file you don't own.

### `DELETE /api/me/files/{id}`

Deletes the object from the bucket, then the library entry (`204`).
- If the bucket refuses the delete, you get `502` and the entry is kept, so you can retry.
- If storage was disconnected, only the entry is removed.
- Cards that used the file stop showing it.

---

## Public

### `GET /api/profiles/{slug}`

Looks up a profile for the public card page (`/p/{slug}`). The slug match is case-insensitive.

| Status | Body |
| ------ | ---- |
| `200` | [`PublicProfile`](#publicprofile-visitor-view) |
| `404` | `"profile not found"` |

### `GET /api/files/{id}`

Serves a library file to anyone who has its ID: on cards, the photo and brochures. It answers **`302`** to a presigned URL for the object in the owner's bucket.
- The signed URL is valid for 15 minutes.
- The response has `Content-Type` and `Content-Disposition: inline; filename=…` set.
- The redirect is sent with `Cache-Control: private, max-age=300` and `Referrer-Policy: no-referrer`.

Unknown IDs, deleted files and files whose owner disconnected storage all return `404`.

### `POST /api/profiles/{id}/leads`

A visitor shares their details with the profile owner. The path takes the numeric profile **ID**, which the frontend gets from `GET /api/profiles/{slug}`.

**Request**

```json
{ "name": "Jane Doe", "email": "jane@example.com", "notes": "Great talk!" }
```

| Field | Rules (all values are trimmed first) |
| ----- | ------------------------------------ |
| `name` | Required, 1–120 bytes |
| `email` | Required, at most 254 bytes. Must be a bare address: `Jane <jane@x.com>` is rejected |
| `notes` | Optional, at most 2000 bytes |

| Status | Body |
| ------ | ---- |
| `201` | `{"status": "ok"}` |
| `400` | `"please enter your name"`, `"please enter a valid email address"` or `"message is too long"` |
| `404` | `"profile not found"` |
| `429` | Rate limited |

The server accepts leads even when the card's `data.collect_leads` is `false`. That flag only hides the form in the UI.

---

## curl walkthrough

```bash
BASE=http://localhost:8080
JAR=$(mktemp)

# Register (sets cookie)
curl -s -c $JAR -X POST $BASE/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"demo","password":"password123"}'

# Create a profile
curl -s -b $JAR -X POST $BASE/api/me/profiles \
  -H 'Content-Type: application/json' \
  -d '{"slug":"demo-card","data":{"name":"Demo User","collect_leads":true}}'

# Public lookup
curl -s $BASE/api/profiles/demo-card

# Submit a lead (use the id from the previous response)
curl -s -X POST $BASE/api/profiles/1/leads \
  -H 'Content-Type: application/json' \
  -d '{"name":"Jane","email":"jane@example.com","notes":"Hi!"}'

# Read leads as the owner, 10 per page
curl -s -b $JAR "$BASE/api/me/leads?profile_id=1&page_size=10"
```

With `COOKIE_SECURE=true` (the default), curl still stores the cookie but only sends it back over HTTPS. Run the backend with `COOKIE_SECURE=false` when testing locally over HTTP.
