# go-with-me

A production-ready Go boilerplate for any backend service. Built on patterns extracted from a real Nigerian fintech system running in production. Batteries included — clone it, rename it, ship it.

---

## What is this?

go-with-me is an opinionated starting point for Go backend services. It is not a framework. It is a complete, working application with a real architecture that you copy and adapt. Every pattern in here has been tested in production.

The "Users" domain ships out of the box and demonstrates every pattern: models, repository interfaces, raw pgx implementations, service layer, JWT auth, middleware, handlers, and routes. Adding a new domain (Products, Orders, Devices, anything) is a matter of following the same pattern.

---

## Documentation

The documentation has been divided into the following guides for easier reading:

- **[Architecture & Security](docs/architecture.md)** — Project structure, security model, and scaling guides.
- **[API Reference](docs/api-reference.md)** — Available endpoints and detailed auth flows.
- **[Development Guide](docs/development-guide.md)** — How to extend, upgrade to SQLC, make targets, and more.
- **[Sector Examples](docs/sector-examples.md)** — Specialized patterns for adapting this boilerplate to IoT, AI, Fintech, HealthTech, etc.

---

## What works out of the box

After `make dev-start`:

- **Full auth system** — login, JWT sessions, TOTP 2FA (mandatory on first login), password reset via email OTP, invite-based onboarding
- **Role-based access control** — `super_admin`, `admin`, `manager`, `member`, `viewer` with permission strings per role
- **Purpose-scoped JWT tokens** — MFA tokens, TOTP setup tokens, password reset tokens, and invite tokens cannot be used interchangeably
- **User management API** — list, get, update, invite, deactivate
- **Sliding-window rate limiting** — per route, keyed by client IP, backed by Redis
- **Immutable audit logging** — append-only `audit_logs` table, captured via middleware
- **Redis Streams event publishing** — `Publisher` emits typed domain events
- **Background job worker** — register named jobs, all run concurrently, ctx-aware
- **Cron scheduler** — register periodic jobs with intervals, each on its own ticker
- **Read replica support** — PostgreSQL write + N read pools with round-robin selection
- **Request ID + structured logging** — Zap, JSON in production, color console in development
- **Panic recovery middleware** — returns a clean 500 JSON response, logs the panic
- **Multi-stage Docker build** — Alpine base, ~15MB image
- **GitHub Actions CI** — test + migrate + docker build on every push and PR
- **GitHub Actions CD** — GHCR push + auto-migrate + deploy hook on merge to main
- **Air live reload** — `make dev` watches `internal/`, `cmd/`, `migrations/`

---

## Tech stack

| Concern | Library |
|---|---|
| HTTP | `gin-gonic/gin` v1.10.0 |
| PostgreSQL | `jackc/pgx/v5` v5.5.4 (raw SQL, no ORM) |
| Redis | `redis/go-redis/v9` v9.3.0 |
| SQL codegen | `sqlc-dev/sqlc` (optional, queries pre-written) |
| Migrations | `pressly/goose/v3` v3.18.0 |
| Logging | `uber-go/zap` v1.26.0 |
| JWT | `golang-jwt/jwt/v5` v5.2.1 |
| TOTP | `pquerna/otp` v1.4.0 |
| Password hashing | `golang.org/x/crypto/bcrypt` (cost 12) |
| Config | `caarlos0/env/v10` |
| UUIDs | `google/uuid` v1.5.0 |

---

## Quick start

```bash
git clone https://github.com/your-org/go-with-me.git
cd go-with-me

cp .env.example .env
# Edit .env — at minimum set APP_SECRET and DATABASE_WRITE_URL

make dev-start
# Creates the DB, runs migrations, starts the server on :8080
```

The first startup creates a `super_admin` using `BOOTSTRAP_ADMIN_EMAIL` and `BOOTSTRAP_ADMIN_PASSWORD` from your `.env`. After that, you can log in and begin using the API.

---

## Configuration reference

All configuration is via environment variables. See `.env.example` for the full set.

| Variable | Default | Required | Description |
|---|---|---|---|
| `HTTP_PORT` | `8080` | No | HTTP server port |
| `APP_ENV` | `development` | No | `development` or `production` (affects log format) |
| `DATABASE_WRITE_URL` | — | Yes | PostgreSQL DSN for the write pool |
| `DATABASE_READ_URLS` | — | No | Comma-separated read replica DSNs |
| `REDIS_URL` | — | Yes | Redis URL |
| `APP_SECRET` | — | Yes | JWT signing secret (min 32 chars) |
| `EVENT_STREAM` | `app:events` | No | Redis stream name for domain events |
| `JWT_EXPIRY_HOURS` | `24` | No | Session token lifetime |
| `MFA_EXPIRY_MINUTES` | `10` | No | MFA token lifetime |
| `TOTP_SETUP_EXPIRY_MINUTES` | `60` | No | TOTP setup token lifetime |
| `PASSWORD_RESET_OTP_MINUTES` | `15` | No | Password reset OTP lifetime |
| `PASSWORD_RESET_JWT_MINUTES` | `30` | No | Password reset JWT lifetime |
| `MAX_FAILED_ATTEMPTS` | `5` | No | Lock account after N failures |
| `INVITE_TTL_HOURS` | `168` | No | Invite token lifetime (7 days) |
| `BOOTSTRAP_ADMIN_EMAIL` | — | No | Super admin email (first-run only) |
| `BOOTSTRAP_ADMIN_PASSWORD` | — | No | Super admin password (first-run only) |
| `BOOTSTRAP_ADMIN_NAME` | `Super Admin` | No | Super admin display name |
| `SMTP_HOST` | — | No | SMTP server host |
| `SMTP_PORT` | `587` | No | SMTP port |
| `SMTP_USER` | — | No | SMTP username |
| `SMTP_PASSWORD` | — | No | SMTP password |
| `SMTP_FROM` | — | No | From address for outbound email |

In development (`APP_ENV != production`), OTPs are logged to the console instead of sent via SMTP.
