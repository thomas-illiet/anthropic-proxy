package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/thomas-illiet/anthropic-proxy/internal/anthropic"
	"github.com/thomas-illiet/anthropic-proxy/internal/config"
	"github.com/thomas-illiet/anthropic-proxy/internal/convert"
)

// testConfig returns a minimal proxy config suitable for HTTP handler tests.
func testConfig(upstreamURL string) *config.Config {
	return &config.Config{
		ListenAddr:     ":0",
		UpstreamURL:    upstreamURL,
		UpstreamKey:    "upstream-key",
		DefaultModel:   "upstream-model",
		ToolFormat:     config.ToolFormatXML,
		ForceModel:     true,
		RequestTimeout: time.Second,
		MaxRequestBody: defaultMaxRequestBodyBytes,
		ModelMap:       map[string]string{},
	}
}

// TestHealthAndCountTokens verifies the health endpoint and local token counting.
func TestHealthAndCountTokens(t *testing.T) {
	p := New(testConfig("http://127.0.0.1:1/v1/chat/completions"))
	server := httptest.NewServer(p.Routes())
	defer server.Close()

	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != "ok" {
		t.Fatalf("health = %d %q", resp.StatusCode, body)
	}

	resp, err = http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("legacy healthz status = %d", resp.StatusCode)
	}

	resp, err = http.Post(server.URL+"/v1/messages/count_tokens", "application/json", strings.NewReader(`{
		"model":"claude-sonnet",
		"messages":[{"role":"user","content":"hello world"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("count_tokens status = %d", resp.StatusCode)
	}
	var out map[string]int
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["input_tokens"] < 1 {
		t.Fatalf("count_tokens response = %#v", out)
	}
}

// TestAuth verifies optional proxy client authentication for protected endpoints.
func TestAuth(t *testing.T) {
	cfg := testConfig("http://127.0.0.1:1/v1/chat/completions")
	cfg.ExpectedClientKey = "secret"
	server := httptest.NewServer(New(cfg).Routes())
	defer server.Close()

	resp, err := http.Post(server.URL+"/v1/messages/count_tokens", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing auth status = %d", resp.StatusCode)
	}

	req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/messages/count_tokens", strings.NewReader(`{
		"model":"claude-sonnet",
		"messages":[{"role":"user","content":"hello"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("authorized status = %d", resp.StatusCode)
	}
}

// TestProtectedEndpointsRequireAuth verifies all protected API endpoints enforce the client key.
func TestProtectedEndpointsRequireAuth(t *testing.T) {
	upstreamHit := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHit = true
	}))
	defer upstream.Close()

	cfg := testConfig(upstream.URL)
	cfg.ExpectedClientKey = "secret"
	server := httptest.NewServer(New(cfg).Routes())
	defer server.Close()

	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "messages", method: http.MethodPost, path: "/v1/messages", body: `{}`},
		{name: "count tokens", method: http.MethodPost, path: "/v1/messages/count_tokens", body: `{}`},
		{name: "models", method: http.MethodGet, path: "/v1/models"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, server.URL+tc.path, strings.NewReader(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			if tc.body != "" {
				req.Header.Set("content-type", "application/json")
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("status = %d body = %s", resp.StatusCode, body)
			}
		})
	}
	if upstreamHit {
		t.Fatal("upstream should not be called for unauthorized requests")
	}

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/models", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("x-api-key", "secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("x-api-key status = %d body = %s", resp.StatusCode, body)
	}
}

// TestRootDiagnosticsOmitSecrets verifies public diagnostics never expose credentials.
func TestRootDiagnosticsOmitSecrets(t *testing.T) {
	cfg := testConfig("http://127.0.0.1:1/v1/chat/completions")
	cfg.UpstreamKey = "upstream-secret"
	cfg.ExpectedClientKey = "client-secret"
	server := httptest.NewServer(New(cfg).Routes())
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body = %s", resp.StatusCode, body)
	}
	if strings.Contains(string(body), cfg.UpstreamKey) || strings.Contains(string(body), cfg.ExpectedClientKey) {
		t.Fatalf("root diagnostics leaked secret: %s", body)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"upstream_key", "expected_client_key", "client_key"} {
		if _, ok := out[forbidden]; ok {
			t.Fatalf("root diagnostics exposed %q: %#v", forbidden, out)
		}
	}
}

// TestMetricsEndpointUnprotected verifies Prometheus metrics are scrapeable even when API auth is enabled.
func TestMetricsEndpointUnprotected(t *testing.T) {
	cfg := testConfig("http://127.0.0.1:1/v1/chat/completions")
	cfg.ExpectedClientKey = "secret"
	server := httptest.NewServer(New(cfg).Routes())
	defer server.Close()

	resp, err := http.Get(server.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("metrics status = %d body = %s", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/plain") {
		t.Fatalf("metrics content-type = %q", ct)
	}
	if !strings.Contains(string(body), "anthropic_proxy_http_in_flight") {
		t.Fatalf("metrics body missing proxy metric:\n%s", body)
	}
}

// TestCORSRemoved verifies the proxy no longer emits CORS headers or handles preflight specially.
func TestCORSRemoved(t *testing.T) {
	server := httptest.NewServer(New(testConfig("http://127.0.0.1:1/v1/chat/completions")).Routes())
	defer server.Close()

	req, err := http.NewRequest(http.MethodOptions, server.URL+"/v1/messages", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("allow origin = %q", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Methods"); got != "" {
		t.Fatalf("allow methods = %q", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Headers"); got != "" {
		t.Fatalf("allow headers = %q", got)
	}
}

// TestModelsEndpointListsAliasesAndMappedClaudeKeys verifies discoverable model catalog output.
func TestModelsEndpointListsAliasesAndMappedClaudeKeys(t *testing.T) {
	cfg := testConfig("http://127.0.0.1:1/v1/chat/completions")
	cfg.ModelMap = map[string]string{
		"claude-custom-20260601": "upstream-custom",
		"meta/upstream-model":    "not-discoverable",
	}
	cfg.ModelDisplayNames = map[string]string{
		"claude-custom-20260601": "Custom Claude",
	}
	server := httptest.NewServer(New(cfg).Routes())
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/models", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "unknown-beta-2026-01-01")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d body = %s", resp.StatusCode, body)
	}
	if got := resp.Header.Get("anthropic-version"); got != "" {
		t.Fatalf("response anthropic-version = %q", got)
	}
	if got := resp.Header.Get("X-Anthropic-Proxy-Ignored-Betas"); got != "" {
		t.Fatalf("ignored betas = %q", got)
	}

	var out anthropic.ModelList
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	models := map[string]anthropic.ModelInfo{}
	for _, model := range out.Data {
		models[model.ID] = model
	}
	for _, want := range []string{"claude-sonnet-4-6", "claude-opus-4-8", "claude-haiku-4-5", "claude-custom-20260601"} {
		if _, ok := models[want]; !ok {
			t.Fatalf("missing model %q in %#v", want, out.Data)
		}
	}
	if _, ok := models["meta/upstream-model"]; ok {
		t.Fatalf("upstream model should not be exposed: %#v", out.Data)
	}
	if models["claude-custom-20260601"].DisplayName != "Custom Claude" {
		t.Fatalf("custom display name = %#v", models["claude-custom-20260601"])
	}
	if out.FirstID != "claude-sonnet-4-6" || out.LastID == "" {
		t.Fatalf("pagination metadata = %#v", out)
	}
}

// TestModelsEndpointPagination verifies limit and cursor pagination for model discovery.
func TestModelsEndpointPagination(t *testing.T) {
	server := httptest.NewServer(New(testConfig("http://127.0.0.1:1/v1/chat/completions")).Routes())
	defer server.Close()

	resp, err := http.Get(server.URL + "/v1/models?limit=2")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d body = %s", resp.StatusCode, body)
	}
	var first anthropic.ModelList
	if err := json.NewDecoder(resp.Body).Decode(&first); err != nil {
		t.Fatal(err)
	}
	if len(first.Data) != 2 || !first.HasMore || first.LastID != "claude-opus-4-8" {
		t.Fatalf("first page = %#v", first)
	}

	resp, err = http.Get(server.URL + "/v1/models?after_id=" + first.LastID + "&limit=1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var second anthropic.ModelList
	if err := json.NewDecoder(resp.Body).Decode(&second); err != nil {
		t.Fatal(err)
	}
	if len(second.Data) != 1 || second.Data[0].ID != "claude-haiku-4-5" || second.HasMore {
		t.Fatalf("second page = %#v", second)
	}
}

// TestSyncProxy verifies sync message forwarding and Anthropic response conversion.
func TestSyncProxy(t *testing.T) {
	// The upstream handler asserts the forwarded OpenAI-compatible request.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer upstream-key" {
			t.Fatalf("Authorization = %q", got)
		}
		var req convert.OpenAIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Model != "upstream-model" {
			t.Fatalf("upstream model = %q", req.Model)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-1",
			"model":"upstream-model",
			"choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}
		}`))
	}))
	defer upstream.Close()

	server := httptest.NewServer(New(testConfig(upstream.URL)).Routes())
	defer server.Close()

	resp, err := http.Post(server.URL+"/v1/messages", "application/json", strings.NewReader(`{
		"model":"claude-sonnet",
		"max_tokens":32,
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d body = %s", resp.StatusCode, body)
	}

	var out struct {
		ID         string `json:"id"`
		Model      string `json:"model"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.ID != "msg_1" || out.Model != "claude-sonnet" || out.StopReason != "end_turn" {
		t.Fatalf("response metadata = %+v", out)
	}
	if out.Usage.InputTokens != 5 || out.Usage.OutputTokens != 2 {
		t.Fatalf("usage = %+v", out.Usage)
	}
	if len(out.Content) != 1 || out.Content[0].Text != "hello" {
		t.Fatalf("content = %+v", out.Content)
	}
}

// TestSyncProxyMetrics verifies successful sync messages record request and token KPIs.
func TestSyncProxyMetrics(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-1",
			"model":"upstream-model",
			"choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":7,"completion_tokens":2,"total_tokens":9,"prompt_tokens_details":{"cached_tokens":2}}
		}`))
	}))
	defer upstream.Close()

	server := httptest.NewServer(New(testConfig(upstream.URL)).Routes())
	defer server.Close()

	resp, err := http.Post(server.URL+"/v1/messages", "application/json", strings.NewReader(`{
		"model":"claude-sonnet",
		"max_tokens":32,
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d body = %s", resp.StatusCode, body)
	}
	_, _ = io.Copy(io.Discard, resp.Body)

	metrics := scrapeMetrics(t, server.URL)
	messageLabels := map[string]string{
		"requested_model": "claude-sonnet",
		"upstream_model":  "upstream-model",
		"mode":            "sync",
	}
	assertMetric(t, metrics, "anthropic_proxy_message_requests_total", withLabels(messageLabels, map[string]string{"result": "success"}), 1)
	assertMetric(t, metrics, "anthropic_proxy_message_tokens_total", withLabels(messageLabels, map[string]string{"type": "input"}), 5)
	assertMetric(t, metrics, "anthropic_proxy_message_tokens_total", withLabels(messageLabels, map[string]string{"type": "output"}), 2)
	assertMetric(t, metrics, "anthropic_proxy_message_tokens_total", withLabels(messageLabels, map[string]string{"type": "cache_read"}), 2)
	assertMetric(t, metrics, "anthropic_proxy_http_requests_total", map[string]string{
		"endpoint": "/v1/messages",
		"method":   "POST",
		"status":   "200",
	}, 1)
}

// TestAnthropicHeadersAcceptedAndNotForwarded verifies Claude Adapter-style header handling.
func TestAnthropicHeadersAcceptedAndNotForwarded(t *testing.T) {
	var gotVersion, gotBeta string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVersion = r.Header.Get("anthropic-version")
		gotBeta = r.Header.Get("anthropic-beta")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-1",
			"model":"upstream-model",
			"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":5,"completion_tokens":1,"total_tokens":6}
		}`))
	}))
	defer upstream.Close()

	server := httptest.NewServer(New(testConfig(upstream.URL)).Routes())
	defer server.Close()

	req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/messages", strings.NewReader(`{
		"model":"claude-sonnet",
		"max_tokens":32,
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("anthropic-version", "2026-01-01")
	req.Header.Add("anthropic-beta", "unknown-beta-2026-01-01")
	req.Header.Add("anthropic-beta", "prompt-caching-2024-07-31,unknown-beta-2026-01-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d body = %s", resp.StatusCode, body)
	}
	if gotVersion != "" {
		t.Fatalf("forwarded version = %q", gotVersion)
	}
	if gotBeta != "" {
		t.Fatalf("forwarded beta = %q", gotBeta)
	}
	if got := resp.Header.Get("anthropic-version"); got != "" {
		t.Fatalf("response anthropic-version = %q", got)
	}
	if got := resp.Header.Get("X-Anthropic-Proxy-Ignored-Betas"); got != "" {
		t.Fatalf("ignored betas = %q", got)
	}
}

// TestUnsupportedAnthropicVersionAccepted verifies version headers are permissive.
func TestUnsupportedAnthropicVersionAccepted(t *testing.T) {
	upstreamHit := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHit = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-1",
			"model":"upstream-model",
			"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":5,"completion_tokens":1,"total_tokens":6}
		}`))
	}))
	defer upstream.Close()

	server := httptest.NewServer(New(testConfig(upstream.URL)).Routes())
	defer server.Close()

	req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/messages", strings.NewReader(`{
		"model":"claude-sonnet",
		"max_tokens":32,
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("anthropic-version", "2024-01-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d body = %s", resp.StatusCode, body)
	}
	if !upstreamHit {
		t.Fatal("upstream should be called for unsupported anthropic-version")
	}
}

// TestStreamProxy verifies OpenAI-compatible SSE is translated to Anthropic SSE.
func TestStreamProxy(t *testing.T) {
	// The upstream handler emits a minimal OpenAI-compatible SSE stream.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if accept := r.Header.Get("Accept"); accept != "text/event-stream" {
			t.Fatalf("Accept = %q", accept)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		fmt.Fprint(w, `data: {"choices":[{"index":0,"delta":{"content":"hel"}}]}`+"\n\n")
		flusher.Flush()
		fmt.Fprint(w, `data: {"choices":[{"index":0,"delta":{"content":"lo"},"finish_reason":"stop"}],"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	server := httptest.NewServer(New(testConfig(upstream.URL)).Routes())
	defer server.Close()

	resp, err := http.Post(server.URL+"/v1/messages", "application/json", strings.NewReader(`{
		"model":"claude-sonnet",
		"max_tokens":32,
		"stream":true,
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body = %s", resp.StatusCode, body)
	}
	got := string(body)
	for _, want := range []string{"event: message_start", "event: content_block_delta", `"text":"hello"`, "event: message_stop"} {
		if !strings.Contains(got, want) {
			t.Fatalf("stream response missing %q:\n%s", want, got)
		}
	}
}

// TestStreamProxySkipsMalformedSSE verifies one bad upstream chunk does not poison the stream.
func TestStreamProxySkipsMalformedSSE(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {bad json\n\n")
		writeSSEChunk(t, w, map[string]any{"choices": []any{map[string]any{
			"index":         0,
			"delta":         map[string]any{"content": "ok"},
			"finish_reason": "stop",
		}}, "usage": map[string]any{"prompt_tokens": 4, "completion_tokens": 1, "total_tokens": 5}})
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	server := httptest.NewServer(New(testConfig(upstream.URL)).Routes())
	defer server.Close()

	resp, err := http.Post(server.URL+"/v1/messages", "application/json", strings.NewReader(`{
		"model":"claude-sonnet",
		"max_tokens":32,
		"stream":true,
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body = %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), `"text":"ok"`) || !strings.Contains(string(body), "event: message_stop") {
		t.Fatalf("stream did not recover after malformed chunk:\n%s", body)
	}
}

// TestNativeStreamEmitsThinking verifies reasoning deltas become Anthropic thinking blocks.
func TestNativeStreamEmitsThinking(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSEChunk(t, w, map[string]any{"choices": []any{map[string]any{
			"index": 0,
			"delta": map[string]any{"reasoning_content": "think"},
		}}})
		writeSSEChunk(t, w, map[string]any{"choices": []any{map[string]any{
			"index":         0,
			"delta":         map[string]any{"content": "ok"},
			"finish_reason": "stop",
		}}, "usage": map[string]any{"prompt_tokens": 4, "completion_tokens": 1, "total_tokens": 5}})
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	cfg := testConfig(upstream.URL)
	cfg.ToolFormat = config.ToolFormatNative
	server := httptest.NewServer(New(cfg).Routes())
	defer server.Close()

	resp, err := http.Post(server.URL+"/v1/messages", "application/json", strings.NewReader(`{
		"model":"claude-sonnet",
		"max_tokens":32,
		"stream":true,
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body = %s", resp.StatusCode, body)
	}
	got := string(body)
	for _, want := range []string{`"type":"thinking"`, `"type":"thinking_delta"`, `"thinking":"think"`, `"text":"ok"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("native stream missing %q:\n%s", want, got)
		}
	}
}

// TestStreamProxyMetrics verifies successful streaming messages record usage from final SSE chunks.
func TestStreamProxyMetrics(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		fmt.Fprint(w, `data: {"choices":[{"index":0,"delta":{"content":"hel"}}]}`+"\n\n")
		flusher.Flush()
		fmt.Fprint(w, `data: {"choices":[{"index":0,"delta":{"content":"lo"},"finish_reason":"stop"}],"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	server := httptest.NewServer(New(testConfig(upstream.URL)).Routes())
	defer server.Close()

	resp, err := http.Post(server.URL+"/v1/messages", "application/json", strings.NewReader(`{
		"model":"claude-sonnet",
		"max_tokens":32,
		"stream":true,
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body = %s", resp.StatusCode, body)
	}

	metrics := scrapeMetrics(t, server.URL)
	messageLabels := map[string]string{
		"requested_model": "claude-sonnet",
		"upstream_model":  "upstream-model",
		"mode":            "stream",
	}
	assertMetric(t, metrics, "anthropic_proxy_message_requests_total", withLabels(messageLabels, map[string]string{"result": "success"}), 1)
	assertMetric(t, metrics, "anthropic_proxy_message_tokens_total", withLabels(messageLabels, map[string]string{"type": "input"}), 4)
	assertMetric(t, metrics, "anthropic_proxy_message_tokens_total", withLabels(messageLabels, map[string]string{"type": "output"}), 2)
}

// TestStreamProxyBuffersToolCallsUntilNameAndArgsComplete verifies robust streamed tool calls.
func TestStreamProxyBuffersToolCallsUntilNameAndArgsComplete(t *testing.T) {
	// The upstream handler splits tool name and argument data across stream chunks.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		fmt.Fprint(w, `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"q\""}}]}}]}`+"\n\n")
		flusher.Flush()
		fmt.Fprint(w, `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"lookup","arguments":":\"go\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":8,"completion_tokens":3,"total_tokens":11}}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	cfg := testConfig(upstream.URL)
	cfg.ToolFormat = config.ToolFormatNative
	server := httptest.NewServer(New(cfg).Routes())
	defer server.Close()

	resp, err := http.Post(server.URL+"/v1/messages", "application/json", strings.NewReader(`{
		"model":"claude-sonnet",
		"max_tokens":32,
		"stream":true,
		"tools":[{"name":"lookup","input_schema":{"type":"object","properties":{"q":{"type":"string"}}}}],
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body = %s", resp.StatusCode, body)
	}
	got := string(body)
	for _, want := range []string{
		`"type":"tool_use"`,
		`"id":"call_1"`,
		`"name":"lookup"`,
		`"partial_json":"{\"q\":\"go\"}"`,
		`"stop_reason":"tool_use"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stream response missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, `"name":""`) {
		t.Fatalf("stream emitted empty tool name:\n%s", got)
	}
}

// TestStreamProxyRepairsDuplicateAndMissingToolIDs verifies native streamed tool IDs remain unique.
func TestStreamProxyRepairsDuplicateAndMissingToolIDs(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSEChunk(t, w, map[string]any{"choices": []any{map[string]any{
			"index": 0,
			"delta": map[string]any{"tool_calls": []any{
				map[string]any{"index": 0, "id": "call_dup", "function": map[string]any{"name": "first", "arguments": `{"x":1}`}},
				map[string]any{"index": 1, "id": "call_dup", "function": map[string]any{"name": "second", "arguments": `{"x":2}`}},
				map[string]any{"index": 2, "function": map[string]any{"arguments": `{"x":3}`}},
			}},
			"finish_reason": "tool_calls",
		}}, "usage": map[string]any{"prompt_tokens": 8, "completion_tokens": 3, "total_tokens": 11}})
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	cfg := testConfig(upstream.URL)
	cfg.ToolFormat = config.ToolFormatNative
	server := httptest.NewServer(New(cfg).Routes())
	defer server.Close()

	resp, err := http.Post(server.URL+"/v1/messages", "application/json", strings.NewReader(`{
		"model":"claude-sonnet",
		"max_tokens":32,
		"stream":true,
		"tools":[
			{"name":"first","input_schema":{"type":"object"}},
			{"name":"second","input_schema":{"type":"object"}},
			{"name":"third","input_schema":{"type":"object"}}
		],
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body = %s", resp.StatusCode, body)
	}
	got := string(body)
	if strings.Count(got, `"id":"call_dup"`) != 1 {
		t.Fatalf("duplicate upstream ID was not repaired:\n%s", got)
	}
	for _, want := range []string{`"name":"first"`, `"name":"second"`, `"name":"third"`, `"id":"toolu_`} {
		if !strings.Contains(got, want) {
			t.Fatalf("stream response missing %q:\n%s", want, got)
		}
	}
}

// TestXMLStreamProxyParsesToolCalls verifies XML fallback stream parsing.
func TestXMLStreamProxyParsesToolCalls(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req convert.OpenAIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if len(req.Tools) != 0 || len(req.ToolChoice) != 0 {
			t.Fatalf("XML mode forwarded native tools: %+v choice=%s", req.Tools, req.ToolChoice)
		}
		if req.Temperature == nil || *req.Temperature != 0 {
			t.Fatalf("XML mode temperature = %v", req.Temperature)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSEChunk(t, w, map[string]any{"choices": []any{map[string]any{
			"index": 0,
			"delta": map[string]any{"content": "<think>secret</think>Need data. <tool_code name=\"lookup\">\n{\"q\""},
		}}})
		writeSSEChunk(t, w, map[string]any{"choices": []any{map[string]any{
			"index":         0,
			"delta":         map[string]any{"content": ":\"go\"}\n</tool_code>"},
			"finish_reason": "stop",
		}}, "usage": map[string]any{"prompt_tokens": 8, "completion_tokens": 3, "total_tokens": 11}})
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	server := httptest.NewServer(New(testConfig(upstream.URL)).Routes())
	defer server.Close()

	resp, err := http.Post(server.URL+"/v1/messages", "application/json", strings.NewReader(`{
		"model":"claude-sonnet",
		"max_tokens":32,
		"stream":true,
		"tools":[{"name":"lookup","input_schema":{"type":"object","properties":{"q":{"type":"string"}}}}],
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body = %s", resp.StatusCode, body)
	}
	got := string(body)
	for _, want := range []string{
		`"text":"Need data."`,
		`"type":"tool_use"`,
		`"name":"lookup"`,
		`"partial_json":"{\"q\":\"go\"}"`,
		`"stop_reason":"tool_use"`,
		"event: message_stop",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("XML stream response missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "secret") || strings.Contains(got, "tool_code") {
		t.Fatalf("XML stream leaked internal text:\n%s", got)
	}
}

// TestXMLStreamInvalidToolArgsFallBackToEmptyObject verifies malformed XML tool JSON is safe.
func TestXMLStreamInvalidToolArgsFallBackToEmptyObject(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSEChunk(t, w, map[string]any{"choices": []any{map[string]any{
			"index":         0,
			"delta":         map[string]any{"content": `<tool_code name="lookup">not json</tool_code>`},
			"finish_reason": "stop",
		}}, "usage": map[string]any{"prompt_tokens": 8, "completion_tokens": 3, "total_tokens": 11}})
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	server := httptest.NewServer(New(testConfig(upstream.URL)).Routes())
	defer server.Close()

	resp, err := http.Post(server.URL+"/v1/messages", "application/json", strings.NewReader(`{
		"model":"claude-sonnet",
		"max_tokens":32,
		"stream":true,
		"tools":[{"name":"lookup","input_schema":{"type":"object"}}],
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body = %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), `"partial_json":"{}"`) || !strings.Contains(string(body), `"stop_reason":"tool_use"`) {
		t.Fatalf("invalid XML args were not sanitized:\n%s", body)
	}
}

// TestBodyLimit verifies oversized requests are rejected before hitting upstream.
func TestBodyLimit(t *testing.T) {
	upstreamHit := false
	// The upstream handler should remain untouched for oversized bodies.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHit = true
	}))
	defer upstream.Close()

	cfg := testConfig(upstream.URL)
	cfg.MaxRequestBody = 128
	server := httptest.NewServer(New(cfg).Routes())
	defer server.Close()

	resp, err := http.Post(server.URL+"/v1/messages", "application/json", strings.NewReader(strings.Repeat("x", 129)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d body = %s", resp.StatusCode, body)
	}
	if upstreamHit {
		t.Fatal("upstream should not be called for oversized body")
	}
}

// TestCountTokensBodyLimit verifies token-count requests share the configured body limit.
func TestCountTokensBodyLimit(t *testing.T) {
	cfg := testConfig("http://127.0.0.1:1/v1/chat/completions")
	cfg.MaxRequestBody = 16
	server := httptest.NewServer(New(cfg).Routes())
	defer server.Close()

	resp, err := http.Post(server.URL+"/v1/messages/count_tokens", "application/json", strings.NewReader(strings.Repeat("x", 17)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d body = %s", resp.StatusCode, body)
	}
}

// TestUpstreamError verifies upstream errors are mapped into Anthropic-style errors.
func TestUpstreamError(t *testing.T) {
	// The upstream handler returns an OpenAI-style error body.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"slow down"}}`, http.StatusTooManyRequests)
	}))
	defer upstream.Close()

	server := httptest.NewServer(New(testConfig(upstream.URL)).Routes())
	defer server.Close()

	resp, err := http.Post(server.URL+"/v1/messages", "application/json", strings.NewReader(`{
		"model":"claude-sonnet",
		"max_tokens":32,
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "rate_limit_error") || !strings.Contains(string(body), "slow down") {
		t.Fatalf("unexpected body = %s", body)
	}
}

// TestUpstreamInvalidJSON verifies bad successful upstream bodies become gateway errors.
func TestUpstreamInvalidJSON(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{bad json`))
	}))
	defer upstream.Close()

	server := httptest.NewServer(New(testConfig(upstream.URL)).Routes())
	defer server.Close()

	resp, err := http.Post(server.URL+"/v1/messages", "application/json", strings.NewReader(`{
		"model":"claude-sonnet",
		"max_tokens":32,
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d body = %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "decode:") || !strings.Contains(string(body), "api_error") {
		t.Fatalf("unexpected body = %s", body)
	}
}

// TestStreamUpstreamError verifies streaming upstream HTTP failures are mapped before SSE starts.
func TestStreamUpstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"upstream down"}}`, http.StatusServiceUnavailable)
	}))
	defer upstream.Close()

	server := httptest.NewServer(New(testConfig(upstream.URL)).Routes())
	defer server.Close()

	resp, err := http.Post(server.URL+"/v1/messages", "application/json", strings.NewReader(`{
		"model":"claude-sonnet",
		"max_tokens":32,
		"stream":true,
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body = %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "api_error") || !strings.Contains(string(body), "upstream down") {
		t.Fatalf("unexpected body = %s", body)
	}
}

// scrapeMetrics returns the raw Prometheus metrics body from a test server.
func scrapeMetrics(t *testing.T, serverURL string) string {
	t.Helper()
	resp, err := http.Get(serverURL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("metrics status = %d body = %s", resp.StatusCode, body)
	}
	return string(body)
}

// writeSSEChunk writes one OpenAI-compatible SSE data chunk to a test response.
func writeSSEChunk(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(w, "data: %s\n\n", b)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

// assertMetric verifies that one metric sample with the requested labels has the expected value.
func assertMetric(t *testing.T, metrics, name string, labels map[string]string, want float64) {
	t.Helper()
	got, ok := metricValue(metrics, name, labels)
	if !ok {
		t.Fatalf("metric %s with labels %#v not found in:\n%s", name, labels, metrics)
	}
	if got != want {
		t.Fatalf("metric %s with labels %#v = %v, want %v", name, labels, got, want)
	}
}

// metricValue extracts a sample value from a Prometheus text exposition body.
func metricValue(metrics, name string, labels map[string]string) (float64, bool) {
	for _, line := range strings.Split(metrics, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		metricName := line
		rest := ""
		if idx := strings.IndexAny(line, "{ "); idx >= 0 {
			metricName = line[:idx]
			rest = strings.TrimSpace(line[idx:])
		}
		if metricName != name {
			continue
		}
		if len(labels) > 0 {
			if !strings.HasPrefix(rest, "{") {
				continue
			}
			end := strings.Index(rest, "}")
			if end < 0 {
				continue
			}
			labelSet := rest[1:end]
			matches := true
			for k, v := range labels {
				if !strings.Contains(labelSet, fmt.Sprintf(`%s="%s"`, k, v)) {
					matches = false
					break
				}
			}
			if !matches {
				continue
			}
			rest = strings.TrimSpace(rest[end+1:])
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		value, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			continue
		}
		return value, true
	}
	return 0, false
}

// withLabels returns a merged copy of base and extra metric label maps.
func withLabels(base, extra map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}
