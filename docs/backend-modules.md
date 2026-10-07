# Adding a backend module

A module is one feature: its routes, handlers, SQL and types in one package under `backend/internal/`. This page walks through adding one, using a made-up **notes** feature (people keep private notes on leads) as the example. Read [Architecture](architecture.md) first for the rules modules follow.

## 1. The package

```
internal/notes/
├── note.go       # types and validation
├── store.go      # all SQL for the feature's tables
├── handler.go    # HTTP handlers
├── module.go     # Routes (and, if needed, Subscribe, JobHandlers, Tasks)
└── handler_test.go
```

Small modules can merge files; big ones split by topic (`files/` has `storage_handler.go` and `storage_store.go`). Keep the names: people look for SQL in `store.go`.

## 2. The migration

Add the next numbered pair in `backend/migrations/` (see [Adding a migration](../backend/README.md#adding-a-migration)):

```sql
-- 005_notes.up.sql
CREATE TABLE lead_notes (
    note_id    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    org_id     BIGINT NOT NULL REFERENCES organizations (org_id) ON DELETE CASCADE,
    lead_id    BIGINT NOT NULL REFERENCES leads (lead_id) ON DELETE CASCADE,
    user_id    BIGINT REFERENCES users (user_id) ON DELETE SET NULL,
    body       TEXT NOT NULL CHECK (length(body) BETWEEN 1 AND 5000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX lead_notes_lead_idx ON lead_notes (lead_id);
```

Every table carries `org_id`, so every query can filter by organisation and deleting an organisation removes everything in it. Put comments in the SQL: the migration is where the schema is documented.

## 3. The store

```go
// Store runs the SQL for lead notes.
type Store struct{ db *pgxpool.Pool }

func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db} }

func (s *Store) Add(ctx context.Context, n *Note) error {
	err := s.db.QueryRow(ctx, `
		INSERT INTO lead_notes (org_id, lead_id, user_id, body) VALUES ($1, $2, $3, $4)
		RETURNING note_id, created_at`,
		n.OrgID, n.LeadID, n.UserID, n.Body,
	).Scan(&n.ID, &n.CreatedAt)
	return database.MapError(err)
}
```

- Parameterise every query. Never build SQL from user input.
- Filter by `org_id` in the `WHERE` clause, and for anything members see, by `auth.Scope` too. `auth.VisibleTo(col, n)` gives the SQL that limits rows to what the caller may see (their own, plus their teammates' for team leads). A row someone can't see should look exactly like a missing one.
- Return errors through `database.MapError`, which turns "no rows" into `database.ErrNotFound` and unique violations into `database.ErrConflict`. Handlers never see Postgres codes.
- Methods that must run inside someone else's transaction take a `database.Querier` instead of using the pool.

## 4. The handler

```go
// Protected: POST /api/me/leads/{id}/notes
func (h *Handler) Add(w http.ResponseWriter, r *http.Request) {
	leadID, ok := httpx.PathID(w, r, "id", "lead")
	if !ok {
		return
	}
	var in struct{ Body string `json:"body"` }
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	p := auth.PrincipalFrom(r.Context())
	…
	if err := h.store.Add(r.Context(), n); err != nil {
		httpx.Internal("add note", err).Write(w)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, n)
}
```

- Decode with `httpx.DecodeJSON` (it caps the body and writes the 400 for you) and respond with `httpx.WriteJSON` / `httpx.WriteError`.
- Error messages go to users as they are. Write them for people, and never include driver or internal details. `httpx.Internal` logs the real error and answers a generic 500.
- Start each handler with a comment naming its group and route (`// Protected: …`, `// Public: …`), like the rest of the code.

## 5. Routes

```go
// Routes registers lead notes.
func (h *Handler) Routes(r *app.Routes) {
	r.User("GET /api/me/leads/{id}/notes", h.List)
	r.User("POST /api/me/leads/{id}/notes", h.Add)
	r.Admin("DELETE /api/org/notes/{id}", h.Delete)
}
```

Pick the group that says who may call the route (`Public`, `External`, `SignedIn`, `Verified`, `User`, `Admin`, `Owner`, `PlatformAdmin`); the group applies the middleware. Signed-in routes live under `/api/<area>/`. Wrap a route in `r.RateLimit(every, burst)(h.X)` for its own per-IP budget, or `r.AuthLimit(h.X)` for anything that checks a password or sends an email code.

## 6. Events, jobs and tasks (optional)

- **React to another module.** Implement `Subscribe(b *events.Bus)` and register with `events.Subscribe(b, func(ctx, q, e leads.Created) error { … })`. The subscriber runs inside the publisher's transaction, so use `q`, not the pool, and stay quick.
- **Background work.** Queue it with `jobs.Enqueue(ctx, q, "notes.something", payload, jobs.Options{OrgID: …})` and implement `JobHandlers() map[string]jobs.Handler`. Handlers run at least once, so make them idempotent; wrap errors that retrying can't fix in `jobs.Permanent`.
- **Periodic work.** Implement `Tasks() []jobs.Task`. Each task runs once at start-up, then on its interval, under an advisory lock.
- **Publish your own event.** Define a past-tense struct with an `EventName()` method in your package, and `bus.Publish(ctx, tx, evt)` inside the transaction that made the change.

## 7. Wire it up

In [`internal/server/server.go`](../backend/internal/server/server.go), create the store with the others, then add the module to the `modules` list:

```go
noteStore = notes.NewStore(pool)
…
modules := []app.Module{
	…
	notes.NewHandler(noteStore, leadStore),
	…
}
```

If the module needs something from another module, define a small interface in your package (`type LeadChecker interface { … }`) and pass the other module's store or service in here. Don't import `server` or reach into another module's SQL.

## 8. Tests and docs

- **Unit tests** sit next to the code (`handler_test.go`) and run with `make test`. Test validation, error mapping and anything with branches.
- **Database tests** go in `internal/integrationtest/` behind the `integration` build tag, using the shared harness there. They run with `make test-integration` against a disposable database ([Testing](../backend/README.md#testing)).
- **Docs.** Add the endpoints to [`backend/API.md`](../backend/API.md) (the summary table and a section each), and the tables to [Database](../backend/README.md#database) in the backend README.
- **Frontend.** If the feature has pages, add a folder under `frontend/src/lib/features/` and a sidebar entry in `core/nav.ts` (see [Frontend structure](frontend-structure.md)).
