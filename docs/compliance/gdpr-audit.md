# GDPR audit

**Date:** 2026-10-08 · **Scope:** the whole product. That covers the landing page, sign-up and sign-in, the dashboard, public cards (`/p/{org}/{slug}`), the platform admin panel (`/admin`), the backend API, background jobs, the database schema, the deployment files and the docs.

This is an engineering audit, not legal advice. It records:

- what personal data Fronko holds, and why;
- how long it's kept and who it goes to;
- what was fixed in this pass;
- what whoever runs a Fronko server still has to do.

Have a lawyer review it before relying on it.

## Summary

Fronko's design was already privacy-friendly in several ways:

- It loads no third-party scripts, fonts or analytics.
- Card analytics are cookieless, with a daily-rotating salted hash and no stored IP addresses.
- Secrets are encrypted at rest, and sessions are HttpOnly cookies.
- Platform admins see aggregates only.

The gaps were in data subject rights, transparency, retention and a few leaks into logs and third parties.

**This pass fixed 22 findings:**

- **Self-service rights:** self-service account deletion (with ownership transfer and organisation deletion), and a personal data export.
- **Transparency:** accurate lead form wording, privacy notice links, and a Do Not Track / Global Privacy Control opt-out.
- **Retention:** automatic retention for leads, codes, feedback and the audit log.
- **Leaks:** bodies of unsent emails kept out of logs.
- **Security:** security headers, IP-free proxy logs, a stronger password hash, and noindex on cards by default.

By product decision, the public card and lead form get no consent boxes or other extra steps for visitors (findings 3 and 6). That moves the weight onto each organisation's privacy notice.

**What's left is mostly paperwork:** a privacy policy, terms, data processing agreements and a breach runbook. See [Open items](#open-items-for-the-operator).

## Roles

| Data | Controller | Processor |
| ---- | ---------- | --------- |
| Accounts of people using Fronko (users, owners, admins) | The operator of the Fronko server | — |
| Card contents, leads, files, analytics of an organisation | The **organisation** (Fronko's customer) | The operator, on the organisation's behalf |
| Leads pushed to an organisation's CRM or webhook | The organisation | The CRM or automation tool, chosen by the organisation |
| Platform feedback and the admin audit log | The operator | — |

On a self-hosted server, the operator and the organisation may be the same company.

## Personal data inventory

The data subjects are:

- **U:** users (people with accounts)
- **V:** visitors to public cards
- **L:** leads (visitors who sent their details)
- **A:** platform admins

| Data | Where | Subjects | Purpose | Legal basis | Retention |
| ---- | ----- | :------: | ------- | ----------- | --------- |
| Username, email, name, role, password hash, last sign-in | `users` | U | Running the account | Contract | Until the account is deleted |
| SSO / SCIM identifiers (`external_id`, `external_username`) | `users` | U | Matching people to the organisation's identity provider | Contract (organisation) | Until the account is deleted |
| One-time codes (HMAC only), target address | `email_codes` | U | Verification, reset, email change, account deletion | Contract | 15 minutes; swept hourly |
| Card contents (name, title, phone, email, links, photo, bio) | `profiles.data` | U | Publishing the card the person made | Contract | Until the card or organisation is deleted |
| Files: name, title, uploader (the content is in the organisation's bucket) | `files` | U | File library | Contract | Until deleted |
| Lead name, email, phone, message, source | `leads` | L | Letting the card holder get back to the visitor, who asked to be contacted | Steps at the visitor's request (Art. 6(1)(b)) / legitimate interest (Art. 6(1)(f)); the organisation's privacy notice is linked from the form | The organisation's `lead_retention_days`, or until deleted |
| Visit events: device class, referrer host, clicked link, daily visitor hash | `card_events` | V | Card analytics for the organisation | Legitimate interest (aggregate, cookieless; opt-out honoured) | `ANALYTICS_RETENTION_DAYS` (395) |
| Daily hash salt | `analytics_salts` | — | Making the visitor hash unlinkable across days | — | Deleted after 1–2 days |
| Integration activity (e.g. "Sent jane@…") | `integration_activity` | L, U | Troubleshooting lead sync and sign-in | Legitimate interest | 90 days; deleted with the lead or user |
| Integration secrets (OAuth tokens, API keys) | `integration_connections.secrets` (AES-256-GCM) | — | Calling the provider | Contract | Until the connection is removed |
| Bucket keys | `user_storage` (AES-256-GCM) | — | Storing files | Contract | Until removed |
| Feedback: sender email, organisation name, message, page path (without query) | `feedback`, `feedback_replies` | U | Product support | Legitimate interest | 2 years; email cleared when the account is deleted |
| Platform admin email, password hash | `platform_admins` | A | Admin sign-in | Legitimate interest | Until removed |
| Admin actions (with org name, suspension reason) | `admin_audit_log` | A, U | Accountability | Legitimate interest | 2 years; org details cleared when the org is deleted |
| Usage snapshots | `org_usage_snapshots`, `platform_usage_snapshots` | — | Trends (counts only) | — | Indefinite (not personal data) |
| Client IP | Memory only (rate limiter) | U, V, L | Abuse prevention | Legitimate interest | Evicted after 10 minutes idle; never stored |
| Proxy access log | nginx stdout | — | Operations | Legitimate interest | Host log retention. No IP, user agent or query string is logged |

## Retention

All retention runs as scheduled tasks in the backend (`app.Scheduled`), each holding a Postgres advisory lock.

| Task | Every | Deletes | Code |
| ---- | ----- | ------- | ---- |
| `expired email codes` | 1 h | Codes past `expires_at` | `account/module.go` |
| `lead retention` | 6 h | Leads older than the organisation's `lead_retention_days` (30–3650; unset keeps them), with their integration activity | `leads/module.go` |
| `analytics retention` | 1 h | Events older than `ANALYTICS_RETENTION_DAYS`, old salts | `analytics/module.go` |
| `admin retention` | 24 h | Feedback and audit log entries older than 2 years | `platformadmin/module.go` |
| integration activity | — | Activity older than 90 days | `integrations/module.go` |
| `jobs: delete finished` | 1 h | Finished jobs older than 30 days | `platform/jobs/tasks.go` |

## Data subject rights

| Right | How |
| ----- | --- |
| **Access, portability** (Art. 15, 20) | Settings → Account → **Download my data** (`GET /api/me/export`). This is a JSON file with the person's account, organisation, teams, cards, the leads they collected, file details, personal integrations and feedback. |
| **Rectification** (Art. 16) | People edit their own cards. Owners change their email in Settings. Admins correct members' names and emails. |
| **Erasure, user** (Art. 17) | Settings → Account → **Danger zone → Delete account** (`DELETE /api/me/account`). It is immediate and needs the password (or an emailed code for SSO-only accounts) plus the username typed out. See below for what happens to the person's things. |
| **Erasure, lead** | Admins delete a lead from its detail sheet, or many at once from the Leads table (`POST /api/me/leads/delete`). The lead's integration log lines go with it. Copies already pushed to a CRM must be deleted there; the dialog says so. |
| **Erasure, visitor** | Nothing to erase: visits aren't linked to a person, and the hash salt is gone after a day. |
| **Objection** (Art. 21) | Card analytics skip any browser that sends `Sec-GPC: 1` or `DNT: 1`. The card footer tells visitors this. |
| **Erasure or objection by a lead** | The visitor contacts the organisation, through its privacy notice, linked from the form. The organisation then deletes the lead. |

What deleting an account does to the person's things:

- **Their work stays with the organisation.** Cards and files pass to the owner, and leads stay.
- **Their traces go.** The email on feedback they sent is removed, and so are integration log lines about them.
- **An owner** first hands the organisation to someone else (`POST /api/me/ownership/transfer`, which re-encrypts the bucket keys for the new owner), or deletes it with everything in it, including its bucket objects.
- **SCIM-managed people** are removed by their identity provider instead.

## Cookies and browser storage

All cookies are strictly necessary, so no consent banner is needed.

| Name | Purpose | Lifetime | Flags |
| ---- | ------- | -------- | ----- |
| `fronko_session` | Signed-in session (JWT) | 24 h | HttpOnly, Secure, SameSite=Lax |
| `fronko_admin` | Platform admin session | 8 h | HttpOnly, Secure, SameSite=Strict |
| `fronko_oauth` | OAuth state while connecting an integration | 10 min | HttpOnly, Secure, SameSite=Lax |
| `fronko_sso` | SAML request state | Short | HttpOnly, Secure, SameSite=None |

`localStorage` holds two preferences, neither of them personal data:

- `fronko-theme`: light or dark.
- `fronko.files.view`: grid or list.

Public card visitors get no cookies and no storage. The visit id lives in the tab's memory only.

## The operator's promise to visitors

The lead form says: **"Fronko never sells or shares your details with anyone but {organisation}."** It is a public commitment, and the design backs it up:

- **No access path for the operator.** The platform admin panel and its API (`/api/admin/*`) read aggregates only: counts of leads, cards and files. No endpoint returns a lead, a card's contents or a file. The integration test in `integrationtest/admin_test.go` checks this.
- **No data leaves except where the organisation sends it.** Leads go only to the organisation's own people and the integrations it connects (its CRM, webhooks), plus booking pages after the form is sent.
- **No sale, advertising or analytics partners.** There are no third-party scripts and no data brokers.

Keeping the promise also means:

- **Admin panel:** never add an admin feature that shows leads or card contents. Support access, if ever needed, should be something the organisation grants explicitly.
- **Database:** treat direct database access to `leads` like any other access to customer data. Restrict it, and don't browse it.
- **Disclosure:** say the same in the operator's privacy policy, and in the DPA offered to customers ("the operator processes leads only to provide the service").

The infrastructure providers that store the database and send emails are processors bound by DPAs, not "sharing", but list them in the privacy policy.

## Recipients and sub-processors

| Recipient | Data | When | Chosen by |
| --------- | ---- | ---- | --------- |
| SMTP provider | Account emails, codes, notices | Always (if configured) | Operator |
| S3-compatible bucket | Uploaded photos and PDFs | When the org connects storage | Organisation |
| HubSpot | Lead details, card and holder | When the org connects it | Organisation |
| Zapier, Make, n8n, custom webhooks | Lead details, card and holder | When the org connects one | Organisation |
| SAML identity providers (Entra ID, Okta, …) / SCIM | Sign-in assertions; directory sync | When the org sets it up | Organisation |
| Booking pages (Calendly, Cal.com, HubSpot Meetings, Microsoft Bookings, Chili Piper, Google Calendar) | The visitor's name and email in the link, after they sent the lead form, when the booking page supports prefilling | Link-out only | Organisation |

No geo-IP, Gravatar, web fonts, analytics SDKs or error trackers are used. The browser loads nothing from third parties. The one exception was older cards with a pasted photo URL, which are now shown only when the URL is on the same site.

## Security (Art. 32)

The full table is in the [backend README](../../backend/README.md#security-measures). In short:

- **Passwords:** bcrypt cost 12, upgraded on sign-in. Passwords are limited to 8–72 bytes, and sign-in timing is equalised.
- **Sessions:** HttpOnly cookies carrying a session version, so a password change or suspension revokes them.
- **Requests:** a same-origin check, a CORS allowlist, and per-IP rate limits.
- **Secrets at rest:** AES-256-GCM with additional authenticated data bound to the row.
- **Codes and tokens:** one-time codes and SCIM tokens are stored only as hashes.
- **Outbound requests:** an SSRF guard after DNS resolution.
- **Headers:** `nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy`, `Permissions-Policy` and COOP on the API; HSTS when secure. nginx adds the same plus `frame-ancestors 'none'`, `base-uri`, and `object-src 'none'`.
- **Logs:** no request bodies, IPs or emails. Unsent email bodies are logged in development only.
- **Platform admins:** they see totals only, plus the owner's email (to reach them about a suspension) and feedback people chose to send.

## Findings

Severity: **H** high, **M** medium, **L** low.

| # | Area | Sev | Finding | Status |
| - | ---- | :-: | ------- | ------ |
| 1 | Settings | H | No way to delete one's own account; the owner could never be deleted; no organisation deletion | **Fixed:** Danger zone, ownership transfer, org deletion (`account/deletion.go`) |
| 2 | Settings | H | No data export (Art. 15/20) | **Fixed:** `GET /api/me/export`, "Download my data" |
| 3 | Public card | H | Lead form had no privacy notice, and said details are "only visible to them" (untrue: admins and integrations see them) | **Fixed:** accurate copy naming the organisation, and a privacy notice link when the organisation sets one. **No consent checkbox, by product decision:** the legal basis is the visitor's own request to be contacted (Art. 6(1)(b)/(f)), not consent. That's defensible as long as the organisation's privacy notice covers lead capture and CRM sync |
| 4 | Backend | H | Without SMTP, email bodies (codes, temporary passwords) were logged in production | **Fixed:** bodies logged only with `FRONKO_ENV=development` |
| 5 | Leads | H | Leads kept forever | **Fixed:** per-org `lead_retention_days` with a confirmation before shortening |
| 6 | Public card | M | Visitor name and email added to third-party booking links | **Accepted, by product decision.** Only after the visitor sent the form, and only into the booking page the card links to. The organisation's privacy notice should mention it |
| 7 | Public card | M | No notice of analytics; no opt-out | **Fixed:** footer notice; GPC/DNT honoured in the browser and on the server |
| 8 | Public card | M | Cards with personal details indexable by search engines | **Fixed:** `noindex` unless "Show in search engines" is on (default off); robots.txt excludes app areas |
| 9 | Leads | M | Deleting a lead left its email in integration activity | **Fixed:** activity deleted with the lead |
| 10 | Leads | M | No bulk delete for erasure requests | **Fixed:** multi-select delete |
| 11 | Feedback | M | Sender email kept after account deletion; feedback and audit log kept forever | **Fixed:** cleared on deletion; 2-year retention |
| 12 | Backend | M | Expired one-time codes never swept | **Fixed:** hourly sweep |
| 13 | Backend | M | No security headers | **Fixed:** API middleware + nginx |
| 14 | Deploy | M | nginx access log recorded IPs, user agents and query strings | **Fixed:** `fronko_private` log format |
| 15 | Users | M | SSO sign-in log lines ("Signed in <email>") outlived deleted users | **Fixed:** deleted with the user |
| 16 | Backend | L | bcrypt cost 10 | **Fixed:** cost 12, rehash on sign-in |
| 17 | Feedback | L | Page path could carry query strings (search terms, tokens) | **Fixed:** query and fragment dropped |
| 18 | Public card | L | Legacy external avatar URL let a third party see visitors | **Fixed:** only same-origin URLs shown |
| 19 | Admin | L | Admin could try to reply to deleted senders | **Fixed:** shown as "Deleted account", reply disabled |
| 20 | Org deletion | M | Deleting an org would leave its files in the bucket | **Fixed:** objects removed first (best effort, logged) |
| 21 | Org deletion | L | Audit log kept org name and suspension reason after org deletion | **Fixed:** scrubbed |
| 22 | Ownership | M | Bucket keys are bound to the owner, so ownership couldn't move | **Fixed:** re-encrypted in the transfer transaction |
| 23 | Settings | L | No place to link the org's privacy notice | **Fixed:** Settings → Organisation → Privacy |
| 24 | Legal | H | No privacy policy, terms, imprint or cookie notice anywhere; no links in the footer | **Open:** operator (see below) |
| 25 | Sign-up | M | No acceptance of terms / privacy policy at registration | **Open:** add once the documents exist |
| 26 | Integrations | M | Erasure doesn't reach CRMs and webhooks that already received a lead | **Open:** by design; the delete dialogs say so. Organisations handle it in their CRM |
| 27 | Infra | M | No documented backups, backup encryption or restore drill; DB TLS (`sslmode`) not enforced | **Open:** operator |
| 28 | Infra | M | No breach-response runbook (Art. 33/34: 72-hour notification) | **Open:** operator |
| 29 | Secrets | L | No key rotation for `SECRETS_KEY` / `JWT_SECRET` (a version byte exists) | **Open** |
| 30 | Frontend | L | Content-Security-Policy restricts framing, base and objects only; scripts and connections aren't restricted, because buckets are customer-chosen and the app has inline bootstrap scripts | **Open:** tighten with nonces when feasible |
| 31 | Public card | L | `noindex` is set by the app after it loads (client-rendered), so crawlers that don't run JavaScript won't see it | **Open:** acceptable for major engines; server rendering of the tag would close it |
| 32 | Accounts | L | Inactive accounts are never purged | **Open:** decide a policy (e.g. notify after 12 months, delete after 13) |
| 33 | Usage snapshots | — | Kept indefinitely | Not personal data (counts only) |

## Open items for the operator

These need a decision or documents rather than code:

1. **Organisations' privacy notices.** The lead form has no consent box, so each organisation's privacy notice (Settings → Organisation → Privacy) must say that leads are stored, who in the organisation sees them, which tools they're synced to (CRM, webhooks), the prefilled booking pages, and how long they're kept. Encourage customers to set one.
2. **Privacy policy, terms and imprint** (the imprint is required in Germany and Austria). Publish them, link them from the landing page footer and the sign-in page, then add an "I agree to the terms and privacy policy" line at sign-up (finding 25).
3. **Data processing agreement** for customers: organisations are controllers of their leads and cards, and the operator processes them. Use a DPA template that names the sub-processors above.
4. **DPAs with your own sub-processors:** SMTP provider, hosting and database provider, and the backup location. Check that each is in the EEA, or covered by SCCs or an adequacy decision.
5. **Record of processing activities.** The [inventory](#personal-data-inventory) above is a starting point.
6. **Breach-response runbook:**
   - who decides;
   - how to assess risk;
   - notify the supervisory authority within 72 hours;
   - notify affected people when the risk is high.
7. **Backups:**
   - encrypted;
   - in the EEA;
   - with a retention period, so deleted accounts eventually leave the backups;
   - with a tested restore.

   Set `sslmode=require` on `DATABASE_URL` when the database is remote.
8. **Configure SMTP in production.** Without it, nobody can verify an email or reset a password.
9. **DPIA:** card analytics are low-risk (aggregate and cookieless). Document that conclusion. Lead capture at scale is ordinary processing.
10. **Inactive accounts:** decide on finding 32.

## Re-running this audit

When adding a feature, use the gdpr-compliant skill's PR checklist (`.claude/skills/gdpr-compliant`). For any new table or field holding personal data:

- add a row to the inventory above;
- add a retention rule;
- include it in the export (`users/export.go`);
- handle it on deletion (`users.forgetUser`, `orgs.DeleteOrganization`).
