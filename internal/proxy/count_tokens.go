package proxy

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/thomas-illiet/anthropic-proxy/internal/anthropic"
	"github.com/thomas-illiet/anthropic-proxy/internal/convert"
)

// handleCountTokens returns a local token count for an Anthropic request after conversion.
func (p *Proxy) handleCountTokens(w http.ResponseWriter, r *http.Request) {
	if !p.checkAuth(r) {
		p.warnf("count_tokens authentication failed client=%s", getClientIP(r))
		anthropic.WriteError(w, http.StatusUnauthorized, "authentication_error", "invalid x-api-key")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, p.maxRequestBodyBytes()))
	if err != nil {
		p.warnf("count_tokens request body too large client=%s limit=%d", getClientIP(r), p.maxRequestBodyBytes())
		anthropic.WriteError(w, http.StatusRequestEntityTooLarge, "invalid_request_error", "request body too large")
		return
	}
	defer r.Body.Close()

	var areq anthropic.Request
	if err := json.Unmarshal(body, &areq); err != nil {
		p.warnf("count_tokens invalid request json: %v", err)
		anthropic.WriteError(w, http.StatusBadRequest, "invalid_request_error", "json: "+err.Error())
		return
	}
	if areq.Model == "" {
		p.warnf("count_tokens invalid request: missing model")
		anthropic.WriteError(w, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	oreq, err := convert.ToOpenAI(&areq, p.cfg)
	if err != nil {
		p.warnf("count_tokens conversion failed: %v", err)
		anthropic.WriteError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	n := convert.CountOpenAITokens(oreq)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"input_tokens": n})
}
