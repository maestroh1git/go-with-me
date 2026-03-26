package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/go-with-me/app/internal/auth"
	"github.com/go-with-me/app/internal/config"
	"github.com/go-with-me/app/internal/lib"
	"github.com/go-with-me/app/internal/models"
	"github.com/go-with-me/app/internal/repository/interfaces"
)

// PasswordResetNotifier sends the OTP to the user via email (or logs it in development).
type PasswordResetNotifier interface {
	NotifyPasswordResetOTP(ctx context.Context, email, otp string) error
}

// logNotifier logs the OTP to console — used when SMTP is not configured (development).
type logNotifier struct{ logger *zap.Logger }

func (n *logNotifier) NotifyPasswordResetOTP(_ context.Context, email, otp string) error {
	n.logger.Info("[DEV] password reset OTP",
		zap.String("email", email),
		zap.String("otp", otp),
	)
	return nil
}

// LoginResult is returned from all auth flows that may issue tokens.
type LoginResult struct {
	// Session token — set when authentication is fully complete.
	AccessToken string
	ExpiresAt   time.Time

	// MFA challenge — set when the user has TOTP enrolled and must verify it.
	MFARequired bool
	MFAToken    string

	// TOTP setup required — set on first login when no TOTP secret is configured (mandatory 2FA).
	TOTPSetupRequired bool
	SetupToken        string
}

// AuthService handles all authentication and session management flows.
type AuthService struct {
	cfg    *config.Config
	users  interfaces.IUserRepository
	resets interfaces.IPasswordResetRepository
	logger *zap.Logger
	notify PasswordResetNotifier
}

// NewAuthService creates an AuthService. Pass nil for notify to use console logging.
func NewAuthService(
	cfg *config.Config,
	users interfaces.IUserRepository,
	resets interfaces.IPasswordResetRepository,
	logger *zap.Logger,
	notify PasswordResetNotifier,
) *AuthService {
	n := notify
	if n == nil {
		n = &logNotifier{logger: logger}
	}
	return &AuthService{cfg: cfg, users: users, resets: resets, logger: logger, notify: n}
}

// Login validates credentials and returns a result indicating next steps.
// If TOTP is enabled: issues an MFA challenge token.
// If TOTP is not set up: issues a TOTP setup token (mandatory 2FA).
func (s *AuthService) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	u, err := s.users.FindByEmail(ctx, strings.TrimSpace(email))
	if err != nil {
		if errors.Is(err, lib.ErrNotFound) {
			return nil, lib.ErrUnauthorized
		}
		return nil, err
	}

	if !u.IsActive {
		return nil, lib.ErrUnauthorized
	}
	if u.IsLocked() {
		return nil, lib.BadRequest("account is locked due to too many failed attempts")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		_ = s.users.IncrFailedAttempts(ctx, u.ID)
		u2, gerr := s.users.FindByID(ctx, u.ID)
		if gerr == nil && u2.FailedAttempts >= s.cfg.MaxFailedAttempts {
			lockUntil := time.Now().UTC().Add(30 * time.Minute)
			_ = s.users.LockUntil(ctx, u.ID, lockUntil)
		}
		return nil, lib.ErrUnauthorized
	}

	// Reset failed attempts on successful password check.
	_ = s.users.ClearLock(ctx, u.ID)

	// TOTP enrolled — require MFA verification before issuing a session.
	if u.TOTPEnabled && u.TOTPSecret != nil && *u.TOTPSecret != "" {
		ttl := time.Duration(s.cfg.MFAExpiryMinutes) * time.Minute
		mfaTok, err := auth.IssueMFAToken(s.cfg.AppSecret, u.ID, ttl)
		if err != nil {
			return nil, fmt.Errorf("issue mfa token: %w", err)
		}
		return &LoginResult{MFARequired: true, MFAToken: mfaTok}, nil
	}

	// TOTP not yet configured — require setup (mandatory 2FA).
	setupTTL := time.Duration(s.cfg.TOTPSetupExpiryMinutes) * time.Minute
	if setupTTL <= 0 {
		setupTTL = time.Hour
	}
	setupTok, err := auth.IssuePurposeToken(s.cfg.AppSecret, u.ID, u.Email, auth.PurposeTOTPSetup, setupTTL)
	if err != nil {
		return nil, fmt.Errorf("issue totp setup token: %w", err)
	}
	return &LoginResult{TOTPSetupRequired: true, SetupToken: setupTok}, nil
}

// VerifyTOTP validates a TOTP code against an MFA-scoped token and issues a session.
func (s *AuthService) VerifyTOTP(ctx context.Context, mfaToken, code string) (*LoginResult, error) {
	claims, err := auth.Parse(mfaToken, s.cfg.AppSecret)
	if err != nil || claims.Purpose != auth.PurposeMFA {
		return nil, lib.ErrUnauthorized
	}

	u, err := s.users.FindByID(ctx, claims.UserID)
	if err != nil {
		return nil, lib.ErrUnauthorized
	}
	if !u.IsActive {
		return nil, lib.ErrUnauthorized
	}
	if u.TOTPSecret == nil || !totp.Validate(strings.TrimSpace(code), *u.TOTPSecret) {
		return nil, lib.Unauthorized("invalid authenticator code")
	}

	return s.issueSession(ctx, u)
}

// SetupTOTP generates a new TOTP secret (not persisted) for the setup flow.
// Requires a totp_setup scoped token.
func (s *AuthService) SetupTOTP(ctx context.Context, setupToken string) (secret, otpauthURL string, err error) {
	claims, err := auth.Parse(setupToken, s.cfg.AppSecret)
	if err != nil || claims.Purpose != auth.PurposeTOTPSetup {
		return "", "", lib.ErrUnauthorized
	}

	u, err := s.users.FindByID(ctx, claims.UserID)
	if err != nil {
		return "", "", lib.ErrUnauthorized
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "go-with-me",
		AccountName: u.Email,
	})
	if err != nil {
		return "", "", fmt.Errorf("generate totp key: %w", err)
	}
	return key.Secret(), key.URL(), nil
}

// ConfirmTOTP validates a TOTP code and, if correct, persists the secret and issues a session.
// Requires a totp_setup scoped token and the secret returned by SetupTOTP.
func (s *AuthService) ConfirmTOTP(ctx context.Context, setupToken, secret, code string) (*LoginResult, error) {
	claims, err := auth.Parse(setupToken, s.cfg.AppSecret)
	if err != nil || claims.Purpose != auth.PurposeTOTPSetup {
		return nil, lib.ErrUnauthorized
	}

	u, err := s.users.FindByID(ctx, claims.UserID)
	if err != nil {
		return nil, lib.ErrUnauthorized
	}
	if !u.IsActive {
		return nil, lib.ErrUnauthorized
	}

	if !totp.Validate(strings.TrimSpace(code), secret) {
		return nil, lib.BadRequest("invalid authenticator code")
	}

	if err := s.users.SetTOTP(ctx, u.ID, secret, true); err != nil {
		return nil, fmt.Errorf("persist totp secret: %w", err)
	}

	// Reload user so issueSession has the latest state.
	u, err = s.users.FindByID(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	return s.issueSession(ctx, u)
}

// ForgotPassword generates an OTP, stores its hash, and notifies the user.
// Always returns nil to prevent user enumeration — callers should return success regardless.
func (s *AuthService) ForgotPassword(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	u, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return nil // anti-enumeration: don't leak whether the email exists
	}
	if !u.IsActive {
		return nil
	}

	otp, err := randomOTP6()
	if err != nil {
		return fmt.Errorf("generate otp: %w", err)
	}

	sum := sha256.Sum256([]byte(otp))
	otpHash := hex.EncodeToString(sum[:])
	exp := time.Now().UTC().Add(time.Duration(s.cfg.PasswordResetOTPMinutes) * time.Minute)

	if _, err := s.resets.Create(ctx, interfaces.CreatePasswordResetParams{
		UserID:    u.ID,
		OTPHash:   otpHash,
		ExpiresAt: exp,
	}); err != nil {
		return fmt.Errorf("store reset record: %w", err)
	}

	return s.notify.NotifyPasswordResetOTP(ctx, u.Email, otp)
}

// VerifyResetEmail checks an emailed OTP and issues a password-reset JWT on success.
func (s *AuthService) VerifyResetEmail(ctx context.Context, email, otp string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	otp = strings.TrimSpace(otp)
	if len(otp) != 6 {
		return "", lib.BadRequest("invalid OTP code")
	}

	u, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return "", lib.ErrUnauthorized
	}

	reset, err := s.resets.FindByUserID(ctx, u.ID)
	if err != nil {
		return "", lib.ErrUnauthorized
	}
	if reset.IsExpired() || reset.IsConsumed() {
		return "", lib.Unauthorized("OTP has expired or already been used")
	}

	sum := sha256.Sum256([]byte(otp))
	otpHash := hex.EncodeToString(sum[:])
	if otpHash != reset.OTPHash {
		return "", lib.ErrUnauthorized
	}

	if err := s.resets.Consume(ctx, reset.ID); err != nil {
		return "", fmt.Errorf("consume reset record: %w", err)
	}

	ttl := time.Duration(s.cfg.PasswordResetJWTMinutes) * time.Minute
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	return auth.IssuePurposeToken(s.cfg.AppSecret, u.ID, u.Email, auth.PurposePasswordReset, ttl)
}

// CompletePasswordReset sets a new password using a pwd_reset-scoped JWT.
// If TOTP is enrolled, totpCode must be provided.
func (s *AuthService) CompletePasswordReset(ctx context.Context, resetToken, newPassword, totpCode string) error {
	claims, err := auth.Parse(resetToken, s.cfg.AppSecret)
	if err != nil || claims.Purpose != auth.PurposePasswordReset {
		return lib.ErrUnauthorized
	}

	u, err := s.users.FindByID(ctx, claims.UserID)
	if err != nil {
		return lib.ErrUnauthorized
	}

	if u.TOTPEnabled && u.TOTPSecret != nil {
		if !totp.Validate(strings.TrimSpace(totpCode), *u.TOTPSecret) {
			return lib.ErrUnauthorized
		}
	}

	if len(newPassword) < 10 {
		return lib.BadRequest("password must be at least 10 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	return s.users.UpdatePassword(ctx, u.ID, string(hash))
}

// AcceptInvite accepts an invite token, sets name + password, and returns a setup token.
// The caller then sends the user through the TOTP setup flow.
func (s *AuthService) AcceptInvite(ctx context.Context, inviteToken, name, password string) (*LoginResult, error) {
	claims, err := auth.Parse(inviteToken, s.cfg.AppSecret)
	if err != nil || claims.Purpose != auth.PurposeInvite {
		return nil, lib.BadRequest("invalid or expired invite token")
	}

	if len(password) < 10 {
		return nil, lib.BadRequest("password must be at least 10 characters")
	}

	u, err := s.users.FindByID(ctx, claims.UserID)
	if err != nil {
		return nil, lib.BadRequest("invite is no longer valid")
	}
	if u.IsActive {
		return nil, lib.Conflict("account is already active")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	trimmedName := strings.TrimSpace(name)
	namePtr := trimmedName
	_, err = s.users.Update(ctx, u.ID, interfaces.UpdateUserParams{Name: &namePtr})
	if err != nil {
		return nil, err
	}
	if err := s.users.UpdatePassword(ctx, u.ID, string(hash)); err != nil {
		return nil, err
	}

	// Activate the user by clearing their lock state.
	if err := s.users.ClearLock(ctx, u.ID); err != nil {
		return nil, err
	}

	// Issue a TOTP setup token to complete mandatory 2FA enrollment.
	setupTTL := time.Duration(s.cfg.TOTPSetupExpiryMinutes) * time.Minute
	setupTok, err := auth.IssuePurposeToken(s.cfg.AppSecret, u.ID, u.Email, auth.PurposeTOTPSetup, setupTTL)
	if err != nil {
		return nil, err
	}
	return &LoginResult{TOTPSetupRequired: true, SetupToken: setupTok}, nil
}

// Me returns the authenticated user's profile.
func (s *AuthService) Me(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	u, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// Logout is a no-op by default — implement token blacklisting in Redis here if needed.
func (s *AuthService) Logout(_ context.Context, _ uuid.UUID) error {
	return nil
}

// issueSession creates a fully authenticated session token for the given user.
func (s *AuthService) issueSession(_ context.Context, u *models.User) (*LoginResult, error) {
	perms := models.RolePermissions(u.Role)
	ttl := time.Duration(s.cfg.JWTExpiryHours) * time.Hour
	tok, exp, err := auth.IssueSessionToken(s.cfg.AppSecret, u.ID, u.Email, string(u.Role), perms, ttl)
	if err != nil {
		return nil, fmt.Errorf("issue session token: %w", err)
	}
	return &LoginResult{AccessToken: tok, ExpiresAt: exp}, nil
}

func randomOTP6() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// EnsureBootstrapAdmin creates the first super_admin when BOOTSTRAP_ADMIN_EMAIL is set
// and no super_admin exists yet.
func EnsureBootstrapAdmin(ctx context.Context, cfg *config.Config, users interfaces.IUserRepository, logger *zap.Logger) error {
	if cfg.BootstrapAdminEmail == "" || cfg.BootstrapAdminPassword == "" {
		return nil
	}

	count, err := users.CountSuperAdmins(ctx)
	if err != nil {
		return fmt.Errorf("count super admins: %w", err)
	}
	if count > 0 {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.BootstrapAdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash bootstrap password: %w", err)
	}

	u, err := users.Create(ctx, interfaces.CreateUserParams{
		Email:        cfg.BootstrapAdminEmail,
		Name:         cfg.BootstrapAdminName,
		PasswordHash: string(hash),
		Role:         models.RoleSuperAdmin,
	})
	if err != nil {
		return fmt.Errorf("create bootstrap admin: %w", err)
	}

	logger.Info("bootstrap super admin created",
		zap.String("email", u.Email),
		zap.String("id", u.ID.String()),
	)
	return nil
}
