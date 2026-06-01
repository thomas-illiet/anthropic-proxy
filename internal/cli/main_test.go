package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/thomas-illiet/anthropic-proxy/internal/config"
	"github.com/thomas-illiet/anthropic-proxy/internal/logging"
)

// TestVersionCommand verifies the Cobra version subcommand prints the build version.
func TestVersionCommand(t *testing.T) {
	original := version
	version = "test-version"
	t.Cleanup(func() { version = original })

	cmd := newRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"version"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != "test-version" {
		t.Fatalf("version output = %q", got)
	}
}

// TestRootShowsHelp verifies the root command is informational and does not start the server.
func TestRootShowsHelp(t *testing.T) {
	cmd := newRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); !strings.Contains(got, "serve") || !strings.Contains(got, "version") {
		t.Fatalf("help output should mention serve and version, got %q", got)
	}
}

// TestServeRejectsUnknownFlag verifies serve accepts no runtime config flags.
func TestServeRejectsUnknownFlag(t *testing.T) {
	cmd := newRootCommand()
	cmd.SetArgs([]string{"serve", "--unknown"})

	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unknown flag: --unknown") {
		t.Fatalf("expected unknown flag error, got %v", err)
	}
}

// TestServeLoadsCurrentDotEnv verifies serve reads .env from the working directory before running.
func TestServeLoadsCurrentDotEnv(t *testing.T) {
	clearServeConfigEnv(t)
	t.Chdir(t.TempDir())
	t.Setenv("ANTHROPIC_PROXY_DEFAULT_MODEL", "env-model")
	writeTestDotEnv(t, `
ANTHROPIC_PROXY_UPSTREAM_API_KEY=file-key
ANTHROPIC_PROXY_DEFAULT_MODEL=file-model
`)

	var got *config.Config
	original := runServer
	runServer = func(ctx context.Context, cfg *config.Config, logger *logging.Logger) error {
		got = cfg
		return nil
	}
	t.Cleanup(func() { runServer = original })

	cmd := newRootCommand()
	cmd.SetArgs([]string{"serve"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("server runner was not called")
	}
	if got.UpstreamKey != "file-key" {
		t.Fatalf("UpstreamKey = %q", got.UpstreamKey)
	}
	if got.DefaultModel != "env-model" {
		t.Fatalf("env should override .env, got DefaultModel = %q", got.DefaultModel)
	}
}

// TestServeLoadsEnvironmentOnly verifies serve works when no .env exists.
func TestServeLoadsEnvironmentOnly(t *testing.T) {
	clearServeConfigEnv(t)
	t.Chdir(t.TempDir())
	t.Setenv("ANTHROPIC_PROXY_UPSTREAM_API_KEY", "env-key")
	t.Setenv("ANTHROPIC_PROXY_DEFAULT_MODEL", "env-model")

	var got *config.Config
	original := runServer
	runServer = func(ctx context.Context, cfg *config.Config, logger *logging.Logger) error {
		got = cfg
		return nil
	}
	t.Cleanup(func() { runServer = original })

	cmd := newRootCommand()
	cmd.SetArgs([]string{"serve"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got == nil || got.UpstreamKey != "env-key" || got.DefaultModel != "env-model" {
		t.Fatalf("unexpected loaded config: %+v", got)
	}
}

// clearServeConfigEnv clears configuration keys used by CLI tests.
func clearServeConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"ANTHROPIC_PROXY_UPSTREAM_API_KEY",
		"ANTHROPIC_PROXY_DEFAULT_MODEL",
		"ANTHROPIC_PROXY_FORCE_MODEL",
		"ANTHROPIC_PROXY_MODEL_MAP",
		"ANTHROPIC_PROXY_LOG_LEVEL",
	} {
		t.Setenv(key, "")
	}
}

// writeTestDotEnv writes a .env file in the test working directory.
func writeTestDotEnv(t *testing.T, body string) {
	t.Helper()
	if err := os.WriteFile(".env", []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
