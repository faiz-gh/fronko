package schedule

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRunStartsEachTaskAndRepeatsOnEachTick(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var a, b atomic.Int32
	tasks := []Task{
		{Name: "a", Every: 10 * time.Millisecond, Run: func(context.Context) error { a.Add(1); return nil }},
		// A failing task keeps running on schedule.
		{Name: "b", Every: 10 * time.Millisecond, Run: func(context.Context) error { b.Add(1); return errors.New("boom") }},
	}
	done := make(chan struct{})
	go func() {
		Run(ctx, tasks)
		close(done)
	}()
	assert.Eventually(t, func() bool { return a.Load() >= 3 && b.Load() >= 3 }, time.Second, 5*time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop when its context ended")
	}
}
