package implementation

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/go-with-me/app/internal/models"
	"github.com/go-with-me/app/internal/repository/interfaces"
)

type auditRepository struct {
	write *pgxpool.Pool
	read  func() *pgxpool.Pool
}

// NewAuditRepository creates a PostgreSQL audit log repository.
// Audit rows are INSERT-only — this implementation never issues UPDATE or DELETE.
func NewAuditRepository(write *pgxpool.Pool, readPool func() *pgxpool.Pool) interfaces.IAuditRepository {
	return &auditRepository{write: write, read: readPool}
}

func (r *auditRepository) Create(ctx context.Context, params interfaces.CreateAuditParams) error {
	const q = `
		INSERT INTO audit_logs (user_id, action, resource, resource_id, ip_address, user_agent, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := r.write.Exec(ctx, q,
		params.UserID,
		params.Action,
		params.Resource,
		params.ResourceID,
		params.IPAddress,
		params.UserAgent,
		params.Metadata,
	)
	if err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}
	return nil
}

func (r *auditRepository) List(ctx context.Context, params interfaces.ListAuditParams) ([]*models.AuditLog, int64, error) {
	var conditions []string
	var args []any
	argIdx := 1

	if params.UserID != nil {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", argIdx))
		args = append(args, *params.UserID)
		argIdx++
	}
	if params.Resource != nil && *params.Resource != "" {
		conditions = append(conditions, fmt.Sprintf("resource = $%d", argIdx))
		args = append(args, *params.Resource)
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM audit_logs %s", where)
	var total int64
	if err := r.read().QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	dataQuery := fmt.Sprintf(`
		SELECT id, user_id, action, resource, resource_id, ip_address, user_agent, metadata, created_at
		FROM audit_logs %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)

	args = append(args, params.PageSize, params.Offset())
	rows, err := r.read().Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()

	var logs []*models.AuditLog
	for rows.Next() {
		al, err := scanAuditLog(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan audit log row: %w", err)
		}
		logs = append(logs, al)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate audit log rows: %w", err)
	}
	return logs, total, nil
}

func scanAuditLog(rows pgx.Rows) (*models.AuditLog, error) {
	al := &models.AuditLog{}
	err := rows.Scan(
		&al.ID,
		&al.UserID,
		&al.Action,
		&al.Resource,
		&al.ResourceID,
		&al.IPAddress,
		&al.UserAgent,
		&al.Metadata,
		&al.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return al, nil
}
