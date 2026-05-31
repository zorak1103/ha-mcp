# Dashboard Safety

Dashboards are the highest-risk entities to edit via API. A malformed save can corrupt the UI.
Follow this pattern to work safely.

## Backup before any write

Always fetch the current config before modifying it:

```json
{ "action": "get", "dashboard_url_path": "lovelace" }
```

Save the returned config. If anything goes wrong, you can restore it with `action=save_config` or `action=update`.

## Use patch for surgical edits

`action=patch` applies targeted changes atomically — if any operation fails, the dashboard is unchanged:

```json
{
  "action": "patch",
  "dashboard_url_path": "lovelace",
  "operations": [
    {
      "op": "replace",
      "match": {"title": "Weather"},
      "section": "views",
      "field": "cards",
      "value": []
    }
  ]
}
```

**Always call `action=get` before patching.** `action=list` does not populate the Config field.

## Large config truncation risk

HA dashboard configs can exceed 100 KB. The MCP text response has practical limits.

Signs of truncation:
- Response ends mid-JSON or mid-YAML
- `get` returns fewer views than expected
- Patch fails with "path not found" on a view that should exist

Mitigation:
- Work with one view at a time using the `path` or semantic `match` ops
- Use `format=json` when you need exact field values to round-trip back to an update
- For large configs, prefer the HA UI Raw Config Editor over the API

## When to use the HA UI Raw Config Editor instead

Prefer the HA UI editor when:
- The config is > 50 KB
- You need to copy-paste large card configurations
- You're doing a wholesale restructure of multiple views
- The dashboard uses custom cards not representable as simple JSON

Prefer the API when:
- Making targeted changes to known paths (card properties, entity lists)
- Automating repetitive changes across multiple dashboards
- Bulk-replacing an entity ID across all actions in a view

## Patch operations for dashboards

| Goal                                          | Op type  | Example                                                          |
| --------------------------------------------- | -------- | ---------------------------------------------------------------- |
| Replace a view's card list by title           | semantic | `match: {title: "Living Room"}, section: views, field: cards`    |
| Add a card to a view                          | standard | `path: /views/0/cards/-`                                         |
| Remove a specific card by type+entity         | semantic | `match: {type: "entity", entity: "light.x"}, section: views/0/cards` |
| Change a view's title                         | standard | `path: /views/0/title, value: "New Title"`                       |

## Dashboard atomicity

All patch operations on a dashboard apply as a unit — first failure rolls back all. You cannot partially save a corrupted dashboard via patch. Standard (path-based) `remove` ops: sort descending by index when issuing multiple removes.
