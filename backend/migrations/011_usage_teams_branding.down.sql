ALTER TABLE platform_usage_snapshots
    DROP COLUMN IF EXISTS orgs_with_logo,
    DROP COLUMN IF EXISTS orgs_with_teams,
    DROP COLUMN IF EXISTS team_count;

ALTER TABLE org_usage_snapshots
    DROP COLUMN IF EXISTS team_count;
