# 0005. Squash migrations 001–013 into one baseline

- **Status:** Accepted
- **Date:** 2026-10-07

## Context

The schema had grown through thirteen migrations, several of which reshaped earlier ones: organisations were added after users (007), card slugs went from globally unique to unique per organisation (013), and data was backfilled along the way. Reading the schema meant replaying all thirteen in your head, and the first migration used `IF NOT EXISTS` throughout for databases set up by hand.

There were no real users, organisations or printed cards, so no deployment needed to be upgraded in place.

## Decision

- Migrations 001–013 were replaced by `001_baseline.up.sql`: the same schema written as clean `CREATE` statements with comments, and none of the backfills. It was checked against `pg_dump --schema-only` of a database built from the old chain; the only differences were intended clean-ups.
- New migrations continue from there (`002_jobs`, `003_integrations`, `004_identity` …).
- Existing databases are reset rather than migrated: the development database is dropped and recreated, and demo data comes from `fronko seed` (refused unless `FRONKO_ENV=development`). Production databases are reset by their operator; see [Resetting the database](../operations/resetting-the-database.md).

## Consequences

- The schema reads in one file.
- A database created with the old chain can't be upgraded: its `schema_migrations` version (13) doesn't exist in the new chain, and `migrate up` stops with an error. It has to be reset.
- `fronko seed` replaced the hand-made fixtures, so contributors get a working demo organisation with one command.
- From here on, merged migrations are never edited; schema changes are new numbered migrations.
