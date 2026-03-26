# go-with-me

A production-ready Go boilerplate for any backend service. Built on patterns extracted from a real Nigerian fintech system running in production. Batteries included — clone it, rename it, ship it.

---

## What is this?

go-with-me is an opinionated starting point for Go backend services. It is not a framework. It is a complete, working application with a real architecture that you copy and adapt. Every pattern in here has been tested in production.

The "Users" domain ships out of the box and demonstrates every pattern: models, repository interfaces, raw pgx implementations, service layer, JWT auth, middleware, handlers, and routes. Adding a new domain (Products, Orders, Devices, anything) is a matter of following the same pattern.

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

## API reference

### Auth endpoints

| Method | Path | Auth required | Rate limit |
|---|---|---|---|
| POST | `/auth/login` | None | 30/min |
| POST | `/auth/verify-totp` | MFA token | — |
| POST | `/auth/totp/setup` | TOTP setup token | — |
| POST | `/auth/totp/confirm` | TOTP setup token | — |
| POST | `/auth/forgot-password` | None | 10/min |
| POST | `/auth/verify-reset-email` | None | 10/min |
| POST | `/auth/complete-password-reset` | Password reset token | — |
| POST | `/auth/accept-invite` | None | 20/min |
| GET | `/auth/me` | Session token | — |

### User endpoints

| Method | Path | Auth required | Roles | Rate limit |
|---|---|---|---|---|
| GET | `/api/v1/users` | Session | admin, super_admin | 100/min |
| GET | `/api/v1/users/:id` | Session | admin, super_admin, manager | — |
| PATCH | `/api/v1/users/:id` | Session | admin, super_admin | — |
| POST | `/api/v1/users/invite` | Session | admin, super_admin | 20/min |
| DELETE | `/api/v1/users/:id` | Session | super_admin | — |

### Health probes

| Method | Path | Auth required |
|---|---|---|
| GET | `/health` | None |
| GET | `/ready` | None |

---

## Auth flows

### Login flow

```
POST /auth/login (email + password)
  │
  ├── Invalid credentials → 401
  ├── Account locked → 400
  │
  ├── TOTP enrolled:
  │     → { mfa_required: true, mfa_token: "..." }
  │     POST /auth/verify-totp (mfa_token + code)
  │     → { access_token, expires_at }
  │
  └── TOTP not set up (mandatory 2FA):
        → { totp_setup_required: true, setup_token: "..." }
        POST /auth/totp/setup (Bearer: setup_token)
        → { secret, otpauth_url }
        POST /auth/totp/confirm (Bearer: setup_token, secret + code)
        → { access_token, expires_at }
```

### Password reset flow

```
POST /auth/forgot-password (email)
  → Always 200 (anti-enumeration)
  → OTP emailed (logged to console in development)

POST /auth/verify-reset-email (email + otp)
  → { reset_token: "..." }

POST /auth/complete-password-reset (Bearer: reset_token, new_password [+ totp_code])
  → { message: "password updated" }
```

### Invite flow

```
POST /api/v1/users/invite (admin session, email + name + role)
  → { invite_token: "..." }
  [Token delivered to user via email]

POST /auth/accept-invite (invite_token + name + password)
  → { totp_setup_required: true, setup_token: "..." }
  [User then completes TOTP setup flow]
```

---

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
│       ├── worker.go           # Concurrent named job runner
│       └── scheduler.go        # Interval-based job scheduler
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

## How to add a new domain

Example: adding a `products` domain.

**1. Write the migration**

```bash
make migrate-make name=create_products
# Edit migrations/004_YYYYMMDDHHMMSS_create_products.sql
make migrate-up
```

```sql
-- +goose Up
CREATE TABLE products (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT        NOT NULL,
    sku         TEXT        NOT NULL UNIQUE,
    price_cents BIGINT      NOT NULL DEFAULT 0,
    is_active   BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose Down
DROP TABLE IF EXISTS products;
```

**2. Add the model**

```go
// internal/models/product.go
type Product struct {
    ID         uuid.UUID
    Name       string
    SKU        string
    PriceCents int64
    IsActive   bool
    CreatedAt  time.Time
    UpdatedAt  time.Time
}
```

**3. Add the repository interface**

```go
// internal/repository/interfaces/product.go
type IProductRepository interface {
    Create(ctx context.Context, params CreateProductParams) (*models.Product, error)
    FindByID(ctx context.Context, id uuid.UUID) (*models.Product, error)
    List(ctx context.Context, params ListProductsParams) ([]*models.Product, int64, error)
    Update(ctx context.Context, id uuid.UUID, params UpdateProductParams) (*models.Product, error)
    Deactivate(ctx context.Context, id uuid.UUID) error
}
```

**4. Write the implementation**

```go
// internal/repository/implementation/product.go
type productRepository struct {
    write *pgxpool.Pool
    read  func() *pgxpool.Pool
}

func NewProductRepository(write *pgxpool.Pool, readPool func() *pgxpool.Pool) interfaces.IProductRepository {
    return &productRepository{write: write, read: readPool}
}
// ... implement each method with raw SQL
```

**5. Add the service**

```go
// internal/services/product_service.go
type ProductService struct { /* repos, logger */ }
func (s *ProductService) List(ctx, params) ([]*models.Product, int64, error) { ... }
```

**6. Add the handler**

```go
// internal/api/handlers/product_handler.go
type ProductHandler struct { products *services.ProductService }
func (h *ProductHandler) List(c *gin.Context)   { ... }
func (h *ProductHandler) Create(c *gin.Context) { ... }
```

**7. Register in routes**

```go
// internal/api/routes/routes.go — inside Setup()
setupProductRoutes(r, cfg, rdb, svcs.Products, auditRepo, logger)
```

**8. Wire into bootstrap**

```go
// internal/bootstrap/repositories.go
type Repositories struct {
    // ... existing
    Products interfaces.IProductRepository
}
// internal/bootstrap/services.go
type Services struct {
    // ... existing
    Products *services.ProductService
}
```

That is the complete pattern. Every domain follows it.

---

## Upgrading to SQLC

The `queries/users.sql` file contains SQLC-annotated queries for all user operations. To switch from raw pgx to SQLC-generated code:

1. Run `make gen-sqlc` — outputs typed Go code to `internal/repository/sqlc/`
2. Rewrite each repository implementation to call the generated `Queries` struct instead of writing raw SQL
3. The interfaces stay the same — no changes to services, handlers, or routes

The raw pgx approach is easier to start with and debug. SQLC becomes valuable once you have many queries and want compile-time SQL checking.

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

---

## Sector examples

go-with-me is domain-agnostic. The Users domain is the starting point. Here is how you would extend it for specific industries.

### IT / SaaS

**What you add:** tenant management, feature flags, webhook delivery

**New tables:** `tenants`, `tenant_memberships`, `feature_flags`, `webhook_endpoints`, `webhook_deliveries`

**New env vars:**
```
DEFAULT_PLAN=free
WEBHOOK_SIGNING_SECRET=...
```

**New events:** `tenant.created`, `feature.enabled`, `webhook.delivered`, `webhook.failed`

**New background jobs:**
- Webhook delivery with retry (enqueue on event, worker retries up to 3 times)
- Feature flag cache warm-up on startup

**Architecture note:** add `tenant_id` to all resource tables and a middleware that resolves tenant from JWT claims or subdomain.

---

### AI / ML Platforms

**What you add:** model registry, inference job queue, usage metering

**New tables:** `ml_models`, `model_versions`, `inference_jobs`, `usage_records`

**New env vars:**
```
INFERENCE_TIMEOUT_SECONDS=30
QUOTA_MAX_REQUESTS_PER_DAY=1000
GPU_WORKER_COUNT=4
```

**New events:** `inference.queued`, `inference.completed`, `inference.failed`, `quota.exceeded`

**New background jobs:**
- Inference worker: polls `inference_jobs` where `status = 'queued'`, calls model endpoint, writes result
- Usage rollup: aggregates `usage_records` hourly for billing
- Quota reset: scheduler resets daily counters at midnight

**Architecture note:** inference jobs can be expensive — use the Worker to run them async rather than blocking HTTP handlers.

---

### Logistics / Delivery

**What you add:** driver management, shipment tracking, route optimisation jobs

**New tables:** `drivers`, `vehicles`, `shipments`, `waypoints`, `delivery_windows`

**New env vars:**
```
MAPS_API_KEY=...
ETA_REFRESH_INTERVAL_MINUTES=5
GEOFENCE_RADIUS_METERS=100
```

**New events:** `shipment.created`, `shipment.assigned`, `shipment.dispatched`, `shipment.delivered`, `driver.location_updated`

**New background jobs:**
- ETA recalculation: scheduler refreshes ETAs for in-transit shipments every 5 minutes using Maps API
- Delayed delivery alert: checks for shipments past their delivery window and notifies operations
- Driver location consumer: Redis Streams consumer reading from a device telemetry stream

---

### Agriculture

**What you add:** farm management, crop cycles, IoT sensor readings, weather-based alerts

**New tables:** `farms`, `plots`, `crop_cycles`, `sensor_devices`, `sensor_readings`, `weather_snapshots`

**New env vars:**
```
WEATHER_API_KEY=...
SENSOR_INGEST_STREAM=sensors:readings
ALERT_EMAIL=ops@farm.example.com
```

**New events:** `sensor.reading`, `alert.soil_moisture_low`, `alert.temperature_critical`, `cycle.harvested`

**New background jobs:**
- Sensor ingestion: Redis Streams consumer reading from `SENSOR_INGEST_STREAM`, writes to `sensor_readings`
- Weather snapshot: scheduler fetches current weather for each farm's coordinates every hour
- Anomaly detection: compares readings against crop-specific thresholds, emits alerts

**Architecture note:** sensor data is high-volume — use Redis Streams for ingest, write to PostgreSQL in batches rather than per-reading.

---

### Education / EdTech

**What you add:** courses, enrollments, lesson progress, certificates, scheduled reminders

**New tables:** `courses`, `lessons`, `enrollments`, `lesson_completions`, `certificates`, `reminders`

**New env vars:**
```
CERTIFICATE_SIGNING_KEY=...
REMINDER_LEAD_DAYS=3
```

**New events:** `enrollment.created`, `lesson.completed`, `course.completed`, `certificate.issued`

**New background jobs:**
- Certificate generator: listens on `course.completed`, generates PDF, stores in S3, emails learner
- Reminder scheduler: daily job finds enrollments with upcoming deadlines and enqueues reminder emails
- Progress rollup: aggregates completion percentages per enrollment for dashboard widgets

---

### Healthcare

**What you add:** patients, appointments, medical records, prescriptions, reminder notifications

**New tables:** `patients`, `appointments`, `medical_records`, `prescriptions`, `appointment_reminders`

**New env vars:**
```
RECORD_ENCRYPTION_KEY=...  # AES-256 for PII at rest
SMS_PROVIDER_API_KEY=...
APPOINTMENT_REMINDER_HOURS=24
```

**New events:** `appointment.booked`, `appointment.cancelled`, `appointment.reminder_sent`, `prescription.issued`

**New background jobs:**
- Appointment reminder: daily scheduler finds appointments in next 24 hours, sends SMS/email
- Record archival: monthly job archives records older than retention period to cold storage

**Architecture note:** encrypt `medical_records.content` at the application layer using AES-256 before storing — the encryption key must not be in the DB. Add a field-level encryption helper in `internal/lib/`.

---

### Construction / Project Management

**What you add:** projects, tasks, materials, inspections, milestone alerts

**New tables:** `projects`, `tasks`, `task_assignments`, `materials`, `inspections`, `milestones`

**New env vars:**
```
MILESTONE_ALERT_LEAD_DAYS=7
INSPECTION_CHECKLIST_BUCKET=s3://...
```

**New events:** `project.started`, `milestone.reached`, `inspection.passed`, `inspection.failed`, `task.overdue`

**New background jobs:**
- Milestone alert: daily job checks upcoming milestones, notifies project manager
- Overdue task detector: finds tasks past due date, updates status, notifies assignees
- Inspection report generator: triggers on `inspection.passed`, compiles report PDF

---

### Energy / Utilities

**What you add:** meters, periodic readings, billing cycles, consumption analytics

**New tables:** `meters`, `meter_readings`, `billing_cycles`, `bills`, `tariff_bands`

**New env vars:**
```
BILLING_CYCLE_DAY=1        # Day of month to generate bills
METER_READING_INTERVAL_HOURS=1
```

**New events:** `meter.reading`, `bill.generated`, `bill.overdue`, `usage.anomaly`

**New background jobs:**
- Meter poller: scheduler reads from meter API every hour, inserts into `meter_readings`
- Billing cycle generator: monthly job calculates consumption, applies tariffs, generates `bills`
- Anomaly detection: compares readings against historical baseline, flags outliers

---

### IoT

**What you add:** device registry, telemetry ingestion, alert rules, command dispatch

**New tables:** `devices`, `device_groups`, `telemetry` (partitioned by time), `alert_rules`, `command_queue`

**New env vars:**
```
TELEMETRY_STREAM=iot:telemetry
COMMAND_STREAM=iot:commands
TELEMETRY_RETENTION_DAYS=90
MAX_DEVICES_PER_GROUP=1000
```

**New events:** `device.registered`, `device.online`, `device.offline`, `device.alert`, `command.dispatched`, `command.acknowledged`

**New background jobs:**
- Telemetry consumer: Redis Streams consumer reading `TELEMETRY_STREAM` in batches, writing to PostgreSQL
- Alert evaluator: per-device rule evaluation on each batch of readings
- Offline detector: scheduler checks last_seen timestamps, emits `device.offline` for stale devices
- Command dispatcher: monitors `command_queue`, sends commands to devices, tracks acknowledgement

**Architecture note:** partition `telemetry` by time (`PARTITION BY RANGE (created_at)`) and create monthly partitions via a scheduler job. Drop old partitions for retention rather than running DELETE.

---

### Manufacturing / MES

**What you add:** production orders, bill of materials, work centres, quality checks

**New tables:** `production_orders`, `bom_items`, `work_centres`, `work_order_steps`, `quality_checks`

**New env vars:**
```
QUALITY_ALERT_EMAIL=quality@factory.example.com
WIP_ALERT_THRESHOLD_HOURS=4
```

**New events:** `order.created`, `order.started`, `order.completed`, `quality.passed`, `quality.rejected`, `wip.stalled`

**New background jobs:**
- WIP monitor: scheduler checks work-in-progress older than `WIP_ALERT_THRESHOLD_HOURS`, alerts floor manager
- Capacity planner: daily job calculates centre utilisation from order schedule
- Scrap tracker: aggregates rejected items per shift for quality reporting

---

### Supply Chain

**What you add:** suppliers, purchase orders, inventory, fulfilment, reorder triggers

**New tables:** `suppliers`, `purchase_orders`, `po_line_items`, `inventory`, `reorder_rules`

**New env vars:**
```
REORDER_CHECK_INTERVAL_HOURS=6
LOW_STOCK_WEBHOOK=https://...
```

**New events:** `po.created`, `po.approved`, `po.received`, `inventory.adjusted`, `inventory.low`, `reorder.triggered`

**New background jobs:**
- Reorder checker: scheduler scans `inventory` against `reorder_rules`, creates POs or sends alerts
- Receiving reconciler: compares PO line items against received goods, flags discrepancies
- Inventory snapshot: daily job snapshots inventory levels for trend reporting

---

### Business / CRM

**What you add:** contacts, deals, pipelines, activities, follow-up reminders

**New tables:** `contacts`, `organisations`, `deals`, `pipeline_stages`, `activities`, `reminders`

**New env vars:**
```
FOLLOW_UP_LEAD_DAYS=3
DEAL_INACTIVITY_DAYS=14
```

**New events:** `contact.created`, `deal.created`, `deal.stage_changed`, `deal.won`, `deal.lost`, `activity.logged`

**New background jobs:**
- Follow-up reminder: daily scheduler finds activities due for follow-up, queues reminders
- Deal inactivity alert: flags deals with no activity in `DEAL_INACTIVITY_DAYS`, notifies owner
- Pipeline report: weekly job aggregates stage counts and conversion rates

---

## Make targets reference

```
make init            # First-run: tidy + db-create + migrate-up + gen-sqlc
make tidy            # go mod tidy
make build           # Linux/amd64 binary → bin/server
make build-local     # Native arch binary
make run             # go run ./cmd/server
make dev             # Air live reload (installs air if not found)
make dev-start       # db-create + migrate-up + run

make test            # go test ./... -count=1
make test-verbose    # go test ./... -v -count=1
make test-cover      # Coverage HTML report

make lint            # golangci-lint
make fmt             # gofmt + goimports
make vet             # go vet

make db-create       # Create database from DATABASE_WRITE_URL
make db-drop         # Drop database
make db-reset        # Drop + create + migrate-up + gen-sqlc

make migrate-make name=create_xxx   # New migration file
make migrate-up      # Apply pending migrations
make migrate-down    # Roll back last migration
make migrate-status  # Show migration status

make gen-sqlc        # Generate typed Go from queries/
make gen-mocks       # Generate mocks from repository interfaces

make docker-build    # docker build -t go-with-me:local
make docker-run      # docker run with .env file
```

---

## Extending and contributing

This is meant to be forked and adapted. Suggested changes when starting a new project:

1. Rename the module in `go.mod` from `github.com/go-with-me/app` to your own path
2. Update the binary name in `Makefile` and `Dockerfile`
3. Change the TOTP issuer name in `internal/services/auth_service.go`
4. Set `BOOTSTRAP_ADMIN_EMAIL` and `BOOTSTRAP_ADMIN_PASSWORD` before first deploy
5. Rotate `APP_SECRET` to a strong random value (`openssl rand -base64 48`)
6. Replace the deploy step in `.github/workflows/cd.yml` with your platform command

Each domain you add follows the same layered pattern. The interfaces are the contract. The implementations are swappable. The services own the rules.
