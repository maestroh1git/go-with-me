package events

import (
	"encoding/json"
	"time"
)

// Event is the envelope for all domain events published to the Redis Stream.
type Event struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Payload    json.RawMessage `json:"payload"`
	OccurredAt time.Time       `json:"occurred_at"`
}

// Common event type constants — extend these as you add domains.
const (
	EventUserCreated   = "user.created"
	EventUserUpdated   = "user.updated"
	EventUserInvited   = "user.invited"
	EventUserLoggedIn  = "user.logged_in"
	EventUserLoggedOut = "user.logged_out"
	EventPasswordReset = "user.password_reset"
	EventUserDeactivated = "user.deactivated"
)
