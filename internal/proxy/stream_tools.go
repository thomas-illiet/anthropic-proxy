package proxy

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/thomas-illiet/anthropic-proxy/internal/anthropic"
	"github.com/thomas-illiet/anthropic-proxy/internal/convert"
)

type pendingToolCall struct {
	id        string
	name      string
	arguments strings.Builder
}

// openToolBlock opens a tool_use content block for a buffered upstream tool call.
func (s *streamState) openToolBlock(oaiIdx int, id, name string) error {
	if err := s.closeCurrentBlock(); err != nil {
		return err
	}
	s.currentBlockIndex = s.nextBlockIndex
	s.nextBlockIndex++
	s.currentBlockType = "tool_use"
	s.toolBlockByOAIIdx[oaiIdx] = s.currentBlockIndex
	id = s.uniqueToolID(id)
	return s.emit("content_block_start", map[string]any{
		"type":  "content_block_start",
		"index": s.currentBlockIndex,
		"content_block": map[string]any{
			"type":  "tool_use",
			"id":    id,
			"name":  name,
			"input": map[string]any{},
		},
	})
}

// uniqueToolID returns the upstream tool ID when possible, otherwise generates a unique replacement.
func (s *streamState) uniqueToolID(id string) string {
	if id != "" && !s.usedToolIDs[id] {
		s.usedToolIDs[id] = true
		return id
	}
	for {
		candidate := "toolu_" + convert.RandomID()
		if !s.usedToolIDs[candidate] {
			s.usedToolIDs[candidate] = true
			return candidate
		}
	}
}

// emitToolArgsDelta emits an input_json_delta event for a tool_use block.
func (s *streamState) emitToolArgsDelta(idx int, partial string) error {
	return s.emit("content_block_delta", map[string]any{
		"type":  "content_block_delta",
		"index": idx,
		"delta": map[string]any{
			"type":         "input_json_delta",
			"partial_json": partial,
		},
	})
}

// recordToolDelta buffers upstream tool-call deltas until they can form valid Anthropic blocks.
func (s *streamState) recordToolDelta(tc convert.OpenAIStreamToolCall) {
	pending, ok := s.toolCalls[tc.Index]
	if !ok {
		pending = &pendingToolCall{}
		s.toolCalls[tc.Index] = pending
		s.toolOrder = append(s.toolOrder, tc.Index)
	}
	if tc.ID != "" {
		pending.id = tc.ID
	}
	if tc.Function == nil {
		return
	}
	if tc.Function.Name != "" {
		if pending.name == "" {
			pending.name = tc.Function.Name
		} else if pending.name != tc.Function.Name {
			pending.name += tc.Function.Name
		}
	}
	if tc.Function.Arguments != "" {
		pending.arguments.WriteString(tc.Function.Arguments)
	}
}

// flushToolCalls emits all buffered tool calls as complete Anthropic tool_use blocks.
func (s *streamState) flushToolCalls() error {
	if len(s.toolOrder) == 0 {
		return nil
	}
	if err := s.closeCurrentBlock(); err != nil {
		return err
	}
	for _, oaiIdx := range s.toolOrder {
		pending := s.toolCalls[oaiIdx]
		if pending == nil {
			continue
		}
		name := pending.name
		if name == "" {
			name = s.fallbackToolName(oaiIdx)
		}
		if err := s.openToolBlock(oaiIdx, pending.id, name); err != nil {
			return err
		}
		args := pending.arguments.String()
		if args == "" || !json.Valid([]byte(args)) {
			args = "{}"
		}
		if err := s.emitToolArgsDelta(s.currentBlockIndex, args); err != nil {
			return err
		}
		if err := s.closeCurrentBlock(); err != nil {
			return err
		}
	}
	return nil
}

// fallbackToolName returns a stable tool name when upstream stream chunks omit one.
func (s *streamState) fallbackToolName(oaiIdx int) string {
	if len(s.fallbackToolNames) == 1 {
		return s.fallbackToolNames[0]
	}
	if oaiIdx >= 0 && oaiIdx < len(s.fallbackToolNames) {
		return s.fallbackToolNames[oaiIdx]
	}
	return fmt.Sprintf("tool_%d", oaiIdx)
}

// requestToolNames extracts custom tool names from the original Anthropic request.
func requestToolNames(tools []anthropic.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		if t.Type != "" && t.Type != "custom" {
			continue
		}
		if t.Name != "" {
			names = append(names, t.Name)
		}
	}
	return names
}
