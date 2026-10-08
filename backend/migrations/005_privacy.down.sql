ALTER TABLE organizations DROP COLUMN IF EXISTS lead_retention_days, DROP COLUMN IF EXISTS privacy_url;
DROP INDEX IF EXISTS leads_created_at_idx;
UPDATE feedback SET sender_email = '' WHERE sender_email IS NULL;
UPDATE feedback SET org_name = '' WHERE org_name IS NULL;
ALTER TABLE feedback ALTER COLUMN sender_email SET NOT NULL, ALTER COLUMN org_name SET NOT NULL;
DELETE FROM email_codes WHERE purpose = 'delete_account';
ALTER TABLE email_codes DROP CONSTRAINT email_codes_purpose_check,
    ADD CONSTRAINT email_codes_purpose_check
        CHECK (purpose IN ('verify_email', 'reset_password', 'change_email'));
