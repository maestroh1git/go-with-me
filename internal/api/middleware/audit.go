package middleware

import (
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/go-with-me/app/internal/repository/interfaces"
)

// Audit records an audit log entry after the request completes.
// Captures the authenticated user, client IP, user-agent, and response status.
// Non-blocking best-effort: failures are logged but do not affect the response.
//
// action is a dot-namespaced string (e.g., "user.invite").
// resource is the entity type (e.g., "users").
func Audit(repo interfaces.IAuditRepository, action, resource string, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		// Only audit state-changing methods that completed without a server error.
		if !isWriteMethod(c.Request.Method) || c.Writer.Status() >= 500 {
			return
		}

		resourceID := c.Param("id")
		var resourceIDPtr *string
		if resourceID != "" {
			resourceIDPtr = &resourceID
		}

		meta, _ := json.Marshal(map[string]any{
			"method":     c.Request.Method,
			"path":       c.FullPath(),
			"status":     c.Writer.Status(),
			"latency_ms": time.Since(start).Milliseconds(),
			"request_id": c.GetString(RequestIDKey),
		})

		uid := GetUserID(c)
		params := interfaces.CreateAuditParams{
			Action:     action,
			Resource:   resource,
			ResourceID: resourceIDPtr,
			IPAddress:  c.ClientIP(),
			UserAgent:  c.Request.UserAgent(),
			Metadata:   json.RawMessage(meta),
		}
		if uid.String() != "00000000-0000-0000-0000-000000000000" {
			params.UserID = &uid
		}

		if err := repo.Create(c.Request.Context(), params); err != nil {
			logger.Warn("audit log write failed",
				zap.String("action", action),
				zap.String("resource", resource),
				zap.Error(err),
			)
		}
	}
}

func isWriteMethod(method string) bool {
	switch method {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	}
	return false
}
