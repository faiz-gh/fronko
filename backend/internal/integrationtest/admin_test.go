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

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/branding"
	"github.com/faiz-gh/fronko/backend/internal/cards"
	"github.com/faiz-gh/fronko/backend/internal/feedback"
	"github.com/faiz-gh/fronko/backend/internal/files"
	"github.com/faiz-gh/fronko/backend/internal/leads"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platformadmin"
	"github.com/faiz-gh/fronko/backend/internal/teams"
	"github.com/faiz-gh/fronko/backend/internal/users"
)

func TestAdminIntegration(t *testing.T) {
	dbUrl := os.Getenv("TEST_DATABASE_URL")
	if dbUrl == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration tests")
	}

	ctx := context.Background()
	pool, err := database.NewPool(ctx, dbUrl)
	require.NoError(t, err)
	defer pool.Close()
	repo := newStores(pool)

	_, err = pool.Exec(ctx, "TRUNCATE TABLE organizations, users, profiles, leads, user_storage, files, file_grants, teams, team_members, file_team_grants, file_refs, email_codes, platform_admins, feedback, feedback_replies, org_usage_snapshots, platform_usage_snapshots, admin_audit_log RESTART IDENTITY CASCADE")
	require.NoError(t, err)

	email := func(s string) *string { return &s }
	newOwner := func(t *testing.T, username, orgName string) *users.User {
		t.Helper()
		u := &users.User{Username: username, Email: email(username + "@example.com"), PasswordHash: "hash"}
		require.NoError(t, repo.CreateOrgWithOwner(ctx, orgName, u))
		return u
	}
	newMember := func(t *testing.T, owner *users.User, username, role string) *users.User {
		t.Helper()
		u := &users.User{OrgID: owner.OrgID, Role: role, Username: username, Email: email(username + "@example.com"), PasswordHash: "hash", CreatedBy: &owner.ID}
		require.NoError(t, repo.CreateUser(ctx, u))
		return u
	}
	newCard := func(t *testing.T, u *users.User, slug string) *cards.Profile {
		t.Helper()
		p := &cards.Profile{OrgID: u.OrgID, UserID: u.ID, Slug: slug, Data: json.RawMessage(`{"secret":"card contents"}`)}
		require.NoError(t, repo.CreateProfile(ctx, p))
		return p
	}
	newLead := func(t *testing.T, p *cards.Profile) {
		t.Helper()
		require.NoError(t, repo.CreateLead(ctx, &leads.Lead{ProfileID: p.ID, Name: "Lead", Email: "lead@example.com"}))
	}
	newFile := func(t *testing.T, u *users.User, area, id string, size int64) {
		t.Helper()
		require.NoError(t, repo.CreateFile(ctx, &files.File{
			PublicID: id, OrgID: u.OrgID, UserID: u.ID, Area: area, Bucket: "b", ObjectKey: "k/" + id,
			Kind: "pdf", ContentType: "application/pdf", SizeBytes: size, OriginalName: id + ".pdf",
		}))
	}

	// Acme: owner, an admin, a suspended member; storage; 2 cards, 3 leads, 2 files.
	acme := newOwner(t, "acme-owner", "Acme")
	newMember(t, acme, "acme-admin", auth.RoleAdmin)
	rep := newMember(t, acme, "acme-rep", auth.RoleMember)
	require.NoError(t, repo.SetUserSuspended(ctx, rep.ID, true))
	require.NoError(t, repo.UpsertStorageSettings(ctx, &files.StorageSettings{
		UserID: acme.ID, Provider: "r2", Endpoint: "https://x.example", Region: "auto", Bucket: "acme",
		AccessKeyIDEnc: []byte{1}, SecretAccessKeyEnc: []byte{2}, AccessKeyHint: "ABCD",
	}))
	card1 := newCard(t, acme, "acme-1")
	card2 := newCard(t, acme, "acme-2")
	newLead(t, card1)
	newLead(t, card1)
	newLead(t, card2)
	newFile(t, acme, files.AreaOrg, "acme-f1", 1000)
	newFile(t, rep, files.AreaPersonal, "acme-f2", 234)
	// A team and a logo: counted, never named.
	require.NoError(t, repo.CreateTeam(ctx, &teams.Team{OrgID: acme.OrgID, Name: "Secret Sales"}))
	logo := "acme-f1"
	require.NoError(t, repo.UpdateOrgBranding(ctx, acme.OrgID, &branding.OrgBranding{
		LogoFile: &logo, LogoPolicy: branding.LogoRequired, Signature: branding.OrgSignature{LockedTemplate: "classic"},
	}))

	// Empty: just the owner.
	empty := newOwner(t, "empty-owner", "Empty Co")

	t.Run("Org Usage", func(t *testing.T) {
		got, err := repo.GetOrgUsage(ctx, acme.OrgID)
		require.NoError(t, err)
		assert.Equal(t, "Acme", got.Name)
		require.NotNil(t, got.OwnerEmail)
		assert.Equal(t, "acme-owner@example.com", *got.OwnerEmail)
		assert.EqualValues(t, 3, got.UserCount)
		assert.EqualValues(t, 1, got.AdminCount)
		assert.EqualValues(t, 1, got.MemberCount)
		assert.EqualValues(t, 1, got.SuspendedUserCount)
		assert.EqualValues(t, 2, got.CardCount)
		assert.EqualValues(t, 3, got.LeadCount)
		assert.EqualValues(t, 2, got.FileCount)
		assert.EqualValues(t, 1234, got.StorageUsedBytes)
		assert.True(t, got.StorageConnected)
		assert.False(t, got.StorageVerified)
		require.NotNil(t, got.StorageProvider)
		assert.Equal(t, "r2", *got.StorageProvider)
		assert.Nil(t, got.SuspendedAt)
		assert.EqualValues(t, 1, got.TeamCount)
		assert.True(t, got.LogoSet)
		assert.Equal(t, branding.LogoRequired, got.LogoPolicy)
		assert.True(t, got.SignatureLocked)
		assert.Equal(t, map[string]int64{"other": 2}, got.FilesByPurpose)

		none, err := repo.GetOrgUsage(ctx, empty.OrgID)
		require.NoError(t, err)
		assert.EqualValues(t, 1, none.UserCount)
		assert.Zero(t, none.CardCount)
		assert.Zero(t, none.LeadCount)
		assert.Zero(t, none.FileCount)
		assert.Zero(t, none.StorageUsedBytes)
		assert.False(t, none.StorageConnected)
		assert.Nil(t, none.StorageProvider)
		assert.Zero(t, none.TeamCount)
		assert.False(t, none.LogoSet)
		assert.False(t, none.SignatureLocked)
		assert.Empty(t, none.FilesByPurpose)

		_, err = repo.GetOrgUsage(ctx, 999999)
		assert.ErrorIs(t, err, database.ErrNotFound)

		// The JSON the panel receives has no card data, leads or member details.
		raw, err := json.Marshal(got)
		require.NoError(t, err)
		for _, leak := range []string{"card contents", "lead@example.com", "acme-rep", "acme-admin", "acme-f1", "Secret Sales"} {
			assert.NotContains(t, string(raw), leak)
		}
	})

	t.Run("List Org Usage", func(t *testing.T) {
		all, total, err := repo.ListOrgUsage(ctx, platformadmin.OrgUsageFilter{Sort: "cards", Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 2, total)
		require.Len(t, all, 2)
		assert.Equal(t, acme.OrgID, all[0].ID, "sorted by cards")

		found, total, err := repo.ListOrgUsage(ctx, platformadmin.OrgUsageFilter{Search: "EMPTY-owner@", Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 1, total, "search matches the owner email, case-insensitively")
		assert.Equal(t, empty.OrgID, found[0].ID)

		_, total, err = repo.ListOrgUsage(ctx, platformadmin.OrgUsageFilter{Search: "%", Limit: 10})
		require.NoError(t, err)
		assert.Zero(t, total, "wildcards are literal")

		_, total, err = repo.ListOrgUsage(ctx, platformadmin.OrgUsageFilter{Status: "suspended", Limit: 10})
		require.NoError(t, err)
		assert.Zero(t, total)

		page2, total, err := repo.ListOrgUsage(ctx, platformadmin.OrgUsageFilter{Sort: "name", Limit: 1, Offset: 1})
		require.NoError(t, err)
		assert.EqualValues(t, 2, total)
		require.Len(t, page2, 1)
		assert.Equal(t, "Empty Co", page2[0].Name)

		_, _, err = repo.ListOrgUsage(ctx, platformadmin.OrgUsageFilter{Sort: "name; DROP TABLE users", Limit: 1})
		assert.Error(t, err, "unknown sort keys are refused")

		s, err := repo.PlatformSummary(ctx)
		require.NoError(t, err)
		assert.EqualValues(t, 2, s.OrgCount)
		assert.EqualValues(t, 4, s.UserCount)
		assert.EqualValues(t, 2, s.CardCount)
		assert.EqualValues(t, 3, s.LeadCount)
		assert.EqualValues(t, 1, s.OrgsWithStorage)
		assert.EqualValues(t, 1234, s.StorageUsedBytes)
		assert.EqualValues(t, 2, s.NewOrgs30d)
		assert.EqualValues(t, 1, s.TeamCount)
		assert.EqualValues(t, 1, s.OrgsWithTeams)
		assert.EqualValues(t, 1, s.OrgsWithLogo)

		byTeams, _, err := repo.ListOrgUsage(ctx, platformadmin.OrgUsageFilter{Sort: "teams", Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, acme.OrgID, byTeams[0].ID, "sorted by teams")
	})

	t.Run("Snapshots", func(t *testing.T) {
		today := time.Now()
		require.NoError(t, repo.TakeUsageSnapshot(ctx, today))
		newLead(t, card2)
		require.NoError(t, repo.TakeUsageSnapshot(ctx, today), "re-running the same day overwrites it")

		points, err := repo.OrgTrend(ctx, acme.OrgID, 30)
		require.NoError(t, err)
		require.Len(t, points, 1)
		assert.Equal(t, today.UTC().Format(time.DateOnly), points[0].Date)
		assert.EqualValues(t, 4, points[0].LeadCount)
		assert.EqualValues(t, 1234, points[0].StorageUsedBytes)
		assert.EqualValues(t, 1, points[0].TeamCount)

		// An older day shows up in order; one outside the window doesn't.
		require.NoError(t, repo.TakeUsageSnapshot(ctx, today.AddDate(0, 0, -3)))
		require.NoError(t, repo.TakeUsageSnapshot(ctx, today.AddDate(0, 0, -40)))
		platform, err := repo.PlatformTrend(ctx, 30)
		require.NoError(t, err)
		require.Len(t, platform, 2)
		assert.Less(t, platform[0].Date, platform[1].Date)
		assert.EqualValues(t, 2, platform[1].OrgCount)
		assert.EqualValues(t, 4, platform[1].LeadCount)
		assert.EqualValues(t, 2, platform[1].NewOrgs, "both orgs were created today")
		assert.EqualValues(t, 1, platform[1].TeamCount)
		assert.EqualValues(t, 1, platform[1].OrgsWithTeams)
		assert.EqualValues(t, 1, platform[1].OrgsWithLogo)
	})

	t.Run("Suspend Org", func(t *testing.T) {
		before, err := repo.GetSessionState(ctx, acme.ID)
		require.NoError(t, err)
		assert.False(t, before.OrgSuspended)

		require.NoError(t, repo.SuspendOrg(ctx, acme.OrgID, "Unpaid invoice"))
		state, err := repo.GetSessionState(ctx, acme.ID)
		require.NoError(t, err)
		assert.True(t, state.OrgSuspended)
		assert.Equal(t, "Unpaid invoice", state.OrgSuspendedReason)
		assert.Equal(t, before.Version+1, state.Version, "sessions are revoked")
		repState, err := repo.GetSessionState(ctx, rep.ID)
		require.NoError(t, err)
		assert.True(t, repState.OrgSuspended, "every member is affected")

		card, err := repo.GetProfileByPath(ctx, "acme", "acme-1")
		require.NoError(t, err)
		assert.True(t, card.OrgSuspended)
		err = repo.CreateLead(ctx, &leads.Lead{ProfileID: card1.ID, Name: "x", Email: "x@example.com"})
		assert.ErrorIs(t, err, database.ErrNotFound, "no leads while suspended")
		suspended, err := repo.IsOrgSuspended(ctx, acme.OrgID)
		require.NoError(t, err)
		assert.True(t, suspended)

		other, err := repo.GetSessionState(ctx, empty.ID)
		require.NoError(t, err)
		assert.False(t, other.OrgSuspended, "other organisations are untouched")

		_, total, err := repo.ListOrgUsage(ctx, platformadmin.OrgUsageFilter{Status: "suspended", Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)

		require.NoError(t, repo.ReinstateOrg(ctx, acme.OrgID))
		state, err = repo.GetSessionState(ctx, acme.ID)
		require.NoError(t, err)
		assert.False(t, state.OrgSuspended)
		card, err = repo.GetProfileByPath(ctx, "acme", "acme-1")
		require.NoError(t, err)
		assert.False(t, card.OrgSuspended)
		newLead(t, card1)

		assert.ErrorIs(t, repo.SuspendOrg(ctx, 999999, "x"), database.ErrNotFound)
	})

	t.Run("Admins And Audit", func(t *testing.T) {
		a := &platformadmin.PlatformAdmin{Email: "Boss@Example.com", PasswordHash: "h"}
		require.NoError(t, repo.CreatePlatformAdmin(ctx, a))
		assert.Equal(t, 1, a.SessionVersion)
		err := repo.CreatePlatformAdmin(ctx, &platformadmin.PlatformAdmin{Email: "boss@example.com", PasswordHash: "h"})
		assert.ErrorIs(t, err, database.ErrConflict, "emails are unique regardless of case")

		got, err := repo.GetPlatformAdminByEmail(ctx, "boss@EXAMPLE.com")
		require.NoError(t, err)
		assert.Equal(t, a.ID, got.ID)

		require.NoError(t, repo.SetPlatformAdminPassword(ctx, "boss@example.com", "h2"))
		got, err = repo.GetPlatformAdmin(ctx, a.ID)
		require.NoError(t, err)
		assert.Equal(t, "h2", got.PasswordHash)
		assert.Equal(t, 2, got.SessionVersion, "a new password revokes sessions")

		require.NoError(t, repo.AddAudit(ctx, a, platformadmin.AuditOrgSuspend, platformadmin.AuditTarget{Type: "org", ID: acme.OrgID},
			map[string]any{"reason": "x"}))
		require.NoError(t, repo.AddAudit(ctx, a, platformadmin.AuditAdminLogin, platformadmin.AuditTarget{}, nil))
		entries, total, err := repo.ListAudit(ctx, 10, 0)
		require.NoError(t, err)
		assert.EqualValues(t, 2, total)
		require.Len(t, entries, 2)
		assert.Equal(t, platformadmin.AuditAdminLogin, entries[0].Action, "newest first")
		assert.Nil(t, entries[0].TargetType)
		require.NotNil(t, entries[1].TargetID)
		assert.Equal(t, acme.OrgID, *entries[1].TargetID)
		assert.JSONEq(t, `{"reason":"x"}`, string(entries[1].Detail))
	})

	t.Run("Feedback", func(t *testing.T) {
		admin, err := repo.GetPlatformAdminByEmail(ctx, "boss@example.com")
		require.NoError(t, err)
		gone := newOwner(t, "gone-owner", "Gone Ltd")

		rating := int16(4)
		path := "/dashboard/leads"
		f := &feedback.Feedback{
			OrgID: &gone.OrgID, UserID: &gone.ID, SenderEmail: "gone-owner@example.com", OrgName: "Gone Ltd",
			Category: feedback.FeedbackIdea, Rating: &rating, Message: "CSV export please", PagePath: &path,
		}
		require.NoError(t, repo.CreateFeedback(ctx, f))
		assert.Equal(t, feedback.FeedbackNew, f.Status)
		require.NoError(t, repo.CreateFeedback(ctx, &feedback.Feedback{
			OrgID: &acme.OrgID, UserID: &acme.ID, SenderEmail: "acme-owner@example.com", OrgName: "Acme",
			Category: feedback.FeedbackBug, Message: "Broken",
		}))

		reply := &feedback.FeedbackReply{FeedbackID: f.ID, AdminID: &admin.ID, Body: "On the roadmap", EmailSent: true}
		require.NoError(t, repo.CreateFeedbackReply(ctx, reply))

		got, err := repo.GetFeedback(ctx, f.ID)
		require.NoError(t, err)
		assert.Equal(t, feedback.FeedbackRead, got.Status, "replying marks new feedback read")
		assert.EqualValues(t, 1, got.ReplyCount)
		require.Len(t, got.Replies, 1)
		assert.Equal(t, "On the roadmap", got.Replies[0].Body)
		require.NotNil(t, got.Replies[0].AdminEmail)
		assert.Equal(t, "Boss@Example.com", *got.Replies[0].AdminEmail)

		newOnes, total, err := repo.ListFeedback(ctx, feedback.FeedbackNew, 10, 0)
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)
		assert.Equal(t, "Broken", newOnes[0].Message)

		require.NoError(t, repo.SetFeedbackStatus(ctx, f.ID, feedback.FeedbackResolved))
		assert.ErrorIs(t, repo.SetFeedbackStatus(ctx, 999999, feedback.FeedbackResolved), database.ErrNotFound)
		counts, err := repo.CountFeedbackByStatus(ctx)
		require.NoError(t, err)
		assert.Equal(t, map[string]int64{"new": 1, "read": 0, "resolved": 1}, counts)

		// Deleting the organisation keeps its feedback, with the copied names.
		_, err = pool.Exec(ctx, `DELETE FROM organizations WHERE org_id = $1`, gone.OrgID)
		require.NoError(t, err)
		got, err = repo.GetFeedback(ctx, f.ID)
		require.NoError(t, err)
		assert.Nil(t, got.OrgID)
		assert.Equal(t, "Gone Ltd", got.OrgName)
		assert.Equal(t, "gone-owner@example.com", got.SenderEmail)
		assert.Len(t, got.Replies, 1)
	})
}
