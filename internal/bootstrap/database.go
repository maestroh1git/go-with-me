package bootstrap

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// DB wraps a write pool and optional read replicas with round-robin selection.
type DB struct {
	Write    *pgxpool.Pool
	read     []*pgxpool.Pool
	rr       atomic.Uint64
}

// Read returns a read pool, cycling through replicas in round-robin order.
// Falls back to the write pool when no replicas are configured.
func (db *DB) Read() *pgxpool.Pool {
	if len(db.read) == 0 {
		return db.Write
	}
	idx := db.rr.Add(1)
	return db.read[(idx-1)%uint64(len(db.read))]
}

// Close releases all database pools.
func (db *DB) Close() {
	if db == nil {
		return
	}
	for _, r := range db.read {
		r.Close()
	}
	if db.Write != nil {
		db.Write.Close()
	}
}

// Connect creates and pings the write pool and any read replica pools.
func Connect(ctx context.Context, writeURL string, readURLs []string) (*DB, error) {
	writer, err := connectPool(ctx, writeURL)
	if err != nil {
		return nil, fmt.Errorf("connect write database: %w", err)
	}

	readers := make([]*pgxpool.Pool, 0, len(readURLs))
	for i, url := range readURLs {
		reader, err := connectPool(ctx, url)
		if err != nil {
			for _, r := range readers {
				r.Close()
			}
			writer.Close()
			return nil, fmt.Errorf("connect read database #%d: %w", i+1, err)
		}
		readers = append(readers, reader)
	}

	return &DB{Write: writer, read: readers}, nil
}

func connectPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// runMigrations runs all pending goose migrations against the write DSN.
func runMigrations(dsn string) error {
	db, err := goose.OpenDBWithDriver("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open db for migrations: %w", err)
	}
	defer db.Close()

	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
