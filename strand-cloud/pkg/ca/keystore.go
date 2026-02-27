package ca

import (
	"crypto/ed25519"
	"fmt"
	"sync"
)

// KeyStore is the interface for persisting CA key material.
type KeyStore interface {
	StorePrivateKey(id string, key ed25519.PrivateKey) error
	LoadPrivateKey(id string) (ed25519.PrivateKey, error)
	StorePublicKey(id string, key ed25519.PublicKey) error
	LoadPublicKey(id string) (ed25519.PublicKey, error)
}

// MemoryKeyStore is an in-memory implementation of KeyStore.
type MemoryKeyStore struct {
	mu       sync.RWMutex
	privKeys map[string]ed25519.PrivateKey
	pubKeys  map[string]ed25519.PublicKey
}

// NewMemoryKeyStore returns an initialised MemoryKeyStore.
func NewMemoryKeyStore() *MemoryKeyStore {
	return &MemoryKeyStore{
		privKeys: make(map[string]ed25519.PrivateKey),
		pubKeys:  make(map[string]ed25519.PublicKey),
	}
}

func (k *MemoryKeyStore) StorePrivateKey(id string, key ed25519.PrivateKey) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.privKeys[id] = key
	return nil
}

func (k *MemoryKeyStore) LoadPrivateKey(id string) (ed25519.PrivateKey, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	key, ok := k.privKeys[id]
	if !ok {
		return nil, fmt.Errorf("private key %q not found", id)
	}
	return key, nil
}

func (k *MemoryKeyStore) StorePublicKey(id string, key ed25519.PublicKey) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.pubKeys[id] = key
	return nil
}

func (k *MemoryKeyStore) LoadPublicKey(id string) (ed25519.PublicKey, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	key, ok := k.pubKeys[id]
	if !ok {
		return nil, fmt.Errorf("public key %q not found", id)
	}
	return key, nil
}
