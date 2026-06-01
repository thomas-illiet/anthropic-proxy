# Models Endpoint

`GET /v1/models` returns Anthropic-compatible model discovery data for Claude Code and other Anthropic clients.

```bash
curl http://127.0.0.1:8787/v1/models \
  -H "anthropic-version: 2023-06-01"
```

Example response:

```json
{
  "data": [
    {
      "id": "claude-sonnet-4-6",
      "type": "model",
      "display_name": "Claude Sonnet 4.6",
      "created_at": "1970-01-01T00:00:00Z"
    },
    {
      "id": "claude-opus-4-8",
      "type": "model",
      "display_name": "Claude Opus 4.8",
      "created_at": "1970-01-01T00:00:00Z"
    },
    {
      "id": "claude-haiku-4-5",
      "type": "model",
      "display_name": "Claude Haiku 4.5",
      "created_at": "1970-01-01T00:00:00Z"
    }
  ],
  "first_id": "claude-sonnet-4-6",
  "last_id": "claude-haiku-4-5",
  "has_more": false
}
```

The endpoint exposes Anthropic-visible model IDs from the configured `sonnet`, `opus`, and `haiku` aliases, plus Claude-looking `ANTHROPIC_PROXY_MODEL_MAP` keys. It does not expose upstream model map values such as `meta/...` or `Qwen/...`.

Supported query parameters:

- `limit`: page size, default `20`, range `1..1000`
- `after_id`: return models after this model ID
- `before_id`: return models before this model ID

See [Model Routing](../../guides/model-routing.md) for alias configuration.
