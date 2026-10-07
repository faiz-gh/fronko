-- Card links include the organisation (/p/{handle}/{slug}), so a card's slug
-- only has to be unique inside its own organisation. The handle is short and
-- readable, and ends up printed on QR codes and written onto NFC chips.
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS handle TEXT;

-- Existing organisations get a handle made from their name.
UPDATE organizations SET handle = LEFT(TRIM(BOTH '-' FROM REGEXP_REPLACE(LOWER(name), '[^a-z0-9]+', '-', 'g')), 32)
WHERE handle IS NULL;
UPDATE organizations SET handle = TRIM(BOTH '-' FROM handle) WHERE handle IS NOT NULL;
UPDATE organizations SET handle = 'org-' || org_id WHERE LENGTH(handle) < 3;
-- Two organisations with the same name: the newer ones get their id added.
UPDATE organizations o SET handle = LEFT(o.handle, 25) || '-' || o.org_id
WHERE EXISTS (SELECT 1 FROM organizations e WHERE LOWER(e.handle) = LOWER(o.handle) AND e.org_id < o.org_id);

ALTER TABLE organizations ALTER COLUMN handle SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS organizations_handle_lower_idx ON organizations (LOWER(handle));

ALTER TABLE profiles DROP CONSTRAINT IF EXISTS profiles_slug_key;
DROP INDEX IF EXISTS profiles_slug_lower_idx;
CREATE UNIQUE INDEX IF NOT EXISTS profiles_org_slug_lower_idx ON profiles (org_id, LOWER(slug));
