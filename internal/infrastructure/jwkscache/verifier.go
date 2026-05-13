package jwkscache

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/joey/lumen-mcp-server/internal/application/ports"
)

var (
	ErrMissingBearer   = errors.New("missing bearer token")
	ErrMalformedJWT    = errors.New("malformed jwt")
	ErrInvalidAlg      = errors.New("invalid jwt alg")
	ErrInvalidSig      = errors.New("invalid jwt signature")
	ErrIssuerMismatch  = errors.New("issuer mismatch")
	ErrAudienceMissing = errors.New("audience mismatch")
	ErrTokenExpired    = errors.New("token expired")
	ErrJWKSKeyNotFound = errors.New("jwks key not found")
)

type Verifier struct {
	Issuer          string
	Audience        string
	JWKSURL         string
	HTTPClient      *http.Client
	RefreshInterval time.Duration
	Now             func() time.Time

	mu          sync.RWMutex
	cachedAt    time.Time
	cachedOctHS map[string][]byte
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Typ string `json:"typ"`
}

type jwtClaims struct {
	Iss      string      `json:"iss"`
	Sub      string      `json:"sub"`
	Aud      interface{} `json:"aud"`
	ClientID string      `json:"client_id"`
	Scope    string      `json:"scope"`
	Exp      int64       `json:"exp"`
}

type jwksResponse struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	Kty string `json:"kty"`
	K   string `json:"k"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
}

func (v *Verifier) VerifyBearer(ctx context.Context, bearer string) (ports.Claims, error) {
	token := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(bearer), "Bearer "))
	if token == "" {
		return ports.Claims{}, ErrMissingBearer
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ports.Claims{}, ErrMalformedJWT
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ports.Claims{}, ErrMalformedJWT
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ports.Claims{}, ErrMalformedJWT
	}
	signatureBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return ports.Claims{}, ErrMalformedJWT
	}

	var hdr jwtHeader
	if err := json.Unmarshal(headerBytes, &hdr); err != nil {
		return ports.Claims{}, ErrMalformedJWT
	}
	if !strings.EqualFold(hdr.Alg, "HS256") {
		return ports.Claims{}, ErrInvalidAlg
	}

	var claims jwtClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return ports.Claims{}, ErrMalformedJWT
	}

	secret, err := v.selectHMACSecret(ctx, hdr.Kid)
	if err != nil {
		return ports.Claims{}, err
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	expected := mac.Sum(nil)
	if subtle.ConstantTimeCompare(expected, signatureBytes) != 1 {
		return ports.Claims{}, ErrInvalidSig
	}

	if strings.TrimSpace(v.Issuer) != "" && claims.Iss != v.Issuer {
		return ports.Claims{}, ErrIssuerMismatch
	}
	if strings.TrimSpace(v.Audience) != "" && !audContains(claims.Aud, v.Audience) {
		return ports.Claims{}, ErrAudienceMissing
	}
	now := time.Now().UTC()
	if v.Now != nil {
		now = v.Now().UTC()
	}
	if claims.Exp > 0 && now.Unix() >= claims.Exp {
		return ports.Claims{}, ErrTokenExpired
	}

	clientID := claims.ClientID
	if clientID == "" {
		clientID = claims.Sub
	}
	return ports.Claims{
		Subject: claims.Sub,
		Client:  clientID,
		Scopes:  strings.Fields(strings.TrimSpace(claims.Scope)),
	}, nil
}

func (v *Verifier) selectHMACSecret(ctx context.Context, kid string) ([]byte, error) {
	keys, err := v.loadJWKSKeys(ctx)
	if err != nil {
		return nil, err
	}
	if kid != "" {
		if secret, ok := keys[kid]; ok {
			return secret, nil
		}
		return nil, ErrJWKSKeyNotFound
	}
	for _, secret := range keys {
		return secret, nil
	}
	return nil, ErrJWKSKeyNotFound
}

func (v *Verifier) loadJWKSKeys(ctx context.Context) (map[string][]byte, error) {
	v.mu.RLock()
	refresh := v.refreshInterval()
	if len(v.cachedOctHS) > 0 && time.Since(v.cachedAt) < refresh {
		out := cloneSecretMap(v.cachedOctHS)
		v.mu.RUnlock()
		return out, nil
	}
	v.mu.RUnlock()

	client := v.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.JWKSURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("jwks fetch failed: status=%d", resp.StatusCode)
	}
	var payload jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	keys := make(map[string][]byte)
	for i, k := range payload.Keys {
		if !strings.EqualFold(k.Kty, "oct") {
			continue
		}
		raw, err := base64.RawURLEncoding.DecodeString(k.K)
		if err != nil || len(raw) == 0 {
			continue
		}
		id := strings.TrimSpace(k.Kid)
		if id == "" {
			id = fmt.Sprintf("idx-%d", i)
		}
		keys[id] = raw
	}
	if len(keys) == 0 {
		return nil, ErrJWKSKeyNotFound
	}

	v.mu.Lock()
	v.cachedOctHS = cloneSecretMap(keys)
	v.cachedAt = time.Now()
	v.mu.Unlock()

	return keys, nil
}

func (v *Verifier) refreshInterval() time.Duration {
	if v.RefreshInterval <= 0 {
		return 1 * time.Minute
	}
	return v.RefreshInterval
}

func cloneSecretMap(in map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(in))
	for k, v := range in {
		dup := make([]byte, len(v))
		copy(dup, v)
		out[k] = dup
	}
	return out
}

func audContains(aud interface{}, want string) bool {
	want = strings.TrimSpace(want)
	if want == "" {
		return true
	}
	switch v := aud.(type) {
	case string:
		return v == want
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok && s == want {
				return true
			}
		}
	}
	return false
}
