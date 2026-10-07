-- What visitors do on public cards: views (by NFC tag, QR code or plain link),
-- clicks, scroll depth, brochure opens, contact saves and the contact form.
-- Visitors are never identified: there are no cookies and no IP addresses,
-- only a hash salted with a random value that is thrown away after a day.
CREATE TABLE IF NOT EXISTS card_events (
    event_id BIGSERIAL PRIMARY KEY,
    org_id BIGINT NOT NULL REFERENCES organizations(org_id) ON DELETE CASCADE,
    profile_id BIGINT NOT NULL REFERENCES profiles(profile_id) ON DELETE CASCADE,
    -- Who held the card when it happened (NULL: the organisation), like leads,
    -- so history stays with the person even after the card is reassigned.
    assigned_user_id BIGINT REFERENCES users(user_id) ON DELETE SET NULL,
    -- One page load of the card; kept only in the visitor's tab memory.
    session_id UUID,
    visitor_hash BYTEA NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('view', 'click', 'scroll', 'doc_open', 'gallery_open',
        'vcard', 'form_open', 'form_submit', 'share', 'leave')),
    source TEXT NOT NULL DEFAULT 'link' CHECK (source IN ('nfc', 'qr', 'link')),
    -- What was used: a link id, a document's file id, or a quick action (email, call, website, booking).
    target TEXT NOT NULL DEFAULT '',
    label TEXT NOT NULL DEFAULT '',
    -- Scroll depth in percent for 'scroll', time on the card in ms for 'leave'.
    value INT NOT NULL DEFAULT 0,
    device TEXT NOT NULL DEFAULT 'desktop' CHECK (device IN ('mobile', 'tablet', 'desktop')),
    referrer_host TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS card_events_profile_idx ON card_events (profile_id, created_at);
CREATE INDEX IF NOT EXISTS card_events_org_idx ON card_events (org_id, created_at);
CREATE INDEX IF NOT EXISTS card_events_user_idx ON card_events (org_id, assigned_user_id, created_at);

-- One random salt per day for visitor hashes; old ones are deleted so a hash
-- can't be traced back to an address later.
CREATE TABLE IF NOT EXISTS analytics_salts (
    day DATE PRIMARY KEY,
    salt BYTEA NOT NULL
);

-- How the visitor reached the card when they sent the form. NULL for older leads.
ALTER TABLE leads
    ADD COLUMN IF NOT EXISTS source TEXT CHECK (source IN ('nfc', 'qr', 'link'));
