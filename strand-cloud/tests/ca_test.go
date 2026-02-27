package tests

import (
	"testing"
	"time"

	"github.com/strand-protocol/strand/strand-cloud/pkg/ca"
	"github.com/strand-protocol/strand/strand-cloud/pkg/model"
)

func newTestCA(t *testing.T) *ca.CA {
	t.Helper()
	ks := ca.NewMemoryKeyStore()
	authority := ca.NewCA(ks)
	if err := authority.GenerateCA(); err != nil {
		t.Fatalf("generate CA: %v", err)
	}
	return authority
}

func TestCA_IssueAndVerify(t *testing.T) {
	authority := newTestCA(t)

	mic := &model.MIC{
		ID:           "mic-test-1",
		NodeID:       "node-test-1",
		ModelHash:    [32]byte{0x01, 0x02, 0x03},
		Capabilities: []string{"route", "stream"},
		ValidFrom:    time.Now(),
		ValidUntil:   time.Now().Add(24 * time.Hour),
	}

	if err := authority.IssueMIC(mic); err != nil {
		t.Fatalf("issue mic: %v", err)
	}
	if len(mic.Signature) == 0 {
		t.Fatal("expected non-empty signature")
	}

	valid, err := authority.VerifyMIC(mic)
	if err != nil {
		t.Fatalf("verify mic: %v", err)
	}
	if !valid {
		t.Fatal("expected mic to be valid")
	}
}

func TestCA_TamperedMIC(t *testing.T) {
	authority := newTestCA(t)

	mic := &model.MIC{
		ID:           "mic-tamper-1",
		NodeID:       "node-tamper-1",
		Capabilities: []string{"route"},
		ValidFrom:    time.Now(),
		ValidUntil:   time.Now().Add(24 * time.Hour),
	}

	if err := authority.IssueMIC(mic); err != nil {
		t.Fatalf("issue mic: %v", err)
	}

	// Tamper with the MIC
	mic.NodeID = "evil-node"

	valid, err := authority.VerifyMIC(mic)
	if err != nil {
		t.Fatalf("verify mic: %v", err)
	}
	if valid {
		t.Fatal("expected tampered mic to be invalid")
	}
}

func TestCA_Revoke(t *testing.T) {
	authority := newTestCA(t)

	mic := &model.MIC{
		ID:           "mic-revoke-1",
		NodeID:       "node-revoke-1",
		Capabilities: []string{"stream"},
		ValidFrom:    time.Now(),
		ValidUntil:   time.Now().Add(24 * time.Hour),
	}

	if err := authority.IssueMIC(mic); err != nil {
		t.Fatalf("issue mic: %v", err)
	}

	// Before revocation the MIC should be valid.
	valid, _ := authority.VerifyMIC(mic)
	if !valid {
		t.Fatal("expected valid mic before revoke")
	}

	// Revoke
	authority.RevokeMIC(mic.ID)

	if !authority.IsRevoked(mic.ID) {
		t.Fatal("expected mic to be marked as revoked")
	}

	valid, _ = authority.VerifyMIC(mic)
	if valid {
		t.Fatal("expected revoked mic to be invalid")
	}
}

func TestCA_RevokedFieldOnMIC(t *testing.T) {
	authority := newTestCA(t)

	mic := &model.MIC{
		ID:           "mic-revfield-1",
		NodeID:       "node-revfield-1",
		Capabilities: []string{"route"},
		ValidFrom:    time.Now(),
		ValidUntil:   time.Now().Add(24 * time.Hour),
		Revoked:      true, // Already marked as revoked
	}

	if err := authority.IssueMIC(mic); err != nil {
		t.Fatalf("issue mic: %v", err)
	}

	valid, _ := authority.VerifyMIC(mic)
	if valid {
		t.Fatal("expected mic with Revoked=true to be invalid")
	}
}

func TestCA_PublicKey(t *testing.T) {
	authority := newTestCA(t)

	pub, err := authority.PublicKey()
	if err != nil {
		t.Fatalf("public key: %v", err)
	}
	if len(pub) == 0 {
		t.Fatal("expected non-empty public key")
	}
}

func TestCA_MultipleIssuances(t *testing.T) {
	authority := newTestCA(t)

	for i := 0; i < 10; i++ {
		mic := &model.MIC{
			ID:           "mic-multi-" + string(rune('0'+i)),
			NodeID:       "node-multi",
			Capabilities: []string{"route"},
			ValidFrom:    time.Now(),
			ValidUntil:   time.Now().Add(24 * time.Hour),
		}
		if err := authority.IssueMIC(mic); err != nil {
			t.Fatalf("issue mic %d: %v", i, err)
		}
		valid, err := authority.VerifyMIC(mic)
		if err != nil {
			t.Fatalf("verify mic %d: %v", i, err)
		}
		if !valid {
			t.Fatalf("expected mic %d to be valid", i)
		}
	}
}
