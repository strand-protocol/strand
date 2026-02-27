// Package postgres provides a PostgreSQL connection pool with Row Level Security helpers.
package postgres

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// DB wraps a sql.DB connection pool.
type DB struct {
	pool *sql.DB
}

// New creates a new PostgreSQL connection pool.
func New(dsn string) (*DB, error) {
	pool, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: open: %w", err)
	}
	pool.SetMaxOpenConns(25)
	pool.SetMaxIdleConns(5)
	if err := pool.Ping(); err != nil {
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	return &DB{pool: pool}, nil
}

// Pool returns the underlying sql.DB for direct queries.
func (db *DB) Pool() *sql.DB {
	return db.pool
}

// Close closes the connection pool.
func (db *DB) Close() error {
	return db.pool.Close()
}

// TenantConn returns a sql.Conn with the RLS tenant context set.
// The caller MUST close the returned conn when done.
func (db *DB) TenantConn(ctx context.Context, tenantID string) (*sql.Conn, error) {
	conn, err := db.pool.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: conn: %w", err)
	}
	// SET LOCAL scopes the setting to the current transaction.
	// For non-transactional queries, SET (without LOCAL) scopes to the connection.
	_, err = conn.ExecContext(ctx, "SET app.current_tenant_id = $1", tenantID)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("postgres: set tenant: %w", err)
	}
	return conn, nil
}

// WithTenant executes fn within a tenant-scoped connection.
func (db *DB) WithTenant(ctx context.Context, tenantID string, fn func(conn *sql.Conn) error) error {
	conn, err := db.TenantConn(ctx, tenantID)
	if err != nil {
		return err
	}
	defer conn.Close()
	return fn(conn)
}
