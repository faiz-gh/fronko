-- Integrations (backend/internal/integrations): connections to outside
-- services such as CRMs and booking pages, and a log of what each one did.

-- One connection to a provider. It belongs to the organisation (user_id NULL,
-- managed by admins) or to one person (user_id set, managed by them).
CREATE TABLE integration_connections (
    connection_id  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    org_id         BIGINT NOT NULL REFERENCES organizations (org_id) ON DELETE CASCADE,
    user_id        BIGINT REFERENCES users (user_id) ON DELETE CASCADE,
    -- The provider's id in the registry, e.g. 'webhook' or 'hubspot'.
    provider       TEXT NOT NULL,
    category       TEXT NOT NULL CHECK (category IN ('lead_sync', 'calendar', 'directory', 'sso')),
    name           TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 80),
    enabled        BOOLEAN NOT NULL DEFAULT true,
    -- pending: set up but not ready (e.g. not authorised yet); error: stopped
    -- after repeated failures until someone fixes it.
    status         TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'error')),
    -- The provider's settings, minus secrets.
    config         JSONB NOT NULL DEFAULT '{}',
    -- Secrets (API keys, OAuth tokens) as one JSON object sealed with
    -- SECRETS_KEY and bound to this row's id.
    secrets        BYTEA,
    last_error     TEXT,
    last_error_at  TIMESTAMPTZ,
    -- Failures in a row that retrying didn't fix; reset by any success.
    failure_count  INTEGER NOT NULL DEFAULT 0,
    last_synced_at TIMESTAMPTZ,
    created_by     BIGINT REFERENCES users (user_id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX integration_connections_org_idx ON integration_connections (org_id, category);
CREATE INDEX integration_connections_user_idx ON integration_connections (user_id) WHERE user_id IS NOT NULL;

-- What a connection did: each lead sent (and each retry), tests, and
-- changes such as authorising. Kept for 90 days.
CREATE TABLE integration_activity (
    activity_id   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    connection_id BIGINT NOT NULL REFERENCES integration_connections (connection_id) ON DELETE CASCADE,
    kind          TEXT NOT NULL CHECK (kind IN ('push_lead', 'test', 'setup')),
    -- retrying: failed, will be tried again; failed: gave up.
    outcome       TEXT NOT NULL CHECK (outcome IN ('success', 'retrying', 'failed')),
    summary       TEXT NOT NULL,
    -- Details for troubleshooting, e.g. the HTTP status and a response excerpt.
    detail        JSONB,
    lead_id       BIGINT REFERENCES leads (lead_id) ON DELETE SET NULL,
    attempt       INTEGER,
    -- Who did it, for actions people take.
    user_id       BIGINT REFERENCES users (user_id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX integration_activity_connection_idx ON integration_activity (connection_id, activity_id DESC);
CREATE INDEX integration_activity_created_idx ON integration_activity (created_at);
CREATE INDEX integration_activity_lead_idx ON integration_activity (lead_id) WHERE lead_id IS NOT NULL;
