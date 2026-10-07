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

func TestRepositoryIntegration(t *testing.T) {
	// Deliberately not DATABASE_URL: these tests TRUNCATE every table, so they
	// must never pick up the dev or production database by accident.
	dbUrl := os.Getenv("TEST_DATABASE_URL")
	if dbUrl == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration tests")
	}

	ctx := context.Background()
	pool, err := database.NewPool(ctx, dbUrl)
	require.NoError(t, err)
	defer pool.Close()

	repo := repository.New(pool)

	// Clean up tables before testing
	_, err = pool.Exec(ctx, "TRUNCATE TABLE organizations, users, profiles, leads, user_storage, files, file_grants, teams, team_members, file_team_grants, file_refs, email_codes, platform_admins, feedback, feedback_replies, org_usage_snapshots, platform_usage_snapshots, admin_audit_log RESTART IDENTITY CASCADE")
	require.NoError(t, err)

	// newOwner registers an organisation and its owner.
	newOwner := func(t *testing.T, username string) *models.User {
		t.Helper()
		u := &models.User{Username: username, PasswordHash: "hash"}
		require.NoError(t, repo.CreateOrgWithOwner(ctx, username+" org", u))
		return u
	}
	// newMember adds a user with the given role to owner's organisation.
	newMember := func(t *testing.T, owner *models.User, username, role string) *models.User {
		t.Helper()
		u := &models.User{OrgID: owner.OrgID, Role: role, Username: username, PasswordHash: "hash", CreatedBy: &owner.ID}
		require.NoError(t, repo.CreateUser(ctx, u))
		return u
	}
	adminScope := func(u *models.User) repository.Scope {
		return repository.Scope{OrgID: u.OrgID, UserID: u.ID, Admin: true}
	}
	memberScope := func(u *models.User) repository.Scope {
		return repository.Scope{OrgID: u.OrgID, UserID: u.ID}
	}

	t.Run("User Flow", func(t *testing.T) {
		user := &models.User{
			Username:     "testuser",
			PasswordHash: "hashed_password",
		}

		err := repo.CreateOrgWithOwner(ctx, "Test Org", user)
		assert.NoError(t, err)
		assert.NotZero(t, user.ID)
		assert.NotZero(t, user.OrgID)
		assert.Equal(t, models.RoleOwner, user.Role)
		assert.NotZero(t, user.CreatedAt)

		org, err := repo.GetOrganization(ctx, user.OrgID)
		require.NoError(t, err)
		assert.Equal(t, "Test Org", org.Name)

		// Get By Username
		fetched, err := repo.GetUserByUsername(ctx, "testuser")
		assert.NoError(t, err)
		assert.Equal(t, user.ID, fetched.ID)

		// Lookups are case-insensitive
		fetchedCI, err := repo.GetUserByUsername(ctx, "TestUser")
		assert.NoError(t, err)
		assert.Equal(t, user.ID, fetchedCI.ID)

		// Get By ID
		fetchedByID, err := repo.GetUserByID(ctx, user.ID)
		assert.NoError(t, err)
		assert.Equal(t, user.ID, fetchedByID.ID)

		// Duplicate usernames (in any case) conflict
		dup := &models.User{Username: "TESTUSER", PasswordHash: "x"}
		assert.ErrorIs(t, repo.CreateOrgWithOwner(ctx, "dup", dup), repository.ErrConflict)

		// One owner per organisation
		second := &models.User{OrgID: user.OrgID, Role: models.RoleOwner, Username: "second-owner", PasswordHash: "x"}
		assert.ErrorIs(t, repo.CreateUser(ctx, second), repository.ErrConflict)

		_, err = repo.GetUserByUsername(ctx, "nobody")
		assert.ErrorIs(t, err, repository.ErrNotFound)
	})

	t.Run("Email And Codes", func(t *testing.T) {
		email := "Mailer@Example.com"
		user := &models.User{Username: "mailer", Email: &email, PasswordHash: "hash"}
		require.NoError(t, repo.CreateOrgWithOwner(ctx, "mail org", user))
		assert.Zero(t, user.SessionVersion)

		// Email lookups and uniqueness are case-insensitive
		byEmail, err := repo.GetUserByEmail(ctx, "mailer@example.com")
		require.NoError(t, err)
		assert.Equal(t, user.ID, byEmail.ID)
		assert.Nil(t, byEmail.EmailVerifiedAt)

		dupEmail := "MAILER@example.com"
		err = repo.CreateOrgWithOwner(ctx, "x", &models.User{Username: "mailer2", Email: &dupEmail, PasswordHash: "x"})
		assert.ErrorIs(t, err, repository.ErrConflict)
		assert.True(t, repository.IsEmailConflict(err))

		dupName := "unique@example.com"
		err = repo.CreateOrgWithOwner(ctx, "x", &models.User{Username: "MAILER", Email: &dupName, PasswordHash: "x"})
		assert.ErrorIs(t, err, repository.ErrConflict)
		assert.False(t, repository.IsEmailConflict(err))

		// Verification
		require.NoError(t, repo.MarkEmailVerified(ctx, user.ID))
		state, err := repo.GetSessionState(ctx, user.ID)
		require.NoError(t, err)
		assert.True(t, state.Verified)
		assert.Zero(t, state.Version)
		assert.Equal(t, user.OrgID, state.OrgID)
		assert.Equal(t, models.RoleOwner, state.Role)

		// Changing the email clears verification
		require.NoError(t, repo.SetUserEmail(ctx, user.ID, "new@example.com"))
		state, err = repo.GetSessionState(ctx, user.ID)
		require.NoError(t, err)
		assert.False(t, state.Verified)

		// Password changes bump the session version
		version, err := repo.UpdatePassword(ctx, user.ID, "newhash")
		require.NoError(t, err)
		assert.Equal(t, 1, version)

		// Codes: attempts are capped, re-issuing resets them
		code := &models.EmailCode{UserID: user.ID, Purpose: "verify_email", CodeHash: "h1", ExpiresAt: time.Now().Add(time.Hour)}
		require.NoError(t, repo.UpsertEmailCode(ctx, code))
		for i := 1; i <= 2; i++ {
			got, err := repo.UseEmailCodeAttempt(ctx, user.ID, "verify_email", 2)
			require.NoError(t, err)
			assert.Equal(t, "h1", got.CodeHash)
			assert.Equal(t, i, got.Attempts)
		}
		_, err = repo.UseEmailCodeAttempt(ctx, user.ID, "verify_email", 2)
		assert.ErrorIs(t, err, repository.ErrNotFound, "attempts exhausted")

		code.CodeHash = "h2"
		require.NoError(t, repo.UpsertEmailCode(ctx, code))
		got, err := repo.UseEmailCodeAttempt(ctx, user.ID, "verify_email", 2)
		require.NoError(t, err)
		assert.Equal(t, "h2", got.CodeHash)

		require.NoError(t, repo.DeleteEmailCode(ctx, user.ID, "verify_email"))
		_, err = repo.GetEmailCode(ctx, user.ID, "verify_email")
		assert.ErrorIs(t, err, repository.ErrNotFound)

		// change_email codes remember their target address
		target := "target@example.com"
		change := &models.EmailCode{UserID: user.ID, Purpose: "change_email", Email: &target, CodeHash: "h4", ExpiresAt: time.Now().Add(time.Hour)}
		require.NoError(t, repo.UpsertEmailCode(ctx, change))
		got, err = repo.UseEmailCodeAttempt(ctx, user.ID, "change_email", 5)
		require.NoError(t, err)
		require.NotNil(t, got.Email)
		assert.Equal(t, target, *got.Email)

		require.NoError(t, repo.ChangeUserEmail(ctx, user.ID, target))
		changed, err := repo.GetUserByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, target, *changed.Email)
		assert.NotNil(t, changed.EmailVerifiedAt, "a confirmed change is verified")

		taken := "TARGET@example.com"
		other := newOwner(t, "other-mailer")
		assert.ErrorIs(t, repo.ChangeUserEmail(ctx, other.ID, taken), repository.ErrConflict)

		// Expired codes can't be used
		expired := &models.EmailCode{UserID: user.ID, Purpose: "reset_password", CodeHash: "h3", ExpiresAt: time.Now().Add(-time.Minute)}
		require.NoError(t, repo.UpsertEmailCode(ctx, expired))
		_, err = repo.UseEmailCodeAttempt(ctx, user.ID, "reset_password", 5)
		assert.ErrorIs(t, err, repository.ErrNotFound)
	})

	t.Run("Organisation Handles", func(t *testing.T) {
		first := newOwner(t, "Handle Co")
		second := newOwner(t, "handle-co")
		a, err := repo.GetOrganization(ctx, first.OrgID)
		require.NoError(t, err)
		b, err := repo.GetOrganization(ctx, second.OrgID)
		require.NoError(t, err)
		assert.Equal(t, "handle-co-org", a.Handle)
		assert.Equal(t, "handle-co-org-2", b.Handle, "a taken handle gets a number")

		assert.ErrorIs(t, repo.SetOrgHandle(ctx, second.OrgID, "Handle-Co-Org"), repository.ErrConflict, "case-insensitive")
		require.NoError(t, repo.SetOrgHandle(ctx, first.OrgID, "handleco"))
		require.NoError(t, repo.SetOrgHandle(ctx, second.OrgID, "handle-co-org"), "the old handle is released")
		assert.ErrorIs(t, repo.SetOrgHandle(ctx, 999999, "nobody"), repository.ErrNotFound)

		// Too little of the name to use: the username is the fallback
		short := &models.User{Username: "fallback-user", PasswordHash: "hash"}
		require.NoError(t, repo.CreateOrgWithOwner(ctx, "!!", short))
		c, err := repo.GetOrganization(ctx, short.OrgID)
		require.NoError(t, err)
		assert.Equal(t, "fallback-user", c.Handle)
	})

	t.Run("Profile Flow", func(t *testing.T) {
		user := newOwner(t, "profileuser")
		scope := adminScope(user)

		profile := &models.Profile{
			OrgID:  user.OrgID,
			UserID: user.ID,
			Slug:   "my-awesome-slug",
			Data:   json.RawMessage(`{"theme": "dark"}`),
		}

		// Insert Profile
		err := repo.CreateProfile(ctx, profile)
		assert.NoError(t, err)
		assert.NotZero(t, profile.ID)

		// Create a second profile to test 1-to-many
		profile2 := &models.Profile{
			OrgID:  user.OrgID,
			UserID: user.ID,
			Slug:   "my-second-slug",
			Data:   json.RawMessage(`{"theme": "light"}`),
		}
		require.NoError(t, repo.CreateProfile(ctx, profile2))

		// Get by link, /p/{handle}/{slug}; both parts are case-insensitive
		org, err := repo.GetOrganization(ctx, user.OrgID)
		require.NoError(t, err)
		assert.Equal(t, "profileuser-org", org.Handle, "made from the organisation name")
		fetched, err := repo.GetProfileByPath(ctx, "ProfileUser-Org", "My-Awesome-Slug")
		assert.NoError(t, err)
		assert.Equal(t, profile.ID, fetched.ID)
		assert.Equal(t, user.OrgID, fetched.OrgID)
		assert.Equal(t, "profileuser-org", fetched.OrgHandle)
		_, err = repo.GetProfileByPath(ctx, "someone-else", "my-awesome-slug")
		assert.ErrorIs(t, err, repository.ErrNotFound)

		// Update Profile
		profile.Data = json.RawMessage(`{"theme": "blue"}`)
		err = repo.UpdateProfile(ctx, scope, profile)
		assert.NoError(t, err)

		// Verify multiple profiles for the organisation
		profiles, err := repo.ListProfiles(ctx, scope)
		assert.NoError(t, err)
		assert.Len(t, profiles, 2)
		// Assuming order by created_at desc
		assert.Equal(t, json.RawMessage(`{"theme": "light"}`), profiles[0].Data) // profile2
		assert.Equal(t, json.RawMessage(`{"theme": "blue"}`), profiles[1].Data)  // profile (updated)

		// Slugs are unique case-insensitively within the organisation
		dup := &models.Profile{OrgID: user.OrgID, UserID: user.ID, Slug: "MY-AWESOME-SLUG", Data: json.RawMessage(`{}`)}
		assert.ErrorIs(t, repo.CreateProfile(ctx, dup), repository.ErrConflict)

		// Other organisations can't read, update or delete the profile
		other := newOwner(t, "intruder")

		// ...but they can use the same slug for their own card
		same := &models.Profile{OrgID: other.OrgID, UserID: other.ID, Slug: "my-awesome-slug", Data: json.RawMessage(`{}`)}
		require.NoError(t, repo.CreateProfile(ctx, same))
		theirs, err := repo.GetProfileByPath(ctx, "intruder-org", "my-awesome-slug")
		require.NoError(t, err)
		assert.Equal(t, same.ID, theirs.ID)
		require.NoError(t, repo.DeleteProfile(ctx, same.ID, other.OrgID))

		_, err = repo.GetProfile(ctx, adminScope(other), profile.ID)
		assert.ErrorIs(t, err, repository.ErrNotFound)

		hijack := &models.Profile{ID: profile.ID, Slug: "hijacked", Data: json.RawMessage(`{}`)}
		assert.ErrorIs(t, repo.UpdateProfile(ctx, adminScope(other), hijack), repository.ErrNotFound)
		assert.ErrorIs(t, repo.DeleteProfile(ctx, profile.ID, other.OrgID), repository.ErrNotFound)

		// Members only see and edit the cards assigned to them
		member := newMember(t, user, "profile-member", models.RoleMember)
		_, err = repo.GetProfile(ctx, memberScope(member), profile.ID)
		assert.ErrorIs(t, err, repository.ErrNotFound)
		assert.ErrorIs(t, repo.UpdateProfile(ctx, memberScope(member), profile), repository.ErrNotFound)

		require.NoError(t, repo.SetProfileAssignee(ctx, profile.ID, user.OrgID, &member.ID))
		mine, err := repo.ListProfiles(ctx, memberScope(member))
		require.NoError(t, err)
		require.Len(t, mine, 1)
		assert.Equal(t, profile.ID, mine[0].ID)
		require.NotNil(t, mine[0].AssignedUser)
		assert.Equal(t, "profile-member", mine[0].AssignedUser.Username)
		assert.NoError(t, repo.UpdateProfile(ctx, memberScope(member), profile))

		assert.ErrorIs(t, repo.SetProfileAssignee(ctx, profile.ID, other.OrgID, nil), repository.ErrNotFound)

		owned, err := repo.GetProfile(ctx, scope, profile.ID)
		assert.NoError(t, err)
		assert.Equal(t, "my-awesome-slug", owned.Slug)

		// The organisation can delete
		assert.NoError(t, repo.DeleteProfile(ctx, profile2.ID, user.OrgID))
		profiles, err = repo.ListProfiles(ctx, scope)
		assert.NoError(t, err)
		assert.Len(t, profiles, 1)
	})

	t.Run("Lead Flow", func(t *testing.T) {
		// Create User and Profile
		user := newOwner(t, "leaduser")
		profile := &models.Profile{OrgID: user.OrgID, UserID: user.ID, Slug: "lead-slug", Data: json.RawMessage(`{}`)}
		require.NoError(t, repo.CreateProfile(ctx, profile))

		// Create Lead
		lead1 := &models.Lead{
			ProfileID:        profile.ID,
			Name:             "John Doe",
			Email:            "john@example.com",
			PhoneCountryCode: "+91",
			PhoneNumber:      "9876543210",
			Notes:            "Interested in product",
		}
		err := repo.CreateLead(ctx, lead1)
		assert.NoError(t, err)
		assert.NotZero(t, lead1.ID)

		// Create another Lead
		time.Sleep(10 * time.Millisecond) // Ensure ordering by created_at
		lead2 := &models.Lead{
			ProfileID: profile.ID,
			Name:      "Jane Smith",
			Email:     "jane@example.com",
			Notes:     "",
		}
		require.NoError(t, repo.CreateLead(ctx, lead2))

		// Get Leads
		leads, _, err := repo.ListLeads(ctx, adminScope(user), repository.LeadFilter{ProfileID: profile.ID, Limit: 10})
		assert.NoError(t, err)
		assert.Len(t, leads, 2)
		// Should be descending order by created_at
		assert.Equal(t, lead2.ID, leads[0].ID)
		assert.Equal(t, lead1.ID, leads[1].ID)
		// The phone round-trips as two parts; a lead without one reads back empty.
		assert.Equal(t, "+91", leads[1].PhoneCountryCode)
		assert.Equal(t, "9876543210", leads[1].PhoneNumber)
		assert.Empty(t, leads[0].PhoneCountryCode)
		assert.Empty(t, leads[0].PhoneNumber)

		// Search matches the phone number too
		found, total, err := repo.ListLeads(ctx, adminScope(user), repository.LeadFilter{Search: "98765", Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)
		require.Len(t, found, 1)
		assert.Equal(t, lead1.ID, found[0].ID)

		// The check constraint rejects a half-filled or malformed phone
		bad := &models.Lead{ProfileID: profile.ID, Name: "x", Email: "x@example.com", PhoneNumber: "98765 43210"}
		assert.Error(t, repo.CreateLead(ctx, bad))

		// Lead count is reported with the profile list
		profiles, err := repo.ListProfiles(ctx, adminScope(user))
		assert.NoError(t, err)
		require.Len(t, profiles, 1)
		assert.EqualValues(t, 2, profiles[0].LeadCount)

		// Leads stay with whoever held the card when they arrived
		rep := newMember(t, user, "lead-rep", models.RoleMember)
		require.NoError(t, repo.SetProfileAssignee(ctx, profile.ID, user.OrgID, &rep.ID))
		repLead := &models.Lead{ProfileID: profile.ID, Name: "For Rep", Email: "rep@example.com"}
		require.NoError(t, repo.CreateLead(ctx, repLead))

		repLeads, total, err := repo.ListLeads(ctx, memberScope(rep), repository.LeadFilter{Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 1, total, "the rep doesn't see leads from before the handover")
		require.Len(t, repLeads, 1)
		require.NotNil(t, repLeads[0].AssignedUser)
		assert.Equal(t, rep.ID, repLeads[0].AssignedUser.ID)

		repCards, err := repo.ListProfiles(ctx, memberScope(rep))
		require.NoError(t, err)
		require.Len(t, repCards, 1)
		assert.EqualValues(t, 1, repCards[0].LeadCount, "members count only their leads")

		_, total, err = repo.ListLeads(ctx, adminScope(user), repository.LeadFilter{UserID: rep.ID, Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)
		_, total, err = repo.ListLeads(ctx, adminScope(user), repository.LeadFilter{Unassigned: true, Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 2, total)

		// After a reassignment the rep keeps their lead
		require.NoError(t, repo.SetProfileAssignee(ctx, profile.ID, user.OrgID, nil))
		_, total, err = repo.ListLeads(ctx, memberScope(rep), repository.LeadFilter{Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)

		// Leads for a missing profile map to ErrNotFound
		orphan := &models.Lead{ProfileID: 999999, Name: "x", Email: "x@example.com"}
		assert.ErrorIs(t, repo.CreateLead(ctx, orphan), repository.ErrNotFound)
	})

	t.Run("Storage Settings", func(t *testing.T) {
		user := newOwner(t, "storer")
		member := newMember(t, user, "storer-member", models.RoleMember)

		_, err := repo.GetStorageSettings(ctx, user.ID)
		assert.ErrorIs(t, err, repository.ErrNotFound)
		assert.ErrorIs(t, repo.DeleteStorageSettings(ctx, user.ID), repository.ErrNotFound)

		now := time.Now()
		s := &models.StorageSettings{
			UserID: user.ID, Provider: "r2", Endpoint: "https://x.r2.cloudflarestorage.com", Region: "auto",
			Bucket: "one", AccessKeyIDEnc: []byte{1, 2}, SecretAccessKeyEnc: []byte{3, 4}, AccessKeyHint: "ABCD",
			VerifiedAt: &now,
		}
		require.NoError(t, repo.UpsertStorageSettings(ctx, s))

		// Upsert replaces in place.
		s.Bucket = "two"
		s.SecretAccessKeyEnc = []byte{9}
		require.NoError(t, repo.UpsertStorageSettings(ctx, s))
		got, err := repo.GetStorageSettings(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, "two", got.Bucket)
		assert.Equal(t, []byte{9}, got.SecretAccessKeyEnc)
		assert.Equal(t, []byte{1, 2}, got.AccessKeyIDEnc)
		require.NotNil(t, got.VerifiedAt)

		// The organisation's storage is the owner's
		orgStore, err := repo.GetOrgStorageSettings(ctx, member.OrgID)
		require.NoError(t, err)
		assert.Equal(t, user.ID, orgStore.UserID)

		require.NoError(t, repo.DeleteStorageSettings(ctx, user.ID))
		_, err = repo.GetStorageSettings(ctx, user.ID)
		assert.ErrorIs(t, err, repository.ErrNotFound)
	})

	t.Run("Files", func(t *testing.T) {
		owner := newOwner(t, "filer")
		other := newOwner(t, "nosy")
		rep := newMember(t, owner, "filer-rep", models.RoleMember)
		rep2 := newMember(t, owner, "filer-rep2", models.RoleMember)

		newFile := func(u *models.User, area, id, kind string) *models.File {
			f := &models.File{
				PublicID: id, OrgID: u.OrgID, UserID: u.ID, Area: area, Bucket: "b", ObjectKey: "fronko/" + id, Kind: kind,
				ContentType: "application/pdf", SizeBytes: 10, OriginalName: id + ".pdf",
			}
			require.NoError(t, repo.CreateFile(ctx, f))
			time.Sleep(5 * time.Millisecond)
			return f
		}
		img := newFile(owner, models.AreaOrg, "img-1", "image")
		pdf1 := newFile(owner, models.AreaOrg, "pdf-1", "pdf")
		pdf2 := newFile(owner, models.AreaShared, "pdf-2", "pdf")
		repFile := newFile(rep, models.AreaPersonal, "rep-1", "pdf")
		rep2File := newFile(rep2, models.AreaPersonal, "rep2-1", "pdf")
		foreign := newFile(other, models.AreaOrg, "foreign", "pdf")

		dup := &models.File{PublicID: "pdf-1", OrgID: owner.OrgID, UserID: owner.ID, Area: models.AreaOrg, Bucket: "b", ObjectKey: "k", Kind: "pdf", ContentType: "x", OriginalName: "x"}
		assert.ErrorIs(t, repo.CreateFile(ctx, dup), repository.ErrConflict)

		// Admins see the whole organisation, newest first
		all, total, err := repo.ListFiles(ctx, adminScope(owner), repository.FileFilter{Limit: 2})
		require.NoError(t, err)
		assert.EqualValues(t, 5, total)
		require.Len(t, all, 2)
		assert.Equal(t, rep2File.PublicID, all[0].PublicID, "newest first")
		require.NotNil(t, all[0].Owner)
		assert.Equal(t, "filer-rep2", all[0].Owner.Username)

		pdfs, total, err := repo.ListFiles(ctx, adminScope(owner), repository.FileFilter{Kind: "pdf", Area: models.AreaOrg, Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)
		assert.Len(t, pdfs, 1)

		_, total, err = repo.ListFiles(ctx, adminScope(owner), repository.FileFilter{Area: models.AreaPersonal, UserID: rep.ID, Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)

		n, err := repo.CountFilesForOrg(ctx, owner.OrgID)
		require.NoError(t, err)
		assert.EqualValues(t, 5, n)

		// Members see their own files and the shared area, nothing else
		visible, total, err := repo.ListFiles(ctx, memberScope(rep), repository.FileFilter{Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 2, total)
		ids := []string{visible[0].PublicID, visible[1].PublicID}
		assert.ElementsMatch(t, []string{repFile.PublicID, pdf2.PublicID}, ids)

		_, err = repo.GetFile(ctx, memberScope(rep), rep2File.PublicID)
		assert.ErrorIs(t, err, repository.ErrNotFound)
		_, err = repo.GetFile(ctx, memberScope(rep), img.PublicID)
		assert.ErrorIs(t, err, repository.ErrNotFound)

		// A grant opens one more file
		require.NoError(t, repo.ReplaceFileGrants(ctx, img.ID, owner.OrgID, owner.ID, []int64{rep.ID, other.ID}))
		grants, err := repo.ListFileGrants(ctx, img.ID)
		require.NoError(t, err)
		require.Len(t, grants, 1, "users from other organisations are ignored")
		assert.Equal(t, rep.ID, grants[0].ID)
		_, err = repo.GetFile(ctx, memberScope(rep), img.PublicID)
		assert.NoError(t, err)
		granted, total, err := repo.ListFiles(ctx, memberScope(rep), repository.FileFilter{GrantedTo: rep.ID, Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)
		assert.Equal(t, img.PublicID, granted[0].PublicID)

		count, err := repo.CountVisibleFiles(ctx, memberScope(rep), []string{img.PublicID, pdf2.PublicID, rep2File.PublicID})
		require.NoError(t, err)
		assert.Equal(t, 2, count)

		// Members can only edit their own personal files, even visible ones
		_, err = repo.GetEditableFile(ctx, memberScope(rep), img.PublicID)
		assert.ErrorIs(t, err, repository.ErrNotFound)
		_, err = repo.UpdateFile(ctx, memberScope(rep), pdf2.PublicID, repository.FilePatch{Title: ptr("mine now")})
		assert.ErrorIs(t, err, repository.ErrNotFound)
		_, err = repo.UpdateFile(ctx, memberScope(rep), repFile.PublicID, repository.FilePatch{Title: ptr("My notes")})
		assert.NoError(t, err)

		require.NoError(t, repo.ReplaceFileGrants(ctx, img.ID, owner.OrgID, owner.ID, nil))
		_, err = repo.GetFile(ctx, memberScope(rep), img.PublicID)
		assert.ErrorIs(t, err, repository.ErrNotFound, "an empty list revokes every grant")

		// Other organisations see nothing
		_, err = repo.GetFile(ctx, adminScope(owner), foreign.PublicID)
		assert.ErrorIs(t, err, repository.ErrNotFound)
		_, err = repo.UpdateFile(ctx, adminScope(owner), foreign.PublicID, repository.FilePatch{Title: ptr("mine now")})
		assert.ErrorIs(t, err, repository.ErrNotFound)
		assert.ErrorIs(t, repo.DeleteFile(ctx, foreign.ID, owner.OrgID), repository.ErrNotFound)

		byIDs, err := repo.GetOrgFilesByPublicIDs(ctx, owner.OrgID, []string{img.PublicID, foreign.PublicID, "missing"})
		require.NoError(t, err)
		require.Len(t, byIDs, 1, "only the organisation's files resolve")
		assert.Equal(t, img.PublicID, byIDs[0].PublicID)

		renamed, err := repo.UpdateFile(ctx, adminScope(owner), pdf1.PublicID, repository.FilePatch{Title: ptr("Spring brochure")})
		require.NoError(t, err)
		assert.Equal(t, "Spring brochure", renamed.Title)

		public, err := repo.GetFileByPublicID(ctx, pdf1.PublicID)
		require.NoError(t, err)
		assert.Equal(t, owner.OrgID, public.OrgID)

		require.NoError(t, repo.DeleteFile(ctx, pdf1.ID, owner.OrgID))
		_, err = repo.GetFileByPublicID(ctx, pdf1.PublicID)
		assert.ErrorIs(t, err, repository.ErrNotFound)

		// Quotas apply to personal files only
		quota := int64(25)
		require.NoError(t, repo.SetUserQuota(ctx, rep.ID, &quota))
		newFile(rep, models.AreaPersonal, "rep-2", "pdf") // 20 of 25 bytes
		over := &models.File{PublicID: "rep-3", OrgID: rep.OrgID, UserID: rep.ID, Area: models.AreaPersonal, Bucket: "b",
			ObjectKey: "k3", Kind: "pdf", ContentType: "x", SizeBytes: 10, OriginalName: "x"}
		assert.ErrorIs(t, repo.CreateFile(ctx, over), repository.ErrQuotaExceeded)
		used, err := repo.UsedBytes(ctx, rep.ID)
		require.NoError(t, err)
		assert.EqualValues(t, 20, used)
		over.Area = models.AreaShared
		assert.NoError(t, repo.CreateFile(ctx, over), "shared files don't count")
	})

	t.Run("Organisation Users", func(t *testing.T) {
		owner := newOwner(t, "boss")
		admin := newMember(t, owner, "boss-admin", models.RoleAdmin)
		rep := newMember(t, owner, "boss-rep", models.RoleMember)
		outsider := newOwner(t, "outsider")

		users, err := repo.ListOrgUsers(ctx, owner.OrgID)
		require.NoError(t, err)
		require.Len(t, users, 3)
		assert.Equal(t, []string{"boss", "boss-admin", "boss-rep"},
			[]string{users[0].Username, users[1].Username, users[2].Username}, "owner, admins, then members")

		_, err = repo.GetUserInOrg(ctx, rep.ID, outsider.OrgID)
		assert.ErrorIs(t, err, repository.ErrNotFound)

		// Roles: the owner's never changes
		assert.ErrorIs(t, repo.SetUserRole(ctx, owner.ID, models.RoleMember), repository.ErrNotFound)
		require.NoError(t, repo.SetUserRole(ctx, admin.ID, models.RoleMember))

		// Suspending ends sessions; restoring keeps them ended
		require.NoError(t, repo.SetUserSuspended(ctx, rep.ID, true))
		state, err := repo.GetSessionState(ctx, rep.ID)
		require.NoError(t, err)
		assert.True(t, state.Suspended)
		assert.Equal(t, 1, state.Version)
		require.NoError(t, repo.SetUserSuspended(ctx, rep.ID, false))
		state, err = repo.GetSessionState(ctx, rep.ID)
		require.NoError(t, err)
		assert.False(t, state.Suspended)

		// A temporary password must be replaced
		require.NoError(t, repo.SetTemporaryPassword(ctx, rep.ID, "temp"))
		state, err = repo.GetSessionState(ctx, rep.ID)
		require.NoError(t, err)
		assert.True(t, state.MustChangePassword)
		_, err = repo.UpdatePassword(ctx, rep.ID, "chosen")
		require.NoError(t, err)
		state, err = repo.GetSessionState(ctx, rep.ID)
		require.NoError(t, err)
		assert.False(t, state.MustChangePassword)

		// Deleting a user keeps their work
		card := &models.Profile{OrgID: owner.OrgID, UserID: admin.ID, AssignedUserID: &rep.ID, Slug: "boss-card", Data: json.RawMessage(`{}`)}
		require.NoError(t, repo.CreateProfile(ctx, card))
		lead := &models.Lead{ProfileID: card.ID, Name: "Kept", Email: "kept@example.com"}
		require.NoError(t, repo.CreateLead(ctx, lead))
		photo := &models.File{PublicID: "rep-photo", OrgID: rep.OrgID, UserID: rep.ID, Area: models.AreaPersonal, Bucket: "b",
			ObjectKey: "kp", Kind: "image", ContentType: "image/png", SizeBytes: 5, OriginalName: "me.png"}
		require.NoError(t, repo.CreateFile(ctx, photo))

		withTotals, err := repo.GetOrgUser(ctx, rep.ID, owner.OrgID)
		require.NoError(t, err)
		assert.EqualValues(t, 1, withTotals.CardCount)
		assert.EqualValues(t, 1, withTotals.LeadCount)
		assert.EqualValues(t, 5, withTotals.UsedBytes)

		assert.ErrorIs(t, repo.DeleteOrgUser(ctx, owner.ID, owner.OrgID), repository.ErrNotFound, "the owner can't be deleted")
		assert.ErrorIs(t, repo.DeleteOrgUser(ctx, rep.ID, outsider.OrgID), repository.ErrNotFound)
		require.NoError(t, repo.DeleteOrgUser(ctx, rep.ID, owner.OrgID))
		require.NoError(t, repo.DeleteOrgUser(ctx, admin.ID, owner.OrgID))

		kept, err := repo.GetProfile(ctx, adminScope(owner), card.ID)
		require.NoError(t, err, "a card created by a deleted admin survives")
		assert.Nil(t, kept.AssignedUserID)
		assert.Equal(t, owner.ID, kept.UserID)
		assert.EqualValues(t, 1, kept.LeadCount)

		moved, err := repo.GetFile(ctx, adminScope(owner), photo.PublicID)
		require.NoError(t, err)
		assert.Equal(t, models.AreaOrg, moved.Area)
		require.NotNil(t, moved.FormerOwner)
		assert.Equal(t, "boss-rep", *moved.FormerOwner)
	})

	t.Run("List Leads", func(t *testing.T) {
		owner := newOwner(t, "pager")
		scope := adminScope(owner)
		cardA := &models.Profile{OrgID: owner.OrgID, UserID: owner.ID, Slug: "pager-a", Data: json.RawMessage(`{}`)}
		cardB := &models.Profile{OrgID: owner.OrgID, UserID: owner.ID, Slug: "pager-b", Data: json.RawMessage(`{}`)}
		require.NoError(t, repo.CreateProfile(ctx, cardA))
		require.NoError(t, repo.CreateProfile(ctx, cardB))

		stranger := newOwner(t, "stranger")
		foreign := &models.Profile{OrgID: stranger.OrgID, UserID: stranger.ID, Slug: "foreign", Data: json.RawMessage(`{}`)}
		require.NoError(t, repo.CreateProfile(ctx, foreign))
		require.NoError(t, repo.CreateLead(ctx, &models.Lead{ProfileID: foreign.ID, Name: "Not Yours", Email: "no@example.com"}))

		// 5 leads on A, 2 on B, created oldest to newest.
		var created []*models.Lead
		for i, p := range []int64{cardA.ID, cardA.ID, cardB.ID, cardA.ID, cardB.ID, cardA.ID, cardA.ID} {
			l := &models.Lead{ProfileID: p, Name: "Lead " + string(rune('A'+i)), Email: "lead@example.com"}
			require.NoError(t, repo.CreateLead(ctx, l))
			created = append(created, l)
			time.Sleep(5 * time.Millisecond)
		}

		all, total, err := repo.ListLeads(ctx, scope, repository.LeadFilter{Limit: 3})
		require.NoError(t, err)
		assert.EqualValues(t, 7, total, "excludes the other organisation's lead")
		require.Len(t, all, 3)
		assert.Equal(t, created[6].ID, all[0].ID, "newest first")

		last, total, err := repo.ListLeads(ctx, scope, repository.LeadFilter{Limit: 3, Offset: 6})
		require.NoError(t, err)
		assert.EqualValues(t, 7, total)
		require.Len(t, last, 1)
		assert.Equal(t, created[0].ID, last[0].ID)

		beyond, total, err := repo.ListLeads(ctx, scope, repository.LeadFilter{Limit: 3, Offset: 30})
		require.NoError(t, err)
		assert.EqualValues(t, 7, total, "total is still reported past the last page")
		assert.Empty(t, beyond)

		onlyB, total, err := repo.ListLeads(ctx, scope, repository.LeadFilter{ProfileID: cardB.ID, Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 2, total)
		for _, l := range onlyB {
			assert.Equal(t, cardB.ID, l.ProfileID)
		}

		_, total, err = repo.ListLeads(ctx, scope, repository.LeadFilter{ProfileID: foreign.ID, Limit: 10})
		require.NoError(t, err)
		assert.Zero(t, total, "another organisation's profile yields nothing")

		found, total, err := repo.ListLeads(ctx, scope, repository.LeadFilter{Search: "lead c", Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 1, total, "search is case-insensitive")
		require.Len(t, found, 1)
		assert.Equal(t, created[2].ID, found[0].ID)

		_, total, err = repo.ListLeads(ctx, scope, repository.LeadFilter{Search: "%", Limit: 10})
		require.NoError(t, err)
		assert.Zero(t, total, "LIKE wildcards in the search are matched literally")

		_, total, err = repo.ListLeads(ctx, scope, repository.LeadFilter{Since: created[5].CreatedAt, Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 2, total, "since is inclusive")
	})
	t.Run("Org Branding", func(t *testing.T) {
		owner := newOwner(t, "brander")
		b, err := repo.GetOrgBranding(ctx, owner.OrgID)
		require.NoError(t, err)
		assert.Equal(t, models.LogoOptional, b.LogoPolicy)
		assert.Nil(t, b.LogoFile)

		newImage := func(id string) *models.File {
			f := &models.File{PublicID: id, OrgID: owner.OrgID, UserID: owner.ID, Area: models.AreaOrg, Bucket: "b",
				ObjectKey: "fronko/" + id, Kind: "image", ContentType: "image/png", SizeBytes: 5, OriginalName: id + ".png"}
			require.NoError(t, repo.CreateFile(ctx, f))
			return f
		}
		logo, banner, other := newImage("brand-logo"), newImage("brand-banner"), newImage("brand-other")

		logoID := logo.PublicID
		b = &models.OrgBranding{LogoFile: &logoID, LogoPolicy: models.LogoRequired, Signature: models.OrgSignature{
			LockedTemplate: "classic", Disclaimer: "Confidential", BannerFile: banner.PublicID, BannerURL: "https://acme.test",
		}}
		require.NoError(t, repo.UpdateOrgBranding(ctx, owner.OrgID, b))
		assert.Equal(t, "brander org", b.Name)

		got, err := repo.GetOrgBranding(ctx, owner.OrgID)
		require.NoError(t, err)
		require.NotNil(t, got.LogoFile)
		assert.Equal(t, logoID, *got.LogoFile)
		assert.Equal(t, models.LogoRequired, got.LogoPolicy)
		assert.Equal(t, b.Signature, got.Signature)

		// Deleting an unrelated file leaves the branding alone.
		require.NoError(t, repo.DeleteFile(ctx, other.ID, owner.OrgID))
		got, _ = repo.GetOrgBranding(ctx, owner.OrgID)
		assert.NotNil(t, got.LogoFile)
		assert.Equal(t, banner.PublicID, got.Signature.BannerFile)

		// Deleting the logo or banner clears the reference.
		require.NoError(t, repo.DeleteFile(ctx, logo.ID, owner.OrgID))
		require.NoError(t, repo.DeleteFile(ctx, banner.ID, owner.OrgID))
		got, err = repo.GetOrgBranding(ctx, owner.OrgID)
		require.NoError(t, err)
		assert.Nil(t, got.LogoFile)
		assert.Empty(t, got.Signature.BannerFile)
		assert.Equal(t, "Confidential", got.Signature.Disclaimer, "other settings are kept")
		assert.ErrorIs(t, repo.DeleteFile(ctx, logo.ID, owner.OrgID), repository.ErrNotFound)
	})

	t.Run("Teams", func(t *testing.T) {
		owner := newOwner(t, "teamer")
		other := newOwner(t, "teamer-other")
		alice := newMember(t, owner, "teamer-alice", models.RoleMember)
		bob := newMember(t, owner, "teamer-bob", models.RoleMember)
		stranger := newMember(t, other, "teamer-stranger", models.RoleMember)

		sales := &models.Team{OrgID: owner.OrgID, Name: "Sales", Color: "#ff0000"}
		require.NoError(t, repo.CreateTeam(ctx, sales))
		finance := &models.Team{OrgID: owner.OrgID, Name: "Finance"}
		require.NoError(t, repo.CreateTeam(ctx, finance))
		assert.ErrorIs(t, repo.CreateTeam(ctx, &models.Team{OrgID: owner.OrgID, Name: "sales"}), repository.ErrConflict,
			"names are unique per organisation, ignoring case")
		require.NoError(t, repo.CreateTeam(ctx, &models.Team{OrgID: other.OrgID, Name: "Sales"}), "other orgs may reuse a name")

		// Members from another organisation are ignored.
		require.NoError(t, repo.ReplaceTeamMembers(ctx, owner.OrgID, sales.ID, []models.TeamMembership{
			{UserID: alice.ID, Role: models.TeamRoleLead}, {UserID: bob.ID, Role: models.TeamRoleMember}, {UserID: stranger.ID, Role: models.TeamRoleMember},
		}))
		members, err := repo.ListTeamMembers(ctx, sales.ID)
		require.NoError(t, err)
		require.Len(t, members, 2)
		assert.Equal(t, "teamer-alice", members[0].Username, "leads first")
		assert.Equal(t, models.TeamRoleLead, members[0].Role)

		// A user can be in several teams.
		require.NoError(t, repo.ReplaceUserTeams(ctx, owner.OrgID, bob.ID, []models.TeamMembership{
			{TeamID: sales.ID, Role: models.TeamRoleMember}, {TeamID: finance.ID, Role: models.TeamRoleLead},
		}))
		teams, err := repo.ListUserTeams(ctx, bob.ID)
		require.NoError(t, err)
		require.Len(t, teams, 2)
		assert.Equal(t, "Finance", teams[0].Name)
		assert.Equal(t, models.TeamRoleLead, teams[0].Role)

		state, err := repo.GetSessionState(ctx, alice.ID)
		require.NoError(t, err)
		require.Len(t, state.Teams, 1)
		assert.Equal(t, sales.ID, state.Teams[0].ID)

		users, err := repo.ListOrgUsers(ctx, owner.OrgID)
		require.NoError(t, err)
		for _, u := range users {
			if u.ID == bob.ID {
				assert.Len(t, u.Teams, 2)
			}
			if u.ID == owner.ID {
				assert.NotNil(t, u.Teams)
				assert.Empty(t, u.Teams)
			}
		}

		got, err := repo.GetTeam(ctx, owner.OrgID, sales.ID)
		require.NoError(t, err)
		assert.EqualValues(t, 2, got.MemberCount)
		assert.EqualValues(t, 1, got.LeadCount)
		_, err = repo.GetTeam(ctx, other.OrgID, sales.ID)
		assert.ErrorIs(t, err, repository.ErrNotFound)

		all, err := repo.ListTeams(ctx, owner.OrgID, 0)
		require.NoError(t, err)
		assert.Len(t, all, 2)
		mine, err := repo.ListTeams(ctx, owner.OrgID, alice.ID)
		require.NoError(t, err)
		require.Len(t, mine, 1)
		assert.Equal(t, "Sales", mine[0].Name)

		// New users can join teams as they're created.
		carol := &models.User{OrgID: owner.OrgID, Role: models.RoleMember, Username: "teamer-carol", PasswordHash: "hash"}
		require.NoError(t, repo.CreateUser(ctx, carol, models.TeamMembership{TeamID: finance.ID, Role: models.TeamRoleMember}))
		teams, _ = repo.ListUserTeams(ctx, carol.ID)
		assert.Len(t, teams, 1)

		// Deleting a team keeps its files, as the organisation's.
		teamFile := &models.File{PublicID: "team-file-1", OrgID: owner.OrgID, UserID: alice.ID, Area: models.AreaTeam, TeamID: &sales.ID,
			Bucket: "b", ObjectKey: "k", Kind: "image", ContentType: "image/png", OriginalName: "a.png"}
		require.NoError(t, repo.CreateFile(ctx, teamFile))
		require.NoError(t, repo.DeleteTeam(ctx, owner.OrgID, sales.ID))
		f, err := repo.GetFile(ctx, adminScope(owner), teamFile.PublicID)
		require.NoError(t, err)
		assert.Equal(t, models.AreaOrg, f.Area)
		assert.Nil(t, f.Team)
		teams, _ = repo.ListUserTeams(ctx, alice.ID)
		assert.Empty(t, teams)
		assert.ErrorIs(t, repo.DeleteTeam(ctx, owner.OrgID, sales.ID), repository.ErrNotFound)
	})

	t.Run("Team File Access", func(t *testing.T) {
		owner := newOwner(t, "tf")
		lead := newMember(t, owner, "tf-lead", models.RoleMember)
		member := newMember(t, owner, "tf-member", models.RoleMember)
		outsider := newMember(t, owner, "tf-outsider", models.RoleMember)
		sales := &models.Team{OrgID: owner.OrgID, Name: "Sales"}
		require.NoError(t, repo.CreateTeam(ctx, sales))
		finance := &models.Team{OrgID: owner.OrgID, Name: "Finance"}
		require.NoError(t, repo.CreateTeam(ctx, finance))
		require.NoError(t, repo.ReplaceTeamMembers(ctx, owner.OrgID, sales.ID, []models.TeamMembership{
			{UserID: lead.ID, Role: models.TeamRoleLead}, {UserID: member.ID, Role: models.TeamRoleMember},
		}))
		require.NoError(t, repo.ReplaceTeamMembers(ctx, owner.OrgID, finance.ID, []models.TeamMembership{
			{UserID: outsider.ID, Role: models.TeamRoleMember},
		}))

		newFile := func(u *models.User, area, id string, team *int64) *models.File {
			f := &models.File{PublicID: id, OrgID: u.OrgID, UserID: u.ID, Area: area, TeamID: team, Bucket: "b",
				ObjectKey: "k/" + id, Kind: "image", ContentType: "image/png", OriginalName: id + ".png"}
			require.NoError(t, repo.CreateFile(ctx, f))
			return f
		}
		salesFile := newFile(owner, models.AreaTeam, "tf-sales", &sales.ID)
		financeFile := newFile(owner, models.AreaTeam, "tf-finance", &finance.ID)
		orgFile := newFile(owner, models.AreaOrg, "tf-org", nil)
		grantedToTeam := newFile(owner, models.AreaOrg, "tf-granted", nil)
		require.NoError(t, repo.ReplaceFileTeamGrants(ctx, grantedToTeam.ID, owner.OrgID, owner.ID, []int64{sales.ID}))
		leadsOwn := newFile(lead, models.AreaPersonal, "tf-leads-own", nil)

		visible := func(s repository.Scope, f *models.File) bool {
			_, err := repo.GetFile(ctx, s, f.PublicID)
			return err == nil
		}
		editable := func(s repository.Scope, f *models.File) bool {
			_, err := repo.GetEditableFile(ctx, s, f.PublicID)
			return err == nil
		}
		for _, c := range []struct {
			name              string
			scope             repository.Scope
			file              *models.File
			canSee, canChange bool
		}{
			{"lead sees and edits team file", memberScope(lead), salesFile, true, true},
			{"member sees team file", memberScope(member), salesFile, true, false},
			{"outsider can't see another team's file", memberScope(outsider), salesFile, false, false},
			{"outsider sees own team's file", memberScope(outsider), financeFile, true, false},
			{"lead can't see another team's file", memberScope(lead), financeFile, false, false},
			{"members don't see org files", memberScope(member), orgFile, false, false},
			{"team grant reaches members", memberScope(member), grantedToTeam, true, false},
			{"team grant doesn't reach others", memberScope(outsider), grantedToTeam, false, false},
			{"teammates don't see each other's personal files", memberScope(member), leadsOwn, false, false},
			{"admins see everything", adminScope(owner), financeFile, true, true},
		} {
			assert.Equal(t, c.canSee, visible(c.scope, c.file), c.name)
			assert.Equal(t, c.canChange, editable(c.scope, c.file), c.name)
		}

		files, total, err := repo.ListFiles(ctx, memberScope(member), repository.FileFilter{Area: models.AreaTeam, TeamID: sales.ID, Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)
		require.Len(t, files, 1)
		require.NotNil(t, files[0].Team)
		assert.Equal(t, "Sales", files[0].Team.Name)

		_, total, err = repo.ListFiles(ctx, memberScope(member), repository.FileFilter{GrantedTo: member.ID, Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 1, total, "files granted through a team count as shared with me")

		// Leads move their own files into their team.
		area := models.AreaTeam
		moved, err := repo.UpdateFile(ctx, memberScope(lead), leadsOwn.PublicID, repository.FilePatch{Area: &area, TeamID: &sales.ID})
		require.NoError(t, err)
		assert.Equal(t, models.AreaTeam, moved.Area)
		assert.True(t, visible(memberScope(member), leadsOwn), "now the team sees it")

		teams, err := repo.ListFileTeamGrants(ctx, grantedToTeam.ID)
		require.NoError(t, err)
		require.Len(t, teams, 1)
		assert.Equal(t, sales.ID, teams[0].ID)
	})

	t.Run("Team Lead Cards And Leads", func(t *testing.T) {
		owner := newOwner(t, "tl")
		lead := newMember(t, owner, "tl-lead", models.RoleMember)
		mate := newMember(t, owner, "tl-mate", models.RoleMember)
		outsider := newMember(t, owner, "tl-outsider", models.RoleMember)
		team := &models.Team{OrgID: owner.OrgID, Name: "Sales"}
		require.NoError(t, repo.CreateTeam(ctx, team))
		require.NoError(t, repo.ReplaceTeamMembers(ctx, owner.OrgID, team.ID, []models.TeamMembership{
			{UserID: lead.ID, Role: models.TeamRoleLead}, {UserID: mate.ID, Role: models.TeamRoleMember},
		}))

		card := func(slug string, u *models.User) *models.Profile {
			p := &models.Profile{OrgID: owner.OrgID, UserID: owner.ID, AssignedUserID: &u.ID, Slug: slug, Data: json.RawMessage(`{}`)}
			require.NoError(t, repo.CreateProfile(ctx, p))
			require.NoError(t, repo.CreateLead(ctx, &models.Lead{ProfileID: p.ID, Name: "Lead for " + slug, Email: "x@example.com"}))
			return p
		}
		leadCard := card("tl-lead-card", lead)
		mateCard := card("tl-mate-card", mate)
		outsiderCard := card("tl-outsider-card", outsider)

		cards, err := repo.ListProfiles(ctx, memberScope(lead))
		require.NoError(t, err)
		var ids []int64
		for _, c := range cards {
			ids = append(ids, c.ID)
		}
		assert.ElementsMatch(t, []int64{leadCard.ID, mateCard.ID}, ids, "leads see their team's cards")

		got, err := repo.GetProfile(ctx, memberScope(lead), mateCard.ID)
		require.NoError(t, err)
		assert.EqualValues(t, 1, got.LeadCount)
		_, err = repo.GetProfile(ctx, memberScope(lead), outsiderCard.ID)
		assert.ErrorIs(t, err, repository.ErrNotFound)
		require.NoError(t, repo.UpdateProfile(ctx, memberScope(lead), mateCard), "leads can edit teammates' cards")

		// Plain members still only see their own.
		cards, err = repo.ListProfiles(ctx, memberScope(mate))
		require.NoError(t, err)
		require.Len(t, cards, 1)
		assert.Equal(t, mateCard.ID, cards[0].ID)

		_, total, err := repo.ListLeads(ctx, memberScope(lead), repository.LeadFilter{Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 2, total)
		_, total, err = repo.ListLeads(ctx, memberScope(mate), repository.LeadFilter{Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)
		_, total, err = repo.ListLeads(ctx, adminScope(owner), repository.LeadFilter{TeamID: team.ID, Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 2, total, "admins filter leads by team")
	})

	t.Run("File Purposes And Usage", func(t *testing.T) {
		owner := newOwner(t, "fp")
		rep := newMember(t, owner, "fp-rep", models.RoleMember)
		newFile := func(id, kind, purpose, title string, size int64) *models.File {
			f := &models.File{PublicID: id, OrgID: owner.OrgID, UserID: owner.ID, Area: models.AreaShared, Bucket: "b",
				ObjectKey: "k/" + id, Kind: kind, Purpose: purpose, ContentType: "x", SizeBytes: size, OriginalName: id, Title: title}
			require.NoError(t, repo.CreateFile(ctx, f))
			time.Sleep(5 * time.Millisecond)
			return f
		}
		avatar := newFile("fp-avatar-000000000001", "image", models.PurposeAvatar, "Headshot", 30)
		cover := newFile("fp-cover-0000000000001", "image", models.PurposeCover, "Beach cover", 20)
		brochure := newFile("fp-brochure-0000000001", "pdf", models.PurposeBrochure, "Price list", 50)
		plain := newFile("fp-plain-0000000000001", "image", "", "", 10)
		assert.Equal(t, models.PurposeOther, plain.Purpose, "purpose defaults to other")

		// Purpose, search and sort.
		files, total, err := repo.ListFiles(ctx, adminScope(owner), repository.FileFilter{Purposes: []string{models.PurposeAvatar, models.PurposeCover}, Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 2, total)
		_, total, err = repo.ListFiles(ctx, adminScope(owner), repository.FileFilter{Search: "beach", Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)
		_, total, err = repo.ListFiles(ctx, adminScope(owner), repository.FileFilter{Search: "100%", Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 0, total, "wildcards match literally")
		files, _, err = repo.ListFiles(ctx, adminScope(owner), repository.FileFilter{Sort: repository.FileSortSize, Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, brochure.PublicID, files[0].PublicID)
		files, _, err = repo.ListFiles(ctx, adminScope(owner), repository.FileFilter{Sort: repository.FileSortName, Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, cover.PublicID, files[0].PublicID, "by title, then file name")
		files, _, err = repo.ListFiles(ctx, adminScope(owner), repository.FileFilter{Sort: repository.FileSortOldest, Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, avatar.PublicID, files[0].PublicID)

		counts, err := repo.CountFilesByPurpose(ctx, adminScope(owner), repository.FileFilter{Purposes: []string{models.PurposeAvatar}})
		require.NoError(t, err)
		assert.Equal(t, map[string]int64{"avatar": 1, "cover": 1, "brochure": 1, "other": 1}, counts)

		// Cards record which files they use, kept up to date on every save.
		data := `{"name":"Rep card","avatar_file":"` + avatar.PublicID + `","documents":[{"file":"` + brochure.PublicID + `"}]}`
		repCard := &models.Profile{OrgID: owner.OrgID, UserID: owner.ID, AssignedUserID: &rep.ID, Slug: "fp-rep", Data: json.RawMessage(data)}
		require.NoError(t, repo.CreateProfile(ctx, repCard))
		ownerCard := &models.Profile{OrgID: owner.OrgID, UserID: owner.ID, Slug: "fp-owner", Data: json.RawMessage(`{"cover_file":"` + avatar.PublicID + `"}`)}
		require.NoError(t, repo.CreateProfile(ctx, ownerCard))

		f, err := repo.GetFile(ctx, adminScope(owner), avatar.PublicID)
		require.NoError(t, err)
		assert.EqualValues(t, 2, f.UseCount)

		usage, err := repo.GetFileUsage(ctx, memberScope(rep), f)
		require.NoError(t, err)
		require.Len(t, usage.Cards, 1, "only cards the member can see are listed")
		assert.Equal(t, "Rep card", usage.Cards[0].Name)
		assert.Equal(t, repository.SlotAvatar, usage.Cards[0].Slot)
		assert.EqualValues(t, 1, usage.HiddenCards)

		ownerCard.Data = json.RawMessage(`{}`)
		require.NoError(t, repo.UpdateProfile(ctx, adminScope(owner), ownerCard))
		f, _ = repo.GetFile(ctx, adminScope(owner), avatar.PublicID)
		assert.EqualValues(t, 1, f.UseCount, "saving a card drops files it no longer uses")

		// The org logo counts as a use.
		logoID := cover.PublicID
		require.NoError(t, repo.UpdateOrgBranding(ctx, owner.OrgID, &models.OrgBranding{LogoFile: &logoID, LogoPolicy: models.LogoOptional}))
		f, _ = repo.GetFile(ctx, adminScope(owner), cover.PublicID)
		assert.EqualValues(t, 1, f.UseCount)
		usage, err = repo.GetFileUsage(ctx, adminScope(owner), f)
		require.NoError(t, err)
		assert.True(t, usage.OrgLogo)

		// Renaming and re-purposing.
		title, purpose := "Team photo", models.PurposeGallery
		updated, err := repo.UpdateFile(ctx, adminScope(owner), plain.PublicID, repository.FilePatch{Title: &title, Purpose: &purpose})
		require.NoError(t, err)
		assert.Equal(t, "Team photo", updated.Title)
		assert.Equal(t, models.PurposeGallery, updated.Purpose)
		assert.Equal(t, models.AreaShared, updated.Area, "area untouched")
		_, err = repo.UpdateFile(ctx, memberScope(rep), plain.PublicID, repository.FilePatch{Title: &title})
		assert.ErrorIs(t, err, repository.ErrNotFound, "members can't change shared files")
	})
}

func ptr[T any](v T) *T { return &v }
