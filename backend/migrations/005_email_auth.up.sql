-- Email on accounts. Nullable because accounts created before this migration
-- have none; the app makes them add and verify one on their next sign-in.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS email TEXT,
    ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMPTZ,
    -- Carried in the session JWT; bumping it signs out every existing session.
    ADD COLUMN IF NOT EXISTS session_version INT NOT NULL DEFAULT 0;

CREATE UNIQUE INDEX IF NOT EXISTS users_email_lower_idx ON users (LOWER(email));

-- One-time codes sent by email. At most one live code per user and purpose;
-- issuing a new one replaces the old. Only an HMAC of the code is stored.
CREATE TABLE IF NOT EXISTS email_codes (
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    purpose TEXT NOT NULL CHECK (purpose IN ('verify_email', 'reset_password')),
    code_hash TEXT NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, purpose)
);
