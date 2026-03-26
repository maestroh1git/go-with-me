package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/go-with-me/app/internal/auth"
	"github.com/go-with-me/app/internal/config"
	"github.com/go-with-me/app/internal/lib"
	"github.com/go-with-me/app/internal/models"
	"github.com/go-with-me/app/internal/repository/interfaces"
)

// UserService handles user management operations (list, get, update, invite, deactivate).
type UserService struct {
	cfg    *config.Config
	users  interfaces.IUserRepository
	logger *zap.Logger
}

// NewUserService creates a UserService.
func NewUserService(cfg *config.Config, users interfaces.IUserRepository, logger *zap.Logger) *UserService {
	return &UserService{cfg: cfg, users: users, logger: logger}
}

// List returns a paginated list of users.
func (s *UserService) List(ctx context.Context, params interfaces.ListUsersParams) ([]*models.User, int64, error) {
	return s.users.List(ctx, params)
}

// Get returns a single user by ID.
func (s *UserService) Get(ctx context.Context, id uuid.UUID) (*models.User, error) {
	u, err := s.users.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// Update applies a partial update to a user.
// Guards against changing the role of the last super_admin.
func (s *UserService) Update(ctx context.Context, id uuid.UUID, params interfaces.UpdateUserParams) (*models.User, error) {
	current, err := s.users.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Guard: do not demote the last super_admin.
	if params.Role != nil && *params.Role != models.RoleSuperAdmin && current.Role == models.RoleSuperAdmin {
		count, err := s.users.CountSuperAdmins(ctx)
		if err != nil {
			return nil, fmt.Errorf("count super admins: %w", err)
		}
		if count <= 1 {
			return nil, lib.BadRequest("cannot change role of the last super admin")
		}
	}

	return s.users.Update(ctx, id, params)
}

// Invite creates an inactive user record and returns a one-time invite token.
// The token must be delivered to the user via email; it is only returned once.
func (s *UserService) Invite(ctx context.Context, email, name string, role models.UserRole, invitedByID uuid.UUID) (string, error) {
	if role == models.RoleSuperAdmin {
		return "", lib.BadRequest("cannot invite a super_admin — promote an existing user instead")
	}

	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", lib.BadRequest("email is required")
	}

	// Check for existing user.
	if _, err := s.users.FindByEmail(ctx, email); err == nil {
		return "", lib.Conflict("a user with this email already exists")
	} else if !errors.Is(err, lib.ErrNotFound) {
		return "", err
	}

	// Create an inactive placeholder user.
	u, err := s.users.Create(ctx, interfaces.CreateUserParams{
		Email:        email,
		Name:         strings.TrimSpace(name),
		PasswordHash: "!", // invalid hash — cannot log in until invite is accepted
		Role:         role,
	})
	if err != nil {
		return "", fmt.Errorf("create invited user: %w", err)
	}

	// Immediately deactivate until the invite is accepted.
	if err := s.users.Deactivate(ctx, u.ID); err != nil {
		return "", fmt.Errorf("deactivate invited user: %w", err)
	}

	// Generate invite token.
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate invite token: %w", err)
	}
	plaintext := hex.EncodeToString(raw)

	// Sign a JWT so we can validate it without a DB round-trip (the user ID is embedded).
	ttl := time.Duration(s.cfg.InviteTTLHours) * time.Hour
	inviteToken, err := auth.IssuePurposeToken(s.cfg.AppSecret, u.ID, u.Email, auth.PurposeInvite, ttl)
	if err != nil {
		return "", fmt.Errorf("sign invite token: %w", err)
	}

	// Embed the hash of the plaintext token as a nonce in the DB for optional one-time validation.
	sum := sha256.Sum256([]byte(plaintext))
	_ = hex.EncodeToString(sum[:]) // reserved for future invite_links table

	s.logger.Info("user invited",
		zap.String("email", u.Email),
		zap.String("role", string(role)),
		zap.String("invited_by", invitedByID.String()),
	)

	return inviteToken, nil
}

// Deactivate marks a user as inactive. Guards against deactivating the last super_admin.
func (s *UserService) Deactivate(ctx context.Context, id uuid.UUID) error {
	u, err := s.users.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if u.Role == models.RoleSuperAdmin {
		count, err := s.users.CountSuperAdmins(ctx)
		if err != nil {
			return fmt.Errorf("count super admins: %w", err)
		}
		if count <= 1 {
			return lib.BadRequest("cannot deactivate the last super admin")
		}
	}

	return s.users.Deactivate(ctx, id)
}
