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
- [Local development](#local-development)
- [Documentation](#documentation)
- [Roadmap](#roadmap)
- [Contributing](#contributing)
- [Security](#security)
- [License](#license)

## Features

### For the people receiving a card

- **No app required.** An NFC tap or QR scan opens the card as an ordinary web page.
- **One-tap save to contacts.** Visitors download a vCard 3.0 (`.vcf`) file, which iOS and Android import directly.
- **Share details back.** A built-in form lets visitors send their name and email, plus an optional phone number and note.

### For card owners

- **Live-preview card editor.** Edit profile details, contact information (with a country-code phone picker), a booking link (Calendly, Cal.com, Google Calendar and others), social links with brand icons, a cropped photo and cover banner, PDF brochures, an accent colour and a light or dark theme.
- **Multiple cards.** Each card has its own link and a QR code you can download as SVG or PNG.
- **Lead inbox.** Leads from every card land in one place, with card filters, search, pagination and CSV export.

### For organisations

- **Organisation accounts.** Every sign-up creates an organisation. The owner, and any admins they promote, create accounts for the team. Each new user is emailed their username and a temporary password, then confirms their email and chooses their own password on first sign-in.
- **Role-based access.** Admins assign cards to people. Each person can edit everything on their cards except the link, and sees only their own cards and the leads those cards collected while assigned to them. Admins see everything and can filter leads by person.
- **User management.** Admins can suspend users, reset their passwords, or delete them without losing their cards, files or leads.
- **Files with access control.** Files are kept in three areas: each person's own files (with an optional per-person storage limit), the organisation's private files, and a shared area for company-wide brochures. Admins can give individual people access to extra files.

### For the people running the server

- **Platform admin panel.** A separate admin area at `/admin`, with its own accounts and sign-in, shows how each organisation uses Fronko: users, cards, leads, files, whether storage is connected and how much is used, plus trends over 30 days to a year. It shows totals only. Card contents, leads, files and team members stay private to each organisation.
- **Product feedback.** Signed-in users send a bug report, idea or other note, with an optional 1–5 rating, from the account menu. It lands in the admin inbox and can be emailed to you. Replies are emailed back to the sender.
- **Suspend organisations.** Suspending an organisation signs everyone in it out, blocks sign-in and takes its public cards offline. Nothing is deleted. The owner is emailed the reason, and again when it's reinstated. Every admin action is kept in an audit log.

### Accounts and security

- **Verified sign-up.** Users sign up with a username and an email address, which they confirm with a 6-digit emailed code. They can sign in with either.
- **Self-service recovery.** Forgotten passwords are reset with an emailed code. From **Settings**, users can change their email address (the old address is notified) or their password (which signs out every other device).
- **Bring your own storage.** Photos and brochures go to the organisation's own S3-compatible bucket. Storage keys are encrypted at rest with AES-256-GCM, buckets stay private, and visitors get short-lived signed links.

## How it works

A tap or scan opens a static public page that loads the card in a single API call.

```mermaid
graph TD
    A[NFC tap / QR scan] -->|opens /p/slug| C{Public card page}
    E[Dashboard: card editor] -->|saves card data| D[(PostgreSQL)]
    E -->|uploads photos and PDFs| S[(Organisation's S3 bucket)]
    D -->|card and file metadata| C
    S -->|short-lived signed links| C
    C -->|Save contact| F[vCard download]
    C -->|Share your details| G[Lead form]
    G -->|stores lead| D
    D -->|lead inbox, CSV export| E
```

Fronko is deployed as two containers:

- **`backend`**: the Go API server. It applies database migrations automatically on startup.
- **`frontend`**: nginx serving the static Svelte app and proxying `/api` and `/auth` to the backend.

PostgreSQL is not part of the stack. Bring your own instance, managed or self-hosted.

## Tech stack

| Layer | Technology |
| ----- | ---------- |
| Backend | Go 1.27, standard library `net/http`, `pgx` |
| Frontend | Svelte 5 and SvelteKit (static single-page build), Tailwind CSS v4, shadcn-svelte |
| Database | PostgreSQL 14+, with schema migrations via `golang-migrate` |
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
| `SECRETS_KEY` | | Base64 of 32 random bytes. Encrypts storage credentials at rest. Without it, photo and brochure uploads are disabled. **Keep it stable and backed up**: if it changes or is lost, saved storage credentials can't be decrypted. |
| `FRONTEND_URL` | | Public origin of the site. Allowed as a CORS origin; comma-separate several. |
| `BACKEND_URL` | | Public API URL the browser calls. Leave empty to proxy through the frontend (same-origin). |
| `COOKIE_SECURE` | | Defaults to `true`. Set to `false` only when testing over plain HTTP. |
| `SMTP_HOST` | | Outgoing mail server. Without it, emails (including their codes) are only written to the backend log. |
| `SMTP_PORT` | | Defaults to `587`. Port `465` uses implicit TLS; other ports upgrade with STARTTLS. |
| `SMTP_USERNAME` / `SMTP_PASSWORD` | | SMTP credentials. |
| `SMTP_FROM` | With `SMTP_HOST` | Sender address, such as `Fronko <no-reply@example.com>`. Must be an address your provider lets you send from. |
| `FEEDBACK_NOTIFY_EMAIL` | | Gets an email for each piece of product feedback, and is the Reply-To on admins' replies and on suspension emails. Without it, feedback only appears in the admin panel. |

The [backend configuration reference](backend/README.md#configuration) covers every option, including `PORT`, `TRUST_PROXY` and `STORAGE_ALLOW_PRIVATE_ENDPOINTS`, plus the fixed server and connection-pool settings.

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

1. Copy the card's public link from the Fronko dashboard, for example `https://cards.example.com/p/jane`.
2. Install an NFC writer app on iOS or Android, such as NFC Tools.
3. Choose **Write → URL/URI**, paste the link, and hold the phone to the blank card.

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

To use the admin panel locally, create an admin with `docker exec -it backend-local ./fronko admin create --email admin@fronko.local` and sign in at `http://localhost:3000/admin/login`.

## Documentation

| Document | Contents |
| -------- | -------- |
| [Backend](backend/README.md) | Setup, configuration, database schema, authentication and security model, testing |
| [API reference](backend/API.md) | Every endpoint, with request and response shapes and error codes |
| [Frontend](frontend/README.md) | Routes, session handling, the card data model, components and styling |

## Roadmap

Planned but not yet built:

- **Shared rate limiting.** Rate limits are currently kept in memory per backend instance. Running several instances needs a shared store such as Redis.
- **CRM integrations.** Push new leads to HubSpot or Salesforce.
- **Richer profile blocks.** More content types on the public card (calendars, embeds, galleries) and a block-based layout.

Ideas and feedback are welcome in [GitHub Issues](https://github.com/faiz-gh/fronko/issues).

## Contributing

Contributions are welcome, from bug reports and documentation fixes to new features. Read the [contributing guide](CONTRIBUTING.md) to set up a development environment, learn the coding guidelines, and submit a pull request.

## Security

If you find a security vulnerability, **please do not open a public issue.** Report it privately as described in the [security policy](SECURITY.md), which also covers how to harden a self-hosted deployment.

## License

Fronko is licensed under the [GNU General Public License v3.0](LICENSE).
