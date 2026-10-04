-- Organisations: the account that registers is an organisation's owner, and
-- the org creates lower-access users who work on cards assigned to them.
CREATE TABLE IF NOT EXISTS organizations (
    org_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL,
    -- Storage limit given to newly created users; NULL means unlimited.
    default_quota_bytes BIGINT CHECK (default_quota_bytes IS NULL OR default_quota_bytes >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS org_id BIGINT REFERENCES organizations(org_id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'owner',
    -- Set when the org picks the password (new user, reset); cleared once the user sets their own.
    ADD COLUMN IF NOT EXISTS must_change_password BOOLEAN NOT NULL DEFAULT false,
    -- Limit on the user's personal files; NULL means unlimited.
    ADD COLUMN IF NOT EXISTS storage_quota_bytes BIGINT,
    ADD COLUMN IF NOT EXISTS suspended_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS created_by BIGINT REFERENCES users(user_id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('owner', 'admin', 'member'));
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_storage_quota_check;
ALTER TABLE users ADD CONSTRAINT users_storage_quota_check
    CHECK (storage_quota_bytes IS NULL OR storage_quota_bytes >= 0);

-- Every existing account becomes the owner of its own organisation.
DO $$
DECLARE
    u RECORD;
    new_org BIGINT;
BEGIN
    FOR u IN SELECT user_id, username FROM users WHERE org_id IS NULL LOOP
        INSERT INTO organizations (name) VALUES (u.username) RETURNING org_id INTO new_org;
        UPDATE users SET org_id = new_org, role = 'owner' WHERE user_id = u.user_id;
    END LOOP;
END $$;

ALTER TABLE users ALTER COLUMN org_id SET NOT NULL;
ALTER TABLE users ALTER COLUMN role SET DEFAULT 'member';

CREATE INDEX IF NOT EXISTS users_org_id_idx ON users (org_id);
-- Exactly one owner per organisation.
CREATE UNIQUE INDEX IF NOT EXISTS users_one_owner_per_org_idx ON users (org_id) WHERE role = 'owner';

-- Cards belong to the organisation; user_id stays as the account that created
-- the card. At most one user works on a card at a time.
ALTER TABLE profiles
    ADD COLUMN IF NOT EXISTS org_id BIGINT REFERENCES organizations(org_id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS assigned_user_id BIGINT REFERENCES users(user_id) ON DELETE SET NULL;

UPDATE profiles p SET org_id = u.org_id FROM users u WHERE u.user_id = p.user_id AND p.org_id IS NULL;
ALTER TABLE profiles ALTER COLUMN org_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS profiles_org_id_idx ON profiles (org_id);
CREATE INDEX IF NOT EXISTS profiles_assigned_user_id_idx ON profiles (assigned_user_id);

-- Who held the card when the lead arrived. Leads stay with that person when
-- the card is reassigned. NULL means the card was unassigned (the org's).
ALTER TABLE leads
    ADD COLUMN IF NOT EXISTS assigned_user_id BIGINT REFERENCES users(user_id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS leads_assigned_user_id_created_at_idx ON leads (assigned_user_id, created_at DESC);

-- Files live in one of three areas: a user's personal files, the org's own
-- (private) files, or the shared area every user in the org can see.
ALTER TABLE files
    ADD COLUMN IF NOT EXISTS org_id BIGINT REFERENCES organizations(org_id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS area TEXT NOT NULL DEFAULT 'org',
    -- Username of the user whose personal files these were, after that user was deleted.
    ADD COLUMN IF NOT EXISTS former_owner TEXT;

UPDATE files f SET org_id = u.org_id FROM users u WHERE u.user_id = f.user_id AND f.org_id IS NULL;
ALTER TABLE files ALTER COLUMN org_id SET NOT NULL;
ALTER TABLE files ALTER COLUMN area DROP DEFAULT;

ALTER TABLE files DROP CONSTRAINT IF EXISTS files_area_check;
ALTER TABLE files ADD CONSTRAINT files_area_check CHECK (area IN ('personal', 'org', 'shared'));

CREATE INDEX IF NOT EXISTS files_org_id_area_created_at_idx ON files (org_id, area, created_at DESC);

-- Extra files a user may see beyond their own and the shared area.
CREATE TABLE IF NOT EXISTS file_grants (
    file_id BIGINT NOT NULL REFERENCES files(file_id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    granted_by BIGINT REFERENCES users(user_id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (file_id, user_id)
);

CREATE INDEX IF NOT EXISTS file_grants_user_id_idx ON file_grants (user_id);
