package leads

import (
	"time"

	"github.com/faiz-gh/fronko/backend/internal/auth"
)

type Lead struct {
	ID        int64  `json:"id"`
	ProfileID int64  `json:"profile_id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	// Optional; both set or both empty. Dial code with "+" ("+91") and the
	// national number, digits only.
	PhoneCountryCode string `json:"phone_country_code,omitempty"`
	PhoneNumber      string `json:"phone_number,omitempty"`
	Notes            string `json:"notes"`
	// Source is how the visitor reached the card: nfc, qr or link ("" for older leads).
	Source    string    `json:"source,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	// AssignedUser held the card when the lead arrived; nil means the organisation did.
	AssignedUser *auth.UserRef `json:"assigned_user"`
}
