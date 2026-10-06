-- Platform admins run the Fronko service itself. They are separate accounts,
-- not organisation users, and sign in on their own page with their own cookie.
CREATE TABLE IF NOT EXISTS platform_admins (
    admin_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    -- Carried in admin session tokens; bumping it revokes them all.
    session_version INT NOT NULL DEFAULT 1,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS platform_admins_email_idx ON platform_admins (LOWER(email));

-- A suspended organisation can't sign in and its public cards are unavailable.
ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS suspended_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS suspended_reason TEXT;

-- Product feedback from signed-in users. The sender's email and org name are
-- copied in, so feedback still reads correctly after either is deleted.
CREATE TABLE IF NOT EXISTS feedback (
    feedback_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    org_id BIGINT REFERENCES organizations(org_id) ON DELETE SET NULL,
    user_id BIGINT REFERENCES users(user_id) ON DELETE SET NULL,
    sender_email TEXT NOT NULL,
    org_name TEXT NOT NULL,
    category TEXT NOT NULL CHECK (category IN ('bug', 'idea', 'other')),
    rating SMALLINT CHECK (rating BETWEEN 1 AND 5),
    message TEXT NOT NULL CHECK (char_length(message) BETWEEN 1 AND 5000),
    page_path TEXT,
    status TEXT NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'read', 'resolved')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS feedback_status_created_at_idx ON feedback (status, created_at DESC);
CREATE INDEX IF NOT EXISTS feedback_created_at_idx ON feedback (created_at DESC);

-- Replies a platform admin emailed to the sender.
CREATE TABLE IF NOT EXISTS feedback_replies (
    reply_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    feedback_id BIGINT NOT NULL REFERENCES feedback(feedback_id) ON DELETE CASCADE,
    admin_id BIGINT REFERENCES platform_admins(admin_id) ON DELETE SET NULL,
    body TEXT NOT NULL CHECK (char_length(body) BETWEEN 1 AND 5000),
    email_sent BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS feedback_replies_feedback_id_idx ON feedback_replies (feedback_id, created_at);

-- Daily usage counts per organisation, for trends. Aggregates only.
CREATE TABLE IF NOT EXISTS org_usage_snapshots (
    snapshot_date DATE NOT NULL,
    org_id BIGINT NOT NULL REFERENCES organizations(org_id) ON DELETE CASCADE,
    user_count INT NOT NULL,
    card_count INT NOT NULL,
    lead_count INT NOT NULL,
    file_count INT NOT NULL,
    storage_used_bytes BIGINT NOT NULL,
    storage_connected BOOLEAN NOT NULL,
    PRIMARY KEY (snapshot_date, org_id)
);
CREATE INDEX IF NOT EXISTS org_usage_snapshots_org_idx ON org_usage_snapshots (org_id, snapshot_date);

-- Daily platform totals, kept apart so history survives deleted organisations.
CREATE TABLE IF NOT EXISTS platform_usage_snapshots (
    snapshot_date DATE PRIMARY KEY,
    org_count INT NOT NULL,
    user_count INT NOT NULL,
    card_count INT NOT NULL,
    lead_count INT NOT NULL,
    file_count INT NOT NULL,
    orgs_with_storage INT NOT NULL,
    storage_used_bytes BIGINT NOT NULL,
    new_orgs INT NOT NULL,
    feedback_count INT NOT NULL
);

-- What platform admins did: suspensions, replies, status changes, sign-ins.
CREATE TABLE IF NOT EXISTS admin_audit_log (
    log_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    admin_id BIGINT REFERENCES platform_admins(admin_id) ON DELETE SET NULL,
    admin_email TEXT NOT NULL,
    action TEXT NOT NULL,
    target_type TEXT,
    target_id BIGINT,
    detail JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS admin_audit_log_created_at_idx ON admin_audit_log (created_at DESC);
