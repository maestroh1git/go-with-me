package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// AuditLog is an immutable record of a user action on a resource.
// Audit rows are never updated or deleted.
type AuditLog struct {
	ID         uuid.UUID
	UserID     *uuid.UUID
	Action     string
	Resource   string
	ResourceID *string
	IPAddress  string
	UserAgent  string
	Metadata   json.RawMessage
	CreatedAt  time.Time
}
