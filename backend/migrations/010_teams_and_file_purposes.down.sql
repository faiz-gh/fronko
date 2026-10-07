DROP TABLE IF EXISTS file_refs;
DROP TABLE IF EXISTS file_team_grants;

UPDATE files SET area = 'org', team_id = NULL WHERE area = 'team';

DROP INDEX IF EXISTS files_team_id_idx;
DROP INDEX IF EXISTS files_org_id_purpose_created_at_idx;
ALTER TABLE files DROP CONSTRAINT IF EXISTS files_purpose_check;
ALTER TABLE files DROP CONSTRAINT IF EXISTS files_team_area_check;
ALTER TABLE files DROP CONSTRAINT IF EXISTS files_area_check;
ALTER TABLE files ADD CONSTRAINT files_area_check CHECK (area IN ('personal', 'org', 'shared'));

ALTER TABLE files
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS thumb_key,
    DROP COLUMN IF EXISTS pages,
    DROP COLUMN IF EXISTS height,
    DROP COLUMN IF EXISTS width,
    DROP COLUMN IF EXISTS purpose,
    DROP COLUMN IF EXISTS team_id;

DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS teams;
