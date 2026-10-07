-- Fails if two organisations now use the same slug; rename one first.
DROP INDEX IF EXISTS profiles_org_slug_lower_idx;
CREATE UNIQUE INDEX IF NOT EXISTS profiles_slug_lower_idx ON profiles (LOWER(slug));
ALTER TABLE profiles ADD CONSTRAINT profiles_slug_key UNIQUE (slug);

DROP INDEX IF EXISTS organizations_handle_lower_idx;
ALTER TABLE organizations DROP COLUMN IF EXISTS handle;
