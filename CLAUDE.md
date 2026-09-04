# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ha-mcp is a Model Context Protocol (MCP) server that provides AI assistants with access to Home Assistant. It uses a hybrid architecture: WebSocket for most operations, REST API for automation/script/scene CRUD (create/update/delete). Translates MCP tool calls into Home Assistant API commands.

**Requirements:** Go 1.27+, golangci-lint v2

## Build and Development Commands

> No Makefile — use `task` (Taskfile.yml) exclusively. Install: https://taskfile.dev/#/installation

```bash
# List all available tasks
task --list

# Build and test
task build                  # go build -o ha-mcp[.exe] ./cmd/ha-mcp
task test                   # go test ./... (no race detector, Windows-safe)
task test:race              # go test -race ./... (CGO_ENABLED=1, Linux/CI)
task test:coverage          # race + coverprofile + per-file 80% enforcement
task test:integration       # go test -tags=integration -v ./internal/handlers/integration/...

# Run specific test directly (not wrapped in a task)
go test -v ./internal/handlers -run TestSpecificName

# Integration tests with env file
set -a && source .env.integration && set +a && task test:integration

# Linting
task lint                   # golangci-lint run --timeout=5m ./...
task lint:integration       # golangci-lint run --timeout=5m --build-tags=integration ...
task lint:install           # install/upgrade golangci-lint pinned to CI's version, built with your local Go toolchain
task fmt                    # gofmt -l . (check)
task fmt:fix                # gofmt -w . (auto-fix)

# Git hooks (run once after cloning)
task install-hooks          # git config core.hooksPath .githooks (pre-commit: auto-fix gofmt)

# Security & analysis
task vulncheck              # govulncheck ./...
task deadcode               # deadcode -test ./...  (install: go install golang.org/x/tools/cmd/deadcode@latest)

# Dev server
task run                    # go run ./cmd/ha-mcp (set HA_URL and HA_TOKEN env vars)

# Release
task release:snapshot       # goreleaser release --snapshot --clean --skip=publish

# Docker
docker pull zorak1103/ha-mcp:latest
docker run -p 8080:8080 -e HA_URL=http://homeassistant.local:8123 zorak1103/ha-mcp:latest
```

## Architecture

### Request Flow

```
AI Client (Claude, Cline)
    → HTTP POST / (JSON-RPC)
    → MCP Server (internal/mcp/server.go)
    → Tool Registry lookup (internal/mcp/registry.go)
    → Tool Handler (internal/handlers/*.go)
    → HybridClient (internal/homeassistant/hybrid_client.go)
        → WebSocket (most operations) OR REST API (automation/script/scene CRUD)
    → Home Assistant API
```

### Key Packages

- **cmd/ha-mcp**: CLI entry point using Cobra, handles flags and signals
- **internal/mcp**: MCP protocol server, JSON-RPC handling, tool/resource registry
- **internal/homeassistant**: Hybrid client (WS + REST), WebSocket with auto-reconnect, REST for automation/script/scene CRUD
- **internal/handlers**: MCP tool handlers organized by domain (entities, automations, helpers, analysis, config_entries, media, dashboards, statistics, services, templates, etc.)
- **internal/handlers/formatter**: Output formatting (natural language for LLMs, JSON for backward compatibility)
- **internal/jsonpatch**: RFC 6902 JSON Patch engine - generic patch operations on `any` types with atomicity via deep clone
- **internal/config**: Viper-based config loading (YAML → .env → ENV → CLI flags)
- **internal/logging**: Structured logging with DEBUG/INFO/WARN/ERROR/TRACE levels

### Key Files

**Core infrastructure:**
- `cmd/ha-mcp/main.go` - Entry point; `internal/handlers/register.go` - central tool registration
- `internal/mcp/access_control.go` - read/write classification; `internal/mcp/tool_filter.go` - ToolFilterEngine; `internal/mcp/registry.go` - tool registry
- `internal/homeassistant/client.go` - Client interface; `hybrid_client.go` - primary implementation; `cached_client.go` - caching wrapper

**Domain handlers** follow naming `handlers/{domain}.go` → `manage_{domain}` tool. Exceptions:
- `entities.go` → `get_state`
- `analysis.go` → `analyze_entity`, `get_entity_dependencies`; `analysis_paths.go` → RFC 6901 path walker (`collectEntityPaths`, `collectSectionReferencePaths`, `collectAutomationReferencePaths`, `referencePathContext`) used to surface exact reference locations in `analyze_entity` output; `analysis_verbose.go` → excerpt summaries (verbose mode)
- `config_search.go` → shared cross-config search primitives: `collectMatchPaths` (generalized, predicate-based version of `collectEntityPaths`), `scanDashboardConfig` (recursive dashboard view/card/chip walker), `scanHelperTemplates` (template-helper `state`/`availability` Jinja scanner). Used by `analyze_entity`'s dashboard/helper-template coverage, `find_references.go`, and `manage_dashboard action=find`. `find_references.go` → `find_references` tool (cross-config search across automations/scripts/scenes/dashboards/helper templates, `match_mode`: substring default/exact, `types` filter)
- `entities_consolidated.go` → `query_entities`; `devices_consolidated.go` → `query_devices`
- `targets_consolidated.go` → `analyze_target`; `registry_consolidated.go` → `get_registry`
- `entities_manage.go` → `manage_entity`; `devices_manage.go` → `manage_device`
- `skill.go` → `get_skill` tool + `RegisterAllResources` (wires 7 skill:// resources)
- `skills/catalog.go` → embedded markdown catalog; add slug here + .md file when adding a skill

Extended logic in dedicated `*_coverage.go`, `*_presence.go`, `*_correlation.go`, `*_health.go` files.

### Handler Pattern

Each handler domain follows this pattern:
1. Create handler struct with `New*Handlers()` factory
2. Implement `RegisterTools(registry *mcp.Registry)` method
3. Register in `internal/handlers/register.go` via `RegisterAllTools()`

`RegisterAllResources(registry)` registers 7 MCP resources (skill:// URI scheme). Called from `initMCPServer` in `cmd/ha-mcp/main.go` after `RegisterAllTools`.

Tool handlers have signature:
```go
func(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error)
```

**Create-then-update pattern**: For controlling immutable entity IDs (e.g., Helper creation):
1. Create entity with desired ID as `name` (controls slug)
2. Immediately update with correct display name
3. Pattern used in `createWSHelper()` for WebSocket-based entities

**Cache Invalidation Pattern**: New mutation operations (create/update/delete) must invalidate related caches in `cached_client.go` (e.g., area CRUD → `invalidateAreaRegistryCache()`)

**Config Entry Flow Multi-Step Pattern**: Some Config Entry platforms require multiple form submissions beyond an initial menu step. This is now **schema-driven, not per-platform hardcoded**: `internal/homeassistant/flow_steps.go`'s `buildStepSubmission()` treats each step's own `DataSchema` as the allow-list — a caller-supplied field is routed into whichever step's schema declares it (via `indexStepSchema()`/`placementOf()`), and `createHelperViaConfigFlow()`/`updateHelperViaOptionsFlow()`'s `runOptionsFlowSteps()` loop submit steps until `create_entry`/`abort`/a step cap. A `consumed map[string]bool`, threaded through the whole loop and seeded by `seedConsumedRoutingKeys()` (`group_type`; `icon` for every config-entry platform; and each platform's own `platformSkipFields` entries, e.g. template's `type`/`template_type`, random's `type`, generic_thermostat's `heater_entity_id`/`target_sensor_entity_id`), tracks which caller fields have already been claimed — a field no step ever claims is reported via `unconsumedUserFields()`, checked only **after** every step has had a chance (this is what lets a field belonging to a later step, e.g. generic_thermostat's presets, be routed correctly instead of rejected against the first step's schema alone). Note `target_domain` and `state_characteristic` are NOT seeded routing keys despite once being treated as such — `target_domain` is switch_as_x's real (and only) "user" step field, and `state_characteristic` is statistics' dedicated step's real field; pre-consuming either broke real creates by hiding them from the step that legitimately wants them (see `platformSkipFields`'s doc comment in `hybrid_client.go`).
- **The create/update asymmetry lives in one line**: HA's `_update_and_remove_omitted_optional_keys` pops every `vol.Optional` key of a step's own schema absent from the submission. On **create** nothing is stored yet, so `buildStepSubmission` starts from an empty payload — an omitted optional is a genuine omission. On **update** it starts from every field's round-tripped `suggested_value` (`extractOptionsFromSchema`, nil-skipping) — so HA finds every optional field present and pops nothing. Get this backwards and update silently deletes every unset field of whichever step is being submitted.
- **Sections (`type: "expandable"`, HA nests exactly one level) are always re-emitted on update**, even when the caller supplies nothing for them — the section key is itself `vol.Optional`, so omitting it deletes the whole nested dict (this is how template's `additional_options`/`availability` survives an update that doesn't touch it).
- **A field's `Selector` is authoritative for duration coercion** (`{"selector":{"duration":{...}}}` in HA's serialized schema) via `fieldIsDuration()`/`coerceForField()`; the old name-keyed `isDurationField()` list and `filterDurationWindowSteps` (which step ids treat `window_size` as a duration vs. a sample count) are now **fallbacks only**, used when a step's schema can't be inspected (e.g. a test mock with no `Selector` set).
- **Read-only selector fields** (HA's `EntitySelectorConfig(read_only=True)`, seen on statistics' `entity_id`/`state_characteristic` in its Options Flow schema) are round-tripped but never overridden by a caller value on update — `fieldIsReadOnly()`.
- **Menu matching tries an exact match before a substring match** (`findMatchingMenuOption()`) — entity domain `"sensor"` is a substring of the menu option `"binary_sensor"`, and HA sorts its menus, so a substring-only search could silently pick the wrong option.
- `statistics`: 3-step flow (`user` → `state_characteristic` → `options`); the `state_characteristic` step is `vol.Required` with no HA-side default, so the `"mean"` fallback lives in `buildStatisticsConfig` (`internal/handlers/helpers_config_builders_extended.go`), not the flow engine.
- `trend`: `settings` step's schema simply doesn't declare `name`, so it's naturally excluded — no special case needed.
- `filter`: 2-step flow (`user` → filter-type-named step, e.g. `outlier`/`time_throttle`); the filter subtype IS the step id.
- `generic_thermostat`: 2-step flow (`user`/`init` → `presets`). On create, no user field matches the presets step's schema unless the caller supplied a preset temperature, so an untouched presets step naturally submits `{}` — the previously-hardcoded empty-map special case is just the schema-driven engine's normal behavior when nothing matches.
- threshold, derivative, integration, group, template use HTTP-based Config Entry Flow (automatically handled by HybridClient)

**Config Entry API Field Mapping**: Some platforms use different API field names than user-facing names:
- `generic_thermostat`: heater_entity_id → heater, target_sensor_entity_id → target_sensor, ac_mode required
- `generic_hygrostat`: humidifier_entity_id → humidifier, target_sensor_entity_id → target_sensor, device_class required
- Mapping done in config builders, original fields filtered by `platformSkipFields` map in `shouldSkipConfigField()`

**Config Entry Source Entity Requirements**: Many Config Entry helpers validate source entity domains — verified against each platform's actual `EntitySelector` in Home Assistant core, not just observed rejections:
- `utility_meter.source`, `filter.entity_id` require `sensor.*` - use template sensor wrapper
- `statistics.entity_id` accepts `sensor.*` or `binary_sensor.*` (HA: `domain=[BINARY_SENSOR_DOMAIN, SENSOR_DOMAIN]`)
- `trend.entity_id` accepts `sensor.*` or `counter.*` (HA: `ALLOWED_DOMAINS = [COUNTER_DOMAIN, SENSOR_DOMAIN]`) - a counter helper can be used directly, no wrapper needed
- `switch_as_x.entity_id` requires `switch.*` - use template switch wrapper
- `generic_thermostat`/`generic_hygrostat` each constrain **two** fields, not one: their actuator field (`heater_entity_id`/`humidifier_entity_id`) accepts `switch.*` **or** `fan.*` (HA: `domain=[fan.DOMAIN, switch.DOMAIN]`), AND `target_sensor_entity_id` must be `sensor.*` **with a specific `device_class`** - `temperature` for generic_thermostat, `humidity` for generic_hygrostat (HA's `EntitySelector` additionally sets `device_class=SensorDeviceClass.TEMPERATURE`/`HUMIDITY`, not just `domain=SENSOR_DOMAIN`) - `helperTypeMetadata.sourceEntities` is a `[]sourceEntityConstraint` (not a single field/domain pair) specifically so both constraints on these two types are checked, not just the first, and each constraint carries an optional `deviceClasses` list for the target-sensor case
- Enforced on both `create` (`checkSourceEntityDomain`) and `update` (`checkUpdateSourceEntityDomain`, which resolves the real integration platform via the entity registry since `update`'s `ParseHelperEntityID` only recovers the entity domain) - a mismatch fails with an actionable wrapper recipe (`wrapperRecipeFor`) naming the required domain, rather than surfacing HA's opaque config-flow error. The device_class check (`checkSourceEntityDeviceClass`) only fires once the domain already matches, and degrades to the domain-only result (does not hard-fail) if the source entity's state fetch fails
- Integration tests: Create helper functions (createSourceSensor, createSourceSwitch, createTemplateBinarySensor, createTemplateFan) that build input + template wrapper. Handler-level preflight behavior (the domain/device_class check itself) is only exercised by tests that go through `s.CallTool("manage_helper", ...)` — tests that call `s.Client().CreateHelper()` directly bypass the handler entirely (see "Integration test scope" below) and cannot catch a preflight regression; `helper_source_domain_tool_dispatch_integration_test.go` covers this

### Home Assistant Client Interface

The `homeassistant.Client` interface abstracts all HA operations. Implementation uses a hybrid approach:
- **HybridClient** (`hybrid_client.go`): Routes to WebSocket or REST based on operation type
- **WebSocket** (`ws_client_impl.go`): Persistent connection with auto-reconnect (1s → 60s backoff)
- **REST** (`rest_client.go`): Used for automation/script/scene CRUD operations
- **Retry** (`retry.go`): Automatic retries with exponential backoff for transient failures (5xx, 429, network errors)
- **CachedClient** (`cached_client.go`): Optional caching layer for static data (services, config, registries)
  - Uses `golang.org/x/sync/singleflight` to prevent thundering herd (duplicate concurrent API calls)
  - Thread-safe with configurable TTLs per cache type
  - Automatically invalidates registry caches after helper create/delete operations

**API Routing:**
- WebSocket: Helpers (`manage_helper`, `helper_action`), state queries, service calls, registry access, config entries, HACS (`manage_hacs`)
- REST: Automation/Script/Scene create/update/delete, template rendering, logbook, config validation

**ID Normalization (Automations/Scripts/Scenes):**

Use `normalize*ID(input)` helpers that return `(entityID, configID)`:
- **entityID**: Full entity_id with prefix (e.g., `script.morning_routine`) - used for WebSocket operations
- **configID**: Bare ID without prefix (e.g., `morning_routine`) - used for REST API operations
- Prevents double-prefix bugs: `script.` + `script.xyz` → `script.script.xyz` ❌

UI-created automations have numeric config IDs differing from entity_id suffix. Always fetch via `GetAutomation()` first, then use `current.Config.ID` for REST operations. Scripts: always use `GetScript()` for updates (returns full config), never `GetState()` (returns only state + friendly_name, causes data loss).
- **Script client method ID asymmetry**: `CreateScript(ctx, scriptID, cfg)` and `DeleteScript(ctx, scriptID)` take the bare ID (e.g. `morning_routine`); `GetScript(ctx, entityID)` and `UpdateScript(ctx, entityID, cfg)` take the full entity ID (e.g. `script.morning_routine`). Integration tests: derive both with `scriptID := GenerateTestID("x")` and `scriptEntityID := BuildEntityID("script", scriptID)`.

**Consolidated Tools (action-based):**

Tool actions and parameters are defined in the handler schemas. Non-obvious aspects only:

- `manage_automation` create: optional `automation_id` (bare slug e.g. `warme_buro`) overrides the auto-generated ID — needed when alias contains non-ASCII chars (umlauts) that HA's slugifier strips
- `manage_helper` — 41 types. **WebSocket helpers** (input_*, counter, timer, schedule): `id` controls entity_id via create-then-update; **Config Entry helpers** (threshold, derivative, integral, group, template_*, utility_meter, min_max, statistics, trend, random_*, filter, tod, generic_thermostat, generic_hygrostat): `name` controls entity_id. `switch_as_x` is the one Config Entry helper where `name` does **not** control entity_id — see the gotcha below. The 15 `template_*` subtypes added for issue #206 (`template_alarm_control_panel`, `template_button`, `template_cover`, `template_device_tracker`, `template_event`, `template_fan`, `template_image`, `template_light`, `template_lock`, `template_number`, `template_select`, `template_switch`, `template_update`, `template_vacuum`, `template_weather` — alongside the pre-existing `template_sensor`/`template_binary_sensor`) are all Config Entry helpers sharing the `template` platform, one HA template domain each; see `internal/handlers/helpers_template_types.go` and the gotchas below. Multi-step flows: statistics=3, filter=2, trend settings excludes name
- `manage_hacs` — actions: list, info, get, releases, release_notes, critical, download, uninstall, add_repository, remove_repository, refresh, toggle_beta. All list filters (search, category, installed_only, pending_update) are client-side
- `manage_system_log` — actions: list, clear. Uses `system_log/list` WS command (bounded to ~50 entries, no HA-side filter params — all filtering is client-side post-fetch). `clear` calls the `system_log.clear` service (not a WS command — HA does not expose a `system_log/clear` WS command). No caching (logs are volatile). Handler: `internal/handlers/system_log.go`
- `get_skill` — actions: list, read. Fallback for tool-only clients; same content served as skill://ha-mcp/* resources. Handler: `internal/handlers/skill.go`
- All `manage_*` CRUD tools support Smart Wait (post-mutation polling, see below)

**Label/Alias Array Mode (manage_entity, manage_device, manage_area, manage_floor):**
- `label_mode`/`alias_mode` on `update` action: `'add'` (default, append+dedup), `'remove'` (subtract), `'replace'` (full replacement)
- `add` and `remove` modes fetch the current registry entry to merge; `replace` sets values directly
- Implemented in `internal/handlers/array_mode.go` (`applyArrayMode`, `getStringSlice`, `getArrayMode`, `arrayModeSchema`)
- Modes apply only to `update`; `create` always sets initial values directly

**JSON Patch (RFC 6902) + Semantic Patch:**
- `manage_automation`, `manage_script`, `manage_scene`, `manage_dashboard` all support `action=patch` with an `operations` array. `manage_dashboard` also supports `action=find` (search a string/entity_id across all views/nested cards)
- **Standard ops** follow RFC 6902: `{"op": "replace", "path": "/mode", "value": "queued"}`
- **Semantic ops** use property-based addressing: `{"op": "add", "match": {"entity_id": "binary_sensor.door"}, "section": "triggers", "field": "for", "value": "00:05:00"}`
  - `match`: key-value pairs to identify element(s) — mutually exclusive with `path`
  - `section`: array to search (`triggers`, `conditions`, `actions`, `sequence`, `views`, …). Matching recurses into nested arrays/objects within the section — not just its direct elements — so a dashboard card/chip nested several levels below `views`, or a nested action block, is found the same way a top-level element is
  - `field`: field within matched element(s) — required for `add`/`replace`/`test`; omit for `remove` (deletes whole element)
  - `match_index`: optional 0-based index to select specific match when multiple elements match (ordering is DFS pre-order: within each section element in array order, that element's own match precedes matches nested inside it — a nested match under an earlier element can therefore come before a later top-level element's match, it is NOT "all top-level matches, then all nested ones")
  - Semantic remove ops are sorted so deeper (nested) matches are removed before their ancestors, and siblings within the same array are removed highest-index-first, so applying them sequentially never invalidates a not-yet-processed match
  - `match`+`section` without `match_index` resolves to *every* matching element including nested ones — a `replace`/`remove` is not implicitly scoped to top-level elements only; use `dry_run` to check the resolved set when match criteria could plausibly hit more than one element
- Supported ops: `add`, `remove`, `replace`, `test` (standard: also `move`, `copy`)
- Atomic: if any operation fails, the config is not modified
- Standard paths use RFC 6901 JSON Pointer syntax: `/triggers/0/entity_id`, `/actions/-` (append)
- **`dry_run: true`** previews a patch without saving. The result is a compact diff — only the resolved paths with truncated before/after values — not the entire patched config, so it stays small even for a large dashboard
- **Nested action blocks:** `then`/`else` are siblings of `if`, NOT nested inside it — `/actions/0/then/0`, not `/actions/0/if/0/then/0`. `choose` nests `conditions`/`sequence` per option (`/actions/0/choose/0/sequence/-`); `default` is a sibling of `choose` (`/actions/0/default/-`). See `.claude/skills/ha-mcp/ha-mcp-patching/SKILL.md` ("Nested Action Structures").
- Implemented in `internal/jsonpatch/` (RFC 6902 engine) + `internal/handlers/patch_semantic.go` (semantic layer: `findMatchingPaths` recursion, `removeBeforePaths` remove ordering) + `internal/handlers/patch.go` (`dryRunPatchResult` diff rendering). A `key not found` error reports the prefix actually navigated (not the full submitted path) plus a structural hint for `then`/`else`/`sequence`/`default` misses (`internal/jsonpatch/pointer.go`: `navigatedPrefix`, `actionBlockKeyHint`, `keyNotFoundError`)

**Access Control & Tool Filtering:**

The server supports flexible tool/action filtering for security and capability control:

- **Read-Only Mode**: `read_only: true` or `--read-only` flag blocks all write operations (internally: `blacklist: ["*:write"]`)
- **Whitelist**: If non-empty, ONLY listed tools/actions allowed (implicit deny-all)
- **Blacklist**: If whitelist empty, block specific tools/actions (implicit allow-all)
- **Mutual Exclusion**: Cannot specify both whitelist and blacklist (validation error)

**Filter Syntax**:
- `"get_state"` - entire tool
- `"manage_automation:create"` - specific action
- `"manage_*:delete"` - glob pattern (all manage_* tools' delete actions)
- `"*:write"` - category expansion (all write operations)
- `"manage_entity:delete"` - specific action on manage_entity

**Implementation**: `ToolFilterEngine` in `internal/mcp/tool_filter.go` with static classification in `internal/mcp/access_control.go` (35 tools, read/write per action). `ValidateFilterConfig()` runs at startup before engine creation — rejects unknown tools, typos, and removed sub-actions with a combined error listing all problems at once (server refuses to start). Three-stage enforcement: startup config validation → registry filtering (removes/modifies tools) → runtime check (blocks calls).

- **`manage_helper:create`/`update` carry the same execution reach as `manage_script`/`manage_automation` for the 15 `template_*` subtypes**: their `tplAction` fields (`press`, `turn_on`, `lock`/`unlock`, `install`, `trigger`, ...) accept an arbitrary HA action sequence, the same shape as an automation's `action:` block, submitted to HA as part of the helper's config. `access_control.go` classifies `manage_helper:create`/`update` as a plain write with no awareness of embedded actions - a filter policy that blocks `manage_script:*`/`manage_automation:*` while still allowing `manage_helper` does **not** block arbitrary service-call execution (create a `template_button` with `press={"action":"shell_command.x"}`, then `call_service button.press`). If a deployment needs to block script/automation execution specifically, filter `manage_helper:create`/`manage_helper:update` too - `read_only` mode is unaffected since it already blocks all writes.

**Post-Mutation Async Confirmation (Smart Wait):**
- **Pattern**: snapshot before → mutate → poll until changed or timeout → append state diff/warning to success message. Timeout → warning appended, never an error
- `automation.reload` + `script.reload` ✅ make REST-created entities appear
- `scene.reload` ❌ does NOT work — scenes require full HA restart after REST creation
- `call_service`: snapshots target entities before, polls for state changes after, appends "\nState changes: entity: old → new" or warning
- **Config**: Env vars `HA_WAIT_TIMEOUT_MS` (default: 5000), `HA_WAIT_POLL_INTERVAL_MS` (default: 100). Injected via `mcp.WithWaitConfig(ctx, cfg)` → `mcp.WaitConfigFromContext(ctx)` in handlers
- **Reads are WebSocket, writes are REST** (`GetAutomation`/`GetScript` via `automation/config`/`script/config` WS commands; `Update*` via REST `/api/config/...`) — a REST config write does not refresh HA's running config until a domain reload runs. `manage_script` `update`/`patch` call `reloadDomain(ctx, client, domain)` (`internal/handlers/waiter.go`) after every successful write for this reason — without it, an immediate `get` after `update`/`patch` returns stale pre-write config. Reload failure (rare) appends a warning to the success message rather than failing the call, since the config write itself already succeeded.
- **`manage_automation` targeted reload**: `create`/`update`/`patch` call `reloadDomainTargeted(ctx, client, "automation", configID)` instead of `reloadDomain` — it passes `automation.reload`'s undocumented `id` field (`map[string]any{"id": configID}`) to reload only the edited automation. A full-domain `automation.reload` (no `id`) tears down and rebuilds *every* automation's triggers, resetting any in-flight `for:` trigger countdown as collateral damage even on unrelated automations; scoping to `id` limits that reset to the automation actually being written. If the targeted call errors (e.g. an HA version without `id` support), it falls back to a full reload so the write still becomes visible. `manage_automation` `delete` still issues an untargeted full reload (`CallService(ctx, "automation", "reload", nil)`, inline in `handleDelete`) — reloading a single just-deleted config id is not a reliable way to remove it.
- **`manage_script` delete registry fallback**: The storage-config `DELETE /api/config/script/config/{id}` endpoint only knows storage-managed scripts. YAML-defined scripts and orphan `_2`-suffixed duplicates (HA appends `_2` on a `unique_id` collision) have a storage key that differs from the entity's `object_id`, so the delete 404s/400s with "Resource not found" even though the entity is readable via `get`/`list`. `handleDelete` (`scripts.go`) detects this via `isNotFoundError()` and falls back to `deleteScriptViaRegistry()` — the same entity-registry `RemoveEntityRegistryEntry()` path `manage_entity delete` uses — rather than surfacing the raw HA error.

### Configuration Priority

`CLI flags > ENV vars > .env file > config.yaml > defaults`

Key environment variables:
- **Connection**: `HA_URL`, `HA_TOKEN`, `HA_MCP_PORT`, `HA_MCP_LOG_LEVEL`
- **Access Control**: `HA_MCP_READ_ONLY`, `HA_MCP_TOOL_FILTER_WHITELIST`, `HA_MCP_TOOL_FILTER_BLACKLIST`
- **REST Rate Limiting**: `HA_REST_RATE_LIMIT`, `HA_REST_RATE_BURST`
- **REST Retry**: `HA_REST_MAX_RETRIES` (default: 3), `HA_REST_RETRY_INITIAL_DELAY_MS` (default: 100), `HA_REST_RETRY_MAX_DELAY_MS` (default: 5000)
- **WebSocket Retry**: `HA_WS_MAX_RETRIES` (default: 3), `HA_WS_RETRY_INITIAL_DELAY_MS` (default: 100), `HA_WS_RETRY_MAX_DELAY_MS` (default: 5000)
- **Caching**: `HA_CACHE_ENABLED` (default: false), `HA_CACHE_SERVICES_TTL_MIN` (default: 60), `HA_CACHE_CONFIG_TTL_MIN` (default: 30), `HA_CACHE_ENTITY_REG_TTL_MIN` (default: 10), `HA_CACHE_DEVICE_REG_TTL_MIN` (default: 10), `HA_CACHE_AREA_REG_TTL_MIN` (default: 30)
- **Post-mutation polling**: `HA_WAIT_TIMEOUT_MS` (default: 5000), `HA_WAIT_POLL_INTERVAL_MS` (default: 100)

## Coding Rules

### Linter Rules

- **goconst**: Extract strings repeated 3+ times to package-level constants (e.g., `const usedInAction = "action"`)
  - Switch case literals must use constants (e.g., `case helperActionUpdate:` not `case "update":`); Enum array literals don't need to
  - Naming convention: `<handler>Action<Verb> = "<verb>"` (e.g., `helperActionUpdate`, `areaActionUpdate`) — each handler file defines its own even if the value is shared
- **funlen**: Functions >60 lines fail - extract helper methods or separate schema builders (e.g., `buildEntityManageSchema()`)
- **gocognit**: Cognitive complexity >23 - extract helpers by operation type: `navigate*()`, `find*()`, `shouldInclude*()`. Pure functions (no ctx/client params) reduce complexity most effectively
- **staticcheck**: Error messages must be lowercase (`fmt.Errorf("error creating: %w"` not `Error creating`)
- **gocritic importShadow**: Use parameter names that don't shadow imported packages (e.g., `cfg` instead of `config` when `internal/config` is imported)
- **gocritic appendCombine**: Combine consecutive appends into single multi-value: `parts = append(parts, a, b, c)` not separate `append()` calls
- **revive unused-parameter**: In test mocks, omit parameter names entirely (`func(context.Context, string)`) instead of underscore
- **revive redefines-builtin-id**: Avoid variable names that shadow Go built-in functions (`min`, `max`, `len`, `copy`, `close`) - use suffixed names (`minVal`, `maxVal`)
- **No trivial wrappers**: Never wrap stdlib functions - use stdlib directly

### API & Type Gotchas

**Convention (going forward)**: an entry here is a terse rule plus a one-line reason - not a narrative. The "how we found it" story (adversarial-review chains, reverted attempts, "fixed in issue #N") belongs in commit/PR history, which already preserves it; repeating it here just costs every future session's context budget for no benefit. A closed GitHub issue is for finished *work* and disappears from view once closed - it is the wrong place for a *standing invariant* a future session needs before touching this code again. Only add an entry when the trap is non-obvious and likely to recur; don't log every bug fix.

- **Pagination**: `PaginationMetadata.NextCursor` is `*string` - requires nil check and dereferencing
- **Person attributes**: `device_trackers` is `[]any` requiring type assertion, not `[]string`

- **`buildDeviceIDsInArea`**: must return errors from `GetDeviceRegistry`, not swallow them - a swallowed fetch failure silently narrows `query_entities`'s `area_id` filter instead of failing the query.
- **Client interface method additions**: touches 12 files - `client.go`, `ws_client_impl.go` (if WS-backed) + `rest_client.go`, `hybrid_client.go`'s `WSOperations`/`RESTOperations` + delegation, `cached_client.go` delegation, and 6 test mock files.
- **`wsClientImpl` implements `WSOperations`, not the full `Client` interface**: REST-only ops (`GetScene`, `GetServices`, `RenderTemplate`, `GetConfig`, automation/script/scene CRUD) live only on `HybridClient`'s `RESTOperations` - don't add a method to `WSOperations` unless a real WS command backs it.
- **`ContentBlock` custom `MarshalJSON`**: don't remove it or add `omitempty` to `Text` - the MCP spec requires the `"text"` key present even for an empty string; `omitempty` causes `invalid_union` errors in MCP clients.
- **Template helper `type` field**: `determineTemplateSubtype` (`hybrid_client.go`) must read the same config key (`template_type`) that `buildTemplateSensorConfig`/`buildTemplateBinarySensorConfig` write - nothing enforces the two agree, so grep before renaming either side.
- **Template sensor `device_class` requires a matching `unit_of_measurement`**: HA's `_validate_unit()` rejects a `template_sensor` create/update with `device_class` set but no matching unit.
- **HACS download API field**: `hacs/repository/download` requires `repository` (not `repository_id`)
- **Automation/Scene entity ID derivation**: HA derives entity IDs from alias/name (slugified), not from config ID. Use `generateAutomationID(alias)` to keep them in sync.
- **List operations don't populate Config**: `ListAutomations()`/`ListScenes()`/`ListScripts()` return State/EntityID only - use `Get*()` for full config.
- **Script entities never expose `sequence` as a state attribute**: don't read `script.Attributes["sequence"]` from `ListScripts()` - fetch via `GetScript()` per script instead.
- **Trace list API requirement**: `trace/list` requires `domain` despite the handler schema making it optional. `wait=true` polls until traces appear (HA records them asynchronously).
- **Trace `item_id` is the `unique_id`, not the `entity_id`**: a wrong key returns an empty list, not an error. `resolveTraceItemID()` resolves this via `GetEntityRegistryEntry` - don't bypass it.
- **Todo `uid`→`item` API field rename**: user param `uid` maps to `data["item"]` in `todo.update_item`/`remove_item` - never pass `item` directly.
- **Empty array parameter handling**: create rejects empty required arrays; update allows an empty array to clear a field.
- **Format parameter constants**: use existing `formatNatural`/`formatJSON` constants, not hardcoded strings (goconst).
- **MCP Registry API**: `registry.RegisterTool(tool, handler)`, not `registry.AddTool(tool)`.
- **InputSchema type**: `mcp.JSONSchema` with `Properties: map[string]mcp.JSONSchema`, not `map[string]any`.
- **`AutomationConfig.UnmarshalJSON` field enumeration**: a new `AutomationConfig` field needs its own unmarshal branch (`types.go:165`) or it's silently dropped. `ScriptConfig` uses default unmarshalling and doesn't need this.
- **Writing an id absent from the managed config file silently orphans instead of editing**: `POST /api/config/{script,automation,scene}/config/<id>` appends a new `<id>_2` entry rather than failing when `id` isn't a key in that config file (YAML-defined/`!include`d entries). `update`/`patch` guard with `configWriteGuardError()`/`ConfigFileEntryExists` (scripts/automations) or a `GetScene`-first read (scenes) before writing.
- **`buildConfigEntryUpdateConfig()` must never forward `args["entity_id"]`**: it's the update's own target identifier, not a per-platform config value - forwarding it broke template/group updates outright and silently overwrote threshold/derivative's actual source field.
- **`addExtendedConfigEntryFields()`'s `device_class` default is gated on `platform == "humidifier"`**: it's a one-size-fits-all update builder shared by every config-entry type - an ungated default corrupts every other type's update.
- **`extractOptionsFromSchema()` must skip a `suggested_value: null`**: HA renders an unset optional Options Flow field this way; resubmitting a literal `null` fails as a type error.
- **`manage_helper` update field docs can drift from what update actually reads**: `updatableFieldNames()` = `requiredFields ∪ optionalFields` minus `isUpdateExcludedField` (`entity_id`, plus each type's own exclusions). `TestUpdatableFields_AreActuallyReadByUpdatePath` pins that every remaining name round-trips through the real builder.
- **Every helper field must go through `argReader`** (`helpers_arg_reader.go`), not a hand-rolled setter: it coerces where unambiguous and errors otherwise. `TestHelperSchemaTypes_MatchBuilderArgTypes` and `TestCreatableFields_AreActuallyReadByCreatePath` pin the schema/builder type contract.
- **`filter` helper fields** (`window_size`, `radius`, `time_constant`, `lower_bound`, `upper_bound`) are real parameters read by `buildFilterConfig` - there is no `filters` array (HA allows exactly one filter type per helper). `window_size` is duration-shaped only for `time_simple_moving_average`/`time_throttle` (`toDurationDict` converts it); `time_sma_type` isn't exposed (HA has one legal value).
- **`min_max`'s calculation selector is the tool arg `min_max_type`, not `type`**: `type` is already consumed by `manage_helper`'s own type-selector arg. Gated by the resolved integration platform (`resolveConfigEntryPlatformForMinMaxType`) so it can't leak into e.g. a sensor-domain `group` update. `min_max`'s Options Flow schema also has no `name` field - omit `name` on a `min_max` update.
- **`manage_script`/`manage_scene` `update`/`patch` resolve by alias/friendly-name the same way `get` does** (`resolveScriptForWrite`/`resolveSceneForWrite`), gated on a confirmed not-found (not any error) and refusing rather than guessing on an ambiguous match.
- **`manage_scene update` reads via `GetScene` and merges onto the fetched config** (`applySceneConfigUpdates`), not a rebuild from `GetState` - HA's scene config write is a full replace, not a merge, so the old path wiped `entities`/`icon`/`metadata`.
- **REST identifiers are URL-escaped** (`neturl.PathEscape`, aliased to avoid `gocritic importShadow`) - an unescaped id like `../automation/config/1` would let one tool's write path bypass another tool's `ToolFilterEngine` blacklist.
- **`isNotFoundError` keeps a substring fallback even for `APIError`**: HA's config-file `DELETE` returns a **400**, not 404, for YAML-defined/orphan scripts - a strict status-code check would break that fallback.
- **`generic_thermostat`'s trailing `presets` step** (`PREVENT_EXTRA` schema) is handled generically by the schema-driven flow engine (see "Config Entry Flow Multi-Step Pattern" above), not a per-platform special case. Its six preset fields (`away_temp`/`eco_temp`/`home_temp`/`comfort_temp`/`sleep_temp`/`activity_temp`) derive from one shared list (`genericThermostatPresets`) that both builders and schema read - don't hand-duplicate. `addGenericThermostatPresetFields` gates on entity domain `"climate"` (pinned by `TestThermostatEntityDomain_IsUniqueAcrossHelperTypes`, which assumes single ownership of that domain). HA doesn't clamp presets to `min_temp`/`max_temp`, and a preset can't be cleared once set.
- **`manage_helper update`'s success message reports only fields that actually reached the payload** (`updateSuccessMessage`/`splitAppliedFields`, snapshotted before `client.UpdateHelper` runs), not every arg the caller passed - a field dropped by a platform gate must show as ignored. `splitAppliedFields` resolves the arg name via `resolveUpdateConfigKey` (`resolveTemplateFieldsForDomain` first, then the static `updateConfigKeyAliases` map) before checking `appliedKeys` - comparing the raw arg name directly misreports every renamed field as ignored. A static alias map alone can't express a template subtype's rename (`open`→`open_cover` only for `template_cover`, bare for `template_lock`) since the same arg renames differently per domain.
- **A Config Entry Flow mutation that HA partially rejects is a success, not an error**: `createHelperViaConfigFlow`/`updateHelperViaOptionsFlow` return `*homeassistant.PartialApplyError` (not a plain error) when some caller fields were never claimed by any flow step - the entity/config entry already exists by then. `manage_helper`'s handlers (`handleCreate`/`handleUpdate`) unwrap it via `errors.As` and render `IsError: false` with an appended `WARNING:` line; treating it as `IsError: true` reads as "nothing happened" and risks a caller retrying create, which duplicates the helper.
- **`manage_helper`'s 15 `template_*` subtypes are table-driven**: `helpers_template_types.go`'s `templateSubtypes` map (one entry per HA template domain, each a list of `templateField`s) drives `buildTemplateHelperConfig(typeName)`; `templateHelperTypes()`/`templateHelperProperties()`/`templateSubtypeNames()` derive the schema, builder registration, and type-name list from the same source so they can't drift apart.
- **`readTemplateField` renames the config key uniformly after every `argReader` call, not per-`kind`** - a per-kind switch would forget the rename for non-string kinds.
- **`resolveTemplateFieldsForDomain` disambiguates `state`/`open`/`stop`, which different subtypes map to different HA keys**: create dispatches per-subtype so it's never ambiguous there; the shared update builder (`addExtendedConfigEntryFields`) matches the entity's actual domain and skips the field entirely (rather than guessing) when no subtype's domain matches, e.g. a `template_sensor` update.
- **`ParseHelperEntityID`'s `HelperPlatforms` list must include every entity domain a helper type's entities use** - a missing domain breaks `update`/`delete` with "invalid entity_id format" while `create` still works fine (currently covers the 15 template domains plus `siren`/`valve`).
- **`checkHelperOnlyDomain`/`handleDelete`'s domain check** resolves the entity's real registry platform and rejects unless it's in `widenedHelperOnlyDomains` (derived from `helperTypes`' `validEntityDomains`) for that domain - a registry fetch failure or not-found entity hard-fails rather than degrading unchecked. Scoped only to the 16 newly-widened domains (`newlyWidenedHelperDomains` = `validEntityDomains` minus `preExistingHelperOnlyDomains`), not the older `sensor`/`binary_sensor`/`climate`/`humidifier`/`select` ambiguity that predates this check. Both `update` and `delete` share the same call.
- **HA template domain config keys don't follow a consistent `<name>_action` convention** - verify each new `haKey` against HA's `template/{domain}.py` source rather than assume a pattern (cover's `open_cover` vs. alarm's bare `arm_away` vs. fan/light's bare `turn_on`, etc.).
- **Two tool-arg naming collisions resolved by renaming the tool arg, not the HA key**: `select`'s `options` → `options_template` (vs. `input_select`'s fixed array); `template_lock`'s `code_format` → `lock_code_format` (vs. `alarm_control_panel`'s incompatible enum shape).
- **`sortedHelperTypeNames()`/`buildHelperConfigBuildersRegistry()`/`buildHelperTypesRegistry()`/`buildPerTypeUpdateExcludedFields()` wrap what were plain package-level literals** in a same-named `build*`/`sorted*` func (`var x = buildX()`) so `helpers_template_types.go`'s data merges in via Go's var-dependency ordering, regardless of which file declares what - a bare `func init()` fails `gochecknoinits`, a `var _ = ...()` side-effect trick fails `unused`.
- **`argReader.actionValue()` bounds an HA `ActionSelector` value by depth/node-count (`maxActionDepth`=16/`maxActionNodes`=2000) AND per-string byte length (`maxScalarStringLen`), not by shape** - depth/node-count alone never catches an oversized leaf (`{"action": "<50MB string>"}` is 2 nodes at depth 2), so every string leaf needs its own length check. `maxActionDepth`=16 because a routine `choose` action already reaches depth 9 (list → option map → conditions/sequence → action map → target map → entity_id list → string).
- **`templateSubtype.inclusivePairs` must be checked against each field's resolved `configKey()`, not its `arg` name** - `readTemplateField` renames fields (`config[f.arg]` → `config[f.configKey()]`) before `checkInclusivePairs` runs, so a check against `arg` strings always misses (`template_cover`'s `open`/`close` renames to `open_cover`/`close_cover` - the vol.Inclusive validation would silently never fire). `subtypeConfigKey()` resolves each pair element first.
- **`resolveTemplateFieldsForDomain`** is memoized per domain (`templateFieldsForDomainCache`, mutex-guarded) since it was rebuilding three maps over ~90 fields on every call, including once per arg inside `splitAppliedFields`'s loop - the returned slice is shared across callers, treat it as read-only. That slice must stay **sorted by arg name**: two different arg names can resolve to the same HA config key for one domain (`template_lock`'s `lock_code_format`→`code_format` vs. `template_alarm_control_panel`'s unrelated, unambiguous `code_format`), and `addTemplateConfigEntryUpdateFields` takes the last write in slice order - unsorted order made the winner vary from one call to the next, not just one process to the next, since Go's map iteration order is randomized per range statement (`templateHelperProperties()`'s "first description wins" dedup has the same bug class). A single-owner fallback field must also be **dropped**, not merely sorted after, when its config key collides with one of the domain's own matched fields - `template_alarm_control_panel`'s bare `code_format` is single-owner but collides with `template_lock`'s renamed `code_format` on a `lock`-domain update, so `matchedConfigKeys` is checked before keeping a fallback.
- **Template subtype `device_class` update support is per-type, not a flat "4 create-only, 11 never" split** - verify each type's OPTIONS_FLOW schema, don't assume CONFIG_FLOW support implies it. HA guards `device_class` behind `if flow_type == "config":` for `button`/`cover`/`event`/`update` (create-only), but declares it unconditionally for `number` (`template_number` is the one type supporting update too). `buildConfigEntryUpdateConfig`'s generic read is gated by `deviceClassSupportedOnTemplateUpdate()`, and `perTypeUpdateExcludedFields` excludes `device_class` precisely when that gate is false - both derive from the same two tables, but a new template subtype still needs its real OPTIONS_FLOW schema checked rather than the pattern copied.
- **`switch_as_x`'s entity_id cannot be predicted from `name`, unlike every other Config Entry helper** - its flow schema has no `name` field; HA derives the wrapper's name from the wrapped switch. `CreateHelperEntity` resolves the real id via the entity registry, keyed by the config entry's `entry_id` and filtered to the expected entity domain (`WaitForConfigEntryEntity`'s `preferDomain`) - a config entry can register more than one entity (a tariffed `utility_meter` registers a `select.*` plus a `sensor.*` per tariff), so an unfiltered match could resolve to the wrong domain. `entityIDPredictable()` gates the old name-based guess to non-`switch_as_x` platforms, since that guess is caller-controlled and may name an unrelated entity - it's never used as a write target; an unresolved id returns `entityUnresolvedError` (success-with-WARNING, helper still exists) instead of a guess.
- **`RESTClient`'s `http.Client` must never leave `Transport` nil** - a nil `Transport` falls back to the shared `http.DefaultTransport`, and `httptest.Server.Close()` unconditionally calls `http.DefaultTransport.CloseIdleConnections()` - not scoped to its own connections. Under `t.Parallel()` REST-client tests (separate `httptest.NewServer` each), one subtest's teardown could silently kill another's still-in-flight connection (flaky `HTTP/1.x transport connection broken`, issue #227). `NewRESTClientWithConfig` clones `http.DefaultTransport` per client for an isolated connection pool.

### Pattern Reference

**Options Flow (config entry options read + write)**: `config_entries/get_single` WS API returns empty Options field. `GetConfigEntryOptions()` initiates Options Flow REST API (init → navigate menu if needed → `readAllOptionsFlowSteps()` walks every remaining form step, merging each step's round-tripped `suggested_value` fields, → abort — never completes the flow, since an options flow only persists at `create_entry`; a step reporting `last_step: true` is read from its own response without submitting it, avoiding an unnecessary commit, and only steps without that signal fall back to submitting to discover a successor). For **updates**: init flow → navigate menu → `runOptionsFlowSteps()` loops `buildStepSubmission()` per step (see "Config Entry Flow Multi-Step Pattern" above) → submit. Template's **create** flow shows a menu requiring subtype navigation via `findEntityDomainForConfigEntry()`; its **options/update** flow has no menu at all (`"init": SchemaFlowFormStep(next_step=choose_options_step)`, HA auto-advances based on the stored `template_type`). See `updateHelperViaOptionsFlow()` in `hybrid_client.go`. Icons are extracted before Options Flow submission and set via Entity Registry after success.

**Helper update routing**: `UpdateHelper` in `HybridClient` checks entity registry for `config_entry_id` — if present, routes to Options Flow REST API; if absent, routes to WebSocket. This match is on the **full entity_id** (`entry.EntityID == helperID`), so callers — including `manage_helper` update, `internal/handlers/helpers_consolidated.go` — must pass the full entity_id (e.g. `sensor.my_template`), not the bare object_id; a bare id never matches and silently falls through to `wsClientImpl.UpdateHelper`'s `<platform>/update` WS command, which config-entry domains (sensor, binary_sensor, climate, humidifier, select, group, ...) don't have (`unknown_command`). `wsClientImpl.UpdateHelper`/`DeleteHelper` accept either a full or bare id (strip via `extractPlatform` when a known WS-helper prefix is present) but reject non-WS-helper platforms outright via `isWSHelperPlatform()` rather than building a doomed command. WebSocket helper updates require **ALL mandatory fields** (not partial) at the WS-command level: name always required, entity ID without prefix via `ExtractEntityID()`, field combos like input_number needs min+max.
- **Partial update merge**: `manage_helper` `update`'s handler (`handleUpdate`/`buildKnownTypeUpdateConfig` in `helpers_consolidated.go`) now calls `mergeCurrentHelperState()` before building the config for genuine WebSocket helper types, fetching the entity's current stored config via `GetHelperConfig(ctx, platform, entityID)` (a generic `"<platform>/list"` WS call - the same `collection.StorageCollectionWebsocket` mechanism that already backs `"<platform>/update"`) and filling in any field the caller omitted — mirroring Config Entry helpers' round-trip-then-override merge in `buildStepSubmission()` (`internal/homeassistant/flow_steps.go`). `GetHelperConfig` replaced the schedule-only `GetScheduleConfig` after storage config turned out to be the right merge source for *every* WS helper type, not just schedule: it returns the raw config verbatim (unlike `GetState`'s entity attributes, which are a lossy, sometimes-conditional projection - e.g. `input_boolean`/`input_select`/`input_text`/`input_datetime`'s `"initial"` was previously excluded from update because `GetState` never exposed it, but storage config does). The gate is `!homeassistant.RequiresConfigEntryFlow(meta.platform)`, not `helperTypes` key presence: `group` is a `helperTypes` key but is a Config Entry Flow platform, so it deliberately skips this merge (as do `threshold`/`derivative`, whose platform names also equal their `helperTypes` map key) and keeps the older unmerged path from before this merge step was added.
  - **Merge-fetch failure hard-fails the update**: `mergeCurrentHelperState()` returns `(merged, currentName, fetchErr error)`; when the current-state fetch itself fails, `fetchErr` is non-nil and `buildKnownTypeUpdateConfig` returns an error instead of proceeding with a partial payload. WS `<platform>/update` commands replace the entire config, so writing a partial payload on a degraded read would reset every field the caller didn't supply to its schema default — for `schedule`, every omitted weekday defaults to `[]` (`vol.Optional(day, default=[])`), silently erasing the user's whole schedule behind a success message. This is deliberately **not** the `configWriteGuardError` "checked=false, proceed anyway" convention (yaml_defined.go:19-20): skipping that check only skips a nice-to-have validation, while skipping this one destroys data.
  - **Source-domain validation on update (`checkUpdateSourceEntityDomain`)**: create's `checkSourceEntityDomain` preflight (validates e.g. `utility_meter`'s `source` is a `sensor.*`, `generic_thermostat`'s `heater_entity_id`/`target_sensor_entity_id` are `switch.*`/`sensor.*`) didn't cover update, letting a caller repoint an existing helper's source at a mismatched domain and get HA's opaque config-flow error instead. `handleUpdate` now resolves the real integration platform via the entity registry's `Platform` field (`ParseHelperEntityID` only recovers the entity *domain*, e.g. `"sensor"` for a statistics helper — not the helper type — so a `helperTypes` map lookup by that value never finds these types on update) and re-runs the same domain check, filtered through `updatableSourceEntities()` to drop any constraint whose field is literally `"entity_id"` — that name IS the tool's own "which helper is being updated" identifier on update, never a caller-suppliable source value, so validating it would be a false positive (e.g. `switch_as_x`'s own entity domain is never `"switch"`) rather than a real check. A registry lookup failure degrades to an unchecked update (mirrors `configWriteGuardError`'s convention here — this one only skips a validation, no data loss risk).

**Entity registry update response**: `UpdateEntityRegistryEntry()` returns the updated `EntityRegistryEntry` (HA wraps response in `entity_entry` key, correctly unwrapped by wsClientImpl); after entity_id rename use `RemoveEntityRegistryEntry()` not `DeleteHelper()`

**Config Entry group cleanup**: Groups created via Config Entry Flow return `unknown_command` for `DeleteHelper` — use `RemoveEntityRegistryEntry()` instead

**Registry tool name resolution**: When adding name-based lookup to registry tools (`manage_*`), use two-phase pattern: `find<Type>ByInput(items, input)` (exact ID → case-insensitive name substring) + `resolve<Type>ID(ctx, client, input)` for update/delete

**Detail map field collisions**: When building detail maps from state attributes, check for field name conflicts with reserved keys (`entity_id`, `state`, `friendly_name`) — rename attribute values to prevent overwriting

## Testing Rules

### Test Coverage Requirements

**Minimum Coverage**: 80% test coverage required for all packages, subject to technical feasibility.

**Integration Test Coverage**: All MCP tools must be covered by integration tests in `internal/handlers/integration/`, subject to technical feasibility. Integration tests verify:
- API integration against real Home Assistant instance
- Response parsing and data mapping
- Error handling with actual API errors
- Write operations (CRUD) with full lifecycle verification
- Read operations validate API calls work and responses parse correctly

**Exceptions** (technical infeasibility):
- Tools requiring external resources not available in test environment (e.g., blueprint `import` action — requires a reachable public https:// URL; private/loopback IPs are blocked by the server-side URL validator)
- Operations that would affect production state irreversibly (e.g., system restarts)
- Features requiring specific hardware or integrations not present in test HA instance

**Coverage Verification**:
```bash
go test -cover ./internal/handlers
go test -tags=integration ./internal/handlers/integration
```

### Unit Test Rules

Test files (`*_test.go`) are excluded from funlen, gocognit, gocyclo, errcheck, gosec, gocritic, govet, goconst, nilnil, and tparallel linters. Shared test utilities are in `internal/handlers/testing_helpers_test.go` (mock clients, result parsing helpers).

- **Backward Compatibility Testing**: When adding new default behaviors (e.g., format parameter), update existing tests to explicitly request old behavior rather than relying on defaults
- **Service mock hardening**: Blind `CallServiceFn`/`CallServiceWithResponseFn` mocks hide domain/service/field-name bugs. Hardening pattern: `UniversalMockClient` (top-level `t`) → `return nil, fmt.Errorf("wrong domain: %s", domain)` to propagate as handler error; local mock inside `t.Run` → `t.Errorf(...)` directly
- **Test mocks for routing logic**: When adding routing logic based on registry lookups (e.g., `UpdateHelper` checking `config_entry_id`), ALL existing tests hitting that code path need registry mocks added
- **Wait mock pattern (entityDeleted flag)**: Specialized mocks (mockScriptClient, mockSceneClient, mockAutomationClient) track `entityDeleted bool`. Set `true` in successful Delete calls → `GetState` returns error immediately → `waitForEntityDisappear` resolves in one poll. For script mocks: only return entity for `strings.HasPrefix(entityID, "script.")` — prevents `snapshotEntities` in `call_service` from capturing unrelated entities causing 5s `waitForStateChanges` timeouts
- **`runHandlerTestCases` fast WaitConfig**: Injects 50ms/5ms WaitConfig automatically via `mcp.WithWaitConfig`. Specialized mocks using `context.Background()` directly need `entityDeleted` tracking manually. Use `mcp.WithWaitConfig(ctx, cfg)` to inject short timeouts in tests calling handler functions directly

### Integration Test Rules

Integration tests in `internal/handlers/integration/` verify both write and read operations against a real Home Assistant instance.

**Running integration tests:**
```bash
export HA_INTEGRATION_TEST_URL=http://homeassistant.local:8123
export HA_INTEGRATION_TEST_TOKEN=<your-token>
go test -tags=integration -v ./internal/handlers/integration/...
set -a && source .env.integration && set +a && go test -tags=integration -v ./internal/handlers/integration/...
```

**Safety:** All test entities use `mcptest_<uuid>_<name>` prefix. Tests are skipped if environment variables are not set.

**Integration test suite helpers**: `GenerateTestID("suffix")` → `mcptest_<uuid>_suffix`; `BuildEntityID("domain", "id")` → `domain.id`; `s.RegisterCleanup(func(){...})` for teardown (suite-lifecycle-aware, preferred over raw `defer`). Wait helpers: `s.WaitForAutomation(configID, timeout)` polls `ListAutomations`; `s.WaitForEntity(entityID, timeout)` polls `GetState`.

**4-pattern for registry CRUD**: Each registry type (labels, floors, tags, areas) needs 4 tests (lifecycle, all fields, partial update, multiple items), 3 cleanup functions (cleanupTestX, deleteXWithRetry, CountTestX), FindXByID suite helper, and TearDownSuite verification block

**Integration test scope**: Integration tests call client methods directly, bypassing handler logic. Handler-level features (config merging for partial updates, ID normalization) are tested via unit tests.

**Tool coverage requirement**: Every MCP tool must have integration test coverage. For tools with write operations (CRUD), tests must verify full lifecycle. Tests may skip gracefully when:
- Required entities don't exist (e.g., no cameras for camera tests)
- Features are unavailable (e.g., read-only calendars, no release notes)
- External dependencies missing (e.g., HACS not installed)

**Test organization**: Group related tools in same file when appropriate (e.g., `updates_blueprints_integration_test.go` combines lightweight read-only tools).

**Writable calendar detection pattern**: Some calendars (holiday calendars) are read-only. Integration tests must iterate calendars to find writable ones by testing GetCalendarEvents first.

**Source entity wrapper pattern**: Config Entry helpers with domain requirements need template wrappers in tests: `createSourceSensor()` (input_number + template sensor), `createSourceSwitch()` (input_boolean + template switch with turn_on/turn_off service actions).

**Zone/Person WS command prefix**: zone/person WS commands use a bare domain prefix (`zone/list`, `person/list`), not `config/` - unlike the genuine `config/*_registry/*` APIs. Pinned by `TestWSClientImpl_ZonePersonCommands`.

**`person/list` response shape**: `person/list` returns `{"storage": [...], "config": [...]}`, not a plain array like `zone/list` - `GetPersons` merges both. Pinned by `TestWSClientImpl_GetPersons_MergesStorageAndConfig`.

## Workflow Preferences

**Documentation Update Checklist**: When adding new tools, update all of the following:
- `README.md` - tool count and summary table (Available Tools section)
- `docs/tools.md` - full tool reference table and any new sections
- `docs/architecture.md` - project structure file listing if adding handler files
- `docs/feature-comparison.md` - tool count, feature comparison table
- `CLAUDE.md` - Key Files section, Consolidated Tools section
- `docs/integration-tests.md` - Test Categories table with new test suite operations
- `.claude/skills/ha-mcp/ha-mcp-tools/SKILL.md` - tool/action enum reference (if new tool or action added)
- `.claude/skills/ha-mcp/ha-mcp-gotchas/SKILL.md` - trap lookup (if new gotcha discovered)

**Markdown Table Formatting**: All Markdown tables MUST be human-readable with properly aligned columns. Use consistent spacing and alignment to ensure tables are easy to read in their raw form, not just when rendered.

**TDD Required**: Tests MUST be written BEFORE writing or modifying code. Run `golangci-lint run --timeout=5m ./...` after implementation.

**Pre-existing Failures Encountered Mid-Task**: If a test, lint run, or build fails for a reason unrelated to the current change (confirm via `git stash`/checkout against `main`, or by checking it's already documented above), do not silently work around it or leave it undocumented. Ask the user whether it should be filed as a new GitHub issue before ending the task.

**Gotcha vs. Issue**: an open GitHub issue is for unresolved *work* and stops being visible once closed - it is not where a settled, non-obvious invariant should live, because a fresh session won't read closed issues before touching the code. A gotcha in "API & Type Gotchas" is for exactly that: a terse, always-loaded "rule + one-line why" a future session needs before it re-discovers the same trap. When you fix a bug worth remembering, add or update one line there (see that section's convention note) instead of leaving the reasoning only in the PR description - and don't write the fix's origin story there, git history already has it.

**File Editing Tool Priority**: Always use the dedicated file tools (Write, Edit) to create or modify files. Never use Bash with Python or shell commands to manipulate file content — the Write tool rewrites a complete file cleanly, the Edit tool makes surgical replacements. Python byte-level manipulation is error-prone, harder to read, and the wrong tool for the job.

**Prefer Subagents for Test Execution and Boilerplate**: Delegate to subagents wherever the task allows it, to keep the main conversation's context free for planning, design, and judgment calls. Two categories are delegated by default, not just when convenient:
- **Test execution** (unit test runs, `go test`, coverage checks, lint runs) always goes through a subagent — **with no exception for integration/e2e tests**: `task test:integration` and any `go test -tags=integration` run against the live HA instance follow the exact same rule as a plain `go test ./...`, not a "the main agent can just run this one" carve-out. A test-running subagent's job is strictly to run the specified commands and report results (pass/fail, exact failure output, coverage numbers) — it must never modify code, fix a failure it finds, or otherwise take independent action. A weaker/cheaper model is sufficient for this role, since it is executing a fixed command list and transcribing output, not making decisions. If a test run reveals something that needs fixing, that decision and the fix itself happen in the main conversation (or a fresh subagent explicitly briefed to implement a fix), never inside the test-running subagent.
- **Boilerplate changes** (mechanical, well-specified edits: wrapping an existing var literal in a function, renaming a symbol project-wide, adding N near-identical entries to a table following an established pattern, updating a doc count across several files) go through a subagent once the exact change is fully specified. The main conversation still designs the change and decides what the mechanical edit should be; the subagent executes it and reports back what it touched.

Planning, architecture decisions, and judgment calls about *what* to change stay with the main agent — subagents execute a task the main agent has already fully specified, they don't decide the task's shape.

### Extending Consolidated Tools (Modes/Actions)

When adding new modes/actions to consolidated tools (`manage_*`, `query_entities`, `get_logbook`):
1. Add constant (e.g., `automationActionCoverage = "coverage"`)
2. Update action/mode switch with new case
3. Update tool schema: Enum array, Description text
4. Update error messages with full list of valid values
5. Update test schema validation (expected enum count)
6. Create dedicated file for complex logic (e.g., `*_coverage.go`, `*_presence.go`, `*_correlation.go`)
7. If adding new tool (not just mode/action): Follow the Documentation Update Checklist above (README.md summary table, docs/tools.md full reference, docs/architecture.md structure, feature-comparison.md, CLAUDE.md, docs/integration-tests.md)

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **ha-mcp** (10603 symbols, 27107 relationships, 300 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

> If any GitNexus tool warns the index is stale, run `npx gitnexus analyze` in terminal first.

## Always Do

- **MUST run impact analysis before editing any symbol.** Before modifying a function, class, or method, run `gitnexus_impact({target: "symbolName", direction: "upstream"})` and report the blast radius (direct callers, affected processes, risk level) to the user.
- **MUST run `gitnexus_detect_changes()` before committing** to verify your changes only affect expected symbols and execution flows.
- **MUST warn the user** if impact analysis returns HIGH or CRITICAL risk before proceeding with edits.
- When exploring unfamiliar code, use `gitnexus_query({query: "concept"})` to find execution flows instead of grepping. It returns process-grouped results ranked by relevance.
- When you need full context on a specific symbol — callers, callees, which execution flows it participates in — use `gitnexus_context({name: "symbolName"})`.

## Never Do

- NEVER edit a function, class, or method without first running `gitnexus_impact` on it.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis.
- NEVER rename symbols with find-and-replace — use `gitnexus_rename` which understands the call graph.
- NEVER commit changes without running `gitnexus_detect_changes()` to check affected scope.

## Resources

| Resource | Use for |
|----------|---------|
| `gitnexus://repo/ha-mcp/context` | Codebase overview, check index freshness |
| `gitnexus://repo/ha-mcp/clusters` | All functional areas |
| `gitnexus://repo/ha-mcp/processes` | All execution flows |
| `gitnexus://repo/ha-mcp/process/{name}` | Step-by-step execution trace |

## CLI

| Task | Read this skill file |
|------|---------------------|
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus/gitnexus-cli/SKILL.md` |

<!-- gitnexus:end -->

<!-- ha-mcp-skills:start -->
# ha-mcp Skills

For procedural guidance on using ha-mcp tools (decision trees, gotcha lookups, patch workflows), this project ships a skill bundle at `.claude/skills/ha-mcp/`. Use these when CLAUDE.md gives you the facts but you need the playbook.

| Task                                     | Read this skill                                           |
| ---------------------------------------- | --------------------------------------------------------- |
| Pick the right tool / action             | `.claude/skills/ha-mcp/ha-mcp-tools/SKILL.md`            |
| Hit unexpected error or behavior         | `.claude/skills/ha-mcp/ha-mcp-gotchas/SKILL.md`          |
| Patch automation/script/scene/dashboard  | `.claude/skills/ha-mcp/ha-mcp-patching/SKILL.md`         |
| Reduce tokens / batch operations         | `.claude/skills/ha-mcp/ha-mcp-efficiency/SKILL.md`       |
| Don't know where to start                | `.claude/skills/ha-mcp/ha-mcp-guide/SKILL.md`            |
<!-- ha-mcp-skills:end -->
