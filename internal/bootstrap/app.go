package bootstrap

import (
	"context"
	"net/http"
	"os"
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
}

type staticCatalog []tool.Definition

func (s staticCatalog) List(context.Context) ([]tool.Definition, error) { return s, nil }

func New(cfg config.Config) *App {
	log := logging.New(cfg.Logging.Level, cfg.Logging.Format)
	metrics := observability.New()
	audit := auditstore.NewMemory()
	inv := invoke.Service{
		Verifier: jwkscache.Verifier{
			Issuer:   cfg.OAuth.Issuer,
			Audience: cfg.OAuth.Audience,
			JWKSURL:  cfg.OAuth.JWKSURL,
		},
		Authorize: authorize.Service{ToolScopeMap: cfg.Auth.ToolScopeMap},
		Gateway:   gatewayclient.New(cfg.Gateway.BaseURL, cfg.Gateway.AdminAPIKey),
		Audit:     audit,
	}

	catalog := staticCatalog{{Name: "list_routes", Description: "List gateway routes", Scope: "routes:read"}, {Name: "put_route", Description: "Create/update route", Scope: "routes:write"}}
	h := httpHandlers.Handler{Invoker: inv, Catalog: catalog, Audit: audit}

	return &App{
		Config:  cfg,
		Logger:  log,
		Metrics: metrics,
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
