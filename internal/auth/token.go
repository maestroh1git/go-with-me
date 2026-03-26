package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Purpose constants scope a JWT to a specific auth flow.
// A session token has an empty purpose. Purpose-scoped tokens are rejected
// by standard auth middleware and must be validated by their dedicated handler.
const (
	PurposeSession       = ""
	PurposeMFA           = "mfa"
	PurposeTOTPSetup     = "totp_setup"
	PurposePasswordReset = "pwd_reset"
	PurposeInvite        = "invite"
)

// Claims is the JWT payload issued to users.
type Claims struct {
	jwt.RegisteredClaims
	UserID      uuid.UUID `json:"uid"`
	Email       string    `json:"email"`
	Role        string    `json:"role"`
	Permissions []string  `json:"perms,omitempty"`
	Purpose     string    `json:"purpose,omitempty"`
}

// Sign creates a signed JWT string from the given claims.
func Sign(secret string, claims Claims) (string, error) {
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// Parse validates and parses a JWT string, returning its Claims.
func Parse(tokenStr, secret string) (*Claims, error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	claims, ok := tok.Claims.(*Claims)
	if !ok || !tok.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	sub, err := claims.GetSubject()
	if err != nil || sub == "" {
		return nil, fmt.Errorf("missing sub claim")
	}
	return claims, nil
}

// IssueSessionToken creates a full session JWT for a successfully authenticated user.
func IssueSessionToken(secret string, userID uuid.UUID, email, role string, perms []string, ttl time.Duration) (string, time.Time, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(ttl)
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
		UserID:      userID,
		Email:       email,
		Role:        role,
		Permissions: perms,
		Purpose:     PurposeSession,
	}
	tok, err := Sign(secret, claims)
	return tok, expiresAt, err
}

// IssueMFAToken creates a short-lived token for TOTP verification after password check.
func IssueMFAToken(secret string, userID uuid.UUID, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		UserID:  userID,
		Purpose: PurposeMFA,
	}
	return Sign(secret, claims)
}

// IssuePurposeToken creates a token scoped to a specific auth flow (TOTP setup, password reset, invite).
func IssuePurposeToken(secret string, userID uuid.UUID, email, purpose string, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		UserID:  userID,
		Email:   email,
		Purpose: purpose,
	}
	return Sign(secret, claims)
}
