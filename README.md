# ha-mcp

[![GitHub release](https://img.shields.io/github/v/release/zorak1103/ha-mcp)](https://github.com/zorak1103/ha-mcp/releases/latest)
[![License](https://img.shields.io/github/license/zorak1103/ha-mcp)](LICENSE)
[![CI](https://github.com/zorak1103/ha-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/zorak1103/ha-mcp/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/zorak1103/ha-mcp?style=flat)](https://goreportcard.com/report/github.com/zorak1103/ha-mcp)
[![Go Version](https://img.shields.io/github/go-mod/go-version/zorak1103/ha-mcp)](https://go.dev/)
[![Go Reference](https://pkg.go.dev/badge/github.com/zorak1103/ha-mcp.svg)](https://pkg.go.dev/github.com/zorak1103/ha-mcp)
[![Docker Hub](https://img.shields.io/docker/v/zorak1103/ha-mcp?label=Docker%20Hub&logo=docker)](https://hub.docker.com/r/zorak1103/ha-mcp)
[![Docker Pulls](https://img.shields.io/docker/pulls/zorak1103/ha-mcp?logo=docker)](https://hub.docker.com/r/zorak1103/ha-mcp)
[![Release](https://github.com/zorak1103/ha-mcp/actions/workflows/release.yml/badge.svg)](https://github.com/zorak1103/ha-mcp/actions/workflows/release.yml)
[![Renovate](https://github.com/zorak1103/ha-mcp/actions/workflows/renovate.yml/badge.svg)](https://github.com/zorak1103/ha-mcp/actions/workflows/renovate.yml)

A Model Context Protocol (MCP) server that provides AI assistants with access to Home Assistant, enabling smart home control and automation management.

## Features

- **40 Specialized Tools**: Entity queries, automation CRUD, helper management, scripts, scenes, devices, areas, labels, floors, zones, persons, tags, traces, blueprints, updates, todos, calendars, cameras, dashboards, system log, and more
- **Hybrid Architecture**: WebSocket for most operations, REST API for automation/script/scene CRUD
- **Complete CRUD**: Create, read, update, delete automations/scripts/scenes/helpers
- **Deep System Access**: Query registries, analyze dependencies, access logbook, validate config
- **Flexible Output**: Natural language (LLM-optimized) and JSON formats
- **Access Control**: Read-only mode, whitelist/blacklist, fine-grained action-level control
- **Auto-Reconnect**: Automatic reconnection with exponential backoff
- **Post-Mutation Confirmation**: Automatic state polling after create/update/delete confirms changes

## vs. Other MCP Servers for Home Assistant

Two alternatives exist: the [official HA MCP integration](https://www.home-assistant.io/integrations/mcp_server) (built-in, ~10 intent-based tools) and the community [homeassistant-ai/ha-mcp](https://github.com/homeassistant-ai/ha-mcp) (Python/FastMCP, 95+ tools).

Choose ha-mcp if you need:
- Full automation/script/scene/helper lifecycle management (create, edit, delete)
- Advanced analysis (dependencies, cross-references, automation coverage)
- System administration (registry queries, config validation, logbook, history)
- Media management (browser, camera streams), HACS, and dashboard access
- Reliable LLM tool selection — 41 consolidated tools reduce selection errors compared to 95+ fine-grained alternatives

Choose the official integration if you need entity-level security or no external infrastructure.

See [docs/feature-comparison.md](docs/feature-comparison.md) for a detailed three-way feature matrix.

## Installation

### From Binary

Download the latest release from the [Releases](../../releases) page.

```bash
# Linux/macOS
tar -xzf ha-mcp_linux_amd64.tar.gz
chmod +x ha-mcp
sudo mv ha-mcp /usr/local/bin/

# Windows: extract ha-mcp_windows_amd64.zip and add to PATH
```

### From Source

Requires Go 1.27 or later.

```bash
git clone https://github.com/zorak1103/ha-mcp.git
cd ha-mcp
task install-hooks  # install git pre-commit hook (auto-fixes gofmt on every commit)
task lint:install   # install golangci-lint built with your local Go toolchain
go build -o ha-mcp ./cmd/ha-mcp
```

### Linux Packages

RPM and DEB packages are available in the releases:

```bash
sudo dpkg -i ha-mcp_amd64.deb   # Debian/Ubuntu
sudo rpm -i ha-mcp_amd64.rpm    # RHEL/Fedora
```

### Docker

```bash
docker pull zorak1103/ha-mcp:latest
docker run -d --name ha-mcp -p 8080:8080 \
  -e HA_URL=http://homeassistant.local:8123 \
  zorak1103/ha-mcp:latest
```

See [docs/configuration.md](docs/configuration.md) for Docker options, HTTPS/WSS, proxy support, and all environment variables.

## Quick Start

1. Get a long-lived access token from your Home Assistant profile page.

2. Start the server:

```bash
# With flags
ha-mcp --ha-url http://homeassistant.local:8123 --ha-token your-token

# Or initialize config files first
ha-mcp init   # creates config.yaml and .env
ha-mcp        # start with config file
```

3. Connect your AI client. Example for Claude Desktop:

```json
{
  "mcpServers": {
    "homeassistant": {
      "type": "http",
      "url": "http://localhost:8080",
      "headers": { "Authorization": "Bearer your-ha-access-token" }
    }
  }
}
```

See [docs/configuration.md](docs/configuration.md) for Cline, opencode, and other client configurations.

### Available Commands

| Command         | Description                                      |
| --------------- | ------------------------------------------------ |
| `ha-mcp`        | Start the MCP server                             |
| `ha-mcp init`   | Create config.yaml and .env in current directory |
| `ha-mcp config` | Display effective configuration (tokens masked)  |
| `ha-mcp --help` | Show help and available flags                    |

## Available Tools

41 tools organized by domain. Full reference at [docs/tools.md](docs/tools.md).

Seven guidance topics are also available as MCP resources under `skill://ha-mcp/<slug>` URIs (format-selection, automation-patterns, template-resilience, helper-selection, dashboard-safety, entity-renaming, debugging-workflow).

| Category          | Count | Highlights                                                                  |
| ----------------- | ----- | --------------------------------------------------------------------------- |
| Entity            | 5     | `query_entities` (history/stats/health), `get_state`, `analyze_entity`      |
| Registry          | 10    | `get_registry`, `manage_area/label/floor/zone/person/tag/entity/device`     |
| Automation        | 1     | `manage_automation` (CRUD, toggle, coverage, JSON Patch + semantic patch)   |
| Helpers           | 2     | `manage_helper` (26 types), `helper_action`                                 |
| Scripts & Scenes  | 2     | `manage_script`, `manage_scene` (CRUD + execute/activate + JSON Patch + semantic patch) |
| Analysis          | 4     | `analyze_entity`, `get_entity_dependencies`, `analyze_target`, `find_references` |
| Services          | 2     | `call_service`, `list_services`                                             |
| History/Logbook   | 2     | `query_entities` modes, `get_logbook` (entries + correlation)               |
| Dashboards/Media  | 4     | `manage_dashboard` (JSON Patch + semantic patch), `browse_media`, `manage_camera`, `sign_media_path` |
| Calendars & Todos | 2     | `manage_calendar`, `manage_todo`                                            |
| System/Admin      | 7     | `get_system_info`, `validate_config`, `manage_update`, `manage_blueprint`   |
| Logs              | 1     | `manage_system_log` (list WARN/ERROR entries, clear ring buffer)            |
| HACS              | 1     | `manage_hacs` (list, download, install, custom repos)                       |
| Guidance          | 1     | `get_skill` (action=list to discover skills, action=read to fetch content)  |

## Access Control

ha-mcp provides read-only mode, whitelist, and blacklist filtering at the tool and action level:

```yaml
# config.yaml — read-only monitoring
server:
  read_only: true

# Or block specific operations
server:
  tool_filter:
    blacklist:
      - "call_service"
      - "manage_*:delete"
```

See [docs/access-control.md](docs/access-control.md) for glob patterns, category filtering (`*:write`), and example scenarios.

## Architecture

```
AI Client → HTTP/JSON-RPC → ha-mcp MCP Server
                                    │
               ┌────────────────────┴────────────────────┐
               │ WebSocket (primary)                      │ REST API
               │ - State queries, service calls           │ - Automation CRUD
               │ - Helper CRUD, Registry access           │ - Script/Scene CRUD
               └────────────────────┬────────────────────┘
                                    │
                             Home Assistant
```

See [docs/architecture.md](docs/architecture.md) for project structure, build commands, and integration test setup.

## Troubleshooting

See [docs/troubleshooting.md](docs/troubleshooting.md) for WebSocket connection issues, debug mode, and common error solutions.

## Development

**Prerequisites:** Go 1.27+, golangci-lint v2, Docker (optional)

```bash
go build -o ha-mcp ./cmd/ha-mcp    # Build
go test ./...                       # Unit tests
golangci-lint run --timeout=5m ./...  # Lint
```

> If `golangci-lint` panics with "file requires newer Go version", your locally installed binary was built with an older Go toolchain than the one on `PATH`. Run `task lint:install` to rebuild it against your current toolchain.

See [docs/architecture.md](docs/architecture.md) for integration test setup and [docs/integration-tests.md](docs/integration-tests.md) for the full test suite documentation.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full workflow (task commands, TDD
requirement, integration test setup, linter rules, and the docs checklist for
new tools). Please also read the [Code of Conduct](CODE_OF_CONDUCT.md).

## License

GPL-3.0 License - see [LICENSE](LICENSE) for details.

## Acknowledgments

- [Model Context Protocol](https://modelcontextprotocol.io/) specification
- [Home Assistant WebSocket API](https://developers.home-assistant.io/docs/api/websocket)
- [coder/websocket](https://github.com/coder/websocket) - Pure Go WebSocket library
