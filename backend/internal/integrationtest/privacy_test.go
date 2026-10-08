//go:build integration

package integrationtest_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/cards"
	"github.com/faiz-gh/fronko/backend/internal/feedback"
	"github.com/faiz-gh/fronko/backend/internal/files"
	"github.com/faiz-gh/fronko/backend/internal/leads"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/users"
)

// TestPrivacyIntegration covers account deletion, ownership transfer, the
// personal data export, lead erasure and the retention purges.
func TestPrivacyIntegration(t *testing.T) {
	dbUrl := os.Getenv("TEST_DATABASE_URL")
	if dbUrl == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration tests")
	}
	ctx := context.Background()
	pool, err := database.NewPool(ctx, dbUrl)
	require.NoError(t, err)
	defer pool.Close()
	repo := newStores(pool)

	_, err = pool.Exec(ctx, "TRUNCATE TABLE organizations, users, profiles, leads, user_storage, files, teams, team_members, email_codes, feedback, admin_audit_log RESTART IDENTITY CASCADE")
	require.NoError(t, err)

	email := func(s string) *string { return &s }
	newOwner := func(t *testing.T, username string) *users.User {
		t.Helper()
		u := &users.User{Username: username, Email: email(username + "@example.com"), PasswordHash: "hash"}
		require.NoError(t, repo.CreateOrgWithOwner(ctx, username+" org", u))
		require.NoError(t, repo.MarkEmailVerified(ctx, u.ID))
		return u
	}
	newMember := func(t *testing.T, owner *users.User, username, role string) *users.User {
		t.Helper()
		u := &users.User{OrgID: owner.OrgID, Role: role, Username: username, Email: email(username + "@example.com"),
			PasswordHash: "hash", CreatedBy: &owner.ID}
		require.NoError(t, repo.CreateUser(ctx, u))
		require.NoError(t, repo.MarkEmailVerified(ctx, u.ID))
		return u
	}
	newCard := func(t *testing.T, u *users.User, slug string) *cards.Profile {
		t.Helper()
		p := &cards.Profile{OrgID: u.OrgID, UserID: u.ID, AssignedUserID: &u.ID, Slug: slug, Data: json.RawMessage(`{"name":"Card"}`)}
		require.NoError(t, repo.CreateProfile(ctx, p))
		return p
	}
	newLead := func(t *testing.T, p *cards.Profile, addr string) *leads.Lead {
		t.Helper()
		l := &leads.Lead{ProfileID: p.ID, Name: "Visitor", Email: addr}
		require.NoError(t, repo.CreateLead(ctx, l))
		return l
	}
	newFeedback := func(t *testing.T, u *users.User) {
		t.Helper()
		require.NoError(t, repo.CreateFeedback(ctx, &feedback.Feedback{
			OrgID: &u.OrgID, UserID: &u.ID, SenderEmail: *u.Email, OrgName: u.Username + " org",
			Category: feedback.FeedbackIdea, Message: "hello",
		}))
	}
	count := func(t *testing.T, sql string, args ...any) int {
		t.Helper()
		var n int
		require.NoError(t, pool.QueryRow(ctx, sql, args...).Scan(&n))
		return n
	}

	t.Run("Member deletes themselves", func(t *testing.T) {
		owner := newOwner(t, "del-owner")
		rep := newMember(t, owner, "del-rep", auth.RoleMember)
		card := newCard(t, rep, "rep-card")
		newLead(t, card, "lead@example.com")
		newFeedback(t, rep)

		sum, err := repo.GetDeletionSummary(ctx, rep.ID, rep.OrgID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), sum.CardsHeld)
		assert.Equal(t, int64(1), sum.Leads)
		assert.Equal(t, int64(2), sum.OrgMembers)

		require.NoError(t, repo.DeleteOrgUser(ctx, rep.ID, rep.OrgID))
		_, err = repo.GetUserByID(ctx, rep.ID)
		assert.ErrorIs(t, err, database.ErrNotFound)
		// The card passes to the owner and its lead stays with the organisation.
		assert.Equal(t, 1, count(t, `SELECT COUNT(*) FROM profiles WHERE profile_id = $1 AND user_id = $2`, card.ID, owner.ID))
		assert.Equal(t, 1, count(t, `SELECT COUNT(*) FROM leads WHERE profile_id = $1`, card.ID))
		// Feedback keeps its message but loses the address.
		assert.Equal(t, 1, count(t, `SELECT COUNT(*) FROM feedback WHERE message = 'hello' AND sender_email IS NULL`))
	})

	t.Run("Ownership transfer moves the bucket keys", func(t *testing.T) {
		owner := newOwner(t, "xfer-owner")
		admin := newMember(t, owner, "xfer-admin", auth.RoleAdmin)
		_, err := pool.Exec(ctx, `
			INSERT INTO user_storage (user_id, provider, endpoint, region, bucket, path_style,
			                          access_key_id_enc, secret_access_key_enc, access_key_hint)
			VALUES ($1, 's3', 'https://s3.example.com', 'us-east-1', 'b', false, '\x01', '\x02', '…abcd')`, owner.ID)
		require.NoError(t, err)

		cands, err := repo.ListTransferCandidates(ctx, owner.OrgID, owner.ID)
		require.NoError(t, err)
		require.Len(t, cands, 1)
		assert.Equal(t, admin.ID, cands[0].ID)

		failing := func([]byte, []byte) ([]byte, []byte, error) { return nil, nil, errors.New("no key") }
		require.Error(t, repo.TransferOwnership(ctx, owner.OrgID, owner.ID, admin.ID, failing))
		still, err := repo.GetUserByID(ctx, owner.ID)
		require.NoError(t, err)
		assert.Equal(t, auth.RoleOwner, still.Role, "a failed transfer changes nothing")

		other := newOwner(t, "xfer-stranger")
		ok := func(id, secret []byte) ([]byte, []byte, error) { return []byte{9}, []byte{8}, nil }
		assert.ErrorIs(t, repo.TransferOwnership(ctx, owner.OrgID, owner.ID, other.ID, ok), database.ErrNotFound)

		require.NoError(t, repo.TransferOwnership(ctx, owner.OrgID, owner.ID, admin.ID, ok))
		newOwnerUser, err := repo.GetOrgOwner(ctx, owner.OrgID)
		require.NoError(t, err)
		assert.Equal(t, admin.ID, newOwnerUser.ID)
		oldOwner, err := repo.GetUserByID(ctx, owner.ID)
		require.NoError(t, err)
		assert.Equal(t, auth.RoleAdmin, oldOwner.Role)
		assert.Greater(t, oldOwner.SessionVersion, still.SessionVersion, "signed out everywhere")
		settings, err := repo.GetOrgStorageSettings(ctx, owner.OrgID)
		require.NoError(t, err)
		assert.Equal(t, admin.ID, settings.UserID)
		assert.Equal(t, []byte{9}, settings.AccessKeyIDEnc)

		// The former owner can now be deleted like anyone else.
		require.NoError(t, repo.DeleteOrgUser(ctx, owner.ID, owner.OrgID))
	})

	t.Run("Deleting an organisation removes everything in it", func(t *testing.T) {
		owner := newOwner(t, "gone-owner")
		rep := newMember(t, owner, "gone-rep", auth.RoleMember)
		card := newCard(t, rep, "gone-card")
		newLead(t, card, "gone-lead@example.com")
		newFeedback(t, owner)
		require.NoError(t, repo.CreateFile(ctx, &files.File{
			PublicID: "gone-file", OrgID: owner.OrgID, UserID: rep.ID, Area: files.AreaPersonal, Bucket: "b",
			ObjectKey: "k/gone", Kind: "pdf", ContentType: "application/pdf", SizeBytes: 1, OriginalName: "x.pdf",
		}))
		survivor := newOwner(t, "keep-owner")
		newCard(t, survivor, "keep-card")

		require.NoError(t, repo.DeleteOrganization(ctx, owner.OrgID))
		assert.Zero(t, count(t, `SELECT COUNT(*) FROM users WHERE org_id = $1`, owner.OrgID))
		assert.Zero(t, count(t, `SELECT COUNT(*) FROM profiles WHERE org_id = $1`, owner.OrgID))
		assert.Zero(t, count(t, `SELECT COUNT(*) FROM leads WHERE email = 'gone-lead@example.com'`))
		assert.Zero(t, count(t, `SELECT COUNT(*) FROM files WHERE public_id = 'gone-file'`))
		assert.Zero(t, count(t, `SELECT COUNT(*) FROM feedback WHERE sender_email = 'gone-owner@example.com' OR org_name = 'gone-owner org'`))
		assert.Equal(t, 1, count(t, `SELECT COUNT(*) FROM profiles WHERE org_id = $1`, survivor.OrgID))
		assert.ErrorIs(t, repo.DeleteOrganization(ctx, owner.OrgID), database.ErrNotFound)
	})

	t.Run("Export has everything about the person", func(t *testing.T) {
		owner := newOwner(t, "exp-owner")
		card := newCard(t, owner, "exp-card")
		newLead(t, card, "exp-lead@example.com")
		newFeedback(t, owner)

		raw, err := repo.ExportUserData(ctx, owner.ID)
		require.NoError(t, err)
		var doc struct {
			Account      map[string]any   `json:"account"`
			Organisation map[string]any   `json:"organisation"`
			Cards        []map[string]any `json:"cards"`
			Leads        []map[string]any `json:"leads"`
			Files        []any            `json:"files"`
			Teams        []any            `json:"teams"`
			Integrations []any            `json:"integrations"`
			Feedback     []map[string]any `json:"feedback"`
		}
		require.NoError(t, json.Unmarshal(raw, &doc))
		assert.Equal(t, "exp-owner@example.com", doc.Account["email"])
		assert.Equal(t, "exp-owner org", doc.Organisation["name"])
		require.Len(t, doc.Cards, 1)
		require.Len(t, doc.Leads, 1)
		assert.Equal(t, "exp-lead@example.com", doc.Leads[0]["email"])
		assert.Len(t, doc.Feedback, 1)
		assert.NotNil(t, doc.Files)
		assert.NotContains(t, string(raw), "hash", "never the password hash")
	})

	t.Run("Lead erasure and retention", func(t *testing.T) {
		owner := newOwner(t, "ret-owner")
		card := newCard(t, owner, "ret-card")
		a := newLead(t, card, "a@example.com")
		b := newLead(t, card, "b@example.com")
		old := newLead(t, card, "old@example.com")
		other := newOwner(t, "ret-other")
		foreign := newLead(t, newCard(t, other, "ret-foreign"), "f@example.com")

		n, err := repo.DeleteLeads(ctx, []int64{a.ID, foreign.ID}, owner.OrgID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), n, "another organisation's lead is untouched")
		assert.Equal(t, 1, count(t, `SELECT COUNT(*) FROM leads WHERE lead_id = $1`, foreign.ID))

		_, err = pool.Exec(ctx, `UPDATE leads SET created_at = now() - interval '100 days' WHERE lead_id = ANY($1)`,
			[]int64{old.ID, foreign.ID})
		require.NoError(t, err)
		n, err = repo.PurgeExpired(ctx)
		require.NoError(t, err)
		assert.Zero(t, n, "no retention set: leads are kept")

		require.NoError(t, repo.SetOrgPrivacy(ctx, owner.OrgID, ptr("https://example.com/privacy"), ptr(90)))
		n, err = repo.PurgeExpired(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(1), n)
		assert.Zero(t, count(t, `SELECT COUNT(*) FROM leads WHERE lead_id = $1`, old.ID))
		assert.Equal(t, 1, count(t, `SELECT COUNT(*) FROM leads WHERE lead_id = $1`, b.ID))
		assert.Equal(t, 1, count(t, `SELECT COUNT(*) FROM leads WHERE lead_id = $1`, foreign.ID))

		org, err := repo.GetOrganization(ctx, owner.OrgID)
		require.NoError(t, err)
		assert.Equal(t, "https://example.com/privacy", *org.PrivacyURL)
		assert.Equal(t, 90, *org.LeadRetentionDays)
		_, err = pool.Exec(ctx, `UPDATE organizations SET lead_retention_days = 5 WHERE org_id = $1`, owner.OrgID)
		assert.Error(t, err, "retention under 30 days is refused")
	})

	t.Run("Expired codes and old feedback are purged", func(t *testing.T) {
		u := newOwner(t, "purge-owner")
		require.NoError(t, repo.UpsertEmailCode(ctx, &users.EmailCode{
			UserID: u.ID, Purpose: auth.PurposeDeleteAccount, CodeHash: "h", ExpiresAt: time.Now().Add(-time.Minute),
		}))
		require.NoError(t, repo.UpsertEmailCode(ctx, &users.EmailCode{
			UserID: u.ID, Purpose: auth.PurposeVerifyEmail, CodeHash: "h", ExpiresAt: time.Now().Add(time.Hour),
		}))
		n, err := repo.PurgeExpiredCodes(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(1), n)

		newFeedback(t, u)
		_, err = pool.Exec(ctx, `UPDATE feedback SET created_at = now() - interval '3 years' WHERE user_id = $1`, u.ID)
		require.NoError(t, err)
		fb, _, err := repo.PurgeOld(ctx, time.Now().Add(-2*365*24*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, int64(1), fb)
	})
}
