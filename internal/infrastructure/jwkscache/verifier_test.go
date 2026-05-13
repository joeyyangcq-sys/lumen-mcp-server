package jwkscache

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestVerifyBearer_ValidToken(t *testing.T) {
	secret := []byte("test-secret")
	srv := newJWKSServer(secret)
	defer srv.Close()

	now := time.Unix(1710000000, 0).UTC()
	tok := makeHS256Token(t, secret, map[string]any{
		"iss":       "http://issuer.local",
		"aud":       []string{"lumen-mcp"},
		"sub":       "svc-a",
		"client_id": "svc-a",
		"scope":     "routes:read routes:write",
		"exp":       now.Add(5 * time.Minute).Unix(),
	})

	v := &Verifier{
		Issuer:   "http://issuer.local",
		Audience: "lumen-mcp",
		JWKSURL:  srv.URL,
		Now:      func() time.Time { return now },
	}
	claims, err := v.VerifyBearer(context.Background(), "Bearer "+tok)
	if err != nil {
		t.Fatalf("VerifyBearer() error = %v", err)
	}
	if claims.Subject != "svc-a" || claims.Client != "svc-a" {
		t.Fatalf("claims mismatch: %+v", claims)
	}
	if len(claims.Scopes) != 2 || claims.Scopes[0] != "routes:read" {
		t.Fatalf("scopes mismatch: %#v", claims.Scopes)
	}
}

func TestVerifyBearer_InvalidIssuerAudienceSignature(t *testing.T) {
	secret := []byte("test-secret")
	srv := newJWKSServer(secret)
	defer srv.Close()

	now := time.Unix(1710000000, 0).UTC()
	base := map[string]any{
		"iss":       "http://issuer.local",
		"aud":       []string{"lumen-mcp"},
		"sub":       "svc-a",
		"client_id": "svc-a",
		"scope":     "routes:read",
		"exp":       now.Add(5 * time.Minute).Unix(),
	}

	t.Run("issuer mismatch", func(t *testing.T) {
		tok := makeHS256Token(t, secret, base)
		v := &Verifier{Issuer: "http://other", Audience: "lumen-mcp", JWKSURL: srv.URL, Now: func() time.Time { return now }}
		_, err := v.VerifyBearer(context.Background(), "Bearer "+tok)
		if err != ErrIssuerMismatch {
			t.Fatalf("err=%v want ErrIssuerMismatch", err)
		}
	})

	t.Run("audience mismatch", func(t *testing.T) {
		tok := makeHS256Token(t, secret, base)
		v := &Verifier{Issuer: "http://issuer.local", Audience: "other-aud", JWKSURL: srv.URL, Now: func() time.Time { return now }}
		_, err := v.VerifyBearer(context.Background(), "Bearer "+tok)
		if err != ErrAudienceMissing {
			t.Fatalf("err=%v want ErrAudienceMissing", err)
		}
	})

	t.Run("signature invalid", func(t *testing.T) {
		tok := makeHS256Token(t, []byte("wrong-secret"), base)
		v := &Verifier{Issuer: "http://issuer.local", Audience: "lumen-mcp", JWKSURL: srv.URL, Now: func() time.Time { return now }}
		_, err := v.VerifyBearer(context.Background(), "Bearer "+tok)
		if err != ErrInvalidSig {
			t.Fatalf("err=%v want ErrInvalidSig", err)
		}
	})
}

func TestVerifyBearer_Expired(t *testing.T) {
	secret := []byte("test-secret")
	srv := newJWKSServer(secret)
	defer srv.Close()
	now := time.Unix(1710000000, 0).UTC()
	tok := makeHS256Token(t, secret, map[string]any{
		"iss":       "http://issuer.local",
		"aud":       []string{"lumen-mcp"},
		"sub":       "svc-a",
		"client_id": "svc-a",
		"scope":     "routes:read",
		"exp":       now.Add(-1 * time.Minute).Unix(),
	})
	v := &Verifier{Issuer: "http://issuer.local", Audience: "lumen-mcp", JWKSURL: srv.URL, Now: func() time.Time { return now }}
	_, err := v.VerifyBearer(context.Background(), "Bearer "+tok)
	if err != ErrTokenExpired {
		t.Fatalf("err=%v want ErrTokenExpired", err)
	}
}

func newJWKSServer(secret []byte) *httptest.Server {
	k := base64.RawURLEncoding.EncodeToString(secret)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{{
				"kty": "oct",
				"kid": "test-key",
				"alg": "HS256",
				"k":   k,
			}},
		})
	})
	return httptest.NewServer(h)
}

func makeHS256Token(t *testing.T, secret []byte, payload map[string]any) string {
	t.Helper()
	header := map[string]any{"alg": "HS256", "typ": "JWT", "kid": "test-key"}
	hRaw, _ := json.Marshal(header)
	pRaw, _ := json.Marshal(payload)
	h := base64.RawURLEncoding.EncodeToString(hRaw)
	p := base64.RawURLEncoding.EncodeToString(pRaw)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(h + "." + p))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return strings.Join([]string{h, p, sig}, ".")
}
