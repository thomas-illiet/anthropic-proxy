package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thomas-illiet/anthropic-proxy/internal/config"
	"github.com/thomas-illiet/anthropic-proxy/internal/logging"
	"github.com/thomas-illiet/anthropic-proxy/internal/server"
)

var (
	loadConfig = config.Load
	runServer  = server.Run
)

// newServeCommand starts the HTTP proxy after loading config from .env and env vars.
func newServeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the Anthropic-compatible proxy server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServe(cmd.Context())
		},
	}
}

// runServe wires together configuration, logging, startup diagnostics, and the server runtime.
func runServe(ctx context.Context) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	logger := logging.NewStderr(cfg.LogLevel)
	if logging.IsVerbose(cfg.LogLevel) {
		logger.Warn("verbose logging is enabled; upstream error details may appear in logs")
	}
	server.LogStartupConfig(logger, cfg)

	return runServer(ctx, cfg, logger)
}
