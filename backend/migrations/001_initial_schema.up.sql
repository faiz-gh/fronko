-- IF NOT EXISTS keeps this safe on databases where the schema was applied by hand.

-- Users Table
CREATE TABLE IF NOT EXISTS users (
    user_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Case-insensitive index for fast username lookups during login
CREATE UNIQUE INDEX IF NOT EXISTS users_username_lower_idx ON users (LOWER(username));

-- Profiles Table
CREATE TABLE IF NOT EXISTS profiles (
    profile_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    slug TEXT NOT NULL UNIQUE,
    data JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Foreign key index (PostgreSQL does not auto-index FKs)
CREATE INDEX IF NOT EXISTS profiles_user_id_idx ON profiles(user_id);
-- Case-insensitive index for routing taps via slug (e.g. /p/faiz)
CREATE UNIQUE INDEX IF NOT EXISTS profiles_slug_lower_idx ON profiles (LOWER(slug));
-- GIN index to allow fast searching within the JSONB blocks if needed
CREATE INDEX IF NOT EXISTS profiles_data_gin_idx ON profiles USING GIN (data);

-- Leads Table
CREATE TABLE IF NOT EXISTS leads (
    lead_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    profile_id BIGINT NOT NULL REFERENCES profiles(profile_id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Foreign key index
CREATE INDEX IF NOT EXISTS leads_profile_id_idx ON leads(profile_id);
