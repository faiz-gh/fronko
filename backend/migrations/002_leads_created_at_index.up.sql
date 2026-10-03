-- Leads are listed newest first, per profile and across a user's profiles.
CREATE INDEX IF NOT EXISTS leads_profile_id_created_at_idx ON leads (profile_id, created_at DESC);
