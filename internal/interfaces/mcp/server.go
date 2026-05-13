package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"

	"github.com/joey/lumen-mcp-server/internal/platform/logging"
)

type Server struct {
	Log *logging.Logger
	In  io.Reader
	Out io.Writer
}

func (s Server) Run(ctx context.Context) error {
	// TODO: replace with full MCP lifecycle + tool/resource/protocol negotiation.
	s.Log.Info("mcp stdio server started")
	scanner := bufio.NewScanner(s.In)
	enc := json.NewEncoder(s.Out)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		var req map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			_ = enc.Encode(map[string]any{"jsonrpc": "2.0", "error": map[string]any{"code": -32700, "message": "parse error"}, "id": nil})
			continue
		}
		_ = enc.Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"error": map[string]any{
				"code":    -32601,
				"message": "method not implemented in scaffold",
			},
		})
	}
	return scanner.Err()
}
