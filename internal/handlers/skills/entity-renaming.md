# Entity Renaming

Renaming an entity in ha-mcp updates the entity registry, not automation configs.
Follow this workflow to rename safely without breaking automations.

## Safe rename workflow

**Step 1: Check current usage**
```json
{ "tool": "analyze_entity", "entity_id": "light.old_name" }
```
`analyze_entity` shows every automation, script, and scene that references this entity. Note them — you'll need to update them after renaming.

**Step 2: Rename the entity**
```json
{
  "tool": "manage_entity",
  "action": "update",
  "entity_id": "light.old_name",
  "new_entity_id": "light.new_name",
  "name": "New Display Name"
}
```
`new_entity_id` renames the entity; `name` updates the friendly name. Both are optional independently.

**Step 3: Reload or wait for Smart Wait**
The server polls for the state change. If the entity appears under the new ID within 5 seconds, the response includes the confirmation. No manual `get_state` needed.

**Step 4: Update automations and scripts**
For each automation referencing `light.old_name`, use a semantic patch:
```json
{
  "action": "patch",
  "automation_id": "my_automation",
  "operations": [
    {
      "op": "replace",
      "match": {"entity_id": "light.old_name"},
      "section": "triggers",
      "field": "entity_id",
      "value": "light.new_name"
    },
    {
      "op": "replace",
      "match": {"entity_id": "light.old_name"},
      "section": "actions",
      "field": "entity_id",
      "value": "light.new_name"
    }
  ]
}
```

## Slugify traps

HA derives `entity_id` from `alias` (automations) or `name` (Config Entry helpers) by slugifying:
- Non-ASCII characters are stripped: `Büro` → `buro`
- Spaces → underscores: `Living Room` → `living_room`
- Uppercase → lowercase

If you rename a helper with a non-ASCII name, the entity_id may change unexpectedly.
**Fix:** For automations, pass `automation_id` explicitly. For helpers, use ASCII-only names.

## Friendly-name resolution hierarchy

Home Assistant resolves what to display as the entity's friendly name in this order (highest wins):

1. **Registry custom name** — set via `manage_entity(name=...)`. Overrides everything.
2. **Automation alias** — set via `manage_automation(alias=...)`. Visible only when no registry name exists.
3. **Auto-generated slug** — derived from `entity_id` (underscores → spaces, title-cased). Fallback of last resort.

**Key traps:**

- `manage_entity(name="")` **clears the registry override**, making HA fall back to the auto-slug — *not* the automation alias. If the alias was `"Morning Lights"` but the entity_id slug is `morning_lights_v2`, the display name will show `"Morning Lights V2"` after the clear.
- `manage_automation(alias=...)` does **not** change the display name if a registry custom name exists. The registry name silently wins.
- When renaming with `new_entity_id`, always set `name=` **in the same call** to avoid the auto-slug flashing as the display name in the window between the rename and a separate name update.

**Recommended pattern for rename + relabel:**
```json
{
  "tool": "manage_entity",
  "action": "update",
  "entity_id": "light.old_name",
  "new_entity_id": "light.new_name",
  "name": "My New Display Name"
}
```

## Duplicate entity on automation update

UI-created automations have numeric config IDs that differ from their entity slug.
Calling `manage_automation:update` with a mismatched ID creates a duplicate entity instead of updating the original.

**Fix:** Always call `manage_automation:get` first. Use `current.Config.ID` (the numeric ID) for the `automation_id` parameter in subsequent updates.

## Area and label assignment

```json
{
  "tool": "manage_entity",
  "action": "update",
  "entity_id": "light.new_name",
  "area_id": "living_room",
  "labels": ["automated", "energy_monitored"],
  "label_mode": "add"
}
```

`label_mode: "add"` (the default) appends labels without overwriting existing ones.
Use `label_mode: "replace"` to set the exact final list.
Use `label_mode: "remove"` to subtract specific labels.

## Alias modes (automations, areas, devices)

Same pattern as label modes: `alias_mode: "add"` | `"remove"` | `"replace"`.
Only applies to `update` actions — `create` always sets the initial values directly.
