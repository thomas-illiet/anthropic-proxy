# Runtime Behavior

## Client Auth

Set `ANTHROPIC_PROXY_CLIENT_KEY` to require API callers to authenticate. `/`, `/health`, and `/metrics` remain unauthenticated by design.

```env
ANTHROPIC_PROXY_CLIENT_KEY=my-local-proxy-key
```

Clients can send either `x-api-key` or `Authorization: Bearer`.

## Request Size

Claude Code can send large histories and tool results. The default request body limit is `32 MiB`.

```env
ANTHROPIC_PROXY_MAX_REQUEST_BODY_BYTES=67108864
```

## Logging

Set `ANTHROPIC_PROXY_LOG_LEVEL` to control proxy verbosity:

```env
ANTHROPIC_PROXY_LOG_LEVEL=info
```

Supported levels are `trace`, `debug`, `info`, `warn`, `error`, and `off`.
Normal access logs are written at `info`, rejected requests and upstream HTTP
errors at `warn`, and internal or network failures at `error`. `debug` adds
request mapping and completion summaries. `trace` is reserved for the most
verbose stream parsing details.

## Prompt Caching Hints

By default, `cache_control` fields are accepted from Claude Code but stripped before forwarding because many OpenAI-compatible APIs reject unknown fields. Enable passthrough only when your upstream gateway supports Anthropic-style cache hints inside OpenAI-compatible payloads.

```env
ANTHROPIC_PROXY_FORWARD_CACHE_CONTROL=1
```

Cached token counts are returned when the upstream provides OpenAI-style `prompt_tokens_details.cached_tokens`.

## Tool Conversion

`ANTHROPIC_PROXY_TOOL_FORMAT=xml` is the default. In this mode, the proxy uses Claude Adapter-style XML tool fallback:

- Anthropic tool definitions are injected into the system prompt as XML instructions.
- Native OpenAI `tools` and `tool_choice` fields are not sent upstream.
- `temperature` is forced to `0`.
- Tool results are returned to the upstream model as `<tool_output>` blocks.
- Streaming `<tool_code name="...">...</tool_code>` output is converted back into Anthropic `tool_use` events.

Use native OpenAI-compatible tool calling only when your upstream model and provider handle function calls reliably:

```env
ANTHROPIC_PROXY_TOOL_FORMAT=native
```

## Anthropic API Headers

For Claude Adapter-style compatibility, `anthropic-version` and `anthropic-beta` are accepted but not validated, interpreted, or forwarded upstream.

## Upstream URL

`ANTHROPIC_PROXY_UPSTREAM_URL` is used as configured. The proxy does not apply a host allowlist, so keep `ANTHROPIC_PROXY_CLIENT_KEY` enabled if the service is reachable by anything other than your local machine.
