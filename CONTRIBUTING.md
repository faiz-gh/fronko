# Contributing to Fronko

Thank you for your interest in contributing. Bug reports, documentation fixes, tests and new features are all welcome.

## Table of contents

- [Ways to contribute](#ways-to-contribute)
- [Reporting bugs](#reporting-bugs)
- [Suggesting features](#suggesting-features)
- [Proposing an integration](#proposing-an-integration)
- [Setting up a development environment](#setting-up-a-development-environment)
- [Demo data](#demo-data)
- [Project structure](#project-structure)
- [Coding guidelines](#coding-guidelines)
- [Testing](#testing)
- [Submitting a pull request](#submitting-a-pull-request)
- [License](#license)

## Ways to contribute

- Report bugs and confirm or reproduce existing reports.
- Suggest features or improvements to existing ones.
- Improve the documentation, including the READMEs and the [API reference](backend/API.md).
- Add tests for code paths that aren't covered.
- Fix an open issue. Issues labelled `good first issue` are a good place to start.

If you've found a security vulnerability, don't open an issue. Follow the [security policy](SECURITY.md) instead.

## Reporting bugs

Before opening an issue, search the [existing issues](https://github.com/faiz-gh/fronko/issues) to check that it hasn't been reported already.

A good bug report includes:

- what you did, what you expected to happen, and what happened instead
- steps to reproduce it, ideally from a fresh account
- the commit you're running, and how you deployed it (Docker Compose, local development, reverse proxy)
- your browser and device, for frontend issues
- relevant backend logs or browser console errors, with secrets and personal data removed

## Suggesting features

Open an issue that describes the problem you want to solve, not only the solution you have in mind. Explain who it helps and how it would fit into the existing product. For larger changes, please wait for a maintainer to discuss the approach before you start writing code, so your work isn't wasted.

## Proposing an integration

Integrations (CRMs, booking pages, directories, identity providers) are the easiest place to contribute: each one is a folder and a line in a registry.

1. Check the Integrations page or [`comingsoon.go`](backend/internal/integrations/providers/comingsoon.go): the provider may already be planned.
2. Open an [integration request](https://github.com/faiz-gh/fronko/issues/new?template=integration_request.yml) with the provider's API documentation and what people would use it for. Say if you'd like to build it.
3. Once a maintainer agrees the shape (which data goes where, how it authenticates), follow [Writing an integration provider](docs/integrations/writing-a-provider.md). It ends with a checklist for the pull request, including a setup guide under `docs/integrations/`.

Providers must use the organisation's own credentials ([ADR 0004](docs/adr/0004-per-organisation-oauth-apps.md)), make every outbound request through the client the core provides, and come with tests.

## Setting up a development environment

You'll need:

- Go 1.27 or later
- Node.js 24
- PostgreSQL 14 or later
- the [`migrate` CLI](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate), built with the `postgres` tag

Fork the repository, clone your fork, and follow the setup guides:

- [Backend: Run locally](backend/README.md#run-locally): environment variables, migrations and starting the API on `localhost:8080`
- [Frontend: Getting started](frontend/README.md#getting-started): the Vite dev server on `localhost:5173`, which proxies API calls to the backend

Alternatively, `deploy/docker-compose.local.yml` runs the full stack in Docker against a PostgreSQL on your machine, along with Mailpit to catch outgoing email. See [Local development](README.md#local-development) in the main README.

## Demo data

`fronko seed` fills an empty development database with a demo organisation, so you don't have to build one by hand. It refuses to run unless `FRONKO_ENV=development`, because the accounts have published passwords.

```bash
cd backend && make seed                 # against DATABASE_URL; ARGS=--reset empties the database first
docker exec -e FRONKO_ENV=development backend-local ./fronko seed --reset   # in the local Docker stack
```

| Sign in at | Account | Password | What it is |
| ---------- | ------- | -------- | ---------- |
| `/login` | `uidemo` | `password123` | Owner of **Lumen Labs** (handle `uidemo`) |
| `/login` | `uidemo-rep` | `reppassword1` | A member of Lumen Labs, lead of its Sales team |
| `/login` | `uidemo-empty` | `password123` | Owner of an empty organisation |
| `/admin/login` | `admin@fronko.local` | `adminpassword123` | Platform admin |

Lumen Labs has three cards (`maya-chen`, `maya-speaker` held by `uidemo-rep`, and `lumen-sales`), 68 leads, about 45 days of demo analytics, a verified email domain (`lumenlabs.example`) and an organisation booking page (a made-up Calendly link). Demo leads don't trigger lead sync.

## Project structure

| Path | Contents |
| ---- | -------- |
| `backend/` | Go API server: one package per feature under `internal/` (cards, leads, files, integrations …), shared infrastructure under `internal/platform/`, and SQL migrations |
| `frontend/` | Svelte 5 single-page app: one folder per feature under `src/lib/features/`, plus the app shell in `src/lib/core/` and shared components |
| `docs/` | Architecture, how-to guides, integration setup guides and decision records |
| `deploy/` | Docker Compose files and the environment template |

Start with [Architecture](docs/architecture.md). [Backend modules](docs/backend-modules.md) and [Frontend structure](docs/frontend-structure.md) explain where new code goes; the [backend README](backend/README.md#project-layout) and [frontend README](frontend/README.md#project-layout) describe each directory.

## Coding guidelines

### General

- Keep pull requests focused on one change. Separate refactoring from behaviour changes.
- Match the style of the surrounding code: naming, comments and structure.
- Update the documentation in the same pull request as the change. New or changed endpoints must be reflected in [`backend/API.md`](backend/API.md), and user-facing behaviour in the relevant README.
- Don't add a new dependency without a good reason. Explain why it's needed in the pull request.

### Backend (Go)

- Format code with `gofmt` (CI also runs `go vet` and `golangci-lint`; `make lint` runs the latter locally).
- Prefer the standard library. Modules are wired by hand in `internal/server/server.go`; there's no DI framework. Modules use each other through small interfaces passed in there, never by reaching into another module's SQL.
- Keep a feature's SQL in its own `store.go`, use parameterised queries, and enforce ownership in the `WHERE` clause (organisation, plus `auth.Scope` for what members may see).
- Register routes on the group that says who may call them (`r.Public`, `r.User`, `r.Admin` …); don't build middleware chains by hand.
- Respond with the helpers in `internal/platform/httpx`. Error messages are shown to end users as they are, so write them for people, and never return driver or internal details.
- Slow or fallible follow-up work goes in a background job (`platform/jobs`), queued inside the transaction that caused it.
- Schema changes go in a new numbered pair of migrations in `backend/migrations` (`NNN_description.up.sql` and `NNN_description.down.sql`). Never edit a migration that has already been merged.

### Frontend (Svelte)

- Use Svelte 5 runes (`$state`, `$derived`, `$effect`, `$props`), not stores or `export let`.
- Keep a feature's code in its folder under `src/lib/features/`. Make API calls through the feature's `api.ts`, not with `fetch` in components.
- Add pages to the sidebar through `core/nav.ts`, and settings through `core/settings-tabs.ts`.
- Build UI from the existing shadcn-svelte components and design tokens. See [Styling and theming](frontend/README.md#styling--theming).
- Import icons individually (`@lucide/svelte/icons/<name>`).
- Check that your change works on a phone-sized screen, and in both light and dark mode in the dashboard.

## Testing

Run these before opening a pull request (CI runs the same):

```bash
# Backend
cd backend
gofmt -l .              # must print nothing
make lint               # golangci-lint v2
make test               # unit tests, with the race detector
make test-integration   # database tests; needs a disposable database

# Frontend
cd frontend
npm run lint            # Prettier check + ESLint
npm run check           # type checking
npm test                # Vitest unit tests
npm run build           # production build
```

The integration tests truncate every table, so they read `TEST_DATABASE_URL` rather than `DATABASE_URL`. Never point them at a database you care about. The [backend testing guide](backend/README.md#testing) shows how to start a throwaway database with Docker.

Add or update tests for the code you change. Bug fixes should include a test that fails without the fix.
## Submitting a pull request

1. Create a branch in your fork from the latest `master`.
2. Make your change, with tests and documentation.
3. Run the checks under [Testing](#testing).
4. Write clear commit messages that explain what changed and why.
5. Open a pull request against `master`. In the description:
   - link the issue it addresses (for example, `Closes #123`)
   - summarise the change and anything reviewers should look at closely
   - add screenshots or a short recording for UI changes
   - mention any new migrations, environment variables or breaking changes
6. Respond to review feedback by pushing further commits to the same branch.

A maintainer will review your pull request as soon as they can. Small, focused pull requests are reviewed faster.

## License

Fronko is licensed under the [GNU General Public License v3.0](LICENSE). By submitting a contribution, you agree that it is licensed under the same terms.
