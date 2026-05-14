package auditstore

import (
	"context"
	"database/sql"

	"github.com/joey/lumen-mcp-server/internal/domain/audit"
)

type sqlStore struct {
	db          *sql.DB
	placeholder func(int) string
}

func (s *sqlStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *sqlStore) Append(ctx context.Context, event audit.Event) error {
	ph := s.placeholder
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mcp_audit_log (at, actor, client_id, tool, resource_kind, resource_id, result, trace_id, message)
		VALUES (`+ph(1)+`, `+ph(2)+`, `+ph(3)+`, `+ph(4)+`, `+ph(5)+`, `+ph(6)+`, `+ph(7)+`, `+ph(8)+`, `+ph(9)+`)`,
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

func (s *sqlStore) List(ctx context.Context, limit int) ([]audit.Event, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT at, actor, client_id, tool, resource_kind, resource_id, result, trace_id, message
		FROM mcp_audit_log
		ORDER BY id DESC
		LIMIT `+s.placeholder(1), limit)
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
