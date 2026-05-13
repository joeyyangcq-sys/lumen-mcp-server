package auditstore

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/joey/lumen-mcp-server/internal/domain/audit"
)

func TestSQLiteStore_AppendAndList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.db")
	store, err := OpenSQLite(path)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer func() { _ = store.Close() }()

	e := audit.Event{
		At:       time.Unix(1710000000, 0).UTC(),
		Actor:    "user-1",
		ClientID: "client-1",
		Tool:     "list_routes",
		Result:   "allow",
		TraceID:  "trace-1",
		Message:  "ok",
	}
	if err := store.Append(context.Background(), e); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	items, err := store.List(context.Background(), 10)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items=%d want=1", len(items))
	}
	if items[0].Tool != "list_routes" || items[0].Result != "allow" {
		t.Fatalf("unexpected item: %+v", items[0])
	}
}
