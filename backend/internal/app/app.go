// Package app is the contract between the server and its feature modules.
//
// A module (cards, leads, files, …) owns its handlers, its SQL and its types.
// It plugs into the server by registering routes on a Routes, and optionally
// by subscribing to events, handling background jobs or contributing
// periodic tasks. The composition root (package server) builds every module
// with its dependencies, hands it the Routes and checks for the optional
// interfaces below.
package app

import (
	"github.com/faiz-gh/fronko/backend/internal/platform/events"
	"github.com/faiz-gh/fronko/backend/internal/platform/jobs"
)

// Module is a feature that serves HTTP routes.
type Module interface {
	Routes(r *Routes)
}

// Subscriber is implemented by modules that react to other modules' events.
type Subscriber interface {
	Subscribe(b *events.Bus)
}

// Worker is implemented by modules that run background jobs. Keys are job
// kinds, prefixed with the module's name ("integrations.push_lead").
type Worker interface {
	JobHandlers() map[string]jobs.Handler
}

// Scheduled is implemented by modules with periodic background work.
type Scheduled interface {
	Tasks() []jobs.Task
}
