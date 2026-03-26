package bootstrap

import (
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/go-with-me/app/internal/config"
	"github.com/go-with-me/app/internal/services"
)

// Services is the application-wide collection of business logic services.
type Services struct {
	Auth  *services.AuthService
	Users *services.UserService
}

// NewServices wires up all services with their dependencies.
func NewServices(repos *Repositories, rdb *redis.Client, cfg *config.Config, logger *zap.Logger) *Services {
	authSvc := services.NewAuthService(cfg, repos.Users, repos.PasswordResets, logger, nil)
	userSvc := services.NewUserService(cfg, repos.Users, logger)

	return &Services{
		Auth:  authSvc,
		Users: userSvc,
	}
}
