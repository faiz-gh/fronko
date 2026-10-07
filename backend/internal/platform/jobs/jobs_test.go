package jobs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackoffDoublesWithJitterAndCaps(t *testing.T) {
	for attempt := 1; attempt <= 30; attempt++ {
		full := min(baseDelay<<min(attempt-1, 20), maxDelay)
		for range 50 {
			d := backoff(attempt)
			require.GreaterOrEqual(t, d, full/2, "attempt %d", attempt)
			require.LessOrEqual(t, d, full, "attempt %d", attempt)
		}
	}
	assert.LessOrEqual(t, backoff(1), baseDelay)
	assert.GreaterOrEqual(t, backoff(100), maxDelay/2)
}

func TestPermanentErrors(t *testing.T) {
	base := errors.New("bad config")
	err := fmt.Errorf("pushing lead: %w", Permanent(base))
	assert.True(t, IsPermanent(err))
	assert.ErrorIs(t, err, base)
	assert.False(t, IsPermanent(base))
	assert.NoError(t, Permanent(nil))
}

func TestDecodeFailureIsPermanent(t *testing.T) {
	var v struct{ LeadID int64 }
	ok := &Job{Kind: "k", Payload: []byte(`{"LeadID": 7}`)}
	require.NoError(t, ok.Decode(&v))
	assert.EqualValues(t, 7, v.LeadID)
	bad := &Job{Kind: "k", Payload: []byte(`{"LeadID": "seven"}`)}
	assert.True(t, IsPermanent(bad.Decode(&v)))
}

func TestCallTurnsPanicsIntoErrors(t *testing.T) {
	err := call(context.Background(), func(context.Context, *Job) error { panic("boom") }, &Job{})
	assert.EqualError(t, err, "panic: boom")
}

func TestErrorTextStaysValidUTF8(t *testing.T) {
	s := errorText(errors.New(strings.Repeat("é", maxErrorLen)))
	assert.True(t, utf8.ValidString(s))
	assert.LessOrEqual(t, len(s), maxErrorLen+len("…"))
}

func TestTasksRunAtStartAndOnEachTick(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var a, b atomic.Int32
	tasks := []Task{
		{Name: "a", Every: 10 * time.Millisecond, Run: func(context.Context) error { a.Add(1); return nil }},
		// A failing task keeps running on schedule.
		{Name: "b", Every: 10 * time.Millisecond, Run: func(context.Context) error { b.Add(1); return errors.New("boom") }},
	}
	done := make(chan struct{})
	go func() {
		runEach(ctx, tasks, func(ctx context.Context, t Task) error { return t.Run(ctx) })
		close(done)
	}()
	assert.Eventually(t, func() bool { return a.Load() >= 3 && b.Load() >= 3 }, time.Second, 5*time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("tasks did not stop when their context ended")
	}
}

func TestLockKeysDifferByName(t *testing.T) {
	assert.Equal(t, lockKey("usage snapshot"), lockKey("usage snapshot"))
	assert.NotEqual(t, lockKey("usage snapshot"), lockKey("analytics retention"))
}
