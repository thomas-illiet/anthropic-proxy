# Server Deployment

Use this guide when one proxy instance is shared by several developers or automation jobs.

## Recommended Shape

Run `anthropic-proxy` behind your normal server boundary:

- TLS and public DNS handled by a reverse proxy or load balancer.
- `ANTHROPIC_PROXY_CLIENT_KEY` set for shared access.
- Provider credentials injected by the service manager, Compose, or orchestrator when your upstream requires them.
- `/health`, `/metrics`, and `/` left public at the application layer.

The proxy does not include IP restriction, built-in request throttling, or browser-facing policy. Put network policy, rate limits, and browser-facing behavior in your reverse proxy or platform if you need them.

## Binary

```bash
ANTHROPIC_PROXY_DEFAULT_MODEL=your-upstream-model \
ANTHROPIC_PROXY_CLIENT_KEY=shared-client-key \
ANTHROPIC_PROXY_LISTEN_ADDR=:8787 \
./anthropic-proxy serve
```

Add `ANTHROPIC_PROXY_UPSTREAM_API_KEY=replace-with-your-key` for providers that require upstream authentication.

For systemd or another process manager, set the same environment variables in the unit and use:

```bash
ExecStart=/usr/local/bin/anthropic-proxy serve
```

## Docker Compose

```bash
cp .env.example .env
docker compose up -d --build
```

The image starts `anthropic-proxy serve` by default. It does not define `ANTHROPIC_PROXY_*` defaults inside the Dockerfile.

Compose loads `.env` as runtime configuration through `env_file`. You can override values with `environment` or your deployment platform.

## Metrics

Prometheus can scrape:

```text
http://your-host:8787/metrics
```

Keep metrics public only on trusted networks. If your server is internet-facing, protect `/metrics` at the reverse proxy.

## Client Setup

For each user:

```bash
export ANTHROPIC_BASE_URL=https://proxy.example.com
export ANTHROPIC_API_KEY=shared-client-key
export CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY=1
```

Rotate `ANTHROPIC_PROXY_CLIENT_KEY` if it is shared outside the intended group.
