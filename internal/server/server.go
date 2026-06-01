package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/thomas-illiet/anthropic-proxy/internal/config"
	"github.com/thomas-illiet/anthropic-proxy/internal/logging"
	"github.com/thomas-illiet/anthropic-proxy/internal/proxy"
)

const shutdownTimeout = 10 * time.Second

// Run owns the HTTP server lifecycle, including timeouts and graceful shutdown.
func Run(ctx context.Context, cfg *config.Config, logger *logging.Logger) error {
	if cfg == nil {
		return fmt.Errorf("nil config")
	}
	if logger == nil {
		logger = logging.NewDiscard()
	}
	if ctx == nil {
		ctx = context.Background()
	}

	p := proxy.NewWithLogger(cfg, logger)
	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           p.Routes(),
		ReadHeaderTimeout: 30 * time.Second,
		ReadTimeout:       cfg.RequestTimeout,
		WriteTimeout:      cfg.RequestTimeout + 30*time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		logger.Error("server stopped", "error", err)
		return err
	case <-signalCtx.Done():
		logger.Info("shutdown signal received")
		stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		_ = srv.Close()
		return err
	}
	if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server stopped", "error", err)
		return err
	}
	logger.Info("server stopped")
	return nil
}

// LogStartupConfig logs effective non-secret startup settings.
func LogStartupConfig(logger *logging.Logger, cfg *config.Config) {
	if logger == nil || cfg == nil {
		return
	}
	logger.Info("startup",
		"service", "anthropic-proxy",
		"listen", cfg.ListenAddr,
		"upstream", cfg.UpstreamURL,
		"default_model", cfg.DefaultModel,
		"tool_format", cfg.ToolFormat,
		"force_model", cfg.ForceModel,
		"models_mapped", len(cfg.ModelMap),
		"log_level", cfg.LogLevel,
	)
	for k, v := range cfg.ModelMap {
		logger.Info("model mapping", "requested_model", k, "upstream_model", v)
	}
}
