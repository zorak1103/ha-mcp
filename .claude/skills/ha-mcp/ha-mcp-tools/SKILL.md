---
name: ha-mcp-tools
description: "Use when choosing which ha-mcp tool or action to call. Examples: \"How do I list entities?\", \"create a helper\", \"turn on a light\", \"find devices in kitchen\", \"which action should I use?\"."
---

# ha-mcp Tool Discovery

## Intent → Tool

| User intent                                          | Tool + Action                                                          |
| ---------------------------------------------------- | ---------------------------------------------------------------------- |
| Turn on / off / toggle a device                      | `call_service` (domain=light/switch, service=turn_on/turn_off/toggle)  |
| Get a single entity's current state                  | `get_state`                                                            |
| List entities in an area / by domain / by state      | `query_entities` mode=current                                          |
| Find unavailable / stale / disabled entities         | `query_entities` mode=health                                           |
| Person / device tracker presence (home/away)         | `query_entities` mode=presence                                         |
| Historical state for a sensor over time              | `query_entities` mode=history                                          |
| Aggregated stats (min/max/mean) for a sensor         | `query_entities` mode=statistics                                       |
| Which entity domains exist in this HA instance?      | `query_entities` mode=domains                                          |
| Device health (orphaned / config errors)             | `query_devices` mode=health                                            |
| Where is entity X used? (automations, scripts, scenes, dashboards, template helpers) | `analyze_entity` — returns RFC 6901 paths to each reference location; `scanned_sources` lists what was actually searched |
| Which entities does automation X depend on?          | `get_entity_dependencies`                                              |
| Find a string/entity_id across ALL config types in one call | `find_references` — automations/scripts/scenes/dashboards/helper templates, match_mode substring/exact |
| Create / edit / delete an automation                 | `manage_automation` action=create/update/delete                        |
| Toggle / enable / disable an automation              | `manage_automation` action=toggle                                       |
| See automation trigger/condition/service coverage    | `manage_automation` action=coverage                                    |
| Add/remove a trigger or condition (surgical edit)    | `manage_automation` action=patch (semantic op)                         |
| Run a script                                         | `manage_script` action=execute                                         |
| Activate a scene                                     | `manage_scene` action=activate                                         |
| Create a helper (input, timer, counter, etc.)        | `manage_helper` action=create                                          |
| Set / toggle / start a helper value at runtime       | `helper_action`                                                        |
| Rename an entity in the registry                     | `manage_entity` action=update (new_entity_id field)                    |
| Assign an entity to an area                          | `manage_entity` action=update (area_id field)                          |
| Add / remove labels on entity or device              | `manage_entity` / `manage_device` action=update + label_mode           |
| List / create / rename an area                       | `manage_area` action=list/create/update                                |
| Check what config entries exist for an integration   | `manage_config_entry` action=list (domain filter)                      |
| Browse available services                            | `list_services`                                                        |
| Render a Jinja2 template                             | `render_template`                                                      |
| Get logbook entries / cause-effect correlation       | `get_logbook` mode=entries/correlation                                 |
| Check HA version / timezone                          | `get_system_info`                                                      |
| Validate configuration.yaml                          | `validate_config`                                                      |
| Manage a dashboard (create / edit / view)            | `manage_dashboard`                                                     |
| Locate content in a large dashboard (avoid size-limit errors) | `manage_dashboard` action=find search=&lt;string/entity_id&gt;   |
| Install / uninstall a HACS add-on                    | `manage_hacs` action=download/uninstall                                |
| View automation / script execution trace             | `manage_trace` action=list/get                                         |
| Import a blueprint                                   | `manage_blueprint` action=import                                       |
| Check for HA / add-on updates                        | `manage_update` action=list (pending_only=true)                        |
| Manage todo list items                               | `manage_todo`                                                          |
| Manage calendar events                               | `manage_calendar`                                                      |
| Read HA error/warning logs                           | `manage_system_log` action=list                                        |
| Clear the HA system log buffer                       | `manage_system_log` action=clear                                       |
| Get ha-mcp guidance on a topic                       | `get_skill` action=list / action=read skill=<slug>                     |

## Consolidated Tool Action Reference

| Tool                  | Read Actions                                     | Write Actions                                   |
| --------------------- | ------------------------------------------------ | ----------------------------------------------- |
| `manage_automation`   | list, get, coverage                              | create, update, delete, toggle, patch           |
| `manage_script`       | list, get                                        | create, update, delete, execute, patch          |
| `manage_scene`        | list, get                                        | create, update, delete, activate, patch         |
| `manage_helper`       | list, get_details                                | create, update, delete                          |
| `manage_dashboard`    | list, get, find                                  | create, update, delete, patch                   |
| `manage_area`         | list, get                                        | create, update, delete                          |
| `manage_entity`       | get                                              | update, delete                                  |
| `manage_device`       | get                                              | update, delete                                  |
| `manage_label`        | list, get                                        | create, update, delete                          |
| `manage_floor`        | list, get                                        | create, update, delete                          |
| `manage_zone`         | list, get                                        | create, update, delete                          |
| `manage_person`       | list, get                                        | create, update, delete                          |
| `manage_tag`          | list, get                                        | create, update, delete                          |
| `manage_config_entry` | list, get                                        | delete (removes entry + its devices/entities)   |
| `manage_hacs`         | info, list, get, releases, release_notes, critical | download, uninstall, add_repository, remove_repository, refresh, toggle_beta |
| `manage_trace`        | list, get, debug                                 | (none — read-only)                              |
| `manage_blueprint`    | list                                             | import                                          |
| `manage_update`       | list, release_notes                              | install, skip                                   |
| `manage_todo`         | list, get_items                                  | add_item, update_item, remove_item              |
| `manage_calendar`     | list, get_events                                 | create_event, delete_event                      |
| `manage_system_log`   | list                                             | clear                                           |
| `get_skill`           | list, read                                       | (none — read-only)                              |
| `query_entities`      | current, history, statistics, domains, presence, health | (none)                                   |
| `query_devices`       | health                                           | (none)                                          |
| `analyze_target`      | all, triggers, conditions, services, entities    | (none)                                          |
| `get_logbook`         | entries, correlation                             | (none)                                          |
| `helper_action`       | (contextual)                                     | toggle, set, increment, decrement, reset, calibrate, start, pause, cancel, finish, change, press, select, set_options, reload, add_entities, remove_entities |

`manage_helper`'s `update` action documents exactly which fields each of the
41 helper types accepts, generated from the `helperTypes` metadata table
(`internal/handlers/helpers_consolidated.go`) so it can't drift from the
code — read the tool's own `Description` for the per-type field list rather
than guessing.

## manage_automation / manage_script — create/update Parameters

Key non-obvious parameters for `create` and `update` actions:

**manage_automation:**
- alias (string, required on create): human-readable name; HA slugifies it to derive entity_id
- automation_id (string, optional on create): override the auto-generated slug (needed for non-ASCII aliases)
- mode (string, optional): `single` | `restart` | `queued` | `parallel` (HA default: single)
- max (integer, optional): concurrent run limit; only for mode=parallel|queued (HA default 10)
- triggers, conditions, actions (arrays): automation body

**manage_script:**
- alias (string, required on create): human-readable name; HA slugifies it to derive entity_id
- mode (string, optional): `single` | `restart` | `queued` | `parallel` (HA default: single)
- max (integer, optional): concurrent run limit; only for mode=parallel|queued (HA default 10)
- sequence (array): script steps
- script_id resolution (get/update/patch): accepts entity_id or an alias/friendly_name substring match. `update`/`patch` refuse with an "ambiguous" error listing candidates if the identifier matches more than one script — use the exact entity_id in that case. `delete`/`execute` accept entity_id or bare id only, no alias/friendly_name matching.

## Read vs. Write

Access control filter syntax (for `HA_MCP_TOOL_FILTER_WHITELIST` / `_BLACKLIST`):

- `"manage_automation"` — blocks/allows entire tool
- `"manage_automation:create"` — specific action
- `"manage_*:delete"` — glob across all manage_ tools
- `"*:write"` — all write operations (see Write Actions column above)

`read_only: true` is shorthand for blacklisting `*:write`. Whitelist and blacklist are mutually exclusive.

## query_entities Mode Guide

| Mode         | Use when                                                         |
| ------------ | ---------------------------------------------------------------- |
| `current`    | Browse entities by domain/area/state — general listing           |
| `history`    | Sensor/entity state over time (requires time range)              |
| `statistics` | Aggregated stats (min/max/mean) for sensor entities              |
| `domains`    | Discover which entity domains exist in this HA instance          |
| `presence`   | Person/device tracker home/away status                           |
| `health`     | Find unavailable, unknown, disabled, orphaned, or stale entities |

<EXTREMELY-IMPORTANT>
Never loop `get_state` for multiple entities. Use `query_entities` with a domain/area filter instead. A single `query_entities` call replaces N×`get_state` calls.
</EXTREMELY-IMPORTANT>

## Anti-Patterns

| Instead of…                                           | Do this                                               |
| ----------------------------------------------------- | ----------------------------------------------------- |
| `call_service` to create/update/delete an automation  | `manage_automation` action=create/update/delete       |
| Multiple `get_state` calls for a set of entities      | `query_entities` mode=current with domain/area filter |
| Patching after a `list` response                      | Call `get` first — `list` does not populate Config    |

## Source for Maintenance

`D:\data\godev\ha-mcp\docs\tools.md`, `internal/handlers/register.go`, `internal/mcp/access_control.go`
