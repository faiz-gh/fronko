# Fronko API Reference

Every endpoint is served by the Go backend. By default the browser reaches them through the frontend's nginx (or the Vite dev proxy) on the **same origin** as the SPA. If the API is served on its own domain, list the frontend's origin in `CORS_ALLOWED_ORIGINS`; those origins get credentialed CORS responses (see the [configuration reference](README.md#configuration)).

- **Content type.** Requests and responses use `application/json`.
- **Auth.** Protected endpoints (`/api/me/*`, `/api/org/*` and `/api/integrations/*`) need the `fronko_session` cookie, which login, register or a single sign-on sets. Browsers send it automatically. With `curl`, use a cookie jar (`-c`/`-b`).
- **Organisations and roles.** Every account belongs to an organisation. Registering creates one, with the new account as its **owner**. The owner and **admins** see and manage everything in the organisation. **Members**, whom the organisation creates, see only the cards assigned to them, the leads those cards collected while they held them, their own files, the shared area and files granted to them. The [Organisation](#organisation--) endpoints manage users.
- **Platform admin.** The [Platform admin](#platform-admin-) endpoints (`/auth/admin/*`, `/api/admin/*`) are for whoever runs the server. They use a separate account and cookie, `fronko_admin`; the user cookie is never accepted there, and the admin cookie never works on user routes.
- **Errors.** Every error body has the shape `{"error": "<message>"}`, and the message is safe to show to users. Some add a machine-readable `code`, or a `field` naming the setting at fault. [SCIM](#scim-20) uses SCIM error bodies instead.
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
| `GET`    | [`/api/me/user`](#get-apimeuser--) | ✅ ✉️ | | Current user |
| `PUT`    | [`/api/me/email`](#put-apimeemail--) | ✅ ✉️ | ✅ auth | Add or correct an unverified email |
| `POST`   | [`/api/me/email/verify`](#post-apimeemailverify--) | ✅ ✉️ | ✅ auth | Verify the email with its code |
| `POST`   | [`/api/me/email/resend`](#post-apimeemailresend--) | ✅ ✉️ | ✅ auth | Send a new verification code |
| `POST`   | [`/api/me/email/change`](#post-apimeemailchange-) | ✅ | ✅ auth | Start changing a verified email (password + code to the new address) |
| `POST`   | [`/api/me/email/change/confirm`](#post-apimeemailchangeconfirm-) | ✅ | ✅ auth | Confirm the new email with its code |
| `PUT`    | [`/api/me/password`](#put-apimepassword-) | ✅ 🔑 | ✅ auth | Change password (or replace a temporary one); signs out other sessions |
| `GET`    | [`/api/me/export`](#get-apimeexport-) | ✅ 🔑 | ✅ auth | Download everything held about me, as JSON |
| `GET`    | [`/api/me/account/deletion`](#get-apimeaccountdeletion-) | ✅ 🔑 | | What deleting my account would do |
| `POST`   | [`/api/me/account/deletion/code`](#post-apimeaccountdeletioncode-) | ✅ 🔑 | ✅ auth | Email a code confirming deletion (accounts without a password) |
| `POST`   | [`/api/me/ownership/transfer`](#post-apimeownershiptransfer-) | 👑 | ✅ auth | Hand the organisation to someone else; I become an admin |
| `DELETE` | [`/api/me/account`](#delete-apimeaccount-) | ✅ 🔑 | ✅ auth | Permanently delete my account (an owner: with the organisation, or after handing it over) |
| `GET`    | [`/api/me/profiles`](#get-apimeprofiles) | ✅ | | Cards I can see |
| `POST`   | [`/api/me/profiles`](#post-apimeprofiles) | 🛡️ | | Create a card, optionally assigned to a user |
| `GET`    | [`/api/me/profiles/{id}`](#get-apimeprofilesid) | ✅ | | Get a card |
| `PUT`    | [`/api/me/profiles/{id}`](#put-apimeprofilesid) | ✅ | | Update a card (members can't change the slug) |
| `DELETE` | [`/api/me/profiles/{id}`](#delete-apimeprofilesid) | 🛡️ | | Delete a card and its leads |
| `GET`    | [`/api/me/profiles/{id}/leads`](#get-apimeprofilesidleads) | ✅ | | List a card's leads (unpaginated) |
| `GET`    | [`/api/me/leads`](#get-apimeleads) | ✅ | | Paginated leads I can see, with filters |
| `DELETE` | [`/api/me/leads/{id}`](#delete-apimeleadsid) | 🛡️ | | Delete a lead |
| `POST`   | [`/api/me/leads/delete`](#post-apimeleadsdelete) | 🛡️ | | Delete many leads at once |
| `GET`    | [`/api/me/storage`](#get-apimestorage) | ✅ | | Whether uploads work, my usage, and (owner) the bucket settings |
| `PUT`    | [`/api/me/storage`](#put-apimestorage) | 👑 | | Check, then save storage settings |
| `POST`   | [`/api/me/storage/test`](#post-apimestoragetest) | 👑 | | Check storage settings without saving |
| `DELETE` | [`/api/me/storage`](#delete-apimestorage) | 👑 | | Forget the storage keys |
| `GET`    | [`/api/me/files`](#get-apimefiles) | ✅ | | Paginated files I can see, with filters, search and sort |
| `GET`    | [`/api/me/files/counts`](#get-apimefilescounts) | ✅ | | How many files there are for each purpose |
| `POST`   | [`/api/me/files`](#post-apimefiles) | ✅ | ✅ upload | Upload a photo or PDF, with an optional preview |
| `PATCH`  | [`/api/me/files/{id}`](#patch-apimefilesid) | ✅ | | Rename, re-purpose or move a file |
| `DELETE` | [`/api/me/files/{id}`](#delete-apimefilesid) | ✅ | | Delete a file from the bucket and library |
| `POST`   | [`/api/me/files/bulk`](#post-apimefilesbulk) | ✅ | | Delete or update many files at once |
| `GET`    | [`/api/me/files/{id}/usage`](#get-apimefilesidusage) | ✅ | | Where a file is used |
| `GET`    | [`/api/me/files/{id}/content`](#get-apimefilesidcontent) | ✅ | | A file's bytes, from this origin |
| `GET`    | [`/api/me/files/{id}/image`](#get-apimefilesidimage) | ✅ | | Any image in my organisation, from this origin (QR centre images) |
| `GET`    | [`/api/me/analytics/summary`](#get-apimeanalyticssummary) | ✅ | | Card analytics totals, previous period and breakdowns |
| `GET`    | [`/api/me/analytics/timeseries`](#get-apimeanalyticstimeseries) | ✅ | | Views, visitors, saves and leads per day |
| `GET`    | [`/api/me/analytics/content`](#get-apimeanalyticscontent) | ✅ | | Most used links, quick actions, brochures and images |
| `GET`    | [`/api/me/analytics/cards`](#get-apimeanalyticscards) | ✅ | | Activity per card |
| `GET`    | [`/api/me/analytics/teams`](#get-apimeanalyticsteams) | 🛡️ or team lead | | Teams compared, with the previous period |
| `GET`    | [`/api/me/analytics/members`](#get-apimeanalyticsmembers) | ✅ | | People ranked by their cards' activity |
| `GET`    | [`/api/me/analytics/activity`](#get-apimeanalyticsactivity) | ✅ | | Latest visits, saves, sent forms and brochure opens |
| `GET`    | [`/api/org`](#get-apiorg) | 🛡️ | | The organisation |
| `PUT`    | [`/api/org`](#put-apiorg) | 👑 | | Rename it, set the default storage limit |
| `PUT`    | [`/api/org/handle`](#put-apiorghandle) | 🛡️ | | Change the handle in every card link |
| `PUT`    | [`/api/org/privacy`](#put-apiorgprivacy) | 🛡️ | | Privacy notice link and lead retention |
| `GET`    | [`/api/org/branding`](#get-apiorgbranding) | ✅ | | Logo, logo policy and email signature settings |
| `PUT`    | [`/api/org/branding`](#put-apiorgbranding) | 🛡️ | | Set the logo, logo policy and signature settings |
| `GET`    | [`/api/org/users`](#get-apiorgusers) | 🛡️ | | Everyone in the organisation, with totals |
| `POST`   | [`/api/org/users`](#post-apiorgusers) | 🛡️ | ✅ auth | Create a user with a temporary password |
| `GET`    | [`/api/org/users/{id}`](#get-apiorgusersid) | 🛡️ | | One user |
| `PATCH`  | [`/api/org/users/{id}`](#patch-apiorgusersid) | 🛡️ | | Correct an unverified email, set the storage limit, role or suspension |
| `POST`   | [`/api/org/users/{id}/password`](#post-apiorgusersidpassword) | 🛡️ | ✅ auth | Give a user a new temporary password |
| `DELETE` | [`/api/org/users/{id}`](#delete-apiorgusersid) | 🛡️ | | Delete a user, keeping their cards, files and leads |
| `PUT`    | [`/api/org/users/{id}/teams`](#put-apiorgusersidteams) | 🛡️ | | Replace the teams a user is in |
| `GET`    | [`/api/org/teams`](#get-apiorgteams) | ✅ | | Teams (admins: all; others: their own) |
| `POST`   | [`/api/org/teams`](#post-apiorgteams) | 🛡️ | | Create a team |
| `GET`    | [`/api/org/teams/{id}`](#get-apiorgteamsid) | ✅ | | A team and its people (admins and the team's members) |
| `PATCH`  | [`/api/org/teams/{id}`](#patch-apiorgteamsid) | 🛡️ | | Rename a team, change its description or colour |
| `DELETE` | [`/api/org/teams/{id}`](#delete-apiorgteamsid) | 🛡️ | | Delete a team, keeping its files as the organisation's |
| `PUT`    | [`/api/org/teams/{id}/members`](#put-apiorgteamsidmembers) | 🛡️ | | Replace who is in a team and their roles |
| `PUT`    | [`/api/org/profiles/{id}/assignee`](#put-apiorgprofilesidassignee) | 🛡️ | | Assign a card to a user, or back to the organisation |
| `GET`    | [`/api/org/files/{id}/grants`](#get-apiorgfilesidgrants) | 🛡️ | | Who a file has been granted to |
| `PUT`    | [`/api/org/files/{id}/grants`](#put-apiorgfilesidgrants) | 🛡️ | | Replace who a file is granted to |
| `GET`    | [`/api/integrations/catalog`](#get-apiintegrationscatalog) | ✅ | | Providers the user may connect, with their connections |
| `GET`    | [`/api/integrations/connections`](#get-apiintegrationsconnections) | ✅ | | Connections the user manages |
| `POST`   | [`/api/integrations/connections`](#post-apiintegrationsconnections) | ✅ | | Connect a provider (organisation: 🛡️) |
| `GET`    | [`/api/integrations/connections/{id}`](#get-apiintegrationsconnectionsid) | ✅ | | One connection |
| `PATCH`  | [`/api/integrations/connections/{id}`](#patch-apiintegrationsconnectionsid) | ✅ | | Rename, pause, or change settings |
| `DELETE` | [`/api/integrations/connections/{id}`](#delete-apiintegrationsconnectionsid) | ✅ | | Remove a connection and its log |
| `POST`   | [`/api/integrations/connections/{id}/test`](#post-apiintegrationsconnectionsidtest) | ✅ | ✅ test | Send a test lead, or check the connection |
| `GET`    | [`/api/integrations/connections/{id}/activity`](#get-apiintegrationsconnectionsidactivity) | ✅ | | The connection's activity log |
| `POST`   | [`/api/integrations/connections/{id}/tokens`](#post-apiintegrationsconnectionsidtokens) | ✅ | ✅ auth | Generate a SCIM token |
| `GET`    | [`/api/integrations/connections/{id}/oauth/start`](#get-apiintegrationsconnectionsidoauthstart) | ✅ | | Start authorising an OAuth connection (browser navigation) |
| `GET`    | [`/api/integrations/oauth/callback`](#get-apiintegrationsoauthcallback) | ✅ | | OAuth return address |
| `GET`    | [`/api/org/domains`](#get-apiorgdomains-) | 🛡️ | | Email domains for single sign-on |
| `POST`   | [`/api/org/domains`](#post-apiorgdomains-) | 🛡️ | | Add a domain |
| `POST`   | [`/api/org/domains/{id}/verify`](#post-apiorgdomainsidverify-) | 🛡️ | ✅ auth | Check its DNS TXT record |
| `DELETE` | [`/api/org/domains/{id}`](#delete-apiorgdomainsid-) | 🛡️ | | Remove a domain |
| `POST`   | [`/auth/sso/discover`](#post-authssodiscover) | | ✅ auth | Find an organisation's single sign-on by email or handle |
| `GET`    | [`/auth/sso/{handle}`](#get-authssohandle) | | | Start signing in with the organisation's identity provider |
| `GET`    | [`/auth/saml/{id}/metadata`](#get-authsamlconnection-idmetadata) | | | Service provider metadata |
| `POST`   | [`/auth/saml/{id}/acs`](#post-authsamlconnection-idacs) | | ✅ auth | The identity provider's sign-in response |
| | [`/scim/v2/*`](#scim-20) | 🎫 | ✅ scim | SCIM 2.0: users and groups (teams) |
| `POST`   | [`/api/me/feedback`](#post-apimefeedback) | ✅ | ✅ feedback | Send product feedback to the platform admins |
| `POST`   | [`/auth/admin/login`](#post-authadminlogin) | | ✅ auth | Platform admin sign-in |
| `POST`   | [`/auth/admin/logout`](#post-authadminlogout) | | | Clear the admin cookie |
| `GET`    | [`/api/admin/me`](#get-apiadminme) | 🖥️ | | Signed-in platform admin |
| `GET`    | [`/api/admin/summary`](#get-apiadminsummary) | 🖥️ | | Platform totals |
| `GET`    | [`/api/admin/trends`](#get-apiadmintrends) | 🖥️ | | Daily platform totals |
| `GET`    | [`/api/admin/orgs`](#get-apiadminorgs) | 🖥️ | | Organisations with their usage, paginated |
| `GET`    | [`/api/admin/orgs/{id}`](#get-apiadminorgsid) | 🖥️ | | One organisation's usage |
| `GET`    | [`/api/admin/orgs/{id}/trends`](#get-apiadminorgsidtrends) | 🖥️ | | One organisation's daily usage |
| `POST`   | [`/api/admin/orgs/{id}/suspend`](#post-apiadminorgsidsuspend) | 🖥️ | | Suspend an organisation and email its owner |
| `POST`   | [`/api/admin/orgs/{id}/reinstate`](#post-apiadminorgsidreinstate) | 🖥️ | | Lift a suspension and email the owner |
| `GET`    | [`/api/admin/feedback`](#get-apiadminfeedback) | 🖥️ | | Feedback inbox, paginated, with counts by status |
| `GET`    | [`/api/admin/feedback/{id}`](#get-apiadminfeedbackid) | 🖥️ | | One piece of feedback and its replies |
| `PATCH`  | [`/api/admin/feedback/{id}`](#patch-apiadminfeedbackid) | 🖥️ | | Set the status |
| `POST`   | [`/api/admin/feedback/{id}/replies`](#post-apiadminfeedbackidreplies) | 🖥️ | | Email a reply to the sender |
| `GET`    | [`/api/admin/audit`](#get-apiadminaudit) | 🖥️ | | What platform admins did, newest first |
| `GET`    | [`/api/profiles/{org}/{slug}`](#get-apiprofilesorgslug) | | | Public profile by its link (organisation handle + slug) |
| `GET`    | [`/api/profiles/{org}/{slug}/vcard`](#get-apiprofilesorgslugvcard) | | | The card as a contact file (`.vcf`) |
| `GET`    | [`/api/files/{id}`](#get-apifilesid) | | | Redirect to a file (short-lived signed URL) |
| `POST`   | [`/api/profiles/{id}/leads`](#post-apiprofilesidleads) | | ✅ lead | Submit a lead to a profile |
| `POST`   | [`/api/profiles/{org}/{slug}/events`](#post-apiprofilesorgslugevents) | | ✅ events | Record what a visitor did on a public card |

✉️ = works before the email is verified. 🔑 = works while the user still has a temporary password. Every other signed-in route needs a verified email and a password the user chose. 🛡️ = owner and admins only. 👑 = owner only. Others get `403`. 🖥️ = platform admins only (`fronko_admin` cookie); anything else gets `401`. 🎫 = a SCIM bearer token, not a session.

## Cross-cutting behaviour

| Status | When |
| ------ | ---- |
| `400` | Body isn't valid JSON (`"invalid request body"`), the body is over 64 KiB, a path ID isn't an integer (`"invalid profile ID"`), or validation failed |
| `401` | `/api/me/*` without a cookie (`"not signed in"`), with an invalid or expired token, or with a token from before a password change or reset (`"session expired, please sign in again"`) |
| `401` | A suspended account: `{"error":"this account is suspended; contact your organisation","code":"account_suspended"}`. Suspending also ends existing sessions, which then get `"session expired, please sign in again"` |
| `401` | Anyone in a suspended organisation: `{"error":"your organisation has been suspended","code":"org_suspended","reason":"<the admin's reason>"}`. This is checked before the session version, so open sessions get it too |
| `403` | Signed-in routes (except the ✉️ routes) while the email isn't verified: `{"error":"verify your email to continue","code":"email_unverified"}` |
| `403` | Signed-in routes (except ✉️ and 🔑) while the user still has the temporary password their organisation set: `{"error":"choose a new password to continue","code":"password_change_required"}` |
| `403` | A 🛡️ or 👑 route called by someone without that role: `"only your organisation's admins can do this"` or `"only your organisation's owner can do this"` |
| `403` | Password sign-in or reset for someone who must use single sign-on: `{"error":"your organisation signs in with single sign-on","code":"sso_required","sso_url":"/auth/sso/acme"}`. Send them to `sso_url` |
| `403` | A `POST`/`PUT`/`PATCH`/`DELETE` whose `Origin` header names a different host (`"cross-origin request rejected"`). Requests without `Origin`, such as curl, are allowed. [SCIM](#scim-20) and the SAML ACS skip this check |
| `429` | Rate limit exceeded (`"too many requests, please try again shortly"`), with a `Retry-After: <seconds>` header |
| `500` | Unexpected server error (`"internal error"` or a specific "failed to …" message) |

**Rate limits** are per client IP:

- **auth**: burst of 10, then 1 request per 10s. Every route marked "auth" shares one bucket.
- **lead**: burst of 5, then 1 request per 15s.
- **events**: burst of 30, then 1 request per 2s. A visit sends a handful of beacons a few seconds apart.
- **feedback**: burst of 5, then 1 request per 12 minutes.
- **test**: burst of 5, then 1 request per 10s (integration tests).
- **scim**: burst of 200, then 1 request per 20ms.

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
  "org_handle": "acme",
  "must_change_password": false,
  "has_password": true,
  "teams": [{ "id": 3, "name": "Sales", "color": "#e11d48", "role": "lead" }]
}
```

`email` is `null` only for accounts created before emails were required; they must add and verify one before using the app. `role` is `owner`, `admin` or `member`. `teams` lists the teams the user is in; their `role` in each is `lead` or `member`.

**Teams.** An organisation can group its people into teams, and anyone can be in several. Being in a team lets you see the team's files. A team's **leads** also add, change and delete the team's files, and they see and edit the cards of the people in their teams, along with those people's leads. Creating, deleting and reassigning cards stays with admins. Org users (`GET /api/org/users`) carry the same `teams` list. `must_change_password` is `true` while the user still has a temporary password their organisation set. `has_password` is `false` for accounts that only sign in with single sign-on (created by SSO or SCIM); they can't change a password they don't have.

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
  "lead_count": 3,
  "booking": null
}
```

`user_id` is the account that created the card. `assigned_user` is the one user who works on it, or `null` when the organisation holds it. `lead_count` counts every lead for admins, and only the member's own leads for members. `booking` is the booking page the card shows, as on the [public profile](#publicprofile-visitor-view). Single-card responses (get, create, update, assign) also include `booking_options`: the pages the card can choose from, which are the holder's own and then the organisation's, in the same shape. Saving a `data.booking_connection_id` that isn't among them answers `400` ("that booking page isn't available for this card"), unless the card already had it.

### PublicProfile (visitor view)

```json
{
  "id": 42,
  "slug": "faiz",
  "org_handle": "acme",
  "data": { "name": "Faiz", "avatar_file": "90meIH31WrEH0xe9ymyzDA", "documents": [] },
  "files": [{ "id": "90meIH31WrEH0xe9ymyzDA", "kind": "image", "name": "me.png", "size_bytes": 48211 }],
  "org": { "name": "Acme", "logo_file": "tt24UJovNZgYmyTuoygXhg", "logo_policy": "optional", "privacy_url": "https://acme.example/privacy" },
  "booking": {
    "connection_id": 12,
    "label": "30-min intro",
    "provider": "calendly",
    "name": "Calendly",
    "url": "https://calendly.com/faiz/30min",
    "prefill": { "name": "name", "email": "email" },
    "scope": "user"
  }
}
```

`booking` is the card's "Book a meeting" page, connected in [Integrations](#integrations-) (Calendar Booking) and chosen by the card's `data.booking_connection_id`. It is `null` when the card chose none, or when the chosen page is gone, paused, or belongs to someone other than the card holder. `connection_id` and `label` are the connection's id and name; `scope` is `"user"` for a person's own page and `"org"` for the organisation's. `prefill`, when present, maps the page's query parameters to what a visitor gave the lead form (`name`, `first_name`, `last_name`, `email`), so the page can open with them filled in. This replaces the card's old `data.calendar_url`, which is no longer read.

`org_handle` and `slug` make up the card's link, `/p/{org_handle}/{slug}`. `org` is the card's organisation: its name, its logo (a public file id, served by [`GET /api/files/{id}`](#get-apifilesid), or `null`) and `logo_policy`. The card shows the logo when the policy is `"required"`, or when it's `"optional"` and `data.show_org_logo` isn't `false`. The organisation's signature settings are not exposed here.

The owner's ID and the timestamps are left out on purpose. `files` lists only the library files the card itself references (`data.avatar_file`, `data.cover_file`, `data.documents[].file` and gallery images in `data.blocks[].images[].file`, up to 64) that the card's organisation still has. Nothing else from the organisation's files is revealed.

### File

```json
{
  "id": "r749f-c4WN5WAPhaxOzysg",
  "area": "personal",
  "owner": { "id": 24, "username": "jane" },
  "kind": "pdf",
  "purpose": "brochure",
  "content_type": "application/pdf",
  "size_bytes": 411,
  "name": "partnership.pdf",
  "title": "Partnership brochure",
  "width": null,
  "height": null,
  "pages": 4,
  "has_thumb": true,
  "use_count": 2,
  "created_at": "2026-10-03T20:53:27Z",
  "updated_at": "2026-10-03T20:53:27Z"
}
```

`id` is a random 22-character public ID (128 bits), the only file identifier the API exposes. `kind` is `image` or `pdf`.

`purpose` says what the file is for: `logo`, `banner`, `avatar` (profile photo), `cover`, `gallery`, `brochure` or `other`. PDFs are `brochure` or `other`; images can be anything except `brochure`.

`width` and `height` (images) and `pages` (PDFs) are what the browser reported at upload; they're `null` for older files. `has_thumb` says there's a small preview at `/api/files/{id}?size=thumb`. `use_count` counts the cards using the file, plus one each if it's the organisation's logo or signature banner.

`area` is one of:

- `personal`: a user's own file. `owner` is that user. It counts toward their storage limit.
- `org`: the organisation's private file, visible to admins and to members it's been granted to. `owner` is whoever uploaded it.
- `shared`: visible to everyone in the organisation.
- `team`: a team's file, visible to its members and editable by its leads. `team` is `{"id", "name", "color"}`.

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
  "source": "qr",
  "created_at": "2026-10-03T12:10:00Z",
  "assigned_user": { "id": 24, "username": "jane" }
}
```

`source` is how the visitor reached the card when they sent the form: `nfc`, `qr` or `link`. It is left out for leads from before analytics (migration 012).

`assigned_user` is who held the card when the lead arrived (`null`: the organisation). Leads stay with that person when the card is reassigned.

`phone_country_code` and `phone_number` are left out when the visitor didn't give a number. Both are digits only (the dial code keeps its `+`).

### `data`

`data` is a free-form JSON **object** that the frontend owns. The backend only checks that it is an object, and stores `{}` when it's missing or `null`. It does read a few keys: `avatar_file`, `cover_file`, `documents[].file` and `blocks[].images[].file` name library files, which must be visible to the editor when newly added, and which `GET /api/profiles/{org}/{slug}` resolves (up to 64 per card). The vCard endpoint reads the contact fields. `show_org_logo` (boolean), `signature` (the card's email signature choices) and `qr` (the QR code's colours, dot and corner style and centre image) are frontend-only keys the backend just stores. For the shape the web app uses, see [`CardData` in the frontend docs](../frontend/README.md#card-data-model).

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
| `403` | `{"error":"your organisation has been suspended","code":"org_suspended","reason":"…"}`, only after a correct password |
| `403` | `{"error":"your organisation signs in with single sign-on","code":"sso_required","sso_url":"/auth/sso/acme"}`: the account has no password, or the organisation requires single sign-on and this isn't the owner. For accounts with a password, only after a correct password |

The first sign-in of a user the organisation created emails them a verification code (subject to the 60-second resend cooldown).

### `POST /auth/logout`

Clears the session cookie. It works without a session. The JWT isn't revoked server-side.

**Response:** `204 No Content`

### `POST /auth/password/forgot`

```json
{ "email": "faiz@example.com" }
```

Emails a reset code if a **verified** account uses this address, the 60-second cooldown has passed, and the account isn't one that must use single sign-on. It always answers the same way, so it can't reveal which emails have accounts.

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
| `403` | `sso_required`, as for login |

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

Adds an email to an older account, or corrects a typo before verification, and sends a verification code to it. A new address always gets a fresh code; re-sending the same address respects the cooldown. A verified email is changed with [`POST /api/me/email/change`](#post-apimeemailchange-) instead.

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

### `GET /api/me/export` 🔒

Downloads everything Fronko holds about the signed-in person (right of access and data portability) as an attachment, `fronko-{username}-data.json`:

```json
{
  "format": "fronko-export/1",
  "exported_at": "2026-10-08T09:00:00Z",
  "data": {
    "account": { "id": 1, "username": "jane", "email": "jane@example.com", "role": "member", "…": "…" },
    "organisation": { "name": "Acme", "handle": "acme" },
    "teams": [ { "name": "Sales", "role": "lead", "added_at": "…" } ],
    "cards": [ { "id": 42, "slug": "jane", "held_by_you": true, "made_by_you": false, "data": { }, "…": "…" } ],
    "leads": [ { "name": "…", "email": "…", "card": "jane", "received_at": "…" } ],
    "files": [ { "id": "…", "name": "brochure.pdf", "area": "personal", "size_bytes": 1024, "…": "…" } ],
    "integrations": [ { "provider": "calendly", "name": "Intro call", "category": "calendar" } ],
    "feedback": [ { "category": "idea", "message": "…", "sent_at": "…" } ]
  }
}
```

`cards` are the cards the person made or holds; `leads` those that arrived while they held the card; `files` are details only (not contents, which are in the organisation's bucket); `integrations` never include secrets. Analytics aren't included: they're anonymous counts, not about the person.

### `GET /api/me/account/deletion` 🔒

What deleting the account would do, so the dashboard can warn first:

```json
{
  "username": "jane", "role": "owner", "has_password": true, "managed": false,
  "org_name": "Acme", "org_handle": "acme", "sole_member": false,
  "summary": { "cards_held": 1, "cards_made": 3, "files": 4, "personal_files": 2, "leads": 12,
               "org_members": 5, "org_cards": 9, "org_leads": 140, "org_files": 30, "org_teams": 2 },
  "transfer_candidates": [ { "id": 7, "username": "sam" } ]
}
```

`owner_username` (members and admins) is who their cards and files pass to. `transfer_candidates` (owner) are the active, verified people who can take over. `managed` is set for people provisioned by SCIM: they're removed by their identity provider, not here.

### `POST /api/me/account/deletion/code` 🔒

For accounts without a password (single sign-on only): emails a 6-digit code (15 minutes, 5 attempts) that confirms the next two endpoints. `204`; `400` `"confirm with your password instead"` for accounts with a password; `429` with `Retry-After` inside the resend cooldown.

### `POST /api/me/ownership/transfer` 🔒

👑 Owner only. `{"user_id": 7, "password": "…"}` (or `"code"` instead of `"password"` for accounts without one). Makes that person the owner and the caller an admin, in one transaction. The organisation's bucket keys are re-encrypted for the new owner. Both are signed out everywhere; the response sets a fresh cookie for the caller and returns their [`User`](#user). The new owner is emailed.

| Status | Body |
| ------ | ---- |
| `200` | [`User`](#user), now `admin` |
| `400` | `"password is incorrect"`, `"invalid or expired code"`, `"choose someone in your organisation"` or `"the new owner must be active and have a verified email"` |
| `403` | `"only the organisation's owner can hand it over"` |

### `DELETE /api/me/account` 🔒

Permanently deletes the signed-in account, straight away (there's no grace period):

```json
{ "confirm": "jane", "password": "…", "delete_org": false }
```

- `confirm` is the username typed out, or the organisation's handle when the organisation is deleted too.
- `password`, or `code` from [`POST /api/me/account/deletion/code`](#post-apimeaccountdeletioncode-) for accounts without one.
- **Members and admins:** as when an admin [deletes them](#delete-apiorgusersid): cards they held become unassigned, leads stay with the organisation, files and cards they made pass to the owner. Their email is also removed from feedback they sent.
- **The owner** must first [hand the organisation over](#post-apimeownershiptransfer-), or send `"delete_org": true`: the organisation and everything in it is deleted (people, cards, leads, analytics, teams, files and integrations), and its files' objects are removed from the bucket first (best effort; other objects in the bucket are left). Feedback its people sent keeps only the message; the platform audit log keeps only the action. An owner who is the organisation's only member always deletes it.

The account's address gets a confirmation email. The response clears the session cookie.

| Status | Body |
| ------ | ---- |
| `204` | Deleted |
| `400` | `"type jane to confirm"`, `"password is incorrect"` or `"invalid or expired code"` |
| `409` | `{"code": "ownership_transfer_required"}` (an owner with other members, without `delete_org`) or `{"code": "managed_by_idp"}` (a SCIM-provisioned account) |

---

## Profiles 🔒

Cards belong to the organisation. Owners and admins can act on every card in it; members only on cards assigned to them. **A card outside what the caller can see returns `404`, the same as a missing one.**

### Slug rules

These rules apply to both create and update. The slug is trimmed and lowercased, must be **3–48 characters**, and must match `^[a-z0-9]+(?:-[a-z0-9]+)*$`: lowercase letters and digits separated by single hyphens, with no leading or trailing hyphen. Slugs are unique within the organisation, case-insensitively; other organisations can use the same slug, because links include the organisation's handle (`/p/{handle}/{slug}`).

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
| `409` | `"another card in your organisation already uses that link"` |

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
| `409` | `"another card in your organisation already uses that link"` |

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
| `user_id` | none | Owner, admins and team leads (`403` otherwise; `none` is admins only). Leads that arrived while this user held the card, or `none` for those that arrived while the organisation held it |
| `team_id` | none | Owner, admins, and leads of that team (`403` otherwise). Leads that arrived while someone in the team held the card |
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

### `DELETE /api/me/leads/{id}`

🛡️ Owners and admins. Deletes one of the organisation's leads, with the lead sync log lines about it (they name the person). Copies already sent to connected tools aren't deleted there. `204`; `404` `"lead not found"`.

### `POST /api/me/leads/delete`

🛡️ Owners and admins. `{"ids": [7, 8, 9]}` (1–500). Deletes those leads as above; ids outside the organisation are ignored. Returns `{"deleted": 3}`.

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

Paginated files the caller can see. Owners and admins see every file in the organisation. Members see their personal files, the shared area, their teams' files, and files granted to them or to one of their teams.

| Param | Rules |
| ----- | ----- |
| `kind` | `image` or `pdf` |
| `area` | `personal`, `org`, `shared`, `team`, or `granted` (files granted to the caller or their teams, or for admins to `user_id`) |
| `team_id` | One team's files |
| `user_id` | Owner and admins only (`403` otherwise): one user's files |
| `purpose` | Comma-separated purposes, e.g. `logo,banner` |
| `q` | Case-insensitive search of titles and file names, up to 100 characters |
| `sort` | `newest` (the default), `oldest`, `name` or `size` (largest first) |
| `page`, `page_size` | 1-based; default 24, max 100 |

**Response:** `200 {"files": [File], "total": 5, "page": 1, "page_size": 24}`

### `GET /api/me/files/counts`

Takes the same filters as listing, except that `purpose` is ignored. Returns `200 {"counts": {"logo": 2, "brochure": 5}, "total": 7}`. Purposes that have no files are left out.

### `POST /api/me/files`

`multipart/form-data` with a `file` part, plus these optional parts. Rate limited per IP: burst of 10, then 1 every 6s.

| Part | Rules |
| ---- | ----- |
| `title` | Up to 120 characters |
| `area` | Where the file goes (see below) |
| `team_id` | Required when `area` is `team` |
| `purpose` | Defaults to `brochure` for PDFs and `other` for images |
| `width`, `height` | Images: their size in pixels |
| `pages` | PDFs: their page count |
| `thumb` | A preview image (JPEG, PNG or WebP, up to 300 KB). A missing, oversized or unreadable preview is skipped; the upload still succeeds |

Where an upload can go:
- Members upload to `personal`, and those files count toward their storage limit.
- Team leads can also upload to `team`, for the teams they lead.
- Owners and admins upload to `org` (the default), `shared`, or any team's area.
- Anything else is `403`.

The type is decided by **sniffing the bytes**, never from the file name or the client's `Content-Type`:

| Accepted | Max size |
| -------- | -------- |
| JPEG, PNG, WebP images | 5 MB |
| PDF | 20 MB |

The object is written to the organisation's bucket as `fronko/<org_id>/<user_id>/<file id>.<ext>`, and the preview next to it as `<file id>.thumb.<ext>`.

| Status | Body |
| ------ | ---- |
| `201` | [`File`](#file) |
| `400` | The purpose doesn't fit the file, or the team isn't in the organisation |
| `403` | The caller can't upload to that area or team |
| `409` | `"connect your storage in Settings first"` |
| `413` | Over the size limit (requests over 25 MB are stopped by nginx first), or `"you've reached your storage limit; …"`. The limit is checked again under a row lock when the file is recorded, so concurrent uploads can't overshoot |
| `415` | `"only JPEG, PNG or WebP images and PDF files can be uploaded"` |
| `502` | `"couldn't save to your storage: <reason>"` |

### `PATCH /api/me/files/{id}`

`{"title": "Spring price list", "purpose": "brochure", "area": "team", "team_id": 3}`. Every field is optional. Returns the updated [`File`](#file).

Who can change what:
- Members can only change their own personal files.
- Team leads can change their teams' files.
- Owners and admins can change any file in the organisation.
- Anything else is `404`.

`area` (with `team_id` for teams) moves the file:
- Nobody can move a file into `personal`.
- Owners and admins can move files to `org`, `shared` or any team.
- Team leads can move their own personal files, and their teams' files, into a team they lead.
- Any other move is `403`.

### `DELETE /api/me/files/{id}`

Deletes the object from the bucket, then the library entry (`204`). The same rules as renaming decide who can delete what.
- If the bucket refuses the delete, you get `502` and the entry is kept, so you can retry.
- If storage was disconnected, only the entry is removed.
- Cards that used the file stop showing it.
- If the file was the organisation's logo or signature banner, that setting is cleared in the same step.
- Its preview is deleted too.

### `POST /api/me/files/bulk`

`{"ids": ["…", "…"], "action": "delete"}` or `{"ids": [...], "action": "update", "patch": {"purpose": "gallery"}}`. Takes 1–100 IDs. Each file is handled as if by its own `DELETE` or `PATCH`. Returns `200 {"done": ["…"], "failed": [{"id": "…", "error": "file not found"}]}`.

### `GET /api/me/files/{id}/usage`

Where a file the caller can see is used:

```json
{
  "cards": [{ "profile_id": 7, "slug": "jane", "name": "Jane Doe", "slot": "avatar" }],
  "hidden_cards": 1,
  "org_logo": false,
  "signature_banner": false
}
```

`slot` is `avatar`, `cover`, `document` or `gallery`. Cards the caller can't see are only counted, in `hidden_cards`.

### `GET /api/me/files/{id}/content`

The bytes of a file the caller can see, served as an attachment with `Cache-Control: private, no-store`. The app uses this to crop an existing image into a new copy, because the bucket usually doesn't allow the browser to read objects cross-origin. Returns `404` for a file the caller can't see, and `502` if storage can't be read.

### `GET /api/me/files/{id}/image`

The bytes of **any image in the caller's organisation**, served inline from this origin with `Cache-Control: private, no-store`. The app draws a card's QR code with the organisation logo or a chosen image in the middle, and needs the image's bytes to embed it in the SVG and PNG downloads. The bucket usually blocks cross-origin reads, and members can't always see the organisation's own files, so `/content` won't do. Images on cards are public through [`GET /api/files/{id}`](#get-apifilesid) anyway; this only removes the cross-origin step. PDFs, unknown ids and other organisations' files return `404`; unreadable storage returns `502`.

---

## Analytics 🔒

What visitors do on public cards. The public card page reports events to [`POST /api/profiles/{org}/{slug}/events`](#post-apiprofilesorgslugevents), and the server records contact saves and sent forms itself. These endpoints only return aggregates; no response includes a visitor hash or visit id.

**Who sees what.** The same rules as leads apply: an event belongs to whoever held the card when it happened. Members see their own cards' activity, team leads also see their teammates', and owners and admins see the whole organisation.

**Shared query parameters** (all optional):

| Parameter | Meaning |
| --------- | ------- |
| `from`, `to` | RFC 3339 timestamps. Defaults to the last 30 days. The period must be longer than zero and at most 366 days. |
| `tz` | IANA time zone that days and hours are counted in (default `UTC`). A legacy name the database doesn't know, such as `Asia/Calcutta`, falls back to that zone's current UTC offset. |
| `profile_id` | One card. Cards the caller can't see simply match nothing. |
| `user_id` | Activity while this person held the card. |
| `team_id` | Activity while someone in this team held the card. Admins, or that team's lead; others get `403`. |

Invalid values return `400` (`"from must be an RFC 3339 timestamp"`, `"invalid tz"`, `"invalid team_id"` and so on).

**Definitions.**
- A **visit** is one page load of the card (the `session` the tracker sends). **Views** count visits.
- **Unique visitors** are distinct visitor hashes. The hash salt changes every day, so this counts distinct visitors per day, summed over the period.
- An **engaged** visit clicked something, opened a brochure or image, saved the contact, opened the form, shared the card, or scrolled at least half way.
- An **action** visit saved the contact or opened the form.
- **Scroll depth** is each visit's deepest 25 / 50 / 75 / 100 % mark. It is only measured once the visitor scrolls, so cards shorter than the screen record none.
- **Time on card** is how long the card was on screen in a visit, capped at an hour.

### `GET /api/me/analytics/summary`

```json
{
  "from": "2026-09-07T11:02:40Z",
  "to": "2026-10-07T11:02:40Z",
  "current": {
    "views": 247, "unique_visitors": 247, "sessions": 247,
    "nfc_views": 90, "qr_views": 82, "link_views": 75,
    "saves": 52, "form_opens": 21, "leads": 68,
    "doc_opens": 40, "clicks": 98, "gallery_opens": 0, "shares": 15,
    "engaged_sessions": 192, "action_sessions": 73, "repeat_visitors": 0,
    "avg_scroll_depth": 55.2, "median_time_ms": 73000
  },
  "previous": { "views": 133, "...": "same fields, for the same-length period just before" },
  "devices": { "mobile": 167, "tablet": 47, "desktop": 33 },
  "lead_sources": { "nfc": 3, "qr": 5, "link": 1, "unknown": 59 },
  "heatmap": [[0, 0, 1, "… 24 hours"], "… 7 days, Sunday first"],
  "scroll_depths": [39, 49, 54, 37, 68],
  "time_buckets": [14, 35, 46, 152, 0]
}
```

- `leads` comes from the leads table, so it includes leads from before analytics.
- `repeat_visitors` counts visitors who came back for another visit on the same day.
- `scroll_depths` counts visits by their deepest scroll: under 25 %, 25 %, 50 %, 75 % and 100 %.
- `time_buckets` counts visits by time on card: under 10 s, 10–30 s, 30 s–1 min, 1–3 min, and 3 min or more.
- `heatmap` counts views by weekday (`0` = Sunday) and hour, in `tz`.

### `GET /api/me/analytics/timeseries`

`{"points": [...]}`: one point per day of the period in `tz`, oldest first, including empty days. Each point is `{date, views, nfc_views, qr_views, link_views, unique_visitors, saves, leads}`, with `date` as `YYYY-MM-DD`.

### `GET /api/me/analytics/content`

`{"items": [...]}`: the 100 most used targets, busiest first. Each item is `{type, target, label, count, unique}`:

| `type` | `target` |
| ------ | -------- |
| `click` | A link's URL, or a quick action: `email`, `call`, `website` or `booking` |
| `doc_open` | The brochure's file id. The same brochure on several cards adds up. |
| `gallery_open` | The image's file id |

`label` is the latest label seen (the link or brochure title), and `unique` counts distinct visitors.

### `GET /api/me/analytics/cards`

`{"cards": [...]}`: every card the caller can see that matches the filters, busiest first, including cards with no views. Each card is `{profile_id, slug, name, assigned_user, views, unique_visitors, nfc_views, qr_views, link_views, saves, form_opens, leads, doc_opens, last_viewed_at}`. `last_viewed_at` is the card's latest view ever, not just in the period, or `null` if it has never been viewed.

### `GET /api/me/analytics/teams`

Owners and admins get every team; team leads get the teams they lead; anyone else gets `403`. `team_id` narrows it to one team. `profile_id` and `user_id` are ignored.

`{"teams": [...]}`, busiest first. Each team is `{id, name, color, members, cards, active_cards, views, unique_visitors, sessions, engaged_sessions, saves, form_opens, leads, prev_views, prev_saves, prev_leads}`.
- `cards` counts the cards held by the team's people right now.
- `active_cards` counts those viewed in the period.
- The `prev_*` fields cover the same-length period just before.

### `GET /api/me/analytics/members`

`{"members": [...]}`: the people the caller can see (members see only themselves), optionally only those in `team_id`. They are ranked by views, at most 200. Each person is `{user_id, username, cards, views, unique_visitors, saves, form_opens, doc_opens, leads}`.

### `GET /api/me/analytics/activity`

`{"items": [...]}`: the latest visits, contact saves, sent forms and brochure opens, newest first. `limit` is 1–50 (default 15). Each item is `{type, source, label, profile_id, card_name, slug, assigned_user, created_at}`, where `type` is `view`, `vcard`, `form_submit` or `doc_open`.

---

## Organisation 🔒 🛡️

Owner and admins only (`403` for members). Admins manage members; only the owner can create admins, change roles, or manage an admin's account. Nobody manages the owner, or themselves, here (use Settings).

### `GET /api/org`

```json
{ "id": 9, "name": "Acme", "handle": "acme", "default_quota_bytes": 524288000,
  "privacy_url": "https://acme.example/privacy", "lead_retention_days": 365, "created_at": "…", "updated_at": "…" }
```

`default_quota_bytes` is the storage limit new users start with (`null`: unlimited). `handle` is the organisation's part of every card link, `/p/{handle}/{slug}`. Registration makes it from the organisation's name (falling back to the username), adding `-2`, `-3`… when it's taken.

### `PUT /api/org`

👑 Owner only. `{"name": "Acme", "default_quota_bytes": null}`. Both fields are sent; the name is 1–80 chars without control characters (it appears in emails) and the limit 0 to 1 TB or `null`. Returns the organisation.

### `PUT /api/org/handle`

🛡️ Owners and admins. `{"handle": "acme"}`. The handle is trimmed and lowercased, must be **3–32 characters** and match the slug pattern, and is unique across organisations, case-insensitively. Returns the organisation.

Every card link changes with it, and links already on QR codes, NFC cards and email signatures stop working; the old handle is released at once, so another organisation can take it. The dashboard warns and asks for confirmation first.

| Status | Body |
| ------ | ---- |
| `400` | `"handle must be 3-32 characters: lowercase letters, numbers and hyphens"` |
| `409` | `"another organisation already uses that handle"` |

### `PUT /api/org/privacy`

🛡️ Owners and admins. `{"privacy_url": "https://acme.example/privacy", "lead_retention_days": 365}`. Returns the organisation, which carries both fields.

- `privacy_url` (full http(s) URL, or `null`/`""` to clear) is linked from every card's contact form and footer, and sent to visitors as `org.privacy_url` on the [public card](#publicprofile-visitor-view).
- `lead_retention_days` (30–3650, or `null` to keep leads until someone deletes them): leads older than this are deleted automatically, every few hours, with their lead sync log lines.

| Status | Body |
| ------ | ---- |
| `400` | `"privacy notice must be a full http(s) URL"` or `"keep leads for 30 to 3650 days, or forever"` |

### `GET /api/org/branding`

✅ Anyone in the organisation (the editor and the Signatures page need it), so this one isn't 🛡️.

```json
{
  "name": "Acme",
  "logo_file": "tt24UJovNZgYmyTuoygXhg",
  "logo_policy": "required",
  "signature": {
    "locked_template": "corporate",
    "brand_color": "#dc2626",
    "disclaimer": "Confidential: for the named recipient only.",
    "banner_file": "iatlE9Ig8c5o8hgdVDLxOw",
    "banner_url": "https://example.com/event"
  }
}
```

- `logo_file`: public id of an image in the organisation's files, or `null`. The web app crops it square (512×512 PNG).
- `logo_policy`: `"required"` (every card and signature shows the logo) or `"optional"` (each card chooses; on by default).
- `signature`: every key is optional and left out when empty.
  - `locked_template`: when set, the only template employees can use. One of `classic`, `corporate`, `compact`, `bold`, `minimal`.
  - `brand_color`: `#rrggbb`; replaces each card's accent in signatures.
  - `disclaimer`: up to 1000 characters, shown under every signature.
  - `banner_file` / `banner_url`: an image shown under every signature, and where clicking it goes. The web app crops the banner to 4:1, 3:1 or 2:1 (JPEG).

### `PUT /api/org/branding`

🛡️ Owner and admins. Send the whole object above, without `name`. Returns it saved.

`400` when:
- `logo_policy` is something other than `"required"` or `"optional"`. Leaving it out means `"optional"`.
- `locked_template` isn't one of the five templates.
- `brand_color` isn't `#` followed by six hex digits.
- `disclaimer` is over 1000 characters.
- `banner_url` isn't a full `http(s)://` URL.
- `logo_file` or `banner_file` isn't an image in the organisation's own or shared files. Personal files are refused, since the logo and banner are public.

Blank strings are trimmed. A blank `logo_file` means no logo.

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

### `PUT /api/org/users/{id}/teams`

`{"teams": [{"team_id": 3, "role": "lead"}, {"team_id": 5}]}` replaces the teams a user is in. `role` is `lead` or `member` (the default). Teams outside the organisation are ignored. Anyone in the organisation can be put in teams, including admins and the owner. Returns the org user. `POST /api/org/users` also takes an optional `teams` list in the same shape.

### `GET /api/org/teams`

Owners and admins get every team; everyone else gets the teams they're in. Teams are sorted by name:

```json
[{ "id": 3, "name": "Sales", "description": "", "color": "#e11d48", "member_count": 6, "lead_count": 1, "file_count": 12, "created_at": "…", "updated_at": "…" }]
```

### `POST /api/org/teams`

`{"name": "Sales", "description": "Field sales", "color": "#e11d48", "members": [{"user_id": 24, "role": "lead"}]}`.
- `name` is 1–60 characters and unique within the organisation, ignoring case (`409` otherwise).
- `description` is up to 280 characters.
- `color` is `#rrggbb` or empty.
- `members` is optional.

Returns `201` and the team with its `members`.

### `GET /api/org/teams/{id}`

The team plus `members: [{"id", "username", "email", "org_role", "role", "added_at"}]`, leads first. Only owners, admins and the team's own members can see it; anyone else gets `404`.

### `PATCH /api/org/teams/{id}`

`{"name", "description", "color"}` with the same rules as creating a team. Returns the team with its members.

### `DELETE /api/org/teams/{id}`

Deletes the team (`204`). Its files move to the `org` area so cards using them keep working. Memberships and grants to the team are removed.

### `PUT /api/org/teams/{id}/members`

`{"members": [{"user_id": 24, "role": "lead"}, {"user_id": 31}]}` replaces who is in the team. Users outside the organisation are ignored. Returns the team with its members.

### `GET /api/org/files/{id}/grants`

`{"users": [{"id": 24, "username": "jane"}], "teams": [{"id": 3, "name": "Sales", "color": "#e11d48"}]}`: the users and teams a file has been granted to, beyond everyone who sees it anyway. Shared files can't have grants (`400`).

### `PUT /api/org/files/{id}/grants`

`{"user_ids": [24, 31], "team_ids": [3]}` replaces either list (`[]` revokes all; a list that isn't sent is left alone). IDs outside the organisation are ignored. Returns the new lists. A card that already uses a file keeps showing it after access is revoked, until someone removes it from the card.

---

## Integrations 🔒

Connections to outside services: lead sync, booking pages, SCIM and SAML. The [integrations overview](../docs/integrations/overview.md) explains how they behave; this section is the API the Integrations page uses. Everything here needs a session, a verified email and a chosen password. Members manage only their own (personal) connections; owners and admins also manage the organisation's. A connection someone can't manage answers `404`, as if it didn't exist.

**Errors** in this section:

| Status | When |
| ------ | ---- |
| `400` | Invalid settings: `{"error":"Payload URL: the URL must use https","field":"url"}`. `field` names the setting at fault, when there is one. Also other mistakes the user can fix (`"fill in Client ID first"`) |
| `403` | `"you can't manage this connection"`: a member creating an organisation connection, or listing `?scope=org` |
| `404` | `"connection not found"`, or `"unknown integration"` for a provider id the server doesn't have |
| `409` | `"you already have a connection to this integration"`, or the category's own message (one directory and one SSO connection per organisation; booking pages have no limit) |
| `422` | `"this integration isn't available on this server"` (coming soon, or the site's address / `SECRETS_KEY` missing), `"finish setting up this connection first"` (testing a pending connection), or `"SECRETS_KEY isn't set on this server, so secrets can't be stored"` |

### Manifest

How a provider describes itself. The catalog serves it, and the page builds the settings form from `fields`.

```json
{
  "id": "webhook",
  "name": "Webhook",
  "category": "lead_sync",
  "description": "Send each new lead as JSON to any URL …",
  "scopes": ["org", "user"],
  "auth": "none",
  "status": "available",
  "multiple": true,
  "fields": [
    { "key": "url", "label": "Payload URL", "type": "url", "required": true, "placeholder": "https://example.com/hooks/fronko", "help": "…" },
    { "key": "signing_secret", "label": "Signing secret", "type": "secret", "generate": true, "help": "…" }
  ],
  "setup_steps": ["…"],
  "docs_url": "https://github.com/faiz-gh/fronko/blob/master/docs/integrations/webhook-zapier.md",
  "requires": ["secrets_key"],
  "keywords": ["zapier", "make", "n8n"]
}
```

| Field | Values |
| ----- | ------ |
| `category` | `lead_sync`, `calendar`, `directory`, `sso` |
| `scopes` | Who may connect it: `org` (admins, for the organisation) and/or `user` (anyone, for themselves). The first is the default |
| `auth` | `none`, `api_key`, `oauth2` (the organisation's own OAuth app), `link`, `saml`, `scim_token` |
| `status` | `available`, `beta`, `coming_soon` |
| `multiple` | More than one connection per owner. Categories `calendar`, `directory` and `sso` allow one per owner regardless |
| `fields[].type` | `text`, `url` (absolute `https`, public host), `secret` (write-only), `select` (with `options: [{value, label}]`), `textarea`, `bool`. Optional: `required`, `help`, `placeholder`, `default`, `generate` (offer a random value), `max_length` |
| `requires` | `public_url`, `secrets_key`: server settings it needs |

### Connection

```json
{
  "id": 7,
  "provider": "webhook",
  "category": "lead_sync",
  "scope": "org",
  "name": "Webhook",
  "enabled": true,
  "status": "active",
  "config": { "url": "https://example.com/hooks/fronko" },
  "secrets": { "signing_secret": true },
  "authorized": false,
  "missing": [],
  "endpoints": [],
  "token": null,
  "last_error": null,
  "last_error_at": null,
  "failure_count": 0,
  "last_synced_at": "2026-10-08T09:20:03Z",
  "created_by": { "id": 1, "username": "faiz" },
  "created_at": "2026-10-07T18:02:11Z",
  "updated_at": "2026-10-08T09:20:03Z"
}
```

| Field | Meaning |
| ----- | ------- |
| `scope` | `org` or `user` (a personal connection, belonging to whoever created it) |
| `status` | `pending` (a required setting is missing, an OAuth connection isn't authorised, or a SCIM connection has no token), `active`, or `error` (three final failures in a row; saving settings or switching it off and on resets it) |
| `enabled` | `false` while paused. Paused and `error` connections get no leads, and missed leads aren't replayed |
| `config` | The non-secret settings |
| `secrets` | For each `secret` field, whether it has a value. Secret values are never returned |
| `authorized` | OAuth connections: whether there's a token |
| `missing` | Keys of required settings still empty |
| `used_by_cards` | Booking pages only: how many cards show it |
| `endpoints` | Values to enter in the provider: `[{key, label, value, help?}]`, such as SAML's ACS URL or SCIM's tenant URL. Empty without `PUBLIC_URL` |
| `token` | `scim_token` connections: `{hint, created_at, last_used_at}` for the current token (its last 4 characters), or `null` |
| `last_error`, `failure_count`, `last_synced_at` | Health: the latest failure that retrying didn't fix, failures in a row, and the last successful delivery |

### `GET /api/integrations/catalog`

Every provider the user may connect, grouped by category, with their connections for the status badges. Members only get providers with the `user` scope.

```json
{
  "categories": [
    { "id": "lead_sync", "label": "Lead Sync", "description": "…", "single": false },
    { "id": "calendar", "label": "Calendar Booking", "description": "…", "single": true }
  ],
  "providers": [
    {
      "id": "webhook", "name": "Webhook", "category": "lead_sync", "…": "the rest of the Manifest",
      "unavailable": "",
      "testable": true,
      "connections": [{ "id": 7, "name": "Webhook", "scope": "org", "status": "active", "enabled": true }]
    }
  ],
  "oauth_redirect_url": "https://cards.example.com/api/integrations/oauth/callback"
}
```

`unavailable` says why a provider can't be connected here (`"Coming soon"`, `"Needs FRONTEND_URL (or PUBLIC_URL) to be set on the server"`, `"Needs SECRETS_KEY to be set on the server"`), or is empty. `testable` means its connections have a Test or Send test lead action. `oauth_redirect_url` is what to register in an OAuth app, or `""` without `PUBLIC_URL`.

### `GET /api/integrations/connections`

The connections the user manages: the organisation's (admins) and their own. Optional query: `scope=org|me` and `provider=<id>`. Returns an array of [`Connection`](#connection).

### `POST /api/integrations/connections`

```json
{
  "provider": "webhook",
  "scope": "user",
  "name": "My Zap",
  "config": { "url": "https://hooks.zapier.com/hooks/catch/123/abc/" },
  "secrets": { "signing_secret": "…" }
}
```

`scope` defaults to the manifest's first scope; `org` needs an admin. `name` defaults to the provider's name (at most 80 characters). Settings are checked against the manifest's fields, then by the provider. Some providers prepare the connection here (SAML makes its key pair). The connection starts `pending` until it has everything it needs.

| Status | Body |
| ------ | ---- |
| `201` | [`Connection`](#connection) |
| `400`, `403`, `404`, `409`, `422` | See the error table above |

### `GET /api/integrations/connections/{id}`

One [`Connection`](#connection).

### `PATCH /api/integrations/connections/{id}`

```json
{ "name": "Sales Zap", "enabled": false, "config": { "url": "…" }, "secrets": { "signing_secret": "…" } }
```

Every field is optional. `config` and `secrets` change only the keys given; a secret sent as `""` (or left out) keeps its value. Changing an OAuth app's `client_id` or `client_secret` drops its token, so it has to be authorised again. Saving settings or switching a connection back on gives one in `error` a fresh start. Returns the [`Connection`](#connection).

### `DELETE /api/integrations/connections/{id}`

Deletes the connection, its secrets, its tokens and its activity log. `204 No Content`.

### `POST /api/integrations/connections/{id}/test`

Lead sync connections receive a sample lead (Test Lead, `test.lead@example.com`, `"test": true` in webhook payloads) through the real delivery path. Other providers run their own check (a booking page opens; SAML metadata loads). The outcome is logged. Rate limited: 5 at once per IP, then 1 every 10 seconds.

| Status | Body |
| ------ | ---- |
| `200` | `{"ok": true, "summary": "Delivered to example.com (200)", "detail": {"status": 200, "delivery": "test-…", "duration_ms": 182}}`. A failed test is still `200`, with `"ok": false` and the reason in `summary` |
| `400` | `"this integration has nothing to test"` |
| `422` | `"finish setting up this connection first"` |

### `GET /api/integrations/connections/{id}/activity`

The connection's log, newest first. Query: `limit` (1–100, default 25) and `before` (an activity id, for the next page). Entries are kept for 90 days.

```json
[
  {
    "id": 412,
    "kind": "push_lead",
    "outcome": "retrying",
    "summary": "the receiver at example.com answered 503",
    "detail": { "status": 503, "delivery": "lead-1042-7", "response": "Service Unavailable" },
    "lead_id": 1042,
    "attempt": 1,
    "user": null,
    "created_at": "2026-10-08T09:20:01Z"
  }
]
```

`kind` is `push_lead`, `test`, `setup` (created, settings changed, authorised, token generated), `provision` (SCIM) or `sign_in` (SSO). `outcome` is `success`, `retrying` (will be tried again) or `failed`. `user` is who did it, for actions people take.

### `POST /api/integrations/connections/{id}/tokens`

For `scim_token` connections: generates the bearer token the identity provider calls [SCIM](#scim-20) with, revoking any earlier one at once. The response (`Cache-Control: no-store`) is the only time the token is shown. Shares the auth rate limit.

```json
{ "token": "fronko_scim_…", "connection": { "…": "the Connection, with its new token hint" } }
```

### `GET /api/integrations/connections/{id}/oauth/start`

For `oauth2` connections. The browser **navigates** here (it's not an XHR): Fronko sets a short-lived `fronko_oauth` cookie and redirects to the provider's authorisation page with a signed `state`. The connection needs its client ID and secret first, and the server needs `PUBLIC_URL`. On a problem it redirects back to the provider's page with `?oauth_error=<message>`.

### `GET /api/integrations/oauth/callback`

Where providers send the browser back (`state`, and `code` or `error`). Fronko checks the state's signature, expiry, user and the cookie's nonce, exchanges the code with the connection's own client ID and secret, and seals the token. It then redirects to `/dashboard/integrations/{provider}?connection={id}` with `&oauth=connected` or `&oauth_error=<message>`. The callback is on the API's address and the final redirect on the site's (`PUBLIC_URL`), so this works when the API has its own domain. Register the catalog's `oauth_redirect_url` (`/api/integrations/oauth/callback` on the API's address) in the OAuth app.

### `GET /api/org/domains` 🛡️

The organisation's email domains:

```json
[
  {
    "id": 3,
    "domain": "acme.com",
    "verified_at": null,
    "created_at": "2026-10-08T10:00:00Z",
    "txt_name": "acme.com",
    "txt_value": "fronko-verification=k3j5…"
  }
]
```

A verified domain sends sign-ins by email to the organisation's single sign-on, and lets SSO and SCIM mark addresses on it as verified.

### `POST /api/org/domains` 🛡️

`{"domain": "acme.com"}`. An email address or URL is accepted too; the domain is taken from it. At most 20 per organisation.

| Status | Body |
| ------ | ---- |
| `201` | The domain, with the TXT record to create |
| `400` | `"enter a domain such as example.com"`, `"an organisation can have at most 20 domains"` |
| `409` | `"you've already added this domain"` |

### `POST /api/org/domains/{id}/verify` 🛡️

Looks up the TXT record. Shares the auth rate limit.

| Status | Body |
| ------ | ---- |
| `200` | The domain, now with `verified_at` |
| `400` | `"the TXT record wasn't found yet; DNS changes can take a while to appear, so try again later"` |
| `409` | `"another organisation has already verified this domain"` |

### `DELETE /api/org/domains/{id}` 🛡️

`204 No Content`.

## Single sign-on

SAML 2.0 sign-in, with Fronko as the service provider. See [SAML single sign-on](../docs/integrations/saml.md) for the behaviour. These routes only work for organisations with an enabled, active SSO connection.

### `POST /auth/sso/discover`

```json
{ "identifier": "ada@acme.com" }
```

Finds where to sign in: `identifier` is an email on a verified domain, or an organisation handle. Shares the auth rate limit.

| Status | Body |
| ------ | ---- |
| `200` | `{"url": "/auth/sso/acme", "handle": "acme"}` |
| `404` | `"we couldn't find single sign-on for that; check the spelling, or sign in with your password"` |

### `GET /auth/sso/{handle}`

The browser navigates here to start signing in. Optional `return_to` (a path on this site; default `/dashboard`). Fronko sets a signed `fronko_sso` cookie naming the request (10 minutes) and redirects to the identity provider (or auto-posts to it, for POST-only providers). Problems redirect to `/login?sso_error=<message>`.

### `GET /auth/saml/{connection id}/metadata`

Fronko's service provider metadata for that connection (`application/samlmetadata+xml`): entity ID, ACS URL and certificate. Its URL is also the entity ID.

### `POST /auth/saml/{connection id}/acs`

The identity provider posts its response here (`SAMLResponse`, form-encoded, up to 1 MiB). It skips the same-origin check, since the identity provider posts from its own origin, and shares the auth rate limit. Fronko checks the signature, audience, destination and timing, and for SP-initiated sign-in that it answers the request in the cookie. It then finds the person by email (creating them if allowed), sets the `fronko_session` cookie and redirects to `return_to`. Refusals redirect to `/login?sso_error=<message>` and are logged on the connection.

## SCIM 2.0

A SCIM 2.0 server ([RFC 7643](https://www.rfc-editor.org/rfc/rfc7643), [RFC 7644](https://www.rfc-editor.org/rfc/rfc7644)) for identity providers to provision people and teams. Setup: [Microsoft Entra ID](../docs/integrations/entra-scim.md).

- **Base URL:** `/scim/v2` on the API's address (`PUBLIC_API_URL`, which is `PUBLIC_URL` unless the API has its own domain).
- **Auth:** `Authorization: Bearer fronko_scim_…`, the token of the organisation's directory connection ([`POST …/tokens`](#post-apiintegrationsconnectionsidtokens)). A missing, revoked or paused token gets `401`; a suspended organisation `403`.
- **Content type:** `application/scim+json` (plain `application/json` is accepted).
- **Errors:** SCIM error bodies, `{"schemas":["urn:ietf:params:scim:api:messages:2.0:Error"],"status":"409","scimType":"uniqueness","detail":"…"}`. Refused writes are logged on the connection.
- **Rate limit:** 200 at once per IP, then 1 every 20 ms.

| Method | Path | Notes |
| ------ | ---- | ----- |
| `GET` | `/ServiceProviderConfig`, `/ResourceTypes`, `/Schemas` | Discovery. PATCH and filtering supported; bulk, sort, ETags and password changes aren't |
| `GET` | `/Users` | `filter` with `eq` on `userName`, `externalId`, `emails`, `emails.value` or `id`; `startIndex`, `count` (up to 200) |
| `POST` | `/Users` | Creates a member. Needs `userName` and an email (`emails`, or a `userName` that is an email). The email must not belong to another organisation. Emails a temporary password, or an SSO invitation when the organisation has SSO (then there's no password). `active: false` creates them suspended |
| `GET` | `/Users/{id}` | |
| `PUT` | `/Users/{id}` | Replaces `userName`, emails, names, `externalId` and `active` |
| `PATCH` | `/Users/{id}` | `add`, `replace`, `remove` operations, with or without paths (Entra's and Okta's styles). Unknown attributes are ignored |
| `DELETE` | `/Users/{id}` | Deletes the account, keeping their cards, files and leads (as when an admin deletes someone) |
| `GET` | `/Groups` | `filter` with `eq` on `displayName`, `externalId` or `id`; `excludedAttributes=members` |
| `POST` | `/Groups` | Creates a team, with `members: [{value: <user id>}]` |
| `GET` | `/Groups/{id}` | |
| `PUT` | `/Groups/{id}` | Replaces the name and members |
| `PATCH` | `/Groups/{id}` | Renames, and adds or removes members |
| `DELETE` | `/Groups/{id}` | Deletes the team; its files move to the organisation |

`active: false` suspends a person (signing them out); `true` restores them. The organisation's owner can't be deactivated or deleted (`400`, `scimType: "mutability"`). Addresses on a [verified domain](#get-apiorgdomains-) are marked verified. User and group `id`s are Fronko's numeric ids, as strings.

## Public

### `GET /api/profiles/{org}/{slug}`

Looks up a profile for the public card page (`/p/{org}/{slug}`): `org` is the organisation's handle and `slug` the card's. Both match case-insensitively.

| Status | Body |
| ------ | ---- |
| `200` | [`PublicProfile`](#publicprofile-visitor-view) |
| `404` | `"profile not found"` |
| `410` | `{"error":"this card is unavailable","code":"org_suspended"}` while its organisation is suspended. Visitors aren't told why |

### `GET /api/profiles/{org}/{slug}/vcard`

Optional query: `via` (`nfc`, `qr` or `link`) and `s` (the visit's id from the public page's tracker). Each download counts as a contact save in [analytics](#analytics-), tied to that visit. Bots, and people signed in to the card's own organisation, aren't counted.

The card as a vCard 3.0 file (`text/vcard; charset=utf-8`, `Content-Disposition: inline; filename="{slug}.vcf"`, not cached). Opening it in a phone browser shows the "Add contact" sheet: the public page's **Save contact** button links here, and the page navigates here itself when an NFC tap or QR scan is set to save the contact.

It includes the name (split into given and family name on the last space), company, title, email, mobile (`phone_country_code` + `phone_number`), website (only if it's a valid http(s) URL), location as the work address, bio as the note, and a link back to the card. That link uses the page's origin when the `Referer` is one of `CORS_ALLOWED_ORIGINS`, and otherwise this request's own origin (`X-Forwarded-Proto` and `Host` from the proxy).

| Status | Body |
| ------ | ---- |
| `200` | The vCard |
| `404` | `"profile not found"` |
| `410` | `"this card is unavailable"` while its organisation is suspended |

### `GET /api/files/{id}`

Serves a library file to anyone who has its ID: on cards, the photo and brochures. It answers **`302`** to a presigned URL for the object in the owner's bucket.
- The signed URL is valid for 15 minutes.
- The response has `Content-Type` and `Content-Disposition: inline; filename=…` set.
- The redirect is sent with `Cache-Control: private, max-age=300` and `Referrer-Policy: no-referrer`.

`?size=thumb` redirects to the file's small preview instead, or to the file itself when it has no preview.

Unknown IDs, deleted files, files whose owner disconnected storage, and files of a suspended organisation all return `404`.

### `POST /api/profiles/{id}/leads`

A visitor shares their details with the profile owner. The path takes the numeric profile **ID**, which the frontend gets from `GET /api/profiles/{org}/{slug}`.

**Request**

```json
{
  "name": "Jane Doe",
  "email": "jane@example.com",
  "phone_country_code": "+91",
  "phone_number": "98765 43210",
  "notes": "Great talk!",
  "source": "qr",
  "session": "5f0c…uuid"
}
```

| Field | Rules (all values are trimmed first) |
| ----- | ------------------------------------ |
| `name` | Required, 1–120 bytes |
| `email` | Required, at most 254 bytes. Must be a bare address: `Jane <jane@x.com>` is rejected |
| `phone_country_code`, `phone_number` | Optional, but both or neither. Spaces, `-`, `.` and parentheses are stripped. The dial code must be `+` and 1–3 digits (`+` is added if missing). The number must be 4–14 digits, with at most 15 digits in total (E.164). They are stored without separators |
| `notes` | Optional, at most 2000 bytes |
| `source`, `session` | Optional. How the visitor reached the card (`nfc`, `qr`, anything else counts as `link`), stored on the lead. The visit id puts the send on that visit's timeline in analytics |

| Status | Body |
| ------ | ---- |
| `201` | `{"status": "ok"}` |
| `400` | `"please enter your name"`, `"please enter a valid email address"`, `"please enter a valid mobile number with its country code"` or `"message is too long"` |
| `404` | `"profile not found"`, also while the card's organisation is suspended |
| `429` | Rate limited |

The server accepts leads even when the card's `data.collect_leads` is `false`. That flag only hides the form in the UI.

### `POST /api/profiles/{org}/{slug}/events`

The public card's tracker reports what a visitor does. It sends batches with `navigator.sendBeacon`, so the body is JSON sent as `text/plain` (any content type is accepted). The session cookie, if any, comes along because the request is same-origin.

```json
{
  "session": "5f0c2b4e-8d1a-4c3e-9b7f-2a6d1e0c9f11",
  "source": "qr",
  "events": [
    { "type": "view" },
    { "type": "scroll", "value": 50 },
    { "type": "click", "target": "https://linkedin.com/in/jane", "label": "LinkedIn" },
    { "type": "doc_open", "target": "k3j9x2m4p8q1", "label": "Company brochure" },
    { "type": "leave", "value": 42000 }
  ]
}
```

| Field | Rules |
| ----- | ----- |
| `session` | A UUID made for this page load and kept only in the tab's memory. Anything else is stored as no session |
| `source` | `nfc` or `qr` (from the `?via=` marker); anything else is `link` |
| `events` | Up to 20; extra events are dropped. `type` is `view`, `click`, `scroll`, `doc_open`, `gallery_open`, `form_open`, `share` or `leave`. Other types are dropped, including `vcard` and `form_submit`, which only the server records |
| `target`, `label` | Trimmed and cut to 300 and 120 characters |
| `value` | `scroll`: must be 25, 50, 75 or 100. `leave`: time on card in ms, clamped to 0–3,600,000. Ignored for other types |

It always answers **`204`** with no body, whether or not anything was stored, so it reveals nothing about the card. The only exception is `400` for a body that isn't JSON. Nothing is stored when:
- no card has that link, or its organisation is suspended;
- the user agent is empty or looks like a bot, crawler, link previewer or script;
- the request carries a valid session for a user in the card's own organisation, so previews don't count;
- no events are valid.

**Privacy.** No IP address is stored. Each event gets `device` (`mobile`, `tablet` or `desktop`, from the user agent), the referring site's host when it's another site, and a visitor hash: SHA-256 of a random daily salt, the client IP, the user agent and the card id. Salts live in `analytics_salts` and are deleted after a day, so a hash can't be linked back to an address, to the same person on another day, or to the same person on another card. Events are deleted after `ANALYTICS_RETENTION_DAYS` (default 395). Requests with `Sec-GPC: 1` (Global Privacy Control) or `DNT: 1` aren't recorded at all, and the web app doesn't send them.

| Status | Body |
| ------ | ---- |
| `204` | Nothing |
| `400` | `"invalid request body"` |
| `429` | Rate limited (`events` budget) |

---

## Feedback 🔒

### `POST /api/me/feedback`

Any signed-in user (verified, with a password they chose) sends feedback about Fronko to the platform admins. Their email and organisation name are stored with it, so admins can reply. When `FEEDBACK_NOTIFY_EMAIL` is set, it's also emailed there, with the sender as Reply-To.

```json
{ "category": "idea", "rating": 4, "message": "Could leads export to CSV?", "page_path": "/dashboard/leads" }
```

| Field | Rules |
| ----- | ----- |
| `category` | Required: `bug`, `idea` or `other` |
| `rating` | Optional: 1–5, or `null` |
| `message` | Required, 1–5000 characters after trimming |
| `page_path` | Optional context: the app page they were on. Dropped unless it starts with `/`, is at most 200 bytes and has no control characters |

| Status | Body |
| ------ | ---- |
| `201` | `{"id": 12}` |
| `400` | `"choose bug, idea or other"`, `"rating must be from 1 to 5"`, `"write a message"` or `"message must be at most 5000 characters"` |
| `429` | Rate limited (feedback bucket) |

---

## Platform admin 🖥️

For whoever runs the server. Admin accounts are separate from organisation users and are created on the command line (`./fronko admin create --email …`; see the [backend README](README.md#platform-admin)). These endpoints return **aggregates only**: no card data, leads, file names or member details. The owner's email and the feedback sender's email are the only personal data.

### `POST /auth/admin/login`

`{"email": "you@example.com", "password": "…"}`. On success: `200` with a [`PlatformAdmin`](#platformadmin) and the `fronko_admin` cookie (HttpOnly, `SameSite=Strict`, 8 hours). Wrong email or password: `401 "invalid email or password"`, with similar timing either way. Shares the **auth** rate limit.

### `POST /auth/admin/logout`

Clears the admin cookie. `204`.

### `GET /api/admin/me`

The signed-in [`PlatformAdmin`](#platformadmin). `401` without a valid admin cookie (`"not signed in"`, `"session expired, please sign in again"` or `"account no longer exists"`).

#### PlatformAdmin

```json
{ "id": 1, "email": "you@example.com", "last_login_at": "2026-10-06T15:35:45Z", "created_at": "2026-10-01T09:00:00Z" }
```

### `GET /api/admin/summary`

```json
{
  "org_count": 10, "suspended_org_count": 1, "new_orgs_30d": 4, "active_orgs_30d": 6,
  "user_count": 31, "card_count": 18, "lead_count": 420, "file_count": 57,
  "orgs_with_storage": 5, "storage_used_bytes": 734003200, "new_feedback": 2,
  "team_count": 7, "orgs_with_teams": 3, "orgs_with_logo": 6
}
```

`active_orgs_30d` counts organisations where someone signed in within 30 days. `new_feedback` counts feedback with status `new`. `team_count` totals every organisation's teams; `orgs_with_teams` and `orgs_with_logo` count organisations with at least one team, or with a logo set.

### `GET /api/admin/trends`

`?days=` 1–366 (default 30). Returns `{"days": 30, "points": [UsagePoint…]}`, oldest first. One point per day with a snapshot (UTC): `date`, `org_count`, `user_count`, `card_count`, `lead_count`, `file_count`, `orgs_with_storage`, `storage_used_bytes`, `new_orgs` and `feedback_count` (both received that day), `team_count`, `orgs_with_teams` and `orgs_with_logo`. The last three read 0 for days before migration 011. Snapshots are taken at startup and hourly, so today's point is at most an hour old. Days the server was down are missing.

### `GET /api/admin/orgs`

| Query | Meaning |
| ----- | ------- |
| `q` | Case-insensitive substring of the name or owner email (`%` and `_` match literally) |
| `status` | `active`, `suspended`, or empty for both |
| `sort` | `newest` (default), `oldest`, `name`, `last_active`, `users`, `teams`, `cards`, `leads` or `storage` |
| `page`, `page_size` | 1-based; `page_size` defaults to 25, at most 100 |

Returns `{"items": [OrgUsage…], "total": 10, "page": 1, "page_size": 25}`. An unknown `status` or `sort` is a `400`.

#### OrgUsage

```json
{
  "id": 9, "name": "Acme", "created_at": "2026-10-04T20:12:32Z",
  "owner_email": "owner@acme.example", "suspended_at": null,
  "user_count": 3, "admin_count": 1, "member_count": 1, "suspended_user_count": 0,
  "card_count": 3, "lead_count": 68, "file_count": 8, "storage_used_bytes": 704376,
  "storage_connected": true, "storage_verified": true, "storage_provider": "r2",
  "default_quota_bytes": null, "last_active_at": "2026-10-06T12:55:10Z",
  "team_count": 1, "logo_set": true, "logo_policy": "optional", "signature_locked": true,
  "files_by_purpose": { "brochure": 6, "logo": 2, "banner": 1 }
}
```

`team_count`, `logo_set`, `logo_policy`, `signature_locked` and `files_by_purpose` are counts and yes/no settings only: team names, file names and the logo itself are never returned.

`user_count` includes the owner. `storage_used_bytes` sums files uploaded through Fronko, not everything in the bucket. `storage_provider` is the type only (`r2`, `b2`, `s3`, `minio`, `other`); the bucket, endpoint and keys are never returned. `owner_email` is `null` for old accounts without one. `last_active_at` is the latest sign-in of anyone in the organisation.

### `GET /api/admin/orgs/{id}`

One [`OrgUsage`](#orgusage), plus `suspended_reason` while suspended. `404 "organisation not found"`.

### `GET /api/admin/orgs/{id}/trends`

Like [`/api/admin/trends`](#get-apiadmintrends) for one organisation. Points have `date`, `user_count`, `card_count`, `lead_count`, `file_count`, `storage_used_bytes` and `team_count`.

### `POST /api/admin/orgs/{id}/suspend`

`{"reason": "Spam cards reported by visitors"}`. The reason is required: 1–500 characters on one line. It's emailed to the owner and shown to members when they try to sign in.

Suspending signs out everyone in the organisation and blocks sign-in (`org_suspended`). Its public cards answer `410`, and its leads and public files are refused. Nothing is deleted.

| Status | Body |
| ------ | ---- |
| `200` | The updated [`OrgUsage`](#orgusage) |
| `400` | `"give a reason of 1-500 characters on one line"` |
| `404` | `"organisation not found"` |
| `409` | `"this organisation is already suspended"` |

### `POST /api/admin/orgs/{id}/reinstate`

No body. Lifts the suspension and emails the owner. `200` with the updated [`OrgUsage`](#orgusage), `404`, or `409 "this organisation isn't suspended"`. Members sign in again; sessions ended by the suspension stay ended.

### `GET /api/admin/feedback`

`?status=` `new`, `read`, `resolved` or empty for all; `page`, `page_size` (default 25, at most 100). Returns newest first:

```json
{
  "items": [{
    "id": 1, "org_id": 9, "sender_email": "rep@example.com", "org_name": "Acme",
    "category": "idea", "rating": 4, "message": "Could leads export to CSV?",
    "page_path": "/dashboard/leads", "status": "new", "reply_count": 0,
    "created_at": "2026-10-06T15:36:09Z", "updated_at": "2026-10-06T15:36:09Z"
  }],
  "total": 1, "page": 1, "page_size": 25,
  "counts": { "new": 1, "read": 0, "resolved": 0 }
}
```

`org_id` becomes `null` if the organisation is deleted; `org_name` and `sender_email` are copies, so they stay.

### `GET /api/admin/feedback/{id}`

The feedback with `replies`, oldest first: `[{"id", "admin_email", "body", "email_sent", "created_at"}]`. `admin_email` is `null` if that admin was deleted. `404 "feedback not found"`.

### `PATCH /api/admin/feedback/{id}`

`{"status": "resolved"}` (`new`, `read` or `resolved`). Returns the feedback with its replies. Changes are audited.

### `POST /api/admin/feedback/{id}/replies`

`{"body": "…"}`, 1–5000 characters. Emails the reply to `sender_email` with their message quoted (Reply-To: `FEEDBACK_NOTIFY_EMAIL` when set), then stores it. `new` feedback becomes `read`. The reply is kept even if the email fails; check `email_sent` on the last reply. Returns the feedback with its replies.

### `GET /api/admin/audit`

`?page=`, `page_size` (default 50, at most 200). Returns `{"items": [AuditEntry…], "total", "page", "page_size"}`, newest first:

```json
{ "id": 3, "admin_email": "you@example.com", "action": "org.suspend", "target_type": "org", "target_id": 9,
  "detail": { "org_name": "Acme", "reason": "Spam cards", "email_sent": true }, "created_at": "2026-10-06T15:36:24Z" }
```

Actions: `admin.login`, `org.suspend`, `org.reinstate` (`detail`: `org_name`, `email_sent`), `feedback.reply` (`email_sent`) and `feedback.status` (`from`, `to`).

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

# Public lookup: the organisation's handle ("demo", from /api/me/user's org_handle), then the slug
curl -s $BASE/api/profiles/demo/demo-card

# Submit a lead (use the id from the previous response)
curl -s -X POST $BASE/api/profiles/1/leads \
  -H 'Content-Type: application/json' \
  -d '{"name":"Jane","email":"jane@example.com","notes":"Hi!"}'

# Read leads as the owner, 10 per page
curl -s -b $JAR "$BASE/api/me/leads?profile_id=1&page_size=10"
```

With `COOKIE_SECURE=true` (the default), curl still stores the cookie but only sends it back over HTTPS. Run the backend with `COOKIE_SECURE=false` when testing locally over HTTP.
