-- Teams group people inside an organisation (Sales, Finance, …). A person can
-- be in several teams; a team's leads see its members' cards and leads and
-- look after the team's files.
CREATE TABLE IF NOT EXISTS teams (
    team_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    org_id BIGINT NOT NULL REFERENCES organizations(org_id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    -- #rrggbb, or '' for the default.
    color TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS teams_org_id_name_idx ON teams (org_id, lower(name));

CREATE TABLE IF NOT EXISTS team_members (
    team_id BIGINT NOT NULL REFERENCES teams(team_id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('lead', 'member')),
    added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (team_id, user_id)
);

CREATE INDEX IF NOT EXISTS team_members_user_id_idx ON team_members (user_id);

-- Files gain a fourth area (a team's files), a purpose that says what the
-- file is for, image size or page count, and an optional preview image.
ALTER TABLE files
    ADD COLUMN IF NOT EXISTS team_id BIGINT REFERENCES teams(team_id),
    ADD COLUMN IF NOT EXISTS purpose TEXT NOT NULL DEFAULT 'other',
    ADD COLUMN IF NOT EXISTS width INT,
    ADD COLUMN IF NOT EXISTS height INT,
    ADD COLUMN IF NOT EXISTS pages INT,
    -- Object key of a small preview (WebP or JPEG) next to the original.
    ADD COLUMN IF NOT EXISTS thumb_key TEXT,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

ALTER TABLE files DROP CONSTRAINT IF EXISTS files_area_check;
ALTER TABLE files ADD CONSTRAINT files_area_check CHECK (area IN ('personal', 'org', 'shared', 'team'));
ALTER TABLE files DROP CONSTRAINT IF EXISTS files_team_area_check;
ALTER TABLE files ADD CONSTRAINT files_team_area_check CHECK ((area = 'team') = (team_id IS NOT NULL));
ALTER TABLE files DROP CONSTRAINT IF EXISTS files_purpose_check;
ALTER TABLE files ADD CONSTRAINT files_purpose_check
    CHECK (purpose IN ('logo', 'banner', 'avatar', 'cover', 'gallery', 'brochure', 'other'));

CREATE INDEX IF NOT EXISTS files_org_id_purpose_created_at_idx ON files (org_id, purpose, created_at DESC);
CREATE INDEX IF NOT EXISTS files_team_id_idx ON files (team_id) WHERE team_id IS NOT NULL;

-- Org or shared files given to a whole team, alongside per-user file_grants.
CREATE TABLE IF NOT EXISTS file_team_grants (
    file_id BIGINT NOT NULL REFERENCES files(file_id) ON DELETE CASCADE,
    team_id BIGINT NOT NULL REFERENCES teams(team_id) ON DELETE CASCADE,
    granted_by BIGINT REFERENCES users(user_id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (file_id, team_id)
);

CREATE INDEX IF NOT EXISTS file_team_grants_team_id_idx ON file_team_grants (team_id);

-- Which cards use which files, kept in step with profiles.data on every save.
CREATE TABLE IF NOT EXISTS file_refs (
    file_id BIGINT NOT NULL REFERENCES files(file_id) ON DELETE CASCADE,
    profile_id BIGINT NOT NULL REFERENCES profiles(profile_id) ON DELETE CASCADE,
    slot TEXT NOT NULL CHECK (slot IN ('avatar', 'cover', 'document', 'gallery')),
    PRIMARY KEY (file_id, profile_id, slot)
);

CREATE INDEX IF NOT EXISTS file_refs_profile_id_idx ON file_refs (profile_id);

INSERT INTO file_refs (file_id, profile_id, slot)
SELECT DISTINCT f.file_id, r.profile_id, r.slot
FROM (
    SELECT profile_id, org_id, 'avatar' AS slot, data->>'avatar_file' AS public_id FROM profiles
    UNION ALL
    SELECT profile_id, org_id, 'cover', data->>'cover_file' FROM profiles
    UNION ALL
    SELECT profile_id, org_id, 'document', v #>> '{}'
    FROM profiles, jsonb_path_query(data, '$.documents[*].file') v
    UNION ALL
    SELECT profile_id, org_id, 'gallery', v #>> '{}'
    FROM profiles, jsonb_path_query(data, '$.blocks[*].images[*].file') v
) r
JOIN files f ON f.public_id = r.public_id AND f.org_id = r.org_id
WHERE r.public_id IS NOT NULL AND r.public_id <> ''
ON CONFLICT DO NOTHING;

-- Best guess at each existing file's purpose.
UPDATE files SET purpose = 'brochure' WHERE kind = 'pdf';
UPDATE files f SET purpose = 'logo'
FROM organizations o WHERE o.logo_file = f.public_id AND f.kind = 'image';
UPDATE files f SET purpose = 'banner'
FROM organizations o WHERE o.signature->>'banner_file' = f.public_id AND f.kind = 'image' AND f.purpose = 'other';
UPDATE files f SET purpose = r.slot
FROM (
    SELECT DISTINCT ON (file_id) file_id, slot FROM file_refs
    WHERE slot IN ('avatar', 'cover', 'gallery')
    ORDER BY file_id, CASE slot WHEN 'avatar' THEN 1 WHEN 'cover' THEN 2 ELSE 3 END
) r
WHERE f.file_id = r.file_id AND f.kind = 'image' AND f.purpose = 'other';
