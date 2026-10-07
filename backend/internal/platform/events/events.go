// Package events is an in-process bus for domain events, such as "a lead
// was created".
//
// Publishing is synchronous and transactional: Publish runs every subscriber
// with the publisher's database handle, normally the transaction that made
// the change. A subscriber that queues follow-up work (a background job)
// therefore does so atomically with the change itself: both commit, or
// neither does. That gives the guarantees of a transactional outbox without
// an outbox table.
//
// Subscribers must be quick and must not call out to other services; slow or
// fallible work belongs in a job (package jobs) that the subscriber queues.
package events

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/faiz-gh/fronko/backend/internal/platform/database"
)

// Event is a fact about something that happened. Events are plain structs,
// named in the past tense (leads.Created), defined by the module that
// publishes them.
type Event interface {
	// EventName identifies the event in logs and errors, e.g. "leads.created".
	EventName() string
}

type handler func(ctx context.Context, q database.Querier, evt Event) error

// Bus routes events to their subscribers. The zero value is not usable; use
// New. A nil *Bus is valid for publishing and drops every event, which keeps
// tests and tools that don't care about events simple.
type Bus struct {
	mu   sync.RWMutex
	subs map[reflect.Type][]handler
}

// New returns an empty bus.
func New() *Bus {
	return &Bus{subs: map[reflect.Type][]handler{}}
}

// Subscribe registers fn for events of type E. Subscribers run in the order
// they were registered. Register them while wiring the app, before serving.
func Subscribe[E Event](b *Bus, fn func(ctx context.Context, q database.Querier, evt E) error) {
	t := reflect.TypeFor[E]()
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs[t] = append(b.subs[t], func(ctx context.Context, q database.Querier, evt Event) error {
		return fn(ctx, q, evt.(E))
	})
}

// Publish runs every subscriber for evt with q, stopping at the first error.
// The publisher should return that error so its transaction rolls back.
func (b *Bus) Publish(ctx context.Context, q database.Querier, evt Event) error {
	if b == nil {
		return nil
	}
	b.mu.RLock()
	subs := b.subs[reflect.TypeOf(evt)]
	b.mu.RUnlock()
	for _, fn := range subs {
		if err := fn(ctx, q, evt); err != nil {
			return fmt.Errorf("%s subscriber: %w", evt.EventName(), err)
		}
	}
	return nil
}
