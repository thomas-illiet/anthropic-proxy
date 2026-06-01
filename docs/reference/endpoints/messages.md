# Messages Endpoint

`POST /v1/messages` accepts Anthropic-compatible message requests and forwards them to an OpenAI-compatible chat completions upstream.

## Sync Request

```bash
curl http://127.0.0.1:8787/v1/messages \
  -H "content-type: application/json" \
  -d '{
    "model": "claude-sonnet-4",
    "max_tokens": 64,
    "messages": [
      { "role": "user", "content": "Reply with exactly: proxy works" }
    ]
  }'
```

Example response:

```json
{
  "id": "msg_xxx",
  "type": "message",
  "role": "assistant",
  "model": "claude-sonnet-4",
  "content": [
    { "type": "text", "text": "proxy works" }
  ],
  "stop_reason": "end_turn",
  "stop_sequence": null,
  "usage": {
    "input_tokens": 41,
    "output_tokens": 3
  }
}
```

## Streaming Request

```bash
curl http://127.0.0.1:8787/v1/messages \
  -H "content-type: application/json" \
  -d '{
    "model": "claude-opus-4",
    "stream": true,
    "max_tokens": 64,
    "messages": [
      { "role": "user", "content": "Say hello" }
    ]
  }'
```

The response is `text/event-stream` using Anthropic-compatible event names such as:

- `message_start`
- `content_block_start`
- `content_block_delta`
- `content_block_stop`
- `message_delta`
- `message_stop`

## Tool Calling

`ANTHROPIC_PROXY_TOOL_FORMAT=xml` is the default. With tools present, the proxy injects XML tool instructions into the upstream system prompt and converts streamed `<tool_code name="...">...</tool_code>` output into Anthropic `tool_use` events. This mirrors Claude Adapter's fallback behavior for OpenAI-compatible models that do not reliably support native function calling.

Use `ANTHROPIC_PROXY_TOOL_FORMAT=native` to preserve OpenAI-compatible `tools` and `tool_choice` passthrough.

Model routing is described in [Model Routing](../../guides/model-routing.md).
