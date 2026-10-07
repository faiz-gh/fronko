//go:build integration

package integrationtest_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/faiz-gh/fronko/backend/internal/cards"
	"github.com/faiz-gh/fronko/backend/internal/leads"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/events"
	"github.com/faiz-gh/fronko/backend/internal/platform/jobs"
	"github.com/faiz-gh/fronko/backend/internal/users"
)

type jobRow struct {
	Status    string
	Attempts  int
	LastError *string
	RunAt     time.Time
	Finished  *time.Time
}

func TestJobsIntegration(t *testing.T) {
	dbUrl := os.Getenv("TEST_DATABASE_URL")
	if dbUrl == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration tests")
	}

	ctx := context.Background()
	pool, err := database.NewPool(ctx, dbUrl)
	require.NoError(t, err)
	defer pool.Close()

	reset := func(t *testing.T) {
		t.Helper()
		_, err := pool.Exec(ctx, "TRUNCATE TABLE jobs RESTART IDENTITY")
		require.NoError(t, err)
	}
	enqueue := func(t *testing.T, kind string, payload any, opts jobs.Options) int64 {
		t.Helper()
		queued, err := jobs.Enqueue(ctx, pool, kind, payload, opts)
		require.NoError(t, err)
		require.True(t, queued)
		var id int64
		require.NoError(t, pool.QueryRow(ctx, `SELECT max(job_id) FROM jobs`).Scan(&id))
		return id
	}
	row := func(t *testing.T, id int64) jobRow {
		t.Helper()
		var r jobRow
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT status, attempts, last_error, run_at, finished_at FROM jobs WHERE job_id = $1`, id,
		).Scan(&r.Status, &r.Attempts, &r.LastError, &r.RunAt, &r.Finished))
		return r
	}
	// runWorker runs a worker until stop is called. A long poll interval
	// means jobs are only picked up promptly through LISTEN/NOTIFY.
	runWorker := func(t *testing.T, handlers map[string]jobs.Handler) (stop func()) {
		t.Helper()
		w := jobs.NewWorker(pool, handlers, 2)
		w.PollInterval = time.Hour
		w.JobTimeout = 5 * time.Second
		wctx, cancel := context.WithCancel(ctx)
		done := make(chan struct{})
		go func() {
			w.Run(wctx)
			close(done)
		}()
		var once sync.Once
		stop = func() {
			once.Do(func() {
				cancel()
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Error("worker did not stop")
				}
			})
		}
		t.Cleanup(stop)
		return stop
	}
	waitStatus := func(t *testing.T, id int64, status string) jobRow {
		t.Helper()
		var r jobRow
		require.Eventually(t, func() bool { r = row(t, id); return r.Status == status }, 5*time.Second, 20*time.Millisecond,
			"job %d never became %s", id, status)
		return r
	}

	t.Run("enqueue in a transaction only counts if it commits", func(t *testing.T) {
		reset(t)
		err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			_, err := jobs.Enqueue(ctx, tx, "test.noop", map[string]int{"n": 1}, jobs.Options{})
			require.NoError(t, err)
			return errors.New("roll back")
		})
		require.Error(t, err)
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM jobs`).Scan(&n))
		assert.Zero(t, n)
	})

	t.Run("dedupe keys collapse pending jobs only", func(t *testing.T) {
		reset(t)
		opts := jobs.Options{DedupeKey: "lead-1"}
		first := enqueue(t, "test.sync", nil, opts)
		queued, err := jobs.Enqueue(ctx, pool, "test.sync", nil, opts)
		require.NoError(t, err)
		assert.False(t, queued, "same kind and key while queued")
		other, err := jobs.Enqueue(ctx, pool, "test.other", nil, opts)
		require.NoError(t, err)
		assert.True(t, other, "the key is per kind")

		_, err = pool.Exec(ctx, `UPDATE jobs SET status = 'done', finished_at = now() WHERE job_id = $1`, first)
		require.NoError(t, err)
		again, err := jobs.Enqueue(ctx, pool, "test.sync", nil, opts)
		require.NoError(t, err)
		assert.True(t, again, "a finished job doesn't block a new one")
	})

	t.Run("a worker runs a queued job and marks it done", func(t *testing.T) {
		reset(t)
		got := make(chan *jobs.Job, 1)
		runWorker(t, map[string]jobs.Handler{"test.hello": func(_ context.Context, j *jobs.Job) error {
			got <- j
			return nil
		}})
		// Give the worker a moment to start listening; the job must still
		// arrive through NOTIFY, not the (hour-long) poll.
		time.Sleep(200 * time.Millisecond)
		id := enqueue(t, "test.hello", map[string]string{"to": "world"}, jobs.Options{})

		select {
		case j := <-got:
			assert.Equal(t, id, j.ID)
			assert.Equal(t, 1, j.Attempt)
			var p struct{ To string }
			require.NoError(t, j.Decode(&p))
			assert.Equal(t, "world", p.To)
		case <-time.After(5 * time.Second):
			t.Fatal("job never ran")
		}
		r := waitStatus(t, id, "done")
		assert.NotNil(t, r.Finished)
		assert.Nil(t, r.LastError)
	})

	t.Run("jobs queued before the worker started are picked up", func(t *testing.T) {
		reset(t)
		id := enqueue(t, "test.backlog", nil, jobs.Options{})
		runWorker(t, map[string]jobs.Handler{"test.backlog": func(context.Context, *jobs.Job) error { return nil }})
		waitStatus(t, id, "done")
	})

	t.Run("a failed job is retried later with its error kept", func(t *testing.T) {
		reset(t)
		var runs atomic.Int32
		runWorker(t, map[string]jobs.Handler{"test.flaky": func(context.Context, *jobs.Job) error {
			runs.Add(1)
			return errors.New("remote said 503")
		}})
		id := enqueue(t, "test.flaky", nil, jobs.Options{})
		require.Eventually(t, func() bool { r := row(t, id); return r.Status == "queued" && r.Attempts == 1 },
			5*time.Second, 20*time.Millisecond)
		r := row(t, id)
		require.NotNil(t, r.LastError)
		assert.Equal(t, "remote said 503", *r.LastError)
		assert.Greater(t, time.Until(r.RunAt), 10*time.Second, "backs off before the next try")
		assert.EqualValues(t, 1, runs.Load())
	})

	t.Run("permanent errors, panics on the last attempt, and running out of attempts end in dead", func(t *testing.T) {
		reset(t)
		runWorker(t, map[string]jobs.Handler{
			"test.permanent": func(context.Context, *jobs.Job) error { return jobs.Permanent(errors.New("bad API key")) },
			"test.last":      func(context.Context, *jobs.Job) error { return errors.New("still down") },
			"test.panic":     func(context.Context, *jobs.Job) error { panic("nil map") },
		})
		permanent := enqueue(t, "test.permanent", nil, jobs.Options{})
		last := enqueue(t, "test.last", nil, jobs.Options{MaxAttempts: 1})
		panicked := enqueue(t, "test.panic", nil, jobs.Options{MaxAttempts: 1})

		r := waitStatus(t, permanent, "dead")
		assert.Equal(t, 1, r.Attempts)
		assert.Equal(t, "bad API key", *r.LastError)
		assert.NotNil(t, r.Finished)
		assert.Equal(t, "still down", *waitStatus(t, last, "dead").LastError)
		assert.Equal(t, "panic: nil map", *waitStatus(t, panicked, "dead").LastError)
	})

	t.Run("workers leave kinds they can't handle alone", func(t *testing.T) {
		reset(t)
		foreign := enqueue(t, "test.unknown", nil, jobs.Options{})
		mine := enqueue(t, "test.mine", nil, jobs.Options{})
		runWorker(t, map[string]jobs.Handler{"test.mine": func(context.Context, *jobs.Job) error { return nil }})
		waitStatus(t, mine, "done")
		assert.Equal(t, "queued", row(t, foreign).Status)
	})

	t.Run("delayed jobs wait for run_at", func(t *testing.T) {
		reset(t)
		var ran atomic.Bool
		runWorker(t, map[string]jobs.Handler{"test.later": func(context.Context, *jobs.Job) error { ran.Store(true); return nil }})
		id := enqueue(t, "test.later", nil, jobs.Options{RunAt: time.Now().Add(time.Hour)})
		time.Sleep(500 * time.Millisecond)
		assert.False(t, ran.Load())
		assert.Equal(t, "queued", row(t, id).Status)
	})

	t.Run("shutdown puts a running job back without using an attempt", func(t *testing.T) {
		reset(t)
		started := make(chan struct{})
		stop := runWorker(t, map[string]jobs.Handler{"test.slow": func(ctx context.Context, _ *jobs.Job) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		}})
		id := enqueue(t, "test.slow", nil, jobs.Options{})
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("job never started")
		}
		stop()
		r := row(t, id)
		assert.Equal(t, "queued", r.Status)
		assert.Equal(t, 0, r.Attempts)
	})

	t.Run("maintenance reclaims abandoned jobs and deletes old finished ones", func(t *testing.T) {
		reset(t)
		abandoned := enqueue(t, "test.a", nil, jobs.Options{})
		exhausted := enqueue(t, "test.b", nil, jobs.Options{MaxAttempts: 1})
		recent := enqueue(t, "test.c", nil, jobs.Options{})
		oldDone := enqueue(t, "test.d", nil, jobs.Options{})
		newDone := enqueue(t, "test.e", nil, jobs.Options{})
		_, err := pool.Exec(ctx, `UPDATE jobs SET status = 'running', attempts = 1, locked_at = now() - interval '11 minutes'
			WHERE job_id IN ($1, $2)`, abandoned, exhausted)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `UPDATE jobs SET status = 'running', attempts = 1, locked_at = now() WHERE job_id = $1`, recent)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `UPDATE jobs SET status = 'done', finished_at = now() - interval '31 days' WHERE job_id = $1`, oldDone)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `UPDATE jobs SET status = 'dead', finished_at = now() - interval '29 days' WHERE job_id = $1`, newDone)
		require.NoError(t, err)

		for _, task := range jobs.MaintenanceTasks(pool) {
			require.NoError(t, task.Run(ctx), task.Name)
		}

		r := row(t, abandoned)
		assert.Equal(t, "queued", r.Status)
		assert.Equal(t, "the worker running it stopped responding", *r.LastError)
		assert.Equal(t, "dead", row(t, exhausted).Status)
		assert.Equal(t, "running", row(t, recent).Status, "still within its lease")
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE job_id = $1`, oldDone).Scan(&n))
		assert.Zero(t, n)
		assert.Equal(t, "dead", row(t, newDone).Status)
	})

	t.Run("a task runs on one instance at a time", func(t *testing.T) {
		var runs atomic.Int32
		release := make(chan struct{})
		task := jobs.Task{Name: "test exclusive", Every: time.Hour, Run: func(context.Context) error {
			runs.Add(1)
			<-release
			return nil
		}}
		tctx, cancel := context.WithCancel(ctx)
		var wg sync.WaitGroup
		// Two "instances", each with its own pool.
		for range 2 {
			p, err := pgxpool.New(ctx, dbUrl)
			require.NoError(t, err)
			defer p.Close()
			wg.Go(func() { jobs.RunTasks(tctx, p, []jobs.Task{task}) })
		}
		require.Eventually(t, func() bool { return runs.Load() >= 1 }, 5*time.Second, 10*time.Millisecond)
		time.Sleep(300 * time.Millisecond) // the other instance has tried by now
		assert.EqualValues(t, 1, runs.Load())
		close(release)
		cancel()
		wg.Wait()
	})

	t.Run("leads.Created is published in the lead's transaction", func(t *testing.T) {
		reset(t)
		_, err := pool.Exec(ctx, "TRUNCATE TABLE organizations, users, profiles, leads RESTART IDENTITY CASCADE")
		require.NoError(t, err)
		repo := newStores(pool)
		owner := &users.User{Username: "jobs-owner", PasswordHash: "hash"}
		require.NoError(t, repo.CreateOrgWithOwner(ctx, "Jobs Org", owner))
		card := &cards.Profile{OrgID: owner.OrgID, UserID: owner.ID, AssignedUserID: &owner.ID, Slug: "jobs-card",
			Data: json.RawMessage(`{}`)}
		require.NoError(t, repo.CreateProfile(ctx, card))

		bus := events.New()
		var fail atomic.Bool
		var seen leads.Created
		events.Subscribe(bus, func(ctx context.Context, q database.Querier, e leads.Created) error {
			seen = e
			if fail.Load() {
				return errors.New("subscriber failed")
			}
			_, err := jobs.Enqueue(ctx, q, "test.push_lead", e, jobs.Options{OrgID: e.OrgID})
			return err
		})
		store := leads.NewStore(pool, bus)

		lead := &leads.Lead{ProfileID: card.ID, Name: "Ada", Email: "ada@example.com"}
		require.NoError(t, store.CreateLead(ctx, lead))
		assert.Equal(t, leads.Created{LeadID: lead.ID, OrgID: owner.OrgID, ProfileID: card.ID, AssignedUserID: owner.ID}, seen)
		var payload leads.Created
		var orgID int64
		require.NoError(t, pool.QueryRow(ctx, `SELECT payload, org_id FROM jobs WHERE kind = 'test.push_lead'`).Scan(&payload, &orgID))
		assert.Equal(t, seen, payload)
		assert.Equal(t, owner.OrgID, orgID)

		// A failing subscriber rolls the lead back with it.
		fail.Store(true)
		err = store.CreateLead(ctx, &leads.Lead{ProfileID: card.ID, Name: "Bob", Email: "bob@example.com"})
		assert.ErrorContains(t, err, "leads.created subscriber")
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM leads`).Scan(&n))
		assert.Equal(t, 1, n)

		// Unassigned cards report no holder; missing cards still fail as before.
		fail.Store(false)
		unassigned := &cards.Profile{OrgID: owner.OrgID, UserID: owner.ID, Slug: "jobs-unassigned", Data: json.RawMessage(`{}`)}
		require.NoError(t, repo.CreateProfile(ctx, unassigned))
		require.NoError(t, store.CreateLead(ctx, &leads.Lead{ProfileID: unassigned.ID, Name: "Cy", Email: "cy@example.com"}))
		assert.Zero(t, seen.AssignedUserID)
		assert.ErrorIs(t, store.CreateLead(ctx, &leads.Lead{ProfileID: 999999, Name: "X", Email: "x@example.com"}), database.ErrNotFound)
	})
}
