# Configuration Reference

`anthropic-proxy` uses Viper for configuration. It reads only:

1. Real environment variables named `ANTHROPIC_PROXY_*`.
2. An optional `.env` file in the current working directory.

Real environment variables override `.env`. Old unprefixed names are ignored. No external config file path is read.

## Required Minimum

```bash
ANTHROPIC_PROXY_DEFAULT_MODEL=your-upstream-model
```

`ANTHROPIC_PROXY_DEFAULT_MODEL` is required when `ANTHROPIC_PROXY_FORCE_MODEL=1`, which is the default.
`ANTHROPIC_PROXY_UPSTREAM_API_KEY` is optional and is only sent when configured.

## Variables

| Variable | Required | Default | Description |
|---|---:|---|---|
| `ANTHROPIC_PROXY_UPSTREAM_URL` | no | `https://api.openai.com/v1/chat/completions` | OpenAI-compatible chat completions endpoint. |
| `ANTHROPIC_PROXY_UPSTREAM_API_KEY` | no | unset | Optional bearer token sent to the upstream provider. |
| `ANTHROPIC_PROXY_DEFAULT_MODEL` | conditional | none | Upstream model used when forcing all requests to one model. |
| `ANTHROPIC_PROXY_TOOL_FORMAT` | no | `xml` | `xml` for prompt/XML tool fallback, `native` for OpenAI-compatible tools. |
| `ANTHROPIC_PROXY_FORCE_MODEL` | no | `1` | Force every incoming model to `ANTHROPIC_PROXY_DEFAULT_MODEL`. |
| `ANTHROPIC_PROXY_MODEL_MAP` | no | `{}` | JSON map from requested Claude/Anthropic model names to upstream model names. |
| `ANTHROPIC_PROXY_DEFAULT_SONNET_MODEL` | no | built-in Claude model ID | Anthropic-visible Sonnet model ID and `sonnet` alias target. |
| `ANTHROPIC_PROXY_DEFAULT_OPUS_MODEL` | no | built-in Claude model ID | Anthropic-visible Opus model ID and `opus`/`best` alias target. |
| `ANTHROPIC_PROXY_DEFAULT_HAIKU_MODEL` | no | built-in Claude model ID | Anthropic-visible Haiku model ID and `haiku` alias target. |
| `ANTHROPIC_PROXY_DEFAULT_SONNET_MODEL_NAME` | no | inferred | Display name returned by `/v1/models`. |
| `ANTHROPIC_PROXY_DEFAULT_OPUS_MODEL_NAME` | no | inferred | Display name returned by `/v1/models`. |
| `ANTHROPIC_PROXY_DEFAULT_HAIKU_MODEL_NAME` | no | inferred | Display name returned by `/v1/models`. |
| `ANTHROPIC_PROXY_CLIENT_KEY` | no | unset | Optional API key required from clients on protected proxy endpoints. |
| `ANTHROPIC_PROXY_REQUEST_TIMEOUT_SEC` | no | `600` | Per-request upstream timeout. |
| `ANTHROPIC_PROXY_MAX_REQUEST_BODY_BYTES` | no | `33554432` | Maximum request body size for Messages and token-count endpoints. |
| `ANTHROPIC_PROXY_FORWARD_CACHE_CONTROL` | no | `0` | Forward Anthropic `cache_control` hints to upstream payloads. |
| `ANTHROPIC_PROXY_LOG_LEVEL` | no | `info` | `trace`, `debug`, `info`, `warn`, `error`, or `off`. |
| `ANTHROPIC_PROXY_LISTEN_ADDR` | no | `:8787` | HTTP bind address. |

## Model Routing

With the default `ANTHROPIC_PROXY_FORCE_MODEL=1`, every incoming request uses `ANTHROPIC_PROXY_DEFAULT_MODEL`.

To map several client-facing model names:

```bash
ANTHROPIC_PROXY_FORCE_MODEL=0
ANTHROPIC_PROXY_MODEL_MAP={"claude-sonnet":"qwen/qwen3-coder","claude-haiku":"meta/llama-3.1-8b-instruct"}
```

If `ANTHROPIC_PROXY_FORCE_MODEL=0`, define either `ANTHROPIC_PROXY_DEFAULT_MODEL` or `ANTHROPIC_PROXY_MODEL_MAP`.

## Logging

Use `ANTHROPIC_PROXY_LOG_LEVEL=debug` instead of the removed legacy `DEBUG` variable.

Verbose levels can include upstream diagnostic details in logs. Do not enable them on shared servers unless you have a retention policy.
