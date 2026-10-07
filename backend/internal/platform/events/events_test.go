package events

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/faiz-gh/fronko/backend/internal/platform/database"
)

type created struct{ ID int64 }

func (created) EventName() string { return "test.created" }

type deleted struct{ ID int64 }

func (deleted) EventName() string { return "test.deleted" }

func TestPublishRunsSubscribersOfThatTypeInOrder(t *testing.T) {
	b := New()
	var got []string
	Subscribe(b, func(_ context.Context, _ database.Querier, e created) error {
		got = append(got, "first", string(rune('0'+e.ID)))
		return nil
	})
	Subscribe(b, func(_ context.Context, _ database.Querier, e created) error {
		got = append(got, "second")
		return nil
	})
	Subscribe(b, func(_ context.Context, _ database.Querier, e deleted) error {
		got = append(got, "deleted")
		return nil
	})

	require.NoError(t, b.Publish(context.Background(), nil, created{ID: 7}))
	assert.Equal(t, []string{"first", "7", "second"}, got)
}

func TestPublishStopsAtTheFirstError(t *testing.T) {
	b := New()
	boom := errors.New("boom")
	ran := false
	Subscribe(b, func(context.Context, database.Querier, created) error { return boom })
	Subscribe(b, func(context.Context, database.Querier, created) error { ran = true; return nil })

	err := b.Publish(context.Background(), nil, created{})
	assert.ErrorIs(t, err, boom)
	assert.Contains(t, err.Error(), "test.created")
	assert.False(t, ran)
}

func TestEventsWithoutSubscribersAndNilBus(t *testing.T) {
	assert.NoError(t, New().Publish(context.Background(), nil, created{}))
	var b *Bus
	assert.NoError(t, b.Publish(context.Background(), nil, created{}))
}
