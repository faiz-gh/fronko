package snapshots

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type purgingSnapshotter struct {
	countingSnapshotter
	before time.Time
	err    error
}

func (p *purgingSnapshotter) PurgeAnalytics(_ context.Context, before time.Time) (int64, error) {
	p.before = before
	return 3, p.err
}

func TestRetentionPurgesThenSnapshots(t *testing.T) {
	p := &purgingSnapshotter{}
	day := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	assert.NoError(t, WithAnalyticsRetention(p, 10*24*time.Hour).TakeUsageSnapshot(context.Background(), day))
	assert.Equal(t, day.Add(-10*24*time.Hour), p.before)
	assert.EqualValues(t, 1, p.n.Load())

	p.err = errors.New("boom")
	assert.NoError(t, WithAnalyticsRetention(p, time.Hour).TakeUsageSnapshot(context.Background(), day), "a failed sweep doesn't block the snapshot")
	assert.EqualValues(t, 2, p.n.Load())
}
