# Troubleshooting

This guide covers common setup issues when running `anthropic-proxy` locally or in Docker.

## Claude Code Cannot Connect

Check that the proxy is running and listening on the expected address:

```bash
curl http://localhost:8787/health
```

Then confirm Claude Code points at the proxy:

```bash
export ANTHROPIC_BASE_URL=http://localhost:8787
export ANTHROPIC_API_KEY=anything
export CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY=1
```

If `ANTHROPIC_PROXY_CLIENT_KEY` is configured, `ANTHROPIC_API_KEY` must match that value.

## Upstream Authentication Fails

Check `.env` in the directory where you start the proxy:

```bash
ANTHROPIC_PROXY_UPSTREAM_API_KEY=replace-with-your-key
ANTHROPIC_PROXY_UPSTREAM_URL=https://your-provider.example/v1/chat/completions
```

Configuration is loaded once at startup. Restart the proxy after changing `.env` or real environment variables.

## Models Do Not Match Your Provider

Set the upstream model directly:

```bash
ANTHROPIC_PROXY_DEFAULT_MODEL=your-upstream-model
ANTHROPIC_PROXY_FORCE_MODEL=1
```

For per-model routing, set `ANTHROPIC_PROXY_FORCE_MODEL=0` and configure `ANTHROPIC_PROXY_MODEL_MAP`. See [Model Routing](model-routing.md).

## Tool Calls Behave Poorly

The default `ANTHROPIC_PROXY_TOOL_FORMAT=xml` mode is designed for broad compatibility with upstream models that do not reliably support native function calling.

Try native tool passthrough only when your provider and model support OpenAI-compatible tools well:

```bash
ANTHROPIC_PROXY_TOOL_FORMAT=native
```

## Docker Starts But The Proxy Is Misconfigured

The Docker image does not bake in default `ANTHROPIC_PROXY_*` values. Pass configuration through Compose `env_file`, Compose `environment`, or your orchestrator. See [Docker](docker.md).
