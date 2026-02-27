// Package ca implements the Nexus Trust CA service. It uses Ed25519 keys to
// issue and verify Machine Identity Certificates (MICs).
package ca

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/nexus-protocol/nexus/nexus-cloud/pkg/model"
)

const rootKeyID = "nexus-root-ca"

// CA is the central certificate authority for the Nexus control plane.
type CA struct {
	mu       sync.RWMutex
	keyStore KeyStore
	revoked  map[string]bool
}

// NewCA creates a CA backed by the given KeyStore. Call GenerateCA() to create
// the root key pair before issuing MICs.
func NewCA(ks KeyStore) *CA {
	return &CA{
		keyStore: ks,
		revoked:  make(map[string]bool),
	}
}

// GenerateCA creates a new Ed25519 root key pair and persists it in the KeyStore.
func (c *CA) GenerateCA() error {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate ed25519 key: %w", err)
	}
	if err := c.keyStore.StorePrivateKey(rootKeyID, priv); err != nil {
		return fmt.Errorf("store private key: %w", err)
	}
	if err := c.keyStore.StorePublicKey(rootKeyID, pub); err != nil {
		return fmt.Errorf("store public key: %w", err)
	}
	return nil
}

// IssueMIC signs a new MIC for the given node. The caller supplies an ID,
// nodeID, capabilities, model hash, and validity window. The CA populates the
// Signature field and returns the complete MIC.
func (c *CA) IssueMIC(mic *model.MIC) error {
	priv, err := c.keyStore.LoadPrivateKey(rootKeyID)
	if err != nil {
		return fmt.Errorf("load signing key: %w", err)
	}

	payload := c.micPayload(mic)
	mic.Signature = ed25519.Sign(priv, payload)
	return nil
}

// VerifyMIC checks the signature of a MIC and whether it has been revoked.
func (c *CA) VerifyMIC(mic *model.MIC) (bool, error) {
	c.mu.RLock()
	if c.revoked[mic.ID] {
		c.mu.RUnlock()
		return false, nil
	}
	c.mu.RUnlock()

	if mic.Revoked {
		return false, nil
	}

	pub, err := c.keyStore.LoadPublicKey(rootKeyID)
	if err != nil {
		return false, fmt.Errorf("load public key: %w", err)
	}

	payload := c.micPayload(mic)
	return ed25519.Verify(pub, payload, mic.Signature), nil
}

// RevokeMIC marks the given MIC ID as revoked. Subsequent calls to VerifyMIC
// for this ID will return false.
func (c *CA) RevokeMIC(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.revoked[id] = true
}

// IsRevoked returns true if the MIC with the given ID has been revoked.
func (c *CA) IsRevoked(id string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.revoked[id]
}

// micPayload builds a deterministic byte representation of a MIC that is signed
// or verified.
func (c *CA) micPayload(mic *model.MIC) []byte {
	h := sha256.New()
	h.Write([]byte(mic.ID))
	h.Write([]byte(mic.NodeID))
	h.Write(mic.ModelHash[:])
	for _, cap := range mic.Capabilities {
		h.Write([]byte(cap))
	}
	vf := make([]byte, 8)
	binary.BigEndian.PutUint64(vf, uint64(mic.ValidFrom.Unix()))
	h.Write(vf)
	vu := make([]byte, 8)
	binary.BigEndian.PutUint64(vu, uint64(mic.ValidUntil.Unix()))
	h.Write(vu)
	return h.Sum(nil)
}

// PublicKey returns the root CA public key.
func (c *CA) PublicKey() (ed25519.PublicKey, error) {
	return c.keyStore.LoadPublicKey(rootKeyID)
}

// DefaultValidity is the default MIC validity window.
const DefaultValidity = 24 * time.Hour * 365
