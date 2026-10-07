# 0001. Organise both apps by feature

- **Status:** Accepted
- **Date:** 2026-10-07

## Context

The backend was split by layer: one `handlers/` package, one `repository/` package holding all SQL, one `models/` package, and a `main.go` that wired about 90 routes and their middleware by hand. Files had grown past 1,000 lines. The frontend was the same shape: a flat `lib/api/`, a flat `lib/components/app/`, and route files of up to 1,354 lines.

Integrations were next, with more to follow. In the layered layout, each one would have touched `handlers/`, `repository/`, `models/` and `main.go`, and every feature change meant editing four unrelated places. There were no users, printed cards or real organisations yet, so a large move was cheap.

## Decision

Both apps are organised by feature.

**Backend.** Each feature is a package under `internal/` (`cards`, `leads`, `files`, `integrations` …) that owns its handlers, its SQL (`store.go`) and its types. Infrastructure without business rules lives in `internal/platform/*`. A module implements `app.Module` (`Routes(r *app.Routes)`) and, optionally, `Subscriber`, `Worker` and `Scheduled`. Modules register routes on groups (`Public`, `User`, `Admin` …) that apply the middleware, instead of building chains themselves. `internal/server` is the composition root and wires everything by hand. Modules depend on each other through small interfaces passed in at wiring time; there are no import cycles and no DI framework.

**Frontend.** Each feature is a folder under `src/lib/features/` holding its API calls, types, stores and components. App-wide plumbing is in `src/lib/core/`, cross-feature components in `src/lib/components/shared/`. The sidebar and the Settings tabs are registries (`core/nav.ts`, `core/settings-tabs.ts`), so a feature adds itself with one entry. Routes stay thin.

## Consequences

- A feature change, or a new feature, is mostly one folder per app.
- The route groups make the access level of every endpoint visible where it's registered, and a signed-in area can't be mounted without the session chain.
- Wiring in `server.go` is explicit and long. That's deliberate: the whole dependency graph reads top to bottom in one file.
- Some types live in the module that owns them and are imported elsewhere (for example `leads.Created`); shared references (`auth.UserRef`, roles) live in `auth`.
- Database integration tests stayed in one package (`internal/integrationtest`) with a shared harness, rather than being split per module, because they share fixtures across modules.
- The move was behaviour-preserving: endpoints and responses stayed byte for byte the same.
