---
name: ha-mcp-guide
description: "Use when working with Home Assistant via the ha-mcp MCP server. Examples: \"create automation\", \"toggle light\", \"list entities in living room\", \"patch dashboard\", \"create helper\", \"query sensors\"."
---

# ha-mcp Guide

Entry point for all ha-mcp work. Read this first, then follow the task-specific skill below.

## When to Use

Whenever the user mentions HA entities, automations, scripts, scenes, helpers, dashboards, areas, devices, services, or anything related to Home Assistant.

## Always Do

- Use `format=natural` for reads — LLM-optimized prose, 30-60% fewer tokens than JSON.
- Call `manage_*:get` **before** `update` or `patch` — `list` returns summary only, Config is not populated.
- Use `manage_automation` / `manage_script` / `manage_scene` for CRUD — never `call_service` for lifecycle.
- Smart Wait is server-side — after mutations the server polls and appends `State changes: entity: old → new` — do not poll manually.
- All IDs accept entity_id, config UUID, or friendly-name substring — use whichever you have.

## Skills

| Task                                          | Skill               |
| --------------------------------------------- | ------------------- |
| Which tool / action to use?                   | `ha-mcp-tools`      |
| Unexpected error or behavior                  | `ha-mcp-gotchas`    |
| Edit automation / script / scene / dashboard  | `ha-mcp-patching`   |
| Reduce tokens or batch operations             | `ha-mcp-efficiency` |

## Tool Cheat Sheet

**Entities**
- `query_entities` — query all entities by state, domain, area, presence, health, history, statistics (modes: current/history/statistics/domains/presence/health)
- `query_devices` — device health check (disabled, orphaned, config errors)
- `get_state` — single entity state + attributes
- `analyze_entity` — entity usage in automations/scripts/scenes + registry metadata
- `get_entity_dependencies` — all entities an automation/script depends on

**Registries**
- `get_registry` — read entities/devices/areas registry in bulk
- `manage_area` — area CRUD + label/alias modes + include_entities/include_automations on get
- `manage_entity` — entity registry update/delete (rename, reassign area, labels, aliases)
- `manage_device` — device registry update/delete (labels, area, disable)
- `manage_label` / `manage_floor` / `manage_zone` / `manage_person` / `manage_tag` — registry CRUD
- `manage_config_entry` — list/get config entries, delete a config entry (and its devices/entities)

**Automations / Scripts / Scenes**
- `manage_automation` — list/get/create/update/delete/toggle/coverage/patch
- `manage_script` — list/get/create/update/delete/execute/patch
- `manage_scene` — list/get/create/update/delete/activate/patch

**Helpers**
- `manage_helper` — lifecycle for all 26 helper types (list/create/update/delete/get_details)
- `helper_action` — runtime operations (toggle, set, increment, start, pause, cancel, …)

**Services**
- `call_service` — call any HA service (use for device control, not CRUD)
- `list_services` — browse available services with descriptions

**Dashboards / Media / Camera**
- `manage_dashboard` — dashboard CRUD + patch (match views/cards by title)
- `browse_media` / `sign_media_path` — media browsing + signed URLs
- `manage_camera` — snapshot (returns image) or stream (returns URL)

**System / Admin**
- `get_system_info` / `get_datetime` / `validate_config` / `render_template`
- `get_logbook` — logbook entries (mode: entries, correlation)
- `manage_update` — HA and add-on updates (list/release_notes/install/skip)
- `manage_trace` — automation/script execution traces (list/get/debug)
- `manage_blueprint` — blueprint list/import (public https:// URL required)
- `manage_calendar` / `manage_todo` / `manage_hacs`

**Analysis**
- `analyze_target` — capabilities of a target (triggers/conditions/services/entities/all)
