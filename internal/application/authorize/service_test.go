package authorize

import "testing"

func TestCheckMatrix(t *testing.T) {
	svc := Service{ToolScopeMap: map[string]string{
		"list_routes":      "read",
		"history_rollback": "gateway:write",
	}, RequiredScopes: []string{"mcp:tools"}}

	t.Run("allow exact scope", func(t *testing.T) {
		d := svc.Check([]string{"mcp:tools", "read"}, "list_routes")
		if !d.Allowed {
			t.Fatalf("expected allow, got deny: %s", d.Reason)
		}
	})

	t.Run("allow legacy read scope for new read requirement", func(t *testing.T) {
		d := svc.Check([]string{"mcp:tools", "routes:read"}, "list_routes")
		if !d.Allowed {
			t.Fatalf("expected allow, got deny: %s", d.Reason)
		}
	})

	t.Run("allow legacy write scope for new gateway write requirement", func(t *testing.T) {
		d := svc.Check([]string{"mcp:tools", "routes:write"}, "history_rollback")
		if !d.Allowed {
			t.Fatalf("expected allow, got deny: %s", d.Reason)
		}
	})

	t.Run("allow admin wildcard when global scope is present", func(t *testing.T) {
		d := svc.Check([]string{"mcp:tools", "admin:*"}, "history_rollback")
		if !d.Allowed {
			t.Fatalf("expected allow, got deny: %s", d.Reason)
		}
	})

	t.Run("deny missing scope", func(t *testing.T) {
		d := svc.Check([]string{"mcp:tools", "read"}, "history_rollback")
		if d.Allowed {
			t.Fatalf("expected deny")
		}
	})

	t.Run("deny missing required global scope", func(t *testing.T) {
		d := svc.Check([]string{"read"}, "list_routes")
		if d.Allowed {
			t.Fatalf("expected deny")
		}
	})

	t.Run("deny admin wildcard without required global scope", func(t *testing.T) {
		d := svc.Check([]string{"admin:*"}, "history_rollback")
		if d.Allowed {
			t.Fatalf("expected deny")
		}
	})

	t.Run("deny unregistered tool", func(t *testing.T) {
		d := svc.Check([]string{"admin:*"}, "unknown_tool")
		if d.Allowed {
			t.Fatalf("expected deny")
		}
	})
}
