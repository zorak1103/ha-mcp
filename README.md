# ha-mcp

[![CI](https://github.com/zorak1103/ha-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/zorak1103/ha-mcp/actions/workflows/ci.yml)
[![Release](https://github.com/zorak1103/ha-mcp/actions/workflows/release.yml/badge.svg)](https://github.com/zorak1103/ha-mcp/actions/workflows/release.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/zorak1103/ha-mcp)](https://go.dev/)
[![License](https://img.shields.io/github/license/zorak1103/ha-mcp)](LICENSE)

A Model Context Protocol (MCP) server that provides AI assistants with access to Home Assistant via WebSocket, enabling smart home control and automation management.

## Features

- **WebSocket-Only Architecture**: Native Home Assistant WebSocket API for real-time, bidirectional communication
- **Entity Management**: Read and control all Home Assistant entities
- **Registry Access**: Query entity, device, and area registries
- **Automation CRUD**: Create, read, update, and delete automations
- **Helper Management**: Full support for all 14 Home Assistant helper types with CRUD operations
- **Script & Scene Control**: Full CRUD operations for scripts and scenes
- **Service Calls**: Execute any Home Assistant service
- **History & Statistics**: Query entity state history and recorder statistics
- **Media Browser**: Browse media sources and get camera streams
- **Lovelace Config**: Access dashboard configurations
- **Auto-Reconnect**: Automatic reconnection with exponential backoff

## Installation

### From Binary

Download the latest release for your platform from the [Releases](../../releases) page.

```bash
# Linux/macOS
tar -xzf ha-mcp_linux_amd64.tar.gz
chmod +x ha-mcp
sudo mv ha-mcp /usr/local/bin/

# Windows
# Extract ha-mcp_windows_amd64.zip and add to PATH
```

### From Source

Requires Go 1.25 or later.

```bash
git clone https://github.com/zorak1103/ha-mcp.git
cd ha-mcp
go build -o ha-mcp ./cmd/ha-mcp
```

### Using Docker

> **Note:** Docker images are currently not published. Build locally using the Dockerfile.

```bash
# Build Docker image locally
docker build -t ha-mcp:latest .

# Run container
docker run -d \
  --name ha-mcp \
  -p 8080:8080 \
  -e HA_URL=http://homeassistant.local:8123 \
  -e HA_TOKEN=your-long-lived-access-token \
  ha-mcp:latest
```

### Linux Packages

RPM and DEB packages are available in the releases:

```bash
# Debian/Ubuntu
sudo dpkg -i ha-mcp_amd64.deb

# RHEL/Fedora
sudo rpm -i ha-mcp_amd64.rpm
```

## Configuration

ha-mcp supports configuration via YAML file, environment variables, or command-line flags.

### Connection Requirements

ha-mcp connects to Home Assistant via **WebSocket** (`ws://{host}/api/websocket` or `wss://{host}/api/websocket` for HTTPS). Ensure:

- Home Assistant is running and accessible
- WebSocket connections are allowed (default in HA)
- The URL points to your Home Assistant instance (HTTP/HTTPS URL is converted to WebSocket internally)
- A valid long-lived access token is configured

### HTTPS/WSS Support

ha-mcp fully supports secure connections. The URL scheme is automatically converted:

| Input URL Scheme | WebSocket Scheme |
|-----------------|------------------|
| `http://`       | `ws://`          |
| `https://`      | `wss://`         |
| `ws://`         | `ws://`          |
| `wss://`        | `wss://`         |

**Example configurations for secure connections:**

```yaml
# config.yaml with HTTPS
homeassistant:
  url: "https://homeassistant.example.com"  # Converted to wss://
  token: "your-long-lived-access-token"
```

```bash
# Environment variables with HTTPS
export HA_URL=https://homeassistant.example.com
export HA_TOKEN=your-long-lived-access-token
```

```bash
# Command-line with HTTPS
ha-mcp --ha-url https://homeassistant.example.com --ha-token your-token
```

**Important notes for HTTPS/WSS:**

1. **SSL/TLS Certificates**: The system's certificate store is used for validation. Self-signed certificates may require additional configuration on the host system.

2. **Reverse Proxy Setup**: When using a reverse proxy (nginx, Traefik, Caddy), ensure WebSocket upgrade headers are properly forwarded:
   ```nginx
   # nginx example
   location /api/websocket {
       proxy_pass http://homeassistant:8123;
       proxy_http_version 1.1;
       proxy_set_header Upgrade $http_upgrade;
       proxy_set_header Connection "upgrade";
       proxy_set_header Host $host;
   }
   ```

3. **Home Assistant Cloud (Nabu Casa)**: For remote access via Nabu Casa, use your unique URL:
   ```yaml
   homeassistant:
     url: "https://your-instance.ui.nabu.casa"
     token: "your-long-lived-access-token"
   ```

### Proxy Support

ha-mcp supports HTTP/HTTPS proxies via standard environment variables. The underlying WebSocket library (`coder/websocket`) uses Go's standard HTTP client, which automatically respects these proxy settings.

**Supported environment variables:**

| Variable | Description |
|----------|-------------|
| `HTTP_PROXY` | Proxy for HTTP connections (e.g., `http://proxy:8080`) |
| `HTTPS_PROXY` | Proxy for HTTPS connections (e.g., `http://proxy:8080`) |
| `NO_PROXY` | Comma-separated list of hosts to bypass proxy |

**Example usage:**

```bash
# Set proxy environment variables
export HTTP_PROXY=http://proxy.example.com:8080
export HTTPS_PROXY=http://proxy.example.com:8080
export NO_PROXY=localhost,127.0.0.1

# Start ha-mcp (will use proxy for Home Assistant connection)
ha-mcp --ha-url https://homeassistant.example.com --ha-token your-token
```

**Docker with proxy:**

```bash
docker run -d \
  --name ha-mcp \
  -p 8080:8080 \
  -e HA_URL=https://homeassistant.example.com \
  -e HA_TOKEN=your-token \
  -e HTTP_PROXY=http://proxy.example.com:8080 \
  -e HTTPS_PROXY=http://proxy.example.com:8080 \
  ha-mcp:latest
```

**Notes:**

- Proxy authentication is supported via URL format: `http://user:password@proxy:8080`
- SOCKS5 proxies are supported: `socks5://proxy:1080`
- For WebSocket connections over HTTPS (wss://), the `HTTPS_PROXY` variable is used
- Ensure the proxy allows WebSocket upgrade requests (HTTP 101 Switching Protocols)

### Configuration File

Create a config file at one of these locations:
- `./config.yaml`
- `./configs/config.yaml`
- `$HOME/.config/ha-mcp/config.yaml`
- `/etc/ha-mcp/config.yaml`

```yaml
homeassistant:
  url: "http://homeassistant.local:8123"  # WebSocket URL derived automatically
  token: "your-long-lived-access-token"

server:
  host: "0.0.0.0"
  port: 8080

logging:
  level: "info"  # debug, info, warn, error
```

### Environment Variables

```bash
export HA_URL=http://homeassistant.local:8123
export HA_TOKEN=your-long-lived-access-token
export SERVER_HOST=0.0.0.0
export SERVER_PORT=8080
export LOG_LEVEL=info
```

### Command-Line Flags

```bash
ha-mcp \
  --ha-url http://homeassistant.local:8123 \
  --ha-token your-long-lived-access-token \
  --host 0.0.0.0 \
  --port 8080 \
  --log-level info
```

### Getting a Home Assistant Token

1. Open Home Assistant web interface
2. Click on your profile (bottom left)
3. Scroll to "Long-Lived Access Tokens"
4. Click "Create Token"
5. Give it a name (e.g., "ha-mcp")
6. Copy the token (it won't be shown again!)

## Usage

### Quick Start

```bash
# Initialize configuration files in current directory
ha-mcp init

# Edit the generated config.yaml or .env with your settings
# Then verify your configuration
ha-mcp config

# Start the server
ha-mcp
```

### Available Commands

| Command | Description |
|---------|-------------|
| `ha-mcp` | Start the MCP server |
| `ha-mcp init` | Create config.yaml and .env in current directory |
| `ha-mcp config` | Display effective configuration (tokens masked) |
| `ha-mcp --help` | Show help and available flags |

### Starting the Server

```bash
# With config file (default: ./config.yaml)
ha-mcp

# With environment variables
HA_URL=http://homeassistant.local:8123 HA_TOKEN=xxx ha-mcp

# With flags
ha-mcp --ha-url http://homeassistant.local:8123 --ha-token xxx
```

### Using with Cline

Add to your Cline MCP configuration (`~/.config/cline/mcp.json`):

```json
{
  "servers": {
    "ha-mcp": {
      "url": "http://localhost:8080/mcp",
      "description": "Home Assistant MCP Server"
    }
  }
}
```

### Using with Claude Desktop

Add to Claude Desktop's MCP configuration:

```json
{
  "mcpServers": {
    "homeassistant": {
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

### Using with opencode

Configure in your opencode settings:

```yaml
mcp:
  servers:
    - name: homeassistant
      url: http://localhost:8080/mcp
```

## API Reference

### MCP Endpoint

All MCP requests are sent to:

```
POST http://localhost:8080/mcp
Content-Type: application/json
```

### Available Tools

#### Entity Tools

| Tool | Description |
|------|-------------|
| `get_states` | List all entity states |
| `get_state` | Get state of a specific entity |
| `get_history` | Get historical states of an entity |
| `list_domains` | List available domains |

#### Registry Tools

| Tool | Description |
|------|-------------|
| `list_entity_registry` | List all entities in the registry with metadata |
| `list_device_registry` | List all devices with manufacturer, model info |
| `list_area_registry` | List all areas/rooms defined in Home Assistant |

#### Automation Tools

| Tool | Description |
|------|-------------|
| `list_automations` | List all automations |
| `get_automation` | Get automation details |
| `create_automation` | Create a new automation |
| `update_automation` | Update an existing automation |
| `delete_automation` | Delete an automation |
| `toggle_automation` | Enable/disable an automation |

#### Helper Tools

ha-mcp provides comprehensive support for all 14 Home Assistant helper types. Each helper type has its own dedicated tools.

##### Generic Helper Tools

| Tool | Description |
|------|-------------|
| `list_helpers` | List all helpers across all types |

##### Input Boolean

| Tool | Description |
|------|-------------|
| `create_input_boolean` | Create an input_boolean toggle |
| `delete_input_boolean` | Delete an input_boolean |
| `toggle_input_boolean` | Toggle an input_boolean on/off |

##### Input Number

| Tool | Description |
|------|-------------|
| `create_input_number` | Create an input_number with min/max/step |
| `delete_input_number` | Delete an input_number |
| `set_input_number_value` | Set the value of an input_number |

##### Input Text

| Tool | Description |
|------|-------------|
| `create_input_text` | Create an input_text field |
| `delete_input_text` | Delete an input_text |
| `set_input_text_value` | Set the value of an input_text |

##### Input Select

| Tool | Description |
|------|-------------|
| `create_input_select` | Create an input_select dropdown |
| `delete_input_select` | Delete an input_select |
| `select_option` | Select an option from the dropdown |
| `set_options` | Update the available options |

##### Input Datetime

| Tool | Description |
|------|-------------|
| `create_input_datetime` | Create an input_datetime picker |
| `delete_input_datetime` | Delete an input_datetime |
| `set_input_datetime` | Set the date/time value |

##### Input Button

| Tool | Description |
|------|-------------|
| `create_input_button` | Create an input_button |
| `delete_input_button` | Delete an input_button |
| `press_input_button` | Press/trigger the button |

##### Counter

| Tool | Description |
|------|-------------|
| `create_counter` | Create a counter with initial/step/min/max |
| `delete_counter` | Delete a counter |
| `increment_counter` | Increment the counter by step |
| `decrement_counter` | Decrement the counter by step |
| `reset_counter` | Reset the counter to initial value |
| `set_counter_value` | Set the counter to a specific value |

##### Timer

| Tool | Description |
|------|-------------|
| `create_timer` | Create a timer with duration |
| `delete_timer` | Delete a timer |
| `start_timer` | Start the timer (optionally with duration) |
| `pause_timer` | Pause a running timer |
| `cancel_timer` | Cancel the timer |
| `finish_timer` | Finish the timer immediately |
| `change_timer` | Change duration of running timer |

##### Schedule

| Tool | Description |
|------|-------------|
| `create_schedule` | Create a weekly schedule with time blocks |
| `delete_schedule` | Delete a schedule |
| `reload_schedule` | Reload schedule from configuration |

##### Group

| Tool | Description |
|------|-------------|
| `create_group` | Create a group of entities |
| `delete_group` | Delete a group |
| `set_group_entities` | Add or remove entities from group |
| `reload_group` | Reload group from configuration |

##### Template (Sensor/Binary Sensor)

| Tool | Description |
|------|-------------|
| `create_template_sensor` | Create a template sensor with Jinja2 state template |
| `create_template_binary_sensor` | Create a template binary sensor |
| `delete_template_helper` | Delete a template sensor/binary sensor |

##### Threshold

| Tool | Description |
|------|-------------|
| `create_threshold` | Create a threshold binary sensor from a source sensor |
| `delete_threshold` | Delete a threshold sensor |

##### Derivative

| Tool | Description |
|------|-------------|
| `create_derivative` | Create a derivative sensor (rate of change) |
| `delete_derivative` | Delete a derivative sensor |

##### Integral (Integration)

| Tool | Description |
|------|-------------|
| `create_integral` | Create an integral sensor (Riemann sum) |
| `delete_integral` | Delete an integral sensor |
| `reset_integral` | Reset the integral value to zero |

#### Script Tools

| Tool | Description |
|------|-------------|
| `list_scripts` | List all scripts |
| `get_script` | Get script details |
| `create_script` | Create a new script |
| `update_script` | Update a script |
| `delete_script` | Delete a script |
| `execute_script` | Execute a script |

#### Scene Tools

| Tool | Description |
|------|-------------|
| `list_scenes` | List all scenes |
| `get_scene` | Get scene details |
| `create_scene` | Create a new scene |
| `update_scene` | Update a scene |
| `delete_scene` | Delete a scene |
| `activate_scene` | Activate a scene |

#### Media Tools

| Tool | Description |
|------|-------------|
| `browse_media` | Browse media sources and libraries |
| `get_camera_stream` | Get camera stream URL for an entity |
| `sign_media_path` | Sign a media path for authenticated access |

#### Statistics Tools

| Tool | Description |
|------|-------------|
| `get_statistics` | Get recorder statistics for entities |

#### Lovelace Tools

| Tool | Description |
|------|-------------|
| `get_lovelace_config` | Get the Lovelace dashboard configuration |

#### Target Tools

| Tool | Description |
|------|-------------|
| `get_triggers_for_target` | Get applicable automation triggers for entities, devices, areas, or labels |
| `get_conditions_for_target` | Get applicable automation conditions for entities, devices, areas, or labels |
| `get_services_for_target` | Get applicable services for entities, devices, areas, or labels |
| `extract_from_target` | Extract and resolve entities, devices, and areas from a target specification |

#### Service Tools

| Tool | Description |
|------|-------------|
| `call_service` | Call any Home Assistant service |

### Example Requests

#### Get All Entity States

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "get_states",
    "arguments": {}
  }
}
```

#### Get Single Entity State

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "get_state",
    "arguments": {
      "entity_id": "light.living_room"
    }
  }
}
```

#### List Entity Registry

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "list_entity_registry",
    "arguments": {}
  }
}
```

#### Browse Media

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "browse_media",
    "arguments": {
      "media_content_id": "media-source://media_source"
    }
  }
}
```

#### Get Statistics

```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "tools/call",
  "params": {
    "name": "get_statistics",
    "arguments": {
      "statistic_ids": ["sensor.temperature", "sensor.humidity"],
      "period": "hour"
    }
  }
}
```

#### Call a Service

```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "method": "tools/call",
  "params": {
    "name": "call_service",
    "arguments": {
      "domain": "light",
      "service": "turn_on",
      "data": {
        "entity_id": "light.living_room",
        "brightness": 255
      }
    }
  }
}
```

#### Create an Automation

```json
{
  "jsonrpc": "2.0",
  "id": 7,
  "method": "tools/call",
  "params": {
    "name": "create_automation",
    "arguments": {
      "alias": "Turn on lights at sunset",
      "trigger": [
        {
          "platform": "sun",
          "event": "sunset"
        }
      ],
      "action": [
        {
          "service": "light.turn_on",
          "target": {
            "entity_id": "light.living_room"
          }
        }
      ]
    }
  }
}
```

## Health Check

The server provides a health check endpoint:

```bash
curl http://localhost:8080/health
# Response: {"status":"ok"}
```

## Troubleshooting

### WebSocket Connection Issues

1. **Verify Home Assistant URL**: Ensure the URL is accessible from where ha-mcp runs
2. **Check Token**: Verify the token is valid and not expired
3. **WebSocket Support**: Ensure Home Assistant allows WebSocket connections (default enabled)
4. **Proxy Configuration**: If using a reverse proxy, ensure WebSocket upgrade is allowed
5. **Firewall**: Ensure port 8123 (HA) and 8080 (MCP) are accessible

### Connection States

ha-mcp includes automatic reconnection with exponential backoff:

- **Initial connection**: Establishes WebSocket and authenticates
- **Disconnection**: Automatic reconnect attempts (1s, 2s, 4s, ... up to 60s)
- **Health monitoring**: Periodic ping to detect connection issues

### Debug Mode

Enable debug logging for more detailed output:

```bash
ha-mcp --log-level debug
```

Debug logs show:
- WebSocket connection state changes
- Message IDs and responses
- Reconnection attempts
- Authentication flow

### Common Errors

| Error | Solution |
|-------|----------|
| `connection refused` | Check if Home Assistant is running and accessible |
| `401 unauthorized` | Token is invalid or expired, create a new one |
| `websocket: bad handshake` | Check URL format and proxy WebSocket support |
| `auth_invalid` | Token authentication failed, verify token |
| `entity not found` | Verify the entity_id exists in Home Assistant |
| `connection closed` | Network issue, ha-mcp will auto-reconnect |

## Development

### Prerequisites

- Go 1.25+
- golangci-lint
- Docker (for container builds)

### Building

```bash
# Build binary
go build -o ha-mcp ./cmd/ha-mcp

# Run tests
go test ./...

# Run linter
golangci-lint run ./...
```

### Project Structure

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
│   │   ├── factory.go           # WebSocket client factory
│   │   ├── ws_client.go         # WebSocket connection management
│   │   ├── ws_client_impl.go    # Full Client implementation
│   │   ├── ws_messages.go       # WebSocket message types
│   │   ├── ws_reconnect.go      # Reconnection logic
│   │   └── types.go             # Data types
│   ├── mcp/
│   │   ├── server.go            # MCP HTTP server
│   │   ├── registry.go          # Tool registry
│   │   └── types.go             # MCP protocol types
│   ├── handlers/
│   │   ├── entities.go          # Entity tool handlers
│   │   ├── automations.go       # Automation tool handlers
│   │   ├── helpers.go           # Helper tool handlers
│   │   ├── scripts.go           # Script tool handlers
│   │   ├── scenes.go            # Scene tool handlers
│   │   ├── registry.go          # Registry tool handlers
│   │   ├── media.go             # Media tool handlers
│   │   ├── statistics.go        # Statistics tool handler
│   │   ├── lovelace.go          # Lovelace tool handler
│   │   ├── targets.go           # Target tool handlers
│   │   └── register.go          # Handler registration
│   └── logging/
│       └── logger.go            # Structured logging
├── configs/
│   ├── config.example.yaml      # Example configuration
│   └── .env.example             # Example environment file
├── Dockerfile                   # Container build
├── .golangci.yml               # Linter configuration
└── README.md                   # This file
```

## Architecture

### WebSocket Communication

ha-mcp uses the Home Assistant WebSocket API exclusively for all operations:

```
┌─────────────┐     HTTP/JSON-RPC      ┌─────────────┐
│  AI Client  │ ◄──────────────────────► │   ha-mcp    │
│  (Claude,   │                         │  MCP Server │
│   Cline)    │                         │             │
└─────────────┘                         └──────┬──────┘
                                               │
                                               │ WebSocket
                                               │ (ws://host/api/websocket)
                                               │
                                        ┌──────▼──────┐
                                        │    Home     │
                                        │  Assistant  │
                                        └─────────────┘
```

### Message Flow

1. AI client sends JSON-RPC request to ha-mcp
2. ha-mcp translates to WebSocket command
3. Home Assistant processes and responds
4. ha-mcp returns result to AI client

## License

GPL-3.0 License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please follow these steps:

1. Fork the repository on GitHub
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Ensure all checks pass:
   ```bash
   # Run linter
   golangci-lint run ./...
   
   # Run tests
   go test -race ./...
   ```
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

### Pull Request Guidelines

- Ensure CI checks pass (lint, test, security scans)
- Update documentation if needed
- Add tests for new functionality
- Keep commits focused and atomic

## Acknowledgments

- [Model Context Protocol](https://modelcontextprotocol.io/) specification
- [Home Assistant WebSocket API](https://developers.home-assistant.io/docs/api/websocket)
- [coder/websocket](https://github.com/coder/websocket) - Pure Go WebSocket library
