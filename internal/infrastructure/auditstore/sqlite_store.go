package auditstore

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	sqlStore
}

func questionMark(_ int) string { return "?" }

func OpenSQLite(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	store := &SQLiteStore{sqlStore{db: db, placeholder: questionMark}}
	if err := store.initSchema(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
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
	if err != nil {
		return fmt.Errorf("init sqlite schema: %w", err)
	}
	return nil
}
