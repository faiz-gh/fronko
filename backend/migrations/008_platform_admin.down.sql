DROP TABLE IF EXISTS admin_audit_log;
DROP TABLE IF EXISTS platform_usage_snapshots;
DROP TABLE IF EXISTS org_usage_snapshots;
DROP TABLE IF EXISTS feedback_replies;
DROP TABLE IF EXISTS feedback;
ALTER TABLE organizations
    DROP COLUMN IF EXISTS suspended_reason,
    DROP COLUMN IF EXISTS suspended_at;
DROP TABLE IF EXISTS platform_admins;
