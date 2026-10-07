//go:build integration

package integrationtest_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/faiz-gh/fronko/backend/internal/analytics"
	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/cards"
	"github.com/faiz-gh/fronko/backend/internal/leads"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/teams"
	"github.com/faiz-gh/fronko/backend/internal/users"
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
	repo := newStores(pool)

	_, err = pool.Exec(ctx, "TRUNCATE TABLE organizations, users, profiles, leads, teams, team_members, card_events, analytics_salts RESTART IDENTITY CASCADE")
	require.NoError(t, err)

	// An organisation with a team: lead + rep in it, and an outsider who isn't.
	owner := &users.User{Username: "an-owner", PasswordHash: "hash"}
	require.NoError(t, repo.CreateOrgWithOwner(ctx, "Analytics Org", owner))
	newMember := func(name string) *users.User {
		u := &users.User{OrgID: owner.OrgID, Role: auth.RoleMember, Username: name, PasswordHash: "hash", CreatedBy: &owner.ID}
		require.NoError(t, repo.CreateUser(ctx, u))
		return u
	}
	lead, rep, outsider := newMember("an-lead"), newMember("an-rep"), newMember("an-outsider")
	team := &teams.Team{OrgID: owner.OrgID, Name: "Sales"}
	require.NoError(t, repo.CreateTeam(ctx, team))
	require.NoError(t, repo.ReplaceTeamMembers(ctx, owner.OrgID, team.ID, []teams.TeamMembership{
		{UserID: lead.ID, Role: auth.TeamRoleLead}, {UserID: rep.ID, Role: auth.TeamRoleMember},
	}))

	newCard := func(slug string, holder *users.User) *cards.Profile {
		p := &cards.Profile{OrgID: owner.OrgID, UserID: owner.ID, AssignedUserID: &holder.ID, Slug: slug,
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

	visit := func(card *cards.Profile, session, source string, hash byte, events ...analytics.CardEvent) {
		require.NoError(t, repo.RecordCardEvents(ctx, analytics.EventBatch{
			ProfileID: card.ID, SessionID: session, VisitorHash: []byte{hash}, Source: source, Device: "mobile", Events: events,
		}))
	}
	view := analytics.CardEvent{Type: analytics.EventView}
	// Two QR visits by the same visitor, one engaged (scrolled 75%, opened a brochure, saved the contact).
	visit(repCard, "11111111-1111-1111-1111-111111111111", analytics.SourceQR, 1,
		view, analytics.CardEvent{Type: analytics.EventScroll, Value: 75},
		analytics.CardEvent{Type: analytics.EventDocOpen, Target: "doc1", Label: "Brochure"},
		analytics.CardEvent{Type: analytics.EventDocOpen, Target: "doc1", Label: "Brochure"},
		analytics.CardEvent{Type: analytics.EventLeave, Value: 40000})
	visit(repCard, "11111111-1111-1111-1111-111111111111", analytics.SourceQR, 1, analytics.CardEvent{Type: analytics.EventVCard})
	visit(repCard, "22222222-2222-2222-2222-222222222222", analytics.SourceQR, 1, view)
	// An NFC visit from someone else that only looked.
	visit(repCard, "33333333-3333-3333-3333-333333333333", analytics.SourceNFC, 2, view)
	// Activity on a card the lead can't see.
	visit(outsiderCard, "44444444-4444-4444-4444-444444444444", analytics.SourceLink, 3, view)
	require.NoError(t, repo.CreateLead(ctx, &leads.Lead{ProfileID: repCard.ID, Name: "L", Email: "l@example.com", Source: analytics.SourceQR}))

	f := analytics.AnalyticsFilter{From: time.Now().Add(-time.Hour), To: time.Now().Add(time.Hour), TZ: "UTC"}
	admin := auth.Scope{OrgID: owner.OrgID, UserID: owner.ID, Admin: true}
	leadScope := auth.Scope{OrgID: owner.OrgID, UserID: lead.ID}
	outsiderScope := auth.Scope{OrgID: owner.OrgID, UserID: outsider.ID}

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
		assert.EqualValues(t, 1, sum.LeadSources[analytics.SourceQR])
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

		gotCards, err := repo.AnalyticsCards(ctx, outsiderScope, f)
		require.NoError(t, err)
		require.Len(t, gotCards, 1)
		assert.Equal(t, outsiderCard.ID, gotCards[0].ProfileID)

		teamOnly := f
		teamOnly.TeamID = team.ID
		sum, err = repo.AnalyticsSummary(ctx, admin, teamOnly)
		require.NoError(t, err)
		assert.EqualValues(t, 3, sum.Current.Views)
	})

	t.Run("timeseries, content, cards", func(t *testing.T) {
		points, err := repo.AnalyticsTimeseries(ctx, admin, f)
		require.NoError(t, err)
		var views, gotLeads int64
		for _, p := range points {
			views += p.Views
			gotLeads += p.Leads
		}
		assert.EqualValues(t, 4, views)
		assert.EqualValues(t, 1, gotLeads)

		content, err := repo.AnalyticsContent(ctx, admin, f)
		require.NoError(t, err)
		require.Len(t, content, 1)
		assert.Equal(t, "Brochure", content[0].Label)
		assert.EqualValues(t, 2, content[0].Count)
		assert.EqualValues(t, 1, content[0].Unique)

		gotCards, err := repo.AnalyticsCards(ctx, admin, f)
		require.NoError(t, err)
		require.Len(t, gotCards, 2)
		assert.Equal(t, repCard.ID, gotCards[0].ProfileID, "busiest card first")
		assert.EqualValues(t, 3, gotCards[0].Views)
		assert.EqualValues(t, 1, gotCards[0].Leads)
		assert.NotNil(t, gotCards[0].LastViewedAt)
	})

	t.Run("teams and members", func(t *testing.T) {
		gotTeams, err := repo.AnalyticsTeams(ctx, owner.OrgID, 0, 0, f.From, f.To)
		require.NoError(t, err)
		require.Len(t, gotTeams, 1)
		assert.EqualValues(t, 2, gotTeams[0].Members)
		assert.EqualValues(t, 1, gotTeams[0].Cards)
		assert.EqualValues(t, 1, gotTeams[0].ActiveCards)
		assert.EqualValues(t, 3, gotTeams[0].Views)
		assert.EqualValues(t, 1, gotTeams[0].Engaged)
		assert.EqualValues(t, 1, gotTeams[0].Leads)

		gotTeams, err = repo.AnalyticsTeams(ctx, owner.OrgID, rep.ID, 0, f.From, f.To)
		require.NoError(t, err)
		assert.Empty(t, gotTeams, "only teams the user leads")

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
