# API Reference

`anthropic-proxy` exposes a compact Anthropic-compatible API.

## Endpoints

| Method | Path | Auth | Description |
|---|---|---:|---|
| `POST` | `/v1/messages` | optional client key | Anthropic-compatible Messages endpoint. |
| `POST` | `/v1/messages/count_tokens` | optional client key | Local token estimate for Anthropic-compatible requests. |
| `GET` | `/v1/models` | optional client key | Anthropic-compatible model discovery. |
| `GET` | `/health` | public | Health check, returns `ok`. |
| `GET` | `/metrics` | public | Prometheus metrics. |
| `GET` | `/` | public | Non-secret runtime configuration summary. |

The proxy is designed for local tools and server-side clients; it does not provide browser preflight handling.

## Headers

Incoming Anthropic compatibility headers such as `anthropic-version` and `anthropic-beta` are accepted implicitly by the HTTP server. They are not validated and are not forwarded upstream.

If `ANTHROPIC_PROXY_CLIENT_KEY` is set, protected endpoints require:

```http
x-api-key: your-proxy-client-key
```

## Errors

Errors use Anthropic-style JSON:

```json
{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "model and max_tokens are required"
  }
}
```

Common error types include `authentication_error`, `invalid_request_error`, `not_found_error`, and `api_error`. Upstream provider errors are mapped to the closest Anthropic-style shape.

## Tools

`ANTHROPIC_PROXY_TOOL_FORMAT=xml` converts Anthropic tools into XML prompt instructions and parses XML tool calls from streamed text. `ANTHROPIC_PROXY_TOOL_FORMAT=native` forwards OpenAI-compatible `tools` and `tool_choice` fields upstream.
