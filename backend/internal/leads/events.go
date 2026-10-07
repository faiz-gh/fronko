package leads

// Created is published when a visitor sends the contact form on a card,
// inside the transaction that stores the lead. Subscribers (such as lead
// sync) can queue jobs for it, which then exist only if the lead does.
type Created struct {
	LeadID    int64
	OrgID     int64
	ProfileID int64
	// AssignedUserID held the card when the lead arrived; 0 means the
	// organisation did.
	AssignedUserID int64
}

func (Created) EventName() string { return "leads.created" }
