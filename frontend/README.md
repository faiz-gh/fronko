# Fronko Frontend

The Fronko web app is a Svelte 5 + SvelteKit single-page app that compiles to static files and is served by nginx. It covers:

- **Landing page** (`/`), with a live demo card.
- **Auth** (`/login`, `/verify-email`, `/forgot-password`): sign in with a username or email, create an account, verify the email with a 6-digit code, and reset a forgotten password by emailed code.
- **Dashboard** (`/dashboard`): an app shell with a sidebar (cards, theme toggle), and an overview with stats, cards and recent leads.
- **Card editor** (`/dashboard/{id}`): edit a card with a live preview. Photo and brochures come from the file library. You can also view or export the card's leads.
- **Leads** (`/dashboard/leads`): all leads across cards, filtered by card, searchable, paginated and exportable.
- **Files** (`/dashboard/files`): the photo and PDF library stored in the user's own S3 bucket.
- **Settings** (`/dashboard/settings`): connect that bucket (R2, B2, AWS S3, MinIO).
- **Public card** (`/p/{slug}`): the page an NFC tap or QR scan opens. Visitors can save the contact as a vCard, share it, book a meeting, or send their own details back as a lead.

It talks to the [Go backend](../backend/README.md). The endpoints are listed in the [API reference](../backend/API.md).

---

## Contents

- [Tech stack](#tech-stack)
- [Getting started](#getting-started)
- [Project layout](#project-layout)
- [Routes](#routes)
- [How it talks to the backend](#how-it-talks-to-the-backend)
- [Session handling](#session-handling)
- [Card data model](#card-data-model)
- [Components](#components)
- [Styling & theming](#styling--theming)
- [Security notes](#security-notes)
- [Build & deployment](#build--deployment)
- [Conventions](#conventions)

---

## Tech stack

| Concern | Choice |
| ------- | ------ |
| Framework | Svelte 5 (runes) + SvelteKit 3, **SPA mode** (`ssr = false`) |
| Build | Vite 8, `@sveltejs/adapter-static` with an `index.html` fallback |
| Language | TypeScript (strict) |
| UI kit | [shadcn-svelte](https://shadcn-svelte.com) ("nova" style) on [bits-ui](https://bits-ui.com) |
| Styling | Tailwind CSS v4 (`@tailwindcss/vite`), `tw-animate-css`, Geist variable font |
| Icons | `@lucide/svelte` (UI), `simple-icons` (social and booking brands) |
| Phone numbers | `libphonenumber-js`: country list, as-you-type formatting, validation |
| Image cropping | `svelte-easy-crop`, plus canvas helpers in `$lib/image.ts` |
| Toasts | `svelte-sonner` |
| QR codes | `qrcode`, lazy-loaded |

## Getting started

### Prerequisites

- Node.js 24 (the version the Docker build uses). `.npmrc` sets `engine-strict=true`.
- A running backend on `localhost:8080`. See the [backend README](../backend/README.md#run-locally), and use `COOKIE_SECURE=false` for local HTTP.

### Run

```bash
cd frontend
npm install
npm run dev          # http://localhost:5173
```

The Vite dev server proxies `/api/*` and `/auth/*` to `http://localhost:8080` (`vite.config.ts`). The browser only ever sees one origin, so the session cookie works without any CORS setup.

### Scripts

| Script | What it does |
| ------ | ------------ |
| `npm run dev` | Vite dev server with HMR and the API proxy |
| `npm run build` | Static production build into `build/` |
| `npm run preview` | Serves the production build locally. **There's no API proxy here**, so API calls fail unless something else serves `/api` |
| `npm run check` | `svelte-kit sync` + `svelte-check` (type checking) |
| `npm run check:watch` | The same, in watch mode |

## Project layout

```
frontend/
├── src/
│   ├── app.css                    # Tailwind entry, design tokens (light/dark), shadcn variants
│   ├── app.html                   # HTML shell
│   ├── lib/
│   │   ├── api/
│   │   │   ├── client.ts          # fetch wrapper, ApiError, 401 and email_unverified handling
│   │   │   ├── auth.ts            # login / register / logout / me, email verify + change, password reset + change
│   │   │   ├── profile.ts         # profile CRUD + public lookup
│   │   │   ├── lead.ts            # submit / list leads
│   │   │   ├── storage.ts         # bucket connection settings
│   │   │   └── files.ts           # file library, uploadFile() with progress, fileUrl()
│   │   ├── card/card.ts           # CardData model, normalization, URL safety, vCard, brand detection
│   │   ├── card/qr.ts             # Lazy-loaded QR generation and PNG/SVG downloads
│   │   ├── phone.ts               # Country list, dial codes, formatting, validation, legacy phone parsing
│   │   ├── image.ts               # Canvas crop/rotate → WebP/JPEG File, used by ImageCropDialog
│   │   ├── components/
│   │   │   ├── app/               # App-specific components (see below)
│   │   │   └── ui/                # shadcn-svelte primitives (generated, see Conventions)
│   │   ├── session.svelte.ts      # Global reactive session store
│   │   ├── cooldown.svelte.ts     # Resend countdown for emailed codes (RESEND_COOLDOWN_SECONDS = 60)
│   │   ├── cards.svelte.ts        # The user's cards, shared by the sidebar and dashboard pages
│   │   ├── storage.svelte.ts      # Storage connection status (storage.ready), shared by Files/Settings/editor
│   │   ├── format.ts              # timeAgo(), formatDateTime(), plural()
│   │   ├── utils.ts               # cn() class merger + shadcn type helpers
│   │   └── assets/favicon.svg
│   └── routes/
│       ├── +layout.ts             # ssr = false, prerender = false
│       ├── +layout.svelte         # Global CSS, <Toaster>, kicks off session.load()
│       ├── +page.svelte           # Landing page
│       ├── login/+page.svelte
│       ├── verify-email/+page.svelte    # Enter the emailed code; add or correct the email
│       ├── forgot-password/+page.svelte # Request a reset code, then set a new password
│       ├── dashboard/
│       │   ├── +layout.svelte     # Auth guard + app shell (sidebar / mobile drawer)
│       │   ├── +page.svelte       # Overview: stats, cards grid, recent leads
│       │   ├── leads/+page.svelte # All leads: card filter, search, pagination, export
│       │   ├── files/+page.svelte # File library: upload, browse, rename, delete
│       │   ├── settings/+page.svelte # Account (email change), password, storage connection (S3 keys)
│       │   └── [id]/+page.svelte  # Card editor + leads
│       └── p/[slug]/+page.svelte  # Public card
├── static/
│   ├── robots.txt
│   └── config.js                  # Runtime settings placeholder (overwritten in Docker)
├── docker/
│   ├── default.conf.template      # nginx server + API proxy (env-templated)
│   └── 40-runtime-config.sh       # Writes config.js from API_URL at container start
├── components.json                # shadcn-svelte CLI config
├── Dockerfile
└── vite.config.ts
```

## Routes

| Path | Access | Purpose |
| ---- | ------ | ------- |
| `/` | Public | Marketing page with a demo `ProfileCard`. The header shows "Dashboard" or "Sign in" depending on the session |
| `/login` | Public | Sign in (username or email, with a "Forgot password?" link) and register (username, email, password) tabs. Query params: `mode=register` opens the register tab, `next=/path` sets where to go afterwards (only same-site relative paths are followed), `expired=1` shows a "session expired" notice and `reset=1` a "password updated" one. Unverified accounts go to `/verify-email` after signing in |
| `/verify-email` | Signed in, unverified | Six-slot code input (paste fills it, submits when complete), resend with a countdown, "Change email", and sign out. Accounts without an email start with an "Add your email" form. Verified users are sent on to `next` |
| `/forgot-password` | Public | Step 1: email → always "if an account exists, we've sent a code". Step 2: code, new password and confirmation → `/login?reset=1` |
| `/dashboard` | Signed in | Overview: stats (cards, leads all time, leads in the last 7 days), a grid of cards with copy link, QR code and delete actions, and the latest leads across all cards |
| `/dashboard/leads` | Signed in | Every lead across your cards. Filter by card (`?card=ID`, kept in the URL so it can be linked to), search (name, email, phone or message), page size and pages, a Refresh button that refetches leads and lead counts without reloading the page, and CSV export of everything that matches (including phone). Clicking a lead's card filters to that card |
| `/dashboard/files` | Signed in | The shared **file library**: drag-and-drop upload with progress, Photos/PDFs tabs, thumbnails, inline rename, "Used on" (which cards use each file), delete with a usage warning, pagination. Shows a "Connect storage" state until a bucket is connected |
| `/dashboard/settings` | Signed in | **Account**: username and email (with a Verified badge). **Change** opens `ChangeEmailForm`: new address + current password → a code sent to the new address → confirm; resend has a countdown, and the current email stays until confirmed. **Password**: current, new and confirm; changing it signs out every other session and keeps this one. **Storage**: choose a provider preset (R2, B2, AWS S3, MinIO, other; each shown with its icon via `StorageProviderIcon`), enter endpoint, bucket, region and keys, then Test connection or Connect. Keys are write-only: once saved, the fields show "Saved · ends in ABCD", and leaving them blank keeps them. Disconnect asks for confirmation. Shows a notice if the server has no `SECRETS_KEY` |
| `/dashboard/{id}` | Signed in | Editor. The **Card** tab has sections for Profile (with photo and 3:1 cover, both cropped before upload), Contact (email, mobile with country picker, website, booking link), Links (add, reorder, remove), Appearance (accent, light/dark theme) and Sharing (public slug, lead collection). A sticky preview pane on the right switches between the card and its QR code; below 1280px the preview opens in a dialog. A save bar with Discard appears when there are unsaved changes. The **Leads** tab shows the same paginated table, locked to this card. `?tab=leads` opens the Leads tab. `Ctrl/⌘+S` saves, and leaving with unsaved changes asks for confirmation |
| `/p/{slug}` | Public | The visitor-facing card. "Save contact" downloads a `.vcf`, plus share (Web Share API, falling back to the clipboard), a "Book a meeting" button when a booking link is set, and a lead form (name, email, optional mobile number, message) when `collect_leads` is on |

`/dashboard/+layout.svelte` acts as the auth guard. While the session check is pending it shows a spinner. If the check finds no session, it redirects to `/login?next=<current path>`; if the email isn't verified, to `/verify-email?next=<current path>`. Protected content never renders before the session is known.

Once signed in, the layout renders the app shell. From 1024px up, a fixed sidebar (`AppSidebar`) lists the user's cards. Below that width, a top bar opens the same sidebar as a slide-in drawer. The layout also loads `cards` (see `cards.svelte.ts`) and mounts the global "New card" dialog, which anything can open with `cards.createOpen = true`. After a create, save or delete, call `cards.upsert()` or `cards.remove()` so the sidebar and overview stay in sync without refetching.

## How it talks to the backend

All requests go through `apiClient` in `src/lib/api/client.ts`:

- **Base URL from runtime config.** `API_URL` comes from `window.__FRONKO_CONFIG__.apiUrl`, set by `/config.js` (loaded first in `app.html`). Empty (the default, and always in dev) means relative URLs: Vite or nginx proxies `/api` and `/auth`, so requests are same-origin. When set, e.g. `https://api.fronko.com`, requests go there directly and the backend must allow this site in `CORS_ALLOWED_ORIGINS`.
- **Credentials.** Requests use `credentials: 'include'` so the HttpOnly cookie is sent in both setups. Build any other backend URL with `apiUrl(path)` from `client.ts`. `uploadFile` (XHR, for progress) does the same and sets `withCredentials`.
- **JSON in and out.** `Content-Type: application/json` is set whenever there's a body, and a `204` resolves to `undefined`.
- **Errors** throw `ApiError(message, status, code?, retryAfter?)`:
  - `message` is the backend's `{"error": "..."}` text, which is written for users and can go straight into a toast or alert.
  - `code` is the backend's machine-readable `code`, when it sends one (e.g. `email_unverified`).
  - `retryAfter` is the `Retry-After` header in seconds, set on `429`s. The code screens use it to start their resend countdown.
  - A network failure throws with `status = 0` and a "Could not reach the server" message.
- **Expired sessions.** A `401` from any `/api/me/*` call runs `session.expire()`, which sends the user to `/login?next=…&expired=1`. You can opt out per call with `{ redirectOnUnauthorized: false }`. `me()` does this, because a 401 there just means nobody is signed in. Changing or resetting the password elsewhere also ends up here, because it revokes older sessions.
- **Unverified email.** A `403` with `code: "email_unverified"` runs `session.requireVerification()`, which sends the user to `/verify-email?next=…`.

The typed wrappers are in `api/auth.ts`, `api/profile.ts` and `api/lead.ts`. Leads are always read through `listLeads()` (one page) or `listAllLeads()` (walks every page, for exports) against the paginated `GET /api/me/leads`. Each returns a `Promise` of the response type (`AuthUser`, `Profile`, `PublicProfile`, `Lead`).

## Session handling

`src/lib/session.svelte.ts` exports a single `session` object that holds reactive `$state`:

| Member | Description |
| ------ | ----------- |
| `status` | `'unknown'` (still checking), `'authenticated'` or `'anonymous'` |
| `username` | Signed-in username, or `null` |
| `email` / `emailVerified` | The account's email (`null` for older accounts) and whether it's verified |
| `isAuthenticated` | `status === 'authenticated'` |
| `load()` | Calls `GET /api/me/user` once per page load. The root layout calls it. Safe to call repeatedly |
| `signIn(user)` | Called with the `AuthUser` from login, register or a verification call |
| `signOut()` | `POST /auth/logout`, clears state, goes to `/login` |
| `expire()` | Clears state and redirects to login, keeping the current path in `next` |
| `requireVerification()` | Called when an API call answers `403 email_unverified`; sends the user to `/verify-email` |

The JWT itself is never visible to JavaScript, because it lives in an HttpOnly cookie. `load()` also deletes the legacy `jwt_token` and `username` keys from `localStorage` that older builds left behind.

## Card data model

The backend stores each profile's `data` as opaque JSONB. The frontend defines the shape in `src/lib/card/card.ts`:

```ts
interface CardData {
  name: string;
  title: string;          // job title
  company: string;
  bio: string;
  avatar_url: string;     // must be http(s); used only when avatar_file is empty
  avatar_file: string;    // library photo (public file id); takes precedence
  cover_file: string;     // library image (public file id) for the 3:1 cover banner
  documents: CardDocument[]; // { id, file, title }: PDF brochures from the library, up to 10
  email: string;
  phone_country: string;      // ISO country ("IN"); +1 and others are shared, so the picker needs it
  phone_country_code: string; // dial code, "+91"; empty when there is no number
  phone_number: string;       // national number, digits only
  website: string;
  calendar_url: string;   // booking page (Calendly, Cal.com, …), shown as "Book a meeting"
  location: string;
  links: CardLink[];      // { id, label, url }, shown in order
  accent: AccentKey;      // 'indigo' | 'violet' | 'rose' | 'orange' | 'emerald' | 'sky' | 'slate'
  theme: 'light' | 'dark';
  collect_leads: boolean; // show the "exchange contact" form on the public page
}
```

**Always read stored data through `normalizeCard(raw)`.** It turns anything (missing fields, wrong types, `null`) into a complete `CardData`, with these defaults: accent `indigo`, theme `light`, `collect_leads: true`. It also upgrades links from older profiles that were saved as plain strings, and splits an old free-text `phone` into the dial code and number. A number without a leading `+` keeps its digits, and the editor asks for the country code. If you add a field, add it to `CardData`, `emptyCard()` and `normalizeCard()` so older profiles keep loading.

Other helpers in `card.ts`:

| Helper | Purpose |
| ------ | ------- |
| `emptyCard(name?)` | Default card for new profiles |
| `coverSrc(card)` | Cover banner URL from `cover_file`, or `null` to show the accent gradient |
| `detectCalendar(url)` | Recognizes Calendly, Cal.com, Google Calendar, HubSpot, Zoho Bookings, Microsoft Bookings and others, for the booking button |
| `avatarSrc(card)` | Photo URL: `avatar_file` (served via `/api/files/{id}`), then a safe `avatar_url`, else `null` for initials |
| `safeUrl(input)` | Returns an absolute `http(s)` URL or `null`. Adds `https://` when there's no scheme, and rejects `javascript:`, `data:` and similar |
| `displayUrl(input)` | Short form for display (`github.com/faiz`) |
| `detectBrand(url)` / `linkLabel(link)` | Recognizes about 65 sites by hostname (LinkedIn, GitHub, Indeed, Figma, Behance, YouTube, Substack, WhatsApp, PayPal and more), for icons and default labels. Icons come from `simple-icons`, plus a bundled LinkedIn path |
| `slugify(input)` / `isValidSlug(slug)` | Client-side copies of the backend's slug rules (3–48 chars, `^[a-z0-9]+(?:-[a-z0-9]+)*$`) |
| `buildVCard()` / `downloadVCard()` | Builds a vCard 3.0 file (escaped per the RFC) that iOS and Android both import |
| `downloadBlob(blob, filename)` | Generic client-side download |
| `publicUrl(slug)` | `${location.origin}/p/${slug}` |
| `initials(name)` | Avatar fallback text |

## Components

App-specific components live in `src/lib/components/app/`:

| Component | Props | Description |
| --------- | ----- | ----------- |
| `ProfileCard` | `card`, `slug`, `actions?` (snippet), `files?`, `class?` | Renders a card: a header in the accent colour (or the cover image with an accent stripe, and an accent ring around the avatar), avatar, quick actions (email, call, website), a "Book a meeting" button when `calendar_url` is set, bio and links. It uses a container query: stacked when narrow, two columns (identity left, links right) once its container is at least 42rem wide, as on the public page at desktop width. Shows a **Brochures** list. When `files` (metadata keyed by file id, from the public profile response) is given, it adds sizes and hides brochures whose file was deleted |
| `FileDropzone` | `kind?`, `onuploaded?`, `compact?`, `multiple?`, `disabled?` | Drag-and-drop or click to upload, with per-file progress and inline errors. It checks type and size on the client for quick feedback; the server re-checks by sniffing |
| `FilePickerDialog` | `open` (bindable), `kind`, `title`, `selected?`, `onselect` | Pick a photo or PDF from the library (paginated), or upload a new one inline. Used by the editor's Photo and Brochures fields |
| `FileThumb` | `id`, `kind`, `class?` | Image thumbnail (lazy, via the file redirect) or a PDF tile |
| `AppSidebar` | `onnavigate?` | Dashboard navigation: overview link, card list with lead counts, account menu |
| `CreateCardDialog` | none (opened through `cards.createOpen`) | Name and slug form. Creates the card and opens the editor |
| `DeleteCardDialog` | `target` (bindable), `ondeleted?` | Confirms and deletes a card, then updates `cards` |
| `FormSection` | `title`, `description?`, `id?` | Editor section. The heading sits beside the fields at 1536px and wider, above them otherwise |
| `CardAvatar` | `card`, `fallback`, `class?` | Avatar using the card's photo or initials on its accent colour |
| `QrCode` | `url`, `svg` (bindable), `class?` | Renders a QR code. `qrcode` is loaded the first time one is shown. Codes are always dark-on-white so they scan reliably |
| `QrDialog` | `open` (bindable), `slug`, `name`, `dark?` | `QrCode` in a dialog, with SVG and 1024px PNG downloads and copy link |
| `LeadsTable` | `profileId?`, `card?`, `oncardchange?`, `filename?` | Server-paginated leads table: card filter (unless locked with `profileId`), debounced search, a Refresh button (it also reloads the cards store so lead counts update), pagination and CSV export of every matching lead, including phone. A new filter or search goes back to page 1, and paging scrolls the table back into view. The Card column appears only when showing all cards |
| `Pagination` | `page` (bindable), `pageSize` (bindable), `total`, `pageSizes?`, `disabled?` | "1–25 of 67", per-page menu, previous/next and page numbers with gaps (`1 … 4 5 6 … 12`) |
| `CardFilter` | `value`, `onchange` | "All cards" or one card, with lead counts |
| `BrandIcon` | `url`, `kind?` (`'link'` or `'calendar'`), `class?` | Brand icon for a social or booking URL, falling back to a generic link or calendar icon |
| `PhoneInput` | `country`, `code`, `number` (all bindable), `id?`, `invalid?`, `contentClass?` | Searchable country picker plus a number field that formats as you type (`libphonenumber-js`). Writes digits only, and writes nothing until the user edits, so it never marks a form dirty. Helpers live in `$lib/phone.ts` |
| `ImageCropDialog` | `file` (bindable; set it to open), `aspect`, `shape`, `outputWidth`, `outputHeight`, `onconfirm` | Crop, zoom and rotate a freshly picked image (`svelte-easy-crop`), then hand back a WebP (or JPEG) `File`. The editor uses 1:1 / 512px for photos and 3:1 / 1500×500 for covers |
| `Logo` | `href?`, `class?` | Wordmark link |
| `AuthLayout` | `children` (snippet) | The split frame shared by `/login`, `/verify-email` and `/forgot-password`: logo and form column on the left, sample card on the right (from 1024px) |
| `CodeInput` | `value` (bindable), `id?`, `disabled?`, `invalid?`, `oncomplete?` | Six-slot input for emailed codes (shadcn `input-otp`, digits only, `autocomplete="one-time-code"`). Pasting fills every slot; `oncomplete` fires once all six are in |
| `ChangeEmailForm` | `onclose` | Settings flow for changing a verified email: new address + current password → code sent to the new address → confirm, with resend countdown and "Use a different address". Updates the session and toasts on success |

`src/lib/components/ui/` holds **shadcn-svelte primitives** (button, card, dialog, dropdown-menu, field, tabs, table and others). The shadcn CLI generates them, so prefer re-adding or updating them with the CLI over editing them by hand:

```bash
npx shadcn-svelte@latest add <component>
```

> **Note:** shadcn-svelte CLI v1.7 can't resolve SvelteKit 3's `"extends": "$app/tsconfig"` and fails with `File '$app/tsconfig' not found`. Until that's fixed upstream, `popover`, `command`, `slider` and `input-group` were taken directly from the registry (`https://shadcn-svelte.com/registry/styles/nova/<name>.json`). Their `$UI$` paths were rewritten to `$lib/components/ui`, and their `IconPlaceholder` elements were replaced with `@lucide/svelte` icons. Do the same if you add another component before the CLI is fixed. (`input-otp` was later added with the CLI without trouble, so try it first.)

## Styling & theming

- **Tailwind v4** is configured CSS-first in `src/app.css`. There's no `tailwind.config.js`.
- **Design tokens** are OKLCH CSS variables (`--background`, `--primary`, `--radius` and so on) defined on `:root` and overridden under `.dark`. The `dark:` variant matches any descendant of `.dark`.
- **Palette.** The UI uses graphite neutrals, and the primary (action) colour is ink. Indigo (`--brand`, with `bg-brand`, `text-brand` and `bg-brand-soft`) is reserved for the logo, lead-count badges and focus rings, so use it sparingly. Each card's own accent colour belongs to the card, not the app.
- **Dashboard dark mode.** `$lib/theme.svelte.ts` stores a Light, Dark or System preference in localStorage (`fronko-theme`), and the sidebar's `ThemeToggle` sets it. The dashboard layout adds `.dark` to `<html>` only while you're in `/dashboard`, so portalled dialogs, menus and toasts match, and the marketing and public pages stay light. An inline script in `app.html` applies the theme before first paint on dashboard URLs. `ProfileCard` always scopes itself with `.dark` or `.light`, so a light card previews as light inside the dark dashboard. The `dark:` variant skips anything inside `.light` for the same reason.
- **Utilities.** `bg-dots` draws the faint dot grid used behind previews, and `tabular` sets tabular numerals for counts.
- **Layout widths.** App pages are full width (the overview caps at 1680px), and the sidebar is `w-68`. Breakpoints that change the structure: `lg` (1024px) shows the sidebar, `xl` (1280px) shows the editor's preview pane, `2xl` (1536px) puts section headings beside the fields and moves recent leads into their own column.
- **Per-card theming.** `ProfileCard` sets `--card-accent` from `ACCENTS[card.accent]`. The public page wraps the card in a `.dark` element when `card.theme === 'dark'`, so a single card can be dark without switching the whole app.
- Use `cn()` from `$lib/utils` to merge conditional classes. It resolves Tailwind conflicts.

## Security notes

- **No tokens in JavaScript.** Auth relies only on the HttpOnly cookie.
- **Visitor-facing URLs go through `safeUrl()`.** That covers the avatar, website, booking link and links, so a card owner can't inject `javascript:` links.
- **Open-redirect protection.** `/login` and `/verify-email` only follow `next` values that start with `/` and not `//`.
- **No account enumeration from the UI.** `/forgot-password` always shows the same "if an account exists" message.
- **CSV injection.** Lead exports prefix cells starting with `=`, `+`, `-`, `@`, tab or CR with `'`, because lead content comes from anonymous visitors.
- **Validation is mirrored, not trusted.** Slug, username, password and email checks in the UI exist for quick feedback. The backend enforces the real rules.

## Build & deployment

```bash
npm run build      # → build/
```

The output is fully static: `index.html` is the SPA fallback, and hashed assets go under `_app/immutable/`.

**Docker.** `Dockerfile` builds with `node:24-alpine` and serves `build/` from `nginx:alpine`. The image is configured at start-up through environment variables, so one build works anywhere:

| Variable | Default | Purpose |
| -------- | ------- | ------- |
| `PORT` | `3000` | Port nginx listens on (`EXPOSE 3000`) |
| `BACKEND_UPSTREAM` | `http://backend:8080` | Where nginx proxies `/api/` and `/auth/` |
| `API_URL` | empty | Public backend URL for the browser, written to `/config.js` by `docker/40-runtime-config.sh`. Empty keeps API calls same-origin through nginx |

The nginx image renders `docker/default.conf.template` with `envsubst` (only defined variables, so nginx's own `$uri` etc. are untouched). The config:

- falls back to `index.html` for client-side routes (`try_files $uri $uri/ /index.html`)
- caches `/_app/immutable/` for a year (`immutable`), and serves `/config.js` with `no-store`
- proxies `/api/` and `/auth/` to `BACKEND_UPSTREAM`, preserving `Host` (so the backend's same-origin check matches the browser's `Origin`) and setting `X-Real-IP` (used by the backend's rate limiter when `TRUST_PROXY=true`)

The `backend` hostname comes from the Docker Compose network. See `deploy/docker-compose.yml` and the [root README](../README.md).

## Conventions

- **Svelte 5 runes only.** Use `$state`, `$derived`, `$effect` and `$props`, not stores or `export let`.
- **Imports** use the `$lib` alias. SvelteKit 3 no longer provides it by default, so `vite.config.ts` and `tsconfig.json` define it. A `#lib` subpath import is also set up in `package.json` for a later migration.
- **API calls** go through `src/lib/api/*`. Don't call `fetch` directly from components.
- **Errors.** Catch `ApiError` and show `e.message`. Check `e.status` for specific cases, for example a 404 on the public page.
- **Icons.** Import them individually (`@lucide/svelte/icons/<name>`) to keep the bundle small.
