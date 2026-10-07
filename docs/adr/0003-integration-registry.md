# 0003. Describe integrations with manifests in an explicit registry

- **Status:** Accepted
- **Date:** 2026-10-07

## Context

The Integrations tab launched with four categories (Lead Sync, Calendar Booking, Team Member Import, SAML SSO) and a long list of providers wanted after that: Salesforce, Zoho, Dynamics, Pipedrive, Okta SCIM and more. Each one needs a catalog entry, a settings form, validation, secret storage, a way to test it, and somewhere to log what it did. If each provider built those itself, the tenth would cost as much as the first, and they would drift apart.

## Decision

- **Manifest.** Every provider describes itself with a `Manifest`: id, name, category, description, allowed scopes (organisation or personal), auth kind, availability (`available`, `beta`, `coming_soon`), whether several connections are allowed, a list of typed settings fields, setup steps, a docs link and server requirements (`public_url`, `secrets_key`). The manifest is the single source of truth: the catalog endpoint serves it, the frontend renders the settings form from it, and the server validates settings against it.
- **Small required interface, optional capabilities.** `Provider` is only `Manifest()` and `Validate()`. What else a provider can do is an optional interface the core finds by type assertion: `LeadPusher`, `Tester`, `OAuthProvider`, `BookingLinker`, `Initializer`, `Describer`.
- **Explicit registry.** `providers/all.go` calls `registry.Register(...)` for each provider, in catalog order. There is no `init()` self-registration. `Register` checks each manifest and panics at start-up on a mistake.
- **Coming soon.** Planned providers are registered as manifest-only `Placeholder`s in `comingsoon.go`, so the catalog shows what's on the way. Building one replaces its placeholder.
- **The core owns the plumbing.** Connections, sealed secrets, OAuth, the outbound HTTP client (SSRF-guarded, token-refreshing), lead dispatch through the job queue, retries, failure tracking and the activity log are written once in `integrations/`.

## Consequences

- A new provider is one folder and one line. A lead sync provider is typically a manifest, `Validate` and `PushLead`.
- The frontend needs no change for a new provider beyond an optional logo.
- Reading `all.go` tells you everything the server offers; there's no hidden registration order.
- Settings that don't fit the field types (`text`, `url`, `secret`, `select`, `textarea`, `bool`) need a new type in both the core and the form. So far the six have covered every provider.
- Providers that must serve HTTP routes (the SAML ACS, the SCIM server) are packages beside the core (`integrations/sso`, `integrations/directory`) and are also wired as modules. That's the exception, not the pattern.
