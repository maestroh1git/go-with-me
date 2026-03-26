# Development Guide

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
