# Contributing

Thanks for taking a look at `anthropic-proxy`. Contributions are welcome when they keep the proxy small, predictable, and easy to run with Claude Code.

## Local Setup

```bash
git clone https://github.com/thomas-illiet/anthropic-proxy.git
cd anthropic-proxy
cp .env.example .env
make build
go test ./...
```

Run the proxy locally with:

```bash
./anthropic-proxy serve
```

## Checks

Before opening a pull request, run the checks that match your change:

```bash
make fmt-check
make vet
make test
make build
```

If your branch changes Docker or release packaging, also run:

```bash
docker compose config --quiet
docker build -t anthropic-proxy:dev .
```

## Style

- Keep changes focused on one behavior or documentation improvement.
- Preserve the existing `ANTHROPIC_PROXY_*` configuration style.
- Do not commit local secrets, `.env` files, build artifacts, logs, or generated archives.
- Add or update tests for behavior changes in `internal/...`.
- Keep documentation examples copy-pasteable and aligned with `.env.example`.

## Pull Requests

Open a PR with:

- a short summary of the user-facing change;
- the commands you ran to verify it;
- notes about compatibility, configuration, or migration impact if any.

Small documentation fixes are very welcome.
