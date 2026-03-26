package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/go-with-me/app/internal/api/routes"
	"github.com/go-with-me/app/internal/config"
	"github.com/go-with-me/app/internal/jobs"
	"github.com/go-with-me/app/internal/lib"
	"github.com/go-with-me/app/internal/services"
)

// App is the application container holding all wired-up dependencies.
type App struct {
	Config *config.Config
	Logger *zap.Logger
	DB     *DB
	Redis  *redis.Client
	Repos  *Repositories
	Svcs   *Services
	Router *gin.Engine
	Jobs   *jobs.Worker
	Sched  *jobs.Scheduler
	server *http.Server
}

// New creates and wires up the full application from environment configuration.
func New(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	logger, err := lib.NewLogger(cfg.AppEnv)
	if err != nil {
		return nil, fmt.Errorf("init logger: %w", err)
	}

	db, err := Connect(ctx, cfg.DatabaseWriteURL, cfg.ReadDSNs())
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}

	if err := runMigrations(cfg.DatabaseWriteURL); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	rdb, err := ConnectRedis(ctx, cfg.RedisURL)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("connect redis: %w", err)
	}

	repos := NewRepositories(db)
	svcs := NewServices(repos, rdb, cfg, logger)

	// Bootstrap the first super_admin if configured and none exist.
	if err := services.EnsureBootstrapAdmin(ctx, cfg, repos.Users, logger); err != nil {
		db.Close()
		_ = rdb.Close()
		return nil, fmt.Errorf("bootstrap admin: %w", err)
	}

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	routes.Setup(router, cfg, rdb, svcs.Auth, svcs.Users, repos.Audit, logger)

	server := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	worker := jobs.New(logger)
	sched := jobs.NewScheduler(logger)

	// Register default scheduled jobs.
	sched.Every(30*time.Minute, "cleanup.expired_password_resets", func(ctx context.Context) error {
		return repos.PasswordResets.DeleteExpired(ctx)
	})

	return &App{
		Config: cfg,
		Logger: logger,
		DB:     db,
		Redis:  rdb,
		Repos:  repos,
		Svcs:   svcs,
		Router: router,
		Jobs:   worker,
		Sched:  sched,
		server: server,
	}, nil
}

// RunAll starts HTTP server + job worker + scheduler concurrently.
func (a *App) RunAll(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	a.Logger.Info("app starting",
		zap.String("port", a.Config.HTTPPort),
		zap.String("env", a.Config.AppEnv),
		zap.String("mode", "all"),
	)

	errCh := make(chan error, 1)

	go func() {
		a.Logger.Info("HTTP server listening", zap.String("addr", a.server.Addr))
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	go func() {
		if err := a.Jobs.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			a.Logger.Error("job worker stopped", zap.Error(err))
		}
	}()

	go func() {
		a.Sched.Run(ctx)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return a.Shutdown()
	}
}

// RunAPI starts the HTTP server only.
func (a *App) RunAPI(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	a.Logger.Info("app starting",
		zap.String("port", a.Config.HTTPPort),
		zap.String("env", a.Config.AppEnv),
		zap.String("mode", "api"),
	)

	errCh := make(chan error, 1)
	go func() {
		a.Logger.Info("HTTP server listening", zap.String("addr", a.server.Addr))
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return a.Shutdown()
	}
}

// RunWorker starts job worker + scheduler only (no HTTP).
func (a *App) RunWorker(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	a.Logger.Info("app starting",
		zap.String("env", a.Config.AppEnv),
		zap.String("mode", "worker"),
	)

	go func() {
		a.Sched.Run(ctx)
	}()

	return a.Jobs.Run(ctx)
}

// Shutdown gracefully terminates the HTTP server and closes connections.
func (a *App) Shutdown() error {
	a.Logger.Info("app shutting down")
	var shutdownErr error
	if a.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		shutdownErr = a.server.Shutdown(ctx)
	}
	if a.Redis != nil {
		_ = a.Redis.Close()
	}
	a.DB.Close()
	_ = a.Logger.Sync()
	return shutdownErr
}
