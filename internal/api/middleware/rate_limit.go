package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimit enforces a sliding window rate limit keyed by client IP.
// namespace must be unique per route group (e.g., "auth:login", "users:list").
// Fail-open: if Redis is unavailable the request is allowed through.
// Sets X-RateLimit-Remaining on all responses.
func RateLimit(rdb *redis.Client, namespace string, max int, window time.Duration) gin.HandlerFunc {
	if namespace == "" {
		namespace = "default"
	}
	return func(c *gin.Context) {
		subject := c.ClientIP()
		key := fmt.Sprintf("rate:%s:%s", namespace, subject)

		remaining, allowed, err := checkRateLimit(c.Request.Context(), rdb, key, max, window)
		if err != nil {
			// Fail-open: Redis unavailable — let the request through.
			c.Next()
			return
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(max))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))

		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error":   gin.H{"code": "TOO_MANY_REQUESTS", "message": "rate limit exceeded — please slow down"},
			})
			return
		}

		c.Next()
	}
}

// checkRateLimit increments the counter and returns (remaining, allowed, error).
func checkRateLimit(ctx context.Context, rdb *redis.Client, key string, max int, window time.Duration) (int, bool, error) {
	count, err := rdb.Incr(ctx, key).Result()
	if err != nil {
		return max, false, err
	}
	// Set TTL only on the first request in the window.
	if count == 1 {
		rdb.Expire(ctx, key, window)
	}

	remaining := max - int(count)
	if remaining < 0 {
		remaining = 0
	}
	return remaining, count <= int64(max), nil
}
