package snapshots

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type countingSnapshotter struct{ n atomic.Int32 }

func (c *countingSnapshotter) TakeUsageSnapshot(context.Context, time.Time) error {
	c.n.Add(1)
	return nil
}

func TestRunSnapshotsAtStartAndOnEachTick(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &countingSnapshotter{}
	done := make(chan struct{})
	go func() {
		Run(ctx, s, 10*time.Millisecond)
		close(done)
	}()
	assert.Eventually(t, func() bool { return s.n.Load() >= 3 }, time.Second, 5*time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop when its context ended")
	}
}
