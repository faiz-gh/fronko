DELETE FROM email_codes WHERE purpose = 'change_email';

ALTER TABLE email_codes DROP CONSTRAINT IF EXISTS email_codes_purpose_check;
ALTER TABLE email_codes ADD CONSTRAINT email_codes_purpose_check
    CHECK (purpose IN ('verify_email', 'reset_password'));

ALTER TABLE email_codes DROP COLUMN IF EXISTS email;
