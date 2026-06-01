package proxy

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/thomas-illiet/anthropic-proxy/internal/config"
	"github.com/thomas-illiet/anthropic-proxy/internal/logging"
)

const defaultMaxRequestBodyBytes int64 = 32 << 20

type Proxy struct {
	cfg     *config.Config
	client  *http.Client
	reqNum  atomic.Uint64
	metrics *proxyMetrics
	logger  *logging.Logger
}

// New creates a Proxy with the default HTTP transport configured.
func New(cfg *config.Config) *Proxy {
	return NewWithLogger(cfg, logging.NewStderr(configLogLevel(cfg)))
}

// NewWithLogger creates a Proxy with an explicit logger.
func NewWithLogger(cfg *config.Config, logger *logging.Logger) *Proxy {
	if logger == nil {
		logger = logging.NewDiscard()
	}
	return &Proxy{
		cfg:     cfg,
		metrics: newProxyMetrics(),
		logger:  logger,
		client: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// maxRequestBodyBytes returns the configured request body limit or the default.
func (p *Proxy) maxRequestBodyBytes() int64 {
	if p.cfg.MaxRequestBody > 0 {
		return p.cfg.MaxRequestBody
	}
	return defaultMaxRequestBodyBytes
}

// Routes registers all HTTP routes served by the proxy.
func (p *Proxy) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("POST /v1/messages", p.instrumentHTTP("/v1/messages", http.HandlerFunc(p.handleMessages)))
	mux.Handle("POST /v1/messages/count_tokens", p.instrumentHTTP("/v1/messages/count_tokens", http.HandlerFunc(p.handleCountTokens)))
	mux.Handle("GET /v1/models", p.instrumentHTTP("/v1/models", http.HandlerFunc(p.handleModels)))
	mux.Handle("GET /metrics", p.instrumentHTTP("/metrics", http.HandlerFunc(p.handleMetrics)))
	// The health handler is intentionally tiny so container healthchecks stay cheap.
	mux.Handle("GET /health", p.instrumentHTTP("/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})))
	// The root handler exposes non-secret runtime settings for local diagnostics.
	mux.Handle("GET /{$}", p.instrumentHTTP("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"service":                "anthropic-proxy",
			"upstream":               p.cfg.UpstreamURL,
			"default_model":          p.cfg.DefaultModel,
			"tool_format":            p.cfg.ToolFormat,
			"force_model":            p.cfg.ForceModel,
			"models":                 p.cfg.ModelMap,
			"model_aliases":          effectiveModelAliases(p.cfg),
			"request_timeout_sec":    int(p.cfg.RequestTimeout / time.Second),
			"max_request_body_bytes": p.maxRequestBodyBytes(),
			"forward_cache_control":  p.cfg.ForwardCacheControl,
			"log_level":              p.cfg.LogLevel,
		})
	})))
	mux.Handle("/", p.instrumentHTTP("/", http.NotFoundHandler()))
	return mux
}
