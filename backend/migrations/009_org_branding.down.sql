ALTER TABLE organizations DROP CONSTRAINT IF EXISTS organizations_logo_policy_check;
ALTER TABLE organizations
    DROP COLUMN IF EXISTS signature,
    DROP COLUMN IF EXISTS logo_policy,
    DROP COLUMN IF EXISTS logo_file;
