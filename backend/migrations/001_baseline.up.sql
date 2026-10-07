-- Fronko baseline schema.
--
-- Migrations 001-013 were squashed into this file before the first public
-- release (see docs/adr/0005-squashed-baseline.md). New schema changes go in
-- new numbered migrations after it; never edit this file once released.
--
-- Tables are grouped by the module that owns them (backend/internal/<module>).

-- ---------------------------------------------------------------------------
-- orgs, users: organisations and the people in them
-- ---------------------------------------------------------------------------

CREATE TABLE organizations (
    org_id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name                TEXT NOT NULL,
    -- The organisation's part of every card link, /p/{handle}/{slug}.
    handle              TEXT NOT NULL,
    -- Storage limit given to new users; NULL is unlimited.
    default_quota_bytes BIGINT CHECK (default_quota_bytes IS NULL OR default_quota_bytes >= 0),
    -- Branding (module branding): logo, logo policy and signature settings.
    logo_file           TEXT,
    logo_policy         TEXT NOT NULL DEFAULT 'optional' CHECK (logo_policy IN ('required', 'optional')),
    signature           JSONB NOT NULL DEFAULT '{}',
    -- Set while a platform admin has suspended the organisation.
    suspended_at        TIMESTAMPTZ,
    suspended_reason    TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX organizations_handle_lower_idx ON organizations (LOWER(handle));

CREATE TABLE users (
    user_id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    org_id               BIGINT NOT NULL REFERENCES organizations (org_id) ON DELETE CASCADE,
    role                 TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('owner', 'admin', 'member')),
    username             TEXT NOT NULL,
    email                TEXT,
    email_verified_at    TIMESTAMPTZ,
    password_hash        TEXT NOT NULL,
    -- Set while the password is one the organisation chose.
    must_change_password BOOLEAN NOT NULL DEFAULT false,
    -- Limit on the user's personal files; NULL is unlimited.
    storage_quota_bytes  BIGINT CONSTRAINT users_storage_quota_check
                             CHECK (storage_quota_bytes IS NULL OR storage_quota_bytes >= 0),
    suspended_at         TIMESTAMPTZ,
    created_by           BIGINT REFERENCES users (user_id) ON DELETE SET NULL,
    last_login_at        TIMESTAMPTZ,
    -- Carried in session tokens; bumping it signs the user out everywhere.
    session_version      INTEGER NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_username_lower_idx ON users (LOWER(username));
CREATE UNIQUE INDEX users_email_lower_idx ON users (LOWER(email));
CREATE UNIQUE INDEX users_one_owner_per_org_idx ON users (org_id) WHERE role = 'owner';
CREATE INDEX users_org_id_idx ON users (org_id);

-- One-time codes sent by email. Only an HMAC of the code is stored.
CREATE TABLE email_codes (
    user_id    BIGINT NOT NULL REFERENCES users (user_id) ON DELETE CASCADE,
    purpose    TEXT NOT NULL CHECK (purpose IN ('verify_email', 'reset_password', 'change_email')),
    -- The address a change_email code was sent to.
    email      TEXT,
    code_hash  TEXT NOT NULL,
    attempts   INTEGER NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, purpose)
);

-- ---------------------------------------------------------------------------
-- teams
-- ---------------------------------------------------------------------------

CREATE TABLE teams (
    team_id     BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    org_id      BIGINT NOT NULL REFERENCES organizations (org_id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    color       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX teams_org_id_name_idx ON teams (org_id, LOWER(name));

CREATE TABLE team_members (
    team_id  BIGINT NOT NULL REFERENCES teams (team_id) ON DELETE CASCADE,
    user_id  BIGINT NOT NULL REFERENCES users (user_id) ON DELETE CASCADE,
    role     TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('lead', 'member')),
    added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (team_id, user_id)
);
CREATE INDEX team_members_user_id_idx ON team_members (user_id);

-- ---------------------------------------------------------------------------
-- cards
-- ---------------------------------------------------------------------------

CREATE TABLE profiles (
    profile_id       BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    org_id           BIGINT NOT NULL REFERENCES organizations (org_id) ON DELETE CASCADE,
    -- The account that created the card.
    user_id          BIGINT NOT NULL REFERENCES users (user_id) ON DELETE CASCADE,
    -- The one user who works on the card; NULL means the organisation holds it.
    assigned_user_id BIGINT REFERENCES users (user_id) ON DELETE SET NULL,
    slug             TEXT NOT NULL,
    -- The card's blocks and settings; opaque to the backend except file references.
    data             JSONB NOT NULL DEFAULT '{}',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX profiles_org_slug_lower_idx ON profiles (org_id, LOWER(slug));
CREATE INDEX profiles_org_id_idx ON profiles (org_id);
CREATE INDEX profiles_user_id_idx ON profiles (user_id);
CREATE INDEX profiles_assigned_user_id_idx ON profiles (assigned_user_id);
CREATE INDEX profiles_data_gin_idx ON profiles USING GIN (data);

-- ---------------------------------------------------------------------------
-- leads
-- ---------------------------------------------------------------------------

CREATE TABLE leads (
    lead_id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    profile_id         BIGINT NOT NULL REFERENCES profiles (profile_id) ON DELETE CASCADE,
    -- Who held the card when the lead arrived; the lead stays theirs.
    assigned_user_id   BIGINT REFERENCES users (user_id) ON DELETE SET NULL,
    name               TEXT NOT NULL,
    email              TEXT NOT NULL,
    phone_country_code TEXT,
    phone_number       TEXT,
    notes              TEXT,
    -- How the visitor reached the card.
    source             TEXT CHECK (source IN ('nfc', 'qr', 'link')),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Both phone parts or neither; digits only, E.164 length.
    CONSTRAINT leads_phone_format CHECK (
        (phone_country_code IS NULL AND phone_number IS NULL)
        OR (phone_country_code ~ '^\+[1-9][0-9]{0,2}$'
            AND phone_number ~ '^[0-9]{4,14}$'
            AND length(phone_country_code) - 1 + length(phone_number) <= 15)
    )
);
CREATE INDEX leads_profile_id_created_at_idx ON leads (profile_id, created_at DESC);
CREATE INDEX leads_assigned_user_id_created_at_idx ON leads (assigned_user_id, created_at DESC);

-- ---------------------------------------------------------------------------
-- files: the library, its access grants, card references and bucket settings
-- ---------------------------------------------------------------------------

-- The organisation's S3-compatible bucket, stored against the owner's account.
-- Keys are sealed with SECRETS_KEY (AES-256-GCM) and never leave the server.
CREATE TABLE user_storage (
    user_id               BIGINT PRIMARY KEY REFERENCES users (user_id) ON DELETE CASCADE,
    provider              TEXT NOT NULL,
    endpoint              TEXT NOT NULL,
    region                TEXT NOT NULL,
    bucket                TEXT NOT NULL,
    path_style            BOOLEAN NOT NULL DEFAULT false,
    access_key_id_enc     BYTEA NOT NULL,
    secret_access_key_enc BYTEA NOT NULL,
    access_key_hint       TEXT NOT NULL,
    verified_at           TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE files (
    file_id       BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- The only identifier visitors see: 16 random bytes, base64url.
    public_id     TEXT NOT NULL UNIQUE,
    org_id        BIGINT NOT NULL REFERENCES organizations (org_id) ON DELETE CASCADE,
    -- The uploader; for personal files, the owner.
    user_id       BIGINT NOT NULL REFERENCES users (user_id) ON DELETE CASCADE,
    area          TEXT NOT NULL CHECK (area IN ('personal', 'org', 'shared', 'team')),
    team_id       BIGINT REFERENCES teams (team_id),
    -- Username of a deleted user whose personal file this was.
    former_owner  TEXT,
    bucket        TEXT NOT NULL,
    object_key    TEXT NOT NULL,
    thumb_key     TEXT,
    kind          TEXT NOT NULL CHECK (kind IN ('image', 'pdf')),
    purpose       TEXT NOT NULL DEFAULT 'other'
                      CHECK (purpose IN ('logo', 'banner', 'avatar', 'cover', 'gallery', 'brochure', 'other')),
    content_type  TEXT NOT NULL,
    size_bytes    BIGINT NOT NULL,
    original_name TEXT NOT NULL,
    title         TEXT NOT NULL DEFAULT '',
    width         INTEGER,
    height        INTEGER,
    pages         INTEGER,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT files_team_area_check CHECK ((area = 'team') = (team_id IS NOT NULL))
);
CREATE INDEX files_org_id_area_created_at_idx ON files (org_id, area, created_at DESC);
CREATE INDEX files_org_id_purpose_created_at_idx ON files (org_id, purpose, created_at DESC);
CREATE INDEX files_user_id_created_at_idx ON files (user_id, created_at DESC);
CREATE INDEX files_team_id_idx ON files (team_id) WHERE team_id IS NOT NULL;

-- Extra files individual people may use.
CREATE TABLE file_grants (
    file_id    BIGINT NOT NULL REFERENCES files (file_id) ON DELETE CASCADE,
    user_id    BIGINT NOT NULL REFERENCES users (user_id) ON DELETE CASCADE,
    granted_by BIGINT REFERENCES users (user_id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (file_id, user_id)
);
CREATE INDEX file_grants_user_id_idx ON file_grants (user_id);

-- Extra files whole teams may use.
CREATE TABLE file_team_grants (
    file_id    BIGINT NOT NULL REFERENCES files (file_id) ON DELETE CASCADE,
    team_id    BIGINT NOT NULL REFERENCES teams (team_id) ON DELETE CASCADE,
    granted_by BIGINT REFERENCES users (user_id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (file_id, team_id)
);
CREATE INDEX file_team_grants_team_id_idx ON file_team_grants (team_id);

-- Which card uses which file, and where. Rewritten on every card save.
CREATE TABLE file_refs (
    file_id    BIGINT NOT NULL REFERENCES files (file_id) ON DELETE CASCADE,
    profile_id BIGINT NOT NULL REFERENCES profiles (profile_id) ON DELETE CASCADE,
    slot       TEXT NOT NULL CHECK (slot IN ('avatar', 'cover', 'document', 'gallery')),
    PRIMARY KEY (file_id, profile_id, slot)
);
CREATE INDEX file_refs_profile_id_idx ON file_refs (profile_id);

-- ---------------------------------------------------------------------------
-- analytics: what visitors do on public cards, without identifying them
-- ---------------------------------------------------------------------------

CREATE TABLE card_events (
    event_id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    org_id           BIGINT NOT NULL REFERENCES organizations (org_id) ON DELETE CASCADE,
    profile_id       BIGINT NOT NULL REFERENCES profiles (profile_id) ON DELETE CASCADE,
    -- Who held the card when the event happened.
    assigned_user_id BIGINT REFERENCES users (user_id) ON DELETE SET NULL,
    session_id       UUID,
    -- sha256(daily salt, address, browser, card): stable for one day only.
    visitor_hash     BYTEA NOT NULL,
    type             TEXT NOT NULL CHECK (type IN ('view', 'click', 'scroll', 'doc_open', 'gallery_open',
                                                   'vcard', 'form_open', 'form_submit', 'share', 'leave')),
    source           TEXT NOT NULL DEFAULT 'link' CHECK (source IN ('nfc', 'qr', 'link')),
    target           TEXT NOT NULL DEFAULT '',
    label            TEXT NOT NULL DEFAULT '',
    value            INTEGER NOT NULL DEFAULT 0,
    device           TEXT NOT NULL DEFAULT 'desktop' CHECK (device IN ('mobile', 'tablet', 'desktop')),
    referrer_host    TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX card_events_org_idx ON card_events (org_id, created_at);
CREATE INDEX card_events_profile_idx ON card_events (profile_id, created_at);
CREATE INDEX card_events_user_idx ON card_events (org_id, assigned_user_id, created_at);

-- One random salt per day for visitor hashes; old ones are deleted.
CREATE TABLE analytics_salts (
    day  DATE PRIMARY KEY,
    salt BYTEA NOT NULL
);

-- ---------------------------------------------------------------------------
-- platformadmin, feedback: the people running the server
-- ---------------------------------------------------------------------------

CREATE TABLE platform_admins (
    admin_id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email           TEXT NOT NULL,
    password_hash   TEXT NOT NULL,
    session_version INTEGER NOT NULL DEFAULT 1,
    last_login_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX platform_admins_email_idx ON platform_admins (LOWER(email));

CREATE TABLE admin_audit_log (
    log_id      BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    admin_id    BIGINT REFERENCES platform_admins (admin_id) ON DELETE SET NULL,
    admin_email TEXT NOT NULL,
    action      TEXT NOT NULL,
    target_type TEXT,
    target_id   BIGINT,
    detail      JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX admin_audit_log_created_at_idx ON admin_audit_log (created_at DESC);

CREATE TABLE feedback (
    feedback_id  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    org_id       BIGINT REFERENCES organizations (org_id) ON DELETE SET NULL,
    user_id      BIGINT REFERENCES users (user_id) ON DELETE SET NULL,
    -- Copied at the time, so the inbox still reads after an account is deleted.
    sender_email TEXT NOT NULL,
    org_name     TEXT NOT NULL,
    category     TEXT NOT NULL CHECK (category IN ('bug', 'idea', 'other')),
    rating       SMALLINT CHECK (rating BETWEEN 1 AND 5),
    message      TEXT NOT NULL CHECK (char_length(message) BETWEEN 1 AND 5000),
    page_path    TEXT,
    status       TEXT NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'read', 'resolved')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX feedback_created_at_idx ON feedback (created_at DESC);
CREATE INDEX feedback_status_created_at_idx ON feedback (status, created_at DESC);

CREATE TABLE feedback_replies (
    reply_id    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    feedback_id BIGINT NOT NULL REFERENCES feedback (feedback_id) ON DELETE CASCADE,
    admin_id    BIGINT REFERENCES platform_admins (admin_id) ON DELETE SET NULL,
    body        TEXT NOT NULL CHECK (char_length(body) BETWEEN 1 AND 5000),
    email_sent  BOOLEAN NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX feedback_replies_feedback_id_idx ON feedback_replies (feedback_id, created_at);

-- Daily usage totals per organisation (aggregates only) for the admin trends.
CREATE TABLE org_usage_snapshots (
    snapshot_date      DATE NOT NULL,
    org_id             BIGINT NOT NULL REFERENCES organizations (org_id) ON DELETE CASCADE,
    user_count         INTEGER NOT NULL,
    team_count         INTEGER NOT NULL DEFAULT 0,
    card_count         INTEGER NOT NULL,
    lead_count         INTEGER NOT NULL,
    file_count         INTEGER NOT NULL,
    storage_used_bytes BIGINT NOT NULL,
    storage_connected  BOOLEAN NOT NULL,
    PRIMARY KEY (snapshot_date, org_id)
);
CREATE INDEX org_usage_snapshots_org_idx ON org_usage_snapshots (org_id, snapshot_date);

CREATE TABLE platform_usage_snapshots (
    snapshot_date      DATE PRIMARY KEY,
    org_count          INTEGER NOT NULL,
    new_orgs           INTEGER NOT NULL,
    user_count         INTEGER NOT NULL,
    team_count         INTEGER NOT NULL DEFAULT 0,
    card_count         INTEGER NOT NULL,
    lead_count         INTEGER NOT NULL,
    file_count         INTEGER NOT NULL,
    orgs_with_storage  INTEGER NOT NULL,
    orgs_with_teams    INTEGER NOT NULL DEFAULT 0,
    orgs_with_logo     INTEGER NOT NULL DEFAULT 0,
    storage_used_bytes BIGINT NOT NULL,
    feedback_count     INTEGER NOT NULL
);
