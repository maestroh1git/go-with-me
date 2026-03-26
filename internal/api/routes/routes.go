package routes

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/go-with-me/app/internal/api/handlers"
	"github.com/go-with-me/app/internal/api/middleware"
	"github.com/go-with-me/app/internal/auth"
	"github.com/go-with-me/app/internal/config"
	"github.com/go-with-me/app/internal/repository/interfaces"
	"github.com/go-with-me/app/internal/services"
)

// Setup registers all routes on the provided Gin engine.
func Setup(
	r *gin.Engine,
	cfg *config.Config,
	rdb *redis.Client,
	authSvc *services.AuthService,
	userSvc *services.UserService,
	auditRepo interfaces.IAuditRepository,
	logger *zap.Logger,
) {
	// Global middleware applied to every route.
	r.Use(middleware.RequestID())
	r.Use(middleware.RequestLogger(logger))
	r.Use(middleware.Recovery(logger))

	// Health and readiness probes — no auth required.
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	setupAuthRoutes(r, cfg, rdb, authSvc)
	setupUserRoutes(r, cfg, rdb, userSvc, auditRepo, logger)
}

// setupAuthRoutes registers all /auth/* endpoints.
func setupAuthRoutes(r *gin.Engine, cfg *config.Config, rdb *redis.Client, authSvc *services.AuthService) {
	h := handlers.NewAuthHandler(authSvc)

	// Public — no token required.
	pub := r.Group("/auth")
	pub.POST("/login",
		middleware.RateLimit(rdb, "auth:login", 30, time.Minute),
		h.Login,
	)
	pub.POST("/forgot-password",
		middleware.RateLimit(rdb, "auth:forgot-password", 10, time.Minute),
		h.ForgotPassword,
	)
	pub.POST("/verify-reset-email",
		middleware.RateLimit(rdb, "auth:verify-reset-email", 10, time.Minute),
		h.VerifyResetEmail,
	)
	pub.POST("/accept-invite",
		middleware.RateLimit(rdb, "auth:accept-invite", 20, time.Minute),
		h.AcceptInvite,
	)

	// MFA token required — exchange after login when TOTP is enrolled.
	mfa := r.Group("/auth")
	mfa.Use(middleware.RequireMFA(cfg.AppSecret))
	mfa.POST("/verify-totp", h.VerifyTOTP)

	// TOTP setup token required — mandatory 2FA enrollment flow.
	totpSetup := r.Group("/auth/totp")
	totpSetup.Use(middleware.RequirePurpose(cfg.AppSecret, auth.PurposeTOTPSetup))
	totpSetup.POST("/setup", h.SetupTOTP)
	totpSetup.POST("/confirm", h.ConfirmTOTP)

	// Password reset token required.
	pwdReset := r.Group("/auth")
	pwdReset.Use(middleware.RequirePurpose(cfg.AppSecret, auth.PurposePasswordReset))
	pwdReset.POST("/complete-password-reset", h.CompletePasswordReset)

	// Session token required.
	session := r.Group("/auth")
	session.Use(middleware.RequireAuth(cfg.AppSecret))
	session.GET("/me", h.Me)
}

// setupUserRoutes registers all /api/v1/users/* endpoints.
func setupUserRoutes(
	r *gin.Engine,
	cfg *config.Config,
	rdb *redis.Client,
	userSvc *services.UserService,
	auditRepo interfaces.IAuditRepository,
	logger *zap.Logger,
) {
	h := handlers.NewUserHandler(userSvc)

	api := r.Group("/api/v1")
	api.Use(middleware.RequireAuth(cfg.AppSecret))

	// List users — admin and super_admin
	api.GET("/users",
		middleware.RequireRole(string(roleAdmin), string(roleSuperAdmin)),
		middleware.RateLimit(rdb, "api:users:list", 100, time.Minute),
		h.List,
	)

	// Get user — admin, super_admin, manager
	api.GET("/users/:id",
		middleware.RequireRole(string(roleSuperAdmin), string(roleAdmin), string(roleManager)),
		h.Get,
	)

	// Update user — admin and super_admin
	api.PATCH("/users/:id",
		middleware.RequireRole(string(roleAdmin), string(roleSuperAdmin)),
		h.Update,
	)

	// Invite user — admin and super_admin
	api.POST("/users/invite",
		middleware.RequireRole(string(roleAdmin), string(roleSuperAdmin)),
		middleware.RateLimit(rdb, "api:users:invite", 20, time.Minute),
		middleware.Audit(auditRepo, "user.invite", "users", logger),
		h.Invite,
	)

	// Deactivate user — super_admin only
	api.DELETE("/users/:id",
		middleware.RequireRole(string(roleSuperAdmin)),
		middleware.Audit(auditRepo, "user.deactivate", "users", logger),
		h.Deactivate,
	)
}

// Role string constants to avoid string literals scattered throughout routes.
type roleName string

const (
	roleSuperAdmin roleName = "super_admin"
	roleAdmin      roleName = "admin"
	roleManager    roleName = "manager"
	roleMember     roleName = "member"
	roleViewer     roleName = "viewer"
)
