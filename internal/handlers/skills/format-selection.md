# Format Selection: natural vs. json

Most ha-mcp tools accept a `format` parameter. Choosing correctly saves 30–60% of tokens per response.

## Default: Always `format=natural`

`format=natural` is the default. Never override it unless you will programmatically parse the output.

- Produces LLM-optimized prose — easy to reason about, no schema noise.
- 30–60% fewer tokens than `format=json` for most responses.
- Describes state, history, lists, and errors in plain English.

## Use `format=json` only when:

1. **Piping to another tool or API call** — you need an exact field value.
2. **Creating or updating entities** — you must inspect the precise field structure (e.g., `manage_script:get` before a full `update`).
3. **Processing nested or structured data** — e.g., extracting a specific nested key from a large automation config.

## Decision table

| Scenario                                         | format   | Why                                          |
| ------------------------------------------------ | -------- | -------------------------------------------- |
| Status checks, diagnostics, general queries      | natural  | Token-efficient; human-readable reasoning    |
| Finding which entities are in an area            | natural  | Prose list is enough                         |
| Reading automation config before a patch         | natural  | You just need to understand the structure    |
| Reading automation config before a full `update` | json     | You need exact field names and types         |
| `manage_script:get` before constructing a call   | json     | Sequence/fields must be round-tripped intact |
| `query_entities` for display / analysis          | natural  | Dense prose beats JSON noise                 |
| `get_state` for display                          | natural  | State + attributes in plain text             |
| Extracting a specific field for a subsequent API | json     | Parse the field; pass it to the next call    |

## Anti-pattern: JSON for read-only operations

```
# WRONG — unnecessary token cost
manage_automation action=list format=json

# RIGHT
manage_automation action=list
# (format=natural is the default — no need to specify)
```

## Token impact example

A typical `query_entities mode=current domain=light` call:
- `format=natural` → ~250 tokens
- `format=json` → ~600 tokens for the same 10 entities

Reserve JSON for when you need it. Natural is the right default for everything else.
