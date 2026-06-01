# anthropic-proxy

![anthropic-proxy banner](docs/assets/banner.svg)

`anthropic-proxy` exposes an Anthropic-compatible HTTP API and forwards requests to an OpenAI-compatible chat completions endpoint. It is built for two simple setups: a local proxy on a developer machine, or a shared proxy on a server for a team.

## Quick Start Local

```bash
cp .env.example .env
make build
./anthropic-proxy serve
```

Then point Claude Code at the local proxy:

```bash
export ANTHROPIC_BASE_URL=http://localhost:8787
export ANTHROPIC_API_KEY=anything
export CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY=1
claude
```

If `ANTHROPIC_PROXY_CLIENT_KEY` is set, use that value as `ANTHROPIC_API_KEY`.

## Docker

```bash
cp .env.example .env
docker compose up --build
```

The Docker image does not define default `ANTHROPIC_PROXY_*` environment variables. Provide configuration at runtime with Compose `env_file`, `environment`, or your orchestrator.

## CLI

```bash
anthropic-proxy serve
anthropic-proxy version
anthropic-proxy --help
```

The root command only prints help. The server starts only through `anthropic-proxy serve`.

Configuration is loaded once at startup from real `ANTHROPIC_PROXY_*` environment variables and an optional `.env` file in the current working directory. There is no `--config` flag and no config file argument.

## Documentation

- [Getting Started](docs/getting-started.md)
- [Server Deployment](docs/guides/server-deployment.md)
- [Configuration Reference](docs/reference/configuration.md)
- [API Reference](docs/reference/api.md)
- [Architecture](docs/architecture.md)
- [Changelog](CHANGELOG.md)
