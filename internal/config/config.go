package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/thomas-illiet/anthropic-proxy/internal/anthropic"
	"github.com/thomas-illiet/anthropic-proxy/internal/logging"
)

type Config struct {
	ListenAddr          string
	UpstreamURL         string
	UpstreamKey         string
	DefaultModel        string
	ToolFormat          string
	ModelMap            map[string]string
	ModelAliases        map[string]string
	ModelDisplayNames   map[string]string
	ForceModel          bool
	LogLevel            string
	ExpectedClientKey   string
	RequestTimeout      time.Duration
	MaxRequestBody      int64
	ForwardCacheControl bool
}

const (
	ToolFormatXML    = "xml"
	ToolFormatNative = "native"
)

const envPrefix = "ANTHROPIC_PROXY_"

const (
	envListenAddr             = envPrefix + "LISTEN_ADDR"
	envUpstreamURL            = envPrefix + "UPSTREAM_URL"
	envUpstreamAPIKey         = envPrefix + "UPSTREAM_API_KEY"
	envDefaultModel           = envPrefix + "DEFAULT_MODEL"
	envToolFormat             = envPrefix + "TOOL_FORMAT"
	envForceModel             = envPrefix + "FORCE_MODEL"
	envModelMap               = envPrefix + "MODEL_MAP"
	envDefaultOpusModel       = envPrefix + "DEFAULT_OPUS_MODEL"
	envDefaultOpusModelName   = envPrefix + "DEFAULT_OPUS_MODEL_NAME"
	envDefaultSonnetModel     = envPrefix + "DEFAULT_SONNET_MODEL"
	envDefaultSonnetModelName = envPrefix + "DEFAULT_SONNET_MODEL_NAME"
	envDefaultHaikuModel      = envPrefix + "DEFAULT_HAIKU_MODEL"
	envDefaultHaikuModelName  = envPrefix + "DEFAULT_HAIKU_MODEL_NAME"
	envClientKey              = envPrefix + "CLIENT_KEY"
	envRequestTimeoutSec      = envPrefix + "REQUEST_TIMEOUT_SEC"
	envMaxRequestBodyBytes    = envPrefix + "MAX_REQUEST_BODY_BYTES"
	envForwardCacheControl    = envPrefix + "FORWARD_CACHE_CONTROL"
	envLogLevel               = envPrefix + "LOG_LEVEL"
)

// Load reads the current directory .env file and environment variables into a validated Config.
func Load() (*Config, error) {
	v, err := newViper()
	if err != nil {
		return nil, err
	}
	return load(v)
}

// newViper creates an isolated Viper instance for one config load.
func newViper() (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigType("env")
	v.AutomaticEnv()

	setDefaults(v)

	if _, err := os.Stat(".env"); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return v, nil
		}
		return nil, err
	}
	v.SetConfigFile(".env")
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	return v, nil
}

// setDefaults registers every supported key and its default value.
func setDefaults(v *viper.Viper) {
	v.SetDefault(envListenAddr, ":8787")
	v.SetDefault(envUpstreamURL, "https://api.openai.com/v1/chat/completions")
	v.SetDefault(envToolFormat, ToolFormatXML)
	v.SetDefault(envForceModel, true)
	v.SetDefault(envRequestTimeoutSec, 600)
	v.SetDefault(envMaxRequestBodyBytes, int64(32<<20))
	v.SetDefault(envForwardCacheControl, false)
	v.SetDefault(envLogLevel, "")
	v.SetDefault(envDefaultOpusModel, anthropic.DefaultOpusModel)
	v.SetDefault(envDefaultSonnetModel, anthropic.DefaultSonnetModel)
	v.SetDefault(envDefaultHaikuModel, anthropic.DefaultHaikuModel)
}

// getBool parses a boolean config value with a default fallback.
func getBool(v *viper.Viper, k string, d bool) bool {
	if !v.IsSet(k) {
		return d
	}
	switch strings.ToLower(strings.TrimSpace(v.GetString(k))) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return d
	}
}

// getInt parses a positive integer config value with a default fallback.
func getInt(v *viper.Viper, k string, d int) int {
	if !v.IsSet(k) {
		return d
	}
	var n int
	if _, err := fmt.Sscanf(v.GetString(k), "%d", &n); err != nil || n <= 0 {
		return d
	}
	return n
}

// getPositiveInt64 parses a positive int64 config value with a default fallback.
func getPositiveInt64(v *viper.Viper, k string, d int64) int64 {
	if !v.IsSet(k) {
		return d
	}
	var n int64
	if _, err := fmt.Sscanf(v.GetString(k), "%d", &n); err != nil || n <= 0 {
		return d
	}
	return n
}

// normalizeToolFormat validates the configured tool conversion mode.
func normalizeToolFormat(raw string) (string, error) {
	format := strings.ToLower(strings.TrimSpace(raw))
	if format == "" {
		return ToolFormatXML, nil
	}
	switch format {
	case ToolFormatXML, ToolFormatNative:
		return format, nil
	default:
		return "", fmt.Errorf("%s must be %q or %q", envToolFormat, ToolFormatXML, ToolFormatNative)
	}
}

// load builds and validates Config from a configured Viper instance.
func load(v *viper.Viper) (*Config, error) {
	modelAliases, modelDisplayNames := loadModelAliases(v)
	toolFormat, err := normalizeToolFormat(v.GetString(envToolFormat))
	if err != nil {
		return nil, err
	}
	logLevel, err := logging.NormalizeLevel(v.GetString(envLogLevel))
	if err != nil {
		return nil, err
	}
	cfg := &Config{
		ListenAddr:          v.GetString(envListenAddr),
		UpstreamURL:         v.GetString(envUpstreamURL),
		UpstreamKey:         v.GetString(envUpstreamAPIKey),
		DefaultModel:        v.GetString(envDefaultModel),
		ToolFormat:          toolFormat,
		ModelAliases:        modelAliases,
		ModelDisplayNames:   modelDisplayNames,
		ForceModel:          getBool(v, envForceModel, true),
		LogLevel:            logLevel,
		ExpectedClientKey:   v.GetString(envClientKey),
		RequestTimeout:      time.Duration(getInt(v, envRequestTimeoutSec, 600)) * time.Second,
		MaxRequestBody:      getPositiveInt64(v, envMaxRequestBodyBytes, 32<<20),
		ForwardCacheControl: getBool(v, envForwardCacheControl, false),
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 10 * time.Minute
	}
	if cfg.UpstreamKey == "" {
		return nil, fmt.Errorf("%s is required", envUpstreamAPIKey)
	}

	if raw := v.GetString(envModelMap); raw != "" {
		m := map[string]string{}
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			return nil, fmt.Errorf("%s parse: %w", envModelMap, err)
		}
		cfg.ModelMap = m
	} else {
		cfg.ModelMap = map[string]string{}
	}

	if cfg.ForceModel && cfg.DefaultModel == "" {
		return nil, fmt.Errorf("%s is required when %s=1", envDefaultModel, envForceModel)
	}
	if !cfg.ForceModel && cfg.DefaultModel == "" && len(cfg.ModelMap) == 0 {
		return nil, fmt.Errorf("define %s or %s", envDefaultModel, envModelMap)
	}
	return cfg, nil
}

// loadModelAliases reads Claude Code model alias settings and display names.
func loadModelAliases(v *viper.Viper) (map[string]string, map[string]string) {
	opusModel := v.GetString(envDefaultOpusModel)
	sonnetModel := v.GetString(envDefaultSonnetModel)
	haikuModel := v.GetString(envDefaultHaikuModel)

	aliases := anthropic.Aliases(opusModel, sonnetModel, haikuModel)
	displayNames := map[string]string{}
	addDisplayName(v, displayNames, opusModel, envDefaultOpusModelName)
	addDisplayName(v, displayNames, sonnetModel, envDefaultSonnetModelName)
	addDisplayName(v, displayNames, haikuModel, envDefaultHaikuModelName)
	return aliases, displayNames
}

// addDisplayName stores a configured display name for a normalized model ID.
func addDisplayName(v *viper.Viper, displayNames map[string]string, model, key string) {
	if !v.IsSet(key) {
		return
	}
	name := strings.TrimSpace(v.GetString(key))
	model = anthropic.StripContextSuffix(model)
	if name == "" || model == "" {
		return
	}
	displayNames[model] = name
}
