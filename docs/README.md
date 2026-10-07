# Fronko documentation

Guides for people who run Fronko, set up its integrations, or work on its code. The READMEs at the top of each app cover setup and reference material; these pages explain how the pieces fit together.

## Running Fronko

| Document | Contents |
| -------- | -------- |
| [Root README](../README.md) | What Fronko does, the quick start with Docker Compose, and configuration |
| [Backend README](../backend/README.md) | Every environment variable, the database, sessions, security measures and testing |
| [Resetting the database](operations/resetting-the-database.md) | Moving an existing deployment onto the squashed baseline migration |

## Integrations

| Document | Contents |
| -------- | -------- |
| [Overview](integrations/overview.md) | Categories, connections, what each one needs from the server, lead delivery and the activity log |
| [Webhook, Zapier, Make and n8n](integrations/webhook-zapier.md) | The lead payload, signature checking, retries, and setting up each automation tool |
| [HubSpot](integrations/hubspot.md) | Registering your own HubSpot app and how leads map to contacts |
| [Calendar booking](integrations/calendar.md) | Booking pages on cards: personal pages, the organisation default, prefilled details |
| [Okta SAML](integrations/okta-saml.md) | Single sign-on with Okta |
| [Microsoft Entra ID SAML](integrations/entra-saml.md) | Single sign-on with Microsoft Entra ID (Azure AD) |
| [Other SAML providers](integrations/saml.md) | Single sign-on with any SAML 2.0 identity provider |
| [Microsoft Entra ID SCIM](integrations/entra-scim.md) | Adding, updating and removing people (and teams) from Entra ID |

## Working on the code

| Document | Contents |
| -------- | -------- |
| [Architecture](architecture.md) | How the backend and frontend are put together: modules, the request pipeline, events and jobs |
| [Backend modules](backend-modules.md) | How to add a backend feature module, step by step |
| [Frontend structure](frontend-structure.md) | Feature folders, the core, shared components and the registries |
| [Writing an integration provider](integrations/writing-a-provider.md) | Adding a new provider, using the webhook provider as the example |
| [API reference](../backend/API.md) | Every endpoint, with request and response shapes |
| [Contributing](../CONTRIBUTING.md) | Development setup, coding guidelines, tests and pull requests |

## Decisions

Architecture decision records explain why things are the way they are. Each one is short and doesn't change once accepted; a later record supersedes it instead.

| ADR | Decision |
| --- | -------- |
| [0001](adr/0001-feature-modules.md) | Organise both apps by feature, not by layer |
| [0002](adr/0002-postgres-job-queue.md) | Run background jobs from a Postgres table, not a separate queue |
| [0003](adr/0003-integration-registry.md) | Describe every integration with a manifest in an explicit registry |
| [0004](adr/0004-per-organisation-oauth-apps.md) | Each organisation registers its own OAuth apps |
| [0005](adr/0005-squashed-baseline-migration.md) | Squash migrations 001–013 into one baseline |
