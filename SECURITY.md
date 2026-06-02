# Security Policy

## Supported Versions

Security fixes target the latest released version and the `main` branch.

## Reporting a Vulnerability

Please do not publish API keys, `.env` files, request logs with secrets, or exploitable vulnerability details in a public issue.

Prefer GitHub Security Advisories if they are enabled for this repository. If advisories are not available, open a minimal public issue that says you have a security report to share, without including secrets or exploit details.

Useful reports include:

- the affected version or commit;
- the configuration involved;
- the expected and actual behavior;
- minimal reproduction steps without real credentials.

## Handling Secrets

`anthropic-proxy` reads upstream and optional client keys from environment variables or a local `.env` file. Keep those files local, rotate exposed keys immediately, and avoid sharing logs that contain authorization headers.
