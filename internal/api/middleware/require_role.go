package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireRole returns a middleware that enforces one of the allowed roles.
// Must be applied after RequireAuth, which sets the role in context.
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(c *gin.Context) {
		role := GetUserRole(c)
		if _, ok := allowed[role]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"error":   gin.H{"code": "FORBIDDEN", "message": "insufficient role for this action"},
			})
			return
		}
		c.Next()
	}
}

// RequirePermission returns a middleware that checks the user holds ALL of the listed permissions.
// The wildcard permission "*" (super_admin) satisfies any check.
// Must be applied after RequireAuth.
func RequirePermission(perms ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userPerms := GetUserPermissions(c)

		// Build a lookup set.
		permSet := make(map[string]struct{}, len(userPerms))
		for _, p := range userPerms {
			permSet[p] = struct{}{}
		}

		// Wildcard satisfies all checks.
		if _, ok := permSet["*"]; ok {
			c.Next()
			return
		}

		for _, required := range perms {
			if _, ok := permSet[required]; !ok {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"success": false,
					"error":   gin.H{"code": "FORBIDDEN", "message": "missing required permission: " + required},
				})
				return
			}
		}
		c.Next()
	}
}
