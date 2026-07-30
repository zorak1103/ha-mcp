---
name: ha-mcp-patching
description: "Use when editing an existing automation, script, scene, or dashboard with `action: patch`. Examples: \"add a trigger\", \"replace a dashboard card\", \"remove a condition\", \"change automation mode\", \"add delay to trigger\"."
---

# ha-mcp Patching (JSON Patch + Semantic Patch)

Applies to: `manage_automation`, `manage_script`, `manage_scene`, `manage_dashboard`.

## When to Patch vs. Update

- **Patch**: changing 1-3 fields, modifying array elements (triggers/conditions/actions/views), adding/removing items. Atomic — all ops succeed or none apply.
- **Update**: rewriting the whole config (major restructure, >5 field changes, or replacing the full action sequence).

## Pre-flight

<EXTREMELY-IMPORTANT>
Always call `manage_*:get` before patching. `list` does not populate the Config field. Without the full config as reference, your ops have no verified base and may target stale indexes.
</EXTREMELY-IMPORTANT>

## Standard Ops (RFC 6902 — path-based)

Use `path` as a JSON Pointer (forward-slash syntax). Mutually exclusive with `match`.

| Op        | Effect                                   | Example path                  |
| --------- | ---------------------------------------- | ----------------------------- |
| `replace` | Overwrite a field                        | `/mode`, `/alias`             |
| `add`     | Insert or append                         | `/actions/-` (append to end)  |
| `remove`  | Delete element or field                  | `/conditions/0`               |
| `test`    | Assert value (fail if mismatch)          | `/alias`                      |
| `move`    | Move element (path → to)                | `/triggers/0` → `/triggers/2` |
| `copy`    | Copy element (path → to)                | `/actions/0` → `/actions/-`   |

```json
[
  {"op": "replace", "path": "/mode", "value": "queued"},
  {"op": "add", "path": "/actions/-", "value": {"service": "notify.mobile_app", "data": {"message": "Done"}}},
  {"op": "remove", "path": "/conditions/0"}
]
```

## Semantic Ops (property-addressed)

Use when array indexes are unreliable — re-ordered, unknown position, or after a previous remove in the same call. Mutually exclusive with `path`.

| Field         | Purpose                                                                      |
| ------------- | ---------------------------------------------------------------------------- |
| `match`       | Key-value pairs identifying the target element(s) in the section             |
| `section`     | Array to search: `triggers`, `conditions`, `actions`, `sequence`, `views`, … Recurses into nested arrays/objects within it — not just the section's direct elements. |
| `field`       | Field within the matched element; omit for `remove` (deletes whole element)  |
| `match_index` | 0-based index when multiple elements match (default: first match)            |

```json
[
  {
    "op": "add",
    "match": {"entity_id": "binary_sensor.motion", "to": "on"},
    "section": "triggers",
    "field": "for",
    "value": "00:02:00"
  }
]
```

## Nested Action Structures

`section` in Semantic Ops recurses into nested arrays/objects — a `match`
inside a `choose`/`if`/`repeat` action block, or a dashboard card/chip nested several
levels below `views`, is found the same way a top-level element is; you no longer need
a `path` op just to reach it. A standard `path` op is still useful when you need to
target one specific occurrence deterministically (e.g. disambiguating without relying
on `match_index`), and the nesting shown below is easy to get wrong when writing one by hand:

| Block    | Structure                                                        | Example path                       |
| -------- | ----------------------------------------------------------------- | ----------------------------------- |
| `choose` | `conditions`/`sequence` are children **inside each choose option** | `/actions/0/choose/0/sequence/-`   |
| `choose` | `default` is a **sibling of `choose`**, not inside an option       | `/actions/0/default/-`             |
| `if`     | `then`/`else` are **siblings of `if`**, NOT nested inside it       | `/actions/0/then/-`, `/actions/0/else/-` |
| `repeat` | `sequence` is a child of `repeat`                                  | `/actions/0/repeat/sequence/-`     |

<EXTREMELY-IMPORTANT>
`then`/`else` are siblings of `if` at the same level — they are NOT inside the `if` array.
`/actions/0/if/0/then/0` is wrong; the correct path is `/actions/0/then/0`.
</EXTREMELY-IMPORTANT>

If a path op fails with "key not found", the error now reports the prefix it actually
navigated (not your full submitted path) plus a hint when the missing key is one of
`then`/`else`/`sequence`/`default` — use that prefix + `get` output to find the right sibling.

## Worked Snippets

**Replace automation mode:**
```json
[{"op": "replace", "path": "/mode", "value": "queued"}]
```

**Append a new action:**
```json
[{"op": "add", "path": "/actions/-", "value": {"service": "light.turn_on", "target": {"entity_id": "light.living_room"}}}]
```

**Add `for:` delay to a trigger matched by entity_id:**
```json
[{"op": "add", "match": {"entity_id": "binary_sensor.door"}, "section": "triggers", "field": "for", "value": "00:05:00"}]
```

**Remove a condition by match (no `field` = delete whole element):**
```json
[{"op": "remove", "match": {"condition": "state", "entity_id": "input_boolean.guest_mode"}, "section": "conditions"}]
```

**Replace a dashboard view's cards by title:**
```json
[{"op": "replace", "match": {"title": "Weather"}, "section": "views", "field": "cards", "value": []}]
```

**Bulk-replace entity_id across actions:**
```json
[{"op": "replace", "match": {"entity_id": "light.old"}, "section": "actions", "field": "entity_id", "value": "light.new"}]
```

**Replace an entity nested inside a dashboard card/chip (recursive match):**
```json
[{"op": "replace", "match": {"content": "Example", "entity": "device_tracker.example_phone"}, "section": "views", "field": "entity", "value": "device_tracker.example_phone_new"}]
```

## Dry Run

Pass `dry_run: true` to preview a patch without saving. The result is a compact diff —
each affected path with its truncated before/after value — not the entire patched config,
so it stays small even for a large dashboard.

## Atomicity & Ordering

- All operations run as a unit — first failure rolls back all changes.
- Semantic `remove` ops are automatically ordered so nested matches are removed before
  their ancestors, and siblings within the same array are removed highest-index-first —
  safe to submit multiple removes (including nested ones) without index shifting.
- For standard `path`-based removes, sort descending manually when issuing multiple removes in one call.

## Anti-Patterns

| Instead of…                                          | Do this                                                |
| ---------------------------------------------------- | ------------------------------------------------------ |
| Full `update` to change 1-2 fields                   | `patch` with `replace` ops on those fields             |
| Using `path` index after a remove in the same call   | Use semantic `match` to address elements by property   |
| Patching without calling `get` first                 | Always `get` first — `list` does not populate Config   |
| Using both `match` and `path` in the same op         | They are mutually exclusive — pick one per op          |

## Source for Maintenance

`internal/jsonpatch/`, `internal/handlers/patch_semantic.go`, CLAUDE.md section "JSON Patch (RFC 6902) + Semantic Patch".
