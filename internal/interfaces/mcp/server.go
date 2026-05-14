package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joey/lumen-mcp-server/internal/domain/tool"
)

type ToolInvoker interface {
	InvokeTool(ctx context.Context, bearer, toolName string, args map[string]any, traceID string) (map[string]any, error)
}

type Server struct {
	inner        *gomcp.Server
	Invoker      ToolInvoker
	Log          *slog.Logger
	StaticBearer string
}

func New(catalog []tool.Definition, invoker ToolInvoker, staticBearer string, logger *slog.Logger) *Server {
	impl := &gomcp.Implementation{
		Name:    "lumen-mcp-server",
		Version: "0.1.0",
	}
	inner := gomcp.NewServer(impl, &gomcp.ServerOptions{
		Instructions: "Lumen API Gateway management server. Use these tools to list, create, update, and delete gateway resources (routes, services, upstreams, plugins, etc).",
		Logger:       logger,
	})

	s := &Server{
		inner:        inner,
		Invoker:      invoker,
		Log:          logger,
		StaticBearer: staticBearer,
	}

	for _, def := range catalog {
		s.registerTool(def)
	}

	return s
}

func (s *Server) registerTool(def tool.Definition) {
	t := &gomcp.Tool{
		Name:        def.Name,
		Description: def.Description,
		InputSchema: inputSchemaFor(def.Name),
	}
	handler := s.makeHandler(def.Name)
	s.inner.AddTool(t, handler)
}

func (s *Server) makeHandler(toolName string) gomcp.ToolHandler {
	return func(ctx context.Context, req *gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		var args map[string]any
		if req.Params.Arguments != nil {
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return errorResult("invalid arguments: " + err.Error()), nil
			}
		}
		if args == nil {
			args = map[string]any{}
		}

		bearer := s.extractBearer(req)
		if bearer == "" {
			return errorResult("unauthorized: bearer token required"), nil
		}

		result, err := s.Invoker.InvokeTool(ctx, bearer, toolName, args, "")
		if err != nil {
			return errorResult(err.Error()), nil
		}

		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return errorResult("failed to marshal result: " + err.Error()), nil
		}

		return &gomcp.CallToolResult{
			Content: []gomcp.Content{
				&gomcp.TextContent{Text: string(data)},
			},
		}, nil
	}
}

func (s *Server) extractBearer(req *gomcp.CallToolRequest) string {
	if req.Extra != nil && req.Extra.Header != nil {
		if auth := req.Extra.Header.Get("Authorization"); auth != "" {
			return auth
		}
	}
	return s.StaticBearer
}

func (s *Server) RunStdio(ctx context.Context) error {
	return s.inner.Run(ctx, &gomcp.StdioTransport{})
}

func (s *Server) StreamableHTTPHandler() http.Handler {
	return gomcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *gomcp.Server { return s.inner },
		&gomcp.StreamableHTTPOptions{
			Logger: s.Log,
		},
	)
}

func errorResult(msg string) *gomcp.CallToolResult {
	return &gomcp.CallToolResult{
		IsError: true,
		Content: []gomcp.Content{
			&gomcp.TextContent{Text: msg},
		},
	}
}
