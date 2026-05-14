package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProtectedResourceMetadata(t *testing.T) {
	h := Handler{
		Resource:            "https://mcp.example.com/mcp",
		AuthorizationServer: "https://auth.example.com",
		ScopesSupported:     []string{"mcp:tools", "mcp:read"},
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	h.ProtectedResourceMetadata(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["resource"] != "https://mcp.example.com/mcp" {
		t.Fatalf("resource=%v", body["resource"])
	}
	authServers, ok := body["authorization_servers"].([]any)
	if !ok || len(authServers) != 1 || authServers[0] != "https://auth.example.com" {
		t.Fatalf("authorization_servers=%#v", body["authorization_servers"])
	}
}

func TestMCPUnauthorizedChallenge(t *testing.T) {
	h := Handler{
		ResourceMetadataURL:   "https://mcp.example.com/.well-known/oauth-protected-resource",
		DefaultChallengeScope: "mcp:tools",
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	h.MCP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rec.Code)
	}
	header := rec.Header().Get("WWW-Authenticate")
	if !strings.Contains(header, `resource_metadata="https://mcp.example.com/.well-known/oauth-protected-resource"`) {
		t.Fatalf("WWW-Authenticate missing resource metadata: %q", header)
	}
	if !strings.Contains(header, `scope="mcp:tools"`) {
		t.Fatalf("WWW-Authenticate missing scope: %q", header)
	}
}
