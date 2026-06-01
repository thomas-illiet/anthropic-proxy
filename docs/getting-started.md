# Getting Started

This guide runs `anthropic-proxy` locally on a developer machine.

## Install

Build from source:

```bash
make build
./anthropic-proxy version
```

Run the server:

```bash
./anthropic-proxy serve
```

The root command does not start the server:

```bash
./anthropic-proxy
```

## Configure

Create `.env` in the directory where you run the binary:

```bash
cp .env.example .env
```

Edit at least:

```bash
ANTHROPIC_PROXY_UPSTREAM_API_KEY=replace-with-your-key
ANTHROPIC_PROXY_DEFAULT_MODEL=your-upstream-model
```

Configuration precedence is:

1. Real environment variables.
2. `.env` in the current working directory.
3. Built-in defaults for optional values.

No other config file path is read.

## Claude Code

Point Claude Code at the proxy:

```bash
export ANTHROPIC_BASE_URL=http://localhost:8787
export ANTHROPIC_API_KEY=anything
export CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY=1
claude
```

If you set `ANTHROPIC_PROXY_CLIENT_KEY`, use that exact value for `ANTHROPIC_API_KEY`.

## Check

```bash
curl http://localhost:8787/health
curl http://localhost:8787/v1/models
```

Next, read [Configuration Reference](reference/configuration.md) and [API Reference](reference/api.md).
