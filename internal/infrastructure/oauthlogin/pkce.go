package oauthlogin

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

type pkceChallenge struct {
	Verifier  string
	Challenge string
	Method    string
}

func newPKCE() (pkceChallenge, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return pkceChallenge{}, err
	}
	verifier := base64.RawURLEncoding.EncodeToString(buf)
	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])
	return pkceChallenge{
		Verifier:  verifier,
		Challenge: challenge,
		Method:    "S256",
	}, nil
}
