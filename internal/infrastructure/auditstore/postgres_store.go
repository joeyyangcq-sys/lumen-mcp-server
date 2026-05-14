package auditstore

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresStore struct {
	sqlStore
}

func dollarN(n int) string { return "$" + strconv.Itoa(n) }

func OpenPostgres(connStr string) (*PostgresStore, error) {
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	store := &PostgresStore{sqlStore{db: db, placeholder: dollarN}}
	if err := store.initSchema(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *PostgresStore) initSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS mcp_audit_log (
			id BIGSERIAL PRIMARY KEY,
			at TIMESTAMPTZ NOT NULL,
			actor TEXT NOT NULL,
			client_id TEXT NOT NULL,
			tool TEXT NOT NULL,
			resource_kind TEXT NOT NULL DEFAULT '',
			resource_id TEXT NOT NULL DEFAULT '',
			result TEXT NOT NULL,
			trace_id TEXT NOT NULL,
			message TEXT NOT NULL DEFAULT ''
		)`)
	if err != nil {
		return fmt.Errorf("init postgres schema: %w", err)
	}
	return nil
}
