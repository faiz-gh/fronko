# Security Policy

Thank you for helping keep Fronko and the people who use it safe.

## Supported versions

Fronko does not publish versioned releases yet. Security fixes are made on the `master` branch, so please make sure you can reproduce an issue against the latest `master` before reporting it.

## Reporting a vulnerability

**Please do not report security vulnerabilities through public GitHub issues, discussions or pull requests.**

Report them privately instead:

1. **GitHub (preferred):** use [private vulnerability reporting](https://github.com/faiz-gh/fronko/security/advisories/new) from the repository's **Security** tab.
2. **Email:** if you can't use GitHub, write to [faizghanchi1928@gmail.com](mailto:faizghanchi1928@gmail.com) with "Fronko security" in the subject.

Please include as much of the following as you can:

- the type of issue (for example authentication bypass, SSRF, XSS, data exposure)
- the affected component, endpoint or file, and the commit you tested against
- step-by-step instructions to reproduce it, with any proof-of-concept code
- the impact, and how an attacker could exploit it
- any configuration it depends on (environment variables, reverse proxy, storage provider)

## What to expect

- Your report will be acknowledged within a few days.
- You'll hear whether the issue is accepted, get updates while a fix is in progress, and agree a disclosure date.
- Once a fix is available, a GitHub security advisory is published, crediting you unless you'd rather stay anonymous.

Please allow reasonable time for a fix to be released before disclosing the issue publicly.

## Scope

**In scope:** the code in this repository, which covers the Go backend, the Svelte frontend, and the Docker images and Compose files under `deploy/`.

**Out of scope:**

- vulnerabilities in third-party services you connect to Fronko, such as your S3 provider, SMTP provider, PostgreSQL host, CRM or identity provider
- issues that require a deployment to ignore the guidance below, such as setting `STORAGE_ALLOW_PRIVATE_ENDPOINTS=true` or `FRONKO_ENV=development`, or serving over plain HTTP in production
- behaviour already documented under [Known limitations](backend/README.md#known-limitations)
- missing hardening headers or best practices without a demonstrated impact
- denial-of-service through volumetric traffic, and findings from automated scanners without a working exploit

When testing, use your own self-hosted instance. Do not test against deployments you don't own, and do not access or modify other people's data.

## Hardening a self-hosted deployment

Fronko's built-in protections are described in the backend's [security measures](backend/README.md#security-measures). When running it yourself:

- **Serve it over HTTPS** and keep `COOKIE_SECURE` at its default of `true`.
- **Use strong, unique secrets.** Generate `JWT_SECRET` and `SECRETS_KEY` with `openssl rand -base64 32`, keep them out of version control, and back up `SECRETS_KEY` securely.
- **Keep the backend behind the frontend's proxy.** The Compose files don't bind any host ports; route public traffic to `frontend:3000`. `TRUST_PROXY` must only be `true` when a proxy always sets `X-Real-IP`.
- **Never enable `STORAGE_ALLOW_PRIVATE_ENDPOINTS` or `FRONKO_ENV=development` in production.** Both are for local development only. The first lets storage endpoints reach internal hosts; the second lets integrations (webhooks, booking links, SAML metadata) call private and local addresses, and enables `fronko seed`, which creates accounts with published passwords. The production Compose file pins `FRONKO_ENV=production`.
- **Keep storage buckets private.** Fronko serves files through short-lived signed links and doesn't need public bucket access.
- **Use TLS for PostgreSQL** (`sslmode=require` or stricter) when the database is reached over a network.
- **Protect platform admin accounts.** They see every organisation's owner email and usage, read feedback, and can suspend organisations. Create only the admins you need (`./fronko admin create`), give each a long unique password, and review the panel's audit log. Admin sign-in has no second factor yet. Remove an admin with SQL (`DELETE FROM platform_admins WHERE email = …`) when they no longer need access.
- **Card analytics stay anonymous by design.** Fronko sets no cookies on public cards and stores no IP addresses: each visitor is a SHA-256 hash with a random salt that is replaced and deleted every day, and analytics endpoints return aggregates only. Events are deleted after `ANALYTICS_RETENTION_DAYS` (default 395). Set it lower if your privacy policy promises a shorter retention, and keep `TRUST_PROXY` correct so the hash (and rate limits) use the real client address.
- **Configure SMTP.** Without it, verification and password-reset codes are written to the backend log.
- **Set `FRONTEND_URL` (and `BACKEND_URL`, when the API has its own domain) to the HTTPS addresses people use.** SAML, SCIM and OAuth URLs are built from them, so they must be the real public origin.

## Integrations

What Fronko does to keep integrations safe, and what's up to you:

- **Secrets at rest.** API keys, webhook signing secrets, OAuth client secrets and tokens, and each SAML connection's private key are sealed with AES-256-GCM using `SECRETS_KEY`, bound to their connection's id. They're never returned by the API; the dashboard only shows whether each is set. Back up `SECRETS_KEY`: losing it means re-entering every secret and re-creating SAML connections.
- **Outbound requests (SSRF).** Every request to an address someone typed in goes through one client that allows only `https`, checks each address it dials **after** DNS resolution (so private, loopback, link-local, CGNAT and cloud metadata addresses are refused even under DNS rebinding), ignores proxy variables, doesn't follow redirects, and times out after 20 seconds.
- **Webhook signing.** With a signing secret set, each delivery carries `X-Fronko-Signature: t=<unix time>,v1=<HMAC-SHA256 of "t.body">`. Receivers should verify it in constant time and reject timestamps more than a few minutes old. See [the webhook guide](docs/integrations/webhook-zapier.md#checking-the-signature). Webhook URLs without a signing secret (such as Zapier's catch hooks) are bearer secrets: anyone with the URL can post to it.
- **OAuth.** Each organisation uses its own OAuth app. The `state` parameter is HMAC-signed, expires after 10 minutes, and is bound to the user, the connection and a nonce in a short-lived cookie; PKCE is used where the provider supports it.
- **SCIM tokens.** Tokens (`fronko_scim_…`) are shown once and stored only as a SHA-256 hash. Generating a new one revokes the old one at once, and pausing the connection stops it working. Treat it like an admin password: it can create, suspend and delete people. The organisation's owner can't be deactivated or deleted over SCIM.
- **SAML.** Responses must be signed and are checked for audience, destination, timing and, for SP-initiated sign-in, that they answer the request this browser started (a signed, short-lived cookie). IdP-initiated sign-in is off by default. XML that doesn't survive a round trip is refused. Each connection has its own key pair.
- **Requiring SSO.** "Require single sign-on" blocks password sign-in and password reset for everyone except the owner, who keeps a break-glass password. Keep the owner's password strong and private.
- **Email domains.** A domain must be proven with a DNS TXT record before it routes sign-ins or lets SSO and SCIM mark addresses as verified, and only one organisation can verify a given domain.
- **Activity log.** Lead deliveries, tests, settings changes, SCIM changes and SSO sign-ins (including refusals and their reasons) are logged per connection for 90 days.
- **Keep up to date** with the latest `master`, and rebuild the images regularly to pick up base-image updates.
