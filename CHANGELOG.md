# Changelog

## v1.0.0

Initial release candidate for the breaking-change v1 line.

### Breaking Changes

- The server starts only with `anthropic-proxy serve`.
- The root command prints help and never starts the server implicitly.
- `--config`, `-c`, and positional config file arguments were removed.
- Configuration is loaded only from real `ANTHROPIC_PROXY_*` environment variables and optional `.env` in the current working directory.
- Legacy unprefixed variables are ignored.
- CORS and special `OPTIONS` handling were removed.
- IP restriction and built-in rate limiting were removed.
- Anthropic compatibility headers are accepted implicitly, but are not validated or forwarded upstream.

### Added

- Cobra subcommands: `serve` and `version`.
- Viper-backed configuration.
- Graceful shutdown for `SIGINT` and `SIGTERM`.
- Server timeout configuration around the HTTP runtime.
- CI and release packaging for v1 artifacts.
