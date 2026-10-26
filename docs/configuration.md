> [README](../README.md) | [Configuration](configuration.md) | [Tools](tools.md) | [Access Control](access-control.md) | [Architecture](architecture.md) | [Troubleshooting](troubleshooting.md) | [Feature Comparison](feature-comparison.md) | [Integration Tests](integration-tests.md)

# Configuration

## Connection Requirements

ha-mcp connects to Home Assistant via **WebSocket** (`ws://{host}/api/websocket` or `wss://{host}/api/websocket` for HTTPS). Ensure:

- Home Assistant is running and accessible
- WebSocket connections are allowed (default in HA)
- The URL points to your Home Assistant instance (HTTP/HTTPS URL is converted to WebSocket internally)
- A valid long-lived access token is configured

## HTTPS/WSS Support

ha-mcp fully supports secure connections. The URL scheme is automatically converted:

| Input URL Scheme  | WebSocket Scheme   |
| ----------------- | ------------------ |
| `http://`         | `ws://`            |
| `https://`        | `wss://`           |
| `ws://`           | `ws://`            |
| `wss://`          | `wss://`           |

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

## Proxy Support

ha-mcp supports HTTP/HTTPS proxies via standard environment variables. The underlying WebSocket library (`coder/websocket`) uses Go's standard HTTP client, which automatically respects these proxy settings.

**Supported environment variables:**

| Variable      | Description                                             |
| ------------- | ------------------------------------------------------- |
| `HTTP_PROXY`  | Proxy for HTTP connections (e.g., `http://proxy:8080`)  |
| `HTTPS_PROXY` | Proxy for HTTPS connections (e.g., `http://proxy:8080`) |
| `NO_PROXY`    | Comma-separated list of hosts to bypass proxy           |

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

## Configuration File

Create a config file at one of these locations:
- `./config.yaml`
- `./configs/config.yaml`
- `$HOME/.config/ha-mcp/config.yaml`
- `/etc/ha-mcp/config.yaml`

See `configs/config.example.yaml` for the full annotated example. Key options:

```yaml
homeassistant:
  url: "http://homeassistant.local:8123"  # WebSocket URL derived automatically
  token: "your-long-lived-access-token"
  rest:
    rate_limit: 10  # Requests per second (0 = unlimited)
    rate_burst: 5   # Maximum burst size
    max_retries: 3  # Retry attempts for transient failures (0 = no retries)
    retry_initial_delay_ms: 100  # Initial delay between retries
    retry_max_delay_ms: 5000     # Maximum delay between retries
  websocket:
    max_retries: 3  # Retry attempts per command (0 = no retries)
    retry_initial_delay_ms: 100
    retry_max_delay_ms: 5000
  cache:
    # The cache applies to the default HA_TOKEN client and to per-request-auth clients alike.
    enabled: false  # Enable caching for static data (opt-in)
    services_ttl_min: 60     # Services cache TTL in minutes
    config_ttl_min: 30       # Config cache TTL in minutes
    entity_reg_ttl_min: 10   # Entity registry cache TTL
    device_reg_ttl_min: 10   # Device registry cache TTL
    area_reg_ttl_min: 30     # Area registry cache TTL
  wait:
    wait_timeout_ms: 5000     # Max wait for state change confirmation (ms)
    wait_poll_interval_ms: 100  # Polling interval for state checks (ms)

server:
  port: 8080
  read_only: false  # Enable read-only mode (blocks all write operations)
  tool_filter:
    whitelist: []   # If non-empty, ONLY these tools/actions are allowed
    blacklist: []   # Block specific tools/actions (only if whitelist is empty)

logging:
  level: "info"  # debug, info, warn, error
```

## Environment Variables

```bash
export HA_URL=http://homeassistant.local:8123
export HA_TOKEN=your-long-lived-access-token
export HA_MCP_PORT=8080
export HA_MCP_LOG_LEVEL=info

# REST API settings (optional)
export HA_REST_RATE_LIMIT=10   # Requests per second (0 = unlimited, default: 10)
export HA_REST_RATE_BURST=5    # Maximum burst size (default: 5)
export HA_REST_MAX_RETRIES=3   # Max retry attempts (0 = no retries, default: 3)
export HA_REST_RETRY_INITIAL_DELAY_MS=100  # Initial retry delay in ms (default: 100)
export HA_REST_RETRY_MAX_DELAY_MS=5000     # Max retry delay in ms (default: 5000)

# WebSocket settings (optional)
export HA_WS_MAX_RETRIES=3     # Max retry attempts per command, 0 = no retries (default: 3)
export HA_WS_RETRY_INITIAL_DELAY_MS=100    # Initial retry delay in ms (default: 100)
export HA_WS_RETRY_MAX_DELAY_MS=5000       # Max retry delay in ms (default: 5000)

# Caching settings (optional, disabled by default; applies to the default HA_TOKEN client and to per-request-auth clients alike)
export HA_CACHE_ENABLED=false              # Enable caching (default: false)
export HA_CACHE_SERVICES_TTL_MIN=60        # Services cache TTL (default: 60)
export HA_CACHE_CONFIG_TTL_MIN=30          # Config cache TTL (default: 30)
export HA_CACHE_ENTITY_REG_TTL_MIN=10      # Entity registry TTL (default: 10)
export HA_CACHE_DEVICE_REG_TTL_MIN=10      # Device registry TTL (default: 10)
export HA_CACHE_AREA_REG_TTL_MIN=30        # Area registry TTL (default: 30)

# Post-mutation polling settings (optional)
export HA_WAIT_TIMEOUT_MS=5000             # Max wait for state confirmation (default: 5000)
export HA_WAIT_POLL_INTERVAL_MS=100        # Poll interval in ms (default: 100)

# Access control settings (optional)
export HA_MCP_READ_ONLY=false              # Enable read-only mode (default: false)
export HA_MCP_TOOL_FILTER_WHITELIST=""     # Comma-separated whitelist (default: empty)
export HA_MCP_TOOL_FILTER_BLACKLIST=""     # Comma-separated blacklist (default: empty)
```

See `configs/.env.example` for the full environment file template.

### Cache behavior

With `HA_CACHE_ENABLED=true`, the services, config and registry reads are cached for their TTL. The cache is cleared early in these cases:

| Event                                                                  | Registry caches (entity, device, area, label, floor) | Services and config caches |
| ---------------------------------------------------------------------- | ---------------------------------------------------- | -------------------------- |
| Automation, script or scene create/update/delete                       | cleared                                              | kept                       |
| Helper create/update/delete, config entry delete                       | cleared                                              | kept                       |
| Any service call (including `*.reload`), successful or not             | cleared                                              | kept                       |
| Entity/device registry update, area/label/floor create/update/delete   | the matching registry cache is cleared               | kept                       |
| WebSocket reconnect (for example after a Home Assistant restart)       | cleared                                              | cleared                    |

Known limits:

- The services and config caches are otherwise refreshed only by TTL (`HA_CACHE_SERVICES_TTL_MIN`, `HA_CACHE_CONFIG_TTL_MIN`). A new integration or service added without a reconnect can stay hidden for up to that TTL.
- Each client instance has its own cache. A request that carries a bearer token is served by a pooled client with a cache separate from the default `HA_TOKEN` client (even when the token equals `HA_TOKEN`), and a write through one instance does not clear the other's cache. Stale registry data then lasts at most the registry TTL.
- Changes made outside ha-mcp (the Home Assistant UI, YAML edits) are not detected; they appear after the TTL expires.

### Retry behavior

The `HA_REST_*` and `HA_WS_*` retry settings are applied to the clients at startup:

- **REST**: GET and HEAD requests are retried on any transient failure (429, 500/502/503/504, network errors; not client timeouts). Other methods (create/update/delete, config flow steps, template render) are retried only on 429 and connection-setup (dial) failures, because replaying them after a 5xx or a timeout could run a write twice.
- **WebSocket**: the settings bound the retries of a single command. `0` disables retries.
- A delay of 0 or less falls back to the default, and a max delay below the initial delay is raised to the initial delay.

## Command-Line Flags

```bash
ha-mcp \
  --ha-url http://homeassistant.local:8123 \
  --ha-token your-long-lived-access-token \
  --port 8080 \
  --read-only  # Optional: enable read-only mode
```

## Getting a Home Assistant Token

1. Open Home Assistant web interface
2. Click on your profile (bottom left)
3. Scroll to "Long-Lived Access Tokens"
4. Click "Create Token"
5. Give it a name (e.g., "ha-mcp")
6. Copy the token (it won't be shown again!)

## Docker

Multi-arch Docker images (amd64/arm64) are published to Docker Hub on each release.

```bash
# Pull the latest image
docker pull zorak1103/ha-mcp:latest

# Run container (token provided by clients via Authorization header)
docker run -d \
  --name ha-mcp \
  -p 8080:8080 \
  -e HA_URL=http://homeassistant.local:8123 \
  zorak1103/ha-mcp:latest

# Or with default token for development (optional)
docker run -d \
  --name ha-mcp \
  -p 8080:8080 \
  -e HA_URL=http://homeassistant.local:8123 \
  -e HA_TOKEN=your-long-lived-access-token \
  zorak1103/ha-mcp:latest

# Use a specific version
docker pull zorak1103/ha-mcp:v0.8.0
```

Available tags:
- `zorak1103/ha-mcp:latest` - Latest release (multi-arch)
- `zorak1103/ha-mcp:vX.Y.Z` - Specific version (multi-arch)

## Authentication

ha-mcp supports flexible authentication via HTTP Bearer tokens. The Home Assistant access token can be provided either per-request via HTTP header or as a server default.

### Token via HTTP Header (Recommended)

MCP clients send the Home Assistant token in the `Authorization` header with every request:

```
Authorization: Bearer <your-long-lived-access-token>
```

This approach is recommended because:
- Each client can use their own Home Assistant token
- Tokens are not stored on the server
- Tokens can have different permissions for different clients

**Example with curl:**

```bash
curl -X POST http://localhost:8080/ \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJ0eXAiOiJKV1QiLCJhbGc..." \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

### Development Mode (Optional)

For local development or testing, you can configure a default token on the server:

```bash
ha-mcp --ha-url http://homeassistant.local:8123 --ha-token your-token
```

When a default token is configured:
- Requests **with** an `Authorization` header use the header token
- Requests **without** an `Authorization` header use the default token

This allows backwards-compatible operation while supporting per-request authentication.

### Authentication Errors

When no token is provided and no default is configured:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32004,
    "message": "authorization header with Bearer token required"
  }
}
```

Tokens shorter than 10 characters are rejected before any connection is attempted:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32603,
    "message": "authorization token too short"
  }
}
```

## Rate Limiting

The server enforces per-IP rate limiting to prevent connection pool exhaustion.

- **Sustained rate**: 10 requests/second per client IP
- **Burst capacity**: 30 requests per client IP
- **Response when exceeded**: HTTP `429 Too Many Requests` with body `{"jsonrpc":"2.0","error":{"code":-32429,"message":"rate limit exceeded"},"id":null}`
- **Health endpoint exempt**: `/health` is never rate-limited (safe for liveness probes)

Clients behind shared NAT share one rate-limit bucket. The per-IP limits are currently fixed constants; configurable env vars are planned for a future release.

## Logging

The server log level is controlled by `HA_MCP_LOG_LEVEL` (`trace`, `debug`, `info` (default), `warn`, `error`).

**Payload redaction at TRACE:** At `trace` level, the server logs only payload summaries — request method, top-level parameter keys, and byte size — never parameter values, tool arguments, response content, or error data. This protects automation configs, template content, entity details, and any embedded credentials from leaking into log files or aggregators even when tracing is enabled.

- Example TRACE output: `Request received remote_addr=… summary="method=tools/call id=1 param_keys=[name arguments]" size=247`
- A startup `WARN` is emitted when TRACE is active to make its use explicit.

## Health Check

The server provides a health check endpoint (no authentication required, not rate-limited):

```bash
curl http://localhost:8080/health
# Response: {"status":"ok"}
```

## Client Setup

### Cline

Add to your Cline MCP configuration (`~/.config/cline/mcp.json`):

```json
{
  "servers": {
    "ha-mcp": {
      "url": "http://localhost:8080",
      "headers": {
        "Authorization": "Bearer your-ha-access-token"
      },
      "description": "Home Assistant MCP Server"
    }
  }
}
```

### Claude Desktop

Add to Claude Desktop's MCP configuration:

```json
{
  "mcpServers": {
    "homeassistant": {
      "type": "http",
      "url": "http://localhost:8080",
      "headers": {
        "Authorization": "Bearer ${HA_TOKEN}"
      }
    }
  }
}
```

### opencode

Configure in your opencode settings:

```yaml
mcp:
  servers:
    - name: homeassistant
      url: http://localhost:8080
      headers:
        Authorization: "Bearer your-ha-access-token"
```
