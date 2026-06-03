# Windows

`anthropic-proxy` publishes a Windows archive for each tagged release.

## Install From Release

1. Download `anthropic-proxy_<version>_windows_amd64.zip` from [GitHub Releases](https://github.com/thomas-illiet/anthropic-proxy/releases/latest).
2. Extract the archive.
3. Copy `.env.example` to `.env`.
4. Edit `.env` and set `ANTHROPIC_PROXY_DEFAULT_MODEL`. Set `ANTHROPIC_PROXY_UPSTREAM_API_KEY` only if your upstream provider requires it.

Start the proxy from PowerShell:

```powershell
.\anthropic-proxy.exe serve
```

## Claude Code Environment

In PowerShell:

```powershell
$env:ANTHROPIC_BASE_URL = "http://localhost:8787"
$env:ANTHROPIC_API_KEY = "anything"
$env:CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY = "1"
claude
```

If `ANTHROPIC_PROXY_CLIENT_KEY` is configured in `.env`, use that exact value for `ANTHROPIC_API_KEY`.

## Build From Source

Install Go, then run:

```powershell
go build -trimpath -o anthropic-proxy.exe .
.\anthropic-proxy.exe version
.\anthropic-proxy.exe serve
```
