package models

import (
	"time"

	"github.com/google/uuid"
)

// UserRole represents the access level of a user.
type UserRole string

const (
	RoleSuperAdmin UserRole = "super_admin"
	RoleAdmin      UserRole = "admin"
	RoleManager    UserRole = "manager"
	RoleMember     UserRole = "member"
	RoleViewer     UserRole = "viewer"
)

// User is the core user domain model.
type User struct {
	ID             uuid.UUID
	Email          string
	Name           string
	PasswordHash   string
	Role           UserRole
	TOTPSecret     *string
	TOTPEnabled    bool
	FailedAttempts int
	LockedUntil    *time.Time
	IsActive       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// IsLocked returns true if the user account is currently locked out.
func (u *User) IsLocked() bool {
	if u.LockedUntil == nil {
		return false
	}
	return time.Now().UTC().Before(*u.LockedUntil)
}

// HasPermission returns true if the user holds the given permission string.
// The wildcard permission "*" grants all access (super_admin).
func (u *User) HasPermission(perm string) bool {
	for _, p := range RolePermissions(u.Role) {
		if p == "*" || p == perm {
			return true
		}
	}
	return false
}

// RolePermissions returns the default set of permissions for a given role.
func RolePermissions(role UserRole) []string {
	switch role {
	case RoleSuperAdmin:
		return []string{"*"}
	case RoleAdmin:
		return []string{
			"users:read",
			"users:write",
			"users:invite",
			"audit:read",
		}
	case RoleManager:
		return []string{
			"users:read",
			"users:invite",
		}
	case RoleMember:
		return []string{
			"profile:read",
			"profile:write",
		}
	case RoleViewer:
		return []string{
			"profile:read",
		}
	default:
		return []string{}
	}
}
