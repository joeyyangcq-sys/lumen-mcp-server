package authorize

import "testing"

func TestCheckMatrix(t *testing.T) {
	svc := Service{ToolScopeMap: map[string]string{
		"list_routes":      "routes:read",
		"history_rollback": "admin:dangerous",
	}}

	t.Run("allow exact scope", func(t *testing.T) {
		d := svc.Check([]string{"routes:read"}, "list_routes")
		if !d.Allowed {
			t.Fatalf("expected allow, got deny: %s", d.Reason)
		}
	})

	t.Run("allow admin wildcard", func(t *testing.T) {
		d := svc.Check([]string{"admin:*"}, "history_rollback")
		if !d.Allowed {
			t.Fatalf("expected allow, got deny: %s", d.Reason)
		}
	})

	t.Run("deny missing scope", func(t *testing.T) {
		d := svc.Check([]string{"routes:read"}, "history_rollback")
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
