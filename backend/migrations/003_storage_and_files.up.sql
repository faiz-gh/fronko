-- Each user's own S3-compatible bucket. Key material is encrypted by the
-- application (AES-256-GCM, key from SECRETS_KEY); only a 4-char hint is plain.
CREATE TABLE IF NOT EXISTS user_storage (
    user_id BIGINT PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    endpoint TEXT NOT NULL,
    region TEXT NOT NULL,
    bucket TEXT NOT NULL,
    path_style BOOLEAN NOT NULL DEFAULT false,
    access_key_id_enc BYTEA NOT NULL,
    secret_access_key_enc BYTEA NOT NULL,
    access_key_hint TEXT NOT NULL,
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The file library. Objects live in the user's bucket; public_id is the only
-- identifier exposed to visitors (unguessable, unlike file_id).
CREATE TABLE IF NOT EXISTS files (
    file_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id TEXT NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    bucket TEXT NOT NULL,
    object_key TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('image', 'pdf')),
    content_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    original_name TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS files_user_id_created_at_idx ON files (user_id, created_at DESC);
