package convert

import (
	"encoding/json"
	"testing"
)

// TestCountOpenAITokensCountsTextToolsAndContentParts verifies token counting covers content and tools.
func TestCountOpenAITokensCountsTextToolsAndContentParts(t *testing.T) {
	content, _ := json.Marshal([]map[string]any{
		{"type": "text", "text": "hello world"},
		{"type": "image_url", "image_url": map[string]string{"url": "data:image/png;base64,abc"}},
	})
	req := &OpenAIRequest{
		Model: "gpt-4o",
		Messages: []OpenAIMessage{{
			Role:    "user",
			Content: content,
		}},
		Tools: []OpenAITool{{
			Type: "function",
			Function: OpenAIFunction{
				Name:       "lookup",
				Parameters: json.RawMessage(`{"type":"object","properties":{"q":{"type":"string"}}}`),
			},
		}},
	}

	withTool := CountOpenAITokens(req)
	req.Tools = nil
	withoutTool := CountOpenAITokens(req)
	if withTool <= withoutTool {
		t.Fatalf("tool schema should add tokens: with=%d without=%d", withTool, withoutTool)
	}
	if withoutTool <= 0 {
		t.Fatalf("count = %d", withoutTool)
	}
}

// TestCountOpenAITokensFallsBackForUnknownModels verifies unknown models use the fallback tokenizer.
func TestCountOpenAITokensFallsBackForUnknownModels(t *testing.T) {
	content, _ := json.Marshal("hello")
	got := CountOpenAITokens(&OpenAIRequest{
		Model:    "not-a-known-model",
		Messages: []OpenAIMessage{{Role: "user", Content: content}},
	})
	if got <= 0 {
		t.Fatalf("count = %d", got)
	}
}
