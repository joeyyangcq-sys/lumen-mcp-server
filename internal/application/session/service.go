package session

import "context"

type Service struct{}

func (Service) List(_ context.Context) ([]map[string]any, error) {
	// TODO: support active MCP session inspection.
	return []map[string]any{}, nil
}
