package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/joey/lumen-mcp-server/internal/application/invoke"
	"github.com/joey/lumen-mcp-server/internal/application/ports"
)

type Handler struct {
	Invoker invoke.Service
	Catalog ports.ToolCatalog
	Audit   ports.AuditStore
}

func (h Handler) Healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
}

func (h Handler) ListTools(w http.ResponseWriter, r *http.Request) {
	tools, err := h.Catalog.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), nil)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"list": tools, "total": len(tools)})
}

func (h Handler) ListAudit(w http.ResponseWriter, r *http.Request) {
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
