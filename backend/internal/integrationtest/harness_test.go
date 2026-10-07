//go:build integration

// Package integrationtest runs the stores of every module against a real
// PostgreSQL. The tests TRUNCATE every table, so they read TEST_DATABASE_URL,
// never DATABASE_URL.
package integrationtest_test

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/faiz-gh/fronko/backend/internal/analytics"
	"github.com/faiz-gh/fronko/backend/internal/branding"
	"github.com/faiz-gh/fronko/backend/internal/cards"
	"github.com/faiz-gh/fronko/backend/internal/feedback"
	"github.com/faiz-gh/fronko/backend/internal/files"
	"github.com/faiz-gh/fronko/backend/internal/leads"
	"github.com/faiz-gh/fronko/backend/internal/orgs"
	"github.com/faiz-gh/fronko/backend/internal/platformadmin"
	"github.com/faiz-gh/fronko/backend/internal/teams"
	"github.com/faiz-gh/fronko/backend/internal/users"
)

// stores puts every module's store behind one value, so a test can set up
// users, cards and leads and then query analytics without juggling handles.
// Store method names are unique across modules, so none of them collide.
type stores struct {
	*userStore
	*orgStore
	*teamStore
	*brandingStore
	*fileStore
	*cardStore
	*leadStore
	*analyticsStore
	*feedbackStore
	*adminStore
}

// Aliases give each embedded store its own field name.
type (
	userStore      = users.Store
	orgStore       = orgs.Store
	teamStore      = teams.Store
	brandingStore  = branding.Store
	fileStore      = files.Store
	cardStore      = cards.Store
	leadStore      = leads.Store
	analyticsStore = analytics.Store
	feedbackStore  = feedback.Store
	adminStore     = platformadmin.Store
)

func newStores(pool *pgxpool.Pool) stores {
	return stores{
		users.NewStore(pool), orgs.NewStore(pool), teams.NewStore(pool), branding.NewStore(pool),
		files.NewStore(pool), cards.NewStore(pool), leads.NewStore(pool), analytics.NewStore(pool),
		feedback.NewStore(pool), platformadmin.NewStore(pool),
	}
}

func ptr[T any](v T) *T { return &v }
