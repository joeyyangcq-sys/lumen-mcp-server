package routes

import (
	"expvar"
	"net/http"

	"github.com/joey/lumen-mcp-server/internal/interfaces/http/handlers"
	"github.com/joey/lumen-mcp-server/internal/interfaces/http/middleware"
	"github.com/joey/lumen-mcp-server/internal/platform/logging"
	"github.com/joey/lumen-mcp-server/internal/platform/observability"
)

func New(log *logging.Logger, metrics *observability.Metrics, h handlers.Handler, mcpHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.Healthz)
	mux.HandleFunc("/.well-known/oauth-protected-resource", h.ProtectedResourceMetadata)
	mux.HandleFunc("/.well-known/oauth-protected-resource/mcp", h.ProtectedResourceMetadata)
	mux.HandleFunc("/admin/tools", h.ListTools)
	mux.HandleFunc("/admin/audit", h.ListAudit)
	mux.HandleFunc("/admin/tools/invoke", h.InvokeTool)
	mux.Handle("/debug/vars", expvar.Handler())

	if mcpHandler != nil {
		mux.Handle("/mcp", mcpHandler)
	}

	return middleware.Chain(
		mux,
		middleware.CORS,
		middleware.RequestID,
		middleware.Recovery(log),
		middleware.AccessLog(log),
		middleware.Metrics(metrics),
	)
}
