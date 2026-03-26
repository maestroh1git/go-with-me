package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/go-with-me/app/internal/lib"
	"github.com/go-with-me/app/internal/models"
)

// IUserRepository defines the persistence contract for the users domain.
type IUserRepository interface {
	// Create inserts a new user. The service layer is responsible for hashing the password
	// before passing it to CreateUserParams.Password.
	Create(ctx context.Context, params CreateUserParams) (*models.User, error)

	// FindByID returns a user by primary key. Returns lib.ErrNotFound when absent.
	FindByID(ctx context.Context, id uuid.UUID) (*models.User, error)

	// FindByEmail returns a user by email address (case-insensitive). Returns lib.ErrNotFound when absent.
	FindByEmail(ctx context.Context, email string) (*models.User, error)

	// List returns a paginated list of users and the total count.
	List(ctx context.Context, params ListUsersParams) ([]*models.User, int64, error)

	// Update applies a partial update to a user record.
	Update(ctx context.Context, id uuid.UUID, params UpdateUserParams) (*models.User, error)

	// UpdatePassword replaces the stored bcrypt hash.
	UpdatePassword(ctx context.Context, id uuid.UUID, hash string) error

	// IncrFailedAttempts increments the failed login counter by one.
	IncrFailedAttempts(ctx context.Context, id uuid.UUID) error

	// LockUntil sets locked_until to prevent login until the given time.
	LockUntil(ctx context.Context, id uuid.UUID, until time.Time) error

	// ClearLock resets failed_attempts and clears locked_until.
	ClearLock(ctx context.Context, id uuid.UUID) error

	// SetTOTP persists the TOTP secret and enabled flag.
	SetTOTP(ctx context.Context, id uuid.UUID, secret string, enabled bool) error

	// Deactivate sets is_active=false on the user.
	Deactivate(ctx context.Context, id uuid.UUID) error

	// CountSuperAdmins returns the number of active super_admin users.
	CountSuperAdmins(ctx context.Context) (int64, error)
}

// CreateUserParams holds the input for creating a new user.
type CreateUserParams struct {
	Email        string
	Name         string
	PasswordHash string // bcrypt hash — callers must hash before passing
	Role         models.UserRole
}

// UpdateUserParams holds optional fields for a partial user update.
type UpdateUserParams struct {
	Name *string
	Role *models.UserRole
}

// ListUsersParams holds filters and pagination for listing users.
type ListUsersParams struct {
	Role   *models.UserRole
	Search *string // partial match on name or email
	lib.Params
}
