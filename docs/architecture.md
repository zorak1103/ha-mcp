> [README](../README.md) | [Configuration](configuration.md) | [Tools](tools.md) | [Access Control](access-control.md) | [Architecture](architecture.md) | [Troubleshooting](troubleshooting.md) | [Feature Comparison](feature-comparison.md) | [Integration Tests](integration-tests.md)

# Architecture & Project Structure

## Hybrid Communication

ha-mcp uses a hybrid approach combining WebSocket and REST APIs:

- **WebSocket (primary)**: Used for most operations including state queries, service calls, helper management (via `manage_helper`/`helper_action`), and registry access
- **REST API**: Used for automation/script/scene CRUD operations, template rendering, logbook, and config validation

```
┌─────────────┐     HTTP/JSON-RPC      ┌─────────────┐
│  AI Client  │ ◄──────────────────────► │   ha-mcp    │
│  (Claude,   │                         │  MCP Server │
│   Cline)    │                         │             │
└─────────────┘                         └──────┬──────┘
                                               │
                           ┌───────────────────┴───────────────────┐
                           │                                       │
                           │ WebSocket (primary)                   │ REST API
                           │ ws://host/api/websocket               │ http://host/api/...
                           │ - State queries, service calls        │ - Automation CRUD
                           │ - Helper CRUD (manage_helper)         │ - Script CRUD
                           │ - Registry access                     │ - Scene CRUD
                           │                                       │
                           └───────────────┬───────────────────────┘
                                           │
                                    ┌──────▼──────┐
                                    │    Home     │
                                    │  Assistant  │
                                    └─────────────┘
```

## Message Flow

1. AI client sends JSON-RPC request to ha-mcp
2. ha-mcp routes to WebSocket (most operations) or REST API (automation/script/scene CRUD)
3. Home Assistant processes and responds
4. ha-mcp returns result to AI client

## API Limitations

Some Home Assistant operations have limitations:

- **Scripts**: REST API works, but `script.reload` service call is needed after create/update for entity to appear immediately
- **Automations/Scenes**: REST API stores config, but entity may not appear until Home Assistant restart or reload
- **Config Entry helpers**: Template, threshold, derivative, integration, and group helpers use HTTP-based Config Entry Flow (automatically handled by ha-mcp)
- **Entity/Device Registry CREATE operations**: `get_registry` and `query_devices` support UPDATE and DELETE but not CREATE. This is by design—entities and devices are created by integrations (Zigbee, Z-Wave, MQTT, etc.), not manually. Creating registry entries without a backing integration produces orphaned entries that cause errors. UPDATE operations (renaming, area assignment, icons, device class, enable/disable) are safe as they modify metadata only. DELETE operations are essential for cleaning up stale/orphaned entries via `query_entities` health mode and `query_devices` health mode.

## Project Structure

```
ha-mcp/
├── cmd/
│   └── ha-mcp/
│       └── main.go              # Application entry point
├── internal/
│   ├── config/
│   │   └── config.go            # Configuration handling
│   ├── homeassistant/
│   │   ├── client.go            # Client interface (~70 methods)
│   │   ├── cached_client.go     # TTL-based caching decorator
│   │   ├── factory.go           # Client factory (creates HybridClient)
│   │   ├── hybrid_client.go     # Hybrid client combining WS + REST
│   │   ├── rest_client.go       # REST client for automation/script/scene CRUD
│   │   ├── retry.go             # Retry logic with exponential backoff
│   │   ├── ws_client.go         # WebSocket connection management
│   │   ├── ws_client_impl.go    # WebSocket Client implementation
│   │   ├── ws_messages.go       # WebSocket message types
│   │   ├── ws_reconnect.go      # Reconnection logic
│   │   ├── entity_poller.go     # Entity state polling for post-mutation confirmation
│   │   ├── client_pool.go       # Per-token client pool with LRU eviction (max 100 entries)
│   │   └── types.go             # Data types
│   ├── mcp/
│   │   ├── server.go            # MCP HTTP server; per-IP rate limiting middleware
│   │   ├── registry.go          # Tool registry
│   │   └── types.go             # MCP protocol types
│   ├── handlers/
│   │   ├── formatter/           # Output formatters (natural/json)
│   │   │   ├── formatter.go     # Formatter interfaces
│   │   │   ├── natural.go       # LLM-optimized natural language output
│   │   │   ├── json.go          # Structured JSON output
│   │   │   ├── registry.go      # Registry formatters
│   │   │   ├── target.go        # Target analysis formatters
│   │   │   ├── automations.go   # Automation list/get formatters
│   │   │   ├── scripts.go       # Script list/get formatters
│   │   │   ├── scenes.go        # Scene list/get formatters
│   │   │   └── helpers.go       # Helper list/get_details formatters
│   │   ├── integration/         # Integration tests (build tag: integration)
│   │   │   ├── helpers.go       # Test ID generation, validation
│   │   │   ├── cleanup.go       # Cleanup utilities
│   │   │   ├── suite_test.go    # Base test suite
│   │   │   └── *_integration_test.go  # Domain-specific tests
│   │   ├── analysis_snapshot.go # Parallel data fetching for analysis
│   │   ├── entities.go          # Entity tool handlers (get_state)
│   │   ├── analysis.go          # Analysis tool handlers (analyze_entity, get_entity_dependencies)
│   │   ├── analysis_paths.go    # RFC 6901 path walker for automation/script reference locations
│   │   ├── config_search.go     # Shared reference-search scanners (dashboards, template helpers)
│   │   ├── find_references.go   # Cross-config find_references tool (automations/scripts/scenes/dashboards/helpers)
│   │   ├── entities_consolidated.go # Consolidated query_entities tool (current/history/statistics/domains/presence/health)
│   │   ├── entities_presence.go # Presence analysis for query_entities (mode=presence)
│   │   ├── entities_health.go   # Health mode: detect and remove dead entities (mode=health)
│   │   ├── devices_consolidated.go # Consolidated query_devices tool (health)
│   │   ├── devices_health.go    # Device health: detect and remove problematic devices
│   │   ├── automations.go       # Consolidated manage_automation tool
│   │   ├── automations_coverage.go # Automation coverage analysis
│   │   ├── helpers_consolidated.go  # manage_helper and helper_action tools
│   │   ├── scripts.go           # Consolidated manage_script tool
│   │   ├── scenes.go            # Consolidated manage_scene tool
│   │   ├── areas.go             # Consolidated manage_area tool
│   │   ├── labels.go            # Consolidated manage_label tool
│   │   ├── floors.go            # Consolidated manage_floor tool
│   │   ├── zones.go             # Consolidated manage_zone tool
│   │   ├── persons.go           # Consolidated manage_person tool
│   │   ├── tags.go              # Consolidated manage_tag tool
│   │   ├── registry.go          # Registry helper functions
│   │   ├── registry_consolidated.go # Consolidated get_registry tool
│   │   ├── media.go             # Media tool handlers
│   │   ├── lovelace.go          # Lovelace tool handler
│   │   ├── targets_consolidated.go  # Consolidated analyze_target tool
│   │   ├── services.go          # Service discovery handler
│   │   ├── system.go            # System info handler
│   │   ├── datetime.go          # Date/time handler
│   │   ├── templates.go         # Template rendering handler
│   │   ├── logbook.go           # Logbook access handler (entries/correlation modes)
│   │   ├── logbook_correlation.go # Logbook correlation analysis
│   │   ├── system_log.go        # System log handler (manage_system_log: list/clear)
│   │   ├── config.go            # Configuration validation handler
│   │   ├── hacs.go              # HACS (Community Store) management handler
│   │   ├── traces.go            # Trace viewing handler (automation/script execution traces)
│   │   ├── blueprints.go        # Blueprint management handler (list/import)
│   │   ├── updates.go           # Update management handler (list/install/skip)
│   │   ├── todos.go             # Todo list management handler (list/get_items/add/update/remove)
│   │   ├── calendars.go         # Calendar management handler (list/get_events/create_event/delete_event)
│   │   ├── cameras.go           # Camera handler (snapshot/stream)
│   │   ├── waiter.go             # Post-mutation wait utilities (state diff detection)
│   │   ├── skill.go             # get_skill tool + RegisterAllResources (7 skill:// resources)
│   │   ├── skills/              # Embedded skill markdown content + catalog (go:embed)
│   │   │   └── catalog.go       # Skill catalog; add slug here + .md file when adding a skill
│   │   └── register.go          # Handler registration
│   └── logging/
│       └── logger.go            # Structured logging
├── configs/
│   ├── config.example.yaml      # Example configuration
│   └── .env.example             # Example environment file
├── docs/
│   ├── access-control.md        # Access control & tool filtering
│   ├── architecture.md          # This file
│   ├── configuration.md         # Configuration reference
│   ├── feature-comparison.md    # ha-mcp vs official HA MCP comparison
│   ├── integration-tests.md     # Integration test documentation
│   ├── tools.md                 # Tools reference & examples
│   └── troubleshooting.md       # Troubleshooting guide
├── scripts/
│   └── check-coverage.sh        # Per-file 80% coverage enforcement
├── Dockerfile                   # Container build
├── Taskfile.yml                 # Build automation (task commands)
├── .golangci.yml               # Linter configuration
└── README.md                   # Project overview and quick start
```

## Development

### Prerequisites

- Go 1.26+
- golangci-lint v2
- [Task](https://taskfile.dev/#/installation) (build automation)
- Docker (for container builds)

### Build Commands

```bash
# List all available tasks
task --list

# Build binary
task build

# Run unit tests
task test

# Run linter
task lint

# Format check / auto-fix
task fmt
task fmt:fix

# Vulnerability check
task vulncheck

# Tests with race detector and per-file 80% coverage enforcement
task test:coverage
```

### Integration Tests

Integration tests verify the MCP server against a real Home Assistant instance. They are isolated using a unique `__mcptest_` prefix for all created entities.

**Prerequisites:**
- Running Home Assistant instance (version 2023.1+)
- Long-lived access token with full API access

**Configuration:**
```bash
export HA_INTEGRATION_TEST_URL=http://homeassistant.local:8123
export HA_INTEGRATION_TEST_TOKEN=<your-long-lived-access-token>
export HA_INTEGRATION_TEST_TIMEOUT=5m  # optional
```

**Running integration tests:**
```bash
# Run all integration tests
task test:integration

# Run specific test suite directly
go test -tags=integration -v ./internal/handlers/integration/... -run TestCounterIntegration
```

**Safety guarantees:**
- All test entities use `__mcptest_` prefix to avoid conflicts
- Pre-test cleanup removes leftover entities from previous runs
- Post-test verification ensures no test data remains
- Tests are skipped automatically if environment variables are not set

See [docs/integration-tests.md](integration-tests.md) for detailed documentation.
