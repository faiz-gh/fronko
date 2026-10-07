<div align="center">

<img src="frontend/static/fronko-mark.svg" alt="Fronko logo" width="96" height="96" />

# Fronko

**Open-source, self-hostable digital business cards for teams.**

Share a card with an NFC tap or a QR scan. No app needed to view it, it saves to contacts in one tap, and leads come back to your team.

[![License: GPL v3](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)](backend/go.mod)
[![Svelte](https://img.shields.io/badge/Svelte-5-FF3E00?logo=svelte&logoColor=white)](frontend/package.json)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-14%2B-4169E1?logo=postgresql&logoColor=white)](backend/README.md#prerequisites)
[![Docker Compose](https://img.shields.io/badge/deploy-Docker%20Compose-2496ED?logo=docker&logoColor=white)](deploy/docker-compose.yml)

[Features](#features) · [Quick start](#quick-start) · [Documentation](#documentation) · [Contributing](#contributing)

</div>

---

## Overview

Fronko replaces paper business cards with a web page. Write a card's link to any NFC card or print its QR code. When someone taps or scans it, the card opens in their mobile browser with nothing to install.

Fronko is an open-source alternative to hosted platforms such as Popl and Mobilo. You run it on your own infrastructure, uploaded files go to your own S3-compatible bucket, and you own all of your data: profiles, contacts and leads.

## Table of contents

- [Features](#features)
- [How it works](#how-it-works)
- [Tech stack](#tech-stack)
- [Quick start](#quick-start)
- [Configuration](#configuration)
- [Connecting storage](#connecting-storage)
- [Writing NFC cards](#writing-nfc-cards)
- [Integrations](#integrations)
- [Local development](#local-development)
- [Documentation](#documentation)
- [Roadmap](#roadmap)
- [Contributing](#contributing)
- [Security](#security)
- [License](#license)

## Features

### For the people receiving a card

- **No app required.** An NFC tap or QR scan opens the card as an ordinary web page.
- **One-tap save to contacts.** Visitors open a vCard 3.0 (`.vcf`) file, which iOS and Android import directly.
- **Tap straight to an action.** Each card chooses separately what an NFC tap and a QR scan do: show the card, open the phone's "Add contact" sheet with the card behind it, or open the contact form.
- **Share details back.** A built-in form lets visitors send their name and email, plus an optional phone number and note.

### For card owners

- **Live-preview card editor.** A section rail lists every part of the card with a one-line summary, so you see the whole card at a glance and edit one section at a time. Edit profile details, contact information (with a country-code phone picker), social links with brand icons, a cropped photo and cover banner, PDF brochures, an accent colour and a light or dark theme.
- **Templates and blocks.** Start from a template (Classic, Event tag, Portfolio, Minimal), then reorder, hide or add blocks: headings, text, an image gallery, an event panel with a role ribbon, and dividers. Three header styles: banner, name badge or compact.
- **Multiple cards.** Each card has its own link, an NFC link and a QR code you can download as SVG or PNG. Links include the organisation's handle (`/p/acme/jane`), so a card name only has to be unique inside your organisation.
- **Write NFC cards from the dashboard.** On Android, Chrome writes the card's link straight onto the NFC chip. On iPhone and desktop, copy the link for the free NFC Tools app, or scan a QR code to open the writer on an Android phone.
- **Branded QR codes.** Put the organisation's logo or any image from Files in the middle of a card's QR code, and pick its colours, dot style (square, rounded, dots) and corner style. Error correction goes up automatically when there's an image, and colours that might not scan are flagged (too little contrast can't be saved).
- **Card analytics.** See how each card is found and used: views split by NFC tap, QR scan or link, unique visitors, which links and quick actions get clicked, how far people scroll, how long they stay, how often each brochure is opened (and by how many people), contact saves, contact forms opened and sent, devices, and the busiest days and hours. Every number is compared with the previous period.
- **Email signatures.** Turn any card into an email signature from one of five templates (Classic, Corporate, Compact, Bold, Minimal), choose what it includes, then copy it straight into Gmail, Outlook or Apple Mail, or download it as HTML. The HTML is built for mail clients: tables, inline styles and PNG/JPEG images only.
- **Lead inbox.** Leads from every card land in one place, with card filters, search, pagination and CSV export.
- **An overview that answers "how are my cards doing?".** The dashboard home shows views, unique visitors, contact saves and leads for the last 7 or 30 days, views by source, a live feed of taps, scans, saves and brochure opens, recent leads, and what needs attention.

### For organisations

- **Organisation accounts.** Every sign-up creates an organisation. The owner, and any admins they promote, create accounts for the team. Each new user is emailed their username and a temporary password, then confirms their email and chooses their own password on first sign-in.
- **Role-based access.** Admins assign cards to people. Each person can edit everything on their cards except the link (and only admins can change the organisation's link handle), and sees only their own cards and the leads those cards collected while assigned to them. Admins see everything and can filter cards and leads by person or team.
- **Teams.** Group people into teams such as Sales or Finance; one person can be in several. Each team has its own files that everyone in it can use. Team leads look after those files and can see and edit their teammates' cards and leads; creating, deleting and reassigning cards stays with admins.
- **Team analytics.** Admins compare every team, and team leads their own: people, active cards, views (and per person), engaged visits, saves, leads and visit-to-lead conversion, each with the change from the previous period. A leaderboard ranks people by views, saves or leads, and cards nobody has viewed in 30 days are flagged.
- **Private by design.** Visitors are never identified: no cookies, no stored IP addresses, only a visitor hash salted with a random value that's replaced and deleted every day. Bots and people signed in to the card's own organisation aren't counted, and events are deleted after 13 months (configurable).
- **User management.** Admins can suspend users, reset their passwords, or delete them without losing their cards, files or leads.
- **Branding.** Upload a square organisation logo, cropped on upload. It sits as an emblem on the lower right of every profile photo and appears in email signatures. Either require it on every card and signature, or let each person choose.
- **Signature rules.** Optionally lock everyone to one signature template, set a brand colour, and add a disclaimer and a clickable banner (cropped to 4:1, 3:1 or 2:1) under every signature.
- **File manager.** Every file has a purpose (logo, banner, profile photo, cover, gallery, brochure or other), set from where it was uploaded and changeable later. Browse by location, filter by purpose, search, sort, and switch between a grid of previews (the first page for PDFs) and a list. Select several files to re-purpose, move or delete them, and open any file to see exactly which cards, logo or banner use it, or crop a copy. Pickers in the card editor and branding settings open on the right purpose and can crop images already in the library.
- **Files with access control.** Files live in four areas: each person's own files (with an optional per-person storage limit), the organisation's private files, a shared area for everyone, and each team's files. Admins can give individual people or whole teams access to extra files.

### Integrations

- **Lead sync.** Every lead goes to your CRM or automation tool as soon as it arrives: a signed webhook to any URL, Zapier, Make, n8n, or HubSpot (contacts upserted by email, with a note saying which card it came from). The organisation can sync every lead, and each person can add their own for the cards they hold. Failed deliveries are retried for about four hours, and every attempt is in an activity log.
- **Booking pages on cards.** Connect Calendly, Chili Piper, Microsoft Bookings, HubSpot Meetings, Google Calendar or any other booking link once, and every card you hold gets a "Book a meeting" button. The organisation can set a default for everyone else. Calendly and HubSpot open with the visitor's name and email filled in.
- **Single sign-on (SAML).** Okta, Microsoft Entra ID or any SAML 2.0 identity provider. People sign in with their work email (on a verified domain) or the organisation's handle, accounts are created on first sign-in, and SSO can be required for everyone but the owner.
- **Team member import (SCIM).** Microsoft Entra ID adds, updates, suspends and removes people automatically, and keeps teams in step with its groups.
- **Bring your own apps.** Each organisation registers its own OAuth apps, so nothing is shared between organisations and the server needs no per-provider configuration. More providers (Salesforce, Zoho, Dynamics 365, Pipedrive, Okta SCIM and others) are listed as coming soon.

### For the people running the server

- **Platform admin panel.** A separate admin area at `/admin`, with its own accounts and sign-in, shows how each organisation uses Fronko: users, teams, cards, leads, files (by purpose), whether storage is connected and how much is used, whether it set a logo and locked its signature template, plus trends over 30 days to a year. It shows totals only. Card contents, leads, files and team members stay private to each organisation.
- **Product feedback.** Signed-in users send a bug report, idea or other note, with an optional 1–5 rating, from the account menu. It lands in the admin inbox and can be emailed to you. Replies are emailed back to the sender.
- **Suspend organisations.** Suspending an organisation signs everyone in it out, blocks sign-in and takes its public cards offline. Nothing is deleted. The owner is emailed the reason, and again when it's reinstated. Every admin action is kept in an audit log.

### Accounts and security

- **Verified sign-up.** Users sign up with a username and an email address, which they confirm with a 6-digit emailed code. They can sign in with either, or with their organisation's single sign-on.
- **Self-service recovery.** Forgotten passwords are reset with an emailed code. From **Settings**, users can change their email address (the old address is notified) or their password (which signs out every other device).
- **Bring your own storage.** Photos and brochures go to the organisation's own S3-compatible bucket. Storage keys are encrypted at rest with AES-256-GCM, buckets stay private, and visitors get short-lived signed links.

## How it works

A tap or scan opens a static public page that loads the card in a single API call.

```mermaid
graph TD
    A[NFC tap / QR scan] -->|opens /p/org/slug?via=nfc or qr| C{Public card page}
    C -->|card's tap action| T[Show card / save contact / contact form]
    E[Dashboard: card editor] -->|saves card data| D[(PostgreSQL)]
    E -->|uploads photos and PDFs| S[(Organisation's S3 bucket)]
    D -->|card and file metadata| C
    S -->|short-lived signed links| C
    C -->|Save contact| F[vCard from /api/profiles/org/slug/vcard]
    E -->|card + org branding| SIG[Email signature HTML, copied into the mail app]
    C -->|Share your details| G[Lead form]
    C -->|views, clicks, scrolls, beacons| AN[(Card analytics events)]
    AN -->|overview, Analytics page| E
    G -->|stores lead + queues sync jobs| D
    D -->|lead inbox, CSV export| E
    D -->|background worker| CRM[Webhook, Zapier, HubSpot …]
```

Fronko is deployed as two containers:

- **`backend`**: the Go API server. It applies database migrations automatically on startup.
- **`frontend`**: nginx serving the static Svelte app and proxying `/api`, `/auth` and `/scim` to the backend.

Background work (lead sync, retention, usage snapshots) runs inside the backend, using a job queue in PostgreSQL.

PostgreSQL is not part of the stack. Bring your own instance, managed or self-hosted.

## Tech stack

| Layer | Technology |
| ----- | ---------- |
| Backend | Go 1.27, standard library `net/http`, `pgx` |
| Frontend | Svelte 5 and SvelteKit (static single-page build), Tailwind CSS v4, shadcn-svelte |
| Database | PostgreSQL 14+, with schema migrations via `golang-migrate`; also the background job queue |
| Integrations | `golang.org/x/oauth2`, `crewjam/saml` (SAML 2.0), a built-in SCIM 2.0 server |
| File storage | Any S3-compatible bucket (Cloudflare R2, Backblaze B2, AWS S3, MinIO) via `aws-sdk-go-v2` |
| Email | Any SMTP provider (Resend, Amazon SES, Brevo and others) |
| Deployment | Docker Compose, with nginx serving the frontend |

## Quick start

### Prerequisites

- Docker with Docker Compose
- A PostgreSQL 14+ database
- A reverse proxy that terminates TLS (for example Traefik, Caddy or Coolify)
- An SMTP account for verification and password-reset emails (recommended)

### 1. Clone the repository

```bash
git clone https://github.com/faiz-gh/fronko.git
cd fronko/deploy
```

### 2. Configure the environment

```bash
cp .env.example .env
```

Edit `deploy/.env`. At a minimum, set `DATABASE_URL` and `JWT_SECRET`:

```env
DATABASE_URL=postgres://fronko:password@your-db-host:5432/fronko?sslmode=require
JWT_SECRET=replace-with-output-of-openssl-rand-base64-32
SECRETS_KEY=replace-with-output-of-openssl-rand-base64-32
FRONTEND_URL=https://cards.example.com
PUBLIC_URL=https://cards.example.com

SMTP_HOST=smtp.resend.com
SMTP_PORT=587
SMTP_USERNAME=resend
SMTP_PASSWORD=your-api-key
SMTP_FROM="Fronko <no-reply@example.com>"
```

See [Configuration](#configuration) for every option.

### 3. Start the stack

```bash
docker compose up -d --build
```

### 4. Route traffic to Fronko

No ports are bound on the host. The frontend exposes port `3000` and the backend port `8080` on the Docker network only. Attach your reverse proxy to the same network and route your domain to `frontend:3000`.

The session cookie is sent over HTTPS only by default (`COOKIE_SECURE=true`), so serve the site over TLS.

Open your domain, create an account, and you become the owner of a new organisation.

### 5. Create a platform admin (optional)

The admin panel at `/admin` uses its own accounts, which can only be created from the command line. Run this once, then type a password of 12–72 characters when prompted:

```bash
docker compose exec backend ./fronko admin create --email you@example.com
```

Sign in at `https://cards.example.com/admin/login`. To change a forgotten admin password later, run the same command with `set-password` in place of `create`. This also signs that admin out everywhere.

### Deployment topologies

| Topology | Setup |
| -------- | ----- |
| **Single domain** (default) | Leave `BACKEND_URL` empty. The frontend's nginx proxies `/api` and `/auth`, so all requests are same-origin and need no CORS setup. |
| **Separate API domain** | Route a domain such as `api.cards.example.com` to `backend:8080`, then set `BACKEND_URL` to it and `FRONTEND_URL` to the site's origin. Use a subdomain of the same site so the `SameSite=Lax` session cookie is still sent, and allow request bodies up to 25 MB on that proxy for uploads. |

Both URLs are read when the containers start, so changing them needs only a restart, not a rebuild.

## Configuration

All settings are environment variables, set in `deploy/.env`.

| Variable | Required | Description |
| -------- | :------: | ----------- |
| `DATABASE_URL` | Yes | PostgreSQL connection string. |
| `JWT_SECRET` | Yes | Key for signing session tokens. Must be at least 16 characters; generate one with `openssl rand -base64 32`. |
| `SECRETS_KEY` | | Base64 of 32 random bytes. Encrypts storage credentials and integration secrets at rest. Without it, photo and brochure uploads are disabled, and so are integrations that store a secret. **Keep it stable and backed up**: if it changes or is lost, saved credentials can't be decrypted. |
| `PUBLIC_URL` | | Where people reach the site, such as `https://cards.example.com` (no trailing slash). Needed by HubSpot (OAuth), SAML SSO and SCIM, which show as unavailable without it, and used for card links in lead sync. |
| `JOB_WORKERS` | | Background jobs (such as lead sync) this instance runs at once. Defaults to `2`; `0` queues jobs without running them. |
| `FRONTEND_URL` | | Public origin of the site. Allowed as a CORS origin; comma-separate several. |
| `BACKEND_URL` | | Public API URL the browser calls. Leave empty to proxy through the frontend (same-origin). |
| `COOKIE_SECURE` | | Defaults to `true`. Set to `false` only when testing over plain HTTP. |
| `SMTP_HOST` | | Outgoing mail server. Without it, emails (including their codes) are only written to the backend log. |
| `SMTP_PORT` | | Defaults to `587`. Port `465` uses implicit TLS; other ports upgrade with STARTTLS. |
| `SMTP_USERNAME` / `SMTP_PASSWORD` | | SMTP credentials. |
| `SMTP_FROM` | With `SMTP_HOST` | Sender address, such as `Fronko <no-reply@example.com>`. Must be an address your provider lets you send from. |
| `ANALYTICS_RETENTION_DAYS` | | How long card analytics events are kept. Defaults to `395` (about 13 months, so a year can be compared with the one before). |
| `FEEDBACK_NOTIFY_EMAIL` | | Gets an email for each piece of product feedback, and is the Reply-To on admins' replies and on suspension emails. Without it, feedback only appears in the admin panel. |

The [backend configuration reference](backend/README.md#configuration) covers every option, including `PORT`, `TRUST_PROXY`, `FRONKO_ENV` and `STORAGE_ALLOW_PRIVATE_ENDPOINTS`, plus the fixed server and connection-pool settings.

**Upgrading a deployment from before the squashed migrations?** Its database has to be reset; see [Resetting the database](docs/operations/resetting-the-database.md).

## Connecting storage

Each organisation's owner connects a bucket under **Dashboard → Settings → Storage**, and everyone in the organisation uploads to it. The server only needs `SECRETS_KEY` set.

Buckets stay **private**, and visitors get signed links that expire after 15 minutes. You don't need public access, a custom domain or CORS rules, because uploads go through the Fronko backend.

The access keys need write and delete permission on objects in the bucket, plus read permission for serving them. They need no account-wide permissions. Fronko checks the keys when you save them by writing and then deleting a `.fronko-probe` object.

| Provider | Endpoint | Region | Path-style | Keys |
| -------- | -------- | ------ | :--------: | ---- |
| **Cloudflare R2** | `https://<ACCOUNT_ID>.r2.cloudflarestorage.com` (EU jurisdiction: `https://<ACCOUNT_ID>.eu.r2.cloudflarestorage.com`) | `auto` | On | R2 → Manage API tokens → Create API token → **Object Read & Write**, scoped to the bucket |
| **Backblaze B2** | `https://s3.<region>.backblazeb2.com` (shown on the bucket page) | e.g. `us-west-004` | Off | Application Keys → new key with **Read and Write** access to this bucket only |
| **AWS S3** | `https://s3.<region>.amazonaws.com` | The bucket's region | Off | IAM user with `s3:GetObject`, `s3:PutObject` and `s3:DeleteObject` on `arn:aws:s3:::<bucket>/*` |
| **MinIO / other** | Your HTTPS S3 API URL | `us-east-1` unless configured | On | Access key with a read/write policy on the bucket |

Objects are stored as `fronko/<org_id>/<user_id>/<file_id>.<ext>`. Keep the bucket private; for R2, don't enable a public `r2.dev` URL.

## Writing NFC cards

Fronko works with any NFC hardware that can store a URL. Blank NTAG215 or NTAG216 cards are inexpensive and widely available.

Card links look like `https://cards.example.com/p/acme/jane`: the organisation's handle (set under **Settings → Organisation → Card links**), then the card's slug, which only has to be unique inside the organisation. Both end up on QR codes and NFC chips, so treat them as permanent once cards go out.

The chip holds the card's **NFC link**, `…/p/acme/jane?via=nfc`. The `?via=nfc` marks the visit as an NFC tap for analytics and runs the NFC tap action chosen under **Sharing**; that action can change later without rewriting the chip. Tags are never locked, so they can be rewritten.

- **Android (Chrome):** open **Write to NFC card** (card editor → Sharing, the card's ⋯ menu, or `/dashboard/{id}/nfc`), tap the button and hold the card to the back of the phone. This uses Web NFC, which needs HTTPS.
- **iPhone, other Android browsers:** the same dialog offers **Copy NFC link** and the steps for the free NFC Tools app (**Write → Add a record → URL/URI**, paste, **Write**, hold the card to the phone).
- **Desktop:** the dialog also shows a QR code that opens the writer page on a phone.

On metal cards the chip only reads through a small window; slide the card slowly over the back of the phone and hold it still once it's found.

## Integrations

Admins connect integrations under **Dashboard → Integrations**; members connect their own booking page and lead sync webhooks there too. The server only needs `SECRETS_KEY`, and `PUBLIC_URL` for HubSpot, SAML and SCIM. There are setup guides for each provider:

| Category | Guides |
| -------- | ------ |
| Lead Sync | [Webhook, Zapier, Make and n8n](docs/integrations/webhook-zapier.md), [HubSpot](docs/integrations/hubspot.md) |
| Calendar Booking | [Calendly, Chili Piper, Microsoft Bookings, HubSpot Meetings, Google Calendar, other links](docs/integrations/calendar.md) |
| SAML SSO | [Okta](docs/integrations/okta-saml.md), [Microsoft Entra ID](docs/integrations/entra-saml.md), [any SAML 2.0 provider](docs/integrations/saml.md) |
| Team Member Import | [Microsoft Entra ID (SCIM)](docs/integrations/entra-scim.md) |

See the [integrations overview](docs/integrations/overview.md) for how connections, retries and the activity log work. Want another provider? [Request it](https://github.com/faiz-gh/fronko/issues/new?template=integration_request.yml), or [build it](docs/integrations/writing-a-provider.md).

## Local development

**Without Docker.** Run the backend and frontend separately. The Vite dev server proxies API calls to the backend on `localhost:8080`.

```bash
# Backend (Go 1.27+). See backend/README.md for environment variables and migrations.
cd backend && make run

# Frontend (Node.js 24), in a second terminal
cd frontend && npm install && npm run dev   # http://localhost:5173
```

See [backend: Run locally](backend/README.md#run-locally) and [frontend: Getting started](frontend/README.md#getting-started) for the full steps.

**With Docker, against a local PostgreSQL.** Edit the hard-coded `DATABASE_URL` in `deploy/docker-compose.local.yml`, then run:

```bash
cd deploy
docker compose -f docker-compose.local.yml up --build
```

This setup:

- serves the app on `http://localhost:3000`, always same-origin (it ignores `FRONTEND_URL` and `BACKEND_URL` from `.env`)
- sets a development `SECRETS_KEY` and `STORAGE_ALLOW_PRIVATE_ENDPOINTS=true`, so storage can point at a local S3 server such as MinIO on `http://host.docker.internal:9000`
- starts [Mailpit](https://mailpit.axllent.org/), which catches every email the backend sends; its inbox is at `http://localhost:8025`
- sends feedback notices to `admin@fronko.local`, which also land in Mailpit
- sets `FRONKO_ENV=development` and `PUBLIC_URL=http://localhost:3000`, so integrations can call a webhook receiver on your machine (as `http://host.docker.internal:<port>`) and SAML and SCIM URLs point at the local site

**Demo data.** Fill the database with a demo organisation, cards, leads, analytics and a platform admin:

```bash
docker exec -e FRONKO_ENV=development backend-local ./fronko seed --reset
```

Then sign in as `uidemo` / `password123`, or at `http://localhost:3000/admin/login` as `admin@fronko.local` / `adminpassword123`. [Demo data](CONTRIBUTING.md#demo-data) lists every account. To create your own admin instead, run `docker exec -it backend-local ./fronko admin create --email you@example.com`.

## Documentation

| Document | Contents |
| -------- | -------- |
| [Backend](backend/README.md) | Setup, configuration, database schema, authentication and security model, testing |
| [API reference](backend/API.md) | Every endpoint, with request and response shapes and error codes |
| [Frontend](frontend/README.md) | Routes, session handling, the card data model, components and styling |
| [docs/](docs/README.md) | Architecture, integration setup guides, writing a provider, decision records and operations |

## Roadmap

Planned but not yet built:

- **Shared rate limiting.** Rate limits are currently kept in memory per backend instance. Running several instances needs a shared store such as Redis.
- **More integrations.** Salesforce, Zoho CRM, Microsoft Dynamics 365, Pipedrive, monday.com, Marketo and Slack for lead sync; Okta SCIM and Google Workspace for team import; meeting-booked webhooks from booking tools.
- **More profile blocks.** Embeds (video, maps) and more block types for the block-based card layout.

Ideas and feedback are welcome in [GitHub Issues](https://github.com/faiz-gh/fronko/issues).

## Contributing

Contributions are welcome, from bug reports and documentation fixes to new features. Read the [contributing guide](CONTRIBUTING.md) to set up a development environment, learn the coding guidelines, and submit a pull request.

## Security

If you find a security vulnerability, **please do not open a public issue.** Report it privately as described in the [security policy](SECURITY.md), which also covers how to harden a self-hosted deployment.

## License

Fronko is licensed under the [GNU General Public License v3.0](LICENSE).
