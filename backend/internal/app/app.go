// Package app is the contract between the server and its feature modules.
//
// A module (cards, leads, files, …) owns its handlers, its SQL and its types.
// It plugs into the server by registering routes on a Routes, and optionally
// by contributing periodic tasks. The composition root (package server)
// builds every module with its dependencies and hands it the Routes.
package app

import "github.com/faiz-gh/fronko/backend/internal/platform/schedule"

// Module is a feature that serves HTTP routes.
type Module interface {
	Routes(r *Routes)
}

// Scheduled is implemented by modules with periodic background work.
type Scheduled interface {
	Tasks() []schedule.Task
}
