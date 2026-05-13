package authorize

import (
	"strings"

	"github.com/joey/lumen-mcp-server/internal/domain/policy"
)

type Service struct {
	ToolScopeMap map[string]string
}

func (s Service) Check(scopes []string, toolName string) policy.Decision {
	required, ok := s.ToolScopeMap[toolName]
	if !ok {
		return policy.Decision{Allowed: false, Reason: "tool not registered in auth.tool_scope_map"}
	}
	for _, scope := range scopes {
		if strings.EqualFold(scope, required) || strings.EqualFold(scope, "admin:*") {
			return policy.Decision{Allowed: true}
		}
	}
	return policy.Decision{Allowed: false, Reason: "missing required scope: " + required}
}
