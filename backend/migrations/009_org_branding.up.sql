-- Organisation branding: a logo shown on cards and email signatures, whether
-- employees may hide it, and the organisation's email signature settings.
ALTER TABLE organizations
    -- Public id of an org-area image in the files library.
    ADD COLUMN IF NOT EXISTS logo_file TEXT,
    -- 'required': the logo is on every card and signature; 'optional': each card chooses.
    ADD COLUMN IF NOT EXISTS logo_policy TEXT NOT NULL DEFAULT 'optional',
    -- {locked_template, brand_color, disclaimer, banner_file, banner_url}
    ADD COLUMN IF NOT EXISTS signature JSONB NOT NULL DEFAULT '{}';

ALTER TABLE organizations DROP CONSTRAINT IF EXISTS organizations_logo_policy_check;
ALTER TABLE organizations ADD CONSTRAINT organizations_logo_policy_check
    CHECK (logo_policy IN ('required', 'optional'));
