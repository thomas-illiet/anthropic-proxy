# Metrics Endpoint

`GET /metrics` exposes Prometheus metrics for proxy HTTP traffic and `/v1/messages` usage KPIs.

```bash
curl http://127.0.0.1:8787/metrics
```

This endpoint is intentionally not protected by `ANTHROPIC_PROXY_CLIENT_KEY`, so expose it only on a trusted network or behind your own scrape-layer access controls.

## Exported Metrics

| Metric | Labels | Description |
|---|---|---|
| `anthropic_proxy_http_requests_total` | `endpoint`, `method`, `status` | HTTP requests served by registered proxy routes |
| `anthropic_proxy_http_request_duration_seconds` | `endpoint`, `method`, `status` | HTTP request duration histogram |
| `anthropic_proxy_http_in_flight` | `endpoint`, `method` | Requests currently being handled |
| `anthropic_proxy_message_requests_total` | `requested_model`, `upstream_model`, `mode`, `result` | Converted `/v1/messages` requests by model, sync/stream mode, and result |
| `anthropic_proxy_message_tokens_total` | `requested_model`, `upstream_model`, `mode`, `type` | Successful `/v1/messages` token usage for `input`, `output`, `cache_read`, and `cache_creation` |

Local validation failures before Anthropic-to-OpenAI conversion appear in HTTP metrics only. Message request and token metrics start after request conversion, when the requested and upstream models are known.
