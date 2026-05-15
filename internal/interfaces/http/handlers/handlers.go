package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/joey/lumen-mcp-server/internal/application/authorize"
	"github.com/joey/lumen-mcp-server/internal/application/invoke"
	"github.com/joey/lumen-mcp-server/internal/application/ports"
)

type Handler struct {
	Invoker               invoke.Service
	Catalog               ports.ToolCatalog
	Audit                 ports.AuditStore
	Resource              string
	AuthorizationServer   string
	ResourceMetadataURL   string
	ScopesSupported       []string
	DefaultChallengeScope string
	Verifier              ports.TokenVerifier
	Authorize             authorize.Service
}

func (h Handler) Healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
}

func (h Handler) ProtectedResourceMetadata(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"resource":                 h.Resource,
		"authorization_servers":    []string{h.AuthorizationServer},
		"scopes_supported":         h.scopesSupported(),
		"bearer_methods_supported": []string{"header"},
	})
}

func (h Handler) MCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required", nil)
		return
	}
	if r.Header.Get("Authorization") == "" {
		h.writeBearerChallenge(w)
		return
	}
	writeError(w, http.StatusNotImplemented, "mcp_http_not_implemented", "streamable HTTP MCP endpoint is not implemented yet", nil)
}

func (h Handler) ListTools(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAuthorized(w, r); !ok {
		return
	}
	tools, err := h.Catalog.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), nil)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"list": tools, "total": len(tools)})
}

func (h Handler) ListAudit(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAuthorized(w, r); !ok {
		return
	}
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	items, err := h.Audit.List(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), nil)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"list": items, "total": len(items)})
}

func (h Handler) InvokeTool(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required", nil)
		return
	}
	var req struct {
		Tool string         `json:"tool"`
		Args map[string]any `json:"args"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid json body", nil)
		return
	}
	bearer := r.Header.Get("Authorization")
	traceID, _ := r.Context().Value("trace_id").(string)
	result, err := h.Invoker.InvokeTool(r.Context(), bearer, req.Tool, req.Args, traceID)
	if err != nil {
		if _, ok := err.(invoke.ErrUnauthorized); ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", err.Error(), map[string]any{"tool": req.Tool})
			return
		}
		if _, ok := err.(invoke.ErrForbidden); ok {
			writeError(w, http.StatusForbidden, "forbidden", err.Error(), map[string]any{"tool": req.Tool})
			return
		}
		writeError(w, http.StatusInternalServerError, "invoke_failed", err.Error(), map[string]any{"tool": req.Tool})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"result": result})
}

func writeError(w http.ResponseWriter, status int, code, message string, details map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code":    code,
		"message": message,
		"details": details,
	})
}

func (h Handler) writeBearerChallenge(w http.ResponseWriter) {
	value := `Bearer resource_metadata="` + h.ResourceMetadataURL + `"`
	if h.DefaultChallengeScope != "" {
		value += `, scope="` + h.DefaultChallengeScope + `"`
	}
	w.Header().Set("WWW-Authenticate", value)
	writeError(w, http.StatusUnauthorized, "unauthorized", "bearer token required", nil)
}

func (h Handler) requireAuthorized(w http.ResponseWriter, r *http.Request) (ports.Claims, bool) {
	bearer := strings.TrimSpace(r.Header.Get("Authorization"))
	if bearer == "" {
		h.writeBearerChallenge(w)
		return ports.Claims{}, false
	}
	if h.Verifier == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "token verifier is not configured", nil)
		return ports.Claims{}, false
	}
	claims, err := h.Verifier.VerifyBearer(r.Context(), bearer)
	if err != nil {
		h.writeBearerChallenge(w)
		return ports.Claims{}, false
	}

	for _, required := range h.Authorize.RequiredScopes {
		required = strings.TrimSpace(required)
		if required == "" {
			continue
		}
		if !hasScope(claims.Scopes, required) {
			writeError(w, http.StatusForbidden, "forbidden", "missing required scope: "+required, nil)
			return ports.Claims{}, false
		}
	}
	return claims, true
}

func hasScope(scopes []string, required string) bool {
	for _, scope := range scopes {
		if strings.EqualFold(strings.TrimSpace(scope), "admin:*") || strings.EqualFold(strings.TrimSpace(scope), required) {
			return true
		}
	}
	return false
}

func (h Handler) scopesSupported() []string {
	if len(h.ScopesSupported) > 0 {
		return h.ScopesSupported
	}
	return []string{"mcp:tools", "read", "gateway:write", "oauth:write"}
}
