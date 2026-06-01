package convert

import (
	"encoding/json"
	"html"
	"strings"

	"github.com/thomas-illiet/anthropic-proxy/internal/anthropic"
)

// xmlSystemContent builds the text system prompt used by XML tool fallback mode.
func xmlSystemContent(raw json.RawMessage, tools []anthropic.Tool) (json.RawMessage, error) {
	var parts []string
	if len(raw) > 0 {
		text, err := flattenSystem(raw)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(text) != "" {
			parts = append(parts, text)
		}
	}
	if instructions := generateXMLToolInstructions(tools); instructions != "" {
		parts = append(parts, instructions)
	}
	if len(parts) == 0 {
		return nil, nil
	}
	content, _ := json.Marshal(strings.Join(parts, "\n\n"))
	return content, nil
}

// generateXMLToolInstructions renders available Anthropic tools as model-readable XML calling instructions.
func generateXMLToolInstructions(tools []anthropic.Tool) string {
	var definitions []string
	for _, tool := range tools {
		if tool.Type != "" && tool.Type != "custom" {
			continue
		}
		params := strings.TrimSpace(string(tool.InputSchema))
		if params == "" {
			params = `{"type":"object","properties":{}}`
		}
		definitions = append(definitions, "- **"+tool.Name+"**: "+html.EscapeString(tool.Description)+"\n  Parameters: "+params)
	}
	if len(definitions) == 0 {
		return ""
	}

	return strings.TrimSpace(`# TOOL CALLING FORMAT

You are required to use tools to fetch information or perform actions.
To invoke a tool, output raw XML in exactly this shape:

<tool_code name="TOOL_NAME">
{"argument_name":"value"}
</tool_code>

Rules:
1. Do not wrap the XML in Markdown code fences.
2. The content inside tool_code must be valid JSON.
3. The name attribute must exactly match one available tool name.
4. If you need to explain anything, write that text before the tool_code block.
5. You may call multiple tools by outputting multiple tool_code blocks.
6. Tool results will be returned as:
<tool_output>
result text or JSON
</tool_output>

Available Tools:

` + strings.Join(definitions, "\n\n"))
}

// xmlToolCallText renders a single tool call in the XML fallback format expected from the model.
func xmlToolCallText(name, args string) string {
	if strings.TrimSpace(args) == "" || !json.Valid([]byte(args)) {
		args = "{}"
	}
	return `<tool_code name="` + html.EscapeString(name) + `">` + "\n" + args + "\n</tool_code>"
}

// xmlToolOutputText wraps tool-result content before sending it back to the upstream model.
func xmlToolOutputText(content string) string {
	return "<tool_output>\n" + content + "\n</tool_output>"
}

// isAssistantPrefill reports whether assistant text is only a partial scaffold that should be dropped.
func isAssistantPrefill(content string) bool {
	trimmed := strings.TrimSpace(content)
	switch trimmed {
	case "{", "[", "```", `{"`, "[{", "<", "<tool_code", "<tool_code>":
		return true
	}
	if len(trimmed) <= 2 {
		return true
	}
	if strings.HasPrefix(trimmed, "<tool_code") && !strings.Contains(trimmed, "</tool_code>") {
		return true
	}
	return false
}
