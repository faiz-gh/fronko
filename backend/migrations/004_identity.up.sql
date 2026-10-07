-- Identity integrations (backend/internal/integrations/directory and /sso):
-- tokens that let an identity provider call Fronko's SCIM API, the email
-- domains an organisation has proved it owns, and what an identity provider
-- knows about each person and team.

-- Identity providers log what they do too: people provisioned over SCIM and
-- sign-ins through single sign-on.
ALTER TABLE integration_activity DROP CONSTRAINT integration_activity_kind_check,
    ADD CONSTRAINT integration_activity_kind_check
        CHECK (kind IN ('push_lead', 'test', 'setup', 'provision', 'sign_in'));

-- Bearer tokens an outside service presents to Fronko (SCIM). Only a
-- SHA-256 of the token is stored; it's shown once, when it's generated.
CREATE TABLE integration_tokens (
    token_id      BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    connection_id BIGINT NOT NULL REFERENCES integration_connections (connection_id) ON DELETE CASCADE,
    token_hash    BYTEA NOT NULL UNIQUE,
    -- The token's last characters, so people can tell tokens apart.
    hint          TEXT NOT NULL,
    created_by    BIGINT REFERENCES users (user_id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at  TIMESTAMPTZ
);
CREATE INDEX integration_tokens_connection_idx ON integration_tokens (connection_id);

-- Email domains of an organisation. Once verified (a DNS TXT record), people
-- signing in with an address on it are sent to the organisation's single
-- sign-on, and single sign-on marks their address as verified.
CREATE TABLE org_domains (
    domain_id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    org_id             BIGINT NOT NULL REFERENCES organizations (org_id) ON DELETE CASCADE,
    domain             TEXT NOT NULL CHECK (domain = lower(domain) AND length(domain) BETWEEN 3 AND 253),
    -- The value of the TXT record that proves ownership.
    verification_token TEXT NOT NULL,
    verified_at        TIMESTAMPTZ,
    created_by         BIGINT REFERENCES users (user_id) ON DELETE SET NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX org_domains_org_domain_idx ON org_domains (org_id, domain);
-- A domain belongs to one organisation once it's verified.
CREATE UNIQUE INDEX org_domains_verified_idx ON org_domains (domain) WHERE verified_at IS NOT NULL;

-- People an identity provider manages. Accounts that only sign in with
-- single sign-on have no password.
ALTER TABLE users
    ALTER COLUMN password_hash DROP NOT NULL,
    ADD COLUMN full_name TEXT,
    -- The identity provider's id for the person (SCIM externalId).
    ADD COLUMN external_id TEXT,
    -- The identity provider's user name (SCIM userName), often their email.
    ADD COLUMN external_username TEXT,
    -- What created the account: NULL for people, or 'scim' or 'sso'.
    ADD COLUMN provisioned_by TEXT CHECK (provisioned_by IN ('scim', 'sso'));
CREATE UNIQUE INDEX users_org_external_id_idx ON users (org_id, external_id) WHERE external_id IS NOT NULL;
CREATE UNIQUE INDEX users_org_external_username_idx ON users (org_id, lower(external_username))
    WHERE external_username IS NOT NULL;

-- Teams an identity provider manages (SCIM groups).
ALTER TABLE teams ADD COLUMN external_id TEXT;
CREATE UNIQUE INDEX teams_org_external_id_idx ON teams (org_id, external_id) WHERE external_id IS NOT NULL;
