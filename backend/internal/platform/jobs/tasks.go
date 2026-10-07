package jobs

import (
	"context"
	"hash/fnv"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// defaultTaskTimeout bounds one run of a task that doesn't set its own.
const defaultTaskTimeout = time.Minute

// Task is periodic work, such as the daily usage snapshot: it runs once at
// start-up and then every Every.
type Task struct {
	// Name identifies the task in logs and names its lock, so keep it stable.
	Name  string
	Every time.Duration
	// Timeout bounds one run; zero means a minute.
	Timeout time.Duration
	Run     func(ctx context.Context) error
}

// RunTasks runs every task on its interval until ctx ends, then waits for
// them to stop. Each run holds a Postgres advisory lock named after the
// task, so with several server instances a task never runs twice at once;
// an instance that finds the lock taken skips that run. A failed run is
// logged and the task carries on at its next tick. Missed runs (the server
// was down) are not made up, so tasks should do whatever is due when they
// run rather than assume a fixed period has passed.
func RunTasks(ctx context.Context, pool *pgxpool.Pool, tasks []Task) {
	runEach(ctx, tasks, func(ctx context.Context, t Task) error { return runLocked(ctx, pool, t) })
}

// runLocked runs t while holding its advisory lock, or does nothing if
// another instance holds it. The lock belongs to a transaction, so Postgres
// releases it however the run ends.
func runLocked(ctx context.Context, pool *pgxpool.Pool, t Task) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	var locked bool
	if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, lockKey(t.Name)).Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return nil
	}
	return t.Run(ctx)
}

// lockKey turns a task name into an advisory lock key.
func lockKey(name string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("fronko.task:" + name))
	return int64(h.Sum64())
}

// runEach is RunTasks with the run step pluggable, for tests.
func runEach(ctx context.Context, tasks []Task, run func(context.Context, Task) error) {
	var wg sync.WaitGroup
	for _, t := range tasks {
		wg.Go(func() { loop(ctx, t, run) })
	}
	wg.Wait()
}

func loop(ctx context.Context, t Task, run func(context.Context, Task) error) {
	timeout := t.Timeout
	if timeout == 0 {
		timeout = defaultTaskTimeout
	}
	once := func() {
		runCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		if err := run(runCtx, t); err != nil && ctx.Err() == nil {
			log.Printf("%s: %v", t.Name, err)
		}
	}
	once()
	ticker := time.NewTicker(t.Every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			once()
		}
	}
}

// MaintenanceTasks keeps the jobs table healthy: it requeues jobs whose
// worker stopped responding, and deletes finished jobs after Retention.
func MaintenanceTasks(pool *pgxpool.Pool) []Task {
	return []Task{
		{
			Name:  "jobs: reclaim abandoned",
			Every: time.Minute,
			Run: func(ctx context.Context) error {
				tag, err := pool.Exec(ctx, `
					UPDATE jobs SET
						status = CASE WHEN attempts >= max_attempts THEN 'dead' ELSE 'queued' END,
						finished_at = CASE WHEN attempts >= max_attempts THEN now() END,
						locked_at = NULL,
						last_error = 'the worker running it stopped responding'
					WHERE status = 'running' AND locked_at < now() - $1::interval`, Lease)
				if n := tag.RowsAffected(); n > 0 {
					log.Printf("jobs: reclaimed %d abandoned jobs", n)
				}
				return err
			},
		},
		{
			Name:  "jobs: delete finished",
			Every: time.Hour,
			Run: func(ctx context.Context) error {
				_, err := pool.Exec(ctx, `DELETE FROM jobs
					WHERE status IN ('done', 'dead') AND finished_at < now() - $1::interval`, Retention)
				return err
			},
		},
	}
}
