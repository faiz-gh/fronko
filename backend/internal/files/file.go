package files

import (
	"time"

	"github.com/faiz-gh/fronko/backend/internal/auth"
)

// File areas: a user's own files, the organisation's private files, and the
// area every user in the organisation can see.
const (
	AreaPersonal = "personal"
	AreaOrg      = "org"
	AreaShared   = "shared"
	AreaTeam     = "team"
)

// File purposes say what a file is for, so the library and pickers can sort
// logos from brochures.
const (
	PurposeLogo     = "logo"
	PurposeBanner   = "banner"
	PurposeAvatar   = "avatar"
	PurposeCover    = "cover"
	PurposeGallery  = "gallery"
	PurposeBrochure = "brochure"
	PurposeOther    = "other"
)

// Purposes lists every file purpose, in the order the library shows them.
var Purposes = []string{PurposeLogo, PurposeBanner, PurposeAvatar, PurposeCover, PurposeGallery, PurposeBrochure, PurposeOther}

// ValidPurpose reports whether p is a known file purpose.
func ValidPurpose(p string) bool {
	for _, v := range Purposes {
		if v == p {
			return true
		}
	}
	return false
}

// StorageSettings is a user's bucket configuration. The key fields hold
// ciphertext (see package secrets) and never leave the server.
type StorageSettings struct {
	UserID             int64
	Provider           string
	Endpoint           string
	Region             string
	Bucket             string
	PathStyle          bool
	AccessKeyIDEnc     []byte
	SecretAccessKeyEnc []byte
	AccessKeyHint      string
	VerifiedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// File is an upload in a user's library. PublicID is the only identifier exposed to visitors.
type File struct {
	ID       int64  `json:"-"`
	PublicID string `json:"id"`
	OrgID    int64  `json:"-"`
	UserID   int64  `json:"-"` // the uploader; for personal files, the owner
	Area     string `json:"area"`
	// Owner is the user a personal file belongs to (or who uploaded an org or shared file).
	Owner *auth.UserRef `json:"owner,omitempty"`
	// FormerOwner is the username of a deleted user whose personal file this was.
	FormerOwner *string `json:"former_owner,omitempty"`
	// TeamID and Team are set for files in a team's area.
	TeamID       *int64        `json:"-"`
	Team         *auth.TeamRef `json:"team,omitempty"`
	Bucket       string        `json:"-"`
	ObjectKey    string        `json:"-"`
	ThumbKey     *string       `json:"-"`
	Kind         string        `json:"kind"` // "image" or "pdf"
	Purpose      string        `json:"purpose"`
	ContentType  string        `json:"content_type"`
	SizeBytes    int64         `json:"size_bytes"`
	OriginalName string        `json:"name"`
	Title        string        `json:"title"`
	Width        *int          `json:"width"`
	Height       *int          `json:"height"`
	Pages        *int          `json:"pages"`
	HasThumb     bool          `json:"has_thumb"`
	// UseCount is how many places use the file: card slots, the logo and the signature banner.
	UseCount  int64     `json:"use_count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FileUse is one card using a file, and where on the card.
type FileUse struct {
	ProfileID int64  `json:"profile_id"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	Slot      string `json:"slot"`
}

// FileUsage is everywhere a file is used that the viewer may know about.
type FileUsage struct {
	Cards []FileUse `json:"cards"`
	// HiddenCards counts cards using the file that the viewer can't see.
	HiddenCards int64 `json:"hidden_cards"`
	OrgLogo     bool  `json:"org_logo"`
	SigBanner   bool  `json:"signature_banner"`
}

// PublicFile is the file metadata shown on a public card.
type PublicFile struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	SizeBytes int64  `json:"size_bytes"`
}
