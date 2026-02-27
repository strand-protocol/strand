// Package apikey provides API key generation, hashing, and validation.
// Key format: pk_live_<32 alphanumeric characters>
// Only the SHA-256 hash is stored; the plaintext is returned once at creation.
package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

const (
	// Prefix for all API keys.
	Prefix = "pk_live_"
	// Length of the random part of the key.
	RandomLength = 32
	// PrefixDisplayLen is how many characters of the full key are stored for display.
	PrefixDisplayLen = 16
)

// Key represents a stored API key record.
type Key struct {
	ID         string
	TenantID   string
	CreatedBy  string
	Name       string
	KeyPrefix  string // First PrefixDisplayLen chars for display
	KeyHash    []byte // SHA-256 hash
	Role       string
	Scopes     []string
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
}

// IsRevoked returns true if the key has been revoked.
func (k *Key) IsRevoked() bool {
	return k.RevokedAt != nil
}

// IsExpired returns true if the key has passed its expiration date.
func (k *Key) IsExpired() bool {
	return k.ExpiresAt != nil && time.Now().After(*k.ExpiresAt)
}

// IsValid returns true if the key is neither revoked nor expired.
func (k *Key) IsValid() bool {
	return !k.IsRevoked() && !k.IsExpired()
}

// Generate creates a new API key, returning the plaintext (shown once) and the key record.
func Generate(tenantID, createdBy, name, role string) (plaintext string, key *Key, err error) {
	random := make([]byte, RandomLength)
	if _, err := rand.Read(random); err != nil {
		return "", nil, fmt.Errorf("apikey: generate random: %w", err)
	}

	randomStr := hex.EncodeToString(random)[:RandomLength]
	plaintext = Prefix + randomStr

	hash := sha256.Sum256([]byte(plaintext))

	key = &Key{
		TenantID:  tenantID,
		CreatedBy: createdBy,
		Name:      name,
		KeyPrefix: plaintext[:PrefixDisplayLen],
		KeyHash:   hash[:],
		Role:      role,
		CreatedAt: time.Now(),
	}

	return plaintext, key, nil
}

// Hash returns the SHA-256 hash of a plaintext API key.
func Hash(plaintext string) []byte {
	h := sha256.Sum256([]byte(plaintext))
	return h[:]
}
