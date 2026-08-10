> [README](../README.md) | [Configuration](configuration.md) | [Tools](tools.md) | [Access Control](access-control.md) | [Architecture](architecture.md) | [Troubleshooting](troubleshooting.md) | [Feature Comparison](feature-comparison.md) | [Integration Tests](integration-tests.md)

# Tools Reference

All MCP requests are sent to:

```
POST http://localhost:8080/
Content-Type: application/json
Authorization: Bearer <your-ha-access-token>
```

## Available Tools

### Entity Tools

| Tool                      | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| ------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `query_entities`          | Consolidated entity queries (mode: current, history, statistics, domains, presence, health; format: natural/json; group_by: domain, area_id, device_class, integration; health: multi-category filter for unavailable/unknown/disabled/orphaned/stale entities). Non-verbose `mode=current` returns a compact list of entity_id + friendly name + state (capped at 50; use `verbose=true` or pagination for more). Verbose output also includes each entity's `entity_id` alongside its name and state. |
| `query_devices`           | Device health check (mode: health; format: natural/json; categories: disabled, orphaned_config_entry, config_entry_error, no_entities, no_config_entries; manufacturer filter)                                                                                                                                                                                                                                                                                                                          |
| `get_state`               | Get state of a specific entity (format: natural/json)                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| `analyze_entity`          | Analyze entity usage in automations, scripts, scenes, dashboards, and template-helper templates; includes registry metadata (platform, area, device, labels, aliases), RFC 6901 JSON Pointer paths to each reference location (e.g. `/sequence/0/target/entity_id  (action: automation.turn_off)`), and a `scanned_sources` list so a "no references" result can be trusted (format: natural/json)                                                                                                      |
| `get_entity_dependencies` | Find all entities an automation/script depends on (format: natural/json)                                                                                                                                                                                                                                                                                                                                                                                                                                |
| `find_references`         | Server-side search for a string/entity_id across automations, scripts, scenes, dashboards, and template-helper templates in one call (match_mode: substring/exact; types filter; format: natural/json)                                                                                                                                                                                                                                                                                                  |

### Registry Tools

| Tool                  | Description                                                                                                   |
| --------------------- | ------------------------------------------------------------------------------------------------------------- |
| `get_registry`        | Query registries (type: entities, devices, areas, all; format: natural/json)                                  |
| `manage_area`         | Consolidated area management (actions: list, get, create, update, delete; update supports label_mode/alias_mode: add/remove/replace; get supports include_entities/include_automations; format: natural/json for list/get) |
| `manage_label`        | Consolidated label management (actions: list, get, create, update, delete; format: natural/json for list/get)                                                           |
| `manage_floor`        | Consolidated floor management (actions: list, get, create, update, delete; update supports alias_mode: add/remove/replace; format: natural/json for list/get)           |
| `manage_zone`         | Consolidated zone management (actions: list, get, create, update, delete; format: natural/json for list/get)                                                            |
| `manage_person`       | Consolidated person management (actions: list, get, create, update, delete; format: natural/json for list/get)                                                          |
| `manage_tag`          | Consolidated tag management (actions: list, get, create, update, delete; format: natural/json for list/get)                                                             |
| `manage_entity`       | Entity registry management (actions: get, update, delete; update fields: name, icon, area_id, disabled_by, hidden_by, labels, aliases, new_entity_id; update supports label_mode/alias_mode: add/remove/replace; format: natural/json). **Friendly-name hierarchy:** registry `name` > automation `alias` > auto-slug. `name=""` clears the registry override and falls back to the auto-slug (not the alias). |
| `manage_device`       | Device registry management (actions: get, update, delete; update fields: name_by_user, area_id, disabled_by, labels; update supports label_mode: add/remove/replace; format: natural/json)                                              |
| `manage_config_entry` | Consolidated config entry management (actions: list, get; list: optional domain filter; get: requires entry_id; format: natural/json) |

### Automation Tools

| Tool                | Description                                                                                                                                   |
| ------------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `manage_automation` | Consolidated automation management (actions: list, get, create, update, delete, toggle, coverage, patch (RFC 6902 JSON Patch + semantic patch); format: natural/json for list/get/coverage) |

**Flexible ID Lookup**: The `automation_id` parameter accepts multiple formats:
- Entity ID: `automation.morning_lights`
- Config ID (UUID): `abc123-def456-...`
- Alias or friendly name: `morning lights` (case-insensitive partial match)

**Semantic Patch**: The `patch` action supports property-based addressing instead of numeric JSON Pointer indices. Use `match` + `section` + `field` to target elements by their properties:
```json
{
  "op": "add",
  "match": {"entity_id": "binary_sensor.door", "to": "off"},
  "section": "triggers",
  "field": "for",
  "value": "00:05:00"
}
```
- `match`: key-value pairs identifying the target element(s)
- `section`: array to search — `triggers`, `conditions`, `actions`
- `field`: field within matched element(s) to modify (omit for `remove` to delete the whole element)
- `match_index`: optional 0-based index to select a specific match when multiple elements match

`section` recurses into nested arrays/objects within it — a `match` inside a
nested `choose`/`if`/`repeat` action block is found the same way a top-level element is. A
standard JSON Pointer `path` op is still useful for pinpointing one specific occurrence
deterministically, and the nesting is easy to get wrong by hand: `then`/`else` are
**siblings of `if`**, not nested inside it. Use `/actions/0/then/0`, not
`/actions/0/if/0/then/0`. See the `ha-mcp-patching` skill ("Nested Action Structures") for
the full `choose`/`if`/`repeat` path reference. `dry_run: true` returns a compact diff of
affected paths (not the full config).

**Key Parameters (create/update):**

| Parameter    | Type    | Required | Description                                                                                     |
| ------------ | ------- | -------- | ----------------------------------------------------------------------------------------------- |
| alias        | string  | Yes      | Human-readable automation name (used to derive entity_id)                                       |
| mode         | string  | No       | Run mode: `single` (default), `restart`, `queued`, `parallel`                                   |
| max          | integer | No       | Concurrent run limit (min 1, HA default 10). Only applies when mode is `parallel` or `queued`. |

### Helper Tools

ha-mcp provides comprehensive support for all 26 Home Assistant helper types through two consolidated tools.

| Tool            | Description                                                                                                                |
| --------------- | -------------------------------------------------------------------------------------------------------------------------- |
| `manage_helper` | List, create, update, delete, or get details for any helper type (format: natural/json) |
| `helper_action` | Execute runtime actions (toggle, set, increment, start, etc.)                                                              |

#### manage_helper

Universal tool for helper lifecycle management:

| Action        | Description                                                                                          |
| ------------- | ---------------------------------------------------------------------------------------------------- |
| `list`        | List all helpers with optional format (natural/json) and verbose mode                                |
| `create`      | Create a new helper (requires `type`, `id`, `name`)                                                  |
| `update`      | Update an existing helper (requires `entity_id`; supports all helper types)                          |
| `delete`      | Delete an existing helper (requires `entity_id`)                                                     |
| `get_details` | Get detailed configuration for any helper type (requires `entity_id`; format: natural/json) |

**Supported helper types (26 total):**
- **Input helpers:** `input_boolean`, `input_number`, `input_text`, `input_select`, `input_datetime`, `input_button`
- **Stateful helpers:** `counter`, `timer`, `schedule`
- **Entity grouping:** `group`
- **Advanced helpers:** `template_sensor`, `template_binary_sensor`, `threshold`, `derivative`, `integral`
- **Utility helpers:** `utility_meter`, `min_max`, `statistics`, `trend`, `filter`
- **Random generators:** `random_sensor`, `random_binary_sensor`
- **Time-based:** `tod` (Time of Day)
- **Climate/Environment:** `generic_thermostat`, `generic_hygrostat`
- **Entity converters:** `switch_as_x`

**ID Parameter Behavior:**
- For WebSocket helpers (`input_*`, `counter`, `timer`, `schedule`): The `id` parameter controls the entity ID (e.g., `id="test_bool"` creates `input_boolean.test_bool`), while `name` sets the display name
- For Config Entry Flow helpers (`threshold`, `derivative`, `integral`, `group`, `template_*`, `utility_meter`, `min_max`, `statistics`, `trend`, `random_*`, `filter`, `tod`, `generic_thermostat`, `generic_hygrostat`, `switch_as_x`): Entity ID is derived from `name` (Home Assistant limitation)

#### helper_action

Universal tool for runtime helper operations:

| Action            | Applicable To                                     | Description                     |
| ----------------- | ------------------------------------------------- | ------------------------------- |
| `toggle`          | input_boolean                                     | Toggle on/off                   |
| `set`             | input_number, input_text, input_datetime, counter | Set value                       |
| `increment`       | counter                                           | Increment by step               |
| `decrement`       | counter                                           | Decrement by step               |
| `reset`           | counter, integral                                 | Reset to initial/zero           |
| `calibrate`       | utility_meter                                     | Calibrate utility meter         |
| `start`           | timer                                             | Start timer (optional duration) |
| `pause`           | timer                                             | Pause running timer             |
| `cancel`          | timer                                             | Cancel timer                    |
| `finish`          | timer                                             | Finish immediately              |
| `change`          | timer                                             | Change duration while running   |
| `press`           | input_button                                      | Press/trigger button            |
| `select`          | input_select                                      | Select an option                |
| `set_options`     | input_select                                      | Update available options        |
| `reload`          | schedule, group                                   | Reload from configuration       |
| `add_entities`    | group                                             | Add entities to group           |
| `remove_entities` | group                                             | Remove entities from group      |

### Script Tools

| Tool            | Description                                                                                                             |
| --------------- | ----------------------------------------------------------------------------------------------------------------------- |
| `manage_script` | Consolidated script management (actions: list, get, create, update, delete, execute, patch (RFC 6902 JSON Patch + semantic patch); format: natural/json for list/get) |

**Flexible ID Lookup**: The `script_id` parameter accepts multiple formats:
- Entity ID: `script.morning_routine`
- Alias or friendly name: `morning routine` (case-insensitive partial match)

**Key Parameters (create/update):**

| Parameter    | Type    | Required | Description                                                                                     |
| ------------ | ------- | -------- | ----------------------------------------------------------------------------------------------- |
| alias        | string  | Yes      | Human-readable script name (used to derive entity_id)                                          |
| mode         | string  | No       | Run mode: `single` (default), `restart`, `queued`, `parallel`                                   |
| max          | integer | No       | Concurrent run limit (min 1, HA default 10). Only applies when mode is `parallel` or `queued`. |

### Scene Tools

| Tool           | Description                                                                                                             |
| -------------- | ----------------------------------------------------------------------------------------------------------------------- |
| `manage_scene` | Consolidated scene management (actions: list, get, create, update, delete, activate, patch (RFC 6902 JSON Patch + semantic patch); format: natural/json for list/get) |

**Flexible ID Lookup**: The `scene_id` parameter accepts multiple formats:
- Entity ID: `scene.movie_time`
- Friendly name: `movie time` (case-insensitive partial match)

### Media Tools

| Tool              | Description                                |
| ----------------- | ------------------------------------------ |
| `browse_media`    | Browse media sources and libraries         |
| `sign_media_path` | Sign a media path for authenticated access |

### Dashboard Tools

| Tool               | Description                                                                        |
| ------------------ | ---------------------------------------------------------------------------------- |
| `manage_dashboard` | Manage Lovelace dashboards - list, get, create, update, delete, save configuration, patch (JSON Patch + semantic patch, e.g. match views by `title`), find (search a string/entity_id across all views and nested cards without fetching the whole config) |

### Template Tools

| Tool              | Description                                                 |
| ----------------- | ----------------------------------------------------------- |
| `render_template` | Render a Jinja2 template using current Home Assistant state |

### Logbook Tools

| Tool          | Description                                                                                       |
| ------------- | ------------------------------------------------------------------------------------------------- |
| `get_logbook` | Get logbook entries (mode: entries, correlation; format: natural/json) with cause-effect analysis |

### Configuration Tools

| Tool              | Description                                                  |
| ----------------- | ------------------------------------------------------------ |
| `validate_config` | Validate Home Assistant configuration.yaml for syntax errors |

### Target Tools

| Tool             | Description                                                                                                             |
| ---------------- | ----------------------------------------------------------------------------------------------------------------------- |
| `analyze_target` | Analyze targets for automation capabilities (info: triggers, conditions, services, entities, all; format: natural/json) |

### Service Tools

| Tool            | Description                                                            |
| --------------- | ---------------------------------------------------------------------- |
| `call_service`  | Call any Home Assistant service (format: natural/json)                 |
| `list_services` | List all available services with descriptions (optional domain filter) |

### System Tools

| Tool              | Description                                                                                |
| ----------------- | ------------------------------------------------------------------------------------------ |
| `get_system_info` | Get Home Assistant system configuration (version, timezone, units, etc.)                   |
| `get_datetime`    | Get current date/time in Home Assistant's configured timezone (optional timezone override) |

### HACS Tools

> **Note**: HACS (Home Assistant Community Store) is an optional third-party add-on. These tools will only work if HACS is installed.

| Tool          | Description                                                                                                                                                                                                 |
| ------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `manage_hacs` | Manage HACS repositories (actions: info, list, get, releases, release_notes, critical, download, uninstall, add_repository, remove_repository, refresh, toggle_beta; format: natural/json for read actions) |

**Key Features:**
- **Repository Management**: List installed repositories, check for updates, download/uninstall
- **Custom Repositories**: Add and remove custom GitHub repositories
- **Beta Versions**: Toggle beta version visibility per repository
- **Release Information**: Get release notes and available versions
- **Filters**: Filter by category (integration, plugin, theme, python_script, appdaemon, netdaemon), installed status, or pending updates

### Trace and Blueprint Tools

| Tool               | Description                                                                                          |
| ------------------ | ---------------------------------------------------------------------------------------------------- |
| `manage_trace`     | View automation and script execution traces (actions: list, get, debug; format: natural/json). `list` supports optional `wait=true` to poll until traces appear — useful immediately after triggering an automation (HA records traces asynchronously). |
| `manage_blueprint` | Manage blueprints for automations and scripts (actions: list, import; format: natural/json). `import` requires an `https://` URL pointing to a public host — non-https schemes, private/loopback IPs, and link-local addresses (e.g. `169.254.x.x`) are rejected. |

### Update Tools

| Tool             | Description                                                                                                                        |
| ---------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| `manage_update`  | Manage system and add-on updates (actions: list, release_notes, install, skip; pending_only filter; format: natural/json)          |

### Todo Tools

| Tool          | Description                                                                                                                               |
| ------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| `manage_todo` | Manage todo and shopping lists (actions: list, get_items, add_item, update_item, remove_item; status_filter; format: natural/json)      |

### Calendar Tools

| Tool               | Description                                                                                                                           |
| ------------------ | ------------------------------------------------------------------------------------------------------------------------------------- |
| `manage_calendar`  | Manage calendars and events (actions: list, get_events, create_event, delete_event; supports date and datetime formats)              |

### Camera Tools

| Tool             | Description                                                                                      |
| ---------------- | ------------------------------------------------------------------------------------------------ |
| `manage_camera`  | Access camera snapshots and streams (actions: snapshot returns image, stream returns URL)        |

### System Log Tools

| Tool                 | Description                                                                                                                                                                           |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `manage_system_log`  | Read or clear the Home Assistant system log ring buffer (actions: list, clear). Supports level/integration/limit/include_exception/format filters. Uses `system_log/list` WebSocket command. |

**Actions:**

| Action  | Access | Description                                          | Key params                                              |
| ------- | ------ | ---------------------------------------------------- | ------------------------------------------------------- |
| `list`  | read   | Fetch recent WARNING/ERROR entries (~50 max by default) | `level`, `integration`, `limit`, `include_exception`, `format` |
| `clear` | write  | Empty the in-memory ring buffer                      | —                                                       |

### Guidance Tools

| Tool        | Actions    | Description                                                                                                                                                                                  |
| ----------- | ---------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `get_skill` | list, read | Retrieve embedded skill guidance topics. `action=list` shows all available skills; `action=read` with `skill=<slug>` fetches full guidance. Fallback for tool-only clients; resource-aware clients use `skill://ha-mcp/*` resources. |

## Guidance Resources (`skill://`)

ha-mcp exposes 7 embedded guidance documents as MCP resources under the `skill://ha-mcp/` URI prefix.
Resource-aware clients (Claude Desktop, Cline, etc.) can discover them via `resources/list` and fetch with `resources/read`.
Tool-only clients (claude.ai web) can use `get_skill` instead.

| URI                                    | Topic                | Covers                                                      |
| -------------------------------------- | -------------------- | ----------------------------------------------------------- |
| `skill://ha-mcp/format-selection`      | Format Selection     | When to use natural vs. json format                         |
| `skill://ha-mcp/automation-patterns`   | Automation Patterns  | Modes, trigger IDs, motion+timer, conditions vs. templates  |
| `skill://ha-mcp/template-resilience`   | Template Resilience  | has_value() guards, unavailable handling, render_template   |
| `skill://ha-mcp/helper-selection`      | Helper Selection     | 26-type decision matrix; id vs. name rules                  |
| `skill://ha-mcp/dashboard-safety`      | Dashboard Safety     | Backup-first, truncation risk, patch workflow               |
| `skill://ha-mcp/entity-renaming`       | Entity Renaming      | Safe rename workflow, slugify traps, area/label modes       |
| `skill://ha-mcp/debugging-workflow`    | Debugging Workflow   | Logbook correlation, trace inspection, system log triage    |

## Output Formats

Most tools support two output formats via the `format` parameter:

- **`natural` (default)**: LLM-optimized natural language output for better readability and reduced token usage
  - Example: `"Living Room Light is on at 80% brightness. Changed 2h ago."`

- **`json`**: Structured JSON output for backward compatibility and programmatic access
  - Example: `{"entity_id": "light.living_room", "state": "on", "attributes": {"brightness": 204, ...}}`

**Tools with format support**: `query_entities`, `query_devices`, `get_state`, `analyze_entity`, `get_entity_dependencies`, `find_references`, `call_service`, `get_registry`, `analyze_target`, `manage_automation`, `manage_script`, `manage_scene`, `manage_area`, `manage_label`, `manage_floor`, `manage_zone`, `manage_person`, `manage_tag`, `manage_helper`, `helper_action`, `manage_hacs`, `manage_trace`, `manage_blueprint`, `manage_update`, `manage_todo`, `manage_calendar`, `manage_camera`

## Example Requests

### Query All Entity States

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "query_entities",
    "arguments": {
      "mode": "current"
    }
  }
}
```

### Get Single Entity State

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

### Query Registry

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "get_registry",
    "arguments": {
      "type": "entities",
      "domain": "light"
    }
  }
}
```

### Browse Media

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

### Get Statistics

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

### Call a Service

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

### Create an Automation

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
