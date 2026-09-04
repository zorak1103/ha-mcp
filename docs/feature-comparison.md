> [README](../README.md) | [Configuration](configuration.md) | [Tools](tools.md) | [Access Control](access-control.md) | [Architecture](architecture.md) | [Troubleshooting](troubleshooting.md) | [Feature Comparison](feature-comparison.md) | [Integration Tests](integration-tests.md)

# Feature Comparison: ha-mcp vs. Community ha-mcp vs. Official HA MCP Server

## Context

Three primary MCP server solutions expose Home Assistant functionality to AI assistants:

| Project | Description |
| ------- | ----------- |
| **ha-mcp** (this project) | Standalone Go binary, 41 consolidated tools, HTTP JSON-RPC transport |
| **Community ha-mcp** ([homeassistant-ai/ha-mcp](https://github.com/homeassistant-ai/ha-mcp)) | Python/FastMCP package (v8.4.2), 88 tools, stdio and Streamable HTTP transport |
| **Official HA MCP Server** (built-in integration) | Home Assistant Core integration (`mcp_server`), ~15 intent tools, Streamable HTTP |

---

## 1. Architectural Overview and Runtime Characteristics

| Aspect               | ha-mcp (this project)                      | Community ha-mcp (homeassistant-ai)          | Official HA MCP Server (Core component)     |
| -------------------- | ------------------------------------------ | -------------------------------------------- | ------------------------------------------- |
| **Implementation**   | Go 1.27 static binary (zero runtime deps)  | Python 3.13+ / FastMCP 3.4                   | Python (HA Core component `mcp_server`)     |
| **Deployment**       | Standalone binary, Docker container        | HACS custom component, Add-on, Docker, PyPI  | Built into HA Core (zero setup required)    |
| **Client Transport** | HTTP JSON-RPC (standard endpoint)          | stdio, Streamable HTTP (`ha-mcp-web`)        | Streamable HTTP (`/api/mcp`), legacy SSE    |
| **HA Interface**     | WebSocket (primary) + REST (CRUD engine)   | REST + WebSocket (hybrid via httpx/aiohttp)  | Internal Python Assist API (`helpers.llm`)  |
| **Tool Topology**    | 41 consolidated, action-based tools        | 88 discrete, fine-grained tools              | ~15 intent tools exposed via Assist API     |
| **Authentication**   | Long-Lived Access Token                    | Long-Lived Token, secret path, OAuth 2.1     | OAuth 2.0 (IndieAuth) or Long-Lived Token   |
| **Caching Layer**    | In-memory singleflight TTL cache           | Short-TTL settings/tool overrides cache      | Direct internal state access                |
| **State Diffing**    | Smart Wait: polls post-mutation diffs      | Async operation status tracker               | None (fire-and-forget)                      |

---

## 2. Token Consumption and Schema Overhead

Tool schema size directly affects LLM context window limits and per-request inference cost.

| Metric                   | ha-mcp (this project)                  | Community ha-mcp (homeassistant-ai)    | Official HA MCP Server                     |
| ------------------------ | -------------------------------------- | -------------------------------------- | ------------------------------------------ |
| **Tool Count**           | 41 tools                               | 88 tools                               | ~15 tools (Assist intents)                 |
| **Schema Token Burden**  | Low (~3,500 tokens)                    | High (~15,000 to 20,000 tokens)        | Low (~2,000 tokens)                        |
| **Tool Search Required** | No (fits cleanly into all models)      | Yes (ships optional BM25 tool search)  | No                                         |
| **Output Formatter**     | Dual: Natural Language (compact) / JSON| Standard JSON                          | TextContent with JSON or YAML string blobs |
| **System Prompt Burden** | Minimal (skills exposed via resources) | Instructions injected via FastMCP      | High (embeds YAML dump of exposed entities)|

---

## 3. Feature and Tool Capabilities Matrix

### Entity Queries and Device Control

| Function                 | ha-mcp (this project)                                                     | Community ha-mcp                                      | Official HA MCP Server                     |
| ------------------------ | ------------------------------------------------------------------------- | ----------------------------------------------------- | ------------------------------------------ |
| Generic service calls    | `call_service` (arbitrary domain, service, target, data)                  | `ha_call_service`, `ha_bulk_control`                  | ❌ Not supported (intents only)            |
| Query entity states      | `query_entities` (modes: current, domains, history, statistics, health)  | `ha_get_overview`, `ha_get_state`                     | `homeassistant__GetLiveContext`            |
| Single entity state      | `get_state` (natural language or JSON)                                    | `ha_get_state`                                        | `homeassistant__GetLiveContext`            |
| Entity history & stats   | `query_entities` mode=history / mode=statistics                           | `ha_get_history`                                      | ❌ Not supported                           |
| Entity health audit      | `query_entities` mode=health (orphans, unavailable, stale)                | `ha_get_system_health` (system-wide only)             | ❌ Not supported                           |
| Presence correlation     | `query_entities` mode=presence (tracker and person correlation)           | ❌ Not supported                                      | ❌ Not supported                           |
| Device registry search   | `query_devices` (mode=health), `get_registry` type=devices, `manage_device` | `ha_get_device`                                       | Via context prompt only                    |
| Post-mutation diffing    | Smart Wait: returns `entity: old -> new` inline                           | `ha_get_operation_status` (separate call)             | ❌ Fire-and-forget                         |

### Automations, Scripts, and Scenes

| Function                 | ha-mcp (this project)                                                     | Community ha-mcp                                      | Official HA MCP Server                     |
| ------------------------ | ------------------------------------------------------------------------- | ----------------------------------------------------- | ------------------------------------------ |
| Automation CRUD          | `manage_automation` (list, get, create, update, delete, toggle)           | `ha_config_{get,set,remove}_automation`               | ❌ Not supported                           |
| RFC 6902 JSON Patch      | `manage_automation` action=patch (standard JSON pointer paths)           | ❌ Not supported (full overwrite only)               | ❌ Not supported                           |
| Semantic JSON Patch      | `manage_automation` action=patch (match by properties, dry_run preview)   | ❌ Not supported                                      | ❌ Not supported                           |
| Automation trace debug   | `manage_trace` (list, get, debug with unified state/logbook snapshot)     | `ha_get_automation_traces`                            | ❌ Not supported                           |
| Automation coverage      | `manage_automation` action=coverage (uncovered areas and entities)         | ❌ Not supported                                      | ❌ Not supported                           |
| Script CRUD & Execute    | `manage_script` (list, get, create, update, delete, execute, patch)      | `ha_config_{get,set,remove}_script`                   | One tool per exposed script (`script__*`)  |
| Scene CRUD & Activate    | `manage_scene` (list, get, create, update, delete, activate, patch)       | `ha_config_{get,set,remove}_scene`                    | Via `intent__HassTurnOn` if exposed        |

### Dashboards (Lovelace)

| Function                 | ha-mcp (this project)                                                     | Community ha-mcp                                      | Official HA MCP Server                     |
| ------------------------ | ------------------------------------------------------------------------- | ----------------------------------------------------- | ------------------------------------------ |
| List & read dashboards   | `manage_dashboard` action=list / action=get                               | `ha_config_get_dashboard`                             | ❌ Not supported                           |
| Create & update dashboard| `manage_dashboard` action=create / action=update / action=save_config      | `ha_config_set_dashboard`                             | ❌ Not supported                           |
| Card & entity search     | `manage_dashboard` action=find (deep scan across views, cards, chips)     | ❌ Not supported                                      | ❌ Not supported                           |
| Semantic card patch      | `manage_dashboard` action=patch (surgical card edits without full rewrite)| ❌ Not supported                                      | ❌ Not supported                           |

### Helpers (Input & Config Entry Helpers)

| Function                 | ha-mcp (this project)                                                     | Community ha-mcp                                      | Official HA MCP Server                     |
| ------------------------ | ------------------------------------------------------------------------- | ----------------------------------------------------- | ------------------------------------------ |
| Helper types supported   | 41 distinct helper types                                                  | WebSocket input helpers only (~6 types)               | ❌ Not supported                           |
| Multi-step Config Flows  | `statistics` (3 steps), `filter` (2 steps), `generic_thermostat`, `trend` | ❌ Not supported                                      | ❌ Not supported                           |
| Template subtypes        | 17 subtypes (15 domain-specific: `template_light`, `template_cover`, etc.)| ❌ Not supported                                      | ❌ Not supported                           |
| Helper mutations         | `manage_helper` (create, update, delete, get_details)                     | `ha_config_{list,set,remove}_helper`                  | ❌ Not supported                           |
| Helper actions           | `helper_action` (toggle, increment, set, timer controls)                  | Via `ha_call_service`                                 | Timer intents (if device supported)        |

### Analysis & Cross-References

| Function                 | ha-mcp (this project)                                                     | Community ha-mcp                                      | Official HA MCP Server                     |
| ------------------------ | ------------------------------------------------------------------------- | ----------------------------------------------------- | ------------------------------------------ |
| Entity blast radius      | `analyze_entity` (references, registry metadata, blast radius)            | ❌ Not supported                                      | ❌ Not supported                           |
| Entity dependencies      | `get_entity_dependencies` (upstream/downstream dependency graph)          | ❌ Not supported                                      | ❌ Not supported                           |
| Cross-config search      | `find_references` (scans automations, scripts, scenes, dashboards, Jinja)  | `ha_search` / `ha_deep_search` (text-based grep)      | ❌ Not supported                           |
| Action target analysis   | `analyze_target` (maps entities affected by action blocks)                | ❌ Not supported                                      | ❌ Not supported                           |
| Jinja2 template rendering| `render_template`                                                         | `ha_eval_template`                                    | ❌ Not supported                           |
| Logbook correlation      | `get_logbook` (entries, correlation mode for causal analysis)            | `ha_get_logbook`                                      | ❌ Not supported                           |

### Registries and System Administration

| Function                 | ha-mcp (this project)                                                     | Community ha-mcp                                      | Official HA MCP Server                     |
| ------------------------ | ------------------------------------------------------------------------- | ----------------------------------------------------- | ------------------------------------------ |
| Entity registry          | `get_registry` type=entities, `manage_entity` (name, icon, labels, area)   | `ha_rename_entity`                                    | ❌ Not supported                           |
| Device registry          | `get_registry` type=devices, `manage_device` (name, area, labels)         | `ha_update_device`, `ha_remove_device`                | Context prompt only                        |
| Area & Floor management  | `manage_area`, `manage_floor` (list, get, create, update, delete)         | `ha_config_{list,set,remove}_{area,floor}`            | Read-only area context in prompt           |
| Label & Zone management  | `manage_label`, `manage_zone` (list, get, create, update, delete)         | `ha_config_{get,set,remove}_label`, `ha_*_zone`       | ❌ Not supported                           |
| Person & Tag management  | `manage_person`, `manage_tag` (full CRUD lifecycle)                       | ❌ Not supported                                      | ❌ Not supported                           |
| Config entries           | `manage_config_entry` (list, get, delete)                                 | Options flow tools                                    | ❌ Not supported                           |
| HACS repository manager  | `manage_hacs` (list, get, install, uninstall, releases, notes, beta)      | `ha_get_hacs_info`, `ha_manage_hacs`                  | ❌ Not supported                           |
| Camera & Media           | `manage_camera` (snapshots, HLS stream), `browse_media`, `sign_media_path`| `ha_get_camera_image`                                 | ❌ Not supported                           |
| Calendar & To-do         | `manage_calendar` (CRUD), `manage_todo` (item CRUD)                       | `ha_config_*_calendar_events`, `ha_*_todo_item`       | `calendar__get_events`, `todo__get_items`  |
| Update management        | `manage_update` (list, release notes, install, skip)                      | `ha_manage_updates`                                   | ❌ Not supported                           |
| System administration    | `get_system_info`, `validate_config`, `manage_system_log`                 | `ha_restart`, `ha_reload_core`, `ha_manage_backup`    | ❌ Not supported                           |

---

## 4. Access Control, Safety, and Security

| Capability               | ha-mcp (this project)                                     | Community ha-mcp (homeassistant-ai)          | Official HA MCP Server                     |
| ------------------------ | --------------------------------------------------------- | -------------------------------------------- | ------------------------------------------ |
| **Control Model**        | Capability and action level (`ToolFilterEngine`)          | Middleware, approval policies, visibility    | Assist Voice Exposure settings             |
| **Read-Only Mode**       | ✅ Native flag (`--read-only` or `read_only: true`)       | ✅ Native flag (`READ_ONLY_MODE=true`)       | ❌ Not supported                           |
| **Action Filtering**     | ✅ Granular sub-actions (`manage_*:delete`, `*:write`)    | ❌ Tool-level enable/disable only            | ❌ Not supported                           |
| **Approval Policy Queue**| ❌ No (relies on client-side confirmation or filter)      | ✅ Yes (`policy/` approval queue in web UI)  | ❌ Not supported                           |
| **Entity Visibility**    | ❌ No (sees all entities accessible by LLAT)              | ✅ Yes (enforce mode conceals hidden items)  | ✅ Yes (Voice Assistant exposure list)     |
| **Secret Redaction**     | ❌ No                                                     | ✅ Yes (redacts passwords/tokens in config)  | ❌ Not supported                           |
| **Automatic Backups**    | ❌ No (manual backups via HA)                             | ✅ Yes (auto-backup before config edits)     | ❌ Not supported                           |
| **Admin Enforcement**    | Token user permissions apply                              | Token user permissions apply                 | ✅ `require_admin` config option           |

---

## 5. Summary and Selection Guide

### When to Choose ha-mcp (this project):
- **High-Performance & Low Footprint**: Runs as a single compiled Go binary (~30 MB) with zero external runtime dependencies and minimal RAM usage.
- **Token Efficiency**: 41 consolidated tools consume only ~3,500 tokens of schema context, compared to 15,000+ tokens for 88 individual tools.
- **Surgical Configuration Editing**: Full RFC 6902 and semantic property-based JSON patching for automations, scripts, scenes, and dashboards, avoiding destructive full-file overwrites.
- **Complex Helper Lifecycle**: Full support for 41 helper types, including multi-step Config Entry flows (`statistics`, `trend`, `filter`, `generic_thermostat`, `generic_hygrostat`) and 17 template subtypes (15 domain-specific subtypes alongside sensor/binary_sensor).
- **Deep Troubleshooting**: Advanced diagnostic tools including entity blast-radius analysis (`analyze_entity`), dependency graphs, cross-configuration reference search (`find_references`), and unified trace debugging.
- **Instant State Feedback**: Smart Wait automatically confirms post-mutation state changes (`entity: off -> on`) directly in the tool response.

### When to Choose Community ha-mcp (homeassistant-ai):
- **In-HA Deployment**: Can run directly inside Home Assistant as a HACS custom component or HA Add-on.
- **Human-in-the-Loop Approval**: Built-in web UI with approval policy queues for high-risk write calls.
- **Entity Concealment**: Visibility filtering to hide specific entities from LLM inspection.
- **HA Lifecycle Control**: Tools to restart Home Assistant, reload core configs, and trigger full backup/restore.
- **Broad Python Ecosystem**: Suitable for environments already standardized on Python and FastMCP.

### When to Choose the Official HA MCP Server:
- **Zero Additional Software**: Pre-installed in Home Assistant Core (HA 2025.2+), requiring no container or external process.
- **Strictly Scoped Voice Assistant Control**: Uses the existing Voice Assistant exposure toggles to limit what the LLM sees.
- **Simple Device Control**: Well-suited for basic smart home voice assistant prompts (lights, thermostats, media, to-do lists).
- **Standards-Based OAuth**: Built-in IndieAuth flow that integrates with web-based LLM clients without manually generating long-lived tokens.

---

## Feature Gaps Matrix

| Feature                               | ha-mcp (this project) | Community ha-mcp | Official HA MCP Server |
| ------------------------------------- | :-------------------: | :--------------: | :--------------------: |
| Single static binary (no runtime)     | ✅                    | ❌               | ❌ (Python Core)       |
| Consolidated low-token tool schema    | ✅                    | ❌               | ✅                     |
| RFC 6902 & Semantic JSON Patch        | ✅                    | ❌               | ❌                     |
| 41 helper types & multi-step flows    | ✅                    | ❌               | ❌                     |
| Cross-config reference search         | ✅                    | ❌               | ❌                     |
| Entity dependency & blast radius      | ✅                    | ❌               | ❌                     |
| Smart Wait post-mutation diffing      | ✅                    | ❌               | ❌                     |
| Natural language output mode          | ✅                    | ❌               | ❌                     |
| Generic service execution             | ✅                    | ✅               | ❌                     |
| Automation & script lifecycle         | ✅                    | ✅               | ❌                     |
| Dashboard card search & patch         | ✅                    | ❌               | ❌                     |
| HACS management                       | ✅                    | ✅               | ❌                     |
| In-HA / HACS component runtime        | ❌                    | ✅               | ✅ (Built-in)          |
| Web UI approval policy queue          | ❌                    | ✅               | ❌                     |
| Entity visibility concealment         | ❌                    | ✅               | ✅                     |
| Core restart & backup triggers        | ❌                    | ✅               | ❌                     |
| Native IndieAuth / OAuth 2.0          | ❌                    | ✅ (Beta)        | ✅                     |
