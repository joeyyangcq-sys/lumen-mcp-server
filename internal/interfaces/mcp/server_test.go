package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joey/lumen-mcp-server/internal/domain/tool"
)

type fakeInvoker struct {
	lastBearer string
}

func (f *fakeInvoker) InvokeTool(_ context.Context, bearer, toolName string, args map[string]any, _ string) (map[string]any, error) {
	f.lastBearer = bearer
	return map[string]any{"tool": toolName, "echo": args}, nil
}

func TestMCPServer_ToolsListAndCall(t *testing.T) {
	catalog := []tool.Definition{
		{Name: "list_routes", Description: "List gateway routes", Scope: "routes:read"},
		{Name: "get_route", Description: "Get route", Scope: "routes:read"},
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	invoker := &fakeInvoker{}
	srv := New(catalog, invoker, "Bearer static-test-token", logger)

	client := gomcp.NewClient(&gomcp.Implementation{Name: "test-client", Version: "0.1"}, nil)
	ct, st := gomcp.NewInMemoryTransports()

	ctx := context.Background()
	serverDone := make(chan error, 1)
	go func() { serverDone <- srv.inner.Run(ctx, st) }()

	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer session.Close()

	toolsResult, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	if len(toolsResult.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(toolsResult.Tools))
	}

	names := map[string]bool{}
	for _, tool := range toolsResult.Tools {
		names[tool.Name] = true
	}
	if !names["list_routes"] || !names["get_route"] {
		t.Fatalf("expected list_routes and get_route, got %v", names)
	}

	callResult, err := session.CallTool(ctx, &gomcp.CallToolParams{
		Name:      "list_routes",
		Arguments: map[string]any{"page": 1},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if callResult.IsError {
		t.Fatalf("CallTool() returned error: %v", callResult.Content)
	}
	if len(callResult.Content) == 0 {
		t.Fatal("expected non-empty content")
	}

	textContent, ok := callResult.Content[0].(*gomcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", callResult.Content[0])
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result["tool"] != "list_routes" {
		t.Errorf("expected tool=list_routes, got %v", result["tool"])
	}
	if invoker.lastBearer != "Bearer static-test-token" {
		t.Errorf("expected bearer='Bearer static-test-token', got %q", invoker.lastBearer)
	}
}

func TestMCPServer_RejectsMissingBearer(t *testing.T) {
	catalog := []tool.Definition{
		{Name: "list_routes", Description: "List gateway routes", Scope: "routes:read"},
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	invoker := &fakeInvoker{}
	srv := New(catalog, invoker, "", logger)

	client := gomcp.NewClient(&gomcp.Implementation{Name: "test-client", Version: "0.1"}, nil)
	ct, st := gomcp.NewInMemoryTransports()

	ctx := context.Background()
	go func() { _ = srv.inner.Run(ctx, st) }()

	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer session.Close()

	callResult, err := session.CallTool(ctx, &gomcp.CallToolParams{
		Name: "list_routes",
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if !callResult.IsError {
		t.Fatal("expected error result when no bearer token")
	}
	tc, ok := callResult.Content[0].(*gomcp.TextContent)
	if !ok {
		t.Fatal("expected TextContent")
	}
	if tc.Text != "unauthorized: bearer token required" {
		t.Errorf("unexpected error message: %s", tc.Text)
	}
}
