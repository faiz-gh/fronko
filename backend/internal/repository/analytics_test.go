//go:build integration

package repository_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/faiz-gh/fronko/backend/internal/database"
	"github.com/faiz-gh/fronko/backend/internal/models"
	"github.com/faiz-gh/fronko/backend/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyticsIntegration(t *testing.T) {
	dbUrl := os.Getenv("TEST_DATABASE_URL")
	if dbUrl == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration tests")
	}

	ctx := context.Background()
	pool, err := database.NewPool(ctx, dbUrl)
	require.NoError(t, err)
	defer pool.Close()
	repo := repository.New(pool)

	_, err = pool.Exec(ctx, "TRUNCATE TABLE organizations, users, profiles, leads, teams, team_members, card_events, analytics_salts RESTART IDENTITY CASCADE")
	require.NoError(t, err)

	// An organisation with a team: lead + rep in it, and an outsider who isn't.
	owner := &models.User{Username: "an-owner", PasswordHash: "hash"}
	require.NoError(t, repo.CreateOrgWithOwner(ctx, "Analytics Org", owner))
	newMember := func(name string) *models.User {
		u := &models.User{OrgID: owner.OrgID, Role: models.RoleMember, Username: name, PasswordHash: "hash", CreatedBy: &owner.ID}
		require.NoError(t, repo.CreateUser(ctx, u))
		return u
	}
	lead, rep, outsider := newMember("an-lead"), newMember("an-rep"), newMember("an-outsider")
	team := &models.Team{OrgID: owner.OrgID, Name: "Sales"}
	require.NoError(t, repo.CreateTeam(ctx, team))
	require.NoError(t, repo.ReplaceTeamMembers(ctx, owner.OrgID, team.ID, []models.TeamMembership{
		{UserID: lead.ID, Role: models.TeamRoleLead}, {UserID: rep.ID, Role: models.TeamRoleMember},
	}))

	newCard := func(slug string, holder *models.User) *models.Profile {
		p := &models.Profile{OrgID: owner.OrgID, UserID: owner.ID, AssignedUserID: &holder.ID, Slug: slug,
			Data: json.RawMessage(`{"name":"` + slug + `"}`)}
		require.NoError(t, repo.CreateProfile(ctx, p))
		return p
	}
	repCard, outsiderCard := newCard("an-rep-card", rep), newCard("an-outsider-card", outsider)

	salt, err := repo.AnalyticsSalt(ctx, time.Now())
	require.NoError(t, err)
	again, err := repo.AnalyticsSalt(ctx, time.Now())
	require.NoError(t, err)
	assert.Equal(t, salt, again, "one salt per day")

	visit := func(card *models.Profile, session, source string, hash byte, events ...repository.CardEvent) {
		require.NoError(t, repo.RecordCardEvents(ctx, repository.EventBatch{
			ProfileID: card.ID, SessionID: session, VisitorHash: []byte{hash}, Source: source, Device: "mobile", Events: events,
		}))
	}
	view := repository.CardEvent{Type: models.EventView}
	// Two QR visits by the same visitor, one engaged (scrolled 75%, opened a brochure, saved the contact).
	visit(repCard, "11111111-1111-1111-1111-111111111111", models.SourceQR, 1,
		view, repository.CardEvent{Type: models.EventScroll, Value: 75},
		repository.CardEvent{Type: models.EventDocOpen, Target: "doc1", Label: "Brochure"},
		repository.CardEvent{Type: models.EventDocOpen, Target: "doc1", Label: "Brochure"},
		repository.CardEvent{Type: models.EventLeave, Value: 40000})
	visit(repCard, "11111111-1111-1111-1111-111111111111", models.SourceQR, 1, repository.CardEvent{Type: models.EventVCard})
	visit(repCard, "22222222-2222-2222-2222-222222222222", models.SourceQR, 1, view)
	// An NFC visit from someone else that only looked.
	visit(repCard, "33333333-3333-3333-3333-333333333333", models.SourceNFC, 2, view)
	// Activity on a card the lead can't see.
	visit(outsiderCard, "44444444-4444-4444-4444-444444444444", models.SourceLink, 3, view)
	require.NoError(t, repo.CreateLead(ctx, &models.Lead{ProfileID: repCard.ID, Name: "L", Email: "l@example.com", Source: models.SourceQR}))

	f := repository.AnalyticsFilter{From: time.Now().Add(-time.Hour), To: time.Now().Add(time.Hour), TZ: "UTC"}
	admin := repository.Scope{OrgID: owner.OrgID, UserID: owner.ID, Admin: true}
	leadScope := repository.Scope{OrgID: owner.OrgID, UserID: lead.ID}
	outsiderScope := repository.Scope{OrgID: owner.OrgID, UserID: outsider.ID}

	t.Run("summary", func(t *testing.T) {
		sum, err := repo.AnalyticsSummary(ctx, admin, f)
		require.NoError(t, err)
		c := sum.Current
		assert.EqualValues(t, 4, c.Views)
		assert.EqualValues(t, 3, c.UniqueVisitors)
		assert.EqualValues(t, 2, c.QRViews)
		assert.EqualValues(t, 1, c.NFCViews)
		assert.EqualValues(t, 1, c.LinkViews)
		assert.EqualValues(t, 1, c.Saves)
		assert.EqualValues(t, 2, c.DocOpens)
		assert.EqualValues(t, 1, c.Leads)
		assert.EqualValues(t, 1, c.EngagedSessions)
		assert.EqualValues(t, 1, c.ActionSessions)
		assert.EqualValues(t, 1, c.RepeatVisitors, "visitor 1 came twice")
		assert.EqualValues(t, 40000, c.MedianTimeMs)
		assert.EqualValues(t, 4, sum.Devices["mobile"])
		assert.EqualValues(t, 1, sum.LeadSources[models.SourceQR])
		assert.EqualValues(t, 1, sum.ScrollDepths[3], "one visit reached 75%")
		assert.Zero(t, sum.Previous.Views)
	})

	t.Run("scoping", func(t *testing.T) {
		sum, err := repo.AnalyticsSummary(ctx, leadScope, f)
		require.NoError(t, err)
		assert.EqualValues(t, 3, sum.Current.Views, "the lead sees their team's cards only")

		sum, err = repo.AnalyticsSummary(ctx, outsiderScope, f)
		require.NoError(t, err)
		assert.EqualValues(t, 1, sum.Current.Views, "a member sees their own card only")
		assert.Zero(t, sum.Current.Leads)

		cards, err := repo.AnalyticsCards(ctx, outsiderScope, f)
		require.NoError(t, err)
		require.Len(t, cards, 1)
		assert.Equal(t, outsiderCard.ID, cards[0].ProfileID)

		teamOnly := f
		teamOnly.TeamID = team.ID
		sum, err = repo.AnalyticsSummary(ctx, admin, teamOnly)
		require.NoError(t, err)
		assert.EqualValues(t, 3, sum.Current.Views)
	})

	t.Run("timeseries, content, cards", func(t *testing.T) {
		points, err := repo.AnalyticsTimeseries(ctx, admin, f)
		require.NoError(t, err)
		var views, leads int64
		for _, p := range points {
			views += p.Views
			leads += p.Leads
		}
		assert.EqualValues(t, 4, views)
		assert.EqualValues(t, 1, leads)

		content, err := repo.AnalyticsContent(ctx, admin, f)
		require.NoError(t, err)
		require.Len(t, content, 1)
		assert.Equal(t, "Brochure", content[0].Label)
		assert.EqualValues(t, 2, content[0].Count)
		assert.EqualValues(t, 1, content[0].Unique)

		cards, err := repo.AnalyticsCards(ctx, admin, f)
		require.NoError(t, err)
		require.Len(t, cards, 2)
		assert.Equal(t, repCard.ID, cards[0].ProfileID, "busiest card first")
		assert.EqualValues(t, 3, cards[0].Views)
		assert.EqualValues(t, 1, cards[0].Leads)
		assert.NotNil(t, cards[0].LastViewedAt)
	})

	t.Run("teams and members", func(t *testing.T) {
		teams, err := repo.AnalyticsTeams(ctx, owner.OrgID, 0, 0, f.From, f.To)
		require.NoError(t, err)
		require.Len(t, teams, 1)
		assert.EqualValues(t, 2, teams[0].Members)
		assert.EqualValues(t, 1, teams[0].Cards)
		assert.EqualValues(t, 1, teams[0].ActiveCards)
		assert.EqualValues(t, 3, teams[0].Views)
		assert.EqualValues(t, 1, teams[0].Engaged)
		assert.EqualValues(t, 1, teams[0].Leads)

		teams, err = repo.AnalyticsTeams(ctx, owner.OrgID, rep.ID, 0, f.From, f.To)
		require.NoError(t, err)
		assert.Empty(t, teams, "only teams the user leads")

		members, err := repo.AnalyticsMembers(ctx, leadScope, team.ID, f.From, f.To)
		require.NoError(t, err)
		require.Len(t, members, 2)
		assert.Equal(t, rep.ID, members[0].UserID)
		assert.EqualValues(t, 3, members[0].Views)
	})

	t.Run("activity and retention", func(t *testing.T) {
		items, err := repo.AnalyticsActivity(ctx, admin, f, 50)
		require.NoError(t, err)
		assert.Len(t, items, 7, "views, the save and the brochure opens")

		n, err := repo.PurgeAnalytics(ctx, time.Now().Add(time.Hour))
		require.NoError(t, err)
		assert.EqualValues(t, 9, n)
	})
}
