package implementation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/google/uuid"

	"github.com/go-with-me/app/internal/lib"
	"github.com/go-with-me/app/internal/models"
	"github.com/go-with-me/app/internal/repository/interfaces"
)

type userRepository struct {
	write *pgxpool.Pool
	read  func() *pgxpool.Pool
}

// NewUserRepository creates a PostgreSQL user repository.
// readPool is a function that returns the pool to use for read queries (supports round-robin replicas).
func NewUserRepository(write *pgxpool.Pool, readPool func() *pgxpool.Pool) interfaces.IUserRepository {
	return &userRepository{write: write, read: readPool}
}

func (r *userRepository) Create(ctx context.Context, params interfaces.CreateUserParams) (*models.User, error) {
	const q = `
		INSERT INTO users (email, name, password_hash, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id, email, name, password_hash, role, totp_secret, totp_enabled,
		          failed_attempts, locked_until, is_active, created_at, updated_at`

	row := r.write.QueryRow(ctx, q,
		strings.ToLower(strings.TrimSpace(params.Email)),
		strings.TrimSpace(params.Name),
		params.PasswordHash,
		params.Role,
	)
	return scanUser(row)
}

func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	const q = `
		SELECT id, email, name, password_hash, role, totp_secret, totp_enabled,
		       failed_attempts, locked_until, is_active, created_at, updated_at
		FROM users WHERE id = $1`
	row := r.read().QueryRow(ctx, q, id)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, lib.ErrNotFound
		}
		return nil, err
	}
	return u, nil
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	const q = `
		SELECT id, email, name, password_hash, role, totp_secret, totp_enabled,
		       failed_attempts, locked_until, is_active, created_at, updated_at
		FROM users WHERE email = $1`
	row := r.read().QueryRow(ctx, q, strings.ToLower(strings.TrimSpace(email)))
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, lib.ErrNotFound
		}
		return nil, err
	}
	return u, nil
}

func (r *userRepository) List(ctx context.Context, params interfaces.ListUsersParams) ([]*models.User, int64, error) {
	var conditions []string
	var args []any
	argIdx := 1

	if params.Role != nil {
		conditions = append(conditions, fmt.Sprintf("role = $%d", argIdx))
		args = append(args, *params.Role)
		argIdx++
	}
	if params.Search != nil && *params.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(name ILIKE $%d OR email ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+*params.Search+"%")
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM users %s", where)
	var total int64
	if err := r.read().QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	dataQuery := fmt.Sprintf(`
		SELECT id, email, name, password_hash, role, totp_secret, totp_enabled,
		       failed_attempts, locked_until, is_active, created_at, updated_at
		FROM users %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)

	args = append(args, params.PageSize, params.Offset())
	rows, err := r.read().Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		u, err := scanUserRow(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan user row: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate user rows: %w", err)
	}
	return users, total, nil
}

func (r *userRepository) Update(ctx context.Context, id uuid.UUID, params interfaces.UpdateUserParams) (*models.User, error) {
	var setClauses []string
	var args []any
	argIdx := 1

	if params.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, strings.TrimSpace(*params.Name))
		argIdx++
	}
	if params.Role != nil {
		setClauses = append(setClauses, fmt.Sprintf("role = $%d", argIdx))
		args = append(args, *params.Role)
		argIdx++
	}

	if len(setClauses) == 0 {
		return r.FindByID(ctx, id)
	}

	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argIdx))
	args = append(args, time.Now().UTC())
	argIdx++

	args = append(args, id)
	q := fmt.Sprintf(`
		UPDATE users SET %s
		WHERE id = $%d
		RETURNING id, email, name, password_hash, role, totp_secret, totp_enabled,
		          failed_attempts, locked_until, is_active, created_at, updated_at`,
		strings.Join(setClauses, ", "), argIdx)

	row := r.write.QueryRow(ctx, q, args...)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, lib.ErrNotFound
		}
		return nil, fmt.Errorf("update user: %w", err)
	}
	return u, nil
}

func (r *userRepository) UpdatePassword(ctx context.Context, id uuid.UUID, hash string) error {
	const q = `UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2`
	cmd, err := r.write.Exec(ctx, q, hash, id)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return lib.ErrNotFound
	}
	return nil
}

func (r *userRepository) IncrFailedAttempts(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE users SET failed_attempts = failed_attempts + 1, updated_at = NOW() WHERE id = $1`
	_, err := r.write.Exec(ctx, q, id)
	return err
}

func (r *userRepository) LockUntil(ctx context.Context, id uuid.UUID, until time.Time) error {
	const q = `UPDATE users SET locked_until = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.write.Exec(ctx, q, until, id)
	return err
}

func (r *userRepository) ClearLock(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE users SET failed_attempts = 0, locked_until = NULL, updated_at = NOW() WHERE id = $1`
	_, err := r.write.Exec(ctx, q, id)
	return err
}

func (r *userRepository) SetTOTP(ctx context.Context, id uuid.UUID, secret string, enabled bool) error {
	const q = `UPDATE users SET totp_secret = $1, totp_enabled = $2, updated_at = NOW() WHERE id = $3`
	_, err := r.write.Exec(ctx, q, secret, enabled, id)
	return err
}

func (r *userRepository) Deactivate(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE users SET is_active = FALSE, updated_at = NOW() WHERE id = $1`
	cmd, err := r.write.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("deactivate user: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return lib.ErrNotFound
	}
	return nil
}

func (r *userRepository) CountSuperAdmins(ctx context.Context) (int64, error) {
	const q = `SELECT COUNT(*) FROM users WHERE role = 'super_admin' AND is_active = TRUE`
	var count int64
	if err := r.read().QueryRow(ctx, q).Scan(&count); err != nil {
		return 0, fmt.Errorf("count super admins: %w", err)
	}
	return count, nil
}

// scanUser scans a single user row from a pgx.Row (QueryRow result).
func scanUser(row pgx.Row) (*models.User, error) {
	u := &models.User{}
	err := row.Scan(
		&u.ID,
		&u.Email,
		&u.Name,
		&u.PasswordHash,
		&u.Role,
		&u.TOTPSecret,
		&u.TOTPEnabled,
		&u.FailedAttempts,
		&u.LockedUntil,
		&u.IsActive,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// scanUserRow scans a user from pgx.Rows (multi-row Query result).
func scanUserRow(rows pgx.Rows) (*models.User, error) {
	u := &models.User{}
	err := rows.Scan(
		&u.ID,
		&u.Email,
		&u.Name,
		&u.PasswordHash,
		&u.Role,
		&u.TOTPSecret,
		&u.TOTPEnabled,
		&u.FailedAttempts,
		&u.LockedUntil,
		&u.IsActive,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return u, nil
}
