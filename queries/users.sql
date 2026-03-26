-- SQLC annotated queries for the users domain.
-- These are for reference and future use with `make gen-sqlc`.
-- The current implementation uses raw pgx queries in internal/repository/implementation/.

-- name: CreateUser :one
INSERT INTO users (email, name, password_hash, role)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUserByID :one
SELECT *
FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT *
FROM users
WHERE email = $1;

-- name: ListUsers :many
SELECT *
FROM users
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: UpdateUser :one
UPDATE users
SET name       = COALESCE(sqlc.narg(name), name),
    role       = COALESCE(sqlc.narg(role), role),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = $2,
    updated_at    = NOW()
WHERE id = $1;

-- name: IncrUserFailedAttempts :exec
UPDATE users
SET failed_attempts = failed_attempts + 1,
    updated_at      = NOW()
WHERE id = $1;

-- name: LockUserUntil :exec
UPDATE users
SET locked_until = $2,
    updated_at   = NOW()
WHERE id = $1;

-- name: ClearUserLock :exec
UPDATE users
SET failed_attempts = 0,
    locked_until    = NULL,
    updated_at      = NOW()
WHERE id = $1;

-- name: SetUserTOTP :exec
UPDATE users
SET totp_secret  = $2,
    totp_enabled = $3,
    updated_at   = NOW()
WHERE id = $1;

-- name: DeactivateUser :exec
UPDATE users
SET is_active  = FALSE,
    updated_at = NOW()
WHERE id = $1;

-- name: CountSuperAdmins :one
SELECT COUNT(*)
FROM users
WHERE role = 'super_admin' AND is_active = TRUE;
