package auditstore

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"

	"github.com/joey/lumen-mcp-server/internal/domain/audit"
)

type SQLiteStore struct {
	db *sql.DB
}

func OpenSQLite(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	store := &SQLiteStore{db: db}
	if err := store.initSchema(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteStore) initSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS mcp_audit_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			at TIMESTAMP NOT NULL,
			actor TEXT NOT NULL,
			client_id TEXT NOT NULL,
			tool TEXT NOT NULL,
			resource_kind TEXT NOT NULL,
			resource_id TEXT NOT NULL,
			result TEXT NOT NULL,
			trace_id TEXT NOT NULL,
			message TEXT NOT NULL
		)`)
	return err
}

func (s *SQLiteStore) Append(ctx context.Context, event audit.Event) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mcp_audit_log (at, actor, client_id, tool, resource_kind, resource_id, result, trace_id, message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.At.UTC(),
		event.Actor,
		event.ClientID,
		event.Tool,
		event.ResourceKind,
		event.ResourceID,
		event.Result,
		event.TraceID,
		event.Message,
	)
	return err
}

func (s *SQLiteStore) List(ctx context.Context, limit int) ([]audit.Event, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT at, actor, client_id, tool, resource_kind, resource_id, result, trace_id, message
		FROM mcp_audit_log
		ORDER BY id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]audit.Event, 0, limit)
	for rows.Next() {
		var e audit.Event
		if err := rows.Scan(&e.At, &e.Actor, &e.ClientID, &e.Tool, &e.ResourceKind, &e.ResourceID, &e.Result, &e.TraceID, &e.Message); err != nil {
			return nil, err
		}
		e.At = e.At.UTC()
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func NewAuditStore(backend, sqlitePath string) (interface {
	Append(context.Context, audit.Event) error
	List(context.Context, int) ([]audit.Event, error)
}, func() error, error) {
	switch backend {
	case "stdout":
		mem := NewMemory()
		return mem, func() error { return nil }, nil
	case "sqlite":
		s, err := OpenSQLite(sqlitePath)
		if err != nil {
			return nil, nil, fmt.Errorf("open sqlite audit store: %w", err)
		}
		return s, s.Close, nil
	default:
		return nil, nil, fmt.Errorf("unsupported audit backend: %s", backend)
	}
}
