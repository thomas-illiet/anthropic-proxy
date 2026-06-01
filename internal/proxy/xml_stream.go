package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/thomas-illiet/anthropic-proxy/internal/anthropic"
	"github.com/thomas-illiet/anthropic-proxy/internal/convert"
)

var (
	xmlThinkBlockPattern = regexp.MustCompile(`(?is)<think>.*?</think>`)
	xmlToolCodePattern   = regexp.MustCompile(`(?is)<tool_code\s+name\s*=\s*"([^"]+)"\s*>(.*?)</\s*tool_code\s*>`)
	xmlNestedToolPattern = regexp.MustCompile(`(?is)<tool\s+name="[^"]*">\s*`)
	xmlCloseToolPattern  = regexp.MustCompile(`(?is)</tool>\s*`)
	xmlToolNameLine      = regexp.MustCompile(`(?m)^[A-Za-z_][A-Za-z0-9_]*\s*\n`)
)

type xmlStreamState struct {
	stream           *streamState
	buffer           string
	toolCallsEmitted int
}

// newXMLStreamState creates XML fallback parsing state for one streaming response.
func newXMLStreamState(stream *streamState) *xmlStreamState {
	return &xmlStreamState{stream: stream}
}

// processText appends model text to the XML buffer and emits complete parsed blocks.
func (s *xmlStreamState) processText(delta string) error {
	if delta == "" {
		return nil
	}
	s.buffer += delta
	return s.processBuffer()
}

// processBuffer extracts complete XML tool calls while preserving preceding user-visible text.
func (s *xmlStreamState) processBuffer() error {
	s.buffer = xmlThinkBlockPattern.ReplaceAllString(s.buffer, "")
	for {
		loc := xmlToolCodePattern.FindStringSubmatchIndex(s.buffer)
		if loc == nil {
			return nil
		}

		textBefore := strings.TrimSpace(s.buffer[:loc[0]])
		if textBefore != "" {
			if err := s.emitTextBlock(textBefore); err != nil {
				return err
			}
		}

		name := s.buffer[loc[2]:loc[3]]
		args := cleanXMLToolArgs(s.buffer[loc[4]:loc[5]])
		if err := s.emitToolUseBlock(name, args); err != nil {
			return err
		}

		s.buffer = s.buffer[loc[1]:]
	}
}

// flushRemainingContent emits any buffered non-tool text after the upstream stream ends.
func (s *xmlStreamState) flushRemainingContent() error {
	s.buffer = xmlThinkBlockPattern.ReplaceAllString(s.buffer, "")
	text := strings.TrimSpace(s.buffer)
	s.buffer = ""
	if text == "" {
		return nil
	}
	return s.emitTextBlock(text)
}

// emitTextBlock emits one complete Anthropic text content block.
func (s *xmlStreamState) emitTextBlock(text string) error {
	if err := s.stream.openTextBlock(); err != nil {
		return err
	}
	if err := s.stream.emitTextDelta(text); err != nil {
		return err
	}
	return s.stream.closeCurrentBlock()
}

// emitToolUseBlock emits one complete Anthropic tool_use block parsed from XML text.
func (s *xmlStreamState) emitToolUseBlock(name, args string) error {
	if err := s.stream.openToolBlock(-1-s.toolCallsEmitted, "", name); err != nil {
		return err
	}
	if strings.TrimSpace(args) == "" || !json.Valid([]byte(args)) {
		args = "{}"
	}
	if err := s.stream.emitToolArgsDelta(s.stream.currentBlockIndex, args); err != nil {
		return err
	}
	if err := s.stream.closeCurrentBlock(); err != nil {
		return err
	}
	s.toolCallsEmitted++
	return nil
}

// cleanXMLToolArgs removes adapter-style wrapper fragments and returns JSON tool arguments.
func cleanXMLToolArgs(args string) string {
	cleaned := xmlNestedToolPattern.ReplaceAllString(args, "")
	cleaned = xmlCloseToolPattern.ReplaceAllString(cleaned, "")
	cleaned = xmlToolNameLine.ReplaceAllString(cleaned, "")
	return strings.TrimSpace(cleaned)
}

// handleXMLStream forwards a converted XML-mode request upstream and translates
// streamed text XML tool calls into Anthropic SSE tool_use blocks.
func (p *Proxy) handleXMLStream(w http.ResponseWriter, ctx context.Context, areq *anthropic.Request, oreq *convert.OpenAIRequest, reqID uint64, inputTokens int, metricLabels messageMetricLabels) {
	result := metricsResultError
	usage := anthropic.Usage{}
	defer func() {
		p.metrics.recordMessage(metricLabels, result, usage)
	}()

	ctx, cancel := context.WithTimeout(ctx, p.cfg.RequestTimeout)
	defer cancel()

	resp, err := p.openUpstreamStream(ctx, oreq)
	if err != nil {
		p.errorf("[#%d] upstream xml stream request failed: %v", reqID, err)
		anthropic.WriteError(w, http.StatusBadGateway, "api_error", "upstream: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if p.writeUpstreamStreamError(w, resp, reqID, "upstream xml stream") {
		return
	}

	prepareSSEResponse(w)

	messageID := "msg_" + convert.RandomID()
	stream, err := newStreamState(w, messageID, areq.Model, inputTokens, requestToolNames(areq.Tools))
	if err != nil {
		p.errorf("[#%d] xml stream init: %v", reqID, err)
		return
	}
	xmlState := newXMLStreamState(stream)

	scanner := newStreamScanner(resp.Body)
	if ok := p.scanOpenAIStream(ctx, scanner, reqID, "xml", func(chunk convert.OpenAIStreamChunk) error {
		return p.handleXMLStreamChunk(stream, xmlState, chunk, reqID)
	}); !ok {
		return
	}

	finalUsage, ok := p.finishXMLStream(stream, xmlState, reqID)
	if !ok {
		return
	}
	usage = finalUsage
	result = metricsResultSuccess
	p.debugf("[#%d] xml stream ok stop=%s in=%d out=%d", reqID, stream.finishReason, stream.inputTokens, stream.outputTokens)
}

// handleXMLStreamChunk translates one streamed OpenAI text delta through the XML tool parser.
func (p *Proxy) handleXMLStreamChunk(stream *streamState, xmlState *xmlStreamState, chunk convert.OpenAIStreamChunk, reqID uint64) error {
	applyStreamUsage(stream, chunk.Usage)

	if !stream.messageStarted {
		if err := stream.emitMessageStart(); err != nil {
			p.errorf("[#%d] emit xml start: %v", reqID, err)
			return err
		}
	}

	for _, ch := range chunk.Choices {
		reasoning := firstNonEmpty(ch.Delta.ReasoningContent, ch.Delta.Reasoning, ch.Delta.Thinking)
		if reasoning != "" {
			if err := stream.openThinkingBlock(); err != nil {
				return err
			}
			if err := stream.emitThinkingDelta(reasoning); err != nil {
				return err
			}
		}

		// XML mode buffers text until a complete tool_code block can be parsed safely.
		if err := xmlState.processText(ch.Delta.Content); err != nil {
			return err
		}

		if ch.FinishReason != nil && *ch.FinishReason != "" {
			stream.finishReason = *ch.FinishReason
		}
	}
	return nil
}

// finishXMLStream flushes buffered XML text and emits the final Anthropic stream events.
func (p *Proxy) finishXMLStream(stream *streamState, xmlState *xmlStreamState, reqID uint64) (anthropic.Usage, bool) {
	if !stream.messageStarted {
		_ = stream.emitMessageStart()
	}
	if err := xmlState.flushRemainingContent(); err != nil {
		p.errorf("[#%d] flush xml stream: %v", reqID, err)
		return anthropic.Usage{}, false
	}
	if err := stream.finishWithToolCalls(xmlState.toolCallsEmitted > 0); err != nil {
		p.errorf("[#%d] finish xml stream: %v", reqID, err)
		return anthropic.Usage{}, false
	}
	return streamUsage(stream), true
}
