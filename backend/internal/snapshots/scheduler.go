// Package snapshots records daily usage counts for the admin panel's trends.
package snapshots

import (
	"context"
	"log"
	"time"
)

// Snapshotter records usage for a day; running it twice the same day overwrites that day.
type Snapshotter interface {
	TakeUsageSnapshot(ctx context.Context, day time.Time) error
}

// Run snapshots usage right away and then every interval until ctx ends.
// Each run rewrites today's row, so today's point stays current and the last
// run of a day becomes that day's final value. Missed runs (the server was
// down) only leave gaps in the trend.
func Run(ctx context.Context, s Snapshotter, interval time.Duration) {
	take := func() {
		runCtx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		if err := s.TakeUsageSnapshot(runCtx, time.Now()); err != nil && ctx.Err() == nil {
			log.Printf("usage snapshot: %v", err)
		}
	}
	take()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			take()
		}
	}
}
