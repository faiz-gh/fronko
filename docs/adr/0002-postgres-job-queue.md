# 0002. Run background jobs from a Postgres table

- **Status:** Accepted
- **Date:** 2026-10-07

## Context

Lead sync has to call outside services after a lead is saved. Those calls are slow, can fail, and must be retried, so they can't run inside the visitor's request. Periodic work (the hourly usage snapshot, analytics retention) ran on a hand-written ticker that wasn't safe with more than one backend instance.

Fronko is self-hosted. Every extra service (Redis, RabbitMQ, a hosted queue) is something each operator has to run, secure and back up. PostgreSQL is already required.

A lead and the jobs that sync it must not disagree: a job for a lead that rolled back would fail, and a lead whose job was lost would never sync.

## Decision

- **Queue.** Jobs are rows in a `jobs` table (`kind`, `payload`, `org_id`, `dedupe_key`, `run_at`, `attempts`, `max_attempts`, `status`, `last_error`, `locked_at`). Workers claim due jobs with `SELECT … FOR UPDATE SKIP LOCKED`, so any number of workers in any number of instances share it without running a job twice at once. Enqueuing sends `NOTIFY fronko_jobs`; workers `LISTEN` and also poll every 5 seconds.
- **Transactional.** Jobs are enqueued with the caller's transaction. Together with the in-process event bus, whose subscribers run inside the publisher's transaction, this gives the guarantees of a transactional outbox: the lead and its sync jobs commit together or not at all, with no outbox table or relay.
- **Retries.** Exponential backoff with jitter from 30 seconds, capped at 6 hours, 10 attempts by default. Handlers mark unrecoverable errors with `jobs.Permanent`. Delivery is at least once: a job whose worker died is reclaimed after a 10-minute lease. Finished jobs are deleted after 30 days.
- **Periodic tasks.** `jobs.Task` runs at start-up and on an interval, holding a Postgres advisory lock so only one instance runs it at a time. The usage snapshot and analytics retention moved onto it.
- **Configuration.** `JOB_WORKERS` (default 2) sets the workers per instance; `0` makes an instance queue only.

## Consequences

- No new infrastructure. Backups of the database include the queue.
- Throughput is bounded by Postgres, which is far beyond what lead sync needs (tens of jobs per minute, not thousands per second).
- Handlers must be idempotent. Providers upsert, or send a stable delivery id.
- The event bus is in-process and synchronous. Subscribers must be quick and must not call out; anything slow goes into a job. Cross-process fan-out isn't possible without a different design, and isn't needed yet.
- A worker holds a connection for `LISTEN` outside the pool.
