BINARY_NAME=server
MODULE=github.com/go-with-me/app

-include .env
-include .env.local
.EXPORT_ALL_VARIABLES:

DB_DSN?=$(DATABASE_WRITE_URL)

.PHONY: init tidy build build-local build-api build-worker run dev dev-start \
        test test-verbose test-cover lint fmt vet \
        db-create db-drop db-reset \
        migrate-make migrate-up migrate-down migrate-status \
        gen-sqlc gen-mocks \
        docker-build docker-run

# ── First-run ─────────────────────────────────────────────────────────────────

init: tidy db-create migrate-up gen-sqlc
	@echo "Setup complete. Run 'make run' to start the server."

# ── Dependencies ──────────────────────────────────────────────────────────────

tidy:
	go mod tidy

# ── Build ─────────────────────────────────────────────────────────────────────

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o bin/$(BINARY_NAME) ./cmd/server

build-local:
	go build -o bin/$(BINARY_NAME) ./cmd/server

build-api:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o bin/$(BINARY_NAME)-api ./cmd/api

build-worker:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o bin/$(BINARY_NAME)-worker ./cmd/worker

# ── Run ───────────────────────────────────────────────────────────────────────

run:
	go run ./cmd/server

run-api:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

# ── Dev (live reload) ─────────────────────────────────────────────────────────

dev:
	@AIR_BIN=$$(command -v air || true); \
		if [ -z "$$AIR_BIN" ] && [ -x "$$(go env GOPATH)/bin/air" ]; then AIR_BIN="$$(go env GOPATH)/bin/air"; fi; \
		if [ -n "$$AIR_BIN" ]; then \
			"$$AIR_BIN" -c .air.toml; \
		else \
			echo "air not found; installing..."; \
			go install github.com/air-verse/air@latest; \
			"$$(go env GOPATH)/bin/air" -c .air.toml; \
		fi

# Create DB, run migrations, and start the server (first-time dev setup).
dev-start: db-create migrate-up run

# ── Test ──────────────────────────────────────────────────────────────────────

test:
	go test ./... -count=1

test-verbose:
	go test ./... -v -count=1

test-cover:
	go test ./... -coverprofile=coverage.out && go tool cover -html=coverage.out

# ── Lint / Format ─────────────────────────────────────────────────────────────

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .
	@if command -v goimports >/dev/null 2>&1; then goimports -w .; fi

vet:
	go vet ./...

# ── Database ──────────────────────────────────────────────────────────────────

db-create:
	@if [ -z "$(DB_DSN)" ]; then echo "Set DATABASE_WRITE_URL in .env"; exit 1; fi
	@if ! command -v psql >/dev/null 2>&1; then echo "psql not found — install PostgreSQL client tools"; exit 1; fi
	@DB_NAME=$$(echo "$(DB_DSN)" | sed -E 's|.*/([^/?]+)(\?.*)?$$|\1|'); \
		ADMIN_DSN=$$(echo "$(DB_DSN)" | sed -E 's|/[^/?]+(\?.*)?$$|/postgres\1|'); \
		EXISTS=$$(psql "$$ADMIN_DSN" -tAc "SELECT 1 FROM pg_database WHERE datname='$$DB_NAME'"); \
		if [ "$$EXISTS" = "1" ]; then \
			echo "Database '$$DB_NAME' already exists"; \
		else \
			echo "Creating database '$$DB_NAME'"; \
			psql "$$ADMIN_DSN" -c "CREATE DATABASE \"$$DB_NAME\""; \
		fi

db-drop:
	@if [ -z "$(DB_DSN)" ]; then echo "Set DATABASE_WRITE_URL in .env"; exit 1; fi
	@if ! command -v psql >/dev/null 2>&1; then echo "psql not found — install PostgreSQL client tools"; exit 1; fi
	@DB_NAME=$$(echo "$(DB_DSN)" | sed -E 's|.*/([^/?]+)(\?.*)?$$|\1|'); \
		ADMIN_DSN=$$(echo "$(DB_DSN)" | sed -E 's|/[^/?]+(\?.*)?$$|/postgres\1|'); \
		echo "Dropping database '$$DB_NAME' if it exists"; \
		psql "$$ADMIN_DSN" -c "DROP DATABASE IF EXISTS \"$$DB_NAME\" WITH (FORCE)"

db-reset: db-drop db-create migrate-up gen-sqlc

# ── Migrations (goose) ────────────────────────────────────────────────────────

migrate-make:
	@if [ -z "$(name)" ]; then echo "Usage: make migrate-make name=create_something"; exit 1; fi
	goose -dir migrations create $(name) sql

migrate-up:
	@if [ -z "$(DB_DSN)" ]; then echo "Set DATABASE_WRITE_URL in .env"; exit 1; fi
	goose -dir migrations postgres "$(DB_DSN)" up

migrate-down:
	@if [ -z "$(DB_DSN)" ]; then echo "Set DATABASE_WRITE_URL in .env"; exit 1; fi
	goose -dir migrations postgres "$(DB_DSN)" down

migrate-status:
	@if [ -z "$(DB_DSN)" ]; then echo "Set DATABASE_WRITE_URL in .env"; exit 1; fi
	goose -dir migrations postgres "$(DB_DSN)" status

# ── Code Generation ───────────────────────────────────────────────────────────

gen-sqlc:
	sqlc generate

gen-mocks:
	mockery --dir internal/repository/interfaces --all --output internal/repository/mocks

# ── Docker ────────────────────────────────────────────────────────────────────

docker-build:
	docker build -t go-with-me:local .

docker-run:
	docker run --rm -p 8080:8080 --env-file .env go-with-me:local
