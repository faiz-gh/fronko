package jobs

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// DefaultPollInterval is how often idle workers look for due jobs when no
	// notification arrives (delayed retries, or a missed NOTIFY).
	DefaultPollInterval = 5 * time.Second
	// DefaultJobTimeout bounds one run of a job. It must stay well under
	// Lease, or a slow job would be reclaimed while still running.
	DefaultJobTimeout = 5 * time.Minute
	// Lease is how long a running job may go without finishing before it
	// is considered abandoned (its worker crashed) and queued again.
	Lease = 10 * time.Minute
	// Retention is how long finished (done or dead) jobs are kept, for
	// inspection, before they are deleted.
	Retention = 30 * 24 * time.Hour

	// maxErrorLen caps the stored last_error.
	maxErrorLen = 2000
)

// Worker runs queued jobs with a fixed number of goroutines.
type Worker struct {
	pool     *pgxpool.Pool
	handlers map[string]Handler
	kinds    []string
	n        int
	wake     chan struct{}

	// PollInterval and JobTimeout default to DefaultPollInterval and
	// DefaultJobTimeout; tests shorten them.
	PollInterval time.Duration
	JobTimeout   time.Duration
}

// NewWorker returns a worker that runs jobs of the kinds in handlers with
// n goroutines. It only claims kinds it has a handler for, so instances
// running different versions can share a queue.
func NewWorker(pool *pgxpool.Pool, handlers map[string]Handler, n int) *Worker {
	kinds := make([]string, 0, len(handlers))
	for k := range handlers {
		kinds = append(kinds, k)
	}
	slices.Sort(kinds)
	return &Worker{
		pool:         pool,
		handlers:     handlers,
		kinds:        kinds,
		n:            n,
		wake:         make(chan struct{}, max(n, 1)),
		PollInterval: DefaultPollInterval,
		JobTimeout:   DefaultJobTimeout,
	}
}

// Run processes jobs until ctx ends, then waits for running jobs to stop.
// Jobs cut short by shutdown are put back in the queue without using up an
// attempt.
func (w *Worker) Run(ctx context.Context) {
	if w.n <= 0 || len(w.kinds) == 0 {
		return
	}
	var wg sync.WaitGroup
	wg.Go(func() { w.listen(ctx) })
	for range w.n {
		wg.Go(func() { w.work(ctx) })
	}
	wg.Wait()
}

// signal wakes one idle worker, if any.
func (w *Worker) signal() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// listen turns NOTIFYs on the jobs channel into wake-ups, reconnecting if
// the connection drops.
func (w *Worker) listen(ctx context.Context) {
	for {
		err := w.listenOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		log.Printf("jobs: listening for new jobs: %v (polling until reconnected)", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(w.PollInterval):
		}
	}
}

func (w *Worker) listenOnce(ctx context.Context) error {
	pc, err := w.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	// The connection stays in LISTEN mode for good, so take it out of the pool.
	conn := pc.Hijack()
	defer conn.Close(context.Background())
	if _, err := conn.Exec(ctx, "LISTEN "+notifyChannel); err != nil {
		return err
	}
	// Catch up on anything queued while we weren't listening.
	w.signal()
	for {
		if _, err := conn.WaitForNotification(ctx); err != nil {
			return err
		}
		w.signal()
	}
}

func (w *Worker) work(ctx context.Context) {
	idle := time.NewTimer(w.PollInterval)
	defer idle.Stop()
	for {
		job, err := w.claim(ctx)
		if err != nil && ctx.Err() == nil {
			log.Printf("jobs: claiming a job: %v", err)
		}
		if job != nil {
			// There may be more; let another idle worker look too.
			w.signal()
			w.run(ctx, job)
			continue
		}
		idle.Reset(w.PollInterval)
		select {
		case <-ctx.Done():
			return
		case <-w.wake:
		case <-idle.C:
		}
	}
}

// claim takes the oldest due job this worker can run, or returns nil.
func (w *Worker) claim(ctx context.Context) (*Job, error) {
	var j Job
	var orgID *int64
	err := w.pool.QueryRow(ctx, `
		UPDATE jobs SET status = 'running', locked_at = now(), attempts = attempts + 1
		WHERE job_id = (
			SELECT job_id FROM jobs
			WHERE status = 'queued' AND run_at <= now() AND kind = ANY($1)
			ORDER BY run_at, job_id
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING job_id, kind, payload, org_id, attempts, max_attempts, locked_at`, w.kinds,
	).Scan(&j.ID, &j.Kind, &j.Payload, &orgID, &j.Attempt, &j.MaxAttempts, &j.lockedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if orgID != nil {
		j.OrgID = *orgID
	}
	return &j, nil
}

func (w *Worker) run(ctx context.Context, job *Job) {
	jobCtx, cancel := context.WithTimeout(ctx, w.JobTimeout)
	err := call(jobCtx, w.handlers[job.Kind], job)
	cancel()

	// Recording the outcome must not be cut short by shutdown.
	done, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err != nil && ctx.Err() != nil {
		// Interrupted by shutdown: not the job's fault, so don't count it.
		w.finish(done, job, `status = 'queued', attempts = attempts - 1`)
		return
	}
	switch {
	case err == nil:
		w.finish(done, job, `status = 'done', finished_at = now(), last_error = NULL`)
	case IsPermanent(err) || job.Attempt >= job.MaxAttempts:
		log.Printf("jobs: %s job %d failed for good (attempt %d of %d): %v", job.Kind, job.ID, job.Attempt, job.MaxAttempts, err)
		w.finish(done, job, `status = 'dead', finished_at = now(), last_error = $3`, errorText(err))
	default:
		wait := backoff(job.Attempt)
		log.Printf("jobs: %s job %d failed (attempt %d of %d), retrying in %s: %v",
			job.Kind, job.ID, job.Attempt, job.MaxAttempts, wait.Round(time.Second), err)
		w.finish(done, job, `status = 'queued', run_at = now() + $3::interval, last_error = $4`, wait, errorText(err))
	}
}

// finish records a job's outcome. The locked_at check makes it a no-op if
// the job's lease expired and another worker has claimed it since.
func (w *Worker) finish(ctx context.Context, job *Job, set string, args ...any) {
	args = append([]any{job.ID, job.lockedAt}, args...)
	_, err := w.pool.Exec(ctx, `UPDATE jobs SET `+set+`, locked_at = NULL
		WHERE job_id = $1 AND status = 'running' AND locked_at = $2`, args...)
	if err != nil {
		log.Printf("jobs: recording the outcome of %s job %d: %v", job.Kind, job.ID, err)
	}
}

// call runs h, turning a panic into an error so one bad job can't take the
// worker down.
func call(ctx context.Context, h Handler, job *Job) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return h(ctx, job)
}

func errorText(err error) string {
	s := err.Error()
	if len(s) > maxErrorLen {
		// Cutting mid-character would leave invalid UTF-8, which Postgres rejects.
		s = strings.ToValidUTF8(s[:maxErrorLen], "") + "…"
	}
	return s
}
