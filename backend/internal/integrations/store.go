package integrations

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
)

// Connection statuses.
const (
	StatusPending = "pending"
	StatusActive  = "active"
	StatusError   = "error"
)

// Activity kinds and outcomes.
const (
	ActivityPushLead  = "push_lead"
	ActivityTest      = "test"
	ActivitySetup     = "setup"
	ActivityProvision = "provision"
	ActivitySignIn    = "sign_in"

	OutcomeSuccess  = "success"
	OutcomeRetrying = "retrying"
	OutcomeFailed   = "failed"
)

// Connection is one organisation's or person's connection to a provider.
type Connection struct {
	ID    int64 `json:"id"`
	OrgID int64 `json:"-"`
	// UserID is set for a personal connection; 0 means the organisation's.
	UserID       int64          `json:"-"`
	Provider     string         `json:"provider"`
	Category     Category       `json:"category"`
	Name         string         `json:"name"`
	Enabled      bool           `json:"enabled"`
	Status       string         `json:"status"`
	Config       map[string]any `json:"config"`
	LastError    *string        `json:"last_error"`
	LastErrorAt  *time.Time     `json:"last_error_at"`
	FailureCount int            `json:"failure_count"`
	LastSyncedAt *time.Time     `json:"last_synced_at"`
	CreatedBy    *auth.UserRef  `json:"created_by"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`

	sealed []byte
	// token describes the newest inbound token, for scim_token providers.
	token *TokenInfo
}

// TokenInfo describes a token an outside service uses to call Fronko. The
// token itself is only shown when it's generated.
type TokenInfo struct {
	// Hint is the token's last characters.
	Hint       string     `json:"hint"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

// Scope is who the connection belongs to.
func (c *Connection) Scope() Scope {
	if c.UserID != 0 {
		return ScopeUser
	}
	return ScopeOrg
}

// Activity is one entry in a connection's log.
type Activity struct {
	ID        int64          `json:"id"`
	Kind      string         `json:"kind"`
	Outcome   string         `json:"outcome"`
	Summary   string         `json:"summary"`
	Detail    map[string]any `json:"detail,omitempty"`
	LeadID    *int64         `json:"lead_id"`
	Attempt   *int           `json:"attempt"`
	User      *auth.UserRef  `json:"user"`
	CreatedAt time.Time      `json:"created_at"`

	connectionID int64
	userID       int64
}

// Store runs the SQL for integrations.
type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

const connectionColumns = `c.connection_id, c.org_id, COALESCE(c.user_id, 0), c.provider, c.category, c.name,
	c.enabled, c.status, c.config, c.secrets, c.last_error, c.last_error_at, c.failure_count, c.last_synced_at,
	c.created_by, cu.username, c.created_at, c.updated_at, tk.hint, tk.created_at, tk.last_used_at`

const connectionFrom = `integration_connections c LEFT JOIN users cu ON cu.user_id = c.created_by
	LEFT JOIN LATERAL (SELECT t.hint, t.created_at, t.last_used_at FROM integration_tokens t
		WHERE t.connection_id = c.connection_id ORDER BY t.token_id DESC LIMIT 1) tk ON true`

func scanConnection(row pgx.Row) (*Connection, error) {
	var c Connection
	var createdBy *int64
	var createdByName, tokenHint *string
	var tokenCreated *time.Time
	var tokenUsed *time.Time
	err := row.Scan(&c.ID, &c.OrgID, &c.UserID, &c.Provider, &c.Category, &c.Name,
		&c.Enabled, &c.Status, &c.Config, &c.sealed, &c.LastError, &c.LastErrorAt, &c.FailureCount, &c.LastSyncedAt,
		&createdBy, &createdByName, &c.CreatedAt, &c.UpdatedAt, &tokenHint, &tokenCreated, &tokenUsed)
	if err != nil {
		return nil, database.MapError(err)
	}
	if tokenHint != nil && tokenCreated != nil {
		c.token = &TokenInfo{Hint: *tokenHint, CreatedAt: *tokenCreated, LastUsedAt: tokenUsed}
	}
	if createdBy != nil && createdByName != nil {
		c.CreatedBy = &auth.UserRef{ID: *createdBy, Username: *createdByName}
	}
	if c.Config == nil {
		c.Config = map[string]any{}
	}
	return &c, nil
}

// CreateConnection inserts c. seal is called with the new row's id (secrets
// are bound to it) and its result stored in the same transaction.
func (s *Store) CreateConnection(ctx context.Context, c *Connection, seal func(id int64) ([]byte, error)) error {
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO integration_connections (org_id, user_id, provider, category, name, enabled, status, config, created_by)
			VALUES ($1, NULLIF($2::bigint, 0), $3, $4, $5, $6, $7, $8, $9)
			RETURNING connection_id, created_at, updated_at`,
			c.OrgID, c.UserID, c.Provider, c.Category, c.Name, c.Enabled, c.Status, c.Config, createdByID(c),
		).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
		if err != nil {
			return database.MapError(err)
		}
		sealed, err := seal(c.ID)
		if err != nil {
			return err
		}
		c.sealed = sealed
		_, err = tx.Exec(ctx, `UPDATE integration_connections SET secrets = $2 WHERE connection_id = $1`, c.ID, sealed)
		return err
	})
}

func createdByID(c *Connection) *int64 {
	if c.CreatedBy == nil {
		return nil
	}
	return &c.CreatedBy.ID
}

// GetConnection returns a connection in the organisation.
func (s *Store) GetConnection(ctx context.Context, orgID, id int64) (*Connection, error) {
	return scanConnection(s.db.QueryRow(ctx, `SELECT `+connectionColumns+` FROM `+connectionFrom+`
		WHERE c.connection_id = $1 AND c.org_id = $2`, id, orgID))
}

// getConnectionByID loads a connection for background work, which has no
// organisation to scope by.
func (s *Store) getConnectionByID(ctx context.Context, id int64) (*Connection, error) {
	return scanConnection(s.db.QueryRow(ctx, `SELECT `+connectionColumns+` FROM `+connectionFrom+`
		WHERE c.connection_id = $1`, id))
}

// ConnectionFilter picks connections in an organisation.
type ConnectionFilter struct {
	// Org includes the organisation's connections.
	Org bool
	// UserID includes this person's connections (0: nobody's).
	UserID   int64
	Provider string
}

// ListConnections returns the matching connections, oldest first.
func (s *Store) ListConnections(ctx context.Context, orgID int64, f ConnectionFilter) ([]*Connection, error) {
	return s.listWhere(ctx, `c.org_id = $1
			AND (($2 AND c.user_id IS NULL) OR ($3::bigint <> 0 AND c.user_id = $3))
			AND ($4 = '' OR c.provider = $4)`, orgID, f.Org, f.UserID, f.Provider)
}

// CountConnections counts an owner's connections to a provider (userID 0:
// the organisation's).
func (s *Store) CountConnections(ctx context.Context, orgID, userID int64, provider string) (int, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM integration_connections
		WHERE org_id = $1 AND user_id IS NOT DISTINCT FROM NULLIF($2::bigint, 0) AND provider = $3`,
		orgID, userID, provider).Scan(&n)
	return n, err
}

// CountInCategory counts an owner's connections in a category (userID 0:
// the organisation's).
func (s *Store) CountInCategory(ctx context.Context, orgID, userID int64, category Category) (int, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM integration_connections
		WHERE org_id = $1 AND user_id IS NOT DISTINCT FROM NULLIF($2::bigint, 0) AND category = $3`,
		orgID, userID, category).Scan(&n)
	return n, err
}

// FindConnections returns the connections in a category that apply to a
// person: the organisation's and, when userID is set, their own. Only
// enabled, active ones are returned unless all is set.
func (s *Store) FindConnections(ctx context.Context, orgID, userID int64, category Category, all bool) ([]*Connection, error) {
	return s.listWhere(ctx, `c.org_id = $1 AND c.category = $3 AND ($4 OR (c.enabled AND c.status = 'active'))
			AND (c.user_id IS NULL OR ($2::bigint <> 0 AND c.user_id = $2))`, orgID, userID, category, all)
}

// connectionsInCategory returns every enabled, active connection in a
// category across the organisation, personal ones included.
func (s *Store) connectionsInCategory(ctx context.Context, orgID int64, category Category) ([]*Connection, error) {
	return s.listWhere(ctx, `c.org_id = $1 AND c.category = $2 AND c.enabled AND c.status = 'active'`, orgID, category)
}

func (s *Store) listWhere(ctx context.Context, where string, args ...any) ([]*Connection, error) {
	rows, err := s.db.Query(ctx, `SELECT `+connectionColumns+` FROM `+connectionFrom+`
		WHERE `+where+` ORDER BY c.created_at, c.connection_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Connection{}
	for rows.Next() {
		c, err := scanConnection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// addToken stores a new inbound token for a connection and removes its
// older ones, so rotating a token revokes the previous one at once.
func (s *Store) addToken(ctx context.Context, connectionID int64, hash []byte, hint string, createdBy int64) error {
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM integration_tokens WHERE connection_id = $1`, connectionID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO integration_tokens (connection_id, token_hash, hint, created_by)
			VALUES ($1, $2, $3, NULLIF($4::bigint, 0))`, connectionID, hash, hint, createdBy)
		return database.MapError(err)
	})
}

// connectionByToken returns the connection a token belongs to, and records
// that the token was used (at most once a minute, to spare writes).
func (s *Store) connectionByToken(ctx context.Context, hash []byte) (*Connection, error) {
	var id int64
	if err := s.db.QueryRow(ctx, `SELECT connection_id FROM integration_tokens WHERE token_hash = $1`, hash).Scan(&id); err != nil {
		return nil, database.MapError(err)
	}
	if _, err := s.db.Exec(ctx, `UPDATE integration_tokens SET last_used_at = now()
		WHERE token_hash = $1 AND (last_used_at IS NULL OR last_used_at < now() - interval '1 minute')`, hash); err != nil {
		return nil, err
	}
	return s.getConnectionByID(ctx, id)
}

// UpdateConnection saves a connection's editable state.
func (s *Store) UpdateConnection(ctx context.Context, c *Connection) error {
	err := s.db.QueryRow(ctx, `
		UPDATE integration_connections SET
			name = $3, enabled = $4, status = $5, config = $6, secrets = $7,
			last_error = $8, last_error_at = $9, failure_count = $10, updated_at = now()
		WHERE connection_id = $1 AND org_id = $2
		RETURNING updated_at`,
		c.ID, c.OrgID, c.Name, c.Enabled, c.Status, c.Config, c.sealed, c.LastError, c.LastErrorAt, c.FailureCount,
	).Scan(&c.UpdatedAt)
	return database.MapError(err)
}

// saveSecrets replaces a connection's sealed secrets, e.g. after a token refresh.
func (s *Store) saveSecrets(ctx context.Context, id int64, sealed []byte) error {
	return database.ExecOne(ctx, s.db, `UPDATE integration_connections SET secrets = $2 WHERE connection_id = $1`, id, sealed)
}

// DeleteConnection removes a connection and its activity.
func (s *Store) DeleteConnection(ctx context.Context, orgID, id int64) error {
	return database.ExecOne(ctx, s.db, `DELETE FROM integration_connections WHERE connection_id = $1 AND org_id = $2`, id, orgID)
}

// markSuccess records that a connection just worked.
func (s *Store) markSuccess(ctx context.Context, id int64, synced bool) error {
	_, err := s.db.Exec(ctx, `UPDATE integration_connections SET
			status = 'active', failure_count = 0, last_error = NULL, last_error_at = NULL,
			last_synced_at = CASE WHEN $2 THEN now() ELSE last_synced_at END
		WHERE connection_id = $1 AND status <> 'pending'`, id, synced)
	return err
}

// markFailure records a failure that retrying didn't fix. After threshold of
// them in a row the connection goes into error and stops receiving work.
func (s *Store) markFailure(ctx context.Context, id int64, msg string, threshold int) (status string, err error) {
	err = s.db.QueryRow(ctx, `UPDATE integration_connections SET
			failure_count = failure_count + 1, last_error = $2, last_error_at = now(),
			status = CASE WHEN failure_count + 1 >= $3 AND status = 'active' THEN 'error' ELSE status END
		WHERE connection_id = $1
		RETURNING status`, id, msg, threshold).Scan(&status)
	return status, database.MapError(err)
}

// leadSyncTargets returns the enabled, working lead sync connections that
// should receive a lead: the organisation's, plus the card holder's own.
func (s *Store) leadSyncTargets(ctx context.Context, q database.Querier, orgID, holderID int64) ([]int64, error) {
	rows, err := q.Query(ctx, `SELECT connection_id FROM integration_connections
		WHERE org_id = $1 AND category = 'lead_sync' AND enabled AND status = 'active'
			AND (user_id IS NULL OR ($2::bigint <> 0 AND user_id = $2))
		ORDER BY connection_id`, orgID, holderID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[int64])
}

// addActivity appends to a connection's log.
func (s *Store) addActivity(ctx context.Context, a *Activity) error {
	var detail []byte
	if len(a.Detail) > 0 {
		var err error
		if detail, err = json.Marshal(a.Detail); err != nil {
			return err
		}
	}
	return s.db.QueryRow(ctx, `
		INSERT INTO integration_activity (connection_id, kind, outcome, summary, detail, lead_id, attempt, user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8::bigint, 0))
		RETURNING activity_id, created_at`,
		a.connectionID, a.Kind, a.Outcome, a.Summary, detail, a.LeadID, a.Attempt, a.userID,
	).Scan(&a.ID, &a.CreatedAt)
}

// ListActivity returns a connection's log, newest first, older than
// beforeID when it is set.
func (s *Store) ListActivity(ctx context.Context, connectionID, beforeID int64, limit int) ([]*Activity, error) {
	rows, err := s.db.Query(ctx, `
		SELECT a.activity_id, a.kind, a.outcome, a.summary, a.detail, a.lead_id, a.attempt, a.user_id, u.username, a.created_at
		FROM integration_activity a LEFT JOIN users u ON u.user_id = a.user_id
		WHERE a.connection_id = $1 AND ($2::bigint = 0 OR a.activity_id < $2)
		ORDER BY a.activity_id DESC
		LIMIT $3`, connectionID, beforeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Activity{}
	for rows.Next() {
		var a Activity
		var userID *int64
		var username *string
		if err := rows.Scan(&a.ID, &a.Kind, &a.Outcome, &a.Summary, &a.Detail, &a.LeadID, &a.Attempt,
			&userID, &username, &a.CreatedAt); err != nil {
			return nil, err
		}
		if userID != nil && username != nil {
			a.User = &auth.UserRef{ID: *userID, Username: *username}
		}
		out = append(out, &a)
	}
	return out, rows.Err()
}

// purgeActivity deletes log entries older than before.
func (s *Store) purgeActivity(ctx context.Context, before time.Time) (int64, error) {
	tag, err := s.db.Exec(ctx, `DELETE FROM integration_activity WHERE created_at < $1`, before)
	return tag.RowsAffected(), err
}
