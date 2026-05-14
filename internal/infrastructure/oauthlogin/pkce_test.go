package oauthlogin

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestNewPKCE_ValidS256(t *testing.T) {
	p, err := newPKCE()
	if err != nil {
		t.Fatalf("newPKCE() error = %v", err)
	}
	if p.Method != "S256" {
		t.Errorf("expected method S256, got %s", p.Method)
	}
	if len(p.Verifier) < 32 {
		t.Errorf("verifier too short: %d", len(p.Verifier))
	}

	h := sha256.Sum256([]byte(p.Verifier))
	expected := base64.RawURLEncoding.EncodeToString(h[:])
	if p.Challenge != expected {
		t.Errorf("challenge mismatch: got %s, want %s", p.Challenge, expected)
	}
}

func TestNewPKCE_Unique(t *testing.T) {
	p1, _ := newPKCE()
	p2, _ := newPKCE()
	if p1.Verifier == p2.Verifier {
		t.Error("two PKCE pairs should not have the same verifier")
	}
}
