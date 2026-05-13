package auditstore

import (
	"context"
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
