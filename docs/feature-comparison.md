> [README](../README.md) | [Configuration](configuration.md) | [Tools](tools.md) | [Access Control](access-control.md) | [Architecture](architecture.md) | [Troubleshooting](troubleshooting.md) | [Feature Comparison](feature-comparison.md) | [Integration Tests](integration-tests.md)

# Feature Comparison: ha-mcp vs. Official Home Assistant MCP Server

## Context

This document compares the features of `ha-mcp` (this project) with the official [Home Assistant MCP Server](https://www.home-assistant.io/integrations/mcp_server) integration.

---

## Architectural Differences

| Aspect               | ha-mcp                                     | Official HA MCP Server                                      |
| -------------------- | ------------------------------------------ | ----------------------------------------------------------- |
| **Type**             | Standalone Go binary (external server)     | HA integration (built-in)                                   |
| **Transport**        | HTTP JSON-RPC                              | Streamable HTTP                                             |
| **HA Communication** | WebSocket + REST API (Hybrid)              | Direct Python API (internal)                                |
| **Tool Design**      | 38 specialized tools with granular control | Dynamically generated tools from Assist API (~10 tools)     |
| **Authentication**   | Long-Lived Access Token                    | OAuth (IndieAuth) + Long-Lived Token                        |
| **Access Control**   | Tool-level filtering (read-only, whitelist/blacklist, action-level) | Entity-level exposure (Voice Assistant Exposure) |

---

## Access Control Comparison

| Feature                  | ha-mcp                                                                 | Official HA MCP                                    |
| ------------------------ | ---------------------------------------------------------------------- | -------------------------------------------------- |
| **Control Level**        | Tool and action level                                                  | Entity level                                       |
| **Read-Only Mode**       | ✅ Yes (blocks all write operations)                                   | ❌ No (entity exposure only)                       |
| **Whitelist**            | ✅ Yes (specify allowed tools/actions)                                 | ✅ Yes (expose specific entities)                  |
| **Blacklist**            | ✅ Yes (block specific tools/actions)                                  | ❌ No                                              |
| **Glob Patterns**        | ✅ Yes (`manage_*:delete`, `get_*`)                                    | ❌ No                                              |
| **Category Filtering**   | ✅ Yes (`*:write`, `*:read`)                                           | ❌ No                                              |
| **Sub-Action Control**   | ✅ Yes (`manage_entity:delete`, `manage_*:delete`)                     | ❌ No                                              |
| **Granularity**          | Per tool, per action, per sub-action                                   | Per entity                                         |
| **Use Case**             | Limit AI capabilities (e.g., read-only monitoring, block deletions)    | Limit entity visibility (e.g., hide private rooms) |
| **Schema Modification**  | ✅ Yes (filtered tools show only allowed actions in schema)            | ❌ No                                              |
| **Runtime Enforcement**  | ✅ Yes (blocked actions return error)                                  | ✅ Yes (unexposed entities invisible)              |

**Summary:** Both provide access control at different levels:
- **ha-mcp**: Controls *what the AI can do* (tool/action level)
- **Official**: Controls *what the AI can see* (entity level)

For maximum security, you could theoretically use both: Official integration for entity filtering + ha-mcp's tool filtering if running both in parallel (though that's not a typical setup).

---

## Detailed Tool Comparison

### Entity Queries & Control

| Function                 | ha-mcp                                                                                                                                                      | Official HA MCP                                         |
| ------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------- |
| Query entity states      | `query_entities` mode=current (Filter by domain, state, name, device_class; group_by: domain, area_id, device_class, integration; Pagination; natural/json) | `HassGetState`, `GetLiveContext` (all exposed entities) |
| Query single entity      | `get_state` (natural/json format)                                                                                                                           | `HassGetState`                                          |
| Entity on/off/toggle     | `call_service` (any service)                                                                                                                                | `HassTurnOn`, `HassTurnOff`, `HassTurnToggle`           |
| Query temperature        | via `get_state`                                                                                                                                             | `HassGetTemperature` (specialized)                      |
| Query weather            | via `get_state`                                                                                                                                             | `HassGetWeather` (specialized)                          |
| List domains             | `query_entities` mode=domains                                                                                                                               | ------------------------------------------------------- |
| Entity history           | `query_entities` mode=history (time range, filter, pagination, natural/json)                                                                                | ------------------------------------------------------- |
| Entity statistics        | `query_entities` mode=statistics (long-term data, pagination, natural/json)                                                                                 | ------------------------------------------------------- |
| Presence analysis        | `query_entities` mode=presence (person/tracker correlation, natural/json)                                                                                   | ------------------------------------------------------- |
| Health detection         | `query_entities` mode=health (detect unavailable/unknown/disabled/orphaned/stale entities, multi-category filter, natural/json)                             | ------------------------------------------------------- |
| Entity registry delete   | `manage_entity` action=delete (remove entity from registry)                                                                                                 | ------------------------------------------------------- |
| Device health check      | `query_devices` mode=health (detect disabled/orphaned/error devices, multi-category filter, natural/json)                                                   | ------------------------------------------------------- |
| Device registry delete   | `manage_device` action=delete (remove device from registry; integration must support removal)                                                               | ------------------------------------------------------- |
| Cover control            | `call_service` (domain=cover)                                                                                                                               | `HassOpenCover`, `HassCloseCover`                       |
| Date/Time                | `get_datetime`                                                                                                                                              | `GetDateTime`                                           |

### Automations

| Function                  | ha-mcp                                                                                         | Official HA MCP   |
| ------------------------- | ---------------------------------------------------------------------------------------------- | ----------------- |
| List automations          | `manage_automation` action=list                                                                | ----------------- |
| Automation details        | `manage_automation` action=get                                                                 | ----------------- |
| Create automation         | `manage_automation` action=create                                                              | ----------------- |
| Edit automation           | `manage_automation` action=update                                                              | ----------------- |
| Delete automation         | `manage_automation` action=delete                                                              | ----------------- |
| Enable/disable automation | `manage_automation` action=toggle                                                              | ----------------- |
| Automation coverage       | `manage_automation` action=coverage (analyze areas/entities without automations, natural/json) | ----------------- |
| Patch automation (RFC 6902)| `manage_automation` action=patch                                                               | N/A               |

### Scripts

| Function       | ha-mcp                         | Official HA MCP                                    |
| -------------- | ------------------------------ | -------------------------------------------------- |
| List scripts   | `manage_script` action=list    | -------------------------------------------------- |
| Script details | `manage_script` action=get     | -------------------------------------------------- |
| Create script  | `manage_script` action=create  | -------------------------------------------------- |
| Edit script    | `manage_script` action=update  | -------------------------------------------------- |
| Delete script  | `manage_script` action=delete  | -------------------------------------------------- |
| Execute script | `manage_script` action=execute | `ScriptTool` (exposed scripts as individual tools) |

### Scenes

| Function       | ha-mcp                         | Official HA MCP    |
| -------------- | ------------------------------ | ------------------ |
| List scenes    | `manage_scene` action=list     | ------------------ |
| Scene details  | `manage_scene` action=get      | ------------------ |
| Create scene   | `manage_scene` action=create   | ------------------ |
| Edit scene     | `manage_scene` action=update   | ------------------ |
| Delete scene   | `manage_scene` action=delete   | ------------------ |
| Activate scene | `manage_scene` action=activate | via Intent/Service |

### Helpers (Input Helpers)

| Function         | ha-mcp                                                                      | Official HA MCP                      |
| ---------------- | --------------------------------------------------------------------------- | ------------------------------------ |
| List helpers     | `manage_helper` action=list                                                 | ------------------------------------ |
| Create helper    | `manage_helper` action=create (26 types)                                    | ------------------------------------ |
| Delete helper    | `manage_helper` action=delete                                               | ------------------------------------ |
| Helper details   | `manage_helper` action=get_details (all 26 helper types; natural/json)      | ------------------------------------ |
| Helper actions   | `helper_action` (toggle, set, increment, etc.)                              | via `call_service` Intents (limited) |
| Timer management | `helper_action` (start/pause/cancel/finish)                                 | Timer Intents (HassStartTimer, etc.) |

### Registry & System

| Function        | ha-mcp                                                                           | Official HA MCP         |
| --------------- | -------------------------------------------------------------------------------- | ----------------------- |
| Entity registry        | `get_registry` type=entities                                                                                 | ----------------------- |
| Entity registry update | `manage_entity` (actions: get, update, delete; fields: name, icon, area_id, disabled_by, hidden_by, labels, aliases) | ----------------------- |
| Device registry        | `get_registry` type=devices                                                                                          | ----------------------- |
| Device registry update | `manage_device` (actions: get, update, delete; fields: name_by_user, area_id, disabled_by, labels)                   | ----------------------- |
| Area registry          | `get_registry` type=areas                                                                                    | Area context in prompts |
| Area management        | `manage_area` (actions: list, get, create, update, delete; format: natural/json)                             | ----------------------- |
| Label management       | `manage_label` (actions: list, get, create, update, delete; format: natural/json)                            | ----------------------- |
| Floor management       | `manage_floor` (actions: list, get, create, update, delete; format: natural/json)                            | ----------------------- |
| Zone management        | `manage_zone` (actions: list, get, create, update, delete; format: natural/json)                             | ----------------------- |
| Person management      | `manage_person` (actions: list, get, create, update, delete; format: natural/json)                           | ----------------------- |
| Tag management         | `manage_tag` (actions: list, get, create, update, delete; format: natural/json)                              | ----------------------- |
| List services          | `list_services`                                                                                              | ----------------------- |
| System info     | `get_system_info`                                                                | ----------------------- |
| Validate config | `validate_config`                                                                | ----------------------- |
| Config entries  | `manage_config_entry` (actions: list, get; format: natural/json)                 | ----------------------- |

### Analysis & Advanced

| Function                | ha-mcp                                                                                          | Official HA MCP   |
| ----------------------- | ----------------------------------------------------------------------------------------------- | ----------------- |
| Entity analysis         | `analyze_entity` (references + registry metadata: platform, area, device, labels)              | ----------------- |
| Dependency analysis     | `get_entity_dependencies`                                                                       | ----------------- |
| Target analysis         | `analyze_target` (triggers/conditions/services)                                                 | ----------------- |
| Render Jinja2 templates | `render_template`                                                                               | ----------------- |
| Logbook                 | `get_logbook` (mode: entries, correlation; cause-effect analysis across entities, natural/json) | ----------------- |
| Statistics (long-term)  | `get_statistics`                                                                                | ----------------- |
| Lovelace dashboard      | `manage_dashboard` (list, get, create, update, delete, save_config; natural/json)               | ----------------- |

### Media

| Function          | ha-mcp              | Official HA MCP   |
| ----------------- | ------------------- | ----------------- |
| Browse media    | `browse_media`    | ----------------- |
| Sign media path | `sign_media_path` | ----------------- |

### HACS (Community Store)

> **Note**: Requires HACS add-on to be installed

| Function              | ha-mcp                                                                       | Official HA MCP   |
| --------------------- | ---------------------------------------------------------------------------- | ----------------- |
| HACS info             | `manage_hacs` action=info                                                    | ----------------- |
| List repositories     | `manage_hacs` action=list (filter: category, installed_only, pending_update) | ----------------- |
| Repository details    | `manage_hacs` action=get                                                     | ----------------- |
| Available releases    | `manage_hacs` action=releases                                                | ----------------- |
| Release notes         | `manage_hacs` action=release_notes                                           | ----------------- |
| Critical repositories | `manage_hacs` action=critical                                                | ----------------- |
| Download/install      | `manage_hacs` action=download (optional version)                             | ----------------- |
| Uninstall             | `manage_hacs` action=uninstall                                               | ----------------- |
| Add custom repository | `manage_hacs` action=add_repository                                          | ----------------- |
| Remove repository     | `manage_hacs` action=remove_repository                                       | ----------------- |
| Refresh repository    | `manage_hacs` action=refresh                                                 | ----------------- |
| Toggle beta versions  | `manage_hacs` action=toggle_beta                                             | ----------------- |

### Calendar & Todo

| Function           | ha-mcp                                                                                    | Official HA MCP     |
| ------------------ | ----------------------------------------------------------------------------------------- | ------------------- |
| List calendars     | `manage_calendar` action=list                                                             | ------------------- |
| Calendar events    | `manage_calendar` action=get_events                                                       | `CalendarGetEvents` |
| Create event       | `manage_calendar` action=create_event (supports date/datetime, location, description)    | ------------------- |
| Delete event       | `manage_calendar` action=delete_event (supports recurrence)                               | ------------------- |
| List todo lists    | `manage_todo` action=list                                                                 | ------------------- |
| Get todo items     | `manage_todo` action=get_items (status_filter: needs_action, completed)                  | `TodoGetItems`      |
| Add todo item      | `manage_todo` action=add_item (supports due_date, due_datetime, description)             | ------------------- |
| Update todo item   | `manage_todo` action=update_item (rename, status, description, due dates)                | ------------------- |
| Remove todo item   | `manage_todo` action=remove_item                                                          | ------------------- |

### Traces & Blueprints

| Function                | ha-mcp                                                        | Official HA MCP |
| ----------------------- | ------------------------------------------------------------- | --------------- |
| List execution traces   | `manage_trace` action=list (domain: automation, script)       | --------------- |
| Get trace details       | `manage_trace` action=get (execution path, trigger, actions)  | --------------- |
| Debug automation        | `manage_trace` action=debug (config+trace+trigger states+logbook in one call) | --------------- |
| List blueprints         | `manage_blueprint` action=list (domain: automation, script)   | --------------- |
| Import blueprint        | `manage_blueprint` action=import (from URL)                   | --------------- |

### Updates

| Function        | ha-mcp                                                              | Official HA MCP |
| --------------- | ------------------------------------------------------------------- | --------------- |
| List updates    | `manage_update` action=list (pending_only filter)                   | --------------- |
| Release notes   | `manage_update` action=release_notes                                | --------------- |
| Install update  | `manage_update` action=install (optional version, backup support)   | --------------- |
| Skip update     | `manage_update` action=skip                                         | --------------- |

### Camera

| Function        | ha-mcp                                                    | Official HA MCP |
| --------------- | --------------------------------------------------------- | --------------- |
| Camera snapshot | `manage_camera` action=snapshot (returns image data)      | --------------- |
| Camera stream   | `manage_camera` action=stream (returns HLS stream URL)    | --------------- |

---

## Summary

### ha-mcp Strengths:
- **CRUD Operations**: Complete create/edit/delete for automations, scripts, scenes, and helpers
- **Registry Access**: Detailed access to entity, device, and area registries
- **Analysis**: Entity dependencies, automation targets, cross-references
- **Historical Data**: Entity history, logbook, long-term statistics
- **System Administration**: Config validation, config entries, service listing
- **Media**: Media browser, camera streams, signed URLs
- **Dashboard**: Read Lovelace configuration
- **Templates**: Jinja2 template rendering
- **Flexibility**: `call_service` can invoke *any* HA service
- **Output Formats**: Natural Language (LLM-optimized) and JSON
- **Pagination**: Comprehensive pagination for large datasets
- **Tool-Level Access Control**: Read-only mode, whitelist/blacklist, glob patterns, category-based filtering (`*:write`, `*:read`)

### Official HA MCP Strengths:
- **Simplicity**: Fewer tools, intent-based, easier for basic scenarios
- **Entity-Level Security**: Fine-grained entity exposure control (only whitelisted entities visible)
- **No Infrastructure**: Runs inside HA itself, no external server needed
- **OAuth Support**: Standards-compliant authentication

### Feature Gaps in ha-mcp:
1. **Entity-Level Access Control** (ha-mcp provides tool-level access control instead)

### Feature Gaps in Official HA MCP:
1. No CRUD for automations/scripts/scenes/helpers
2. No registry queries
3. No history/statistics/logbook
4. No analysis tools (dependencies, targets)
5. No template rendering
6. No media tools
7. No dashboard insights
8. No config validation
9. No pagination
10. No `call_service` for arbitrary services
