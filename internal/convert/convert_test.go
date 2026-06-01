package convert

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/thomas-illiet/anthropic-proxy/internal/anthropic"
	"github.com/thomas-illiet/anthropic-proxy/internal/config"
)

// TestModelMapping verifies exact, prefix, alias, fallback, and forced model routing.
func TestModelMapping(t *testing.T) {
	cfg := &config.Config{
		DefaultModel: "fallback",
		ModelAliases: anthropic.Aliases(
			anthropic.DefaultOpusModel,
			anthropic.DefaultSonnetModel,
			anthropic.DefaultHaikuModel,
		),
		ModelMap: map[string]string{
			"claude":        "broad",
			"claude-sonnet": "specific",
			"exact-model":   "exact",
		},
	}

	if got := Model(cfg, "claude-sonnet-4"); got != "specific" {
		t.Fatalf("longest prefix mapping = %q", got)
	}
	if got := Model(cfg, "exact-model"); got != "exact" {
		t.Fatalf("exact mapping = %q", got)
	}
	if got := Model(cfg, "sonnet"); got != "specific" {
		t.Fatalf("sonnet alias mapping = %q", got)
	}
	if got := Model(cfg, "sonnet[1m]"); got != "specific" {
		t.Fatalf("sonnet[1m] alias mapping = %q", got)
	}
	if got := Model(cfg, "opus"); got != "broad" {
		t.Fatalf("opus alias mapping = %q", got)
	}
	if got := Model(cfg, "unknown"); got != "fallback" {
		t.Fatalf("fallback mapping = %q", got)
	}

	cfg.ForceModel = true
	if got := Model(cfg, "exact-model"); got != "fallback" {
		t.Fatalf("force model mapping = %q", got)
	}
}

// TestToOpenAIConvertsRequest verifies core request conversion, tools, and streaming options.
func TestToOpenAIConvertsRequest(t *testing.T) {
	cfg := &config.Config{DefaultModel: "upstream-model", ForceModel: true, ToolFormat: config.ToolFormatNative}
	req := &anthropic.Request{
		Model:     "claude",
		MaxTokens: 128,
		System:    json.RawMessage(`[{"type":"text","text":"system prompt"}]`),
		Messages: []anthropic.Message{
			{Role: "user", Content: json.RawMessage(`"hello"`)},
			{Role: "assistant", Content: json.RawMessage(`[
				{"type":"text","text":"thinking"},
				{"type":"tool_use","id":"toolu_1","name":"lookup","input":{"q":"go"}}
			]`)},
			{Role: "user", Content: json.RawMessage(`[
				{"type":"text","text":"see this"},
				{"type":"image","source":{"type":"base64","media_type":"image/png","data":"abc"}}
			]`)},
		},
		Tools: []anthropic.Tool{{
			Name:        "lookup",
			Description: "Lookup things",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}},
		ToolChoice: &anthropic.ToolChoice{Type: "any"},
		Stream:     true,
	}

	got, err := ToOpenAI(req, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != "upstream-model" || got.MaxTokens != 128 || !got.Stream {
		t.Fatalf("unexpected request: %+v", got)
	}
	if got.StreamOptions == nil || !got.StreamOptions.IncludeUsage {
		t.Fatalf("stream options = %+v", got.StreamOptions)
	}
	if len(got.Messages) != 4 {
		t.Fatalf("messages len = %d", len(got.Messages))
	}
	if got.Messages[0].Role != "system" || string(got.Messages[0].Content) != `"system prompt"` {
		t.Fatalf("system message = %+v", got.Messages[0])
	}
	if len(got.Messages[2].ToolCalls) != 1 || got.Messages[2].ToolCalls[0].Function.Arguments != `{"q":"go"}` {
		t.Fatalf("assistant tool call = %+v", got.Messages[2])
	}
	if got.Messages[3].Role != "user" || !json.Valid(got.Messages[3].Content) {
		t.Fatalf("image user content = %+v", got.Messages[3])
	}
	if len(got.Tools) != 1 || got.Tools[0].Function.Name != "lookup" {
		t.Fatalf("tools = %+v", got.Tools)
	}
	if string(got.ToolChoice) != `"required"` {
		t.Fatalf("tool choice = %s", got.ToolChoice)
	}
}

// TestConvertToolResult verifies Anthropic tool results become OpenAI-compatible tool messages.
func TestConvertToolResult(t *testing.T) {
	msgs, err := convertMessage(anthropic.Message{
		Role: "user",
		Content: json.RawMessage(`[
			{"type":"text","text":"before"},
			{"type":"tool_result","tool_use_id":"toolu_1","content":[{"type":"text","text":"tool text"}]},
			{"type":"text","text":"after"}
		]`),
	}, false, config.ToolFormatNative, newToolIDContext())
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 {
		t.Fatalf("messages len = %d", len(msgs))
	}
	if msgs[1].Role != "tool" || msgs[1].ToolCallID != "toolu_1" || string(msgs[1].Content) != `"tool text"` {
		t.Fatalf("tool result = %+v", msgs[1])
	}
}

// TestConvertModernBlocks verifies modern Anthropic content blocks are handled safely.
func TestConvertModernBlocks(t *testing.T) {
	cfg := &config.Config{DefaultModel: "upstream-model", ForceModel: true, ToolFormat: config.ToolFormatNative}
	req := &anthropic.Request{
		Model: "claude",
		Messages: []anthropic.Message{
			{Role: "assistant", Content: json.RawMessage(`[
				{"type":"thinking","thinking":"private reasoning","signature":"sig"},
				{"type":"text","text":"visible"},
				{"type":"tool_use","id":"toolu_1","name":"lookup","input":{"q":"go"}}
			]`)},
			{Role: "user", Content: json.RawMessage(`[
				{"type":"document","title":"notes.txt","source":{"type":"base64","media_type":"text/plain","data":"bm90ZXM="}},
				{"type":"tool_result","tool_use_id":"toolu_1","content":[
					{"type":"text","text":"tool text"},
					{"type":"document","title":"result.txt","source":{"type":"base64","media_type":"text/plain","data":"cmVzdWx0"}}
				]}
			]`)},
		},
	}

	got, err := ToOpenAI(req, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got.Messages[0].Content), "private reasoning") {
		t.Fatalf("assistant thinking should not be replayed upstream: %s", got.Messages[0].Content)
	}
	if len(got.Messages[0].ToolCalls) != 1 {
		t.Fatalf("tool calls = %+v", got.Messages[0].ToolCalls)
	}
	if !strings.Contains(string(got.Messages[1].Content), "[Document: notes.txt]") || !strings.Contains(string(got.Messages[1].Content), "notes") {
		t.Fatalf("document content = %s", got.Messages[1].Content)
	}
	if got.Messages[2].Role != "tool" || !strings.Contains(string(got.Messages[2].Content), "[Document: result.txt]") || !strings.Contains(string(got.Messages[2].Content), "result") {
		t.Fatalf("tool result = %+v", got.Messages[2])
	}
}

// TestToOpenAIForwardsCacheControlWhenEnabled verifies cache_control passthrough.
func TestToOpenAIForwardsCacheControlWhenEnabled(t *testing.T) {
	cfg := &config.Config{DefaultModel: "upstream-model", ForceModel: true, ForwardCacheControl: true, ToolFormat: config.ToolFormatNative}
	req := &anthropic.Request{
		Model:  "claude",
		System: json.RawMessage(`[{"type":"text","text":"system prompt","cache_control":{"type":"ephemeral"}}]`),
		Messages: []anthropic.Message{{
			Role: "user",
			Content: json.RawMessage(`[
				{"type":"text","text":"cached text","cache_control":{"type":"ephemeral"}}
			]`),
		}},
		Tools: []anthropic.Tool{{
			Name:         "lookup",
			InputSchema:  json.RawMessage(`{"type":"object"}`),
			CacheControl: json.RawMessage(`{"type":"ephemeral"}`),
		}},
	}

	got, err := ToOpenAI(req, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got.Messages[0].Content), `"cache_control":{"type":"ephemeral"}`) {
		t.Fatalf("system cache_control not forwarded: %s", got.Messages[0].Content)
	}
	if !strings.Contains(string(got.Messages[1].Content), `"cache_control":{"type":"ephemeral"}`) {
		t.Fatalf("message cache_control not forwarded: %s", got.Messages[1].Content)
	}
	if string(got.Tools[0].CacheControl) != `{"type":"ephemeral"}` {
		t.Fatalf("tool cache_control = %s", got.Tools[0].CacheControl)
	}
}

// TestToOpenAIXMLToolFallback verifies Claude Adapter-style XML tool fallback conversion.
func TestToOpenAIXMLToolFallback(t *testing.T) {
	temp := 0.8
	cfg := &config.Config{DefaultModel: "upstream-model", ForceModel: true, ToolFormat: config.ToolFormatXML}
	req := &anthropic.Request{
		Model:       "claude",
		MaxTokens:   128,
		System:      json.RawMessage(`"system prompt"`),
		Temperature: &temp,
		Messages: []anthropic.Message{
			{Role: "user", Content: json.RawMessage(`"hello"`)},
			{Role: "assistant", Content: json.RawMessage(`[
				{"type":"text","text":"I will look."},
				{"type":"tool_use","id":"toolu_1","name":"lookup","input":{"q":"go"}}
			]`)},
			{Role: "user", Content: json.RawMessage(`[
				{"type":"tool_result","tool_use_id":"toolu_1","content":[{"type":"text","text":"tool text"}]}
			]`)},
		},
		Tools: []anthropic.Tool{{
			Name:        "lookup",
			Description: "Lookup things",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"q":{"type":"string"}}}`),
		}},
		ToolChoice: &anthropic.ToolChoice{Type: "any"},
	}

	got, err := ToOpenAI(req, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Tools) != 0 || len(got.ToolChoice) != 0 {
		t.Fatalf("native tools should be omitted in XML mode: tools=%+v choice=%s", got.Tools, got.ToolChoice)
	}
	if got.Temperature == nil || *got.Temperature != 0 {
		t.Fatalf("XML temperature = %v", got.Temperature)
	}
	if !strings.Contains(string(got.Messages[0].Content), "# TOOL CALLING FORMAT") || !strings.Contains(string(got.Messages[0].Content), "lookup") {
		t.Fatalf("XML system prompt = %s", got.Messages[0].Content)
	}
	if !strings.Contains(string(got.Messages[2].Content), `tool_code name=\"lookup\"`) || !strings.Contains(string(got.Messages[2].Content), `{\"q\":\"go\"}`) {
		t.Fatalf("XML assistant content = %s", got.Messages[2].Content)
	}
	if !strings.Contains(string(got.Messages[3].Content), "tool_output") || !strings.Contains(string(got.Messages[3].Content), "tool text") {
		t.Fatalf("XML tool output = %s", got.Messages[3].Content)
	}
}

// TestAssistantPrefillFilteringMatchesAdapter verifies Claude Adapter-style prefill stripping.
func TestAssistantPrefillFilteringMatchesAdapter(t *testing.T) {
	cfg := &config.Config{DefaultModel: "upstream-model", ForceModel: true, ToolFormat: config.ToolFormatNative}
	req := &anthropic.Request{
		Model: "claude",
		Messages: []anthropic.Message{
			{Role: "user", Content: json.RawMessage(`"make JSON"`)},
			{Role: "assistant", Content: json.RawMessage(`"{"`)},
			{Role: "assistant", Content: json.RawMessage(`"ok"`)},
			{Role: "assistant", Content: json.RawMessage(`[{"type":"text","text":"<tool_code name=\"lookup\">"}]`)},
		},
	}

	got, err := ToOpenAI(req, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) != 1 || got.Messages[0].Role != "user" {
		t.Fatalf("prefill messages should be stripped: %+v", got.Messages)
	}
}

// TestDuplicateToolIDsAreRepaired verifies duplicate assistant tool IDs remap tool results.
func TestDuplicateToolIDsAreRepaired(t *testing.T) {
	cfg := &config.Config{DefaultModel: "upstream-model", ForceModel: true, ToolFormat: config.ToolFormatNative}
	req := &anthropic.Request{
		Model: "claude",
		Messages: []anthropic.Message{
			{Role: "assistant", Content: json.RawMessage(`[{"type":"tool_use","id":"toolu_dup","name":"first","input":{"x":1}}]`)},
			{Role: "assistant", Content: json.RawMessage(`[{"type":"tool_use","id":"toolu_dup","name":"second","input":{"x":2}}]`)},
			{Role: "user", Content: json.RawMessage(`[
				{"type":"tool_result","tool_use_id":"toolu_dup","content":"first result"},
				{"type":"tool_result","tool_use_id":"toolu_dup","content":"second result"}
			]`)},
		},
	}

	got, err := ToOpenAI(req, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) != 4 {
		t.Fatalf("messages = %+v", got.Messages)
	}
	firstID := got.Messages[0].ToolCalls[0].ID
	secondID := got.Messages[1].ToolCalls[0].ID
	if firstID != "toolu_dup" {
		t.Fatalf("first ID = %q", firstID)
	}
	if secondID == "" || secondID == firstID {
		t.Fatalf("second ID was not repaired: %q", secondID)
	}
	if got.Messages[2].ToolCallID != firstID || got.Messages[3].ToolCallID != secondID {
		t.Fatalf("tool results not remapped: %+v / %+v", got.Messages[2], got.Messages[3])
	}
}

// TestFromOpenAIConvertsResponse verifies response metadata, content, tool calls, and usage.
func TestFromOpenAIConvertsResponse(t *testing.T) {
	content, _ := json.Marshal("hello")
	resp := FromOpenAI(&OpenAIResponse{
		ID:    "chatcmpl-abc",
		Model: "upstream",
		Choices: []OpenAIChoice{{
			Message: OpenAIMessage{
				Content: content,
				ToolCalls: []OpenAIToolCall{{
					ID:   "call_1",
					Type: "function",
					Function: OpenAIFunctionCall{
						Name:      "lookup",
						Arguments: `{"q":"go"}`,
					},
				}},
			},
			FinishReason: "tool_calls",
		}},
		Usage: OpenAIUsage{PromptTokens: 3, CompletionTokens: 4},
	}, "claude")

	if resp.ID != "msg_abc" || resp.Model != "claude" || resp.StopReason != "tool_use" {
		t.Fatalf("response metadata = %+v", resp)
	}
	if resp.Usage.InputTokens != 3 || resp.Usage.OutputTokens != 4 {
		t.Fatalf("usage = %+v", resp.Usage)
	}
	if len(resp.Content) != 2 || resp.Content[0].Text != "hello" || resp.Content[1].Name != "lookup" {
		t.Fatalf("content = %+v", resp.Content)
	}
}

// TestFromOpenAIMapsCachedPromptTokens verifies upstream cached prompt tokens map to Anthropic usage.
func TestFromOpenAIMapsCachedPromptTokens(t *testing.T) {
	resp := FromOpenAI(&OpenAIResponse{
		ID: "chatcmpl-cache",
		Choices: []OpenAIChoice{{
			Message:      OpenAIMessage{Content: json.RawMessage(`"hello"`)},
			FinishReason: "stop",
		}},
		Usage: OpenAIUsage{
			PromptTokens:     100,
			CompletionTokens: 5,
			PromptTokensDetails: &OpenAIPromptTokenDetails{
				CachedTokens: 40,
			},
		},
	}, "claude")

	if resp.Usage.InputTokens != 60 || resp.Usage.CacheReadInputTokens != 40 || resp.Usage.OutputTokens != 5 {
		t.Fatalf("usage = %+v", resp.Usage)
	}
}

// TestFinishReason verifies OpenAI finish reasons map to Anthropic stop reasons.
func TestFinishReason(t *testing.T) {
	tests := []struct {
		name         string
		finishReason string
		hasTools     bool
		want         string
	}{
		{name: "stop", finishReason: "stop", want: "end_turn"},
		{name: "stop with tools", finishReason: "stop", hasTools: true, want: "tool_use"},
		{name: "length", finishReason: "length", want: "max_tokens"},
		{name: "tool calls", finishReason: "tool_calls", want: "tool_use"},
		{name: "default with tools", finishReason: "other", hasTools: true, want: "tool_use"},
	}

	for _, tc := range tests {
		// Run each finish-reason case independently for clearer failures.
		t.Run(tc.name, func(t *testing.T) {
			if got := FinishReason(tc.finishReason, tc.hasTools); got != tc.want {
				t.Fatalf("FinishReason() = %q, want %q", got, tc.want)
			}
		})
	}
}
