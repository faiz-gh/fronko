DROP INDEX IF EXISTS teams_org_external_id_idx;
ALTER TABLE teams DROP COLUMN IF EXISTS external_id;
DROP INDEX IF EXISTS users_org_external_username_idx, users_org_external_id_idx;
-- Accounts without a password can't be kept once it's required again.
DELETE FROM users WHERE password_hash IS NULL;
ALTER TABLE users
    DROP COLUMN IF EXISTS provisioned_by,
    DROP COLUMN IF EXISTS external_username,
    DROP COLUMN IF EXISTS external_id,
    DROP COLUMN IF EXISTS full_name,
    ALTER COLUMN password_hash SET NOT NULL;
DROP TABLE IF EXISTS org_domains, integration_tokens;
DELETE FROM integration_activity WHERE kind IN ('provision', 'sign_in');
ALTER TABLE integration_activity DROP CONSTRAINT integration_activity_kind_check,
    ADD CONSTRAINT integration_activity_kind_check CHECK (kind IN ('push_lead', 'test', 'setup'));
