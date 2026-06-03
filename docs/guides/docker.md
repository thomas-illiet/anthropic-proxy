# Docker

## Build Image

```bash
docker build -t anthropic-proxy:local .
```

Run with variables from `.env`:

```bash
docker run --rm \
  --env-file .env \
  -p 8787:8787 \
  anthropic-proxy:local
```

The image default command is `anthropic-proxy serve`.

## Docker Compose

```bash
cp .env.example .env
docker compose up --build
```

The compose service publishes the proxy on `http://127.0.0.1:8787` and reads configuration from `.env`.

## Local Upstreams

Inside Docker, `localhost` means the proxy container, not your host machine. For local upstreams such as Ollama, LM Studio, or vLLM running on the host, use `host.docker.internal`:

```env
ANTHROPIC_PROXY_UPSTREAM_URL=http://host.docker.internal:11434/v1/chat/completions
ANTHROPIC_PROXY_DEFAULT_MODEL=qwen3-coder:30b
ANTHROPIC_PROXY_FORCE_MODEL=1
```

Set `ANTHROPIC_PROXY_UPSTREAM_API_KEY` only if your local gateway requires one.

The compose file includes `host.docker.internal:host-gateway` so this works on Linux as well as Docker Desktop.

## Claude Code

```bash
export ANTHROPIC_BASE_URL=http://localhost:8787
export ANTHROPIC_API_KEY=anything
export CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY=1
claude
```

If `ANTHROPIC_PROXY_CLIENT_KEY` is set in `.env`, use that value as `ANTHROPIC_API_KEY`.
