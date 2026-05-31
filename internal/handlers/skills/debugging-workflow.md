# Debugging Workflow

A step-by-step process for diagnosing why an automation, script, or entity is behaving unexpectedly.

## Step 1: Check the entity state

```json
{ "tool": "get_state", "entity_ids": ["automation.my_auto", "binary_sensor.motion", "light.living_room"] }
```

Use the `entity_ids` array form to check all relevant entities in one call. Look for `unavailable`, `unknown`, or unexpected state values.

## Step 2: Look for timing patterns in the logbook

```json
{
  "tool": "get_logbook",
  "mode": "correlation",
  "entity_id": "automation.my_auto",
  "start_time": "2024-01-15T00:00:00",
  "end_time": "2024-01-15T23:59:59"
}
```

`mode=correlation` groups related events by causal chain — trigger → action → state change. This shows whether the automation fired, what triggered it, and what happened to dependent entities afterward.

Use `mode=entries` for a raw chronological log.

## Step 3: Inspect the last execution trace

```json
{ "tool": "manage_trace", "action": "list", "domain": "automation", "entity_id": "automation.my_auto" }
```

Always pass `domain` — it is required despite the schema marking it optional.

Then fetch the most recent trace:
```json
{ "tool": "manage_trace", "action": "get", "domain": "automation", "trace_id": "<id from list>", "entity_id": "automation.my_auto" }
```

The trace shows exactly which conditions passed/failed and which actions executed.

## Step 4: Validate any templates

```json
{ "tool": "render_template", "template": "{{ states('sensor.temperature') | float > 22 }}" }
```

Test every template condition in the automation against the live HA state before assuming it works. A template returning `unavailable` or an error string will always evaluate to falsy.

## Step 5: Check the system log for errors

```json
{ "tool": "manage_system_log", "action": "list", "level": "error", "integration": "my_integration" }
```

The system log contains ERROR and WARNING messages from integrations. Filter by `level=error` for the most critical issues. Filter by `integration` (substring match) to narrow to a specific component.

## Step 6: Check entity health

```json
{ "tool": "query_entities", "mode": "health" }
```

Returns unavailable, unknown, disabled, orphaned, and stale entities. Stale entities may indicate a device that disconnected.

## Debugging specific failure modes

| Symptom                                                    | Debug tool                                       | What to look for                                      |
| ---------------------------------------------------------- | ------------------------------------------------ | ----------------------------------------------------- |
| Automation not firing                                      | `manage_trace:list` + `manage_trace:get`         | Condition failures; trace may show zero runs          |
| Automation fires but action has no effect                  | `get_logbook mode=correlation`                   | State didn't change after action; check call_service  |
| Sensor reading is wrong / stuck                            | `query_entities mode=health`; `get_state`        | `unavailable` or `unknown` state                      |
| Integration throwing errors                                | `manage_system_log action=list level=error`      | Look for exception traces from the integration        |
| Entity disappeared from UI but still in automations        | `analyze_entity` + `query_entities mode=health`  | Orphaned entity; re-add the integration or delete it  |
| Template condition always passes or always fails           | `render_template`                                | Template syntax error or None value; add `has_value`  |

## Quick reference: read-only diagnostic tools

| Tool                                            | What it reveals                                        |
| ----------------------------------------------- | ------------------------------------------------------ |
| `get_state` with `entity_ids: [...]` array      | Current state + attributes for multiple entities       |
| `get_logbook mode=correlation`                  | Causal chains: trigger → action → state change         |
| `manage_trace action=get`                       | Per-step execution trace with condition pass/fail      |
| `render_template`                               | Whether a template evaluates correctly against live HA |
| `manage_system_log action=list level=error`     | Integration error messages with stack traces           |
| `query_entities mode=health`                    | Unavailable, stale, orphaned entities                  |
| `analyze_entity`                                | Which automations/scripts depend on a given entity     |
| `get_entity_dependencies`                       | Which entities an automation depends on                |
