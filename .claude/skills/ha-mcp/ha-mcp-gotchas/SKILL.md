---
name: ha-mcp-gotchas
description: "Use when a ha-mcp call returns an unexpected result, error, or behaves differently than expected. Examples: \"scene not appearing after create\", \"helper entity_id is wrong\", \"patch failing with missing config\", \"automation ID mismatch\"."
---

# ha-mcp Gotchas

Look up by symptom. Fix is actionable.

## Trap Lookup

| Symptom                                                   | Cause                                                                     | Fix                                                                                                             |
| --------------------------------------------------------- | ------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| Scene doesn't appear after `create`                       | `scene.reload` via REST is a no-op                                        | Warning is expected and informational. Scene requires full HA restart to appear.                                |
| `patch` returns "missing field" / wrong base on automation | `list` does not populate the Config field                                 | Call `manage_automation:get` first; patch using the returned full Config as reference.                          |
| Helper has wrong `entity_id` after create                 | WS helpers use `id` param; Config Entry helpers use `name` param          | See helper ID rules table below.                                                                                |
| `utility_meter`, `statistics`, `trend`, `filter` fail on create | Source entity must be `sensor.*`, not `input_number.*`              | Wrap `input_number` in a `template_sensor` that sources the input value.                                        |
| `generic_thermostat`, `generic_hygrostat`, `switch_as_x` fail | Source entity must be `switch.*`, not `input_boolean.*`              | Wrap `input_boolean` in a `template_binary_sensor` with `turn_on`/`turn_off` service actions.                   |
| `manage_trace:list` returns error about missing field     | `domain` param is required despite schema marking it optional             | Always pass `domain: "automation"` or `domain: "script"`.                                                       |
| `manage_blueprint:import` returns 4xx                     | HA blocks non-public URLs — loopback/link-local/private IPs rejected      | URL must be a public `https://` address. No local URLs.                                                         |
| Todo `update_item` or `remove_item` fails                 | User param `uid` maps internally to API field `item`                      | Pass `uid` in tool args — the mapping to `item` is done by the handler. Never pass `item` directly.             |
| HACS `download` action fails with unknown field           | Wrong field name — uses `repository`, not `repository_id`                 | Pass `repository` (the repo path, e.g. `hacs-integrations/hacs`).                                              |
| Automation `update` creates a duplicate entity            | UI-created automations have numeric config IDs differing from entity slug  | Always call `manage_automation:get` first; use `current.Config.ID` for REST updates.                            |
| Empty array `[]` rejected on `create`                     | Create validates non-empty required arrays                                | Omit the field or pass a semantic placeholder. Empty arrays are only valid on `update` (to clear).              |
| Helper `update` silently reverts to old values            | WebSocket helpers require ALL mandatory fields on every update             | Include all required fields: `name` always required, plus type-specific combos (e.g., `min`+`max` for `input_number`). |
| Label or alias didn't change after `update`               | Default `label_mode`/`alias_mode` is `add`, not `replace`                 | Pass `label_mode: "replace"` or `alias_mode: "replace"` for full overwrite.                                     |
| Entity ID doesn't match alias/name after create           | HA slugifies `alias`/`name` to derive entity_id; strips non-ASCII chars   | Use ASCII alias, or pass `automation_id` to override the slug explicitly.                                       |
| Automation display name shows wrong value (slug or wrong string) | Three-tier hierarchy: registry custom name > automation alias > auto-slug. Setting `name=""` falls back to slug, not alias. `alias` changes are invisible if a registry name override exists. | To show the alias, clear registry name with `manage_entity(name="")` and verify alias is set. To show a specific name, use `manage_entity(name="...")`. |
| Script `update` loses fields (sequence, fields, etc.)     | `get_state` returns only state + friendly_name, not full script config     | Always call `manage_script:get` (uses `GetScript()`), never derive config from a state call.                    |
| Coverage action returns empty even for active automations | Requires at least one automation with triggers using the target entity     | Expected behavior — no coverage data = target not referenced.                                                   |
| Light (or actuator) stays on indefinitely after patching an automation | `patch`/`update` writes via REST, causing HA to reload the automation and reset in-flight `for:` trigger timers. If the sensor is already in the target state, no new state transition fires. | Tool now warns when `for:` triggers are present. Verify actuators manually after the call. Identical (no-op) patches are skipped automatically to avoid a needless reload. |
| `get` still shows pre-change config right after a successful `update`/`patch` | Reads are WebSocket (`automation/config`/`script/config`), writes are REST — a config write only becomes visible after a domain reload. `update`/`patch` now call `automation.reload`/`script.reload` automatically after every real write (#126). | Check the success message for a "reload after save failed" warning — if present, the write persisted but the reload itself failed; retry `manage_automation:get`/`manage_script:get` or call `automation.reload`/`script.reload` manually. |
| `manage_script:delete` fails with "Resource not found" for a script visible in `get`/`list` | Script is YAML-defined or an orphan `_2`-suffixed duplicate — its storage-config key differs from the entity `object_id`, so the storage-config `DELETE` API 404s/400s (#123) | Nothing to do — `delete` now auto-falls back to removing the entity-registry entry (same path `manage_entity:delete` uses). Success message notes "removed via entity registry" when this path is taken. |

## Helper ID Rules

| Helper types                                                                                                                                                               | entity_id controlled by                      |
| -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------- |
| `input_boolean`, `input_number`, `input_text`, `input_select`, `input_datetime`, `input_button`, `counter`, `timer`, `schedule`                                            | `id` parameter (you control the slug)        |
| `threshold`, `derivative`, `integral`, `group`, `template_sensor`, `template_binary_sensor`, `utility_meter`, `min_max`, `statistics`, `trend`, `random_sensor`, `random_binary_sensor`, `filter`, `tod`, `generic_thermostat`, `generic_hygrostat`, `switch_as_x` | `name` parameter (HA slugifies it — ASCII only) |

## Source for Maintenance

`D:\data\godev\ha-mcp\CLAUDE.md` sections "API & Type Gotchas" and "Pattern Reference".
