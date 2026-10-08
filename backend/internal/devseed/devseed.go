// Package devseed fills an empty development database with a demo
// organisation, so a fresh checkout has something to look at:
//
//	FRONKO_ENV=development fronko seed [--reset]
//
// It goes through the modules' own stores where it can, so the data is
// shaped exactly as the app would make it. Leads and analytics events are
// backdated with plain SQL, which the app never does.
package devseed

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	mrand "math/rand/v2"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/cards"
	"github.com/faiz-gh/fronko/backend/internal/leads"
	"github.com/faiz-gh/fronko/backend/internal/orgs"
	"github.com/faiz-gh/fronko/backend/internal/platform/config"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platformadmin"
	"github.com/faiz-gh/fronko/backend/internal/teams"
	"github.com/faiz-gh/fronko/backend/internal/users"
)

const usage = `usage: FRONKO_ENV=development fronko seed [--reset]

Creates demo organisations, users, cards, leads and analytics in an empty
database. --reset first deletes everything in it.`

// Accounts the seed creates. They are documented in CONTRIBUTING.md.
const (
	OwnerUsername  = "uidemo"
	OwnerPassword  = "password123"
	MemberUsername = "uidemo-rep"
	MemberPassword = "reppassword1"
	EmptyUsername  = "uidemo-empty"
	AdminEmail     = "admin@fronko.local"
	AdminPassword  = "adminpassword123"
)

// tables lists every table, for --reset.
const tables = `organizations, users, email_codes, teams, team_members, profiles, leads, user_storage,
	files, file_grants, file_team_grants, file_refs, card_events, analytics_salts, platform_admins,
	admin_audit_log, feedback, feedback_replies, org_usage_snapshots, platform_usage_snapshots, jobs, integration_connections, integration_activity, integration_tokens, org_domains`

// RunCLI is `fronko seed`.
func RunCLI(args []string) error {
	fs := flag.NewFlagSet("seed", flag.ContinueOnError)
	reset := fs.Bool("reset", false, "delete every row in the database first")
	fs.Usage = func() { fmt.Fprintln(os.Stderr, usage) }
	if err := fs.Parse(args); err != nil {
		return err
	}
	env, err := config.Environment()
	if err != nil {
		return err
	}
	if env != config.EnvDevelopment {
		return errors.New("refusing to seed: set FRONKO_ENV=development (this writes demo accounts with known passwords)")
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return errors.New("DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := database.NewPool(ctx, dbURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if *reset {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE "+tables+" RESTART IDENTITY CASCADE"); err != nil {
			return fmt.Errorf("reset: %w", err)
		}
	} else {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM organizations`).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return errors.New("the database already has organisations; run with --reset to replace them")
		}
	}
	if err := Seed(ctx, pool); err != nil {
		return err
	}
	fmt.Printf(`Seeded. Sign in at /login as:
  %s / %s       (owner of "Lumen Labs")
  %s / %s  (member, lead of the Sales team)
  %s / %s  (owner of an empty organisation)
and at /admin/login as %s / %s.
`, OwnerUsername, OwnerPassword, MemberUsername, MemberPassword, EmptyUsername, OwnerPassword, AdminEmail, AdminPassword)
	return nil
}

// Seed writes the demo data. It expects an empty database.
func Seed(ctx context.Context, pool *pgxpool.Pool) error {
	var (
		userStore  = users.NewStore(pool)
		orgStore   = orgs.NewStore(pool)
		teamStore  = teams.NewStore(pool)
		cardStore  = cards.NewStore(pool)
		leadStore  = leads.NewStore(pool, nil) // no subscribers: demo leads sync nowhere
		adminStore = platformadmin.NewStore(pool)
		hasher     = auth.NewService("") // only bcrypt is used
	)
	hash := func(pw string) string {
		h, err := hasher.HashPassword(pw)
		if err != nil {
			panic(err)
		}
		return h
	}
	verified := func(u *users.User) error { return userStore.MarkEmailVerified(ctx, u.ID) }

	// Organisations and people.
	owner := &users.User{Username: OwnerUsername, Email: ptr("uidemo@example.com"), PasswordHash: hash(OwnerPassword)}
	if err := orgStore.CreateOrgWithOwner(ctx, "Lumen Labs", owner); err != nil {
		return fmt.Errorf("owner: %w", err)
	}
	if err := orgStore.SetOrgHandle(ctx, owner.OrgID, "uidemo"); err != nil {
		return err
	}
	rep := &users.User{
		OrgID: owner.OrgID, Role: auth.RoleMember, Username: MemberUsername, Email: ptr("rep@example.com"),
		PasswordHash: hash(MemberPassword), StorageQuotaBytes: ptr(int64(1 << 20)), CreatedBy: &owner.ID,
	}
	if err := orgStore.CreateMember(ctx, rep); err != nil {
		return fmt.Errorf("member: %w", err)
	}
	empty := &users.User{Username: EmptyUsername, Email: ptr("empty@example.com"), PasswordHash: hash(OwnerPassword)}
	if err := orgStore.CreateOrgWithOwner(ctx, EmptyUsername, empty); err != nil {
		return fmt.Errorf("empty org: %w", err)
	}
	for _, u := range []*users.User{owner, rep, empty} {
		if err := verified(u); err != nil {
			return err
		}
	}

	sales := &teams.Team{OrgID: owner.OrgID, Name: "Sales", Description: "Field sales and events", Color: "#2563eb"}
	if err := teamStore.CreateTeam(ctx, sales); err != nil {
		return err
	}
	if err := teamStore.ReplaceTeamMembers(ctx, owner.OrgID, sales.ID, []teams.TeamMembership{
		{UserID: rep.ID, Role: auth.TeamRoleLead},
	}); err != nil {
		return err
	}

	// A verified email domain (for single sign-on by email) and an
	// organisation booking page, which Maya's card shows. The Calendly link
	// is made up, so the button opens Calendly's "not found" page.
	if _, err := pool.Exec(ctx, `INSERT INTO org_domains (org_id, domain, verification_token, verified_at, created_by)
		VALUES ($1, 'lumenlabs.example', 'seeded', now(), $2)`, owner.OrgID, owner.ID); err != nil {
		return fmt.Errorf("domain: %w", err)
	}
	var bookingID int64
	if err := pool.QueryRow(ctx, `INSERT INTO integration_connections (org_id, provider, category, name, status, config, created_by)
		VALUES ($1, 'calendly', 'calendar', 'Intro call', 'active', '{"url": "https://calendly.com/lumenlabs-demo/intro"}', $2)
		RETURNING connection_id`, owner.OrgID, owner.ID).Scan(&bookingID); err != nil {
		return fmt.Errorf("booking page: %w", err)
	}

	if err := adminStore.CreatePlatformAdmin(ctx, &platformadmin.PlatformAdmin{Email: AdminEmail, PasswordHash: hash(AdminPassword)}); err != nil {
		return fmt.Errorf("platform admin: %w", err)
	}

	// Cards: one the organisation holds, one assigned to the rep, one blank.
	newCard := func(slug string, assignee *int64, data map[string]any) (*cards.Profile, error) {
		raw, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		p := &cards.Profile{OrgID: owner.OrgID, UserID: owner.ID, AssignedUserID: assignee, Slug: slug, Data: raw}
		return p, cardStore.CreateProfile(ctx, p)
	}
	maya, err := newCard("maya-chen", nil, map[string]any{
		"name": "Maya Chen", "title": "Head of Partnerships", "company": "Lumen Labs Inc.",
		"bio":   "I connect climate-tech startups with the people who can scale them. Always happy to grab a coffee after a panel.",
		"email": "maya@lumenlabs.example", "phone": "+1 415 555 0142", "website": "lumenlabs.example",
		"location": "San Francisco, CA", "theme": "light", "accent": "emerald", "collect_leads": true,
		"booking_connection_id": bookingID,
		"links": []map[string]string{
			{"id": "a", "url": "https://linkedin.com/in/mayachen", "label": ""},
			{"id": "b", "url": "https://x.com/mayachen", "label": ""},
		},
	})
	if err != nil {
		return fmt.Errorf("card: %w", err)
	}
	speaker, err := newCard("maya-speaker", &rep.ID, map[string]any{
		"name": "Maya Chen", "title": "Speaker", "company": "Climate Week 2026",
		"bio": "Slides and recordings from my talks.", "email": "talks@lumenlabs.example",
		"theme": "dark", "accent": "violet", "collect_leads": true,
		"links": []map[string]string{
			{"id": "a", "url": "https://youtube.com/@mayachen", "label": ""},
			{"id": "b", "url": "https://github.com/mayachen", "label": ""},
		},
	})
	if err != nil {
		return fmt.Errorf("card: %w", err)
	}
	if _, err := newCard("lumen-sales", nil, map[string]any{"name": "", "accent": "orange"}); err != nil {
		return fmt.Errorf("card: %w", err)
	}

	// Leads over the last six weeks, mostly on the main card.
	rng := mrand.New(mrand.NewPCG(42, 7))
	now := time.Now()
	for i := range 68 {
		card := maya
		if i%9 == 0 {
			card = speaker
		}
		first, last := firstNames[rng.IntN(len(firstNames))], lastNames[rng.IntN(len(lastNames))]
		lead := &leads.Lead{
			ProfileID: card.ID,
			Name:      first + " " + last,
			Email:     fmt.Sprintf("%s.%s%d@example.com", strings.ToLower(first), strings.ToLower(last), i),
			Notes:     notes[rng.IntN(len(notes))],
			Source:    sources[rng.IntN(len(sources))],
		}
		if i%3 == 0 {
			lead.PhoneCountryCode, lead.PhoneNumber = "+1", fmt.Sprintf("415555%04d", rng.IntN(10000))
		}
		if err := leadStore.CreateLead(ctx, lead); err != nil {
			return fmt.Errorf("lead: %w", err)
		}
		at := now.Add(-time.Duration(rng.IntN(42*24)) * time.Hour)
		if _, err := pool.Exec(ctx, `UPDATE leads SET created_at = $2 WHERE lead_id = $1`, lead.ID, at); err != nil {
			return err
		}
	}

	return seedVisits(ctx, pool, rng, owner.OrgID, []*cards.Profile{maya, speaker})
}

// seedVisits writes about 380 visits over the last 45 days, each a view plus
// a few of the things visitors do on a card.
func seedVisits(ctx context.Context, pool *pgxpool.Pool, rng *mrand.Rand, orgID int64, onCards []*cards.Profile) error {
	now := time.Now()
	type event struct {
		typ, target, label string
		value              int
	}
	for range 380 {
		card := onCards[rng.IntN(len(onCards))]
		at := now.Add(-time.Duration(rng.IntN(45*24*60)) * time.Minute)
		source := sources[rng.IntN(len(sources))]
		device := devices[rng.IntN(len(devices))]
		session := make([]byte, 16)
		hash := make([]byte, 32)
		rand.Read(session)
		rand.Read(hash)
		session[6] = session[6]&0x0f | 0x40
		session[8] = session[8]&0x3f | 0x80

		evs := []event{{typ: "view"}}
		if rng.IntN(3) == 0 {
			evs = append(evs, event{typ: "click", target: "https://linkedin.com/in/mayachen", label: "LinkedIn"})
		}
		if rng.IntN(4) == 0 {
			evs = append(evs, event{typ: "vcard"})
		}
		if rng.IntN(6) == 0 {
			evs = append(evs, event{typ: "form_open"})
		}
		evs = append(evs, event{typ: "scroll", value: 25 * (1 + rng.IntN(4))})
		evs = append(evs, event{typ: "leave", value: 5000 + rng.IntN(90000)})

		for i, e := range evs {
			if _, err := pool.Exec(ctx, `
				INSERT INTO card_events (org_id, profile_id, assigned_user_id, session_id, visitor_hash, type, source,
					target, label, value, device, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
				orgID, card.ID, card.AssignedUserID, fmt.Sprintf("%x-%x-%x-%x-%x", session[0:4], session[4:6], session[6:8], session[8:10], session[10:]),
				hash, e.typ, source, e.target, e.label, e.value, device, at.Add(time.Duration(i)*7*time.Second),
			); err != nil {
				return fmt.Errorf("event: %w", err)
			}
		}
	}
	return nil
}

var (
	firstNames = []string{"Alex", "Priya", "Sam", "Jordan", "Wei", "Fatima", "Lucas", "Amara", "Diego", "Hana", "Noah", "Zara"}
	lastNames  = []string{"Kim", "Patel", "Okafor", "Garcia", "Nguyen", "Rossi", "Schmidt", "Haddad", "Silva", "Tanaka"}
	notes      = []string{"", "", "Met at the climate panel.", "Interested in the partnership programme.", "Send the brochure, please!"}
	sources    = []string{"nfc", "nfc", "qr", "qr", "qr", "link"}
	devices    = []string{"mobile", "mobile", "mobile", "tablet", "desktop"}
)

func ptr[T any](v T) *T { return &v }
