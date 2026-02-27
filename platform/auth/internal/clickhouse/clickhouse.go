// Package clickhouse provides a ClickHouse connection for telemetry and traffic metering.
package clickhouse

import (
	"database/sql"
	"fmt"
)

// DB wraps a ClickHouse connection.
type DB struct {
	pool *sql.DB
}

// New creates a new ClickHouse connection.
// DSN format: "clickhouse://host:9000/strand"
func New(dsn string) (*DB, error) {
	pool, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: open: %w", err)
	}
	if err := pool.Ping(); err != nil {
		return nil, fmt.Errorf("clickhouse: ping: %w", err)
	}
	return &DB{pool: pool}, nil
}

// Pool returns the underlying sql.DB.
func (db *DB) Pool() *sql.DB {
	return db.pool
}

// Close closes the connection.
func (db *DB) Close() error {
	return db.pool.Close()
}
