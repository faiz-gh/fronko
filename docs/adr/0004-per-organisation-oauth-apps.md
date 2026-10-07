# 0004. Each organisation registers its own OAuth apps

- **Status:** Accepted
- **Date:** 2026-10-07

## Context

OAuth providers (HubSpot first, Salesforce, Zoho and others later) need a registered app: a client ID, a client secret and a redirect URL. A SaaS product registers one app per provider and every customer authorises it. Fronko is self-hosted, so there is no single operator who could register them for everyone. The alternatives were:

1. **Server-wide apps from environment variables** (`HUBSPOT_CLIENT_ID` …), registered once by whoever runs the server.
2. **Per-organisation apps entered in the UI**, registered by each organisation's admin.

Server-wide apps mean one more set of secrets per provider in every deployment, a restart to add one, and one app's rate limits and permissions shared by every organisation on the server. Marketplace listing and review, which some providers require for public apps used by many accounts, would fall on the operator.

## Decision

Each organisation registers its own OAuth app with the provider and enters its client ID and secret on the connection in the Integrations page.

- OAuth providers declare required `client_id` (text) and `client_secret` (secret) fields; the registry refuses an OAuth manifest without them.
- The client secret and the tokens are sealed together in the connection's secrets with `SECRETS_KEY`, bound to the connection's id.
- The redirect URL is the same for every provider and organisation, `<PUBLIC_URL>/api/integrations/oauth/callback`, and the page shows it. The OAuth `state` is HMAC-signed and names the connection and user, so one callback serves all of them.
- Changing the client ID or secret drops the stored token.

## Consequences

- No per-provider server configuration: an operator only sets `PUBLIC_URL` and `SECRETS_KEY`.
- Each organisation's rate limits, scopes and audit trail at the provider are its own, and revoking one organisation's app affects nobody else.
- Setup is longer for admins: they create an app in the provider's developer portal before connecting. Each provider's guide walks through it with exact menu names, and the page shows the redirect URL and scopes to enter.
- Providers that only offer OAuth to listed marketplace apps can't be supported this way. None of the planned ones are like that today; if one is, a server-wide app could be added for that provider alone.
