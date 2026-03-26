package bootstrap

import (
	"github.com/go-with-me/app/internal/repository/implementation"
	"github.com/go-with-me/app/internal/repository/interfaces"
)

// Repositories is the application-wide collection of data access objects.
type Repositories struct {
	Users          interfaces.IUserRepository
	PasswordResets interfaces.IPasswordResetRepository
	Audit          interfaces.IAuditRepository
}

// NewRepositories wires up all repository implementations against the given database pools.
func NewRepositories(db *DB) *Repositories {
	readPool := db.Read

	return &Repositories{
		Users:          implementation.NewUserRepository(db.Write, readPool),
		PasswordResets: implementation.NewPasswordResetRepository(db.Write, readPool),
		Audit:          implementation.NewAuditRepository(db.Write, readPool),
	}
}
