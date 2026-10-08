-- Privacy: self-service account deletion, lead retention and the
-- organisation's privacy notice (docs/compliance/gdpr-audit.md).

-- People who sign in only with single sign-on have no password, so deleting
-- their account is confirmed with an emailed code instead.
ALTER TABLE email_codes DROP CONSTRAINT email_codes_purpose_check,
    ADD CONSTRAINT email_codes_purpose_check
        CHECK (purpose IN ('verify_email', 'reset_password', 'change_email', 'delete_account'));

-- Feedback outlives the account that sent it, but not its email address
-- (or, once the organisation is deleted, its name).
ALTER TABLE feedback
    ALTER COLUMN sender_email DROP NOT NULL,
    ALTER COLUMN org_name DROP NOT NULL;

-- Lead retention deletes by age.
CREATE INDEX leads_created_at_idx ON leads (created_at);

ALTER TABLE organizations
    -- The organisation's privacy notice, linked from its cards' contact forms.
    ADD COLUMN privacy_url TEXT,
    -- Leads older than this are deleted automatically; NULL keeps them.
    ADD COLUMN lead_retention_days INTEGER
        CHECK (lead_retention_days IS NULL OR lead_retention_days BETWEEN 30 AND 3650);
