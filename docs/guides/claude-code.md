# Claude Code Guide

## Connect Claude Code

```bash
export ANTHROPIC_BASE_URL=http://localhost:8787
export ANTHROPIC_API_KEY=anything
claude
```

If `ANTHROPIC_PROXY_CLIENT_KEY` is set, use that value as `ANTHROPIC_API_KEY`.

## Model Discovery

`GET /v1/models` returns the configured `sonnet`, `opus`, and `haiku` model IDs in Anthropic's list shape. Claude Code only imports discovered IDs that begin with `claude` or `anthropic`, so the proxy exposes Anthropic-visible IDs and maps them to upstream IDs separately.

```bash
export ANTHROPIC_BASE_URL=http://localhost:8787
export ANTHROPIC_API_KEY=my-local-proxy-key
export CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY=1
claude
```

With `ANTHROPIC_PROXY_FORCE_MODEL=1`, Claude Code can still discover the visible models, but every request routes to `ANTHROPIC_PROXY_DEFAULT_MODEL`. Use `ANTHROPIC_PROXY_FORCE_MODEL=0` plus `ANTHROPIC_PROXY_MODEL_MAP` when you want `sonnet`, `opus`, and `haiku` to hit different upstream models.

## Auth Headers

Claude Code sends `ANTHROPIC_API_KEY` as the Anthropic client key. The proxy accepts either:

```http
x-api-key: my-local-proxy-key
```

or:

```http
Authorization: Bearer my-local-proxy-key
```

See [Model Routing](model-routing.md) for alias and `ANTHROPIC_PROXY_MODEL_MAP` behavior.
