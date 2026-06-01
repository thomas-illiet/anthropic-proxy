# Model Routing

## Forced Model Mode

When `ANTHROPIC_PROXY_FORCE_MODEL=1`, every incoming model is replaced with `ANTHROPIC_PROXY_DEFAULT_MODEL`.

```env
ANTHROPIC_PROXY_DEFAULT_MODEL=z-ai/glm-5.1
ANTHROPIC_PROXY_FORCE_MODEL=1
```

Incoming request:

```json
{
  "model": "claude-sonnet-4",
  "max_tokens": 256,
  "messages": [
    { "role": "user", "content": "hello" }
  ]
}
```

Upstream request:

```json
{
  "model": "z-ai/glm-5.1",
  "messages": [
    { "role": "user", "content": "hello" }
  ]
}
```

## Mapping Mode

When `ANTHROPIC_PROXY_FORCE_MODEL=0`, the proxy uses:

1. exact match in `ANTHROPIC_PROXY_MODEL_MAP` for the incoming model, for example `sonnet`
2. exact match in `ANTHROPIC_PROXY_MODEL_MAP` for the resolved alias target, for example `claude-sonnet-4-6`
3. longest prefix match in `ANTHROPIC_PROXY_MODEL_MAP` for either value
4. `ANTHROPIC_PROXY_DEFAULT_MODEL` as fallback
5. the resolved model ID if no fallback is configured

```env
ANTHROPIC_PROXY_FORCE_MODEL=0
ANTHROPIC_PROXY_DEFAULT_MODEL=meta/llama-3.1-8b-instruct
ANTHROPIC_PROXY_MODEL_MAP={"claude-opus":"meta/llama-3.1-70b-instruct","claude-sonnet":"meta/llama-3.1-8b-instruct","claude-haiku":"nvidia/llama-3.1-nemotron-nano-8b-v1"}
```

## Claude Code Aliases

The proxy understands Claude Code family aliases. Incoming `sonnet`, `opus`, `haiku`, and `best` are normalized to the configured `ANTHROPIC_PROXY_DEFAULT_*_MODEL` IDs before prefix mapping.

Default aliases:

| Alias | Model ID |
|---|---|
| `sonnet` | `claude-sonnet-4-6` |
| `opus` | `claude-opus-4-8` |
| `haiku` | `claude-haiku-4-5` |
| `best` | `claude-opus-4-8` |

A Claude Code context suffix such as `[1m]` is stripped before upstream routing.

## Visible Model IDs

You can override the Anthropic-visible IDs returned by `/v1/models` without changing upstream routing:

```env
ANTHROPIC_PROXY_DEFAULT_SONNET_MODEL=claude-sonnet-4-6
ANTHROPIC_PROXY_DEFAULT_OPUS_MODEL=claude-opus-4-8
ANTHROPIC_PROXY_DEFAULT_HAIKU_MODEL=claude-haiku-4-5
ANTHROPIC_PROXY_DEFAULT_SONNET_MODEL_NAME=Claude Sonnet 4.6
ANTHROPIC_PROXY_DEFAULT_OPUS_MODEL_NAME=Claude Opus 4.8
ANTHROPIC_PROXY_DEFAULT_HAIKU_MODEL_NAME=Claude Haiku 4.5
```

Keep these as Anthropic-looking IDs when using Claude Code discovery. Upstream provider IDs belong in `ANTHROPIC_PROXY_MODEL_MAP` values.
