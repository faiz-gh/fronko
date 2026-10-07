// Package schedule runs periodic background tasks, such as the daily usage
// snapshot and the analytics retention sweep.
package schedule

import (
	"context"
	"log"
	"sync"
	"time"
)

// defaultTimeout bounds one run of a task that doesn't set its own.
const defaultTimeout = time.Minute

// Task is work that runs once at start-up and then every Every.
type Task struct {
	// Name identifies the task in logs.
	Name  string
	Every time.Duration
	// Timeout bounds one run; zero means a minute.
	Timeout time.Duration
	Run     func(ctx context.Context) error
}

// Run starts every task and blocks until ctx ends and they have all stopped.
// A failed run is logged and the task carries on at its next tick. Missed
// runs (the server was down) are not made up.
func Run(ctx context.Context, tasks []Task) {
	var wg sync.WaitGroup
	for _, t := range tasks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			loop(ctx, t)
		}()
	}
	wg.Wait()
}

func loop(ctx context.Context, t Task) {
	timeout := t.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	run := func() {
		runCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		if err := t.Run(runCtx); err != nil && ctx.Err() == nil {
			log.Printf("%s: %v", t.Name, err)
		}
	}
	run()
	ticker := time.NewTicker(t.Every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
