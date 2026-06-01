package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/thomas-illiet/anthropic-proxy/internal/anthropic"
	"github.com/thomas-illiet/anthropic-proxy/internal/convert"
)

// openUpstreamStream creates and sends the OpenAI-compatible SSE request.
func (p *Proxy) openUpstreamStream(ctx context.Context, oreq *convert.OpenAIRequest) (*http.Response, error) {
	reqBytes, _ := json.Marshal(oreq)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.cfg.UpstreamURL, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.cfg.UpstreamKey)
	httpReq.Header.Set("Accept", "text/event-stream")
	return p.client.Do(httpReq)
}

// writeUpstreamStreamError forwards upstream stream failures as Anthropic-style errors.
func (p *Proxy) writeUpstreamStreamError(w http.ResponseWriter, resp *http.Response, reqID uint64, label string) bool {
	if resp.StatusCode < 400 {
		return false
	}
	body, _ := io.ReadAll(resp.Body)
	p.warnf("[#%d] %s returned status=%d", reqID, label, resp.StatusCode)
	p.debugf("[#%d] %s %d: %s", reqID, label, resp.StatusCode, truncate(string(body), 500))
	anthropic.WriteError(w, resp.StatusCode, anthropic.MapErrorType(resp.StatusCode), anthropic.ExtractUpstreamError(body))
	return true
}

// prepareSSEResponse writes the fixed Anthropic stream response headers.
func prepareSSEResponse(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
}

// newStreamScanner configures Scanner for large SSE data lines.
func newStreamScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return scanner
}

// openAIStreamData extracts one SSE data payload and skips comments, blanks, and [DONE].
func openAIStreamData(line string) (string, bool) {
	if line == "" || !strings.HasPrefix(line, "data:") {
		return "", false
	}
	data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	return data, data != "" && data != "[DONE]"
}

// scanOpenAIStream decodes OpenAI SSE chunks and lets the caller translate each chunk.
func (p *Proxy) scanOpenAIStream(ctx context.Context, scanner *bufio.Scanner, reqID uint64, logPrefix string, handle func(convert.OpenAIStreamChunk) error) bool {
	parseLabel := "chunk"
	scannerLabel := "scanner"
	if logPrefix != "" {
		parseLabel = logPrefix + " chunk"
		scannerLabel = logPrefix + " scanner"
	}

	for scanner.Scan() {
		if ctx.Err() != nil {
			return false
		}
		data, ok := openAIStreamData(scanner.Text())
		if !ok {
			continue
		}

		var chunk convert.OpenAIStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			p.warnf("[#%d] %s parse: %v", reqID, parseLabel, err)
			p.tracef("[#%d] %s parse data: %s", reqID, parseLabel, truncate(data, 200))
			continue
		}
		if err := handle(chunk); err != nil {
			return false
		}
	}
	if err := scanner.Err(); err != nil {
		p.warnf("[#%d] %s: %v", reqID, scannerLabel, err)
	}
	return true
}

// applyStreamUsage updates fallback token counts when the upstream emits usage chunks.
func applyStreamUsage(state *streamState, usage *convert.OpenAIUsage) {
	if state == nil || usage == nil {
		return
	}
	mapped := convert.UsageFromOpenAI(*usage)
	if mapped.InputTokens > 0 || mapped.CacheReadInputTokens > 0 || mapped.CacheCreationInputTokens > 0 {
		state.inputTokens = mapped.InputTokens
		state.cacheReadTokens = mapped.CacheReadInputTokens
		state.cacheCreateTokens = mapped.CacheCreationInputTokens
	}
	if mapped.OutputTokens > 0 {
		state.outputTokens = mapped.OutputTokens
	}
}

// streamUsage returns the final Anthropic usage from stream state counters.
func streamUsage(state *streamState) anthropic.Usage {
	return anthropic.Usage{
		InputTokens:              state.inputTokens,
		OutputTokens:             state.outputTokens,
		CacheReadInputTokens:     state.cacheReadTokens,
		CacheCreationInputTokens: state.cacheCreateTokens,
	}
}
