package invoke

import (
	"context"
	"time"

	"github.com/joey/lumen-mcp-server/internal/application/authorize"
	"github.com/joey/lumen-mcp-server/internal/application/ports"
	"github.com/joey/lumen-mcp-server/internal/domain/audit"
)

type Service struct {
	Verifier  ports.TokenVerifier
	Authorize authorize.Service
	Gateway   ports.GatewayAdminClient
	Audit     ports.AuditStore
}

func (s Service) InvokeTool(ctx context.Context, bearer, toolName string, args map[string]any, traceID string) (map[string]any, error) {
	claims, err := s.Verifier.VerifyBearer(ctx, bearer)
	if err != nil {
		_ = s.Audit.Append(ctx, audit.Event{At: time.Now().UTC(), Tool: toolName, Result: "deny", TraceID: traceID, Message: err.Error()})
		return nil, ErrUnauthorized{Reason: err.Error()}
	}
	decision := s.Authorize.Check(claims.Scopes, toolName)
	if !decision.Allowed {
		_ = s.Audit.Append(ctx, audit.Event{At: time.Now().UTC(), Actor: claims.Subject, ClientID: claims.Client, Tool: toolName, Result: "deny", TraceID: traceID, Message: decision.Reason})
		return nil, ErrForbidden{Reason: decision.Reason}
	}
	result, err := s.Gateway.InvokeTool(ctx, toolName, args)
	if err != nil {
		_ = s.Audit.Append(ctx, audit.Event{At: time.Now().UTC(), Actor: claims.Subject, ClientID: claims.Client, Tool: toolName, Result: "error", TraceID: traceID, Message: err.Error()})
		return nil, err
	}
	_ = s.Audit.Append(ctx, audit.Event{At: time.Now().UTC(), Actor: claims.Subject, ClientID: claims.Client, Tool: toolName, Result: "allow", TraceID: traceID})
	return result, nil
}

type ErrForbidden struct {
	Reason string
}

func (e ErrForbidden) Error() string {
	return e.Reason
}

type ErrUnauthorized struct {
	Reason string
}

func (e ErrUnauthorized) Error() string {
	return e.Reason
}
