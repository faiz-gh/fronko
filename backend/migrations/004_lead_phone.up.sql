-- Optional phone number on leads, stored as two digit-only parts so it can be
-- formatted per country: a dial code ("+91") and the national number.
ALTER TABLE leads
    ADD COLUMN IF NOT EXISTS phone_country_code TEXT,
    ADD COLUMN IF NOT EXISTS phone_number TEXT;

ALTER TABLE leads DROP CONSTRAINT IF EXISTS leads_phone_format;
ALTER TABLE leads ADD CONSTRAINT leads_phone_format CHECK (
    (phone_country_code IS NULL AND phone_number IS NULL)
    OR (
        phone_country_code ~ '^\+[1-9][0-9]{0,2}$'
        AND phone_number ~ '^[0-9]{4,14}$'
        AND length(phone_country_code) - 1 + length(phone_number) <= 15
    )
);
