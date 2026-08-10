> [README](../README.md) | [Configuration](configuration.md) | [Tools](tools.md) | [Access Control](access-control.md) | [Architecture](architecture.md) | [Troubleshooting](troubleshooting.md) | [Feature Comparison](feature-comparison.md) | [Integration Tests](integration-tests.md)

# Feature Comparison: ha-mcp vs. Community ha-mcp vs. Official HA MCP Server

## Context

Three MCP server projects expose Home Assistant functionality to AI assistants:

| Project | Description |
| ------- | ----------- |
| **ha-mcp** (this project) | Go binary, 41 specialized tools, HTTP JSON-RPC transport |
| **Community ha-mcp** ([homeassistant-ai/ha-mcp](https://github.com/homeassistant-ai/ha-mcp)) | Python/FastMCP package, 95+ tools, stdio/SSE/HTTP/WebSocket transport |
| **Official HA MCP Server** (built-in integration) | HA integration, ~10 intent-based tools, Streamable HTTP |

---

## Architectural Differences

| Aspect               | ha-mcp                                     | Community ha-mcp                                             | Official HA MCP Server                                      |
| -------------------- | ------------------------------------------ | ------------------------------------------------------------ | ----------------------------------------------------------- |
| **Type**             | Standalone Go binary (external server)     | Python package (pip/uvx/Docker/Add-on)                       | HA integration (built-in)                                   |
| **Transport**        | HTTP JSON-RPC                              | stdio, SSE, HTTP, WebSocket                                  | Streamable HTTP                                             |
| **HA Communication** | WebSocket + REST API (Hybrid)              | WebSocket + REST API                                         | Direct Python API (internal)                                |
| **Tool Design**      | 41 specialized tools with granular control | 95+ specialized tools                                        | Dynamically generated tools from Assist API (~10 tools)     |
| **Authentication**   | Long-Lived Access Token                    | Long-Lived Token + OAuth (beta)                              | OAuth (IndieAuth) + Long-Lived Token                        |
| **Access Control**   | Tool-level filtering (read-only, whitelist/blacklist, action-level) | Entity/Service allow/deny lists with wildcard patterns | Entity-level exposure (Voice Assistant Exposure) |

---

## Access Control Comparison

| Feature                  | ha-mcp                                                                 | Community ha-mcp                                              | Official HA MCP                                    |
| ------------------------ | ---------------------------------------------------------------------- | ------------------------------------------------------------- | -------------------------------------------------- |
| **Control Level**        | Tool and action level                                                  | Entity and service level                                      | Entity level                                       |
| **Read-Only Mode**       | ✅ Yes (blocks all write operations)                                   | ❌ No                                                         | ❌ No (entity exposure only)                       |
| **Whitelist**            | ✅ Yes (specify allowed tools/actions)                                 | ✅ Yes (`ALLOW_ENTITIES`, `ALLOW_SERVICES`)                   | ✅ Yes (expose specific entities)                  |
| **Blacklist**            | ✅ Yes (block specific tools/actions)                                  | ✅ Yes (`DENY_ENTITIES`, `DENY_SERVICES`)                     | ❌ No                                              |
| **Glob Patterns**        | ✅ Yes (`manage_*:delete`, `get_*`)                                    | ✅ Yes (wildcard patterns on entity/service names)            | ❌ No                                              |
| **Category Filtering**   | ✅ Yes (`*:write`, `*:read`)                                           | ❌ No (entity/service level only)                             | ❌ No                                              |
| **Sub-Action Control**   | ✅ Yes (`manage_entity:delete`, `manage_*:delete`)                     | ❌ No                                                         | ❌ No                                              |
| **Granularity**          | Per tool, per action, per sub-action                                   | Per entity, per service                                       | Per entity                                         |
| **Use Case**             | Limit AI capabilities (e.g., read-only monitoring, block deletions)    | Restrict accessible entities/services                         | Limit entity visibility (e.g., hide private rooms) |
| **Schema Modification**  | ✅ Yes (filtered tools show only allowed actions in schema)            | ❌ No                                                         | ❌ No                                              |
| **Runtime Enforcement**  | ✅ Yes (blocked actions return error)                                  | ✅ Yes (filtered entities/services rejected)                  | ✅ Yes (unexposed entities invisible)              |

**Summary:** All three provide access control at different levels:
- **ha-mcp**: Controls *what the AI can do* (tool/action level)
- **Community ha-mcp**: Controls *which entities/services the AI can access*
- **Official**: Controls *what the AI can see* (entity level)


---

## Detailed Tool Comparison

### Entity Queries & Control

| Function                 | ha-mcp                                                                                                                                                      | Community ha-mcp                                                            | Official HA MCP                                         |
| ------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------- | ------------------------------------------------------- |
| Query entity states      | `query_entities` mode=current (Filter by domain, state, name, device_class; group_by: domain, area_id, device_class, integration; Pagination; natural/json) | `ha_get_overview`, `ha_get_bulk_status` (multi-entity status)               | `HassGetState`, `GetLiveContext` (all exposed entities) |
| Fuzzy entity search      | `query_entities` mode=current (name/domain filter)                                                                                                          | `ha_search_entities` (fuzzy), `ha_deep_search` (config file search)         | ---                                                     |
| Query single entity      | `get_state` (natural/json format)                                                                                                                           | `ha_get_state`                                                              | `HassGetState`                                          |
| Entity on/off/toggle     | `call_service` (any service)                                                                                                                                | `ha_call_service`, `ha_bulk_control` (multi-entity bulk operations)         | `HassTurnOn`, `HassTurnOff`, `HassTurnToggle`           |
| Query temperature        | via `get_state`                                                                                                                                             | via `ha_get_state`                                                          | `HassGetTemperature` (specialized)                      |
| Query weather            | via `get_state`                                                                                                                                             | via `ha_get_state`                                                          | `HassGetWeather` (specialized)                          |
| List domains             | `query_entities` mode=domains                                                                                                                               | via `ha_get_overview`                                                       | ---                                                     |
| Entity history           | `query_entities` mode=history (time range, filter, pagination, natural/json)                                                                                | `ha_get_history`                                                            | ---                                                     |
| Entity statistics        | `query_entities` mode=statistics (long-term data, pagination, natural/json)                                                                                 | `ha_get_statistics`                                                         | ---                                                     |
| Presence analysis        | `query_entities` mode=presence (person/tracker correlation, natural/json)                                                                                   | ---                                                                         | ---                                                     |
| Health detection         | `query_entities` mode=health (detect unavailable/unknown/disabled/orphaned/stale entities, multi-category filter, natural/json)                             | ---                                                                         | ---                                                     |
| Entity registry delete   | `manage_entity` action=delete (remove entity from registry)                                                                                                 | ---                                                                         | ---                                                     |
| Device health check      | `query_devices` mode=health (detect disabled/orphaned/error devices, multi-category filter, natural/json)                                                   | ---                                                                         | ---                                                     |
| Device registry delete   | `manage_device` action=delete (remove device from registry; integration must support removal)                                                               | `ha_remove_device`                                                          | ---                                                     |
| Cover control            | `call_service` (domain=cover)                                                                                                                               | `ha_call_service`                                                           | `HassOpenCover`, `HassCloseCover`                       |
| Date/Time                | `get_datetime`                                                                                                                                              | via `ha_get_system_info`                                                    | `GetDateTime`                                           |

### Automations

| Function                  | ha-mcp                                                                                         | Community ha-mcp                                         | Official HA MCP   |
| ------------------------- | ---------------------------------------------------------------------------------------------- | -------------------------------------------------------- | ----------------- |
| List automations          | `manage_automation` action=list                                                                | `ha_config_get_automation` (list + get combined)         | ---               |
| Automation details        | `manage_automation` action=get                                                                 | `ha_config_get_automation`                               | ---               |
| Create automation         | `manage_automation` action=create                                                              | `ha_config_set_automation`                               | ---               |
| Edit automation           | `manage_automation` action=update                                                              | `ha_config_set_automation`                               | ---               |
| Delete automation         | `manage_automation` action=delete                                                              | `ha_config_remove_automation`                            | ---               |
| Enable/disable automation | `manage_automation` action=toggle                                                              | ---                                                      | ---               |
| Automation coverage       | `manage_automation` action=coverage (analyze areas/entities without automations, natural/json) | ---                                                      | ---               |
| Patch automation (RFC 6902)| `manage_automation` action=patch                                                              | ---                                                      | N/A               |
| Automation traces         | `manage_trace` action=list/get/debug                                                           | `ha_get_automation_traces`                               | ---               |

### Scripts

| Function       | ha-mcp                         | Community ha-mcp                                 | Official HA MCP                                    |
| -------------- | ------------------------------ | ------------------------------------------------ | -------------------------------------------------- |
| List scripts   | `manage_script` action=list    | `ha_config_get_script` (list + get combined)     | ---                                                |
| Script details | `manage_script` action=get     | `ha_config_get_script`                           | ---                                                |
| Create script  | `manage_script` action=create  | `ha_config_set_script`                           | ---                                                |
| Edit script    | `manage_script` action=update  | `ha_config_set_script`                           | ---                                                |
| Delete script  | `manage_script` action=delete  | `ha_config_remove_script`                        | ---                                                |
| Execute script | `manage_script` action=execute | via `ha_call_service`                            | `ScriptTool` (exposed scripts as individual tools) |
| Patch script (RFC 6902) | `manage_script` action=patch | ---                                         | ---                                                |

### Scenes

| Function       | ha-mcp                         | Community ha-mcp      | Official HA MCP    |
| -------------- | ------------------------------ | --------------------- | ------------------ |
| List scenes    | `manage_scene` action=list     | ---                   | ---                |
| Scene details  | `manage_scene` action=get      | ---                   | ---                |
| Create scene   | `manage_scene` action=create   | ---                   | ---                |
| Edit scene     | `manage_scene` action=update   | ---                   | ---                |
| Delete scene   | `manage_scene` action=delete   | ---                   | ---                |
| Activate scene | `manage_scene` action=activate | via `ha_call_service` | via Intent/Service |

### Helpers (Input Helpers)

| Function         | ha-mcp                                                                      | Community ha-mcp                                                                     | Official HA MCP                      |
| ---------------- | --------------------------------------------------------------------------- | ------------------------------------------------------------------------------------ | ------------------------------------ |
| List helpers     | `manage_helper` action=list                                                 | `ha_config_list_helpers`                                                             | ---                                  |
| Create helper    | `manage_helper` action=create (26 types)                                    | `ha_config_set_helper` (subset of types)                                             | ---                                  |
| Delete helper    | `manage_helper` action=delete                                               | `ha_config_remove_helper`                                                            | ---                                  |
| Helper details   | `manage_helper` action=get_details (all 26 helper types; natural/json)      | ---                                                                                  | ---                                  |
| Helper actions   | `helper_action` (toggle, set, increment, etc.)                              | via `ha_call_service`                                                                | via `call_service` Intents (limited) |
| Timer management | `helper_action` (start/pause/cancel/finish)                                 | via `ha_call_service`                                                                | Timer Intents (HassStartTimer, etc.) |

### Registry & System

| Function               | ha-mcp                                                                                                                               | Community ha-mcp                                                                    | Official HA MCP         |
| ---------------------- | ------------------------------------------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------- | ----------------------- |
| Entity registry        | `get_registry` type=entities                                                                                                         | ---                                                                                 | ---                     |
| Entity registry update | `manage_entity` (actions: get, update, delete; fields: name, icon, area_id, disabled_by, hidden_by, labels, aliases)                 | `ha_rename_entity` (rename only)                                                    | ---                     |
| Device registry        | `get_registry` type=devices                                                                                                          | `ha_get_device`                                                                     | ---                     |
| Device registry update | `manage_device` (actions: get, update, delete; fields: name_by_user, area_id, disabled_by, labels)                                   | `ha_update_device`, `ha_remove_device`                                              | ---                     |
| Area registry          | `get_registry` type=areas                                                                                                            | `ha_config_list_areas`                                                              | Area context in prompts |
| Area management        | `manage_area` (actions: list, get, create, update, delete; format: natural/json)                                                     | `ha_config_list_areas`, `ha_config_set_area`, `ha_config_remove_area`               | ---                     |
| Label management       | `manage_label` (actions: list, get, create, update, delete; format: natural/json)                                                    | `ha_config_get_label`, `ha_config_set_label`, `ha_config_remove_label`, `ha_manage_entity_labels` | ---         |
| Floor management       | `manage_floor` (actions: list, get, create, update, delete; format: natural/json)                                                    | `ha_config_list_floors`, `ha_config_set_floor`, `ha_config_remove_floor`            | ---                     |
| Zone management        | `manage_zone` (actions: list, get, create, update, delete; format: natural/json)                                                     | `ha_get_zone`, `ha_set_zone`, `ha_remove_zone`                                      | ---                     |
| Group management       | via `manage_helper` (group type)                                                                                                     | `ha_config_list_groups`, `ha_config_set_group`, `ha_config_remove_group`            | ---                     |
| Person management      | `manage_person` (actions: list, get, create, update, delete; format: natural/json)                                                   | ---                                                                                 | ---                     |
| Tag management         | `manage_tag` (actions: list, get, create, update, delete; format: natural/json)                                                      | ---                                                                                 | ---                     |
| List services          | `list_services`                                                                                                                      | via `ha_get_integration`                                                            | ---                     |
| System info            | `get_system_info`                                                                                                                    | `ha_get_system_info`                                                                | ---                     |
| Validate config        | `validate_config`                                                                                                                    | `ha_check_config`                                                                   | ---                     |
| Config entries         | `manage_config_entry` (actions: list, get, delete; format: natural/json)                                                             | Integration options flow tools                                                      | ---                     |
| Integration info       | via `manage_config_entry`                                                                                                            | `ha_get_integration`, `ha_get_entity_integration_source`                            | ---                     |

### Analysis & Advanced

| Function                | ha-mcp                                                                                          | Community ha-mcp        | Official HA MCP   |
| ----------------------- | ----------------------------------------------------------------------------------------------- | ----------------------- | ----------------- |
| Entity analysis         | `analyze_entity` (references + registry metadata: platform, area, device, labels)              | ---                     | ---               |
| Dependency analysis     | `get_entity_dependencies`                                                                       | ---                     | ---               |
| Target analysis         | `analyze_target` (triggers/conditions/services)                                                 | ---                     | ---               |
| Render Jinja2 templates | `render_template`                                                                               | `ha_eval_template`      | ---               |
| Logbook                 | `get_logbook` (mode: entries, correlation; cause-effect analysis across entities, natural/json) | `ha_get_logbook`        | ---               |
| Statistics (long-term)  | `get_statistics`                                                                                | `ha_get_statistics`     | ---               |

### Dashboards

| Function                | ha-mcp                                                                            | Community ha-mcp                                                                    | Official HA MCP   |
| ----------------------- | --------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------- | ----------------- |
| List dashboards         | `manage_dashboard` action=list                                                    | `ha_config_get_dashboard`                                                           | ---               |
| Get dashboard config    | `manage_dashboard` action=get                                                     | `ha_config_get_dashboard`                                                           | ---               |
| Create/update dashboard | `manage_dashboard` action=create/update/save_config/patch (RFC 6902 JSON Patch)   | `ha_config_set_dashboard`                                                           | ---               |
| Delete dashboard        | `manage_dashboard` action=delete                                                  | `ha_config_delete_dashboard`                                                        | ---               |
| Dashboard UI guidance   | ---                                                                               | `ha_get_dashboard_guide` (domain-specific UI guidance)                              | ---               |
| Card documentation      | ---                                                                               | `ha_get_card_documentation` (card-type reference docs)                              | ---               |

### Media

| Function          | ha-mcp              | Community ha-mcp   | Official HA MCP   |
| ----------------- | ------------------- | ------------------ | ----------------- |
| Browse media      | `browse_media`      | ---                | ---               |
| Sign media path   | `sign_media_path`   | ---                | ---               |

### Camera

| Function        | ha-mcp                                                 | Community ha-mcp                           | Official HA MCP |
| --------------- | ------------------------------------------------------ | ------------------------------------------ | --------------- |
| Camera snapshot | `manage_camera` action=snapshot (returns image data)   | `ha_get_camera_image` (snapshot only)      | ---             |
| Camera stream   | `manage_camera` action=stream (returns HLS stream URL) | ---                                        | ---             |

### HACS (Community Store)

> **Note**: Requires HACS add-on to be installed

| Function              | ha-mcp                                                                       | Community ha-mcp   | Official HA MCP   |
| --------------------- | ---------------------------------------------------------------------------- | ------------------ | ----------------- |
| HACS info             | `manage_hacs` action=info                                                    | ---                | ---               |
| List repositories     | `manage_hacs` action=list (filter: category, installed_only, pending_update) | ---                | ---               |
| Repository details    | `manage_hacs` action=get                                                     | ---                | ---               |
| Available releases    | `manage_hacs` action=releases                                                | ---                | ---               |
| Release notes         | `manage_hacs` action=release_notes                                           | ---                | ---               |
| Critical repositories | `manage_hacs` action=critical                                                | ---                | ---               |
| Download/install      | `manage_hacs` action=download (optional version)                             | ---                | ---               |
| Uninstall             | `manage_hacs` action=uninstall                                               | ---                | ---               |
| Add custom repository | `manage_hacs` action=add_repository                                          | ---                | ---               |
| Remove repository     | `manage_hacs` action=remove_repository                                       | ---                | ---               |
| Refresh repository    | `manage_hacs` action=refresh                                                 | ---                | ---               |
| Toggle beta versions  | `manage_hacs` action=toggle_beta                                             | ---                | ---               |

### Calendar & Todo

| Function           | ha-mcp                                                                                    | Community ha-mcp                                                                                          | Official HA MCP     |
| ------------------ | ----------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------- | ------------------- |
| List calendars     | `manage_calendar` action=list                                                             | ---                                                                                                       | ---                 |
| Calendar events    | `manage_calendar` action=get_events                                                       | `ha_config_get_calendar_events`                                                                           | `CalendarGetEvents` |
| Create event       | `manage_calendar` action=create_event (supports date/datetime, location, description)     | `ha_config_set_calendar_event`                                                                            | ---                 |
| Delete event       | `manage_calendar` action=delete_event (supports recurrence)                               | `ha_config_remove_calendar_event`                                                                         | ---                 |
| List todo lists    | `manage_todo` action=list                                                                 | ---                                                                                                       | ---                 |
| Get todo items     | `manage_todo` action=get_items (status_filter: needs_action, completed)                   | `ha_get_todo`                                                                                             | `TodoGetItems`      |
| Add todo item      | `manage_todo` action=add_item (supports due_date, due_datetime, description)              | `ha_add_todo_item`                                                                                        | ---                 |
| Update todo item   | `manage_todo` action=update_item (rename, status, description, due dates)                 | `ha_update_todo_item`                                                                                     | ---                 |
| Remove todo item   | `manage_todo` action=remove_item                                                          | `ha_remove_todo_item`                                                                                     | ---                 |

### Traces & Blueprints

| Function                | ha-mcp                                                        | Community ha-mcp                                            | Official HA MCP |
| ----------------------- | ------------------------------------------------------------- | ----------------------------------------------------------- | --------------- |
| List execution traces   | `manage_trace` action=list (domain: automation, script)       | `ha_get_automation_traces` (automations only)               | ---             |
| Get trace details       | `manage_trace` action=get (execution path, trigger, actions)  | `ha_get_automation_traces`                                  | ---             |
| Debug automation        | `manage_trace` action=debug (config+trace+trigger states+logbook in one call) | ---                                           | ---             |
| List blueprints         | `manage_blueprint` action=list (domain: automation, script)   | `ha_list_blueprints`                                        | ---             |
| Get blueprint           | ---                                                           | `ha_get_blueprint`                                          | ---             |
| Import blueprint        | `manage_blueprint` action=import (from URL)                   | `ha_import_blueprint`                                       | ---             |

### Updates

| Function        | ha-mcp                                                              | Community ha-mcp                           | Official HA MCP |
| --------------- | ------------------------------------------------------------------- | ------------------------------------------ | --------------- |
| List updates    | `manage_update` action=list (pending_only filter)                   | `ha_get_updates` (list only)               | ---             |
| Release notes   | `manage_update` action=release_notes                                | ---                                        | ---             |
| Install update  | `manage_update` action=install (optional version, backup support)   | ---                                        | ---             |
| Skip update     | `manage_update` action=skip                                         | ---                                        | ---             |

### System Administration

| Function               | ha-mcp                                            | Community ha-mcp                                            | Official HA MCP |
| ---------------------- | ------------------------------------------------- | ----------------------------------------------------------- | --------------- |
| Restart HA             | ---                                               | `ha_restart`                                                | ---             |
| Reload core config     | ---                                               | `ha_reload_core`                                            | ---             |
| System health          | ---                                               | `ha_get_system_health`                                      | ---             |
| Create backup          | ---                                               | `ha_backup_create`                                          | ---             |
| Restore backup         | ---                                               | `ha_backup_restore`                                         | ---             |
| Add-on info            | ---                                               | `ha_get_addon`                                              | ---             |

### MCP Resources & Knowledge Base

| Function                    | ha-mcp   | Community ha-mcp                                                         | Official HA MCP |
| --------------------------- | -------- | ------------------------------------------------------------------------ | --------------- |
| Bundled domain knowledge    | `get_skill` tool + `skill://ha-mcp/*` resources (7 topics)          | MCP resources via `skill://` URIs (HA domain-specific guidance for AI)   | ---             |

---

## Summary

### ha-mcp Strengths:
- **LLM-Optimised Tool Architecture**: 41 consolidated tools with `action` parameters reduce tool-selection errors. LLM benchmarks show accuracy degrades significantly above ~40 parallel tools; the `tool + action` two-level hierarchy is easier to reason over than 95+ equally-ranked options
- **CRUD Operations**: Complete create/edit/delete for automations, scripts, scenes, and helpers
- **Registry Access**: Detailed access to entity, device, and area registries
- **Analysis**: Entity dependencies, automation targets, cross-references — unique to this project
- **Historical Data**: Entity history, logbook, long-term statistics
- **System Administration**: Config validation, config entries, service listing
- **Media**: Media browser, camera streams (HLS), signed URLs
- **Output Formats**: Natural Language (LLM-optimized) and JSON
- **Pagination**: Comprehensive pagination for large datasets
- **Tool-Level Access Control**: Read-only mode, whitelist/blacklist, glob patterns, category-based filtering (`*:write`, `*:read`)
- **HACS Integration**: Full HACS repository management including install/uninstall
- **Advanced Automation**: Trace debugging (config+trace+logbook in one call), automation coverage analysis, RFC 6902 JSON Patch
- **Scene CRUD**: Full scene lifecycle management — absent from Community ha-mcp
- **Person & Tag Registries**: Manage persons and tags — absent from Community ha-mcp

### Community ha-mcp Strengths:
- **Fine-Grained Tools**: 95+ individual tools, each with a narrow, well-defined purpose — but note that higher tool count also increases the LLM's tool-selection burden; several functional areas (scenes, HACS, media browser, analysis) are absent despite the larger number
- **Fuzzy Search**: `ha_search_entities` and `ha_deep_search` for config file search
- **Bulk Operations**: `ha_bulk_control` for controlling multiple entities in one call
- **Dashboard Documentation**: `ha_get_dashboard_guide` and `ha_get_card_documentation` — no equivalent elsewhere
- **System Lifecycle**: `ha_restart`, `ha_reload_core`, `ha_get_system_health`
- **Backup & Restore**: `ha_backup_create`, `ha_backup_restore` — unique to this project
- **Add-on Management**: `ha_get_addon`
- **OAuth Support**: Long-Lived Token + OAuth (beta)
- **Group Registry**: Dedicated group CRUD tools (`ha_config_list_groups`, etc.)
- **Entity Labels**: `ha_manage_entity_labels` for bulk label assignment
- **MCP Resources**: Bundled domain knowledge via `skill://` URIs
- **Transport Flexibility**: stdio, SSE, HTTP, WebSocket — broadest client compatibility
- **Deployment Options**: pip, uvx, Docker, HA Add-on

### Official HA MCP Server Strengths:
- **Simplicity**: Fewer tools, intent-based, easier for basic scenarios
- **Entity-Level Security**: Fine-grained entity exposure control (only whitelisted entities visible)
- **No Infrastructure**: Runs inside HA itself, no external server needed
- **OAuth Support**: Standards-compliant IndieAuth authentication

### Feature Gaps

| Gap | ha-mcp | Community ha-mcp | Official HA MCP |
| --- | ------ | ---------------- | --------------- |
| Entity-level access control | ❌ (tool-level instead) | ✅ | ✅ |
| Backup / restore | ❌ | ✅ | ❌ |
| HA restart / reload | ❌ | ✅ | ❌ |
| Fuzzy entity search | ❌ (filter-based) | ✅ | ❌ |
| Bulk entity control | ❌ | ✅ | ❌ |
| Dashboard UI docs/guides | ❌ | ✅ | ❌ |
| HACS integration | ✅ | ❌ | ❌ |
| Scene CRUD | ✅ | ❌ | ❌ |
| Automation coverage analysis | ✅ | ❌ | ❌ |
| Entity dependency graph | ✅ | ❌ | ❌ |
| RFC 6902 JSON Patch | ✅ | ❌ | ❌ |
| Read-only mode | ✅ | ❌ | ❌ |
| Camera stream (HLS) | ✅ | ❌ | ❌ |
| Media browser | ✅ | ❌ | ❌ |
| MCP resources / skill URIs | ✅ | ✅ | ❌ |
| No external server needed | ❌ | ❌ | ✅ |
| Standards OAuth (IndieAuth) | ❌ | ❌ (beta only) | ✅ |
