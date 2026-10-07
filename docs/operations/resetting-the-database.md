# Resetting the database

Fronko's migrations were squashed into a single baseline ([ADR 0005](../adr/0005-squashed-baseline-migration.md)). A database created before that, with migrations `001`–`013`, can't be upgraded: on start-up the backend's `migrate up` fails with `no migration found for version 13`, and the container exits.

Such a database has to be **reset**: emptied, then rebuilt from the new migrations. **This deletes every organisation, account, card, lead, file record and analytics event in it.** Only do it for a deployment whose data you don't need, which was the case for every deployment when the squash happened.

You can tell whether you need this from the backend's log, or by asking the database:

```bash
psql "$DATABASE_URL" -c 'SELECT version, dirty FROM schema_migrations;'
```

A `version` of 5 or more, from before the squash (anything up to 13), means the old chain. After the reset it reads `4` (or higher, as new migrations arrive).

## Production

Run these from the machine that runs `deploy/docker-compose.yml`, with `DATABASE_URL` from `deploy/.env` in your shell.

### 1. Back up

Even when you expect to throw the data away:

```bash
pg_dump --format=custom --file="fronko-before-reset-$(date +%F).dump" "$DATABASE_URL"
```

### 2. Stop the backend

```bash
cd deploy
docker compose stop backend
```

### 3. Empty the database

Drop and recreate the `public` schema, which holds every Fronko table:

```bash
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 <<'SQL'
DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
SQL
```

On a managed database where your user doesn't own the `public` schema, the drop fails with "must be owner of schema public". Then either run it as the database owner, or drop and recreate the whole database from your provider's console with the same name and owner.

If anything else lives in that database (it shouldn't), it goes too. Fronko should have a database of its own.

### 4. Update the environment

Compare `deploy/.env` with [`deploy/.env.example`](../../deploy/.env.example). Since the squash, these were added:

| Variable | What to set |
| -------- | ----------- |
| `FRONTEND_URL` | The site's address, such as `https://cards.example.com`, even if the API is on the same domain. HubSpot, SAML and SCIM need it. (`PUBLIC_URL` overrides it, if the site's address isn't the first `FRONTEND_URL`.) |
| `BACKEND_URL` | Only if the API has its own domain (such as `https://api.cards.example.com`). The backend now also builds OAuth, SAML and SCIM addresses on it; `FRONTEND_URL` stays the site's address |
| `JOB_WORKERS` | Leave at `2` unless you know you want otherwise |
| `SECRETS_KEY` | Already optional for storage; integrations with secrets need it too. If you set it for the first time, generate it with `openssl rand -base64 32` and back it up |

`FRONKO_ENV` is fixed to `production` in `docker-compose.yml`; don't change it.

### 5. Rebuild and start

```bash
docker compose up -d --build
docker compose logs -f backend
```

The entrypoint runs the migrations (`001_baseline` to the latest), then starts the server. Wait for `Server listening on :8080`, then check the site loads.

### 6. Set up again

- **Platform admin.** Admin accounts were in the old database too. Create yours again:

  ```bash
  docker compose exec backend ./fronko admin create --email you@example.com
  ```

- **Organisations.** Every account is gone. Register again at `/login?mode=register`; the first account becomes the owner of a new organisation. Reconnect storage under **Settings → Storage**.
- **Old files.** Objects uploaded before the reset are still in your S3 bucket under `fronko/<org_id>/…`, but nothing refers to them any more. Delete the `fronko/` prefix in your provider's console once you're sure you don't need them. New uploads may reuse the same organisation ids, so delete the old objects **before** anyone uploads again, or leave them.
- **NFC cards and QR codes** printed before the reset point at card links that no longer exist until cards with the same organisation handle and slug are created again.

**Don't run `fronko seed` in production.** It creates demo accounts with published passwords, and it refuses to run unless `FRONKO_ENV=development`.

## Development

For the local Docker stack, reset and reseed in one go:

```bash
docker exec -e FRONKO_ENV=development backend-local ./fronko seed --reset
```

`--reset` deletes every row (the schema stays), then creates the demo data listed in [Demo data](../../CONTRIBUTING.md#demo-data), including the platform admin `admin@fronko.local`.

If the local database is still on the old migration chain, drop and recreate it first (step 3 above, against your local `DATABASE_URL`), then restart the backend so it migrates, then seed. Without Docker, `make seed ARGS=--reset` in `backend/` does the same against `DATABASE_URL`.
