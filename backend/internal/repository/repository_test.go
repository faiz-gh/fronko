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
	_, err = pool.Exec(ctx, "TRUNCATE TABLE users, profiles, leads, user_storage, files RESTART IDENTITY CASCADE")
	require.NoError(t, err)

	t.Run("User Flow", func(t *testing.T) {
		user := &models.User{
			Username:     "testuser",
			PasswordHash: "hashed_password",
		}

		err := repo.CreateUser(ctx, user)
		assert.NoError(t, err)
		assert.NotZero(t, user.ID)
		assert.NotZero(t, user.CreatedAt)

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
		assert.ErrorIs(t, repo.CreateUser(ctx, dup), repository.ErrConflict)

		_, err = repo.GetUserByUsername(ctx, "nobody")
		assert.ErrorIs(t, err, repository.ErrNotFound)
	})

	t.Run("Profile Flow", func(t *testing.T) {
		// First create a user for the profile
		user := &models.User{Username: "profileuser", PasswordHash: "hash"}
		err := repo.CreateUser(ctx, user)
		require.NoError(t, err)

		profile := &models.Profile{
			UserID: user.ID,
			Slug:   "my-awesome-slug",
			Data:   json.RawMessage(`{"theme": "dark"}`),
		}

		// Insert Profile
		err = repo.CreateProfile(ctx, profile)
		assert.NoError(t, err)
		assert.NotZero(t, profile.ID)

		// Create a second profile to test 1-to-many
		profile2 := &models.Profile{
			UserID: user.ID,
			Slug:   "my-second-slug",
			Data:   json.RawMessage(`{"theme": "light"}`),
		}
		require.NoError(t, repo.CreateProfile(ctx, profile2))

		// Get By Slug
		fetched, err := repo.GetProfileBySlug(ctx, "My-Awesome-Slug") // Testing case-insensitivity
		assert.NoError(t, err)
		assert.Equal(t, profile.ID, fetched.ID)

		// Update Profile
		profile.Data = json.RawMessage(`{"theme": "blue"}`)
		err = repo.UpdateProfile(ctx, profile)
		assert.NoError(t, err)

		// Verify multiple profiles for user
		profiles, err := repo.GetProfilesByUserID(ctx, user.ID)
		assert.NoError(t, err)
		assert.Len(t, profiles, 2)
		// Assuming order by created_at desc
		assert.Equal(t, json.RawMessage(`{"theme": "light"}`), profiles[0].Data) // profile2
		assert.Equal(t, json.RawMessage(`{"theme": "blue"}`), profiles[1].Data)  // profile (updated)

		// Slugs are unique case-insensitively
		dup := &models.Profile{UserID: user.ID, Slug: "MY-AWESOME-SLUG", Data: json.RawMessage(`{}`)}
		assert.ErrorIs(t, repo.CreateProfile(ctx, dup), repository.ErrConflict)

		// Other users can't read, update or delete the profile
		other := &models.User{Username: "intruder", PasswordHash: "hash"}
		require.NoError(t, repo.CreateUser(ctx, other))

		_, err = repo.GetProfileForUser(ctx, profile.ID, other.ID)
		assert.ErrorIs(t, err, repository.ErrNotFound)

		hijack := &models.Profile{ID: profile.ID, UserID: other.ID, Slug: "hijacked", Data: json.RawMessage(`{}`)}
		assert.ErrorIs(t, repo.UpdateProfile(ctx, hijack), repository.ErrNotFound)
		assert.ErrorIs(t, repo.DeleteProfile(ctx, profile.ID, other.ID), repository.ErrNotFound)

		owned, err := repo.GetProfileForUser(ctx, profile.ID, user.ID)
		assert.NoError(t, err)
		assert.Equal(t, "my-awesome-slug", owned.Slug)

		// Owner can delete
		assert.NoError(t, repo.DeleteProfile(ctx, profile2.ID, user.ID))
		profiles, err = repo.GetProfilesByUserID(ctx, user.ID)
		assert.NoError(t, err)
		assert.Len(t, profiles, 1)
	})

	t.Run("Lead Flow", func(t *testing.T) {
		// Create User and Profile
		user := &models.User{Username: "leaduser", PasswordHash: "hash"}
		require.NoError(t, repo.CreateUser(ctx, user))
		profile := &models.Profile{UserID: user.ID, Slug: "lead-slug", Data: json.RawMessage(`{}`)}
		require.NoError(t, repo.CreateProfile(ctx, profile))

		// Create Lead
		lead1 := &models.Lead{
			ProfileID: profile.ID,
			Name:      "John Doe",
			Email:     "john@example.com",
			Notes:     "Interested in product",
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
		leads, err := repo.GetLeadsByProfileID(ctx, profile.ID)
		assert.NoError(t, err)
		assert.Len(t, leads, 2)
		// Should be descending order by created_at
		assert.Equal(t, lead2.ID, leads[0].ID)
		assert.Equal(t, lead1.ID, leads[1].ID)

		// Lead count is reported with the profile list
		profiles, err := repo.GetProfilesByUserID(ctx, user.ID)
		assert.NoError(t, err)
		require.Len(t, profiles, 1)
		assert.EqualValues(t, 2, profiles[0].LeadCount)

		// Leads for a missing profile map to ErrNotFound
		orphan := &models.Lead{ProfileID: 999999, Name: "x", Email: "x@example.com"}
		assert.ErrorIs(t, repo.CreateLead(ctx, orphan), repository.ErrNotFound)
	})

	t.Run("Storage Settings", func(t *testing.T) {
		user := &models.User{Username: "storer", PasswordHash: "hash"}
		require.NoError(t, repo.CreateUser(ctx, user))

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

		require.NoError(t, repo.DeleteStorageSettings(ctx, user.ID))
		_, err = repo.GetStorageSettings(ctx, user.ID)
		assert.ErrorIs(t, err, repository.ErrNotFound)
	})

	t.Run("Files", func(t *testing.T) {
		owner := &models.User{Username: "filer", PasswordHash: "hash"}
		other := &models.User{Username: "nosy", PasswordHash: "hash"}
		require.NoError(t, repo.CreateUser(ctx, owner))
		require.NoError(t, repo.CreateUser(ctx, other))

		newFile := func(userID int64, id, kind string) *models.File {
			f := &models.File{
				PublicID: id, UserID: userID, Bucket: "b", ObjectKey: "fronko/" + id, Kind: kind,
				ContentType: "application/pdf", SizeBytes: 10, OriginalName: id + ".pdf",
			}
			require.NoError(t, repo.CreateFile(ctx, f))
			time.Sleep(5 * time.Millisecond)
			return f
		}
		img := newFile(owner.ID, "img-1", "image")
		pdf1 := newFile(owner.ID, "pdf-1", "pdf")
		pdf2 := newFile(owner.ID, "pdf-2", "pdf")
		foreign := newFile(other.ID, "foreign", "pdf")

		dup := &models.File{PublicID: "pdf-1", UserID: owner.ID, Bucket: "b", ObjectKey: "k", Kind: "pdf", ContentType: "x", OriginalName: "x"}
		assert.ErrorIs(t, repo.CreateFile(ctx, dup), repository.ErrConflict)

		all, total, err := repo.ListFilesForUser(ctx, owner.ID, "", 2, 0)
		require.NoError(t, err)
		assert.EqualValues(t, 3, total)
		require.Len(t, all, 2)
		assert.Equal(t, pdf2.PublicID, all[0].PublicID, "newest first")

		pdfs, total, err := repo.ListFilesForUser(ctx, owner.ID, "pdf", 10, 0)
		require.NoError(t, err)
		assert.EqualValues(t, 2, total)
		assert.Len(t, pdfs, 2)

		n, err := repo.CountFilesForUser(ctx, owner.ID)
		require.NoError(t, err)
		assert.EqualValues(t, 3, n)

		// Ownership: another user's file is invisible to owner-scoped lookups.
		_, err = repo.GetFileForUser(ctx, foreign.PublicID, owner.ID)
		assert.ErrorIs(t, err, repository.ErrNotFound)
		_, err = repo.UpdateFileTitle(ctx, foreign.PublicID, owner.ID, "mine now")
		assert.ErrorIs(t, err, repository.ErrNotFound)
		assert.ErrorIs(t, repo.DeleteFile(ctx, foreign.PublicID, owner.ID), repository.ErrNotFound)

		byIDs, err := repo.GetFilesByPublicIDs(ctx, owner.ID, []string{img.PublicID, foreign.PublicID, "missing"})
		require.NoError(t, err)
		require.Len(t, byIDs, 1, "only the owner's files resolve")
		assert.Equal(t, img.PublicID, byIDs[0].PublicID)

		renamed, err := repo.UpdateFileTitle(ctx, pdf1.PublicID, owner.ID, "Spring brochure")
		require.NoError(t, err)
		assert.Equal(t, "Spring brochure", renamed.Title)

		public, err := repo.GetFileByPublicID(ctx, pdf1.PublicID)
		require.NoError(t, err)
		assert.Equal(t, owner.ID, public.UserID)

		require.NoError(t, repo.DeleteFile(ctx, pdf1.PublicID, owner.ID))
		_, err = repo.GetFileByPublicID(ctx, pdf1.PublicID)
		assert.ErrorIs(t, err, repository.ErrNotFound)
	})

	t.Run("List Leads For User", func(t *testing.T) {
		owner := &models.User{Username: "pager", PasswordHash: "hash"}
		require.NoError(t, repo.CreateUser(ctx, owner))
		cardA := &models.Profile{UserID: owner.ID, Slug: "pager-a", Data: json.RawMessage(`{}`)}
		cardB := &models.Profile{UserID: owner.ID, Slug: "pager-b", Data: json.RawMessage(`{}`)}
		require.NoError(t, repo.CreateProfile(ctx, cardA))
		require.NoError(t, repo.CreateProfile(ctx, cardB))

		stranger := &models.User{Username: "stranger", PasswordHash: "hash"}
		require.NoError(t, repo.CreateUser(ctx, stranger))
		foreign := &models.Profile{UserID: stranger.ID, Slug: "foreign", Data: json.RawMessage(`{}`)}
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

		all, total, err := repo.ListLeadsForUser(ctx, owner.ID, repository.LeadFilter{Limit: 3})
		require.NoError(t, err)
		assert.EqualValues(t, 7, total, "excludes the stranger's lead")
		require.Len(t, all, 3)
		assert.Equal(t, created[6].ID, all[0].ID, "newest first")

		last, total, err := repo.ListLeadsForUser(ctx, owner.ID, repository.LeadFilter{Limit: 3, Offset: 6})
		require.NoError(t, err)
		assert.EqualValues(t, 7, total)
		require.Len(t, last, 1)
		assert.Equal(t, created[0].ID, last[0].ID)

		beyond, total, err := repo.ListLeadsForUser(ctx, owner.ID, repository.LeadFilter{Limit: 3, Offset: 30})
		require.NoError(t, err)
		assert.EqualValues(t, 7, total, "total is still reported past the last page")
		assert.Empty(t, beyond)

		onlyB, total, err := repo.ListLeadsForUser(ctx, owner.ID, repository.LeadFilter{ProfileID: cardB.ID, Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 2, total)
		for _, l := range onlyB {
			assert.Equal(t, cardB.ID, l.ProfileID)
		}

		_, total, err = repo.ListLeadsForUser(ctx, owner.ID, repository.LeadFilter{ProfileID: foreign.ID, Limit: 10})
		require.NoError(t, err)
		assert.Zero(t, total, "another user's profile yields nothing")

		found, total, err := repo.ListLeadsForUser(ctx, owner.ID, repository.LeadFilter{Search: "lead c", Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 1, total, "search is case-insensitive")
		require.Len(t, found, 1)
		assert.Equal(t, created[2].ID, found[0].ID)

		_, total, err = repo.ListLeadsForUser(ctx, owner.ID, repository.LeadFilter{Search: "%", Limit: 10})
		require.NoError(t, err)
		assert.Zero(t, total, "LIKE wildcards in the search are matched literally")

		_, total, err = repo.ListLeadsForUser(ctx, owner.ID, repository.LeadFilter{Since: created[5].CreatedAt, Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 2, total, "since is inclusive")
	})
}
