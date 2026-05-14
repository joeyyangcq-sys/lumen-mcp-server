package auditstore

import (
	"context"
	"fmt"
	"sync"

	"github.com/joey/lumen-mcp-server/internal/domain/audit"
)

type MemoryStore struct {
	mu     sync.RWMutex
	events []audit.Event
}

func NewMemory() *MemoryStore {
	return &MemoryStore{}
}

func (s *MemoryStore) Append(_ context.Context, event audit.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

func (s *MemoryStore) List(_ context.Context, limit int) ([]audit.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > len(s.events) {
		limit = len(s.events)
	}
	start := len(s.events) - limit
	if start < 0 {
		start = 0
	}
	out := make([]audit.Event, 0, limit)
	out = append(out, s.events[start:]...)
	return out, nil
}

type AuditStore interface {
	Append(context.Context, audit.Event) error
	List(context.Context, int) ([]audit.Event, error)
}

func NewAuditStore(backend, sqlitePath, postgresURL string) (AuditStore, func() error, error) {
	switch backend {
	case "stdout":
		return NewMemory(), func() error { return nil }, nil
	case "sqlite":
		s, err := OpenSQLite(sqlitePath)
		if err != nil {
			return nil, nil, fmt.Errorf("open sqlite audit store: %w", err)
		}
		return s, s.Close, nil
	case "postgres":
		s, err := OpenPostgres(postgresURL)
		if err != nil {
			return nil, nil, fmt.Errorf("open postgres audit store: %w", err)
		}
		return s, s.Close, nil
	default:
		return nil, nil, fmt.Errorf("unsupported audit backend: %s", backend)
	}
}
