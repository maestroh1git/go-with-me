package implementation

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/google/uuid"

	"github.com/go-with-me/app/internal/lib"
	"github.com/go-with-me/app/internal/models"
	"github.com/go-with-me/app/internal/repository/interfaces"
)

type passwordResetRepository struct {
	write *pgxpool.Pool
	read  func() *pgxpool.Pool
}

// NewPasswordResetRepository creates a PostgreSQL password reset repository.
func NewPasswordResetRepository(write *pgxpool.Pool, readPool func() *pgxpool.Pool) interfaces.IPasswordResetRepository {
	return &passwordResetRepository{write: write, read: readPool}
}

func (r *passwordResetRepository) Create(ctx context.Context, params interfaces.CreatePasswordResetParams) (*models.PasswordReset, error) {
	// Remove any existing pending reset for this user before creating a new one.
	_, _ = r.write.Exec(ctx, `DELETE FROM password_resets WHERE user_id = $1`, params.UserID)

	const q = `
		INSERT INTO password_resets (user_id, otp_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, otp_hash, token, expires_at, used_at, created_at`

	row := r.write.QueryRow(ctx, q, params.UserID, params.OTPHash, params.ExpiresAt)
	return scanPasswordReset(row)
}

func (r *passwordResetRepository) FindByUserID(ctx context.Context, userID uuid.UUID) (*models.PasswordReset, error) {
	const q = `
		SELECT id, user_id, otp_hash, token, expires_at, used_at, created_at
		FROM password_resets
		WHERE user_id = $1 AND used_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1`

	row := r.read().QueryRow(ctx, q, userID)
	pr, err := scanPasswordReset(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, lib.ErrNotFound
		}
		return nil, fmt.Errorf("find password reset by user: %w", err)
	}
	return pr, nil
}

func (r *passwordResetRepository) SetToken(ctx context.Context, id uuid.UUID, token string) error {
	const q = `UPDATE password_resets SET token = $1 WHERE id = $2`
	cmd, err := r.write.Exec(ctx, q, token, id)
	if err != nil {
		return fmt.Errorf("set reset token: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return lib.ErrNotFound
	}
	return nil
}

func (r *passwordResetRepository) Consume(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE password_resets SET used_at = NOW() WHERE id = $1 AND used_at IS NULL`
	cmd, err := r.write.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("consume password reset: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return lib.ErrNotFound
	}
	return nil
}

func (r *passwordResetRepository) DeleteExpired(ctx context.Context) error {
	const q = `DELETE FROM password_resets WHERE expires_at < NOW()`
	_, err := r.write.Exec(ctx, q)
	if err != nil {
		return fmt.Errorf("delete expired resets: %w", err)
	}
	return nil
}

func scanPasswordReset(row pgx.Row) (*models.PasswordReset, error) {
	pr := &models.PasswordReset{}
	err := row.Scan(
		&pr.ID,
		&pr.UserID,
		&pr.OTPHash,
		&pr.Token,
		&pr.ExpiresAt,
		&pr.UsedAt,
		&pr.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return pr, nil
}
