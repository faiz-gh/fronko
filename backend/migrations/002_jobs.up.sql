-- Background jobs (backend/internal/platform/jobs): a Postgres queue that
-- workers claim with FOR UPDATE SKIP LOCKED. Jobs are usually queued inside
-- the transaction that caused them, so they exist only if it commits.
CREATE TABLE jobs (
    job_id       BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- Which handler runs it, e.g. 'integrations.push_lead'.
    kind         TEXT NOT NULL,
    payload      JSONB NOT NULL DEFAULT '{}',
    -- The organisation it's for, if any; its jobs go when it does.
    org_id       BIGINT REFERENCES organizations (org_id) ON DELETE CASCADE,
    -- At most one queued or running job per (kind, dedupe_key).
    dedupe_key   TEXT,
    status       TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'done', 'dead')),
    run_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    attempts     INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 10 CHECK (max_attempts > 0),
    last_error   TEXT,
    -- When a worker claimed it; a running job older than the lease is reclaimed.
    locked_at    TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at  TIMESTAMPTZ
);
CREATE INDEX jobs_due_idx ON jobs (run_at, job_id) WHERE status = 'queued';
CREATE INDEX jobs_running_idx ON jobs (locked_at) WHERE status = 'running';
CREATE INDEX jobs_finished_idx ON jobs (finished_at) WHERE status IN ('done', 'dead');
CREATE INDEX jobs_org_idx ON jobs (org_id) WHERE org_id IS NOT NULL;
CREATE UNIQUE INDEX jobs_dedupe_idx ON jobs (kind, dedupe_key)
    WHERE dedupe_key IS NOT NULL AND status IN ('queued', 'running');
