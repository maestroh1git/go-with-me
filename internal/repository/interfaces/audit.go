package interfaces

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/go-with-me/app/internal/lib"
	"github.com/go-with-me/app/internal/models"
)

// IAuditRepository provides append-only audit log persistence.
// Implementations must never UPDATE or DELETE rows.
type IAuditRepository interface {
	// Create inserts an audit log entry. Non-blocking best-effort: callers should
	// not fail their primary operation if this returns an error.
	Create(ctx context.Context, params CreateAuditParams) error

	// List returns a filtered, paginated list of audit logs.
	List(ctx context.Context, params ListAuditParams) ([]*models.AuditLog, int64, error)
}

// CreateAuditParams holds the fields for a new audit log entry.
type CreateAuditParams struct {
	UserID     *uuid.UUID
	Action     string
	Resource   string
	ResourceID *string
	IPAddress  string
	UserAgent  string
	Metadata   json.RawMessage
}

// ListAuditParams holds filters and pagination for listing audit logs.
type ListAuditParams struct {
	UserID   *uuid.UUID
	Resource *string
	lib.Params
}
