-- Users created by organisations can't exist without one; remove them first.
-- Their cards stay with the org owner, and their personal files are forgotten
-- (the objects remain in the bucket).
DELETE FROM users WHERE role <> 'owner';

DROP TABLE IF EXISTS file_grants;

DROP INDEX IF EXISTS files_org_id_area_created_at_idx;
ALTER TABLE files DROP CONSTRAINT IF EXISTS files_area_check;
ALTER TABLE files
    DROP COLUMN IF EXISTS former_owner,
    DROP COLUMN IF EXISTS area,
    DROP COLUMN IF EXISTS org_id;

DROP INDEX IF EXISTS leads_assigned_user_id_created_at_idx;
ALTER TABLE leads DROP COLUMN IF EXISTS assigned_user_id;

DROP INDEX IF EXISTS profiles_assigned_user_id_idx;
DROP INDEX IF EXISTS profiles_org_id_idx;
ALTER TABLE profiles
    DROP COLUMN IF EXISTS assigned_user_id,
    DROP COLUMN IF EXISTS org_id;

DROP INDEX IF EXISTS users_one_owner_per_org_idx;
DROP INDEX IF EXISTS users_org_id_idx;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_storage_quota_check;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users
    DROP COLUMN IF EXISTS last_login_at,
    DROP COLUMN IF EXISTS created_by,
    DROP COLUMN IF EXISTS suspended_at,
    DROP COLUMN IF EXISTS storage_quota_bytes,
    DROP COLUMN IF EXISTS must_change_password,
    DROP COLUMN IF EXISTS role,
    DROP COLUMN IF EXISTS org_id;

DROP TABLE IF EXISTS organizations;
