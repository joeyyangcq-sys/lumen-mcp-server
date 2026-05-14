package auditstore

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/joey/lumen-mcp-server/internal/domain/audit"
)

func TestPostgresStore_AppendAndList(t *testing.T) {
	connStr := os.Getenv("LUMEN_MCP_POSTGRES_URL")
	if connStr == "" {
		t.Skip("LUMEN_MCP_POSTGRES_URL not set, skipping Postgres integration test")
	}

	store, err := OpenPostgres(connStr)
	if err != nil {
		t.Fatalf("OpenPostgres() error = %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	_, _ = store.db.ExecContext(ctx, "DELETE FROM mcp_audit_log")

	e := audit.Event{
		At:       time.Unix(1710000000, 0).UTC(),
		Actor:    "pg-user-1",
		ClientID: "pg-client-1",
		Tool:     "list_routes",
		Result:   "allow",
		TraceID:  "pg-trace-1",
		Message:  "ok",
	}
	if err := store.Append(ctx, e); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	items, err := store.List(ctx, 10)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items=%d want=1", len(items))
	}
	got := items[0]
	if got.Tool != "list_routes" {
		t.Errorf("Tool=%q want=list_routes", got.Tool)
	}
	if got.Result != "allow" {
		t.Errorf("Result=%q want=allow", got.Result)
	}
	if got.Actor != "pg-user-1" {
		t.Errorf("Actor=%q want=pg-user-1", got.Actor)
	}
	if !got.At.Equal(e.At) {
		t.Errorf("At=%v want=%v", got.At, e.At)
	}
}
