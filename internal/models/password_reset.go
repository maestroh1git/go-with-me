package models

import (
	"time"

	"github.com/google/uuid"
)

// PasswordReset represents a pending password-reset request initiated via email OTP.
type PasswordReset struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	OTPHash   string     // SHA-256 hex of the plaintext 6-digit OTP
	Token     *string    // Short-lived JWT issued after OTP verification
	ExpiresAt time.Time
	UsedAt    *time.Time // Non-nil once the OTP has been consumed
	CreatedAt time.Time
}

// IsExpired returns true if the reset request has passed its expiry.
func (r *PasswordReset) IsExpired() bool {
	return time.Now().UTC().After(r.ExpiresAt)
}

// IsConsumed returns true if the OTP has already been used.
func (r *PasswordReset) IsConsumed() bool {
	return r.UsedAt != nil
}
