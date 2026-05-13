package ports

import (
	"context"

	"github.com/joey/lumen-mcp-server/internal/domain/audit"
	"github.com/joey/lumen-mcp-server/internal/domain/tool"
)

type Claims struct {
	Subject string
	Client  string
	Scopes  []string
}

type TokenVerifier interface {
	VerifyBearer(ctx context.Context, bearer string) (Claims, error)
}

type GatewayAdminClient interface {
	InvokeTool(ctx context.Context, toolName string, args map[string]any) (map[string]any, error)
}

type ToolCatalog interface {
	List(ctx context.Context) ([]tool.Definition, error)
}

type AuditStore interface {
	Append(ctx context.Context, event audit.Event) error
	List(ctx context.Context, limit int) ([]audit.Event, error)
}
