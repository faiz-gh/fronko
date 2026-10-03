# 📇 Fronko: The Open-Source Digital Business Card

Fronko is a lightweight, self-hostable digital business card platform designed for frictionless networking. It combines NFC, QR codes, and a zero-friction mobile web experience to replace traditional paper business cards.

Built as a high-performance alternative to proprietary platforms like Mobilo and Popl, Fronko gives individuals, freelancers, and businesses complete ownership of their data, profiles, and routing logic without enterprise lock-in.

## ✨ Core Features

* **⚡ Frictionless Exchange:** The recipient does not need an app. Tapping an NFC card or scanning a QR code instantly renders a lightning-fast web profile.

* **📱 One-tap Save to Contacts:** Visitors download a vCard 3.0 (`.vcf`), which both iOS and Android import directly.

* **🎨 Card Editor:** A live-preview editor for profile details, contact info, social links (with brand icons), a photo, PDF brochures, an accent colour and a light or dark theme. You can keep several cards, each with its own link and QR code.

* **🤝 Lead Capture:** A built-in form lets recipients share their details back. Leads land in one inbox with card filters, search, pagination and CSV export.

* **☁️ Bring Your Own Storage:** Photos and brochures go to each user's own S3-compatible bucket (Cloudflare R2, Backblaze B2, AWS S3, MinIO). Keys are encrypted at rest and buckets stay private.

## 🏗️ Architecture & Data Flow

When a networking event happens, speed is everything. A tap or scan opens a static, app-less web page that loads the card with a single API call.

```mermaid
graph TD
    A[NFC tap / QR scan] -->|opens /p/slug| C{App-less public card}
    E[Dashboard: card editor] -->|saves card data| D[(PostgreSQL)]
    E -->|uploads photo / PDFs| S[(User's own S3 bucket)]
    D -->|card + file metadata| C
    S -->|short-lived signed links| C
    C -->|Save contact| F[vCard download]
    C -->|Share your contact| G[Lead form]
    G -->|stores lead| D
    D -->|leads inbox, CSV export| E
```

### 💻 The Tech Stack

* **Backend:** Go (Standard Library `net/http`). High concurrency, ultra-low memory footprint, and near-zero latency for tap routing.

* **Frontend:** Svelte 5 (Vite SPA) + shadcn-svelte. Compiles to tiny vanilla JavaScript for instantaneous mobile loading over cellular networks without the overhead of SSR.

* **Database:** PostgreSQL. Strict relational integrity for users, with `JSONB` support for dynamic profile blocks.

* **Storage:** Each user's own S3-compatible bucket (R2, B2, S3, MinIO), reached through aws-sdk-go-v2. Credentials are sealed with AES-256-GCM.

* **Deployment:** Multi-container Docker Compose (Nginx for static frontend serving).

## 📚 Documentation

* **[Backend](backend/README.md):** setup, configuration, database schema, auth and security model, testing.
* **[API Reference](backend/API.md):** every endpoint with its request and response shapes and error codes.
* **[Frontend](frontend/README.md):** routes, session handling, the card data model, components and styling.

## 🚀 Getting Started (Self-Hosting)

Fronko is deployed with Docker Compose. The stack is two containers: the Go **backend**, and an nginx **frontend** that serves the SPA and proxies `/api` and `/auth` to the backend. PostgreSQL is **not** part of the stack, so bring your own instance. The backend runs database migrations automatically on startup.

### 1. Clone the repository

```bash
git clone https://github.com/faiz-gh/fronko.git
cd fronko/deploy
```

### 2. Configure environment variables

```bash
cp .env.example .env
```

*Example `deploy/.env`:*

```env
# PostgreSQL (managed externally)
DATABASE_URL=postgres://fronko_admin:password@your-db-host:5432/fronko_db?sslmode=require

# At least 16 characters, e.g. `openssl rand -base64 32`
JWT_SECRET=replace-with-a-long-random-string

# The session cookie is HTTPS-only by default. Set to false only for plain-http testing.
# COOKIE_SECURE=false

# Optional: enables photo and brochure uploads to each user's own S3 bucket.
# Encrypts their keys at rest. Keep it stable and backed up.
SECRETS_KEY=generate-with-openssl-rand-base64-32
```

With `SECRETS_KEY` set, each user connects their own bucket (Cloudflare R2, Backblaze B2, AWS S3 or MinIO) under **Settings** in the dashboard. Fronko never needs a bucket of its own.

For every backend option, see the [configuration reference](backend/README.md#configuration).

### 3. Spin up the containers

```bash
docker compose up -d --build
```

* The app is served at `http://localhost:3000`. The API sits on the same origin under `/api` and `/auth`.
* The backend isn't exposed directly. Only the frontend container publishes a port.
* In production, put a TLS-terminating reverse proxy in front of port 3000. With the default `COOKIE_SECURE=true`, the session cookie is only sent over HTTPS.

### Local development

* **Without Docker:** run the backend and frontend separately. See [backend → Run locally](backend/README.md#run-locally) and [frontend → Getting started](frontend/README.md#getting-started).
* **With Docker against a Postgres on your machine:** `docker compose -f docker-compose.local.yml up --build`. Edit its hard-coded `DATABASE_URL` first. This file also sets a development `SECRETS_KEY` and `STORAGE_ALLOW_PRIVATE_ENDPOINTS=true`, so storage can point at a local S3 server such as MinIO or SeaweedFS on `http://host.docker.internal:9000`.

## 🪣 Connecting storage (photos & brochures)

Each user connects their own bucket under **Dashboard → Settings → Storage**. The server only needs `SECRETS_KEY` set (see above). Buckets stay **private**: visitors get 15-minute signed links. You don't need public access, a custom domain or CORS rules, because uploads go through the Fronko backend.

**Keys need:** write and delete on objects in the bucket, and read for serving. Nothing account-wide. Fronko checks this when you save, by writing and deleting a `.fronko-probe` object.

| Provider | Endpoint | Region | Path-style | Keys |
| -------- | -------- | ------ | :--------: | ---- |
| **Cloudflare R2** | `https://<ACCOUNT_ID>.r2.cloudflarestorage.com` (EU jurisdiction: `https://<ACCOUNT_ID>.eu.r2.cloudflarestorage.com`) | `auto` | On | R2 → Manage API tokens → Create API token → **Object Read & Write**, scoped to the bucket |
| **Backblaze B2** | `https://s3.<region>.backblazeb2.com` (shown on the bucket page) | e.g. `us-west-004` | Off | Application Keys → new key with **Read and Write** for the bucket only (keyID / applicationKey) |
| **AWS S3** | `https://s3.<region>.amazonaws.com` | the bucket's region | Off | IAM user with `s3:GetObject`, `s3:PutObject`, `s3:DeleteObject` on `arn:aws:s3:::<bucket>/*` |
| **MinIO / other** | your HTTPS S3 API URL | `us-east-1` unless configured | On | access key with a read/write policy on the bucket |

Uploaded files are stored as `fronko/<user_id>/<file_id>.<ext>`. Keep the bucket private, and don't enable a public `r2.dev` URL; Fronko doesn't need one.

## 💳 Writing to Physical NFC Cards

Fronko is hardware-agnostic. You can purchase any blank NTAG215 or NTAG216 PVC card from Amazon.

1. Generate your profile URL in the Fronko dashboard (e.g., `https://yourdomain.com/p/faiz`).

2. Download a free NFC writer app (like NFC Tools) on iOS or Android.

3. Select "Write URI", paste your generated URL, and hold your phone to the blank card.

## 🗺️ Backlog

Planned, but not built yet:

* **Account email & verification:** add an `email` column to users, send a one-time code (OTP) to confirm it at sign-up, and support password recovery by email. Needs an SMTP/transactional email provider.
* **Shared rate limiting:** rate limits are kept in memory per backend instance. Running several instances needs a shared store such as Redis.
* **CRM integrations:** push new leads to HubSpot or Salesforce.
* **Richer profile blocks:** more content types on the public card (calendars, embeds, galleries) and a block-based layout.

## 🤝 Contributing

We welcome contributions from the open-source community! Whether it is adding new profile blocks, expanding CRM integrations (HubSpot, Salesforce), or optimizing the Svelte UI, please submit a pull request.

Please ensure all Go code passes `gofmt` and Svelte components adhere to the existing `shadcn-svelte` design system.
