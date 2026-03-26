package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/go-with-me/app/internal/api/middleware"
	"github.com/go-with-me/app/internal/lib"
	"github.com/go-with-me/app/internal/services"
)

// AuthHandler handles all authentication-related HTTP requests.
type AuthHandler struct {
	auth *services.AuthService
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(auth *services.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

// loginRequest is the body for POST /auth/login.
type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Login authenticates a user and returns a session token, MFA token, or TOTP setup token.
// POST /auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		lib.Fail(c, lib.BadRequest(err.Error()))
		return
	}

	result, err := h.auth.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		lib.Fail(c, err)
		return
	}

	if result.MFARequired {
		lib.Success(c, gin.H{
			"mfa_required": true,
			"mfa_token":    result.MFAToken,
		})
		return
	}

	if result.TOTPSetupRequired {
		lib.Success(c, gin.H{
			"totp_setup_required": true,
			"setup_token":         result.SetupToken,
		})
		return
	}

	lib.Success(c, gin.H{
		"access_token": result.AccessToken,
		"expires_at":   result.ExpiresAt,
	})
}

// verifyTOTPRequest is the body for POST /auth/verify-totp.
type verifyTOTPRequest struct {
	MFAToken string `json:"mfa_token" binding:"required"`
	Code     string `json:"code" binding:"required"`
}

// VerifyTOTP exchanges an MFA token + TOTP code for a session token.
// POST /auth/verify-totp
func (h *AuthHandler) VerifyTOTP(c *gin.Context) {
	var req verifyTOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		lib.Fail(c, lib.BadRequest(err.Error()))
		return
	}

	result, err := h.auth.VerifyTOTP(c.Request.Context(), req.MFAToken, req.Code)
	if err != nil {
		lib.Fail(c, err)
		return
	}

	lib.Success(c, gin.H{
		"access_token": result.AccessToken,
		"expires_at":   result.ExpiresAt,
	})
}

// SetupTOTP returns a TOTP secret and QR code URL for the setup flow.
// POST /auth/totp/setup — requires totp_setup token
func (h *AuthHandler) SetupTOTP(c *gin.Context) {
	setupToken := extractSetupToken(c)
	if setupToken == "" {
		lib.Fail(c, lib.ErrUnauthorized)
		return
	}

	secret, otpauthURL, err := h.auth.SetupTOTP(c.Request.Context(), setupToken)
	if err != nil {
		lib.Fail(c, err)
		return
	}

	lib.Success(c, gin.H{
		"secret":      secret,
		"otpauth_url": otpauthURL,
	})
}

// confirmTOTPRequest is the body for POST /auth/totp/confirm.
type confirmTOTPRequest struct {
	Secret string `json:"secret" binding:"required"`
	Code   string `json:"code" binding:"required"`
}

// ConfirmTOTP validates the TOTP code, persists the secret, and issues a session.
// POST /auth/totp/confirm — requires totp_setup token
func (h *AuthHandler) ConfirmTOTP(c *gin.Context) {
	setupToken := extractSetupToken(c)
	if setupToken == "" {
		lib.Fail(c, lib.ErrUnauthorized)
		return
	}

	var req confirmTOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		lib.Fail(c, lib.BadRequest(err.Error()))
		return
	}

	result, err := h.auth.ConfirmTOTP(c.Request.Context(), setupToken, req.Secret, req.Code)
	if err != nil {
		lib.Fail(c, err)
		return
	}

	lib.Success(c, gin.H{
		"access_token": result.AccessToken,
		"expires_at":   result.ExpiresAt,
	})
}

// forgotPasswordRequest is the body for POST /auth/forgot-password.
type forgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ForgotPassword initiates the password reset flow by emailing an OTP.
// POST /auth/forgot-password — always returns 200 (anti-enumeration).
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req forgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		lib.Fail(c, lib.BadRequest(err.Error()))
		return
	}

	// Best-effort — always succeed from the client's perspective.
	_ = h.auth.ForgotPassword(c.Request.Context(), req.Email)

	lib.Success(c, gin.H{"message": "if that email is registered, you will receive a reset code shortly"})
}

// verifyResetEmailRequest is the body for POST /auth/verify-reset-email.
type verifyResetEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
	OTP   string `json:"otp" binding:"required"`
}

// VerifyResetEmail checks the emailed OTP and issues a password-reset JWT.
// POST /auth/verify-reset-email
func (h *AuthHandler) VerifyResetEmail(c *gin.Context) {
	var req verifyResetEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		lib.Fail(c, lib.BadRequest(err.Error()))
		return
	}

	resetToken, err := h.auth.VerifyResetEmail(c.Request.Context(), req.Email, req.OTP)
	if err != nil {
		lib.Fail(c, err)
		return
	}

	lib.Success(c, gin.H{"reset_token": resetToken})
}

// completePasswordResetRequest is the body for POST /auth/complete-password-reset.
type completePasswordResetRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=10"`
	TOTPCode    string `json:"totp_code"` // required only when TOTP is enrolled
}

// CompletePasswordReset sets a new password using the pwd_reset JWT.
// POST /auth/complete-password-reset — requires pwd_reset token
func (h *AuthHandler) CompletePasswordReset(c *gin.Context) {
	resetToken := extractSetupToken(c)
	if resetToken == "" {
		lib.Fail(c, lib.ErrUnauthorized)
		return
	}

	var req completePasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		lib.Fail(c, lib.BadRequest(err.Error()))
		return
	}

	if err := h.auth.CompletePasswordReset(c.Request.Context(), resetToken, req.NewPassword, req.TOTPCode); err != nil {
		lib.Fail(c, err)
		return
	}

	lib.Success(c, gin.H{"message": "password updated — please log in with your new password"})
}

// acceptInviteRequest is the body for POST /auth/accept-invite.
type acceptInviteRequest struct {
	InviteToken string `json:"invite_token" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Password    string `json:"password" binding:"required,min=10"`
}

// AcceptInvite completes onboarding from an invite token.
// POST /auth/accept-invite
func (h *AuthHandler) AcceptInvite(c *gin.Context) {
	var req acceptInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		lib.Fail(c, lib.BadRequest(err.Error()))
		return
	}

	result, err := h.auth.AcceptInvite(c.Request.Context(), req.InviteToken, req.Name, req.Password)
	if err != nil {
		lib.Fail(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": gin.H{
			"totp_setup_required": result.TOTPSetupRequired,
			"setup_token":         result.SetupToken,
		},
	})
}

// Me returns the authenticated user's profile.
// GET /auth/me — requires session token
func (h *AuthHandler) Me(c *gin.Context) {
	userID := middleware.GetUserID(c)
	u, err := h.auth.Me(c.Request.Context(), userID)
	if err != nil {
		lib.Fail(c, err)
		return
	}

	lib.Success(c, gin.H{
		"id":           u.ID,
		"email":        u.Email,
		"name":         u.Name,
		"role":         u.Role,
		"totp_enabled": u.TOTPEnabled,
		"is_active":    u.IsActive,
		"created_at":   u.CreatedAt,
	})
}

// extractSetupToken reads the Bearer token for purpose-scoped endpoints.
// For these endpoints the token IS the purpose token (totp_setup, pwd_reset, invite).
func extractSetupToken(c *gin.Context) string {
	header := c.GetHeader("Authorization")
	if len(header) > 7 && header[:7] == "Bearer " {
		return header[7:]
	}
	return ""
}
