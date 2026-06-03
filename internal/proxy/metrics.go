package proxy

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/thomas-illiet/anthropic-proxy/internal/anthropic"
)

const (
	metricsResultSuccess = "success"
	metricsResultError   = "error"
)

type proxyMetrics struct {
	registry *prometheus.Registry

	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec
	httpInFlight *prometheus.GaugeVec

	messageRequests *prometheus.CounterVec
	messageTokens   *prometheus.CounterVec
}

type messageMetricLabels struct {
	requestedModel string
	upstreamModel  string
	mode           string
}

// newProxyMetrics initializes the private Prometheus registry and all proxy metric collectors.
func newProxyMetrics() *proxyMetrics {
	m := &proxyMetrics{
		registry: prometheus.NewRegistry(),
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "anthropic_proxy_http_requests_total",
			Help: "Total HTTP requests served by the proxy.",
		}, []string{"endpoint", "method", "status"}),
		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "anthropic_proxy_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"endpoint", "method", "status"}),
		httpInFlight: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "anthropic_proxy_http_in_flight",
			Help: "HTTP requests currently being handled by the proxy.",
		}, []string{"endpoint", "method"}),
		messageRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "anthropic_proxy_message_requests_total",
			Help: "Total Anthropic messages requests after successful request conversion.",
		}, []string{"requested_model", "upstream_model", "mode", "result"}),
		messageTokens: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "anthropic_proxy_message_tokens_total",
			Help: "Total Anthropic messages tokens reported for successful requests.",
		}, []string{"requested_model", "upstream_model", "mode", "type"}),
	}
	m.registry.MustRegister(
		m.httpRequests,
		m.httpDuration,
		m.httpInFlight,
		m.messageRequests,
		m.messageTokens,
	)
	return m
}

// instrumentHTTP wraps an HTTP handler with access logging and Prometheus HTTP metrics.
func (p *Proxy) instrumentHTTP(endpoint string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w}
		rw := http.ResponseWriter(rec)
		if _, ok := w.(http.Flusher); ok {
			rw = &flushStatusRecorder{statusRecorder: rec}
		}

		start := time.Now()
		method := r.Method
		if p.metrics != nil {
			p.metrics.httpInFlight.WithLabelValues(endpoint, method).Inc()
		}
		defer func() {
			duration := time.Since(start)
			statusCode := rec.statusCode()
			if p.metrics != nil {
				status := strconv.Itoa(statusCode)
				p.metrics.httpInFlight.WithLabelValues(endpoint, method).Dec()
				p.metrics.httpRequests.WithLabelValues(endpoint, method, status).Inc()
				p.metrics.httpDuration.WithLabelValues(endpoint, method, status).Observe(duration.Seconds())
			}
			if shouldLogAccess(r.URL.Path, endpoint) {
				p.logAccess(r, endpoint, method, statusCode, duration)
			}
		}()

		next.ServeHTTP(rw, r)
	})
}

func shouldLogAccess(path, endpoint string) bool {
	switch endpoint {
	case "/health", "/metrics":
		return false
	}
	switch path {
	case "/health", "/metric", "/metrics":
		return false
	default:
		return true
	}
}

// logAccess writes one structured access log record for a completed HTTP request.
func (p *Proxy) logAccess(r *http.Request, endpoint, method string, status int, duration time.Duration) {
	level := slog.LevelInfo
	if status >= 500 {
		level = slog.LevelError
	} else if status >= 400 {
		level = slog.LevelWarn
	}
	p.logger.Log(level, "http request",
		"method", method,
		"path", r.URL.Path,
		"endpoint", endpoint,
		"status", status,
		"duration_ms", float64(duration.Microseconds())/1000,
		"client_ip", getClientIP(r),
	)
}

// handleMetrics serves the Prometheus scrape endpoint from the proxy's private registry.
func (p *Proxy) handleMetrics(w http.ResponseWriter, r *http.Request) {
	promhttp.HandlerFor(p.metrics.registry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
}

// recordMessage records message request counts and token usage for one converted request.
func (m *proxyMetrics) recordMessage(labels messageMetricLabels, result string, usage anthropic.Usage) {
	if m == nil {
		return
	}
	m.messageRequests.WithLabelValues(labels.requestedModel, labels.upstreamModel, labels.mode, result).Inc()
	if result != metricsResultSuccess {
		return
	}
	m.addMessageTokens(labels, "input", usage.InputTokens)
	m.addMessageTokens(labels, "output", usage.OutputTokens)
	m.addMessageTokens(labels, "cache_read", usage.CacheReadInputTokens)
	m.addMessageTokens(labels, "cache_creation", usage.CacheCreationInputTokens)
}

// addMessageTokens adds a token counter sample when the value is positive.
func (m *proxyMetrics) addMessageTokens(labels messageMetricLabels, tokenType string, tokens int) {
	if tokens <= 0 {
		return
	}
	m.messageTokens.WithLabelValues(labels.requestedModel, labels.upstreamModel, labels.mode, tokenType).Add(float64(tokens))
}

// messageMode returns the metrics label for streaming versus synchronous message requests.
func messageMode(stream bool) string {
	if stream {
		return "stream"
	}
	return "sync"
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

// WriteHeader captures the status code before forwarding it to the wrapped response writer.
func (w *statusRecorder) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// Write records an implicit 200 status before writing a response body.
func (w *statusRecorder) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// statusCode returns the captured status code, defaulting to 200 when nothing was written.
func (w *statusRecorder) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

type flushStatusRecorder struct {
	*statusRecorder
}

// Flush marks the response as successful before flushing streaming data to the client.
func (w *flushStatusRecorder) Flush() {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.ResponseWriter.(http.Flusher).Flush()
}
