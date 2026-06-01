package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/thomas-illiet/anthropic-proxy/internal/convert"
)

type streamState struct {
	w       http.ResponseWriter
	flusher http.Flusher

	messageID string
	model     string

	messageStarted bool

	currentBlockIndex int
	currentBlockType  string
	nextBlockIndex    int

	toolBlockByOAIIdx map[int]int
	toolCalls         map[int]*pendingToolCall
	toolOrder         []int
	fallbackToolNames []string
	usedToolIDs       map[string]bool

	inputTokens       int
	outputTokens      int
	cacheReadTokens   int
	cacheCreateTokens int
	finishReason      string

	stopped bool
}

// newStreamState initializes Anthropic SSE stream state for one upstream streaming response.
func newStreamState(w http.ResponseWriter, messageID, model string, inputTokens int, fallbackToolNames []string) (*streamState, error) {
	f, ok := w.(http.Flusher)
	if !ok {
		return nil, errors.New("streaming unsupported")
	}
	return &streamState{
		w:                 w,
		flusher:           f,
		messageID:         messageID,
		model:             model,
		toolBlockByOAIIdx: map[int]int{},
		toolCalls:         map[int]*pendingToolCall{},
		fallbackToolNames: fallbackToolNames,
		usedToolIDs:       map[string]bool{},
		inputTokens:       inputTokens,
		currentBlockType:  "",
	}, nil
}

// emit writes one Anthropic SSE event and flushes it to the client.
func (s *streamState) emit(event string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, b); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// emitMessageStart emits the Anthropic message_start event once.
func (s *streamState) emitMessageStart() error {
	if s.messageStarted {
		return nil
	}
	s.messageStarted = true
	return s.emit("message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":            s.messageID,
			"type":          "message",
			"role":          "assistant",
			"content":       []any{},
			"model":         s.model,
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage":         s.usageMap(true),
		},
	})
}

// usageMap builds an Anthropic usage object for message_start and message_delta events.
func (s *streamState) usageMap(includeInput bool) map[string]any {
	usage := map[string]any{"output_tokens": s.outputTokens}
	if includeInput {
		usage["input_tokens"] = s.inputTokens
	}
	if s.cacheCreateTokens > 0 {
		usage["cache_creation_input_tokens"] = s.cacheCreateTokens
	}
	if s.cacheReadTokens > 0 {
		usage["cache_read_input_tokens"] = s.cacheReadTokens
	}
	return usage
}

// closeCurrentBlock emits content_block_stop for the currently open block.
func (s *streamState) closeCurrentBlock() error {
	if s.currentBlockType == "" {
		return nil
	}
	idx := s.currentBlockIndex
	s.currentBlockType = ""
	return s.emit("content_block_stop", map[string]any{
		"type":  "content_block_stop",
		"index": idx,
	})
}

// openTextBlock opens a text content block, closing any prior block first.
func (s *streamState) openTextBlock() error {
	if s.currentBlockType == "text" {
		return nil
	}
	if err := s.closeCurrentBlock(); err != nil {
		return err
	}
	s.currentBlockIndex = s.nextBlockIndex
	s.nextBlockIndex++
	s.currentBlockType = "text"
	return s.emit("content_block_start", map[string]any{
		"type":  "content_block_start",
		"index": s.currentBlockIndex,
		"content_block": map[string]any{
			"type": "text",
			"text": "",
		},
	})
}

// openThinkingBlock opens a thinking content block, closing any prior block first.
func (s *streamState) openThinkingBlock() error {
	if s.currentBlockType == "thinking" {
		return nil
	}
	if err := s.closeCurrentBlock(); err != nil {
		return err
	}
	s.currentBlockIndex = s.nextBlockIndex
	s.nextBlockIndex++
	s.currentBlockType = "thinking"
	return s.emit("content_block_start", map[string]any{
		"type":  "content_block_start",
		"index": s.currentBlockIndex,
		"content_block": map[string]any{
			"type":     "thinking",
			"thinking": "",
		},
	})
}

// emitTextDelta emits a text_delta event for the current text block.
func (s *streamState) emitTextDelta(text string) error {
	return s.emit("content_block_delta", map[string]any{
		"type":  "content_block_delta",
		"index": s.currentBlockIndex,
		"delta": map[string]any{
			"type": "text_delta",
			"text": text,
		},
	})
}

// emitThinkingDelta emits a thinking_delta event for the current thinking block.
func (s *streamState) emitThinkingDelta(thinking string) error {
	return s.emit("content_block_delta", map[string]any{
		"type":  "content_block_delta",
		"index": s.currentBlockIndex,
		"delta": map[string]any{
			"type":     "thinking_delta",
			"thinking": thinking,
		},
	})
}

// finish closes open blocks and emits Anthropic message_delta and message_stop events.
func (s *streamState) finish() error {
	return s.finishWithToolCalls(len(s.toolOrder) > 0)
}

// finishWithToolCalls closes open blocks and emits the terminal Anthropic SSE events.
func (s *streamState) finishWithToolCalls(hasToolCalls bool) error {
	if s.stopped {
		return nil
	}
	s.stopped = true
	if err := s.flushToolCalls(); err != nil {
		return err
	}
	if err := s.closeCurrentBlock(); err != nil {
		return err
	}
	stopReason := convert.FinishReason(s.finishReason, hasToolCalls)
	if err := s.emit("message_delta", map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason":   stopReason,
			"stop_sequence": nil,
		},
		"usage": s.usageMap(false),
	}); err != nil {
		return err
	}
	return s.emit("message_stop", map[string]any{"type": "message_stop"})
}
