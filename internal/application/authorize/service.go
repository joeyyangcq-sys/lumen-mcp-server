package authorize

import (
	"strings"

	"github.com/joey/lumen-mcp-server/internal/domain/policy"
)

type Service struct {
	ToolScopeMap   map[string]string
	RequiredScopes []string
}

func (s Service) Check(scopes []string, toolName string) policy.Decision {
	for _, required := range s.RequiredScopes {
		required = strings.TrimSpace(required)
		if required == "" {
			continue
		}
		if !hasAnyScope(scopes, required, false) {
			return policy.Decision{Allowed: false, Reason: "missing required scope: " + required}
		}
	}

	required, ok := s.ToolScopeMap[toolName]
	if !ok {
		return policy.Decision{Allowed: false, Reason: "tool not registered in auth.tool_scope_map"}
	}
	if hasAnyScope(scopes, required, true) {
		return policy.Decision{Allowed: true}
	}
	return policy.Decision{Allowed: false, Reason: "missing required scope: " + required}
}

func hasAnyScope(scopes []string, required string, allowAdminWildcard bool) bool {
	for _, scope := range scopes {
		if (allowAdminWildcard && strings.EqualFold(scope, "admin:*")) || hasScope(required, scope) {
			return true
		}
	}
	return false
}

func hasScope(required, actual string) bool {
	if strings.EqualFold(required, actual) {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(required)) {
	case "read":
		legacyRead := []string{
			"mcp:read",
			"routes:read",
			"services:read",
			"upstreams:read",
			"plugins:read",
			"global_rules:read",
			"metrics:read",
			"gateway:read",
		}
		for _, scope := range legacyRead {
			if strings.EqualFold(actual, scope) {
				return true
			}
		}
	case "gateway:write":
		legacyWrite := []string{
			"mcp:write",
			"routes:write",
			"services:write",
			"upstreams:write",
			"plugins:write",
			"global_rules:write",
			"gateway:bundle:apply",
			"gateway:dangerous",
			"admin:dangerous",
		}
		for _, scope := range legacyWrite {
			if strings.EqualFold(actual, scope) {
				return true
			}
		}
	}
	return false
}
