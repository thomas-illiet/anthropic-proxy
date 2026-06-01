package proxy

import (
	"context"
	"net/http"

	"github.com/thomas-illiet/anthropic-proxy/internal/anthropic"
	"github.com/thomas-illiet/anthropic-proxy/internal/convert"
)

// handleStream forwards a converted request upstream and translates OpenAI SSE into Anthropic SSE.
func (p *Proxy) handleStream(w http.ResponseWriter, ctx context.Context, areq *anthropic.Request, oreq *convert.OpenAIRequest, reqID uint64, inputTokens int, metricLabels messageMetricLabels) {
	result := metricsResultError
	usage := anthropic.Usage{}
	defer func() {
		p.metrics.recordMessage(metricLabels, result, usage)
	}()

	ctx, cancel := context.WithTimeout(ctx, p.cfg.RequestTimeout)
	defer cancel()

	resp, err := p.openUpstreamStream(ctx, oreq)
	if err != nil {
		p.errorf("[#%d] upstream stream request failed: %v", reqID, err)
		anthropic.WriteError(w, http.StatusBadGateway, "api_error", "upstream: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if p.writeUpstreamStreamError(w, resp, reqID, "upstream stream") {
		return
	}

	prepareSSEResponse(w)

	messageID := "msg_" + convert.RandomID()
	state, err := newStreamState(w, messageID, areq.Model, inputTokens, requestToolNames(areq.Tools))
	if err != nil {
		p.errorf("[#%d] stream init: %v", reqID, err)
		return
	}

	scanner := newStreamScanner(resp.Body)
	if ok := p.scanOpenAIStream(ctx, scanner, reqID, "", func(chunk convert.OpenAIStreamChunk) error {
		return p.handleNativeStreamChunk(state, chunk, reqID)
	}); !ok {
		return
	}

	finalUsage, ok := p.finishNativeStream(state, reqID)
	if !ok {
		return
	}
	usage = finalUsage
	result = metricsResultSuccess
	p.debugf("[#%d] stream ok stop=%s in=%d out=%d", reqID, state.finishReason, state.inputTokens, state.outputTokens)
}

// handleNativeStreamChunk translates one OpenAI native-tool stream chunk into Anthropic SSE events.
func (p *Proxy) handleNativeStreamChunk(state *streamState, chunk convert.OpenAIStreamChunk, reqID uint64) error {
	applyStreamUsage(state, chunk.Usage)

	if !state.messageStarted {
		if err := state.emitMessageStart(); err != nil {
			p.errorf("[#%d] emit start: %v", reqID, err)
			return err
		}
	}

	for _, ch := range chunk.Choices {
		if err := emitNativeReasoningDelta(state, ch); err != nil {
			return err
		}
		if err := emitNativeTextDelta(state, ch); err != nil {
			return err
		}

		// Tool-call deltas can arrive split across many chunks; streamState accumulates them.
		for _, tc := range ch.Delta.ToolCalls {
			state.recordToolDelta(tc)
		}

		if ch.FinishReason != nil && *ch.FinishReason != "" {
			state.finishReason = *ch.FinishReason
		}
	}
	return nil
}

// emitNativeReasoningDelta opens the thinking block lazily before writing reasoning text.
func emitNativeReasoningDelta(state *streamState, ch convert.OpenAIStreamChoice) error {
	reasoning := firstNonEmpty(ch.Delta.ReasoningContent, ch.Delta.Reasoning, ch.Delta.Thinking)
	if reasoning == "" {
		return nil
	}
	if err := state.openThinkingBlock(); err != nil {
		return err
	}
	return state.emitThinkingDelta(reasoning)
}

// emitNativeTextDelta opens the text block lazily before writing content deltas.
func emitNativeTextDelta(state *streamState, ch convert.OpenAIStreamChoice) error {
	if ch.Delta.Content == "" {
		return nil
	}
	if err := state.openTextBlock(); err != nil {
		return err
	}
	return state.emitTextDelta(ch.Delta.Content)
}

// finishNativeStream emits the trailing Anthropic stream events and returns final usage.
func (p *Proxy) finishNativeStream(state *streamState, reqID uint64) (anthropic.Usage, bool) {
	if !state.messageStarted {
		_ = state.emitMessageStart()
	}
	if err := state.finish(); err != nil {
		p.errorf("[#%d] finish stream: %v", reqID, err)
		return anthropic.Usage{}, false
	}
	return streamUsage(state), true
}
