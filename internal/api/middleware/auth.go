package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/go-with-me/app/internal/auth"
)

// Context keys for values set by auth middleware.
const (
	ctxUserID          = "user_id"
	ctxUserEmail       = "user_email"
	ctxUserRole        = "user_role"
	ctxUserPermissions = "user_permissions"
)

// RequireAuth validates a session JWT (purpose must be empty string).
// Sets userID, userEmail, userRole, and userPermissions in the Gin context.
func RequireAuth(secret string) gin.HandlerFunc {
	return requirePurpose(secret, auth.PurposeSession)
}

// RequireMFA validates an MFA-scoped token (purpose == "mfa").
func RequireMFA(secret string) gin.HandlerFunc {
	return requirePurpose(secret, auth.PurposeMFA)
}

// RequirePurpose validates a token with a specific purpose claim.
func RequirePurpose(secret, purpose string) gin.HandlerFunc {
	return requirePurpose(secret, purpose)
}

func requirePurpose(secret, expectedPurpose string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearer(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   gin.H{"code": "UNAUTHORIZED", "message": "missing authorization token"},
			})
			return
		}

		claims, err := auth.Parse(token, secret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   gin.H{"code": "UNAUTHORIZED", "message": "invalid or expired token"},
			})
			return
		}

		if claims.Purpose != expectedPurpose {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   gin.H{"code": "UNAUTHORIZED", "message": "token not valid for this endpoint"},
			})
			return
		}

		// For session tokens, require a role claim.
		if expectedPurpose == auth.PurposeSession && claims.Role == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   gin.H{"code": "UNAUTHORIZED", "message": "invalid token claims"},
			})
			return
		}

		c.Set(ctxUserID, claims.UserID)
		c.Set(ctxUserEmail, claims.Email)
		c.Set(ctxUserRole, claims.Role)
		c.Set(ctxUserPermissions, claims.Permissions)
		c.Next()
	}
}

// GetUserID extracts the authenticated user's UUID from the Gin context.
// Must be called after RequireAuth.
func GetUserID(c *gin.Context) uuid.UUID {
	val, _ := c.Get(ctxUserID)
	id, _ := val.(uuid.UUID)
	return id
}

// GetUserRole extracts the authenticated user's role string from the Gin context.
func GetUserRole(c *gin.Context) string {
	return c.GetString(ctxUserRole)
}

// GetUserPermissions extracts the authenticated user's permission list from the Gin context.
func GetUserPermissions(c *gin.Context) []string {
	val, _ := c.Get(ctxUserPermissions)
	perms, _ := val.([]string)
	return perms
}

// extractBearer reads the Bearer token from the Authorization header.
func extractBearer(c *gin.Context) string {
	header := c.GetHeader("Authorization")
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
