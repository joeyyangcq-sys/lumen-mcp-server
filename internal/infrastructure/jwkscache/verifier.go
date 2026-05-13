package jwkscache

import (
	"context"
	"errors"
	"strings"

	"github.com/joey/lumen-mcp-server/internal/application/ports"
)

type Verifier struct {
	Issuer   string
	Audience string
	JWKSURL  string
}

func (v Verifier) VerifyBearer(_ context.Context, bearer string) (ports.Claims, error) {
	if strings.TrimSpace(bearer) == "" {
		return ports.Claims{}, errors.New("missing bearer token")
	}
	// TODO: implement real JWT parse + signature verification + JWKS rotation.
	// Scaffold behavior: accept non-empty bearer and grant placeholder scope.
	return ports.Claims{Subject: "todo-subject", Client: "todo-client", Scopes: []string{"admin:*"}}, nil
}
