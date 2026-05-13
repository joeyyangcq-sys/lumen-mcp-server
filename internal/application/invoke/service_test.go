package invoke

import (
	"context"
	"errors"
	"testing"

	"github.com/joey/lumen-mcp-server/internal/application/authorize"
	"github.com/joey/lumen-mcp-server/internal/application/ports"
	"github.com/joey/lumen-mcp-server/internal/domain/audit"
)

type fakeVerifier struct {
	claims ports.Claims
	err    error
}

func (f fakeVerifier) VerifyBearer(context.Context, string) (ports.Claims, error) {
	return f.claims, f.err
}

type fakeGateway struct {
	resp map[string]any
	err  error
}

func (f fakeGateway) InvokeTool(context.Context, string, map[string]any) (map[string]any, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

type memAudit struct {
	events []audit.Event
}

func (m *memAudit) Append(_ context.Context, e audit.Event) error {
	m.events = append(m.events, e)
	return nil
}

func (m *memAudit) List(_ context.Context, _ int) ([]audit.Event, error) {
	return m.events, nil
}

func TestInvokeTool_AuditAllowDenyAndError(t *testing.T) {
	t.Run("deny on verifier error", func(t *testing.T) {
		a := &memAudit{}
		svc := Service{
			Verifier:  fakeVerifier{err: errors.New("bad token")},
			Authorize: authorize.Service{ToolScopeMap: map[string]string{"list_routes": "routes:read"}},
			Gateway:   fakeGateway{resp: map[string]any{"ok": true}},
			Audit:     a,
		}
		_, err := svc.InvokeTool(context.Background(), "Bearer x", "list_routes", nil, "trace-1")
		if err == nil {
			t.Fatal("expected error")
		}
		if len(a.events) != 1 || a.events[0].Result != "deny" {
			t.Fatalf("unexpected audit events: %#v", a.events)
		}
	})

	t.Run("deny on scope mismatch", func(t *testing.T) {
		a := &memAudit{}
		svc := Service{
			Verifier:  fakeVerifier{claims: ports.Claims{Subject: "s", Client: "c", Scopes: []string{"routes:read"}}},
			Authorize: authorize.Service{ToolScopeMap: map[string]string{"history_rollback": "admin:dangerous"}},
			Gateway:   fakeGateway{resp: map[string]any{"ok": true}},
			Audit:     a,
		}
		_, err := svc.InvokeTool(context.Background(), "Bearer x", "history_rollback", nil, "trace-2")
		if err == nil {
			t.Fatal("expected error")
		}
		if len(a.events) != 1 || a.events[0].Result != "deny" {
			t.Fatalf("unexpected audit events: %#v", a.events)
		}
	})

	t.Run("error on gateway failure", func(t *testing.T) {
		a := &memAudit{}
		svc := Service{
			Verifier:  fakeVerifier{claims: ports.Claims{Subject: "s", Client: "c", Scopes: []string{"routes:read"}}},
			Authorize: authorize.Service{ToolScopeMap: map[string]string{"list_routes": "routes:read"}},
			Gateway:   fakeGateway{err: errors.New("gateway down")},
			Audit:     a,
		}
		_, err := svc.InvokeTool(context.Background(), "Bearer x", "list_routes", nil, "trace-3")
		if err == nil {
			t.Fatal("expected error")
		}
		if len(a.events) != 1 || a.events[0].Result != "error" {
			t.Fatalf("unexpected audit events: %#v", a.events)
		}
	})

	t.Run("allow and audit success", func(t *testing.T) {
		a := &memAudit{}
		svc := Service{
			Verifier:  fakeVerifier{claims: ports.Claims{Subject: "s", Client: "c", Scopes: []string{"routes:read"}}},
			Authorize: authorize.Service{ToolScopeMap: map[string]string{"list_routes": "routes:read"}},
			Gateway:   fakeGateway{resp: map[string]any{"ok": true}},
			Audit:     a,
		}
		out, err := svc.InvokeTool(context.Background(), "Bearer x", "list_routes", nil, "trace-4")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out["ok"] != true {
			t.Fatalf("unexpected output: %#v", out)
		}
		if len(a.events) != 1 || a.events[0].Result != "allow" {
			t.Fatalf("unexpected audit events: %#v", a.events)
		}
	})
}
