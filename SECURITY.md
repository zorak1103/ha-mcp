# Security Policy

## Supported versions

ha-mcp releases use calendar versioning (e.g. `v2026.8.3`) rather than a
maintained major-version line. Only the latest released version receives
security fixes; please upgrade before reporting an issue that may already be
fixed.

## Reporting a vulnerability

Please **do not** report security vulnerabilities through public GitHub issues
or discussions.

Instead, use GitHub's private reporting: go to the
[Security tab](https://github.com/zorak1103/ha-mcp/security) and click
**"Report a vulnerability"**. Include:

- a description of the issue and its impact
- the ha-mcp version and how it's deployed (Docker, binary, source)
- reproduction steps or a proof of concept

There's no bounty for reports. With the reporter's consent, we're happy to
credit them in the release notes.

## Security model

ha-mcp holds a Home Assistant long-lived access token and, depending on
configuration, can read and control every entity in a home. A few things worth
knowing before treating something as a vulnerability report vs. a deployment
question:

- The server does **not** implement its own authentication on the MCP HTTP
  endpoint. It is meant to run behind a trusted boundary (localhost, a
  reverse proxy with auth, or a private network) — binding it to `0.0.0.0`
  without a proxy in front is a deployment choice, not a bug in the server.
- `--read-only` and tool/action allow-/deny-listing
  (`HA_MCP_TOOL_FILTER_WHITELIST` / `HA_MCP_TOOL_FILTER_BLACKLIST`, see
  [docs/access-control.md](docs/access-control.md)) exist precisely to limit
  what an MCP client — trusted or not — can do through the server. If you're
  exposing ha-mcp to a client you don't fully trust, use them.
- The Home Assistant token itself is the highest-value secret in this system;
  a report showing it can leak (via logs, error messages, or an API response)
  is treated as high severity.
