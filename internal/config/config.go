package config

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/caarlos0/env/v10"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Server
	HTTPPort string `env:"HTTP_PORT" envDefault:"8080"`
	AppEnv   string `env:"APP_ENV" envDefault:"development"`

	// PostgreSQL
	DatabaseWriteURL string   `env:"DATABASE_WRITE_URL,required"`
	DatabaseReadURLs []string `env:"DATABASE_READ_URLS" envSeparator:","`

	// Redis
	RedisURL string `env:"REDIS_URL,required"`

	// Events
	EventStream string `env:"EVENT_STREAM" envDefault:"app:events"`

	// Auth
	AppSecret string `env:"APP_SECRET,required"`

	// JWT
	JWTExpiryHours         int `env:"JWT_EXPIRY_HOURS" envDefault:"24"`
	MFAExpiryMinutes       int `env:"MFA_EXPIRY_MINUTES" envDefault:"10"`
	TOTPSetupExpiryMinutes int `env:"TOTP_SETUP_EXPIRY_MINUTES" envDefault:"60"`
	PasswordResetOTPMinutes int `env:"PASSWORD_RESET_OTP_MINUTES" envDefault:"15"`
	PasswordResetJWTMinutes int `env:"PASSWORD_RESET_JWT_MINUTES" envDefault:"30"`

	// Security
	MaxFailedAttempts int `env:"MAX_FAILED_ATTEMPTS" envDefault:"5"`
	InviteTTLHours    int `env:"INVITE_TTL_HOURS" envDefault:"168"`

	// Bootstrap super admin (runs once on first startup if no user exists)
	BootstrapAdminEmail    string `env:"BOOTSTRAP_ADMIN_EMAIL"`
	BootstrapAdminPassword string `env:"BOOTSTRAP_ADMIN_PASSWORD"`
	BootstrapAdminName     string `env:"BOOTSTRAP_ADMIN_NAME" envDefault:"Super Admin"`

	// SMTP (optional — logs OTPs to console in development)
	SMTPHost     string `env:"SMTP_HOST"`
	SMTPPort     int    `env:"SMTP_PORT" envDefault:"587"`
	SMTPUser     string `env:"SMTP_USER"`
	SMTPPassword string `env:"SMTP_PASSWORD"`
	SMTPFrom     string `env:"SMTP_FROM"`
}

// IsDevelopment returns true when the app is not running in production mode.
func (c *Config) IsDevelopment() bool {
	return c.AppEnv != "production"
}

// ReadDSNs returns the list of read replica DSNs, filtering empty entries.
func (c *Config) ReadDSNs() []string {
	dsns := make([]string, 0, len(c.DatabaseReadURLs))
	for _, dsn := range c.DatabaseReadURLs {
		trimmed := strings.TrimSpace(dsn)
		if trimmed != "" {
			dsns = append(dsns, trimmed)
		}
	}
	return dsns
}

// Load parses configuration from environment variables and validates it.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
}

// Validate checks required fields and structural correctness.
func (c *Config) Validate() error {
	if err := requireURL("DATABASE_WRITE_URL", c.DatabaseWriteURL); err != nil {
		return err
	}
	for i, dsn := range c.ReadDSNs() {
		if err := requireURL(fmt.Sprintf("DATABASE_READ_URLS[%d]", i), dsn); err != nil {
			return err
		}
	}
	if err := requireURL("REDIS_URL", c.RedisURL); err != nil {
		return err
	}
	if len(strings.TrimSpace(c.AppSecret)) < 32 {
		return fmt.Errorf("APP_SECRET must be at least 32 characters")
	}
	if strings.TrimSpace(c.EventStream) == "" {
		return fmt.Errorf("EVENT_STREAM must not be empty")
	}
	return nil
}

func requireURL(name, value string) error {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return fmt.Errorf("%s must not be empty", name)
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s must be a valid URL/DSN (got %q)", name, raw)
	}
	return nil
}
