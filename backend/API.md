# Fronko API Reference

Every endpoint is served by the Go backend. By default the browser reaches them through the frontend's nginx (or the Vite dev proxy) on the **same origin** as the SPA. If the API is served on its own domain, list the frontend's origin in `CORS_ALLOWED_ORIGINS`; those origins get credentialed CORS responses (see the [configuration reference](README.md#configuration)).

- **Content type.** Requests and responses use `application/json`.
- **Auth.** Protected endpoints (`/api/me/*` and `/api/org/*`) need the `fronko_session` cookie, which login or register sets. Browsers send it automatically. With `curl`, use a cookie jar (`-c`/`-b`).
- **Organisations and roles.** Every account belongs to an organisation. Registering creates one, with the new account as its **owner**. The owner and **admins** see and manage everything in the organisation. **Members**, whom the organisation creates, see only the cards assigned to them, the leads those cards collected while they held them, their own files, the shared area and files granted to them. The [Organisation](#organisation--) endpoints manage users.
- **Errors.** Every error body has the shape `{"error": "<message>"}`, and the message is safe to show to users.
- **Timestamps.** RFC 3339 strings, for example `"2026-10-03T12:34:56.789Z"`.
- **IDs.** 64-bit integers.

## Endpoint summary

| Method | Path | Auth | Rate limited | Description |
| ------ | ---- | :--: | :----------: | ----------- |
| `GET`    | [`/health`](#get-health) | | | Liveness check |
| `POST`   | [`/auth/register`](#post-authregister) | | ✅ auth | Create an organisation and its owner account, email a verification code, and sign in |
| `POST`   | [`/auth/login`](#post-authlogin) | | ✅ auth | Sign in with username or email |
| `POST`   | [`/auth/logout`](#post-authlogout) | | | Clear the session cookie |
| `POST`   | [`/auth/password/forgot`](#post-authpasswordforgot) | | ✅ auth | Email a password-reset code |
| `POST`   | [`/auth/password/reset`](#post-authpasswordreset) | | ✅ auth | Set a new password with the code |
| `GET`    | [`/api/me/user`](#get-apimeuser) | ✅ ✉️ | | Current user |
| `PUT`    | [`/api/me/email`](#put-apimeemail) | ✅ ✉️ | ✅ auth | Add or correct an unverified email |
| `POST`   | [`/api/me/email/verify`](#post-apimeemailverify) | ✅ ✉️ | ✅ auth | Verify the email with its code |
| `POST`   | [`/api/me/email/resend`](#post-apimeemailresend) | ✅ ✉️ | ✅ auth | Send a new verification code |
| `POST`   | [`/api/me/email/change`](#post-apimeemailchange) | ✅ | ✅ auth | Start changing a verified email (password + code to the new address) |
| `POST`   | [`/api/me/email/change/confirm`](#post-apimeemailchangeconfirm) | ✅ | ✅ auth | Confirm the new email with its code |
| `PUT`    | [`/api/me/password`](#put-apimepassword) | ✅ 🔑 | ✅ auth | Change password (or replace a temporary one); signs out other sessions |
| `GET`    | [`/api/me/profiles`](#get-apimeprofiles) | ✅ | | Cards I can see |
| `POST`   | [`/api/me/profiles`](#post-apimeprofiles) | 🛡️ | | Create a card, optionally assigned to a user |
| `GET`    | [`/api/me/profiles/{id}`](#get-apimeprofilesid) | ✅ | | Get a card |
| `PUT`    | [`/api/me/profiles/{id}`](#put-apimeprofilesid) | ✅ | | Update a card (members can't change the slug) |
| `DELETE` | [`/api/me/profiles/{id}`](#delete-apimeprofilesid) | 🛡️ | | Delete a card and its leads |
| `GET`    | [`/api/me/profiles/{id}/leads`](#get-apimeprofilesidleads) | ✅ | | List a card's leads (unpaginated) |
| `GET`    | [`/api/me/leads`](#get-apimeleads) | ✅ | | Paginated leads I can see, with filters |
| `GET`    | [`/api/me/storage`](#get-apimestorage) | ✅ | | Whether uploads work, my usage, and (owner) the bucket settings |
| `PUT`    | [`/api/me/storage`](#put-apimestorage) | 👑 | | Check, then save storage settings |
| `POST`   | [`/api/me/storage/test`](#post-apimestoragetest) | 👑 | | Check storage settings without saving |
| `DELETE` | [`/api/me/storage`](#delete-apimestorage) | 👑 | | Forget the storage keys |
| `GET`    | [`/api/me/files`](#get-apimefiles) | ✅ | | Paginated files I can see, by area |
| `POST`   | [`/api/me/files`](#post-apimefiles) | ✅ | ✅ upload | Upload a photo or PDF |
| `PATCH`  | [`/api/me/files/{id}`](#patch-apimefilesid) | ✅ | | Rename a file |
| `DELETE` | [`/api/me/files/{id}`](#delete-apimefilesid) | ✅ | | Delete a file from the bucket and library |
| `GET`    | [`/api/org`](#get-apiorg) | 🛡️ | | The organisation |
| `PUT`    | [`/api/org`](#put-apiorg) | 👑 | | Rename it, set the default storage limit |
| `GET`    | [`/api/org/users`](#get-apiorgusers) | 🛡️ | | Everyone in the organisation, with totals |
| `POST`   | [`/api/org/users`](#post-apiorgusers) | 🛡️ | ✅ auth | Create a user with a temporary password |
| `GET`    | [`/api/org/users/{id}`](#get-apiorgusersid) | 🛡️ | | One user |
| `PATCH`  | [`/api/org/users/{id}`](#patch-apiorgusersid) | 🛡️ | | Correct an unverified email, set the storage limit, role or suspension |
| `POST`   | [`/api/org/users/{id}/password`](#post-apiorgusersidpassword) | 🛡️ | ✅ auth | Give a user a new temporary password |
| `DELETE` | [`/api/org/users/{id}`](#delete-apiorgusersid) | 🛡️ | | Delete a user, keeping their cards, files and leads |
| `PUT`    | [`/api/org/profiles/{id}/assignee`](#put-apiorgprofilesidassignee) | 🛡️ | | Assign a card to a user, or back to the organisation |
| `GET`    | [`/api/org/files/{id}/grants`](#get-apiorgfilesidgrants) | 🛡️ | | Who a file has been granted to |
| `PUT`    | [`/api/org/files/{id}/grants`](#put-apiorgfilesidgrants) | 🛡️ | | Replace who a file is granted to |
| `GET`    | [`/api/profiles/{slug}`](#get-apiprofilesslug) | | | Public profile by slug |
| `GET`    | [`/api/files/{id}`](#get-apifilesid) | | | Redirect to a file (short-lived signed URL) |
| `POST`   | [`/api/profiles/{id}/leads`](#post-apiprofilesidleads) | | ✅ lead | Submit a lead to a profile |

✉️ = works before the email is verified. 🔑 = works while the user still has a temporary password. Every other signed-in route needs a verified email and a password the user chose. 🛡️ = owner and admins only. 👑 = owner only. Others get `403`.

## Cross-cutting behaviour

| Status | When |
| ------ | ---- |
| `400` | Body isn't valid JSON (`"invalid request body"`), the body is over 64 KiB, a path ID isn't an integer (`"invalid profile ID"`), or validation failed |
| `401` | `/api/me/*` without a cookie (`"not signed in"`), with an invalid or expired token, or with a token from before a password change or reset (`"session expired, please sign in again"`) |
| `401` | A suspended account: `{"error":"this account is suspended; contact your organisation","code":"account_suspended"}`. Suspending also ends existing sessions, which then get `"session expired, please sign in again"` |
| `403` | Signed-in routes (except the ✉️ routes) while the email isn't verified: `{"error":"verify your email to continue","code":"email_unverified"}` |
| `403` | Signed-in routes (except ✉️ and 🔑) while the user still has the temporary password their organisation set: `{"error":"choose a new password to continue","code":"password_change_required"}` |
| `403` | A 🛡️ or 👑 route called by someone without that role: `"only your organisation's admins can do this"` or `"only your organisation's owner can do this"` |
| `403` | A `POST`/`PUT`/`DELETE` whose `Origin` header names a different host (`"cross-origin request rejected"`). Requests without `Origin`, such as curl, are allowed |
| `429` | Rate limit exceeded (`"too many requests, please try again shortly"`), with a `Retry-After: <seconds>` header |
| `500` | Unexpected server error (`"internal error"` or a specific "failed to …" message) |

**Rate limits** are per client IP:

- **auth**: burst of 10, then 1 request per 10s. Every route marked "auth" shares one bucket.
- **lead**: burst of 5, then 1 request per 15s.

## Objects

### User

```json
{
  "id": 1,
  "username": "faiz",
  "email": "faiz@example.com",
  "email_verified": true,
  "role": "owner",
  "org_name": "Acme",
  "must_change_password": false
}
```

`email` is `null` only for accounts created before emails were required; they must add and verify one before using the app. `role` is `owner`, `admin` or `member`. `must_change_password` is `true` while the user still has a temporary password their organisation set.

**Email codes.** Verification and password-reset codes are 6 digits, valid for 15 minutes, and allow 5 guesses, after which a new code is needed. A new code can be requested once every 60 seconds; requesting one replaces the previous code. Codes are stored only as an HMAC.

### Profile (owner view)

```json
{
  "id": 42,
  "user_id": 1,
  "assigned_user": { "id": 24, "username": "jane" },
  "slug": "faiz",
  "data": { "name": "Faiz", "title": "Engineer", "links": [] },
  "created_at": "2026-10-03T12:00:00Z",
  "updated_at": "2026-10-03T12:00:00Z",
  "lead_count": 3
}
```

`user_id` is the account that created the card. `assigned_user` is the one user who works on it, or `null` when the organisation holds it. `lead_count` counts every lead for admins, and only the member's own leads for members.

### PublicProfile (visitor view)

```json
{
  "id": 42,
  "slug": "faiz",
  "data": { "name": "Faiz", "avatar_file": "90meIH31WrEH0xe9ymyzDA", "documents": [] },
  "files": [{ "id": "90meIH31WrEH0xe9ymyzDA", "kind": "image", "name": "me.png", "size_bytes": 48211 }]
}
```

The owner's ID and the timestamps are left out on purpose. `files` lists only the library files the card itself references (`data.avatar_file`, `data.cover_file` and `data.documents[].file`) that the card's organisation still has. Nothing else from the organisation's files is revealed.

### File

```json
{
  "id": "r749f-c4WN5WAPhaxOzysg",
  "area": "personal",
  "owner": { "id": 24, "username": "jane" },
  "kind": "pdf",
  "content_type": "application/pdf",
  "size_bytes": 411,
  "name": "partnership.pdf",
  "title": "Partnership brochure",
  "created_at": "2026-10-03T20:53:27Z"
}
```

`id` is a random 22-character public ID (128 bits), the only file identifier the API exposes. `kind` is `image` or `pdf`.

`area` is one of:

- `personal`: a user's own file. `owner` is that user. It counts toward their storage limit.
- `org`: the organisation's private file, visible to admins and to members it's been granted to. `owner` is whoever uploaded it.
- `shared`: visible to everyone in the organisation.

`former_owner` (only present when set) is the username of a deleted user whose personal file this was.

### Lead

```json
{
  "id": 7,
  "profile_id": 42,
  "name": "Jane Doe",
  "email": "jane@example.com",
  "phone_country_code": "+91",
  "phone_number": "9876543210",
  "notes": "Great talk!",
  "created_at": "2026-10-03T12:10:00Z",
  "assigned_user": { "id": 24, "username": "jane" }
}
```

`assigned_user` is who held the card when the lead arrived (`null`: the organisation). Leads stay with that person when the card is reassigned.

`phone_country_code` and `phone_number` are left out when the visitor didn't give a number. Both are digits only (the dial code keeps its `+`).

### `data`

`data` is a free-form JSON **object** that the frontend owns. The backend only checks that it is an object, and stores `{}` when it's missing or `null`. For the shape the web app uses, see [`CardData` in the frontend docs](../frontend/README.md#card-data-model).

---

## Health

### `GET /health`

Returns `200` with the plain-text body `OK`. It doesn't touch the database.

---

## Auth

### `POST /auth/register`

Creates an organisation with this account as its owner, emails a verification code, and signs it in. Until the email is verified, only the ✉️ routes work.

**Request**

```json
{ "username": "faiz", "email": "faiz@example.com", "password": "correct horse battery", "organization": "Acme" }
```

| Field | Rules |
| ----- | ----- |
| `username` | Trimmed. 3–32 chars of `[a-zA-Z0-9_.-]`. Unique **case-insensitively** |
| `email` | Trimmed and lower-cased. A bare address, at most 254 chars. Unique **case-insensitively** |
| `password` | 8–72 bytes (bcrypt's limit) |
| `organization` | Optional, at most 80 chars, no control characters. Defaults to the username; the owner can rename it later |

**Responses**

| Status | Body |
| ------ | ---- |
| `201` | [`User`](#user) with `email_verified: false`. Sets the `fronko_session` cookie |
| `400` | `"username must be 3-32 characters: letters, numbers, '.', '_' or '-'"`, `"enter a valid email address"` or `"password must be 8-72 characters"` |
| `409` | `"username already taken"` or `"an account with this email already exists"` |

### `POST /auth/login`

**Request**

```json
{ "username": "faiz", "password": "correct horse battery" }
```

`username` may also be the account's email (anything containing `@` is looked up as an email). Both matches are case-insensitive. Unverified accounts can sign in; check `email_verified` in the response.

**Responses**

| Status | Body |
| ------ | ---- |
| `200` | [`User`](#user). Sets the `fronko_session` cookie |
| `401` | `"invalid username or password"`. The same message and similar timing whether or not the user exists |
| `403` | `{"error":"this account is suspended; contact your organisation","code":"account_suspended"}`, only after a correct password |

The first sign-in of a user the organisation created emails them a verification code (subject to the 60-second resend cooldown).

### `POST /auth/logout`

Clears the session cookie. It works without a session. The JWT isn't revoked server-side.

**Response:** `204 No Content`

### `POST /auth/password/forgot`

```json
{ "email": "faiz@example.com" }
```

Emails a reset code if a **verified** account uses this address and the 60-second cooldown has passed. It always answers the same way, so it can't reveal which emails have accounts.

| Status | Body |
| ------ | ---- |
| `204` | Always, for any well-formed address |
| `400` | `"enter a valid email address"` |

### `POST /auth/password/reset`

```json
{ "email": "faiz@example.com", "code": "042917", "password": "new password" }
```

Sets the new password and **signs out every session** (including the one asking, if any). Sign in again afterwards.

| Status | Body |
| ------ | ---- |
| `204` | Password changed |
| `400` | `"invalid or expired code"` (wrong, expired, used up, or never issued; each wrong guess counts) or `"password must be 8-72 characters"` |

### `GET /api/me/user` 🔒 ✉️

Returns the signed-in user. The SPA calls this at startup to restore the session, because it can't read the HttpOnly cookie.

| Status | Body |
| ------ | ---- |
| `200` | [`User`](#user) |
| `401` | Not signed in, session expired, or the account no longer exists (`"account no longer exists"`) |

### `PUT /api/me/email` 🔒 ✉️

```json
{ "email": "faiz@example.com" }
```

Adds an email to an older account, or corrects a typo before verification, and sends a verification code to it. A new address always gets a fresh code; re-sending the same address respects the cooldown. A verified email is changed with [`POST /api/me/email/change`](#post-apimeemailchange) instead.

| Status | Body |
| ------ | ---- |
| `200` | [`User`](#user) |
| `400` | `"enter a valid email address"` |
| `409` | `"your email is already verified"` or `"an account with this email already exists"` |

### `POST /api/me/email/verify` 🔒 ✉️

```json
{ "code": "042917" }
```

| Status | Body |
| ------ | ---- |
| `200` | [`User`](#user) with `email_verified: true` (also when it already was) |
| `400` | `"invalid or expired code"` or `"add an email address first"` |

### `POST /api/me/email/resend` 🔒 ✉️

| Status | Body |
| ------ | ---- |
| `204` | A new code was sent |
| `400` | `"add an email address first"` |
| `409` | `"your email is already verified"` |
| `429` | `"please wait before requesting another code"`, with `Retry-After` |

### `POST /api/me/email/change` 🔒

```json
{ "email": "new@example.com", "password": "current password" }
```

Checks the password and sends a code to the **new** address. The current email stays in force (for sign-in and password resets) until the code is confirmed. Asking for a different address replaces the pending one; asking again for the same address inside the 60-second cooldown answers 429.

| Status | Body |
| ------ | ---- |
| `204` | Code sent |
| `400` | `"password is incorrect"`, `"enter a valid email address"` or `"that's already your email"` |
| `409` | `"an account with this email already exists"` |
| `429` | `"please wait before requesting another code"`, with `Retry-After`. The earlier code still works |

### `POST /api/me/email/change/confirm` 🔒

```json
{ "code": "042917" }
```

Switches the account to the address the code was sent to, already verified. The old address gets a notice naming the new one (masked, e.g. `n***@example.com`). Sessions are not signed out.

| Status | Body |
| ------ | ---- |
| `200` | [`User`](#user) with the new `email` |
| `400` | `"invalid or expired code"` |
| `409` | `"an account with this email already exists"` (someone else took the address in the meantime) |

### `PUT /api/me/password` 🔒

```json
{ "current_password": "old password", "new_password": "new password" }
```

Signs out every other session. This response sets a fresh `fronko_session` cookie, so the caller stays signed in. It also clears `must_change_password`: a user replacing their organisation's temporary password sends it as `current_password`.

Members and admins can't change their email (`PUT /api/me/email`, `POST /api/me/email/change` and its confirm return `403` `"your email is managed by your organisation; ask them to change it"`). An admin corrects it with [`PATCH /api/org/users/{id}`](#patch-apiorgusersid) while it's unverified.

| Status | Body |
| ------ | ---- |
| `204` | Password changed |
| `400` | `"current password is incorrect"`, `"password must be 8-72 characters"` or `"choose a password different from your current one"` |

---

## Profiles 🔒

Cards belong to the organisation. Owners and admins can act on every card in it; members only on cards assigned to them. **A card outside what the caller can see returns `404`, the same as a missing one.**

### Slug rules

These rules apply to both create and update. The slug is trimmed and lowercased, must be **3–48 characters**, and must match `^[a-z0-9]+(?:-[a-z0-9]+)*$`: lowercase letters and digits separated by single hyphens, with no leading or trailing hyphen. Slugs are globally unique, case-insensitively.

Breaking either rule returns `400` with `"slug must be 3-48 characters: lowercase letters, numbers and hyphens"`. If `data` is present but isn't a JSON object, the response is `400` with `"data must be a JSON object"`.

### `GET /api/me/profiles`

Lists the cards the caller can see, newest first, each with its `lead_count` and `assigned_user`.

**Response:** `200`, an array of [`Profile`](#profile-owner-view). It's `[]` when there are none, never `null`.

### `POST /api/me/profiles`

**Request**

```json
{ "slug": "faiz", "data": { "name": "Faiz" }, "assigned_user_id": 24 }
```

Owner and admins only. `data` is optional and defaults to `{}`. `assigned_user_id` is optional (`null`: the organisation holds the card).

| Status | Body |
| ------ | ---- |
| `201` | The created [`Profile`](#profile-owner-view) |
| `400` | Slug or data validation failed, `"that user isn't in your organisation"`, or `"this card uses a file you don't have access to"` |
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
| `400` | Validation failed, or `"this card uses a file you don't have access to"` (only checked for files the card didn't already use) |
| `403` | A member sent a different slug: `"your organisation manages this card's link"` |
| `404` | `"profile not found"` |
| `409` | `"that slug is already taken"` |

Changing a slug breaks every NFC card or QR code that points at the old URL, so only owners and admins can.

### `DELETE /api/me/profiles/{id}`

Owner and admins only. Deletes the card and, through the `ON DELETE CASCADE` foreign key, all of its leads.

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

Returns one page of the leads the caller can see, newest first (ties broken by newest ID): every lead on the organisation's cards for owners and admins, and for members only the leads that arrived while they held the card (including cards since reassigned). The web app's Leads page, the editor's Leads tab and the overview all use it.

**Query parameters** (all optional)

| Param | Default | Rules |
| ----- | ------- | ----- |
| `profile_id` | none | Only this card's leads. A card outside the caller's view returns an empty page, never its leads |
| `user_id` | none | Owner and admins only (`403` otherwise). Leads that arrived while this user held the card, or `none` for those that arrived while the organisation held it |
| `q` | none | Trimmed, at most 100 chars. Case-insensitive substring match on name, email, phone number or notes. `%` and `_` match literally |
| `since` | none | RFC 3339 timestamp (`2026-10-01T00:00:00Z`). Only leads received at or after it |
| `page` | `1` | 1-based |
| `page_size` | `25` | 1–100 |

**Response:** `200`

```json
{
  "leads": [ { "id": 7, "profile_id": 42, "name": "Jane Doe", "email": "jane@example.com", "phone_country_code": "+91", "phone_number": "9876543210", "notes": "", "created_at": "2026-10-03T12:10:00Z" } ],
  "total": 67,
  "page": 1,
  "page_size": 25
}
```

`total` counts every lead matching the filters, not just this page, so the page count is `ceil(total / page_size)`. A page past the end returns `"leads": []` with the real `total`. To fetch everything (e.g. for an export), request `page_size=100` and walk pages until you have `total` leads.

| Status | Body |
| ------ | ---- |
| `400` | `"invalid page"`, `"invalid page_size"`, `"invalid profile_id"`, `"invalid user_id"`, `"search is too long"` or `"since must be an RFC 3339 timestamp"` |

---

## Storage 🔒

The organisation's **owner** connects their own S3-compatible bucket (Cloudflare R2, Backblaze B2, AWS S3, MinIO, ...), and everyone in the organisation uploads to it. Uploaded photos and PDFs live there; the bucket can stay private. Only the owner can change the settings (`PUT`, `POST …/test` and `DELETE` are 👑).

- Every endpoint here returns **503** `"file storage is not enabled on this server"` when the server has no `SECRETS_KEY`.
- Keys are **write-only**. They're encrypted before storage and never returned. `access_key_hint` (the last 4 characters of the key ID) is the only trace you get back.

### `GET /api/me/storage`

```json
{
  "enabled": true,
  "configured": true,
  "used_bytes": 0,
  "quota_bytes": null,
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

`used_bytes` and `quota_bytes` describe the caller's own personal files against their limit (`null`: unlimited). Only the owner gets the bucket details and `file_count` (every file in the organisation). Admins and members get just `enabled`, `configured`, `used_bytes` and `quota_bytes`, so they know whether uploads work.

When nothing is connected, `configured` is `false` and the bucket fields are left out.

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

Paginated files the caller can see, newest first. Owners and admins see every file in the organisation. Members see their personal files, the shared area, and files granted to them.

| Param | Rules |
| ----- | ----- |
| `kind` | `image` or `pdf` |
| `area` | `personal`, `org`, `shared`, or `granted` (files granted to the caller, or for admins to `user_id`) |
| `user_id` | Owner and admins only (`403` otherwise): one user's files |
| `page`, `page_size` | 1-based; default 24, max 100 |

**Response:** `200 {"files": [File], "total": 5, "page": 1, "page_size": 24}`

### `POST /api/me/files`

`multipart/form-data` with a `file` part, an optional `title` part (up to 120 chars) and an optional `area` part. Rate limited per IP: burst of 10, then 1 every 6s.

Members always upload to `personal` (any other `area` is `403`), and those files count toward their storage limit. Owners and admins upload to `org` (the default) or `shared`.

The type is decided by **sniffing the bytes**, never from the file name or the client's `Content-Type`:

| Accepted | Max size |
| -------- | -------- |
| JPEG, PNG, WebP images | 5 MB |
| PDF | 20 MB |

The object is written to the organisation's bucket as `fronko/<org_id>/<user_id>/<file id>.<ext>`.

| Status | Body |
| ------ | ---- |
| `201` | [`File`](#file) |
| `403` | A member tried to upload outside their personal files |
| `409` | `"connect your storage in Settings first"` |
| `413` | Over the size limit (requests over 25 MB are stopped by nginx first), or `"you've reached your storage limit; …"`. The limit is checked again under a row lock when the file is recorded, so concurrent uploads can't overshoot |
| `415` | `"only JPEG, PNG or WebP images and PDF files can be uploaded"` |
| `502` | `"couldn't save to your storage: <reason>"` |

### `PATCH /api/me/files/{id}`

`{"title": "Spring price list"}`. Returns the updated [`File`](#file). Members can only rename their own personal files; owners and admins any file in the organisation. Anything else is `404`.

### `DELETE /api/me/files/{id}`

Deletes the object from the bucket, then the library entry (`204`). The same rules as renaming decide who can delete what.
- If the bucket refuses the delete, you get `502` and the entry is kept, so you can retry.
- If storage was disconnected, only the entry is removed.
- Cards that used the file stop showing it.

---

## Organisation 🔒 🛡️

Owner and admins only (`403` for members). Admins manage members; only the owner can create admins, change roles, or manage an admin's account. Nobody manages the owner, or themselves, here (use Settings).

### `GET /api/org`

```json
{ "id": 9, "name": "Acme", "default_quota_bytes": 524288000, "created_at": "…", "updated_at": "…" }
```

`default_quota_bytes` is the storage limit new users start with (`null`: unlimited).

### `PUT /api/org`

👑 Owner only. `{"name": "Acme", "default_quota_bytes": null}`. Both fields are sent; the name is 1–80 chars without control characters (it appears in emails) and the limit 0 to 1 TB or `null`. Returns the organisation.

### `GET /api/org/users`

Everyone in the organisation (owner first, then admins, then members, by username), each an **OrgUser**:

```json
{
  "id": 24,
  "role": "member",
  "username": "jane",
  "email": "jane@example.com",
  "email_verified_at": "2026-10-04T20:25:24Z",
  "must_change_password": false,
  "storage_quota_bytes": 1048576,
  "suspended_at": null,
  "last_login_at": "2026-10-04T20:30:00Z",
  "created_at": "2026-10-04T19:54:00Z",
  "updated_at": "2026-10-04T20:25:24Z",
  "card_count": 1,
  "lead_count": 12,
  "used_bytes": 700009
}
```

`lead_count` counts leads that arrived while they held a card; `used_bytes` is the size of their personal files.

### `POST /api/org/users`

```json
{ "username": "jane", "email": "jane@example.com", "password": "temporary-pass", "role": "member", "quota_bytes": 1048576 }
```

Creates a user with a temporary password and emails them their username and that temporary password ("<org> added you to Fronko"). On first sign-in they verify their email (a code is sent then), then must choose their own password before anything else works, so the emailed password stops working as soon as they've set up the account.

| Field | Rules |
| ----- | ----- |
| `username`, `email`, `password` | As for [register](#post-authregister) |
| `role` | `member` (default) or `admin`. Only the owner can create admins |
| `quota_bytes` | Optional. Omitted: the organisation's default. `null`: unlimited |

| Status | Body |
| ------ | ---- |
| `201` | OrgUser |
| `400` | Validation failed |
| `403` | `"only the owner can add admins"` |
| `409` | `"username already taken"` or `"an account with this email already exists"` |

### `GET /api/org/users/{id}`

One OrgUser, or `404` for anyone outside the organisation.

### `PATCH /api/org/users/{id}`

Every field is optional:

```json
{ "email": "jane@example.org", "quota_bytes": null, "role": "admin", "suspended": true }
```

- `email`: only while the user hasn't verified it (`409` after). A new code goes to the new address.
- `quota_bytes`: a new limit, or `null` for unlimited. Lowering it below current usage only stops new uploads.
- `role`: `member` or `admin`; owner only.
- `suspended`: `true` blocks sign-in and ends their sessions at once; their cards stay live and keep collecting leads. `false` restores the account.

Returns the updated OrgUser. `400` `"manage your own account in Settings"` when targeting yourself; `403` when targeting the owner, or an admin as a non-owner.

### `POST /api/org/users/{id}/password`

`{"password": "new-temporary"}`. Sets a new temporary password, signs the user out everywhere, and makes them choose their own on next sign-in. Returns the OrgUser. This one isn't emailed; the admin passes it on.

### `DELETE /api/org/users/{id}`

Deletes the user (`204`) without losing their work, in one transaction:

- Their cards become unassigned. Leads they collected stay on the cards, as the organisation's.
- Their personal files move to the `org` area with `former_owner` set to their username, so cards using them keep working. Other files they uploaded, and cards they created, pass to the owner.
- Grants to them are removed.

### `PUT /api/org/profiles/{id}/assignee`

`{"user_id": 24}` assigns the card to that user; `{"user_id": null}` returns it to the organisation. Past leads stay with whoever held the card when they arrived. Returns the [`Profile`](#profile-owner-view); `400` `"that user isn't in your organisation"`.

### `GET /api/org/files/{id}/grants`

`{"users": [{"id": 24, "username": "jane"}]}`: the users a file has been granted to, beyond everyone who sees it anyway. Shared files can't have grants (`400`).

### `PUT /api/org/files/{id}/grants`

`{"user_ids": [24, 31]}` replaces the list (`[]` revokes all). IDs outside the organisation are ignored. Returns the new list. A card that already uses a file keeps showing it after access is revoked, until someone removes it from the card.

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
{
  "name": "Jane Doe",
  "email": "jane@example.com",
  "phone_country_code": "+91",
  "phone_number": "98765 43210",
  "notes": "Great talk!"
}
```

| Field | Rules (all values are trimmed first) |
| ----- | ------------------------------------ |
| `name` | Required, 1–120 bytes |
| `email` | Required, at most 254 bytes. Must be a bare address: `Jane <jane@x.com>` is rejected |
| `phone_country_code`, `phone_number` | Optional, but both or neither. Spaces, `-`, `.` and parentheses are stripped. The dial code must be `+` and 1–3 digits (`+` is added if missing). The number must be 4–14 digits, with at most 15 digits in total (E.164). They are stored without separators |
| `notes` | Optional, at most 2000 bytes |

| Status | Body |
| ------ | ---- |
| `201` | `{"status": "ok"}` |
| `400` | `"please enter your name"`, `"please enter a valid email address"`, `"please enter a valid mobile number with its country code"` or `"message is too long"` |
| `404` | `"profile not found"` |
| `429` | Rate limited |

The server accepts leads even when the card's `data.collect_leads` is `false`. That flag only hides the form in the UI.

---

## curl walkthrough

```bash
BASE=http://localhost:8080
JAR=$(mktemp)

# Register (sets cookie and emails a code; locally, read it at http://localhost:8025)
curl -s -c $JAR -X POST $BASE/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"demo","email":"demo@example.com","password":"password123"}'

# Verify the email with the code from the message
curl -s -b $JAR -X POST $BASE/api/me/email/verify \
  -H 'Content-Type: application/json' \
  -d '{"code":"123456"}'

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
