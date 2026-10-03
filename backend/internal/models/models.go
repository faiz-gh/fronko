package models

import "time"

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"` // Never exposed in JSON
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Profile struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Slug      string    `json:"slug"`
	Data      []byte    `json:"data"` // JSONB block data
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Lead struct {
	ID        string    `json:"id"`
	ProfileID string    `json:"profile_id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
}
