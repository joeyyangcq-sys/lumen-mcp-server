package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/joey/lumen-mcp-server/internal/application/authorize"
	"github.com/joey/lumen-mcp-server/internal/application/ports"
	"github.com/joey/lumen-mcp-server/internal/domain/audit"
	"github.com/joey/lumen-mcp-server/internal/domain/tool"
)

func TestListToolsRequiresBearer(t *testing.T) {
	h := Handler{
		ResourceMetadataURL:   "https://mcp.example.com/.well-known/oauth-protected-resource",
		DefaultChallengeScope: "mcp:tools",
		Catalog:               staticCatalog{{Name: "list_routes"}},
		Verifier:              stubVerifier{claims: ports.Claims{Scopes: []string{"mcp:tools"}}},
		Authorize:             authorize.Service{RequiredScopes: []string{"mcp:tools"}},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/tools", nil)
	h.ListTools(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rec.Code)
	}
}

func TestListAuditRequiresValidBearer(t *testing.T) {
	h := Handler{
		Audit:                 staticAuditStore{},
		ResourceMetadataURL:   "https://mcp.example.com/.well-known/oauth-protected-resource",
		DefaultChallengeScope: "mcp:tools",
		Verifier:              stubVerifier{err: errors.New("invalid token")},
		Authorize:             authorize.Service{RequiredScopes: []string{"mcp:tools"}},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/audit", nil)
	req.Header.Set("Authorization", "Bearer bad")
	h.ListAudit(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rec.Code)
	}
}

func TestListToolsForbiddenWhenRequiredScopeMissing(t *testing.T) {
	h := Handler{
		Catalog:   staticCatalog{{Name: "list_routes"}},
		Verifier:  stubVerifier{claims: ports.Claims{Scopes: []string{"read"}}},
		Authorize: authorize.Service{RequiredScopes: []string{"mcp:tools"}},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/tools", nil)
	req.Header.Set("Authorization", "Bearer ok")
	h.ListTools(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", rec.Code)
	}
}

type stubVerifier struct {
	claims ports.Claims
	err    error
}

func (s stubVerifier) VerifyBearer(context.Context, string) (ports.Claims, error) {
	if s.err != nil {
		return ports.Claims{}, s.err
	}
	return s.claims, nil
}

type staticCatalog []tool.Definition

func (s staticCatalog) List(context.Context) ([]tool.Definition, error) {
	return s, nil
}

type staticAuditStore struct{}

func (staticAuditStore) Append(context.Context, audit.Event) error { return nil }

func (staticAuditStore) List(context.Context, int) ([]audit.Event, error) {
	return []audit.Event{{At: time.Now().UTC()}}, nil
}
