# Health and Introspection

## Health Check

```bash
curl http://127.0.0.1:8787/health
```

Expected:

```text
ok
```

## Introspection

```bash
curl http://127.0.0.1:8787/
```

Example response:

```json
{
  "service": "anthropic-proxy",
  "upstream": "https://integrate.api.nvidia.com/v1/chat/completions",
  "default_model": "z-ai/glm-5.1",
  "tool_format": "xml",
  "force_model": true,
  "models": {},
  "model_aliases": {
    "sonnet": "claude-sonnet-4-6",
    "opus": "claude-opus-4-8",
    "haiku": "claude-haiku-4-5",
    "best": "claude-opus-4-8"
  },
  "request_timeout_sec": 600,
  "max_request_body_bytes": 33554432
}
```
