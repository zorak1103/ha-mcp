# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ha-mcp is a Model Context Protocol (MCP) server that provides AI assistants with access to Home Assistant. It uses a hybrid architecture: WebSocket for most operations, REST API fallback for delete operations (automations, scripts, scenes). Translates MCP tool calls into Home Assistant API commands.

## Build and Development Commands

```bash
# Build the binary
go build -o ha-mcp ./cmd/ha-mcp

# Run tests with race detection
go test -race ./...

# Run a single test
go test -v -race ./internal/handlers -run TestEntityHandlers

# Run linter (uses golangci-lint v2)
golangci-lint run ./...

# Check formatting
gofmt -l .

# Run security vulnerability scan
govulncheck ./...

# Initialize config files in current directory
./ha-mcp init

# Display effective configuration (tokens masked)
./ha-mcp config

# Run the server
./ha-mcp --ha-url http://homeassistant.local:8123 --ha-token YOUR_TOKEN
```

## Architecture

### Request Flow

```
AI Client (Claude, Cline)
    → HTTP POST / (JSON-RPC)
    → MCP Server (internal/mcp/server.go)
    → Tool Registry lookup (internal/mcp/registry.go)
    → Tool Handler (internal/handlers/*.go)
    → HybridClient (internal/homeassistant/hybrid_client.go)
        → WebSocket (most operations) OR REST API (delete operations)
    → Home Assistant API
```

### Key Packages

- **cmd/ha-mcp**: CLI entry point using Cobra, handles flags and signals
- **internal/mcp**: MCP protocol server, JSON-RPC handling, tool/resource registry
- **internal/homeassistant**: Hybrid client (WS + REST), WebSocket with auto-reconnect, REST for deletes
- **internal/handlers**: MCP tool handlers organized by domain (entities, automations, helpers, analysis, etc.)
- **internal/config**: Viper-based config loading (YAML → .env → ENV → CLI flags)
- **internal/logging**: Structured logging with DEBUG/INFO/WARN/ERROR/TRACE levels

### Handler Pattern

Each handler domain follows this pattern:
1. Create handler struct with `New*Handlers()` factory
2. Implement `RegisterTools(registry *mcp.Registry)` method
3. Register in `internal/handlers/register.go` via `RegisterAllTools()`

Tool handlers have signature:
```go
func(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error)
```

### Home Assistant Client Interface

The `homeassistant.Client` interface abstracts all HA operations. Implementation uses a hybrid approach:
- **HybridClient** (`hybrid_client.go`): Routes to WebSocket or REST based on operation type
- **WebSocket** (`ws_client_impl.go`): Persistent connection with auto-reconnect (1s → 60s backoff)
- **REST** (`rest_client.go`): Used for delete operations not reliably supported via WebSocket

Factory pattern in `factory.go` creates the appropriate client based on configuration.

### Configuration Priority

`CLI flags > ENV vars > .env file > config.yaml > defaults`

Key environment variables: `HA_URL`, `HA_TOKEN`, `HA_MCP_PORT`, `HA_MCP_LOG_LEVEL`
