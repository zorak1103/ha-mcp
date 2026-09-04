> [README](../README.md) | [Configuration](configuration.md) | [Tools](tools.md) | [Access Control](access-control.md) | [Architecture](architecture.md) | [Troubleshooting](troubleshooting.md) | [Feature Comparison](feature-comparison.md) | [Integration Tests](integration-tests.md)

# Access Control & Tool Filtering

ha-mcp provides flexible access control to restrict which tools and actions are available. This is useful for security-conscious deployments, shared instances, or limiting AI capabilities.

## Read-Only Mode

The simplest form of access control is read-only mode, which blocks all write operations while allowing read operations:

```yaml
# config.yaml
server:
  read_only: true
```

```bash
# Command line
ha-mcp --read-only

# Environment variable
export HA_MCP_READ_ONLY=true
```

**What gets blocked in read-only mode:**
- All `create`, `update`, `delete` actions in `manage_*` tools
- Service calls (`call_service`)
- Helper actions (`helper_action` - toggle, set, increment, etc.)
- Script execution, scene activation
- Any operation that modifies Home Assistant state

**What remains available:**
- All `list` and `get` actions
- State queries (`get_state`, `query_entities`, `query_devices`)
- History and statistics
- Analysis tools (`analyze_entity`, `get_entity_dependencies`)
- Registry queries (`get_registry`)
- Logbook access

## Tool Filtering

For more granular control, use the tool filter system with whitelist or blacklist.

### Whitelist Mode

When whitelist is non-empty, **ONLY** the specified tools/actions are allowed:

```yaml
server:
  tool_filter:
    whitelist:
      - "get_state"                    # Allow entire tool
      - "manage_automation:list"       # Allow specific action
      - "manage_automation:get"
      - "manage_script:list"
      - "query_entities"
```

```bash
# Environment variable (comma-separated)
export HA_MCP_TOOL_FILTER_WHITELIST="get_state,manage_automation:list,manage_automation:get"
```

**Tool-level entries are forward-open, not a snapshot of today's actions.** Whitelisting an
entire tool (e.g. `"manage_config_entry"`, with no `:action` suffix) allows every action that
tool supports — including ones added in a later release. `manage_config_entry` gained a
`delete` action (removes a config entry and all its associated devices/entities) after
previously supporting only `list`/`get`; a deployment that whitelisted the bare tool name
before that change was granted delete capability automatically, with no configuration change
on its part. If a deployment needs to freeze what a whitelisted tool can do, use action-level
entries instead of the bare tool name, e.g. `"manage_config_entry:list"` and
`"manage_config_entry:get"` rather than `"manage_config_entry"`.

**Blocking `manage_script`/`manage_automation` alone does not block script execution once
`manage_helper` is allowed.** The 15 `template_*` helper subtypes (`template_button`,
`template_switch`, `template_lock`, ...) accept HA action-sequence fields (`press`, `turn_on`,
`lock`/`unlock`, `install`, `trigger`, ...) — the same shape as an automation's `action:` block —
directly in `manage_helper:create`/`manage_helper:update`. `access_control.go` classifies both
as plain writes with no awareness of the embedded action, so a policy like
`blacklist: ["manage_script:*", "manage_automation:*"]` (or a whitelist including `manage_helper`
but not those tools) still allows: create a `template_button` with
`press={"action":"shell_command.dangerous"}`, then `call_service button.press` to run it. A
deployment that intends to prevent arbitrary service-call execution must also filter
`manage_helper:create` and `manage_helper:update` explicitly. `read_only` mode is unaffected —
it already blocks all writes, including these.

### Blacklist Mode

When whitelist is empty, use blacklist to block specific tools/actions:

```yaml
server:
  tool_filter:
    blacklist:
      - "call_service"                 # Block entire tool
      - "manage_automation:delete"     # Block specific action
      - "manage_script:delete"
      - "manage_scene:delete"
```

```bash
# Environment variable (comma-separated)
export HA_MCP_TOOL_FILTER_BLACKLIST="call_service,manage_automation:delete"
```

### Glob Patterns

Use glob patterns to match multiple tools:

```yaml
server:
  tool_filter:
    blacklist:
      - "manage_*:delete"              # Block delete action across all manage_* tools
      - "manage_*:create"              # Block create action across all manage_* tools
      - "get_*"                        # Block all get_* tools
```

### Category-Based Filtering

Filter by action category (read/write):

```yaml
server:
  tool_filter:
    blacklist:
      - "*:write"                      # Block all write operations (same as read_only: true)
```

```yaml
server:
  tool_filter:
    whitelist:
      - "*:read"                       # Allow only read operations
```

### Action Filtering on manage_entity / manage_device

To allow read access but block deletions:

```yaml
server:
  tool_filter:
    blacklist:
      - "manage_entity:delete"   # Block entity registry deletion
      - "manage_device:delete"   # Block device registry deletion
```

## Filter Behavior

- **Tool Removal**: Completely blocked tools disappear from `tools/list` (the AI won't see them)
- **Schema Modification**: Partially blocked tools have their schemas updated to show only allowed actions
- **Runtime Check**: Attempted blocked actions return an error at runtime
- **Startup Validation**: Every entry is validated against the known tool set at startup — unknown tools, typos, and removed actions cause the server to refuse to start with a descriptive error listing all invalid entries at once
- **Mutual Exclusion**: Whitelist and blacklist cannot be used together - configuration validation will fail if both are non-empty
- **Whitelist Mode**: If whitelist is specified, ONLY listed items are allowed (implicit deny-all)
- **Blacklist Mode**: If whitelist is empty, blacklist blocks specific items (implicit allow-all)
- **AI Notification**: When filter is active, clients receive a "⚠️ SERVER IN RESTRICTED MODE" message

## Example Scenarios

**Scenario 1: Read-only for monitoring**
```yaml
server:
  read_only: true
```

**Scenario 2: Allow only safe operations**
```yaml
server:
  tool_filter:
    whitelist:
      - "get_*"                        # All get tools
      - "query_*"                      # All query tools
      - "analyze_*"                    # All analysis tools
      - "manage_automation:list"
      - "manage_automation:get"
```

**Scenario 3: Block dangerous operations**
```yaml
server:
  tool_filter:
    blacklist:
      - "call_service"                 # Block arbitrary service calls
      - "manage_*:delete"              # Block all delete operations
      - "manage_script:execute"        # Block script execution
      - "manage_scene:activate"        # Block scene activation
```

**Scenario 4: Whitelist with specific write operations allowed**
```yaml
server:
  tool_filter:
    whitelist:
      - "get_*"                        # All get tools
      - "query_*"                      # All query tools
      - "analyze_*"                    # All analysis tools
      - "manage_automation:list"       # Read automation
      - "manage_automation:get"
      - "manage_automation:toggle"     # Exception: allow enable/disable
      - "helper_action"                # Exception: allow helper actions
```

**Note:** You cannot combine whitelist and blacklist - the server will refuse to start with a validation error if both are specified. Every individual entry is also validated; stale or mistyped entries (e.g. `"manage_entity:frobnicate"`, `"query_entities:health:remove"`) cause the server to refuse to start with a clear error message listing all problems at once.
