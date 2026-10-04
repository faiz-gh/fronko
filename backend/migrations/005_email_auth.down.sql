DROP TABLE IF EXISTS email_codes;
DROP INDEX IF EXISTS users_email_lower_idx;
ALTER TABLE users
    DROP COLUMN IF EXISTS session_version,
    DROP COLUMN IF EXISTS email_verified_at,
    DROP COLUMN IF EXISTS email;
