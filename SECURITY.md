# Security

## Reporting a vulnerability

Please report security problems privately through GitHub: on the repository's **Security** tab, choose **Report a vulnerability**. Don't open a public issue.

Include what you found, how to reproduce it, and the version (`litechat -version`). You'll get a reply within a week.

## Scope

liteChat is a single-user app protected by one password. Things that are expected and not vulnerabilities on their own:

- Anyone with the password can make the server send requests to provider and MCP URLs they enter.
- With `LITECHAT_MCP_STDIO=1`, anyone with the password can start local programs. That's why it's off by default.

See [docs/deployment.md](docs/deployment.md#security-checklist) for hardening advice.
