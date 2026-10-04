package models

import (
	"encoding/json"
	"time"
)

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"` // Never exposed in JSON
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Profile struct {
	ID        int64           `json:"id"`
	UserID    int64           `json:"user_id"`
	Slug      string          `json:"slug"`
	Data      json.RawMessage `json:"data"` // JSONB block data
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	LeadCount int64           `json:"lead_count"` // only populated when listing a user's profiles
}

// PublicProfile is the shape served to anonymous visitors; it omits owner details.
type PublicProfile struct {
	ID    int64           `json:"id"`
	Slug  string          `json:"slug"`
	Data  json.RawMessage `json:"data"`
	Files []PublicFile    `json:"files"` // library files the card references
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
	ID           int64     `json:"-"`
	PublicID     string    `json:"id"`
	UserID       int64     `json:"-"`
	Bucket       string    `json:"-"`
	ObjectKey    string    `json:"-"`
	Kind         string    `json:"kind"` // "image" or "pdf"
	ContentType  string    `json:"content_type"`
	SizeBytes    int64     `json:"size_bytes"`
	OriginalName string    `json:"name"`
	Title        string    `json:"title"`
	CreatedAt    time.Time `json:"created_at"`
}

// PublicFile is the file metadata shown on a public card.
type PublicFile struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	SizeBytes int64  `json:"size_bytes"`
}

type Lead struct {
	ID        int64  `json:"id"`
	ProfileID int64  `json:"profile_id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	// Optional; both set or both empty. Dial code with "+" ("+91") and the
	// national number, digits only.
	PhoneCountryCode string    `json:"phone_country_code,omitempty"`
	PhoneNumber      string    `json:"phone_number,omitempty"`
	Notes            string    `json:"notes"`
	CreatedAt        time.Time `json:"created_at"`
}
