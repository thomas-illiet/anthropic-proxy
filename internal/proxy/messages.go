package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/thomas-illiet/anthropic-proxy/internal/anthropic"
	"github.com/thomas-illiet/anthropic-proxy/internal/config"
	"github.com/thomas-illiet/anthropic-proxy/internal/convert"
)

// handleMessages processes Anthropic Messages requests in sync or streaming mode.
func (p *Proxy) handleMessages(w http.ResponseWriter, r *http.Request) {
	reqID := p.reqNum.Add(1)

	clientIP := getClientIP(r)
	if !p.checkAuth(r) {
		p.warnf("[#%d] authentication failed client=%s", reqID, clientIP)
		anthropic.WriteError(w, http.StatusUnauthorized, "authentication_error", "invalid x-api-key")
		return
	}

	defer r.Body.Close()
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, p.maxRequestBodyBytes()))
	if err != nil {
		p.warnf("[#%d] request body too large client=%s limit=%d", reqID, clientIP, p.maxRequestBodyBytes())
		anthropic.WriteError(w, http.StatusRequestEntityTooLarge, "invalid_request_error", "request body too large")
		return
	}

	var areq anthropic.Request
	if err := json.Unmarshal(body, &areq); err != nil {
		p.warnf("[#%d] invalid request json: %v", reqID, err)
		anthropic.WriteError(w, http.StatusBadRequest, "invalid_request_error", "json: "+err.Error())
		return
	}
	if areq.Model == "" || areq.MaxTokens == 0 {
		p.warnf("[#%d] invalid request: missing model or max_tokens", reqID)
		anthropic.WriteError(w, http.StatusBadRequest, "invalid_request_error", "model and max_tokens are required")
		return
	}

	oreq, err := convert.ToOpenAI(&areq, p.cfg)
	if err != nil {
		p.warnf("[#%d] request conversion failed: %v", reqID, err)
		anthropic.WriteError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}

	p.debugf("[#%d] %s requested_model=%s upstream_model=%s stream=%v tools=%d msgs=%d",
		reqID, r.URL.Path, areq.Model, oreq.Model, areq.Stream, len(areq.Tools), len(areq.Messages))

	inputTokens := convert.CountOpenAITokens(oreq)
	metricLabels := messageMetricLabels{
		requestedModel: areq.Model,
		upstreamModel:  oreq.Model,
		mode:           messageMode(areq.Stream),
	}
	if areq.Stream {
		if effectiveToolFormat(p.cfg) == config.ToolFormatXML {
			p.handleXMLStream(w, r.Context(), &areq, oreq, reqID, inputTokens, metricLabels)
			return
		}
		p.handleStream(w, r.Context(), &areq, oreq, reqID, inputTokens, metricLabels)
		return
	}
	p.handleSync(w, r.Context(), &areq, oreq, reqID, inputTokens, metricLabels)
}

// handleSync forwards a converted request upstream and writes one Anthropic response.
func (p *Proxy) handleSync(w http.ResponseWriter, ctx context.Context, areq *anthropic.Request, oreq *convert.OpenAIRequest, reqID uint64, inputTokens int, metricLabels messageMetricLabels) {
	result := metricsResultError
	usage := anthropic.Usage{}
	defer func() {
		p.metrics.recordMessage(metricLabels, result, usage)
	}()

	ctx, cancel := context.WithTimeout(ctx, p.cfg.RequestTimeout)
	defer cancel()

	reqBytes, _ := json.Marshal(oreq)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.cfg.UpstreamURL, bytes.NewReader(reqBytes))
	if err != nil {
		p.errorf("[#%d] create upstream request: %v", reqID, err)
		anthropic.WriteError(w, http.StatusInternalServerError, "api_error", err.Error())
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.cfg.UpstreamKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		p.errorf("[#%d] upstream request failed: %v", reqID, err)
		anthropic.WriteError(w, http.StatusBadGateway, "api_error", "upstream: "+err.Error())
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		p.warnf("[#%d] upstream returned status=%d", reqID, resp.StatusCode)
		p.debugf("[#%d] upstream %d: %s", reqID, resp.StatusCode, truncate(string(respBody), 500))
		anthropic.WriteError(w, resp.StatusCode, anthropic.MapErrorType(resp.StatusCode), anthropic.ExtractUpstreamError(respBody))
		return
	}

	var ores convert.OpenAIResponse
	if err := json.Unmarshal(respBody, &ores); err != nil {
		p.errorf("[#%d] decode upstream response: %v", reqID, err)
		anthropic.WriteError(w, http.StatusBadGateway, "api_error", "decode: "+err.Error())
		return
	}
	ares := convert.FromOpenAI(&ores, areq.Model)
	if ares.Usage.InputTokens == 0 && ares.Usage.CacheReadInputTokens == 0 && inputTokens > 0 {
		ares.Usage.InputTokens = inputTokens
	}
	usage = ares.Usage
	result = metricsResultSuccess

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ares)
	p.debugf("[#%d] sync ok stop=%s in=%d out=%d", reqID, ares.StopReason, ares.Usage.InputTokens, ares.Usage.OutputTokens)
}
