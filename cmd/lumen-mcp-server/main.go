package main

import (
	"context"
	"flag"
	"os/signal"
	"syscall"

	"github.com/joey/lumen-mcp-server/internal/bootstrap"
	"github.com/joey/lumen-mcp-server/internal/config"
)

func main() {
	configPath := flag.String("config", "configs/config.example.yaml", "path to config yaml")
	stdioMode := flag.Bool("stdio", false, "run as MCP stdio server (for Claude Code integration)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		panic(err)
	}
	app := bootstrap.New(cfg)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if *stdioMode {
		if err := app.RunStdio(ctx); err != nil && err != context.Canceled {
			panic(err)
		}
		return
	}

	if err := app.Run(ctx); err != nil && err != context.Canceled {
		panic(err)
	}
}
