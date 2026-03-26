package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/go-with-me/app/internal/models"
)

// IPasswordResetRepository manages password reset OTP records.
type IPasswordResetRepository interface {
	// Create inserts a new reset record, replacing any existing one for the same user.
	Create(ctx context.Context, params CreatePasswordResetParams) (*models.PasswordReset, error)

	// FindByUserID returns the most recent reset record for a user.
	FindByUserID(ctx context.Context, userID uuid.UUID) (*models.PasswordReset, error)

	// SetToken persists the JWT reset token after OTP verification.
	SetToken(ctx context.Context, id uuid.UUID, token string) error

	// Consume marks the reset record as used (sets used_at = now).
	Consume(ctx context.Context, id uuid.UUID) error

	// DeleteExpired removes all records past their expiry (for maintenance jobs).
	DeleteExpired(ctx context.Context) error
}

// CreatePasswordResetParams holds the input for a new reset record.
type CreatePasswordResetParams struct {
	UserID    uuid.UUID
	OTPHash   string // SHA-256 hex of plaintext OTP
	ExpiresAt time.Time
}
