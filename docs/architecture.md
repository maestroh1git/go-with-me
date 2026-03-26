# Architecture

## Project structure

```
go-with-me/
├── cmd/
│   ├── server/main.go          # All-in-one entrypoint (HTTP + worker + scheduler)
│   ├── api/main.go             # API-only entrypoint
│   └── worker/main.go          # Worker-only entrypoint
│
├── internal/
│   ├── config/
│   │   └── config.go           # Env-based config (caarlos0/env)
│   │
│   ├── lib/
│   │   ├── apierror.go         # APIError type + constructors
│   │   ├── response.go         # JSON response helpers (Success, Fail, Created, NoContent)
│   │   ├── pagination.go       # Params, Meta, ParseParams
│   │   └── logger.go           # Zap logger factory
│   │
│   ├── auth/
│   │   └── token.go            # JWT sign/parse, purpose-scoped token issuers
│   │
│   ├── models/
│   │   ├── user.go             # User, UserRole, RolePermissions
│   │   ├── audit.go            # AuditLog
│   │   └── password_reset.go   # PasswordReset
│   │
│   ├── repository/
│   │   ├── interfaces/         # Repository contracts (interfaces only)
│   │   │   ├── user.go
│   │   │   ├── audit.go
│   │   │   └── password_reset.go
│   │   ├── implementation/     # Raw pgx implementations
│   │   │   ├── user.go
│   │   │   ├── audit.go
│   │   │   └── password_reset.go
│   │   └── sqlc/               # (generated) SQLC output — not used by default
│   │
│   ├── bootstrap/
│   │   ├── app.go              # App struct — wires and runs everything
│   │   ├── database.go         # pgxpool connect + round-robin read pool
│   │   ├── redis.go            # Redis connect + ping
│   │   ├── repositories.go     # Repositories struct
│   │   └── services.go         # Services struct
│   │
│   ├── services/
│   │   ├── auth_service.go     # Login, TOTP, password reset, invite, bootstrap admin
│   │   └── user_service.go     # List, Get, Update, Invite, Deactivate
│   │
│   ├── api/
│   │   ├── middleware/
│   │   │   ├── request_id.go   # UUID request ID → X-Request-ID header
│   │   │   ├── request_logger.go # Zap request log
│   │   │   ├── recovery.go     # Panic → 500 JSON
│   │   │   ├── auth.go         # RequireAuth, RequireMFA, RequirePurpose
│   │   │   ├── require_role.go # RequireRole, RequirePermission
│   │   │   ├── rate_limit.go   # Redis sliding-window rate limiter
│   │   │   └── audit.go        # Audit log middleware
│   │   ├── handlers/
│   │   │   ├── auth_handler.go # HTTP handlers for all auth flows
│   │   │   └── user_handler.go # HTTP handlers for user management
│   │   └── routes/
│   │       └── routes.go       # Route registration
│   │
│   ├── events/
│   │   ├── publisher.go        # Redis Streams publisher
│   │   └── types.go            # Event struct + type constants
│   │
│   └── jobs/
│   │   ├── worker.go           # Concurrent named job runner
│   │   └── scheduler.go        # Interval-based job scheduler
│
├── migrations/                 # Goose SQL migrations (run in order)
│   ├── 001_enums.sql
│   ├── 002_users.sql
│   └── 003_audit_logs.sql
│
├── queries/                    # SQLC annotated queries (for gen-sqlc)
│   └── users.sql
│
├── sqlc.yaml                   # SQLC config
├── Makefile                    # All dev commands
├── Dockerfile                  # Multi-stage Alpine build
├── .air.toml                   # Live reload config
├── .env.example                # Config template
└── .github/workflows/
    ├── ci.yml                  # Test + migrate + build on PR
    └── cd.yml                  # Push to GHCR + migrate + deploy on main
```

---

## Scaling

The API is stateless by design. Scale horizontally:

- **API**: run N replicas behind a load balancer — no shared state except Redis + PostgreSQL
- **Workers**: run a single worker process (or multiple if your jobs are idempotent)
- **Database**: promote a read replica and add its URL to `DATABASE_READ_URLS` — the app will start using it for reads immediately
- **Events**: Redis Streams provide durable, fan-out event delivery — add consumers without changing publishers

---

## Security model

- **Passwords**: bcrypt with cost 12 (`golang.org/x/crypto/bcrypt`)
- **TOTP**: HMAC-SHA1 per RFC 6238 (`pquerna/otp`)
- **JWT**: HS256 signed with `APP_SECRET` — tokens carry a `purpose` claim that middleware enforces, preventing token reuse across auth flows
- **Rate limiting**: Redis INCR + EXPIRE sliding window — fail-open on Redis unavailability
- **Audit trail**: immutable `audit_logs` table — no UPDATE or DELETE in repository implementation
- **Account lockout**: `MAX_FAILED_ATTEMPTS` failures lock the account for 30 minutes
- **Anti-enumeration**: `/auth/forgot-password` always returns 200 regardless of whether the email exists
