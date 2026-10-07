# Integrations

The **Integrations** page (`/dashboard/integrations`) connects Fronko to other services. Integrations come in four categories:

| Category | What it does | Working today | Coming soon |
| -------- | ------------ | ------------- | ----------- |
| **Lead Sync** | Sends every lead your cards collect to a CRM or automation tool as it arrives | [Webhook](webhook-zapier.md), [Zapier, Make, n8n](webhook-zapier.md#zapier), [HubSpot](hubspot.md) | Salesforce, Zoho CRM, Microsoft Dynamics 365, Salesforce Account Engagement (Pardot), Pipedrive, monday.com, Marketo, Slack, Microsoft Outlook |
| **Calendar Booking** | Puts a "Book a meeting" button on cards | [Calendly, Chili Piper, Microsoft Bookings, HubSpot Meetings, Google Calendar, any other booking link](calendar.md) | |
| **Team Member Import** | Adds, updates and removes people (and teams) from your identity provider over SCIM | [Microsoft Entra ID](entra-scim.md) | Okta, Google Workspace |
| **SAML SSO** | Lets people sign in with your company's identity provider | [Okta](okta-saml.md), [Microsoft Entra ID](entra-saml.md), [any SAML 2.0 provider](saml.md) | |

"Coming soon" providers are listed so you can see what's planned; they can't be connected yet. If you need one, [open an integration request](https://github.com/faiz-gh/fronko/issues/new/choose).

## What the server needs

Some integrations depend on server settings. When one is missing, the integration shows as **Unavailable**, with the reason.

| Setting | Needed by | Why |
| ------- | --------- | --- |
| `SECRETS_KEY` | Anything with a secret: webhook signing secrets, HubSpot, SAML | Secrets are encrypted at rest with it (AES-256-GCM), bound to their connection |
| `PUBLIC_URL` | HubSpot (OAuth), SAML, SCIM | The addresses you enter in the other service (OAuth redirect URL, SAML ACS URL, SCIM tenant URL) are built from it |

`PUBLIC_URL` is where people reach the site, such as `https://cards.example.com`, with no trailing slash. It's also put in lead payloads as the card's link. See [Configuration](../../README.md#configuration).

In production, integrations never call private or local addresses (see [Outbound requests](#outbound-requests)). For local development, `FRONKO_ENV=development` lifts that, so a webhook can reach a receiver on your machine.

## Connections

A **connection** is one link to a provider: its settings, its encrypted secrets and its health. It belongs either to the organisation or to one person:

| Scope | Managed by | Example |
| ----- | ---------- | ------- |
| **Organisation** | The owner and admins | A HubSpot portal that gets every lead; the organisation's default booking page; SSO |
| **Personal** | The person who created it | A rep's own Zapier hook for the leads on their cards; their own Calendly page |

Members only see the integrations they can connect for themselves (lead sync webhooks and booking pages). Admins see everything, and can connect both kinds.

**How many.** Some categories allow one connection per owner, whichever provider it's to: one booking page per person (and one organisation default), one directory and one SSO connection per organisation. Lead sync allows several, except HubSpot, which allows one per organisation.

**Status.**

| Status | Meaning |
| ------ | ------- |
| **Setup incomplete** (`pending`) | A required setting is missing, an OAuth connection hasn't been authorised, or a SCIM connection has no token yet |
| **Active** | Working |
| **Needs attention** (`error`) | Three deliveries in a row failed for good (after their retries). The connection stops getting new leads until someone saves its settings or switches it off and on again |

A connection can also be **paused** with its switch. Paused and broken connections don't receive leads, and leads that arrive meanwhile aren't sent later.

**Secrets** (API keys, signing secrets, OAuth tokens, SAML keys) are encrypted with `SECRETS_KEY` and never sent back to the browser. The page only shows whether each one is set. Leave a secret field blank to keep it.

**Test.** Every lead sync connection has **Send test lead**, which delivers a made-up lead (Test Lead, `test.lead@example.com`) through the real path. Booking pages and SAML connections have **Test**, which opens the booking page or fetches the identity provider's metadata. Tests are rate limited per address (5 at once, then one every 10 seconds).

## Lead delivery

When a visitor sends a card's contact form:

1. The lead is saved. In the same database transaction, Fronko queues one delivery job for each lead sync connection that should get it: every active organisation connection, plus the card holder's own personal ones.
2. A background worker picks the job up within moments and calls the provider.
3. A failure that might be temporary (a timeout, a 5xx, a 429) is retried with growing waits: about 30 seconds, 1, 2, 4 minutes and so on, up to 10 attempts over about four hours. A failure that retrying can't fix (bad credentials, a rejected payload, a redirect) stops at once.
4. Each attempt is written to the connection's **activity log**.

Deliveries are **at least once**: in rare cases (a server restart mid-delivery) the same lead can be sent twice. The webhook sends the same `X-Fronko-Delivery` id each time so receivers can drop duplicates, and HubSpot upserts by email.

Background work needs a worker: each backend runs `JOB_WORKERS` of them (default 2). With `JOB_WORKERS=0` an instance only queues jobs, which is useful when a separate instance runs them.

## Activity log

Each connection's page has an activity log: leads sent (with the attempt number), tests, settings changes, authorisations, people provisioned over SCIM and SSO sign-ins. Expand an entry for details such as the HTTP status, the response excerpt or the reason a sign-in was refused. Entries are kept for 90 days.

## Outbound requests

Every request an integration makes to an address someone typed in (a webhook URL, a booking page, SAML metadata) goes through an SSRF guard:

- Only `https` (except in development).
- The address is checked **after** DNS resolution, at connect time, so a hostname can't point at a private, loopback, link-local, CGNAT or cloud metadata address, even by changing its DNS between the check and the request.
- Redirects aren't followed, and proxy environment variables are ignored.
- Requests time out after 20 seconds.

## Calendar booking on cards

A card's "Book a meeting" button comes from Integrations, not from the card:

1. If the card's holder has a personal booking page connected, the card shows it.
2. Otherwise, if the organisation has a default booking page, the card shows that.
3. Otherwise there's no button.

Cards held by the organisation (unassigned) always use the default. See [Calendar booking](calendar.md).

## For developers

- [Writing an integration provider](writing-a-provider.md): adding a new provider.
- [API reference: Integrations](../../backend/API.md#integrations-): the endpoints the page uses.
- [ADR 0003](../adr/0003-integration-registry.md) and [ADR 0004](../adr/0004-per-organisation-oauth-apps.md): why it's built this way.
