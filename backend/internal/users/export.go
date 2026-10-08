package users

import (
	"context"
	"encoding/json"

	"github.com/faiz-gh/fronko/backend/internal/platform/database"
)

// exportQuery gathers everything Fronko holds about one person, as JSON:
// their account, organisation and teams, the cards they made or hold, the
// leads that arrived on cards they held, their files (details, not
// contents), their personal integrations (never secrets) and the feedback
// they sent. Analytics are anonymous visit counts, not about them, so
// they're left out.
const exportQuery = `
	SELECT jsonb_build_object(
		'account', jsonb_build_object(
			'id', u.user_id, 'username', u.username, 'email', u.email, 'full_name', u.full_name,
			'role', u.role, 'email_verified_at', u.email_verified_at, 'last_login_at', u.last_login_at,
			'provisioned_by', u.provisioned_by, 'created_at', u.created_at, 'updated_at', u.updated_at),
		'organisation', jsonb_build_object('name', o.name, 'handle', o.handle),
		'teams', COALESCE((
			SELECT jsonb_agg(jsonb_build_object('name', t.name, 'role', tm.role, 'added_at', tm.added_at) ORDER BY t.name)
			FROM team_members tm JOIN teams t ON t.team_id = tm.team_id WHERE tm.user_id = u.user_id), '[]'),
		'cards', COALESCE((
			SELECT jsonb_agg(jsonb_build_object(
				'id', p.profile_id, 'slug', p.slug, 'held_by_you', p.assigned_user_id = u.user_id,
				'made_by_you', p.user_id = u.user_id, 'data', p.data,
				'created_at', p.created_at, 'updated_at', p.updated_at) ORDER BY p.profile_id)
			FROM profiles p WHERE p.user_id = u.user_id OR p.assigned_user_id = u.user_id), '[]'),
		'leads', COALESCE((
			SELECT jsonb_agg(jsonb_build_object(
				'name', l.name, 'email', l.email, 'phone_country_code', l.phone_country_code,
				'phone_number', l.phone_number, 'message', l.notes, 'source', l.source, 'card', p.slug,
				'received_at', l.created_at)
				ORDER BY l.created_at)
			FROM leads l JOIN profiles p ON p.profile_id = l.profile_id WHERE l.assigned_user_id = u.user_id), '[]'),
		'files', COALESCE((
			SELECT jsonb_agg(jsonb_build_object(
				'id', f.public_id, 'name', f.original_name, 'title', f.title, 'area', f.area,
				'purpose', f.purpose, 'content_type', f.content_type, 'size_bytes', f.size_bytes,
				'created_at', f.created_at) ORDER BY f.created_at)
			FROM files f WHERE f.user_id = u.user_id), '[]'),
		'integrations', COALESCE((
			SELECT jsonb_agg(jsonb_build_object(
				'provider', c.provider, 'name', c.name, 'category', c.category, 'created_at', c.created_at)
				ORDER BY c.created_at)
			FROM integration_connections c WHERE c.user_id = u.user_id), '[]'),
		'feedback', COALESCE((
			SELECT jsonb_agg(jsonb_build_object(
				'category', fb.category, 'rating', fb.rating, 'message', fb.message,
				'status', fb.status, 'sent_at', fb.created_at) ORDER BY fb.created_at)
			FROM feedback fb WHERE fb.user_id = u.user_id), '[]'))
	FROM users u JOIN organizations o ON o.org_id = u.org_id
	WHERE u.user_id = $1`

// ExportUserData returns everything held about the user as one JSON
// document, for the right of access and data portability.
func (r *Store) ExportUserData(ctx context.Context, userID int64) (json.RawMessage, error) {
	var doc json.RawMessage
	if err := r.db.QueryRow(ctx, exportQuery, userID).Scan(&doc); err != nil {
		return nil, database.MapError(err)
	}
	return doc, nil
}
