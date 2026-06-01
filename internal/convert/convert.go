package convert

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/thomas-illiet/anthropic-proxy/internal/anthropic"
	"github.com/thomas-illiet/anthropic-proxy/internal/config"
)

// ToOpenAI converts an Anthropic Messages request into an OpenAI-compatible chat request.
func ToOpenAI(a *anthropic.Request, cfg *config.Config) (*OpenAIRequest, error) {
	toolFormat := normalizedToolFormat(cfg)
	o := newOpenAIRequest(a, cfg)

	if err := appendSystemMessage(o, a, cfg, toolFormat); err != nil {
		return nil, err
	}
	toolIDs := newToolIDContext()
	if err := appendConvertedMessages(o, a, cfg, toolFormat, toolIDs); err != nil {
		return nil, err
	}
	if len(o.Messages) == 0 {
		return nil, fmt.Errorf("no messages after conversion")
	}

	if toolFormat == config.ToolFormatNative {
		applyNativeTools(o, a, cfg)
	}

	return o, nil
}

// newOpenAIRequest copies the request-level fields shared by both tool modes.
func newOpenAIRequest(a *anthropic.Request, cfg *config.Config) *OpenAIRequest {
	o := &OpenAIRequest{
		Model:       Model(cfg, a.Model),
		MaxTokens:   a.MaxTokens,
		Temperature: a.Temperature,
		TopP:        a.TopP,
		Stop:        a.StopSequences,
		Stream:      a.Stream,
	}
	if a.Stream {
		o.StreamOptions = &OpenAIStreamOptions{IncludeUsage: true}
	}
	return o
}

// appendSystemMessage converts Anthropic system content into the active upstream format.
func appendSystemMessage(o *OpenAIRequest, a *anthropic.Request, cfg *config.Config, toolFormat string) error {
	if toolFormat == config.ToolFormatXML {
		content, err := xmlSystemContent(a.System, a.Tools)
		if err != nil {
			return fmt.Errorf("system parse: %w", err)
		}
		if len(content) > 0 {
			o.Messages = append(o.Messages, OpenAIMessage{Role: "system", Content: content})
		}
		zero := 0.0
		o.Temperature = &zero
		return nil
	}

	if len(a.System) == 0 {
		return nil
	}
	content, err := systemContent(a.System, cfg.ForwardCacheControl)
	if err != nil {
		return fmt.Errorf("system parse: %w", err)
	}
	if len(content) > 0 {
		o.Messages = append(o.Messages, OpenAIMessage{Role: "system", Content: content})
	}
	return nil
}

// appendConvertedMessages converts each Anthropic message while preserving request order.
func appendConvertedMessages(o *OpenAIRequest, a *anthropic.Request, cfg *config.Config, toolFormat string, toolIDs *toolIDContext) error {
	for i, m := range a.Messages {
		msgs, err := convertMessage(m, cfg.ForwardCacheControl, toolFormat, toolIDs)
		if err != nil {
			return fmt.Errorf("message[%d]: %w", i, err)
		}
		o.Messages = append(o.Messages, msgs...)
	}
	return nil
}

// applyNativeTools attaches OpenAI function tools and translated tool choice settings.
func applyNativeTools(o *OpenAIRequest, a *anthropic.Request, cfg *config.Config) {
	o.Tools = nativeOpenAITools(a.Tools, cfg.ForwardCacheControl)
	if toolChoice := nativeToolChoice(a.ToolChoice); len(toolChoice) > 0 {
		o.ToolChoice = toolChoice
	}
}

// nativeOpenAITools converts custom Anthropic tools into OpenAI function tools.
func nativeOpenAITools(tools []anthropic.Tool, forwardCacheControl bool) []OpenAITool {
	out := make([]OpenAITool, 0, len(tools))
	for _, t := range tools {
		if t.Type != "" && t.Type != "custom" {
			continue
		}
		params := t.InputSchema
		if len(params) == 0 {
			params = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		tool := OpenAITool{
			Type: "function",
			Function: OpenAIFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		}
		if forwardCacheControl {
			tool.CacheControl = t.CacheControl
		}
		out = append(out, tool)
	}
	return out
}

// nativeToolChoice translates Anthropic tool_choice into OpenAI-compatible JSON.
func nativeToolChoice(choice *anthropic.ToolChoice) json.RawMessage {
	if choice == nil {
		return nil
	}
	switch choice.Type {
	case "auto":
		return json.RawMessage(`"auto"`)
	case "any":
		return json.RawMessage(`"required"`)
	case "none":
		return json.RawMessage(`"none"`)
	case "tool":
		b, _ := json.Marshal(map[string]any{
			"type":     "function",
			"function": map[string]string{"name": choice.Name},
		})
		return b
	default:
		return nil
	}
}

// normalizedToolFormat returns the effective tool-conversion mode for callers
// that construct Config directly in tests or embeddings.
func normalizedToolFormat(cfg *config.Config) string {
	if cfg != nil && cfg.ToolFormat == config.ToolFormatNative {
		return config.ToolFormatNative
	}
	return config.ToolFormatXML
}

// Model resolves an incoming Anthropic or Claude Code model name to the upstream model name.
func Model(cfg *config.Config, anthropicModel string) string {
	if cfg.ForceModel && cfg.DefaultModel != "" {
		return cfg.DefaultModel
	}

	candidates := modelCandidates(cfg, anthropicModel)
	for _, candidate := range candidates {
		if v, ok := cfg.ModelMap[candidate]; ok {
			return v
		}
	}

	keys := make([]string, 0, len(cfg.ModelMap))
	for k := range cfg.ModelMap {
		keys = append(keys, k)
	}
	// Prefer the most specific configured model-map prefix before broader family prefixes.
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})
	for _, k := range keys {
		for _, candidate := range candidates {
			if strings.HasPrefix(candidate, k) {
				return cfg.ModelMap[k]
			}
		}
	}

	if cfg.DefaultModel != "" {
		return cfg.DefaultModel
	}
	if len(candidates) > 0 {
		return candidates[len(candidates)-1]
	}
	return anthropic.StripContextSuffix(anthropicModel)
}

// modelCandidates returns the normalized model names to try when applying the configured model map.
func modelCandidates(cfg *config.Config, anthropicModel string) []string {
	seen := map[string]bool{}
	candidates := []string{}
	// add normalizes and deduplicates model candidates while preserving resolution order.
	add := func(model string) {
		model = anthropic.StripContextSuffix(model)
		if model == "" || seen[model] {
			return
		}
		seen[model] = true
		candidates = append(candidates, model)
	}

	add(anthropicModel)
	aliasKey := strings.ToLower(anthropic.StripContextSuffix(anthropicModel))
	aliases := anthropic.EffectiveAliases(cfg.ModelAliases)
	if aliasTarget, ok := aliases[aliasKey]; ok {
		add(aliasTarget)
	}
	return candidates
}

// FromOpenAI converts an OpenAI-compatible chat response into an Anthropic message response.
func FromOpenAI(o *OpenAIResponse, originalModel string) *anthropic.Response {
	ares := &anthropic.Response{
		ID:    "msg_" + sanitizeID(o.ID),
		Type:  "message",
		Role:  "assistant",
		Model: originalModel,
		Usage: UsageFromOpenAI(o.Usage),
	}
	if len(o.Choices) == 0 {
		ares.StopReason = "end_turn"
		ares.Content = []anthropic.Block{}
		return ares
	}
	ch := o.Choices[0]

	var textContent string
	if len(ch.Message.Content) > 0 {
		_ = json.Unmarshal(ch.Message.Content, &textContent)
	}
	if textContent != "" {
		ares.Content = append(ares.Content, anthropic.Block{Type: "text", Text: textContent})
	}
	if reasoning := firstNonEmpty(ch.Message.ReasoningContent, ch.Message.Reasoning, ch.Message.Thinking); reasoning != "" {
		ares.Content = append([]anthropic.Block{{
			Type:     "thinking",
			Thinking: reasoning,
		}}, ares.Content...)
	}

	for _, tc := range ch.Message.ToolCalls {
		var input json.RawMessage
		if tc.Function.Arguments == "" || !json.Valid([]byte(tc.Function.Arguments)) {
			input = json.RawMessage(`{}`)
		} else {
			input = json.RawMessage(tc.Function.Arguments)
		}
		ares.Content = append(ares.Content, anthropic.Block{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: input,
		})
	}

	if len(ares.Content) == 0 {
		ares.Content = []anthropic.Block{{Type: "text", Text: ""}}
	}

	ares.StopReason = FinishReason(ch.FinishReason, len(ch.Message.ToolCalls) > 0)
	return ares
}

// UsageFromOpenAI maps OpenAI-compatible usage fields into Anthropic usage fields.
func UsageFromOpenAI(u OpenAIUsage) anthropic.Usage {
	cacheReadTokens := 0
	if u.PromptTokensDetails != nil {
		cacheReadTokens = u.PromptTokensDetails.CachedTokens
	}
	inputTokens := u.PromptTokens - cacheReadTokens
	if inputTokens < 0 {
		inputTokens = 0
	}
	return anthropic.Usage{
		InputTokens:          inputTokens,
		OutputTokens:         u.CompletionTokens,
		CacheReadInputTokens: cacheReadTokens,
	}
}

// FinishReason maps OpenAI finish reasons into Anthropic stop reasons.
func FinishReason(r string, hasToolCalls bool) string {
	switch r {
	case "stop":
		if hasToolCalls {
			return "tool_use"
		}
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls", "function_call":
		return "tool_use"
	case "content_filter":
		return "end_turn"
	default:
		if hasToolCalls {
			return "tool_use"
		}
		return "end_turn"
	}
}

// RandomID returns a random hexadecimal identifier fragment.
func RandomID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// flattenSystem converts Anthropic system content into plain text for upstreams without structured support.
func flattenSystem(raw json.RawMessage) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	var blocks []anthropic.Block
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, b := range blocks {
		if b.Type == "text" {
			sb.WriteString(b.Text)
		} else if b.Type == "thinking" && b.Thinking != "" {
			sb.WriteString(b.Thinking)
		}
	}
	return sb.String(), nil
}

// systemContent converts Anthropic system content into the configured upstream content shape.
func systemContent(raw json.RawMessage, forwardCacheControl bool) (json.RawMessage, error) {
	if !forwardCacheControl {
		sys, err := flattenSystem(raw)
		if err != nil {
			return nil, err
		}
		if sys == "" {
			return nil, nil
		}
		c, _ := json.Marshal(sys)
		return c, nil
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		c, _ := json.Marshal(s)
		return c, nil
	}
	var blocks []anthropic.Block
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil, err
	}
	parts := make([]map[string]any, 0, len(blocks))
	for _, b := range blocks {
		switch b.Type {
		case "text":
			part := map[string]any{"type": "text", "text": b.Text}
			addCacheControl(part, b.CacheControl, true)
			parts = append(parts, part)
		case "thinking":
			part := map[string]any{"type": "text", "text": b.Thinking}
			addCacheControl(part, b.CacheControl, true)
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return nil, nil
	}
	c, _ := json.Marshal(parts)
	return c, nil
}

type toolIDContext struct {
	seen        map[string]bool
	idMappings  map[string][]string
	resultIndex map[string]int
}

// newToolIDContext creates per-request state for repairing duplicate or missing tool IDs.
func newToolIDContext() *toolIDContext {
	return &toolIDContext{
		seen:        map[string]bool{},
		idMappings:  map[string][]string{},
		resultIndex: map[string]int{},
	}
}

// recordToolUse returns a unique OpenAI tool-call ID and records its mapping from the original Anthropic ID.
func (c *toolIDContext) recordToolUse(original string) string {
	if c == nil {
		return original
	}
	id := original
	if id == "" || c.seen[id] {
		id = c.uniqueID()
	}
	c.seen[id] = true
	c.idMappings[original] = append(c.idMappings[original], id)
	return id
}

// remapToolResult returns the repaired OpenAI tool-call ID for an incoming Anthropic tool result.
func (c *toolIDContext) remapToolResult(original string) string {
	if c == nil {
		return original
	}
	mappings := c.idMappings[original]
	if len(mappings) == 0 {
		return original
	}
	idx := c.resultIndex[original]
	if idx >= len(mappings) {
		return original
	}
	c.resultIndex[original] = idx + 1
	return mappings[idx]
}

// uniqueID generates a tool-use ID that has not appeared in this conversion.
func (c *toolIDContext) uniqueID() string {
	for {
		id := "toolu_" + RandomID()
		if !c.seen[id] {
			return id
		}
	}
}

// convertMessage converts one Anthropic message into one or more OpenAI-compatible messages.
func convertMessage(m anthropic.Message, forwardCacheControl bool, toolFormat string, toolIDs *toolIDContext) ([]OpenAIMessage, error) {
	var asString string
	if err := json.Unmarshal(m.Content, &asString); err == nil {
		if m.Role == "assistant" && isAssistantPrefill(asString) {
			return nil, nil
		}
		c, _ := json.Marshal(asString)
		return []OpenAIMessage{{Role: m.Role, Content: c}}, nil
	}

	var blocks []anthropic.Block
	if err := json.Unmarshal(m.Content, &blocks); err != nil {
		return nil, fmt.Errorf("content: %w", err)
	}

	if m.Role == "assistant" {
		return convertAssistantBlocks(blocks, toolFormat, toolIDs), nil
	}

	if toolFormat == config.ToolFormatXML {
		return convertUserBlocksXML(blocks), nil
	}
	return convertUserBlocksNative(blocks, forwardCacheControl, toolIDs), nil
}

// convertAssistantBlocks converts Anthropic assistant content blocks into a single OpenAI assistant message.
func convertAssistantBlocks(blocks []anthropic.Block, toolFormat string, toolIDs *toolIDContext) []OpenAIMessage {
	var textBuf strings.Builder
	toolCalls := []OpenAIToolCall{}
	for _, b := range blocks {
		switch b.Type {
		case "text":
			textBuf.WriteString(b.Text)
		case "thinking", "redacted_thinking":
			continue
		case "tool_use", "server_tool_use":
			args := string(b.Input)
			if args == "" || !json.Valid([]byte(args)) {
				args = "{}"
			}
			toolCalls = append(toolCalls, OpenAIToolCall{
				ID:   toolIDs.recordToolUse(b.ID),
				Type: "function",
				Function: OpenAIFunctionCall{
					Name:      b.Name,
					Arguments: args,
				},
			})
		}
	}

	text := textBuf.String()
	if len(toolCalls) == 0 && isAssistantPrefill(text) {
		return nil
	}
	if toolFormat == config.ToolFormatXML {
		fullContent := text
		for _, tc := range toolCalls {
			if fullContent != "" {
				fullContent += "\n\n"
			}
			fullContent += xmlToolCallText(tc.Function.Name, tc.Function.Arguments)
		}
		c, _ := json.Marshal(fullContent)
		return []OpenAIMessage{{Role: "assistant", Content: c}}
	}

	msg := OpenAIMessage{Role: "assistant"}
	if text != "" {
		c, _ := json.Marshal(text)
		msg.Content = c
	}
	msg.ToolCalls = toolCalls
	if msg.Content == nil && len(msg.ToolCalls) == 0 {
		msg.Content = json.RawMessage(`""`)
	}
	return []OpenAIMessage{msg}
}

// hasNativeStructuredContent reports whether the user message needs OpenAI content parts.
func hasNativeStructuredContent(blocks []anthropic.Block, forwardCacheControl bool) bool {
	for _, b := range blocks {
		switch b.Type {
		case "image", "document":
			return true
		case "text":
			if forwardCacheControl && len(b.CacheControl) > 0 {
				return true
			}
		}
	}
	return false
}

type nativeUserAccumulator struct {
	out                 []OpenAIMessage
	userParts           []map[string]any
	userPlainText       strings.Builder
	hasStructured       bool
	forwardCacheControl bool
	toolIDs             *toolIDContext
}

// newNativeUserAccumulator creates per-message state for splitting user content around tool results.
func newNativeUserAccumulator(blocks []anthropic.Block, forwardCacheControl bool, toolIDs *toolIDContext) *nativeUserAccumulator {
	return &nativeUserAccumulator{
		hasStructured:       hasNativeStructuredContent(blocks, forwardCacheControl),
		forwardCacheControl: forwardCacheControl,
		toolIDs:             toolIDs,
	}
}

// convertUserBlocksNative converts Anthropic user blocks into OpenAI content parts and tool-result messages.
func convertUserBlocksNative(blocks []anthropic.Block, forwardCacheControl bool, toolIDs *toolIDContext) []OpenAIMessage {
	acc := newNativeUserAccumulator(blocks, forwardCacheControl, toolIDs)
	for _, b := range blocks {
		switch b.Type {
		case "text":
			acc.appendTextBlock(b)
		case "image":
			acc.appendImageBlock(b)
		case "document":
			acc.appendDocumentBlock(b)
		case "tool_result", "server_tool_result":
			acc.appendToolResultBlock(b)
		default:
			acc.appendFallbackBlock(b)
		}
	}
	acc.flushUser()
	return acc.out
}

// flushUser emits accumulated user content whenever a tool result splits the message.
func (a *nativeUserAccumulator) flushUser() {
	if len(a.userParts) > 0 {
		c, _ := json.Marshal(a.userParts)
		a.out = append(a.out, OpenAIMessage{Role: "user", Content: c})
		a.userParts = nil
		return
	}
	if a.userPlainText.Len() == 0 {
		return
	}
	c, _ := json.Marshal(a.userPlainText.String())
	a.out = append(a.out, OpenAIMessage{Role: "user", Content: c})
	a.userPlainText.Reset()
}

// appendText appends text in either plain string mode or structured content-part mode.
func (a *nativeUserAccumulator) appendText(text string, cacheControl json.RawMessage) {
	if text == "" {
		return
	}
	if a.hasStructured {
		part := map[string]any{"type": "text", "text": text}
		addCacheControl(part, cacheControl, a.forwardCacheControl)
		a.userParts = append(a.userParts, part)
		return
	}
	a.userPlainText.WriteString(text)
}

// appendTextBlock preserves cache-control metadata when structured content is required.
func (a *nativeUserAccumulator) appendTextBlock(b anthropic.Block) {
	a.appendText(b.Text, b.CacheControl)
}

// appendImageBlock maps Anthropic image sources onto OpenAI image_url parts.
func (a *nativeUserAccumulator) appendImageBlock(b anthropic.Block) {
	if b.Source == nil {
		return
	}
	dataURL := sourceURL(b.Source)
	if dataURL == "" {
		return
	}
	part := map[string]any{
		"type":      "image_url",
		"image_url": map[string]string{"url": dataURL},
	}
	addCacheControl(part, b.CacheControl, a.forwardCacheControl)
	a.userParts = append(a.userParts, part)
}

// appendDocumentBlock flattens Anthropic document content into a text part.
func (a *nativeUserAccumulator) appendDocumentBlock(b anthropic.Block) {
	part := map[string]any{
		"type": "text",
		"text": documentText(b),
	}
	addCacheControl(part, b.CacheControl, a.forwardCacheControl)
	a.userParts = append(a.userParts, part)
}

// appendToolResultBlock emits native tool result messages or falls back to user text without an ID.
func (a *nativeUserAccumulator) appendToolResultBlock(b anthropic.Block) {
	if b.ToolUseID == "" {
		a.appendText(flattenToolResultContent(b.Content), nil)
		return
	}

	// Tool results must be their own OpenAI messages, so flush pending user content first.
	a.flushUser()
	txt := flattenToolResultContent(b.Content)
	if txt == "" {
		txt = "(empty)"
	}
	if b.IsError {
		txt = "Error: " + txt
	}
	c, _ := json.Marshal(txt)
	a.out = append(a.out, OpenAIMessage{
		Role:       "tool",
		ToolCallID: a.toolIDs.remapToolResult(b.ToolUseID),
		Content:    c,
	})
}

// appendFallbackBlock preserves unsupported Anthropic blocks as user-visible text when possible.
func (a *nativeUserAccumulator) appendFallbackBlock(b anthropic.Block) {
	a.appendText(fallbackBlockText(b), nil)
}

// convertUserBlocksXML converts Anthropic user blocks into plain text for XML tool fallback mode.
func convertUserBlocksXML(blocks []anthropic.Block) []OpenAIMessage {
	parts := []string{}
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if b.Text != "" {
				parts = append(parts, b.Text)
			}
		case "tool_result", "server_tool_result":
			parts = append(parts, xmlToolOutputText(flattenToolResultContent(b.Content)))
		case "document":
			parts = append(parts, documentText(b))
		case "image":
			if b.Source != nil {
				if url := sourceURL(b.Source); url != "" {
					parts = append(parts, "[Image: "+url+"]")
				}
			}
		default:
			if txt := fallbackBlockText(b); txt != "" {
				parts = append(parts, txt)
			}
		}
	}
	content := strings.Join(parts, "\n\n")
	if content == "" {
		return nil
	}
	c, _ := json.Marshal(content)
	return []OpenAIMessage{{Role: "user", Content: c}}
}

// addCacheControl attaches cache_control to a structured content part when passthrough is enabled.
func addCacheControl(part map[string]any, raw json.RawMessage, forward bool) {
	if forward && len(raw) > 0 {
		part["cache_control"] = json.RawMessage(raw)
	}
}

// sourceURL returns an OpenAI-compatible URL for Anthropic image or document sources.
func sourceURL(src *anthropic.ContentSource) string {
	if src == nil {
		return ""
	}
	if src.URL != "" {
		return src.URL
	}
	if src.Data != "" && src.MediaType != "" {
		return fmt.Sprintf("data:%s;base64,%s", src.MediaType, src.Data)
	}
	return ""
}

// documentText converts an Anthropic document block into text that can be sent upstream.
func documentText(b anthropic.Block) string {
	label := "document"
	if b.Title != "" {
		label = b.Title
	} else if b.Source != nil && b.Source.MediaType != "" {
		label = b.Source.MediaType
	}
	if b.Source != nil && b.Source.Data != "" {
		if decoded := decodeTextDocument(b.Source); decoded != "" {
			return fmt.Sprintf("[Document: %s]\n%s", label, decoded)
		}
		return fmt.Sprintf("[Document: %s]\n%s", label, b.Source.Data)
	}
	if b.Source != nil && b.Source.URL != "" {
		return fmt.Sprintf("[Document: %s]\n%s", label, b.Source.URL)
	}
	if b.Context != "" {
		return fmt.Sprintf("[Document: %s]\n%s", label, b.Context)
	}
	return fmt.Sprintf("[Document: %s]", label)
}

// decodeTextDocument decodes inline text documents from supported base64 document sources.
func decodeTextDocument(src *anthropic.ContentSource) string {
	if src == nil || src.Data == "" || !strings.HasPrefix(src.MediaType, "text/") {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(src.Data)
	if err != nil {
		return ""
	}
	return string(decoded)
}

// flattenToolResultContent converts Anthropic tool result content into plain text.
func flattenToolResultContent(raw json.RawMessage) string {
	var txt string
	if err := json.Unmarshal(raw, &txt); err == nil {
		return txt
	}
	var rblocks []anthropic.Block
	if err := json.Unmarshal(raw, &rblocks); err == nil {
		var sb strings.Builder
		for _, rb := range rblocks {
			switch rb.Type {
			case "text":
				sb.WriteString(rb.Text)
			case "image":
				if rb.Source != nil {
					if url := sourceURL(rb.Source); url != "" {
						sb.WriteString("\n[Image: ")
						sb.WriteString(url)
						sb.WriteString("]")
					}
				}
			case "document":
				sb.WriteString("\n")
				sb.WriteString(documentText(rb))
			}
		}
		return strings.TrimSpace(sb.String())
	}
	return ""
}

// fallbackBlockText returns a safe textual placeholder for blocks without OpenAI equivalents.
func fallbackBlockText(b anthropic.Block) string {
	if b.Text != "" {
		return b.Text
	}
	if b.Thinking != "" {
		return b.Thinking
	}
	if b.Data != "" {
		return b.Data
	}
	if len(b.Content) > 0 {
		var txt string
		if err := json.Unmarshal(b.Content, &txt); err == nil {
			return txt
		}
		return string(b.Content)
	}
	return ""
}

// sanitizeID removes provider-specific prefixes from upstream response IDs.
func sanitizeID(id string) string {
	if id == "" {
		return RandomID()
	}
	return strings.TrimPrefix(id, "chatcmpl-")
}

// firstNonEmpty returns the first non-empty string from a list of candidates.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
