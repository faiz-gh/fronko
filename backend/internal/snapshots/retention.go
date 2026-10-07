package snapshots

import (
	"context"
	"log"
	"time"
)

// AnalyticsPurger deletes card analytics events older than a cut-off.
type AnalyticsPurger interface {
	PurgeAnalytics(ctx context.Context, before time.Time) (int64, error)
}

type retention struct {
	Snapshotter
	purger AnalyticsPurger
	keep   time.Duration
}

// WithAnalyticsRetention makes each snapshot run also delete analytics events
// older than keep (and stale visitor-hash salts). A failed sweep is logged and
// doesn't hold up the snapshot.
func WithAnalyticsRetention[T interface {
	Snapshotter
	AnalyticsPurger
}](s T, keep time.Duration) Snapshotter {
	return retention{Snapshotter: s, purger: s, keep: keep}
}

func (r retention) TakeUsageSnapshot(ctx context.Context, day time.Time) error {
	if n, err := r.purger.PurgeAnalytics(ctx, day.Add(-r.keep)); err != nil {
		log.Printf("analytics retention: %v", err)
	} else if n > 0 {
		log.Printf("analytics retention: deleted %d events", n)
	}
	return r.Snapshotter.TakeUsageSnapshot(ctx, day)
}
