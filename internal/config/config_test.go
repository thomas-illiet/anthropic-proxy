package config

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/thomas-illiet/anthropic-proxy/internal/anthropic"
)

var envKeys = []string{
	envListenAddr,
	envUpstreamURL,
	envUpstreamAPIKey,
	envDefaultModel,
	envToolFormat,
	envForceModel,
	envModelMap,
	envDefaultOpusModel,
	envDefaultOpusModelName,
	envDefaultSonnetModel,
	envDefaultSonnetModelName,
	envDefaultHaikuModel,
	envDefaultHaikuModelName,
	envClientKey,
	envRequestTimeoutSec,
	envMaxRequestBodyBytes,
	envForwardCacheControl,
	envLogLevel,
	"LISTEN_ADDR",
	"UPSTREAM_URL",
	"UPSTREAM_API_KEY",
	"DEFAULT_MODEL",
	"TOOL_FORMAT",
	"FORCE_MODEL",
	"MODEL_MAP",
	"ANTHROPIC_DEFAULT_OPUS_MODEL",
	"ANTHROPIC_DEFAULT_OPUS_MODEL_NAME",
	"ANTHROPIC_DEFAULT_SONNET_MODEL",
	"ANTHROPIC_DEFAULT_SONNET_MODEL_NAME",
	"ANTHROPIC_DEFAULT_HAIKU_MODEL",
	"ANTHROPIC_DEFAULT_HAIKU_MODEL_NAME",
	"ANTHROPIC_SMALL_FAST_MODEL",
	"PROXY_CLIENT_KEY",
	"REQUEST_TIMEOUT_SEC",
	"MAX_REQUEST_BODY_BYTES",
	"FORWARD_CACHE_CONTROL",
	"FORWARD_ANTHROPIC_HEADERS",
	"REQUIRE_ANTHROPIC_VERSION",
	"RATE_LIMIT",
	"RATE_LIMIT_WINDOW_SEC",
	"DEBUG",
	"LOG_LEVEL",
}

// clearConfigEnv clears all configuration environment variables for isolated config tests.
func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, k := range envKeys {
		t.Setenv(k, "")
	}
}

// loadFromValues writes a temporary dotenv file and loads it through Viper.
func loadFromValues(t *testing.T, values map[string]string) (*Config, error) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var body strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&body, "%s=%s\n", key, values[key])
	}
	if err := os.WriteFile(".env", []byte(body.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return Load()
}

// TestLoadDotEnv verifies comments, export prefixes, and quoted values.
func TestLoadDotEnv(t *testing.T) {
	clearConfigEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(".env", []byte(`
# comment
ANTHROPIC_PROXY_UPSTREAM_API_KEY=file-key
export ANTHROPIC_PROXY_DEFAULT_MODEL = "model-from-file"
ANTHROPIC_PROXY_FORCE_MODEL='0'
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UpstreamKey != "file-key" {
		t.Fatalf("UpstreamKey = %q", cfg.UpstreamKey)
	}
	if cfg.DefaultModel != "model-from-file" {
		t.Fatalf("DefaultModel = %q", cfg.DefaultModel)
	}
	if cfg.ForceModel {
		t.Fatal("ForceModel should be false")
	}
}

// TestLoadDotEnvRejectsInvalidLines verifies Viper rejects malformed dotenv input.
func TestLoadDotEnvRejectsInvalidLines(t *testing.T) {
	clearConfigEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(".env", []byte("IGNORED_LINE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("expected malformed dotenv error")
	}
}

// TestLoadWithoutDotEnv verifies missing .env is accepted when real env vars are set.
func TestLoadWithoutDotEnv(t *testing.T) {
	clearConfigEnv(t)
	t.Chdir(t.TempDir())
	t.Setenv(envDefaultModel, "env-model")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UpstreamKey != "" || cfg.DefaultModel != "env-model" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

// TestLoadEnvPrecedenceAndDefaults verifies environment precedence and common default parsing.
func TestLoadEnvPrecedenceAndDefaults(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv(envDefaultModel, "env-model")

	cfg, err := loadFromValues(t, map[string]string{
		envUpstreamAPIKey:      "file-key",
		envDefaultModel:        "file-model",
		envToolFormat:          "native",
		envRequestTimeoutSec:   "12",
		envMaxRequestBodyBytes: "123456",
		envForwardCacheControl: "1",
		envClientKey:           "client-key",
		envModelMap:            `{"claude":"mapped"}`,
		envLogLevel:            "debug",
		envListenAddr:          ":9999",
		envUpstreamURL:         "http://127.0.0.1:9999/v1/chat/completions",
	})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.DefaultModel != "env-model" {
		t.Fatalf("env should override file %s, got %q", envDefaultModel, cfg.DefaultModel)
	}
	if cfg.ToolFormat != ToolFormatNative {
		t.Fatalf("ToolFormat = %q", cfg.ToolFormat)
	}
	if cfg.UpstreamKey != "file-key" {
		t.Fatalf("UpstreamKey = %q", cfg.UpstreamKey)
	}
	if cfg.RequestTimeout != 12*time.Second {
		t.Fatalf("RequestTimeout = %v", cfg.RequestTimeout)
	}
	if cfg.MaxRequestBody != 123456 {
		t.Fatalf("MaxRequestBody = %d", cfg.MaxRequestBody)
	}
	if cfg.ExpectedClientKey != "client-key" || cfg.LogLevel != "debug" || cfg.ListenAddr != ":9999" || !cfg.ForwardCacheControl {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.ModelMap["claude"] != "mapped" {
		t.Fatalf("ModelMap = %#v", cfg.ModelMap)
	}
	if cfg.ModelAliases["sonnet"] != anthropic.DefaultSonnetModel || cfg.ModelAliases["opus"] != anthropic.DefaultOpusModel || cfg.ModelAliases["haiku"] != anthropic.DefaultHaikuModel {
		t.Fatalf("ModelAliases = %#v", cfg.ModelAliases)
	}
}

// TestLoadIgnoresLegacyEnvNames verifies the breaking change does not fall back to old names.
func TestLoadIgnoresLegacyEnvNames(t *testing.T) {
	t.Run("real environment", func(t *testing.T) {
		clearConfigEnv(t)
		t.Chdir(t.TempDir())
		t.Setenv("UPSTREAM_API_KEY", "legacy-key")
		t.Setenv("DEFAULT_MODEL", "legacy-model")
		t.Setenv("LOG_LEVEL", "debug")

		if _, err := Load(); err == nil || !strings.Contains(err.Error(), envDefaultModel) {
			t.Fatalf("legacy real env should be ignored, got err %v", err)
		}
	})

	t.Run("dotenv", func(t *testing.T) {
		clearConfigEnv(t)
		if _, err := loadFromValues(t, map[string]string{
			"UPSTREAM_API_KEY": "legacy-key",
			"DEFAULT_MODEL":    "legacy-model",
		}); err == nil || !strings.Contains(err.Error(), envDefaultModel) {
			t.Fatalf("legacy dotenv keys should be ignored, got err %v", err)
		}
	})
}

// TestLoadLogLevel verifies level parsing.
func TestLoadLogLevel(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    string
		wantErr bool
	}{
		{
			name: "default info",
			env:  map[string]string{},
			want: "info",
		},
		{
			name: "explicit level",
			env:  map[string]string{envLogLevel: "warn"},
			want: "warn",
		},
		{
			name: "warning alias",
			env:  map[string]string{envLogLevel: "warning"},
			want: "warn",
		},
		{
			name:    "invalid",
			env:     map[string]string{envLogLevel: "chatty"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clearConfigEnv(t)
			env := map[string]string{
				envUpstreamAPIKey: "key",
				envDefaultModel:   "model",
			}
			for k, v := range tc.env {
				env[k] = v
			}
			cfg, err := loadFromValues(t, env)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.LogLevel != tc.want {
				t.Fatalf("LogLevel = %q, want %q", cfg.LogLevel, tc.want)
			}
		})
	}
}

// TestLoadToolFormatDefaultsAndValidation verifies the XML default and accepted tool modes.
func TestLoadToolFormatDefaultsAndValidation(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "default xml",
			env:  map[string]string{envUpstreamAPIKey: "key", envDefaultModel: "model"},
			want: ToolFormatXML,
		},
		{
			name: "explicit xml",
			env:  map[string]string{envUpstreamAPIKey: "key", envDefaultModel: "model", envToolFormat: "xml"},
			want: ToolFormatXML,
		},
		{
			name: "explicit native",
			env:  map[string]string{envUpstreamAPIKey: "key", envDefaultModel: "model", envToolFormat: "native"},
			want: ToolFormatNative,
		},
		{
			name: "case-insensitive native",
			env:  map[string]string{envUpstreamAPIKey: "key", envDefaultModel: "model", envToolFormat: "NATIVE"},
			want: ToolFormatNative,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clearConfigEnv(t)
			cfg, err := loadFromValues(t, tc.env)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.ToolFormat != tc.want {
				t.Fatalf("ToolFormat = %q, want %q", cfg.ToolFormat, tc.want)
			}
		})
	}
}

// TestLoadClaudeCodeModelAliasOverrides verifies Claude Code alias and display-name overrides.
func TestLoadClaudeCodeModelAliasOverrides(t *testing.T) {
	clearConfigEnv(t)
	cfg, err := loadFromValues(t, map[string]string{
		envUpstreamAPIKey:         "key",
		envDefaultModel:           "model",
		envDefaultOpusModel:       "claude-opus-custom",
		envDefaultOpusModelName:   `"Opus via Gateway"`,
		envDefaultSonnetModel:     "claude-sonnet-custom[1m]",
		envDefaultSonnetModelName: `"Sonnet via Gateway"`,
		envDefaultHaikuModel:      "claude-haiku-custom",
		envDefaultHaikuModelName:  `"Haiku via Gateway"`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ModelAliases["opus"] != "claude-opus-custom" || cfg.ModelAliases["best"] != "claude-opus-custom" {
		t.Fatalf("opus aliases = %#v", cfg.ModelAliases)
	}
	if cfg.ModelAliases["sonnet"] != "claude-sonnet-custom" {
		t.Fatalf("sonnet alias = %#v", cfg.ModelAliases)
	}
	if cfg.ModelAliases["haiku"] != "claude-haiku-custom" {
		t.Fatalf("haiku alias = %#v", cfg.ModelAliases)
	}
	if cfg.ModelDisplayNames["claude-sonnet-custom"] != "Sonnet via Gateway" {
		t.Fatalf("display names = %#v", cfg.ModelDisplayNames)
	}
}

// TestLoadValidation verifies required fields and malformed JSON validation.
func TestLoadValidation(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{
			name: "force model without default",
			env:  map[string]string{envUpstreamAPIKey: "key", envForceModel: "1"},
		},
		{
			name: "mapping mode without default or map",
			env:  map[string]string{envUpstreamAPIKey: "key", envForceModel: "0"},
		},
		{
			name: "bad model map",
			env: map[string]string{
				envUpstreamAPIKey: "key",
				envDefaultModel:   "model",
				envModelMap:       `{bad}`,
			},
		},
		{
			name: "bad tool format",
			env:  map[string]string{envUpstreamAPIKey: "key", envDefaultModel: "model", envToolFormat: "json"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clearConfigEnv(t)
			if _, err := loadFromValues(t, tc.env); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
