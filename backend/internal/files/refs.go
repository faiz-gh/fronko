package files

import (
	"context"
	"encoding/json"
	"regexp"

	"github.com/faiz-gh/fronko/backend/internal/platform/database"
)

// Card slots that hold library files.
const (
	SlotAvatar   = "avatar"
	SlotCover    = "cover"
	SlotDocument = "document"
	SlotGallery  = "gallery"
)

// MaxCardFiles bounds how many files one card can reference: a photo, a
// cover, 10 brochures and a few galleries.
const MaxCardFiles = 64

// FileRef is one file a card uses, and where on the card.
type FileRef struct {
	PublicID string
	Slot     string
}

var publicIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{22}$`)

// CardFileRefs pulls the library files a card's data points at. The backend
// otherwise treats card data as opaque; these keys are the exception, so a
// public card only reveals files it uses and the library knows what's in use.
func CardFileRefs(data []byte) []FileRef {
	var card struct {
		AvatarFile string `json:"avatar_file"`
		CoverFile  string `json:"cover_file"`
		Documents  []struct {
			File string `json:"file"`
		} `json:"documents"`
		Blocks []struct {
			Images []struct {
				File string `json:"file"`
			} `json:"images"`
		} `json:"blocks"`
	}
	if err := json.Unmarshal(data, &card); err != nil {
		return nil
	}
	seen := map[FileRef]bool{}
	var refs []FileRef
	add := func(id, slot string) {
		ref := FileRef{id, slot}
		if publicIDPattern.MatchString(id) && !seen[ref] && len(refs) < MaxCardFiles {
			seen[ref] = true
			refs = append(refs, ref)
		}
	}
	add(card.AvatarFile, SlotAvatar)
	add(card.CoverFile, SlotCover)
	for _, d := range card.Documents {
		add(d.File, SlotDocument)
	}
	for _, b := range card.Blocks {
		for _, img := range b.Images {
			add(img.File, SlotGallery)
		}
	}
	return refs
}

// SyncRefs records exactly which of the organisation's files a card uses.
func SyncRefs(ctx context.Context, q database.Querier, profileID, orgID int64, data []byte) error {
	refs := CardFileRefs(data)
	ids, slots := make([]string, len(refs)), make([]string, len(refs))
	for i, ref := range refs {
		ids[i], slots[i] = ref.PublicID, ref.Slot
	}
	if _, err := q.Exec(ctx, `DELETE FROM file_refs WHERE profile_id = $1`, profileID); err != nil {
		return err
	}
	if len(refs) == 0 {
		return nil
	}
	_, err := q.Exec(ctx, `
		INSERT INTO file_refs (file_id, profile_id, slot)
		SELECT f.file_id, $1, r.slot
		FROM unnest($2::text[], $3::text[]) AS r(public_id, slot)
		JOIN files f ON f.public_id = r.public_id AND f.org_id = $4
		ON CONFLICT DO NOTHING`, profileID, ids, slots, orgID)
	return database.MapError(err)
}

// ReferencedIDs is the distinct library files a card's data points at.
func ReferencedIDs(data []byte) []string {
	seen := map[string]bool{}
	var ids []string
	for _, ref := range CardFileRefs(data) {
		if !seen[ref.PublicID] {
			seen[ref.PublicID] = true
			ids = append(ids, ref.PublicID)
		}
	}
	return ids
}
