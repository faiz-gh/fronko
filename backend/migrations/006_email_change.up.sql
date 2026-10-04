-- Changing a verified email: the code goes to the new address, which is kept
-- on the code row until it's confirmed. The current email stays in force until then.
ALTER TABLE email_codes ADD COLUMN IF NOT EXISTS email TEXT;

ALTER TABLE email_codes DROP CONSTRAINT IF EXISTS email_codes_purpose_check;
ALTER TABLE email_codes ADD CONSTRAINT email_codes_purpose_check
    CHECK (purpose IN ('verify_email', 'reset_password', 'change_email'));
