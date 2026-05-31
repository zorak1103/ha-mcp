# Automation Patterns

Practical patterns for creating and editing automations with `manage_automation`.

## Mode selection

| Mode       | Behavior when triggered while running    | Use for                                     |
| ---------- | ---------------------------------------- | ------------------------------------------- |
| `single`   | Ignores new trigger (HA default)         | Most automations; idempotent actions        |
| `restart`  | Cancels current run, starts fresh        | Motion lights; presence-based scenes        |
| `queued`   | Queues new runs up to `max` (default 10) | Sequential notifications; ordered actions   |
| `parallel` | Runs all triggers concurrently           | Independent per-person or per-device flows  |

Pass `mode` and optionally `max` (only meaningful for `queued` and `parallel`) in `create` or `update`.

## Trigger IDs

Give every trigger a unique `id` so you can reference it in conditions and actions:

```yaml
triggers:
  - platform: state
    entity_id: binary_sensor.motion_living_room
    to: "on"
    id: motion_on
  - platform: state
    entity_id: binary_sensor.motion_living_room
    to: "off"
    id: motion_off
```

Use `trigger.id` in conditions: `"{{ trigger.id == 'motion_on' }}"`.

## Motion + timer pattern

The standard pattern for "lights off after N minutes of no motion":

```yaml
alias: Living Room Motion Light
mode: restart
triggers:
  - platform: state
    entity_id: binary_sensor.motion_living_room
    id: motion
conditions: []
actions:
  - if:
      - condition: trigger
        id: motion
        to: "on"
    then:
      - service: light.turn_on
        target:
          entity_id: light.living_room
    else:
      - delay: "00:05:00"
      - service: light.turn_off
        target:
          entity_id: light.living_room
```

Use `mode: restart` so a new motion event restarts the 5-minute delay.

## Conditions vs. templates

| Use condition objects when…                          | Use a template condition when…                        |
| ---------------------------------------------------- | ----------------------------------------------------- |
| Checking a single entity state or time               | Logic combines multiple entities or requires math     |
| The HA UI would offer it as a condition type         | The check can't be expressed as a single condition    |

Template condition syntax in an automation:
```yaml
conditions:
  - condition: template
    value_template: "{{ states('sensor.temperature') | float > 22 }}"
```

## Creating automations with non-ASCII aliases

HA slugifies `alias` to derive `entity_id`. Non-ASCII characters (umlauts, accents) are stripped:
`alias: "Büro Licht"` → `entity_id: automation.buro_licht` (not `büro`).

Fix: pass `automation_id` explicitly:
```json
{
  "action": "create",
  "alias": "Büro Licht",
  "automation_id": "buro_licht"
}
```

## Patching vs. updating

Use `action=patch` for surgical changes (adding/removing triggers, changing mode).
Use `action=update` only when rewriting the entire automation.
Always call `action=get` before patching — `action=list` does not populate the Config field.

See `skill://ha-mcp/dashboard-safety` for patch syntax examples.
