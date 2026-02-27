package apikey

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// Store provides API key persistence backed by PostgreSQL.
type Store struct {
	db *sql.DB
}

// NewStore creates an API key store.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Create inserts a new API key record.
func (s *Store) Create(ctx context.Context, key *Key) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO api_keys (tenant_id, created_by, name, key_prefix, key_hash, role, scopes)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		key.TenantID, key.CreatedBy, key.Name, key.KeyPrefix, key.KeyHash,
		key.Role, pq.Array(key.Scopes),
	)
	if err != nil {
		return fmt.Errorf("apikey: create: %w", err)
	}
	return nil
}

// Validate looks up an API key by its plaintext, verifies it is active, and returns the key record.
func (s *Store) Validate(ctx context.Context, plaintext string) (*Key, error) {
	hash := Hash(plaintext)

	var key Key
	var scopes []string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, created_by, name, key_prefix, key_hash, role, scopes,
		        last_used_at, expires_at, revoked_at, created_at
		 FROM api_keys WHERE key_hash = $1`,
		hash,
	).Scan(
		&key.ID, &key.TenantID, &key.CreatedBy, &key.Name, &key.KeyPrefix,
		&key.KeyHash, &key.Role, pq.Array(&scopes),
		&key.LastUsedAt, &key.ExpiresAt, &key.RevokedAt, &key.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("apikey: not found")
	}
	if err != nil {
		return nil, fmt.Errorf("apikey: validate: %w", err)
	}
	key.Scopes = scopes

	// Constant-time comparison for the hash (defense in depth)
	if subtle.ConstantTimeCompare(key.KeyHash, hash) != 1 {
		return nil, fmt.Errorf("apikey: hash mismatch")
	}

	if !key.IsValid() {
		return nil, fmt.Errorf("apikey: key is revoked or expired")
	}

	// Update last_used_at asynchronously (fire-and-forget)
	go func() {
		_, _ = s.db.ExecContext(context.Background(),
			`UPDATE api_keys SET last_used_at = $1 WHERE id = $2`,
			time.Now(), key.ID,
		)
	}()

	return &key, nil
}

// List returns all API keys for a tenant (without hashes).
func (s *Store) List(ctx context.Context, tenantID string) ([]Key, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, created_by, name, key_prefix, role, scopes,
		        last_used_at, expires_at, revoked_at, created_at
		 FROM api_keys WHERE tenant_id = $1 ORDER BY created_at DESC`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("apikey: list: %w", err)
	}
	defer rows.Close()

	var keys []Key
	for rows.Next() {
		var k Key
		var scopes []string
		if err := rows.Scan(
			&k.ID, &k.TenantID, &k.CreatedBy, &k.Name, &k.KeyPrefix,
			&k.Role, pq.Array(&scopes),
			&k.LastUsedAt, &k.ExpiresAt, &k.RevokedAt, &k.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("apikey: scan: %w", err)
		}
		k.Scopes = scopes
		keys = append(keys, k)
	}
	return keys, nil
}

// Revoke marks an API key as revoked.
func (s *Store) Revoke(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE api_keys SET revoked_at = $1 WHERE id = $2 AND revoked_at IS NULL`,
		time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("apikey: revoke: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("apikey: not found or already revoked")
	}
	return nil
}
