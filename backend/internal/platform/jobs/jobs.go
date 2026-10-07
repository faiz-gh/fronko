// Package jobs is a background job queue in Postgres, plus the periodic
// tasks that keep the app tidy.
//
// A job is a row in the jobs table: a kind (which Handler runs it), a JSON
// payload, and its retry state. Jobs are usually queued with Enqueue inside
// the transaction that caused them, often from an events subscriber, so a
// job exists exactly when the change that needs it committed.
//
// Workers (see Worker) claim due jobs with FOR UPDATE SKIP LOCKED, so any
// number of them, in any number of server instances, share the queue without
// running a job twice at once. They wake on LISTEN/NOTIFY when a job is
// queued, and poll every few seconds as a fallback.
//
// Delivery is at least once: a job whose worker died is run again after its
// lease expires. Handlers must therefore be idempotent, for example by
// upserting rather than inserting on the remote side.
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/faiz-gh/fronko/backend/internal/platform/database"
)

// notifyChannel is the LISTEN/NOTIFY channel that wakes workers.
const notifyChannel = "fronko_jobs"

// DefaultMaxAttempts is how many times a job runs before it is dead. With the
// backoff below, ten attempts span about four hours.
const DefaultMaxAttempts = 10

// Retry backoff: the delay doubles from baseDelay with each failed attempt,
// up to maxDelay, with jitter so a burst of failures doesn't retry in step.
const (
	baseDelay = 30 * time.Second
	maxDelay  = 6 * time.Hour
)

// Job is one claimed job, as its Handler sees it.
type Job struct {
	ID      int64
	Kind    string
	Payload json.RawMessage
	// OrgID is the organisation the job is for, or 0.
	OrgID int64
	// Attempt is 1 on the first run.
	Attempt     int
	MaxAttempts int

	lockedAt time.Time
}

// Decode unmarshals the payload into v.
func (j *Job) Decode(v any) error {
	if err := json.Unmarshal(j.Payload, v); err != nil {
		return Permanent(fmt.Errorf("decoding %s payload: %w", j.Kind, err))
	}
	return nil
}

// Handler runs one job. Returning nil marks it done. Any other error retries
// it later, unless the error is Permanent or the job is out of attempts; then
// it is dead. The context ends when the job times out or the server stops.
type Handler func(ctx context.Context, job *Job) error

// Options tune a queued job. The zero value runs it now with the default
// number of attempts.
type Options struct {
	// OrgID ties the job to an organisation, so it goes when the
	// organisation is deleted.
	OrgID int64
	// DedupeKey, when set, makes Enqueue a no-op while a job of the same
	// kind and key is still queued or running.
	DedupeKey string
	// RunAt delays the first run.
	RunAt time.Time
	// MaxAttempts defaults to DefaultMaxAttempts.
	MaxAttempts int
}

// Enqueue queues a job of the given kind. q is normally the caller's
// transaction: the job, and the notification that wakes a worker, then
// only take effect if it commits. It reports false when DedupeKey matched a
// job already pending.
func Enqueue(ctx context.Context, q database.Querier, kind string, payload any, opts Options) (bool, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("encoding %s payload: %w", kind, err)
	}
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = DefaultMaxAttempts
	}
	var runAt *time.Time
	if !opts.RunAt.IsZero() {
		runAt = &opts.RunAt
	}
	tag, err := q.Exec(ctx, `
		WITH queued AS (
			INSERT INTO jobs (kind, payload, org_id, dedupe_key, run_at, max_attempts)
			VALUES ($1, $2, NULLIF($3::bigint, 0), NULLIF($4, ''), COALESCE($5, now()), $6)
			ON CONFLICT (kind, dedupe_key) WHERE dedupe_key IS NOT NULL AND status IN ('queued', 'running')
			DO NOTHING
			RETURNING 1
		)
		SELECT pg_notify('`+notifyChannel+`', '') FROM queued`,
		kind, data, opts.OrgID, opts.DedupeKey, runAt, opts.MaxAttempts)
	if err != nil {
		return false, fmt.Errorf("queueing %s job: %w", kind, err)
	}
	return tag.RowsAffected() > 0, nil
}

type permanentError struct{ err error }

func (e permanentError) Error() string { return e.err.Error() }
func (e permanentError) Unwrap() error { return e.err }

// Permanent marks err as one that retrying won't fix (bad configuration,
// a payload the handler can't read, a 4xx from a remote API), so the job is
// marked dead straight away.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return permanentError{err}
}

// IsPermanent reports whether err was marked with Permanent.
func IsPermanent(err error) bool {
	var p permanentError
	return errors.As(err, &p)
}

// backoff is the wait before the next try after the given failed attempt
// (1-based): about 30s, 1m, 2m, … capped at 6h, each between half and the
// whole of that.
func backoff(attempt int) time.Duration {
	d := maxDelay
	if attempt < 20 {
		d = min(baseDelay<<max(attempt-1, 0), maxDelay)
	}
	return d/2 + rand.N(d/2+1)
}
