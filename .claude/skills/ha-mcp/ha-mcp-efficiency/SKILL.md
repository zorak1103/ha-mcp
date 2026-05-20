---
name: ha-mcp-efficiency
description: "Use when planning multi-call ha-mcp workflows or when token usage matters. Examples: \"query 50 entities\", \"batch-update labels\", \"reduce response size\", \"avoid unnecessary API calls\"."
---

# ha-mcp Efficiency

## Always: format=natural

`format=natural` is the default. Never override it unless the caller will programmatically parse JSON.

- Produces LLM-optimized prose — easier to reason about, no schema overhead.
- 30-60% fewer tokens than `format=json` for most responses.
- Use `format=json` only when piping output to another tool or doing machine parsing.

## Use query_entities Modes

**Never loop `get_state` N times.** One `query_entities` call replaces all of them.

| Scenario                                           | Mode         | Key params                             |
| -------------------------------------------------- | ------------ | -------------------------------------- |
| Browse entities by domain, area, or state          | `current`    | `domain`, `area_id`, `state_filter`    |
| Find unavailable / unknown / stale entities        | `health`     | `categories`, `include_disabled`       |
| Who's home? Device trackers / persons              | `presence`   | (none extra)                           |
| Sensor readings over a time range                  | `history`    | `entity_ids`, `start_time`, `end_time` |
| Min/max/mean aggregated stats                      | `statistics` | `statistic_ids`, `period`              |
| What entity domains exist in this HA instance?     | `domains`    | (none)                                 |

## Patch Over Update

For partial changes to automations, scripts, scenes, or dashboards:

| Change type                           | Use      | Why                                              |
| ------------------------------------- | -------- | ------------------------------------------------ |
| 1-3 field changes                     | `patch`  | Sends only ops, not full config                  |
| Adding / removing array elements      | `patch`  | Semantic ops handle position automatically       |
| Full config rewrite / major restructure | `update` | Simpler when structure changes significantly   |

## get vs. list

| Operation              | Populates Config? | Token cost | Use for                           |
| ---------------------- | ----------------- | ---------- | --------------------------------- |
| `list`                 | No                | Low        | Discovery, count, quick scan      |
| `get`                  | Yes               | Medium     | Before update/patch, full details |
| `get_details` (helper) | Yes               | Medium     | Full helper config                |

**Rule:** Never patch after a `list`. Always `get` first.

## Smart Wait Is Server-Side

After any mutation (`create`, `update`, `delete`, `patch`, `call_service`):

- The server snapshots state before, mutates, then polls HA until state changes (default: 5s timeout, 100ms interval).
- Result appends `State changes: entity: old → new` or a timeout warning.
- **Do not** call `get_state` manually after a mutation — that duplicates server-side polling and adds latency.

## Label / Alias Array Modes

When updating labels or aliases, use modes to avoid sending the full current list:

| mode      | Behavior                                  | When to use                              |
| --------- | ----------------------------------------- | ---------------------------------------- |
| `add`     | Appends new items, deduplicates (default) | Adding one label without touching others |
| `remove`  | Subtracts listed items                    | Removing a specific label                |
| `replace` | Full replacement                          | Setting the exact final list             |

`label_mode` / `alias_mode` apply only to `update` actions on `manage_entity`, `manage_device`, `manage_area`, `manage_floor`.

## Cache-Aware Reads

If `HA_CACHE_ENABLED=true`, registry reads (`get_registry`, `manage_*:list`) are cached server-side with per-type TTLs (10-60 min). Cache is automatically invalidated after create/update/delete mutations on the same registry type. Do not add client-side caching — the server already prevents thundering-herd via `singleflight`.

## Source for Maintenance

CLAUDE.md sections "Post-Mutation Async Confirmation (Smart Wait)", "Configuration Priority", "Consolidated Tools".
