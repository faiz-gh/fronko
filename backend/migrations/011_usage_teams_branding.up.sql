-- Usage snapshots learn about teams (010) and branding (009), so the admin
-- panel can chart them. Days recorded before this read as zero.
ALTER TABLE org_usage_snapshots
    ADD COLUMN IF NOT EXISTS team_count INT NOT NULL DEFAULT 0;

ALTER TABLE platform_usage_snapshots
    ADD COLUMN IF NOT EXISTS team_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS orgs_with_teams INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS orgs_with_logo INT NOT NULL DEFAULT 0;
