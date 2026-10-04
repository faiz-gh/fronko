ALTER TABLE leads DROP CONSTRAINT IF EXISTS leads_phone_format;
ALTER TABLE leads
    DROP COLUMN IF EXISTS phone_number,
    DROP COLUMN IF EXISTS phone_country_code;
