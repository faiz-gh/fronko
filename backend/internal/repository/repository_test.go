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
	_, err = pool.Exec(ctx, "TRUNCATE TABLE organizations, users, profiles, leads, user_storage, files, file_grants, email_codes, platform_admins, feedback, feedback_replies, org_usage_snapshots, platform_usage_snapshots, admin_audit_log RESTART IDENTITY CASCADE")
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

		// Get By Slug
		fetched, err := repo.GetProfileBySlug(ctx, "My-Awesome-Slug") // Testing case-insensitivity
		assert.NoError(t, err)
		assert.Equal(t, profile.ID, fetched.ID)
		assert.Equal(t, user.OrgID, fetched.OrgID)

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

		// Slugs are unique case-insensitively
		dup := &models.Profile{OrgID: user.OrgID, UserID: user.ID, Slug: "MY-AWESOME-SLUG", Data: json.RawMessage(`{}`)}
		assert.ErrorIs(t, repo.CreateProfile(ctx, dup), repository.ErrConflict)

		// Other organisations can't read, update or delete the profile
		other := newOwner(t, "intruder")

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
		_, err = repo.UpdateFileTitle(ctx, memberScope(rep), pdf2.PublicID, "mine now")
		assert.ErrorIs(t, err, repository.ErrNotFound)
		_, err = repo.UpdateFileTitle(ctx, memberScope(rep), repFile.PublicID, "My notes")
		assert.NoError(t, err)

		require.NoError(t, repo.ReplaceFileGrants(ctx, img.ID, owner.OrgID, owner.ID, nil))
		_, err = repo.GetFile(ctx, memberScope(rep), img.PublicID)
		assert.ErrorIs(t, err, repository.ErrNotFound, "an empty list revokes every grant")

		// Other organisations see nothing
		_, err = repo.GetFile(ctx, adminScope(owner), foreign.PublicID)
		assert.ErrorIs(t, err, repository.ErrNotFound)
		_, err = repo.UpdateFileTitle(ctx, adminScope(owner), foreign.PublicID, "mine now")
		assert.ErrorIs(t, err, repository.ErrNotFound)
		assert.ErrorIs(t, repo.DeleteFile(ctx, foreign.ID, owner.OrgID), repository.ErrNotFound)

		byIDs, err := repo.GetOrgFilesByPublicIDs(ctx, owner.OrgID, []string{img.PublicID, foreign.PublicID, "missing"})
		require.NoError(t, err)
		require.Len(t, byIDs, 1, "only the organisation's files resolve")
		assert.Equal(t, img.PublicID, byIDs[0].PublicID)

		renamed, err := repo.UpdateFileTitle(ctx, adminScope(owner), pdf1.PublicID, "Spring brochure")
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
}
