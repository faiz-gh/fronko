package analytics

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
)

// Store runs the SQL for card analytics events and reports.
type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// CardEvent is one thing a visitor did on a public card.
type CardEvent struct {
	Type   string
	Target string
	Label  string
	Value  int
}

// EventBatch is a set of events from one visit, all sharing its context.
type EventBatch struct {
	ProfileID int64
	// SessionID is the visit's UUID, or "" when the client didn't send one.
	SessionID   string
	VisitorHash []byte
	Source      string
	Device      string
	Referrer    string
	Events      []CardEvent
}

// RecordCardEvents stores a visit's events against the card, noting who holds
// it right now. Cards of suspended organisations record nothing.
func (r *Store) RecordCardEvents(ctx context.Context, b EventBatch) error {
	if len(b.Events) == 0 {
		return nil
	}
	types := make([]string, len(b.Events))
	targets := make([]string, len(b.Events))
	labels := make([]string, len(b.Events))
	values := make([]int32, len(b.Events))
	for i, e := range b.Events {
		types[i], targets[i], labels[i], values[i] = e.Type, e.Target, e.Label, int32(e.Value)
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO card_events (org_id, profile_id, assigned_user_id, session_id, visitor_hash, source, device, referrer_host,
		                         type, target, label, value)
		SELECT p.org_id, p.profile_id, p.assigned_user_id, NULLIF($2, '')::uuid, $3, $4, $5, $6,
		       t.type, t.target, t.label, t.value
		FROM profiles p
		JOIN organizations o ON o.org_id = p.org_id AND o.suspended_at IS NULL
		CROSS JOIN unnest($7::text[], $8::text[], $9::text[], $10::int[]) AS t(type, target, label, value)
		WHERE p.profile_id = $1`,
		b.ProfileID, b.SessionID, b.VisitorHash, b.Source, b.Device, b.Referrer, types, targets, labels, values)
	return database.MapError(err)
}

// AnalyticsSalt returns the random salt for visitor hashes on the given (UTC)
// day, creating it on first use.
func (r *Store) AnalyticsSalt(ctx context.Context, day time.Time) ([]byte, error) {
	fresh := make([]byte, 32)
	if _, err := rand.Read(fresh); err != nil {
		return nil, err
	}
	// The insert's row isn't visible to the second SELECT, so exactly one branch returns.
	var salt []byte
	err := r.db.QueryRow(ctx, `
		WITH ins AS (
			INSERT INTO analytics_salts (day, salt) VALUES ($1, $2)
			ON CONFLICT (day) DO NOTHING RETURNING salt)
		SELECT salt FROM ins
		UNION ALL
		SELECT salt FROM analytics_salts WHERE day = $1
		LIMIT 1`, day.UTC().Format(time.DateOnly), fresh).Scan(&salt)
	return salt, database.MapError(err)
}

// PurgeAnalytics deletes events older than before, and every salt but today's
// and yesterday's, so old visitor hashes can't be recomputed from an address.
func (r *Store) PurgeAnalytics(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM card_events WHERE created_at < $1`, before)
	if err != nil {
		return 0, err
	}
	if _, err := r.db.Exec(ctx, `DELETE FROM analytics_salts WHERE day < (now() AT TIME ZONE 'UTC')::date - 1`); err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// KnowsTimeZone reports whether the database recognises a time zone name.
// Browsers sometimes report legacy names (Asia/Calcutta) that newer zone data drops.
func (r *Store) KnowsTimeZone(ctx context.Context, name string) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_timezone_names WHERE name = $1)`, name).Scan(&ok)
	return ok, err
}

// AnalyticsFilter narrows card activity. Zero ids mean "no filter".
type AnalyticsFilter struct {
	From, To  time.Time
	ProfileID int64
	// UserID keeps activity that happened while this user held the card.
	UserID int64
	// TeamID keeps activity that happened while someone in this team held the card.
	TeamID int64
	// TZ is the IANA time zone days and hours are counted in.
	TZ string
}

// args are the parameters every scoped analytics query starts with ($1–$7).
func (f AnalyticsFilter) args(scope auth.Scope) []any {
	return []any{scope.OrgID, scope.MemberID(), f.From, f.To, f.ProfileID, f.UserID, f.TeamID}
}

// eventScope limits card_events (as alias a) to the scope and filter in $1–$7.
// Like leads, an event belongs to whoever held the card when it happened.
func eventScope(a string) string {
	return fmt.Sprintf(`%[1]s.org_id = $1 AND `+auth.VisibleTo(a+".assigned_user_id", 2)+`
		AND %[1]s.created_at >= $3 AND %[1]s.created_at < $4
		AND ($5::bigint = 0 OR %[1]s.profile_id = $5)
		AND ($6::bigint = 0 OR %[1]s.assigned_user_id = $6)
		AND ($7::bigint = 0 OR %[1]s.assigned_user_id IN (SELECT tm.user_id FROM team_members tm WHERE tm.team_id = $7))`, a)
}

// leadScope is the same limit for leads (alias l, joined to profiles p).
const leadScope = `p.org_id = $1 AND ` + "%s" + `
		AND l.created_at >= $3 AND l.created_at < $4
		AND ($5::bigint = 0 OR l.profile_id = $5)
		AND ($6::bigint = 0 OR l.assigned_user_id = $6)
		AND ($7::bigint = 0 OR l.assigned_user_id IN (SELECT tm.user_id FROM team_members tm WHERE tm.team_id = $7))`

func leadWhere() string { return fmt.Sprintf(leadScope, auth.VisibleTo("l.assigned_user_id", 2)) }

// sessionStats groups a filtered event set (CTE "e") into visits.
const sessionStats = `
	s AS (
		SELECT e.session_id,
		       (array_agg(e.visitor_hash))[1] AS vh,
		       COALESCE(max(e.value) FILTER (WHERE e.type = 'scroll'), 0) AS depth,
		       COALESCE(max(e.value) FILTER (WHERE e.type = 'leave'), 0) AS time_ms,
		       bool_or(e.type IN ('click', 'doc_open', 'gallery_open', 'vcard', 'form_open', 'share')
		               OR (e.type = 'scroll' AND e.value >= 50)) AS engaged,
		       bool_or(e.type IN ('vcard', 'form_open')) AS acted
		FROM e
		WHERE e.session_id IS NOT NULL
		GROUP BY e.session_id
		HAVING bool_or(e.type = 'view'))`

// analyticsTotals reads the headline numbers for the filter's period.
func (r *Store) analyticsTotals(ctx context.Context, scope auth.Scope, f AnalyticsFilter) (AnalyticsTotals, error) {
	var t AnalyticsTotals
	args := f.args(scope)
	err := r.db.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE type = 'view'),
		       count(DISTINCT visitor_hash) FILTER (WHERE type = 'view'),
		       count(DISTINCT session_id) FILTER (WHERE type = 'view'),
		       count(*) FILTER (WHERE type = 'view' AND source = 'nfc'),
		       count(*) FILTER (WHERE type = 'view' AND source = 'qr'),
		       count(*) FILTER (WHERE type = 'view' AND source = 'link'),
		       count(*) FILTER (WHERE type = 'vcard'),
		       count(*) FILTER (WHERE type = 'form_open'),
		       count(*) FILTER (WHERE type = 'doc_open'),
		       count(*) FILTER (WHERE type = 'click'),
		       count(*) FILTER (WHERE type = 'gallery_open'),
		       count(*) FILTER (WHERE type = 'share')
		FROM card_events e WHERE `+eventScope("e"), args...,
	).Scan(&t.Views, &t.UniqueVisitors, &t.Sessions, &t.NFCViews, &t.QRViews, &t.LinkViews,
		&t.Saves, &t.FormOpens, &t.DocOpens, &t.Clicks, &t.GalleryOpens, &t.Shares)
	if err != nil {
		return t, fmt.Errorf("event totals: %w", err)
	}

	err = r.db.QueryRow(ctx, `
		WITH e AS (SELECT * FROM card_events e WHERE `+eventScope("e")+`),`+sessionStats+`
		SELECT count(*) FILTER (WHERE engaged), count(*) FILTER (WHERE acted),
		       COALESCE(avg(depth), 0)::float8,
		       COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY time_ms) FILTER (WHERE time_ms > 0), 0)::bigint,
		       (SELECT count(*) FROM (SELECT vh FROM s GROUP BY vh HAVING count(*) > 1) x)
		FROM s`, args...,
	).Scan(&t.EngagedSessions, &t.ActionSessions, &t.AvgScrollDepth, &t.MedianTimeMs, &t.RepeatVisitors)
	if err != nil {
		return t, fmt.Errorf("session totals: %w", err)
	}

	err = r.db.QueryRow(ctx, `
		SELECT count(*) FROM leads l JOIN profiles p ON p.profile_id = l.profile_id
		WHERE `+leadWhere(), args...).Scan(&t.Leads)
	if err != nil {
		return t, fmt.Errorf("lead totals: %w", err)
	}
	return t, nil
}

// AnalyticsSummary returns the period's totals, the same-length period before
// it, and how visits break down by device, lead source, time, depth and duration.
func (r *Store) AnalyticsSummary(ctx context.Context, scope auth.Scope, f AnalyticsFilter) (*AnalyticsSummary, error) {
	sum := &AnalyticsSummary{
		From:        f.From.Format(time.RFC3339),
		To:          f.To.Format(time.RFC3339),
		Devices:     map[string]int64{},
		LeadSources: map[string]int64{},
	}
	var err error
	if sum.Current, err = r.analyticsTotals(ctx, scope, f); err != nil {
		return nil, err
	}
	prev := f
	prev.From, prev.To = f.From.Add(-f.To.Sub(f.From)), f.From
	if sum.Previous, err = r.analyticsTotals(ctx, scope, prev); err != nil {
		return nil, err
	}

	args := f.args(scope)
	if err := r.collectCounts(ctx, sum.Devices, `
		SELECT device, count(*) FROM card_events e
		WHERE e.type = 'view' AND `+eventScope("e")+` GROUP BY device`, args...); err != nil {
		return nil, fmt.Errorf("devices: %w", err)
	}
	if err := r.collectCounts(ctx, sum.LeadSources, `
		SELECT COALESCE(l.source, 'unknown'), count(*) FROM leads l JOIN profiles p ON p.profile_id = l.profile_id
		WHERE `+leadWhere()+` GROUP BY 1`, args...); err != nil {
		return nil, fmt.Errorf("lead sources: %w", err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT extract(dow FROM e.created_at AT TIME ZONE $8)::int, extract(hour FROM e.created_at AT TIME ZONE $8)::int, count(*)
		FROM card_events e WHERE e.type = 'view' AND `+eventScope("e")+` GROUP BY 1, 2`, append(args, f.TZ)...)
	if err != nil {
		return nil, fmt.Errorf("heatmap: %w", err)
	}
	var dow, hour int
	var n int64
	if _, err := pgx.ForEachRow(rows, []any{&dow, &hour, &n}, func() error {
		if dow >= 0 && dow < 7 && hour >= 0 && hour < 24 {
			sum.Heatmap[dow][hour] = n
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("heatmap: %w", err)
	}

	var d, tb [5]int64
	err = r.db.QueryRow(ctx, `
		WITH e AS (SELECT * FROM card_events e WHERE `+eventScope("e")+`),`+sessionStats+`
		SELECT count(*) FILTER (WHERE depth < 25), count(*) FILTER (WHERE depth >= 25 AND depth < 50),
		       count(*) FILTER (WHERE depth >= 50 AND depth < 75), count(*) FILTER (WHERE depth >= 75 AND depth < 100),
		       count(*) FILTER (WHERE depth >= 100),
		       count(*) FILTER (WHERE time_ms > 0 AND time_ms < 10000), count(*) FILTER (WHERE time_ms >= 10000 AND time_ms < 30000),
		       count(*) FILTER (WHERE time_ms >= 30000 AND time_ms < 60000), count(*) FILTER (WHERE time_ms >= 60000 AND time_ms < 180000),
		       count(*) FILTER (WHERE time_ms >= 180000)
		FROM s`, args...,
	).Scan(&d[0], &d[1], &d[2], &d[3], &d[4], &tb[0], &tb[1], &tb[2], &tb[3], &tb[4])
	if err != nil {
		return nil, fmt.Errorf("distributions: %w", err)
	}
	sum.ScrollDepths, sum.TimeBuckets = d, tb
	return sum, nil
}

func (r *Store) collectCounts(ctx context.Context, into map[string]int64, sql string, args ...any) error {
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return err
	}
	var key string
	var n int64
	_, err = pgx.ForEachRow(rows, []any{&key, &n}, func() error {
		into[key] = n
		return nil
	})
	return err
}

// AnalyticsTimeseries returns one point per day of the period, in the filter's
// time zone, including days with no activity.
func (r *Store) AnalyticsTimeseries(ctx context.Context, scope auth.Scope, f AnalyticsFilter) ([]AnalyticsPoint, error) {
	rows, err := r.db.Query(ctx, `
		WITH days AS (
			SELECT d::date AS day
			FROM generate_series(($3::timestamptz AT TIME ZONE $8)::date,
			                     (($4::timestamptz - interval '1 microsecond') AT TIME ZONE $8)::date,
			                     interval '1 day') d),
		ev AS (
			SELECT (e.created_at AT TIME ZONE $8)::date AS day,
			       count(*) FILTER (WHERE e.type = 'view') AS views,
			       count(*) FILTER (WHERE e.type = 'view' AND e.source = 'nfc') AS nfc,
			       count(*) FILTER (WHERE e.type = 'view' AND e.source = 'qr') AS qr,
			       count(*) FILTER (WHERE e.type = 'view' AND e.source = 'link') AS link,
			       count(DISTINCT e.visitor_hash) FILTER (WHERE e.type = 'view') AS uniq,
			       count(*) FILTER (WHERE e.type = 'vcard') AS saves
			FROM card_events e WHERE `+eventScope("e")+` GROUP BY 1),
		ld AS (
			SELECT (l.created_at AT TIME ZONE $8)::date AS day, count(*) AS leads
			FROM leads l JOIN profiles p ON p.profile_id = l.profile_id
			WHERE `+leadWhere()+` GROUP BY 1)
		SELECT to_char(days.day, 'YYYY-MM-DD'), COALESCE(ev.views, 0), COALESCE(ev.nfc, 0), COALESCE(ev.qr, 0),
		       COALESCE(ev.link, 0), COALESCE(ev.uniq, 0), COALESCE(ev.saves, 0), COALESCE(ld.leads, 0)
		FROM days LEFT JOIN ev USING (day) LEFT JOIN ld USING (day)
		ORDER BY days.day`, append(f.args(scope), f.TZ)...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (AnalyticsPoint, error) {
		var p AnalyticsPoint
		err := row.Scan(&p.Date, &p.Views, &p.NFCViews, &p.QRViews, &p.LinkViews, &p.UniqueVisitors, &p.Saves, &p.Leads)
		return p, err
	})
}

// AnalyticsContent returns the most used links, quick actions, documents and
// images. Links are grouped by URL and documents by file, so the same brochure
// on many cards adds up.
func (r *Store) AnalyticsContent(ctx context.Context, scope auth.Scope, f AnalyticsFilter) ([]ContentStat, error) {
	rows, err := r.db.Query(ctx, `
		SELECT e.type, e.target, (array_agg(e.label ORDER BY e.created_at DESC))[1], count(*), count(DISTINCT e.visitor_hash)
		FROM card_events e
		WHERE e.type IN ('click', 'doc_open', 'gallery_open') AND e.target <> '' AND `+eventScope("e")+`
		GROUP BY e.type, e.target
		ORDER BY count(*) DESC, e.target
		LIMIT 100`, f.args(scope)...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (ContentStat, error) {
		var c ContentStat
		err := row.Scan(&c.Type, &c.Target, &c.Label, &c.Count, &c.Unique)
		return c, err
	})
}

// AnalyticsCards returns every card the scope can see that matches the filter,
// with its activity in the period; cards nobody visited are included.
func (r *Store) AnalyticsCards(ctx context.Context, scope auth.Scope, f AnalyticsFilter) ([]CardStat, error) {
	rows, err := r.db.Query(ctx, `
		SELECT p.profile_id, p.slug, COALESCE(p.data->>'name', ''), p.assigned_user_id, au.username,
		       COALESCE(st.views, 0), COALESCE(st.uniq, 0), COALESCE(st.nfc, 0), COALESCE(st.qr, 0), COALESCE(st.link, 0),
		       COALESCE(st.saves, 0), COALESCE(st.forms, 0), COALESCE(ld.n, 0), COALESCE(st.docs, 0),
		       to_char((SELECT max(x.created_at) FROM card_events x WHERE x.profile_id = p.profile_id AND x.type = 'view')
		               AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM profiles p
		LEFT JOIN users au ON au.user_id = p.assigned_user_id
		LEFT JOIN LATERAL (
			SELECT count(*) FILTER (WHERE e.type = 'view') AS views,
			       count(DISTINCT e.visitor_hash) FILTER (WHERE e.type = 'view') AS uniq,
			       count(*) FILTER (WHERE e.type = 'view' AND e.source = 'nfc') AS nfc,
			       count(*) FILTER (WHERE e.type = 'view' AND e.source = 'qr') AS qr,
			       count(*) FILTER (WHERE e.type = 'view' AND e.source = 'link') AS link,
			       count(*) FILTER (WHERE e.type = 'vcard') AS saves,
			       count(*) FILTER (WHERE e.type = 'form_open') AS forms,
			       count(*) FILTER (WHERE e.type = 'doc_open') AS docs
			FROM card_events e WHERE e.profile_id = p.profile_id AND `+eventScope("e")+`) st ON true
		LEFT JOIN LATERAL (
			SELECT count(*) AS n FROM leads l
			WHERE l.profile_id = p.profile_id AND `+fmt.Sprintf(leadScope, auth.VisibleTo("l.assigned_user_id", 2))+`) ld ON true
		WHERE p.org_id = $1 AND `+auth.VisibleTo("p.assigned_user_id", 2)+`
		  AND ($5::bigint = 0 OR p.profile_id = $5)
		  AND ($6::bigint = 0 OR p.assigned_user_id = $6)
		  AND ($7::bigint = 0 OR p.assigned_user_id IN (SELECT tm.user_id FROM team_members tm WHERE tm.team_id = $7))
		ORDER BY COALESCE(st.views, 0) DESC, p.created_at DESC`, f.args(scope)...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (CardStat, error) {
		var c CardStat
		var userID *int64
		var username *string
		err := row.Scan(&c.ProfileID, &c.Slug, &c.Name, &userID, &username,
			&c.Views, &c.UniqueVisitors, &c.NFCViews, &c.QRViews, &c.LinkViews,
			&c.Saves, &c.FormOpens, &c.Leads, &c.DocOpens, &c.LastViewedAt)
		if userID != nil && username != nil {
			c.AssignedUser = &auth.UserRef{ID: *userID, Username: *username}
		}
		return c, err
	})
}

// AnalyticsTeams compares teams over the period and the one before it.
// leadUserID limits the result to the teams that user leads (0: every team);
// teamID to one team (0: all).
func (r *Store) AnalyticsTeams(ctx context.Context, orgID, leadUserID, teamID int64, from, to time.Time) ([]TeamStat, error) {
	prevFrom := from.Add(-to.Sub(from))
	rows, err := r.db.Query(ctx, `
		WITH t AS (
			SELECT team_id, name, COALESCE(color, '') AS color FROM teams
			WHERE org_id = $1
			  AND ($2::bigint = 0 OR team_id IN (SELECT team_id FROM team_members WHERE user_id = $2 AND role = 'lead'))
			  AND ($6::bigint = 0 OR team_id = $6)),
		m AS (SELECT tm.team_id, tm.user_id FROM team_members tm JOIN t USING (team_id)),
		ev AS (
			SELECT m.team_id,
			       count(*) FILTER (WHERE e.type = 'view' AND e.created_at >= $3) AS views,
			       count(DISTINCT e.visitor_hash) FILTER (WHERE e.type = 'view' AND e.created_at >= $3) AS uniq,
			       count(DISTINCT e.session_id) FILTER (WHERE e.type = 'view' AND e.created_at >= $3) AS sessions,
			       count(DISTINCT e.profile_id) FILTER (WHERE e.type = 'view' AND e.created_at >= $3) AS active,
			       count(*) FILTER (WHERE e.type = 'vcard' AND e.created_at >= $3) AS saves,
			       count(*) FILTER (WHERE e.type = 'form_open' AND e.created_at >= $3) AS forms,
			       count(*) FILTER (WHERE e.type = 'view' AND e.created_at < $3) AS pviews,
			       count(*) FILTER (WHERE e.type = 'vcard' AND e.created_at < $3) AS psaves
			FROM card_events e JOIN m ON m.user_id = e.assigned_user_id
			WHERE e.org_id = $1 AND e.created_at >= $5 AND e.created_at < $4
			GROUP BY m.team_id),
		eng AS (
			SELECT m.team_id, count(*) AS n
			FROM (
				SELECT e.session_id, e.assigned_user_id FROM card_events e
				WHERE e.org_id = $1 AND e.created_at >= $3 AND e.created_at < $4 AND e.session_id IS NOT NULL
				GROUP BY 1, 2
				HAVING bool_or(e.type = 'view')
				   AND bool_or(e.type IN ('click', 'doc_open', 'gallery_open', 'vcard', 'form_open', 'share')
				               OR (e.type = 'scroll' AND e.value >= 50))) s
			JOIN m ON m.user_id = s.assigned_user_id
			GROUP BY m.team_id),
		ld AS (
			SELECT m.team_id, count(*) FILTER (WHERE l.created_at >= $3) AS leads, count(*) FILTER (WHERE l.created_at < $3) AS pleads
			FROM leads l JOIN m ON m.user_id = l.assigned_user_id
			WHERE l.created_at >= $5 AND l.created_at < $4
			GROUP BY m.team_id),
		cards AS (
			SELECT m.team_id, count(DISTINCT p.profile_id) AS n
			FROM profiles p JOIN m ON m.user_id = p.assigned_user_id
			WHERE p.org_id = $1 GROUP BY m.team_id),
		mc AS (SELECT team_id, count(*) AS n FROM m GROUP BY team_id)
		SELECT t.team_id, t.name, t.color, COALESCE(mc.n, 0), COALESCE(cards.n, 0), COALESCE(ev.active, 0),
		       COALESCE(ev.views, 0), COALESCE(ev.uniq, 0), COALESCE(ev.sessions, 0), COALESCE(eng.n, 0),
		       COALESCE(ev.saves, 0), COALESCE(ev.forms, 0), COALESCE(ld.leads, 0),
		       COALESCE(ev.pviews, 0), COALESCE(ev.psaves, 0), COALESCE(ld.pleads, 0)
		FROM t
		LEFT JOIN mc USING (team_id) LEFT JOIN cards USING (team_id) LEFT JOIN ev USING (team_id)
		LEFT JOIN eng USING (team_id) LEFT JOIN ld USING (team_id)
		ORDER BY COALESCE(ev.views, 0) DESC, lower(t.name)`, orgID, leadUserID, from, to, prevFrom, teamID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (TeamStat, error) {
		var t TeamStat
		err := row.Scan(&t.ID, &t.Name, &t.Color, &t.Members, &t.Cards, &t.ActiveCards,
			&t.Views, &t.UniqueVisitors, &t.Sessions, &t.Engaged, &t.Saves, &t.FormOpens, &t.Leads,
			&t.PrevViews, &t.PrevSaves, &t.PrevLeads)
		return t, err
	})
}

// AnalyticsMembers ranks the people the scope can see (optionally one team's)
// by their cards' activity in the period.
func (r *Store) AnalyticsMembers(ctx context.Context, scope auth.Scope, teamID int64, from, to time.Time) ([]MemberStat, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.user_id, u.username,
		       (SELECT count(*) FROM profiles p WHERE p.org_id = $1 AND p.assigned_user_id = u.user_id),
		       COALESCE(st.views, 0), COALESCE(st.uniq, 0), COALESCE(st.saves, 0), COALESCE(st.forms, 0), COALESCE(st.docs, 0),
		       (SELECT count(*) FROM leads l WHERE l.assigned_user_id = u.user_id AND l.created_at >= $3 AND l.created_at < $4)
		FROM users u
		LEFT JOIN LATERAL (
			SELECT count(*) FILTER (WHERE e.type = 'view') AS views,
			       count(DISTINCT e.visitor_hash) FILTER (WHERE e.type = 'view') AS uniq,
			       count(*) FILTER (WHERE e.type = 'vcard') AS saves,
			       count(*) FILTER (WHERE e.type = 'form_open') AS forms,
			       count(*) FILTER (WHERE e.type = 'doc_open') AS docs
			FROM card_events e
			WHERE e.org_id = $1 AND e.assigned_user_id = u.user_id AND e.created_at >= $3 AND e.created_at < $4) st ON true
		WHERE u.org_id = $1 AND `+auth.VisibleTo("u.user_id", 2)+`
		  AND ($5::bigint = 0 OR u.user_id IN (SELECT tm.user_id FROM team_members tm WHERE tm.team_id = $5))
		ORDER BY COALESCE(st.views, 0) DESC, lower(u.username)
		LIMIT 200`, scope.OrgID, scope.MemberID(), from, to, teamID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (MemberStat, error) {
		var m MemberStat
		err := row.Scan(&m.UserID, &m.Username, &m.Cards, &m.Views, &m.UniqueVisitors, &m.Saves, &m.FormOpens, &m.DocOpens, &m.Leads)
		return m, err
	})
}

// AnalyticsActivity returns the latest notable events in the filter: visits,
// contact saves, sent forms and brochure opens, newest first.
func (r *Store) AnalyticsActivity(ctx context.Context, scope auth.Scope, f AnalyticsFilter, limit int) ([]ActivityItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT e.type, e.source, e.label, e.profile_id, COALESCE(p.data->>'name', ''), p.slug, e.assigned_user_id, u.username,
		       to_char(e.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM card_events e
		JOIN profiles p ON p.profile_id = e.profile_id
		LEFT JOIN users u ON u.user_id = e.assigned_user_id
		WHERE e.type IN ('view', 'vcard', 'form_submit', 'doc_open') AND `+eventScope("e")+`
		ORDER BY e.created_at DESC, e.event_id DESC
		LIMIT $8`, append(f.args(scope), limit)...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (ActivityItem, error) {
		var a ActivityItem
		var userID *int64
		var username *string
		err := row.Scan(&a.Type, &a.Source, &a.Label, &a.ProfileID, &a.CardName, &a.Slug, &userID, &username, &a.CreatedAt)
		if userID != nil && username != nil {
			a.AssignedUser = &auth.UserRef{ID: *userID, Username: *username}
		}
		return a, err
	})
}
