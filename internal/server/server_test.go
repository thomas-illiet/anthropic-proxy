package server

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/thomas-illiet/anthropic-proxy/internal/config"
	"github.com/thomas-illiet/anthropic-proxy/internal/logging"
)

func serverTestConfig() *config.Config {
	return &config.Config{
		ListenAddr:     "127.0.0.1:0",
		UpstreamURL:    "http://127.0.0.1:1/v1/chat/completions",
		UpstreamKey:    "upstream-secret",
		DefaultModel:   "upstream-model",
		ToolFormat:     config.ToolFormatXML,
		ForceModel:     true,
		RequestTimeout: 200 * time.Millisecond,
		MaxRequestBody: 32 << 20,
		ModelMap:       map[string]string{},
		LogLevel:       "info",
	}
}

// TestRunRejectsNilConfig verifies lifecycle startup validates required wiring.
func TestRunRejectsNilConfig(t *testing.T) {
	if err := Run(context.Background(), nil, logging.NewDiscard()); err == nil || !strings.Contains(err.Error(), "nil config") {
		t.Fatalf("expected nil config error, got %v", err)
	}
}

// TestRunReturnsListenError verifies listener startup failures are returned.
func TestRunReturnsListenError(t *testing.T) {
	cfg := serverTestConfig()
	cfg.ListenAddr = "127.0.0.1:not-a-port"
	err := Run(context.Background(), cfg, logging.NewDiscard())
	if err == nil {
		t.Fatal("expected listen error")
	}
}

// TestRunStopsWhenContextIsCanceled verifies graceful shutdown on caller cancellation.
func TestRunStopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, serverTestConfig(), logging.NewDiscard())
	}()

	time.Sleep(25 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error after cancellation: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}
}

// TestLogStartupConfigOmitsSecrets verifies startup diagnostics do not leak credentials.
func TestLogStartupConfigOmitsSecrets(t *testing.T) {
	cfg := serverTestConfig()
	cfg.ExpectedClientKey = "client-secret"
	cfg.ModelMap = map[string]string{"claude-sonnet": "upstream-model"}

	var buf bytes.Buffer
	logger, err := logging.New(&buf, "info")
	if err != nil {
		t.Fatal(err)
	}
	LogStartupConfig(logger, cfg)

	got := buf.String()
	for _, want := range []string{"startup", "model mapping", "claude-sonnet", "upstream-model"} {
		if !strings.Contains(got, want) {
			t.Fatalf("startup log missing %q:\n%s", want, got)
		}
	}
	for _, secret := range []string{cfg.UpstreamKey, cfg.ExpectedClientKey} {
		if strings.Contains(got, secret) {
			t.Fatalf("startup log leaked secret %q:\n%s", secret, got)
		}
	}

	LogStartupConfig(nil, cfg)
	LogStartupConfig(logger, nil)
}
