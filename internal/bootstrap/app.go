package bootstrap

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joey/lumen-mcp-server/internal/application/authorize"
	"github.com/joey/lumen-mcp-server/internal/application/invoke"
	"github.com/joey/lumen-mcp-server/internal/config"
	"github.com/joey/lumen-mcp-server/internal/domain/tool"
	"github.com/joey/lumen-mcp-server/internal/infrastructure/auditstore"
	"github.com/joey/lumen-mcp-server/internal/infrastructure/gatewayclient"
	"github.com/joey/lumen-mcp-server/internal/infrastructure/jwkscache"
	httpHandlers "github.com/joey/lumen-mcp-server/internal/interfaces/http/handlers"
	httpRoutes "github.com/joey/lumen-mcp-server/internal/interfaces/http/routes"
	"github.com/joey/lumen-mcp-server/internal/interfaces/mcp"
	"github.com/joey/lumen-mcp-server/internal/platform/logging"
	"github.com/joey/lumen-mcp-server/internal/platform/observability"
)

type App struct {
	Config     config.Config
	Logger     *logging.Logger
	Metrics    *observability.Metrics
	HTTPServer *http.Server
	MCPServer  mcp.Server
	closeAudit func() error
}

type staticCatalog []tool.Definition

func (s staticCatalog) List(context.Context) ([]tool.Definition, error) { return s, nil }

func New(cfg config.Config) *App {
	log := logging.New(cfg.Logging.Level, cfg.Logging.Format)
	metrics := observability.New()
	audit, closeAudit, err := auditstore.NewAuditStore(cfg.Audit.Backend, cfg.Audit.SQLitePath)
	if err != nil {
		panic(err)
	}
	inv := invoke.Service{
		Verifier: &jwkscache.Verifier{
			Issuer:   cfg.OAuth.Issuer,
			Audience: cfg.OAuth.Audience,
			JWKSURL:  cfg.OAuth.JWKSURL,
		},
		Authorize: authorize.Service{ToolScopeMap: cfg.Auth.ToolScopeMap},
		Gateway:   gatewayclient.New(cfg.Gateway.BaseURL, cfg.Gateway.AdminAPIKey),
		Audit:     audit,
	}

	catalog := staticCatalog{
		{Name: "list_routes", Description: "List gateway routes", Scope: "routes:read"},
		{Name: "get_route", Description: "Get route", Scope: "routes:read"},
		{Name: "put_route", Description: "Create/update route", Scope: "routes:write"},
		{Name: "patch_route", Description: "Patch route", Scope: "routes:write"},
		{Name: "delete_route", Description: "Delete route", Scope: "routes:write"},
		{Name: "list_services", Description: "List gateway services", Scope: "services:read"},
		{Name: "put_service", Description: "Create/update service", Scope: "services:write"},
		{Name: "list_upstreams", Description: "List gateway upstreams", Scope: "upstreams:read"},
		{Name: "put_upstream", Description: "Create/update upstream", Scope: "upstreams:write"},
		{Name: "list_plugin_configs", Description: "List plugin configs", Scope: "plugins:read"},
		{Name: "put_plugin_config", Description: "Create/update plugin config", Scope: "plugins:write"},
		{Name: "list_global_rules", Description: "List global rules", Scope: "global_rules:read"},
		{Name: "put_global_rule", Description: "Create/update global rule", Scope: "global_rules:write"},
		{Name: "preview_import", Description: "Preview import bundle", Scope: "gateway:bundle:apply"},
		{Name: "apply_import", Description: "Apply import bundle", Scope: "gateway:bundle:apply"},
		{Name: "export_bundle", Description: "Export bundle", Scope: "routes:read"},
		{Name: "history_list", Description: "List history", Scope: "routes:read"},
		{Name: "history_rollback", Description: "Rollback history", Scope: "admin:dangerous"},
		{Name: "get_schema", Description: "Get control schema", Scope: "routes:read"},
		{Name: "list_plugins", Description: "List plugin catalog", Scope: "plugins:read"},
		{Name: "get_stats", Description: "Get control stats", Scope: "metrics:read"},
	}
	resource := cfg.OAuth.Audience
	if resource == "" {
		resource = strings.TrimRight(cfg.Server.PublicBaseURL, "/") + cfg.Server.MCPEndpoint
	}
	h := httpHandlers.Handler{
		Invoker:               inv,
		Catalog:               catalog,
		Audit:                 audit,
		Resource:              resource,
		AuthorizationServer:   cfg.OAuth.Issuer,
		ResourceMetadataURL:   cfg.OAuth.ProtectedResourceMetadataURL,
		ScopesSupported:       cfg.Auth.ScopesSupported,
		DefaultChallengeScope: cfg.Auth.DefaultChallengeScope,
	}

	return &App{
		Config:     cfg,
		Logger:     log,
		Metrics:    metrics,
		closeAudit: closeAudit,
		HTTPServer: &http.Server{
			Addr:         cfg.Server.HTTPListen,
			Handler:      httpRoutes.New(log, metrics, h),
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
		},
		MCPServer: mcp.Server{Log: log, In: os.Stdin, Out: os.Stdout},
	}
}

func (a *App) Run(ctx context.Context) error {
	defer func() {
		if a.closeAudit != nil {
			if err := a.closeAudit(); err != nil {
				a.Logger.Error("close audit store failed", "error", err)
			}
		}
	}()

	errCh := make(chan error, 2)
	go func() {
		a.Logger.Info("mcp admin http server starting", "listen", a.Config.Server.HTTPListen)
		errCh <- a.HTTPServer.ListenAndServe()
	}()
	go func() {
		errCh <- a.MCPServer.Run(ctx)
	}()

	select {
	case <-ctx.Done():
		a.Logger.Info("mcp server shutting down")
		sdCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = a.HTTPServer.Shutdown(sdCtx)
		return ctx.Err()
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}
