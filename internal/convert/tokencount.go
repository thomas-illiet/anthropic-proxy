package convert

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/tiktoken-go/tokenizer"
)

var codecCache sync.Map

// CountOpenAITokens estimates tokens for the OpenAI-compatible request sent upstream.
func CountOpenAITokens(req *OpenAIRequest) int {
	codec := codecForModel(req.Model)
	count := 0

	for _, msg := range req.Messages {
		count += 4
		count += countText(codec, msg.Role)
		count += countText(codec, msg.Name)
		count += countRawContent(codec, msg.Content)
		count += countText(codec, msg.ToolCallID)
		count += countText(codec, msg.ReasoningContent)

		for _, tc := range msg.ToolCalls {
			count += 4
			count += countText(codec, tc.ID)
			count += countText(codec, tc.Type)
			count += countText(codec, tc.Function.Name)
			count += countText(codec, tc.Function.Arguments)
		}
	}

	if len(req.Tools) > 0 {
		count += countJSON(codec, req.Tools)
	}
	if len(req.ToolChoice) > 0 {
		count += countText(codec, string(req.ToolChoice))
	}
	if len(req.Stop) > 0 {
		count += countJSON(codec, req.Stop)
	}

	if count > 0 {
		count += 3
	}
	return count
}

// codecForModel returns a tokenizer codec for a model, falling back to cl100k_base.
func codecForModel(model string) tokenizer.Codec {
	model = strings.TrimSpace(model)
	if model != "" {
		if cached, ok := codecCache.Load(model); ok {
			return cached.(tokenizer.Codec)
		}
		if codec, err := tokenizer.ForModel(tokenizer.Model(model)); err == nil {
			codecCache.Store(model, codec)
			return codec
		}
	}

	const fallback = string(tokenizer.Cl100kBase)
	if cached, ok := codecCache.Load(fallback); ok {
		return cached.(tokenizer.Codec)
	}
	codec, err := tokenizer.Get(tokenizer.Cl100kBase)
	if err != nil {
		panic(err)
	}
	codecCache.Store(fallback, codec)
	return codec
}

// countRawContent counts tokens in string or structured JSON message content.
func countRawContent(codec tokenizer.Codec, raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return countText(codec, text)
	}

	var parts []map[string]any
	if err := json.Unmarshal(raw, &parts); err == nil {
		total := 0
		for _, part := range parts {
			total += countContentPart(codec, part)
		}
		return total
	}

	return countText(codec, string(raw))
}

// countContentPart counts tokens for a single structured content part.
func countContentPart(codec tokenizer.Codec, part map[string]any) int {
	switch part["type"] {
	case "text":
		if text, ok := part["text"].(string); ok {
			return countText(codec, text)
		}
	case "image_url":
		if imageURL, ok := part["image_url"].(map[string]any); ok {
			if url, ok := imageURL["url"].(string); ok {
				return countText(codec, url)
			}
		}
	}
	return countJSON(codec, part)
}

// countJSON marshals a value and counts the resulting JSON text.
func countJSON(codec tokenizer.Codec, v any) int {
	b, err := json.Marshal(v)
	if err != nil {
		return 0
	}
	return countText(codec, string(b))
}

// countText counts tokens in plain text, falling back to rune count on tokenizer errors.
func countText(codec tokenizer.Codec, text string) int {
	if text == "" {
		return 0
	}
	n, err := codec.Count(text)
	if err != nil {
		return len([]rune(text))
	}
	return n
}
