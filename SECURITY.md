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

- vulnerabilities in third-party services you connect to Fronko, such as your S3 provider, SMTP provider or PostgreSQL host
- issues that require a deployment to ignore the guidance below, such as setting `STORAGE_ALLOW_PRIVATE_ENDPOINTS=true` or serving over plain HTTP in production
- behaviour already documented under [Known limitations](backend/README.md#known-limitations)
- missing hardening headers or best practices without a demonstrated impact
- denial-of-service through volumetric traffic, and findings from automated scanners without a working exploit

When testing, use your own self-hosted instance. Do not test against deployments you don't own, and do not access or modify other people's data.

## Hardening a self-hosted deployment

Fronko's built-in protections are described in the backend's [security measures](backend/README.md#security-measures). When running it yourself:

- **Serve it over HTTPS** and keep `COOKIE_SECURE` at its default of `true`.
- **Use strong, unique secrets.** Generate `JWT_SECRET` and `SECRETS_KEY` with `openssl rand -base64 32`, keep them out of version control, and back up `SECRETS_KEY` securely.
- **Keep the backend behind the frontend's proxy.** The Compose files don't bind any host ports; route public traffic to `frontend:3000`. `TRUST_PROXY` must only be `true` when a proxy always sets `X-Real-IP`.
- **Never enable `STORAGE_ALLOW_PRIVATE_ENDPOINTS` in production.** It is for local development only and would let users make the server connect to internal hosts.
- **Keep storage buckets private.** Fronko serves files through short-lived signed links and doesn't need public bucket access.
- **Use TLS for PostgreSQL** (`sslmode=require` or stricter) when the database is reached over a network.
- **Protect platform admin accounts.** They see every organisation's owner email and usage, read feedback, and can suspend organisations. Create only the admins you need (`./fronko admin create`), give each a long unique password, and review the panel's audit log. Admin sign-in has no second factor yet. Remove an admin with SQL (`DELETE FROM platform_admins WHERE email = …`) when they no longer need access.
- **Configure SMTP.** Without it, verification and password-reset codes are written to the backend log.
- **Keep up to date** with the latest `master`, and rebuild the images regularly to pick up base-image updates.
