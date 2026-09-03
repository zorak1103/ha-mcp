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

**Config Entry Flow Multi-Step Pattern**: Some Config Entry platforms require multiple form submissions beyond initial menu step:
- `statistics`: 3-step flow (state_characteristic menu → options form → user form) - each step requires different field subset
- `trend`: settings step excludes "name" field (only entity_id + options)
- `filter`: 2-step flow (user form with filter type → filter-specific form with entity_id only)
- `generic_thermostat`: 2-step flow (user/init form → all-Optional "presets" form). On **create**, that step is submitted empty (nothing exists yet to preserve). On **update**, submitting empty would delete every stored preset temperature instead - see the `buildConfigForFlowStep`/`submitOptionsFlowPresetsStep` gotcha below
- Implementation: `buildConfigForFlowStep()` maps step_id to required fields, `createHelperViaConfigFlow()` loops until create_entry
- threshold, derivative, integration, group, template use HTTP-based Config Entry Flow (automatically handled by HybridClient)

**Config Entry API Field Mapping**: Some platforms use different API field names than user-facing names:
- `generic_thermostat`: heater_entity_id → heater, target_sensor_entity_id → target_sensor, ac_mode required
- `generic_hygrostat`: humidifier_entity_id → humidifier, target_sensor_entity_id → target_sensor, device_class required
- Mapping done in config builders, original fields filtered by `platformSkipFields` map in `shouldSkipConfigField()`

**Config Entry Source Entity Requirements**: Many Config Entry helpers validate source entity domains — verified against each platform's actual `EntitySelector` in Home Assistant core, not just observed rejections (an earlier version of this table was narrower than HA itself, silently rejecting valid configs like a `binary_sensor.*` statistics source):
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
- `manage_helper` — 26 types. **WebSocket helpers** (input_*, counter, timer, schedule): `id` controls entity_id via create-then-update; **Config Entry helpers** (threshold, derivative, integral, group, template_*, utility_meter, min_max, statistics, trend, random_*, filter, tod, generic_thermostat, switch_as_x, generic_hygrostat): `name` controls entity_id. Multi-step flows: statistics=3, filter=2, trend settings excludes name
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

- **Pagination**: `PaginationMetadata.NextCursor` is `*string` - requires nil check and dereferencing
- **Person attributes**: `device_trackers` is `[]any` requiring type assertion, not `[]string`
- **`buildDeviceIDsInArea` used to swallow `GetDeviceRegistry` errors**: `entityRegistryFilter.buildDeviceIDsInArea` (`internal/handlers/registry.go`) populates the `deviceIDsInArea` map used by `query_entities`'s `area_id` filter to also match entities reached only via their device's area. A failed `GetDeviceRegistry` call used to be silently ignored, leaving the map empty and making `area_id` degrade to "direct `AreaID` matches only" - a plausible but wrong result reported as success. It now returns `error`, and its sole caller (`handleEntities` in `registry_consolidated.go`) surfaces it via `errorResult`. Don't reintroduce a swallowed error here - a fetch failure must fail the query, not silently narrow it.
- **Client interface method additions**: When adding methods to `Client` interface, must update 12 files total: (1) client.go interface, (2-3) ws_client_impl.go (only if the method is WS-backed) + rest_client.go implementations, (4-5) hybrid_client.go WSOperations (only if WS-backed)/RESTOperations interfaces + delegation, (6) cached_client.go delegation, (7-12) 6 test mock files (testing_helpers_test.go, server_test.go, cached_client_test.go, factory_test.go, client_pool_test.go, hybrid_client_test.go)
- **`wsClientImpl` implements `WSOperations`, not the full `Client` interface**: automation/script/scene create/update/delete and other REST-only operations (e.g. `GetScene`, `GetServices`, `RenderTemplate`, `GetConfig`) live exclusively on `HybridClient`, routed to `c.rest.*` — `wsClientImpl` simply doesn't declare them, so no error-returning stub is needed. Historically `wsClientImpl` asserted the full `Client` interface and carried 19 stub/never-invoked methods purely for compliance (dead code, confirmed via review — HA has no reliable `config/{automation,script,scene}/{create,update,delete}` WS commands, same class of issue as the zone/person prefix bug). A REST-only method only needs `RESTOperations` + `Client`; do not add it to `WSOperations` unless a real WS command backs it.
- **ContentBlock custom MarshalJSON**: `ContentBlock` has a `MarshalJSON` method in `internal/mcp/types.go` that emits only type-specific fields. Do NOT remove it or add `omitempty` back to `Text` — the MCP spec requires `"text"` key always present on text blocks even for empty strings; `omitempty` silently drops it and causes `invalid_union` Zod errors in MCP clients (e.g., `render_template` returning empty string). Uses anonymous struct literals per case to avoid infinite MarshalJSON recursion.
- **Template helper type field**: `type` field in template config determines sensor vs binary_sensor subtype for Config Entry Flow menu selection, but must be filtered by `shouldSkipConfigField` before API submission
- **Template sensor `device_class` requires a matching `unit_of_measurement`**: Home Assistant's template `config_flow.py` `_validate_unit()` rejects a `template_sensor` create/update whenever `device_class` is set but `unit_of_measurement` is missing or not one of that device class's valid units (e.g. `temperature` → `°C`/`°F`/`K`). This applies to real `manage_helper create type=template_sensor` calls, not just test fixtures - discovered via `createTemplateSensor`'s integration-test fixture (issue #207).
- **HACS download API field**: `hacs/repository/download` command requires `repository` field (not `repository_id`)
- **Automation/Scene entity ID derivation**: Home Assistant derives entity IDs from alias/name field (slugified), NOT from config ID. Integration tests: Use matching alias/name and config ID for predictable entity IDs. Handlers: Use `generateAutomationID(alias)` to create matching config IDs
- **List operations don't populate Config**: `ListAutomations()`, `ListScenes()`, `ListScripts()` return entities with State/EntityID but NOT Config field - use `Get*()` for full config
- **Script entities never expose `sequence` as a state attribute**: `analyze_entity`'s and `find_references`'s script-reference scanners used to read `script.Attributes["sequence"]` from `ListScripts()`'s state list - which works against unit-test mocks (they set `sequence` directly on `Attributes`) but silently returns zero results against real Home Assistant, since real script entity attributes are only `current`, `friendly_name`, `last_triggered`, `mode`. Found via a live-HA integration test that unit tests could not catch. Fixed by fetching the full config via `GetScript(ctx, script.EntityID)` per script (`internal/handlers/analysis.go`'s `findScriptReferences`, `internal/handlers/find_references.go`'s `scanScriptsForReferences`), mirroring how automation-reference scanning already used `GetAutomation`.
- **Trace list API requirement**: `trace/list` WebSocket command requires `domain` parameter (automation or script) - not optional despite handler schema making it optional. Uses `SendHACSCommand` for generic WS dispatch. `wait=true` opt-in parameter polls `trace/list` until traces appear (HA records asynchronously — may lag a fresh `automation.trigger` call)
- **Trace `item_id` is the unique_id, not the entity_id — and a wrong one returns an empty list, not an error**: HA keys its trace store as `f"{domain}.{item_id}"` (`components/trace/models.py` `ActionTrace.key`), where `item_id` is the object's `unique_id` (`_attr_unique_id`), not its `entity_id`. Passing the full entity_id (e.g. `script.morning_routine`) produces the nonexistent key `script.script.morning_routine` — HA's `trace/list`/`trace/get` silently return an empty result for an unknown key rather than erroring, so this class of bug is invisible to a `NoError`/`NotNil`-only test. `resolveTraceItemID()` (`internal/handlers/traces.go`) resolves entity_id → item_id via `GetEntityRegistryEntry` (a targeted `config/entity_registry/get` WS call — this resolves both automations and scripts, since an automation's unique_id is its config `id`, which differs from the entity's object_id for UI-created automations, while a script's unique_id matches its object_id unless the entity was renamed). A failed lookup or a missing unique_id degrades to the object_id, logs a `slog.WarnContext` warning, and returns `resolved=false` to the caller — an earlier version of this fix let the degraded path recreate the exact "empty result, no error" ambiguity the fix exists to close (a wrong object_id still returns `[]` from HA, and the generic "may not be available yet, try wait=true" message told the caller to retry something that can never succeed). `handleListTraces`/`handleGetTrace`/`fetchLatestTrace` now thread `resolved` through: an empty result following a failed resolution renders `unresolvedItemIDWarning()` instead of the generic message, and `wait=true` polling is skipped entirely when resolution failed (polling a key already known to be wrong just burns the timeout). Wired into `manage_trace` `list`, `get`, and the `debug` action's latest-trace fetch. Also fixed in the same pass: `fetchLatestTrace` was comparing `timestamp` as a flat string to find the newest trace, but HA's short-dict `timestamp` is `{"start":..,"finish":..}` — the comparison always degraded to "" > "" and silently returned the *oldest* trace under the "Latest Trace" heading; `traceTimestamps()`/`traceDuration()` fix the extraction and derive duration from `finish - start` since HA's short dict has no `duration` key (there is no flat-string timestamp or explicit `duration` field in any real HA response — both were dead fallback branches for fixtures that couldn't occur, since removed). `formatTraceNatural`/`formatDebugTraceSection` also now surface `error` and `script_execution` (HA's `ActionTrace.as_short_dict()` fields for *why* a run failed - e.g. `failed_conditions` - previously silently dropped) and `not_triggered`.
  - **`GetEntityRegistryEntry` added to the `Client` interface to avoid a full-registry fetch per trace lookup**: the original fix called `GetEntityRegistry()` (`config/entity_registry/list`, no filter — every registered entity, one of the heaviest WS reads available) just to find one entry by `EntityID`. `GetEntityRegistryEntry(ctx, entityID)` backs onto HA's `config/entity_registry/get` (`components/config/entity_registry.py` `websocket_get_entity`), which takes `entity_id` and returns one entry's `extended_dict` — a superset of `list`'s per-entry fields (adds `aliases`/`capabilities`/`device_class`/`original_device_class`/`original_icon`), so the existing `EntityRegistryEntry` struct unmarshals it unchanged. Added to `WSOperations` (WS-backed) and implemented in `wsClientImpl`; `CachedClient` passes it through **uncached** — the call is already a targeted single-entity fetch, so a per-entity cache would duplicate `GetEntityRegistry`'s TTL/singleflight machinery for a call that's already cheap.
- **Todo uid→item API field rename**: User param `uid` maps to `data["item"]` in HA `todo.update_item`/`todo.remove_item` service calls — not `data["uid"]`
- **Empty array parameter handling**: Distinguish missing array parameter from empty array in validation errors. Create operations reject empty arrays (validation error), update operations allow empty arrays to clear fields. Automation triggers: empty array `[]` auto-fills with manual-only placeholder trigger
- **Format parameter constants**: Always use existing `formatNatural` (entities_manage.go) and `formatJSON` (areas.go) constants in new tools — avoid hardcoded strings (goconst linter)
- **MCP Registry API**: Use `registry.RegisterTool(tool, handler)` NOT `registry.AddTool(tool)` (method doesn't exist)
- **InputSchema type**: Use `mcp.JSONSchema` with `Properties: map[string]mcp.JSONSchema`, NOT `map[string]any`
- **AutomationConfig.UnmarshalJSON field enumeration:** The custom UnmarshalJSON (types.go:165) explicitly handles each field. Any new field added to `AutomationConfig` MUST also get an unmarshal branch — otherwise the value is silently dropped when reading from HA's API. `ScriptConfig` uses default unmarshalling and only needs the struct field.
- **Writing an id absent from the managed config file silently orphans instead of editing:** `POST /api/config/{script,automation,scene}/config/<id>` is Home Assistant's generic config-file editor (`components/config/view.py`) — it loads `{script,automation,scene}s.yaml` (or whatever `!include`d file HA loaded it from), and if `id` is not already a key in that in-memory data, it *appends* a new entry rather than failing, creating a duplicate `<entity>_2` while the original entity (wherever it's actually defined) is left untouched and the tool reports false success. This is not specifically a "YAML vs storage" distinction — an automation with an explicit `id:` living under `!include_dir_merge_list` has that same problem despite having an id, and `ScriptEntity` sets a non-empty `unique_id` unconditionally so the entity-registry `unique_id` field can never detect it for scripts at all. `manage_script`/`manage_automation` `update`/`patch` guard against this with `configWriteGuardError()` (`yaml_defined.go`), which calls `RESTClient.ConfigFileEntryExists` — a `GET` against the exact same config-file view the write would target — immediately before the write, using the precise id the write is about to submit. A probe failure is *not* treated as "entry missing" — the write proceeds (graceful degradation) rather than blocking a legitimate edit because a check couldn't run. `manage_scene patch` (unchanged) and `manage_scene update` — which was later brought in line with the same read-before-write guard — take a different shape: both already call `GetScene` as their read-before-write step, and a successful `GetScene` already proves the entry exists in `scenes.yaml` — a separate `ConfigFileEntryExists` probe would be redundant. On a `GetScene` 404, `update` discriminates via one `GetState` call: if the entity still exists, the id is YAML-defined elsewhere and `configFileMissingWriteError()` is returned directly (same message text `configWriteGuardError` would produce, just without the extra round trip); if the entity is also gone, it's a genuine "scene not found".
- **`buildConfigEntryUpdateConfig()` leaked the tool's top-level `entity_id` into config-entry update payloads:** `internal/handlers/helpers_consolidated.go`'s config-entry `update` builder forwarded `args["entity_id"]` - the required identifier of *which* helper to update - into the config sent to Home Assistant's Options Flow, as if it were a per-platform config field. For helpers without an `entity_id` config field (template, group, ...) this made every update fail with `"extra keys not allowed @ data['entity_id']"`; for helpers where `entity_id` *is* a legitimate field (threshold, derivative, integral - the monitored source entity), it silently overwrote the source with the helper's own id. Unreachable until the routing fix that let config-entry update calls reach Options Flow submission for the first time. Fixed by removing the forwarding - the top-level identifier is never also a legitimate per-platform config value.
- **`addExtendedConfigEntryFields()` defaulted `device_class` to `"humidifier"` for every config-entry helper's update, not just `generic_hygrostat`:** the one-size-fits-all update builder (`internal/handlers/helpers_config_builders_extended.go`) has no per-platform dispatch, so this default - meant only for `generic_hygrostat` - corrupted or rejected updates for template, threshold, group, and every other config-entry type whenever the caller didn't explicitly supply a `device_class`. Fixed by threading `platform` (already computed via `ParseHelperEntityID` in the caller, `internal/handlers/helpers_consolidated.go`) through and gating the default on `platform == "humidifier"`.
- **`extractOptionsFromSchema()` propagated a literal JSON `null` for unset optional Options Flow fields:** an optional field left unset at helper creation (e.g. a threshold's `lower` bound, when only `upper` was set) reports `suggested_value: null` in Home Assistant's Options Flow schema - key present, value nil. `internal/homeassistant/hybrid_client.go`'s schema extractor stored this literal `nil`, and `mergeOptionsFlowConfig()` resubmitted it verbatim on any subsequent update; HA rejects an explicitly-submitted `null` for an optional numeric field as a type error (`"expected float"}`) - the real HA frontend omits the key instead. Fixed by also requiring `suggestedValue != nil` before including the field, matching how a genuinely-absent field was already omitted.
- **`manage_helper` update field docs can silently drift from what the update builders actually read:** `helperTypeMetadata.updatableFieldNames()` (used both by `mergeCurrentHelperState()`'s field-preservation loop and by `manage_helper`'s generated per-type description) is `requiredFields ∪ optionalFields` minus `updateExcludedFields` - names that appear in a type's create-time fields but are never consumed by the update path. Two found so far: `entity_id` (collides with update's own "which helper are we updating" identifier - never forwarded, see the `buildConfigEntryUpdateConfig()` gotcha above), and `filter` singular (the filter-type selector is excluded from update the same way - `manage_helper update` cannot change a filter helper's algorithm after creation, only its parameters). `TestUpdatableFields_AreActuallyReadByUpdatePath` (`helpers_consolidated_test.go`) round-trips every remaining `updatableFieldNames()` entry through the real update builder for every helper type, so a field that stops being read on update fails a test instead of silently drifting from the generated docs - add newly-discovered gaps to `updateExcludedFields`, don't just special-case the test.
- **`manage_helper` create silently dropped fields whose declared schema `Type` disagreed with the Go type the config builder demanded - the type contract between the two was never checked anywhere:** `input_number`'s `initial` was declared `Type: "string"` while `buildInputNumberConfig` read it via a setter that only accepted `float64` (`addOptionalFloat`); a schema-conformant client sending `"3000"` had the field silently discarded, with no error, and the helper came up at `min` instead. `counter`'s `initial` masked the same bug for years because HA's `counter` default (`0`) happened to match the always-empty result. Three more fields had the same defect: `time_window`, `delay_on`, `delay_off` were read as strings on create but as numbers on `manage_helper update`, so a value that worked at create silently vanished on update. Fixed by replacing the five silently-dropping `addOptional*` setters with `internal/handlers/helpers_arg_reader.go`'s `argReader`, which **coerces** where unambiguous (a numeric string for a number field, `"yes"`/`"on"` for a bool, ...) and **returns an error** naming the field when it can't - every one of the ~170 call sites across both builder files now goes through it, so this failure mode cannot recur field-by-field. Two independent regression tests pin the contract permanently: `TestHelperSchemaTypes_MatchBuilderArgTypes` (`helpers_arg_types_test.go`) asserts the schema's declared `Type` for every property agrees with an independently-authored expected-Go-type table (deliberately not derived from either the schema or the builders - see `realHelperStorageConfig`'s doc comment for why a derived fixture would be a tautology), and `TestCreatableFields_AreActuallyReadByCreatePath` (`helpers_create_fields_test.go`) is the create-path mirror of `TestUpdatableFields_AreActuallyReadByUpdatePath` above - a field declared in `helperTypes` that no create builder reads now fails a test instead of shipping silently. A field whose Go type is genuinely polymorphic (`initial` - bool/number/whole-number/string depending on helper type; `filter`'s `window_size` - see below) declares no schema `Type` at all (`mcp.JSONSchema.Type` is `omitempty`) rather than picking a misleading one.
- **`manage_helper create type=filter` had no way to configure any filter-type-specific field, and Home Assistant's filter Options Flow schemas reject unknown keys outright:** the tool exposed only `entity_id`/`filter`/`icon` plus a `filters` array parameter that could never work - HA's config-entry flow for `filter` takes exactly one filter type per helper via the top-level `filter` field, and rejects a `filters` key alongside it (`extra keys not allowed @ data['filters']`). Creating a `time_simple_moving_average` or `time_throttle` filter (both require `window_size`) had no working path at all. Fixed by removing `filters` entirely and adding `window_size`, `radius`, `time_constant`, `lower_bound`, `upper_bound` as real parameters read by `buildFilterConfig`/`addExtendedConfigEntryFields`. `window_size` is genuinely polymorphic per HA's own schema: a plain sample-count number for `outlier`/`lowpass`/`throttle`, but a **`DurationSelector` dict** (`{"hours":.,"minutes":.,"seconds":.}`) for `time_simple_moving_average`/`time_throttle` - HA rejects a bare number or `"HH:MM:SS"` string for those two types outright (`"expected dict"`). `internal/homeassistant/hybrid_client.go`'s `toDurationDict()` normalises a dict, an `"H:MM:SS"`/`"MM:SS"`/`"SS"` string, or a bare number-of-seconds into HA's required shape; `filterStepFields`/`filterDurationWindowSteps` (keyed on the config-entry flow's `stepID`, which **is** the filter type name for every step after `"user"`) apply it only where HA's schema demands a duration, and reject/forward everything else via an explicit per-step allow-list instead of the previous "forward everything except name/filter" approach that let stray keys like `filters` leak through. `time_sma_type` (the moving-average sub-type selector) is deliberately **not** exposed as a parameter: its wire key is `type`, already contended by `min_max`/`group`/`random`/`template`, and HA's schema has exactly one legal value with that value as the default - exposing it would only reproduce the documented `min_max_type` collision class for zero added capability.
  - **The Options Flow *update* path had no equivalent duration handling, so a field settable at create could not be changed on update:** `updateHelperViaOptionsFlow` never called the create path's `transformConfigForFlow`/`toDurationDict` machinery at all - a bare-seconds or `"HH:MM:SS"` override for `window_size` (or `derivative`'s `time_window`, `template_binary_sensor`'s `delay_on`/`delay_off` - the same latent gap, discovered as a side effect while fixing this) reached HA's `SubmitConfigEntryOptionsFlowStep` unconverted and failed with `"expected dict"`. Fixed generically rather than with a filter-specific special case: for every key in `extractOptionsFromSchema()`'s current values that is itself a dict (Home Assistant renders a `DurationSelector` field's `suggested_value` as `{"hours":..}`, which is how "this field is duration-shaped" is detected, without a hardcoded field-name list), a non-dict user override is run through `toDurationDict()` before merging.
  - **Home Assistant's Options Flow forms use `PREVENT_EXTRA` voluptuous schemas, so a stray key fails the *entire* update, not just that field:** `mergeOptionsFlowConfig()` used to forward every key in the caller's config verbatim - the removed `filters` parameter, or `manage_helper update`'s own `name` argument for a helper type whose Options Flow schema doesn't declare `name` (filter's per-type update schemas don't; neither does `min_max`'s, see the `min_max` gotcha above, whose only prior workaround was "just don't pass it"). `restrictToSchemaFields()` now derives the allowed key set from `result.DataSchema` and drops anything else (logged via `slog.WarnContext`, not silently) before the merge - this applies to every config-entry helper type, not just filter, so passing `name` to a `min_max` update no longer 400s either.
- **`min_max`'s calculation selector is exposed as `min_max_type`, not `type`:** `manage_helper create` reads `args["type"]` to pick which helper type to build (e.g. `"min_max"`) before the same args map ever reaches `buildHelperConfig`/`buildMinMaxConfig`. A per-instance field also named `type` (HA's min_max config flow requires `CONF_TYPE`, one of min/max/mean/median/last/range/sum, in both `CONFIG_SCHEMA` and `OPTIONS_SCHEMA`) would always read back the already-consumed helper-type selector instead of the caller's intended calculation - a real, previously-shipped bug, not a hypothetical one, since `min_max` is the only helper type whose per-instance config field happens to share a name with the tool's own type-selector argument. Fixed by naming the tool argument `min_max_type` instead; `buildMinMaxConfig` and `addExtendedConfigEntryFields` both map it to HA's `type` config key on create and update respectively. Same disambiguation pattern as `heater_entity_id` → `heater` and `humidifier_entity_id` → `humidifier`.
  - **`min_max_type` had to be gated by the resolved integration platform, not just accepted on any update:** `addExtendedConfigEntryFields()` is a one-size-fits-all update builder shared by every config-entry helper type (template, threshold, sensor-domain `group`, `statistics`, ...), all of which reach it the same way `min_max` does - none of them have a `helperTypes` key matching their entity domain (`sensor`, `binary_sensor`, ...), so `handleUpdate` routes them all through `buildConfigEntryUpdateConfig`. An unconditional `config["type"] = min_max_type` write let a caller's `min_max_type` leak into any of them: loudly rejected by most (`extra keys not allowed @ data['type']`), but **silently applied** to a sensor-domain `group` update, since HA's group `CONF_TYPE` enum (`last/first_available/max/mean/median/min/product/range/stdev/sum`) is a strict superset of min_max's seven values - every min_max_type value validates as a group aggregation type too. Fixed by resolving the real platform via the entity registry (`resolveConfigEntryPlatformForMinMaxType()`, only fetched when the caller actually supplied `min_max_type`) and hard-failing - not degrading, unlike `checkUpdateSourceEntityDomain`'s registry-failure convention - when the resolved platform isn't `min_max`: silently dropping the field on a fetch failure would report the update as successful while discarding the one change the caller asked for, the same data-loss class `mergeCurrentHelperState`'s merge-fetch-failure guard exists for.
  - **`min_max`'s Options Flow schema has no `name` field, unlike its create-time `CONFIG_SCHEMA`:** HA's `min_max` `OPTIONS_SCHEMA` covers only `entity_ids`/`type`/`round_digits`; `CONFIG_SCHEMA` extends it with `name` for create only. `manage_helper update` on a min_max helper with a `name` argument always fails with `"extra keys not allowed @ data['name']"`, since `buildConfigEntryUpdateConfig()` forwards `name` unconditionally for every config-entry type. `name` is optional on `manage_helper update` for min_max in the sense that omitting it works fine - just don't pass it.
- **`manage_script` `update`/`patch` read-before-write parity with `get`:** `get`'s `script_id` always accepted entity_id or an alias/friendly_name substring match (`findScriptByID`), but `update`/`patch` only ever accepted the exact entity_id/bare id - a caller following the tool's own schema description would get "script not found" on an alias that `get` resolved fine. `resolveScriptForWrite()` (`scripts.go`) now gives `update`/`patch` the same fallback, gated on `isNotFoundError()` so a transient `GetScript` failure (WS disconnect, timeout) never triggers the fallback and silently retargets a write. Unlike `get`, the write-path resolver (`findScriptForWrite()`) refuses rather than guesses when the identifier matches more than one script - `get`'s first-match semantics are safe for a read but not for selecting the target of a destructive write. `update`/`patch` also refuse when the resolved script's config has no `sequence` (an empty write base that would otherwise silently wipe the script's steps) - unreachable via the real `wsClientImpl.GetScript` (which always populates `Sequence`) but cheap insurance against a future client bug. Success/error messages name the resolved `entity_id` alongside the caller's input whenever the fallback retargeted the write, via `describeScriptTarget()`.
- **`manage_scene update` did a full config rebuild from `GetState`, same bug class as `manage_script update` before its read-before-write fix:** `handleUpdate` (`scenes.go`) called `client.GetState(ctx, entityID)` - entity attributes only - and rebuilt a fresh `SceneConfig` from the caller's args via a since-removed `buildSceneConfigFromArgs`, discarding every field the caller didn't pass. This is more destructive than the script case: Home Assistant's `EditSceneConfigView._write_value` does a full replace of the `scenes.yaml` entry (`data[index] = updated_value`, no merge), so `manage_scene update` with only `name` wrote `entities: {}` and wiped the scene's entire contents, plus `icon` and `metadata`. Fixed by reading via `GetScene` first (mirroring `manage_scene patch`, which already did this) and merging onto the fetched config via `applySceneConfigUpdates()` - only `name`/`icon`/`entities` present in args are overwritten; `entities`, when present, replaces the whole map wholesale (surgical single-entity edits remain `action=patch`'s job), everything else (including `metadata`) survives untouched. Switching the read to `GetScene` required two companion fixes in `homeassistant.SceneConfig`/`SceneState` that had never mattered while `update` used `GetState`: (1) `SceneConfig` had no `Metadata` field, so HA's own `metadata: {<entity_id>: {entity_only: true}}` block (written by its scene editor) would be silently dropped by any read-modify-write, including the pre-existing `patch` path - now a passthrough `map[string]any` field, never synthesized or pruned; (2) `SceneState.UnmarshalJSON` only accepted a JSON object (flat `{"state":"on","brightness":255}`, which it already round-trips correctly - attributes were never actually broken there, despite that looking like the more obvious suspect), and hard-failed on HA's shorthand entity values (`light.kitchen: on` / `: true`) - now also accepts a bare top-level string or bool before falling back to object parsing.
  - **Adversarial review follow-up - `manage_scene patch` had a wrong-entity write hole `update` didn't:** the first `update`/`patch` read-before-write pass gave `update` the same `isNotFoundError()` gate used by `manage_script`'s read-before-write fix but left `patch`'s pre-existing fallback ungated - it retried via `findSceneByID()` (fuzzy `friendly_name` search, first match wins) on *any* `GetScene` error, not just a confirmed 404, so a transient timeout could silently apply patch operations to an unrelated scene. Fixed by giving `update` and `patch` one shared resolver, `resolveSceneForWrite()` (`scenes.go`), mirroring `resolveScriptForWrite`/`findScriptForWrite`/`describeScriptTarget`: falls back to `findSceneForWrite()` (friendly-name search) only on a confirmed not-found, refuses rather than guesses when more than one scene matches, and re-derives `entityID`/`configID` from whichever entity was actually resolved. `findSceneForWrite` needs no per-item `GetState` fetch (unlike the script equivalent) - `ListScenes()`'s `Entity.Attributes["friendly_name"]` is already enough to match on. This also gave `update` the friendly-name resolution the tool schema always claimed it had but never actually implemented. Same pass added three more hardening fixes to `handleUpdate`/`handlePatch`: a no-op guard (before/after `configToMap` snapshot, `reflect.DeepEqual` - mirrors `manage_script update`'s no-op check) that skips the write and the reload it would trigger when nothing actually changed; `applySceneConfigUpdates()` now reuses `parseSceneEntities()` (the same validator `create` uses) instead of re-implementing its loop and silently discarding the validity flag, so a malformed entity state on `update` is rejected with the same message `create` would give instead of being stored as an empty `SceneState`; and the nil-`Config` guard was widened to also refuse an empty `Entities` map (mirrors `manage_script`'s empty-`Sequence` guard) since HA marks `entities` required and `buildSceneData` omits the key entirely when the map is nil.
  - **Adversarial review follow-up - REST identifiers were never URL-escaped:** every `RESTClient` method that interpolates a caller-supplied id into an HA API path (`fmt.Sprintf("%s/api/.../%s", c.baseURL, id)`, ~18 sites across automation/script/scene/config-entry/logbook/calendar/camera_proxy) sent that id unescaped. A `scene_id` of `../automation/config/1` would reach HA as a request path Go's client does not normalize but aiohttp does - letting `manage_scene:update` write an automation config even when `manage_automation:update` is blacklisted via `ToolFilterEngine`, since the filter checks the tool/action, not the resulting HTTP path. Fixed by wrapping every such identifier in `neturl.PathEscape()` (`net/url` aliased to `neturl` in `rest_client.go` - the file's ~25 functions all name their local URL variable `url`, so an unaliased import would trip `gocritic importShadow` throughout). `TestRESTClient_EscapesIdentifiersInURLPath` asserts the exact `r.URL.EscapedPath()` an `httptest` server receives for an id containing `../` and a space.
  - **Adversarial review follow-up - `isNotFoundError` and HA's non-404 "not found":** tempting to make `isNotFoundError` check `*homeassistant.APIError.StatusCode == 404` instead of a lowercase substring match on the error text (a substring match can misfire on an unrelated error whose body happens to mention "not found"). Don't narrow it that far: HA's own config-file `DELETE` returns a **400**, not 404, with body `{"message":"Resource not found"}` for YAML-defined/orphan scripts - see `manage_script` delete's registry fallback below - so `isNotFoundError` must still fall through to the substring check for a non-404 `APIError`, or that fallback stops firing. `isNotFoundError` now short-circuits `true` for a clean 404 and otherwise keeps the substring fallback for everything else, `APIError` included. `TestIsNotFoundError` pins both the 404 fast path and the 400/"Resource not found" case together so the two can't be silently decoupled again.
- **`generic_thermostat` create/update both fail against HA's newer two-step config flow (`user`/`init` → `presets`)** (issue #194): HA core's `generic_thermostat/config_flow.py` added a trailing `presets` step (`PRESETS_SCHEMA`, all-Optional preset-temperature fields we don't expose as tool parameters) to both `CONFIG_FLOW` and `OPTIONS_FLOW`. That schema is `PREVENT_EXTRA`, so resubmitting the full config there - `buildConfigForFlowStep`'s `default:` behavior, since `generic_thermostat` had no case - fails with `"extra keys not allowed @ data['ac_mode']"` etc. This had been mis-attributed in this file as a generic "HA version-specific 400 error" (see the pre-existing-integration-test-failures note above, now corrected) rather than root-caused. Fixed on **create** by adding a `platformGenericThermostat` case (delegating to `buildGenericThermostatStepConfig`, mirroring how `platformFilter` delegates to `buildFilterStepConfig`) that returns an empty map for `stepID == "presets"` - the create loop already resubmits per-step generically, so no other change was needed there. **Update** needed a separate fix: `updateHelperViaOptionsFlow` submitted once and hard-failed on anything but `create_entry`, so every `generic_thermostat` update (previously untested - no update test existed) hit `"unexpected options flow result type: form"`, since the flow always advances `init` → `presets`. Fixed via `submitOptionsFlowPresetsStep()`, gated on HA's own `StepID` (not `config.Platform`: on the update path `HelperConfig.Platform` is the entity **domain** (`"climate"`), not the helper type (`"generic_thermostat"`) - `ParseHelperEntityID` only recovers the domain, see the `min_max_type`/`resolveConfigEntryPlatformForMinMaxType` gotcha above for the same distinction). `TestGenericThermostatLifecycle` (`internal/handlers/integration/generic_thermostat_integration_test.go`) now includes an update step, confirmed passing against live HA.
  - **Adversarial review follow-up - the update-path fix initially submitted an empty map to the presets step, the same shape as create, which silently deleted every stored preset temperature on every single update:** HA's `SchemaCommonFlowHandler._update_and_remove_omitted_optional_keys` (`helpers/schema_config_entry_flow.py`) does not merely merge a form submission into the flow's accumulated options - for every `vol.Optional` key in *that step's own schema* absent from the submission, it pops the key's existing value out of `self._options`. On **create** there is nothing stored yet, so an empty submission is a genuine no-op - correct, and unchanged. On **update**, `self._options` starts as the config entry's real current values, so an empty presets submission deletes `away_temp`/`eco_temp`/`home_temp`/`comfort_temp`/`sleep_temp`/`activity_temp` unconditionally, with the call still reporting success. Fixed by having `submitOptionsFlowPresetsStep()` resubmit `extractOptionsFromSchema(result.DataSchema)` instead of `map[string]any{}` - HA renders the presets step's response with each preset's current value as `suggested_value` (`_show_next_step`'s `suggested_values = self._options`), so round-tripping it preserves every set preset and, via `extractOptionsFromSchema`'s existing nil-check, leaves a genuinely-unset preset unset rather than fabricating a value for it. `TestUpdateHelperViaOptionsFlow_GenericThermostatPresetsStepPreservesExistingValues` pins this by asserting the second submission's payload, not just that the call succeeds - confirmed to fail against the empty-map version by temporarily reverting the fix and re-running it.
  - Same pass also fixed two related issues an adversarial review found in `updateHelperViaOptionsFlow`'s abort/error handling, both introduced by the initial presets-step change: (1) the terminal-error abort called `AbortConfigEntryOptionsFlow(ctx, submitResult.FlowID)` - whatever HA's *last* response happened to contain - instead of the flow id captured at `init`/menu-navigation time, which is guaranteed non-empty and stable across every step; a response omitting `flow_id` would abort with `""` and leak the flow server-side. Reverted to the pre-existing `result.FlowID`. (2) a validation failure on the `init`/presets step (e.g. `generic_thermostat`'s own `min_max_runtime` check) reported only `"unexpected options flow result type: form"`, discarding `submitResult.Errors` - unlike `createHelperViaConfigFlow`, which already surfaces `config entry flow validation errors: %v`. `updateHelperViaOptionsFlow` now does the same before the terminal-type check.

### Pattern Reference

**Options Flow (config entry options read + write)**: `config_entries/get_single` WS API returns empty Options field. `GetConfigEntryOptions()` initiates Options Flow REST API (init → navigate menu if needed → extract `suggested_value` from `data_schema` → abort). For **updates**: init flow → navigate menu → extract current values via `mergeOptionsFlowConfig()` (preserves unchanged fields) → submit. Template helpers show sensor/binary_sensor menu requiring navigation via `findEntityDomainForConfigEntry()`. See `updateHelperViaOptionsFlow()` in `hybrid_client.go`. Icons are extracted before Options Flow submission and set via Entity Registry after success.

**Helper update routing**: `UpdateHelper` in `HybridClient` checks entity registry for `config_entry_id` — if present, routes to Options Flow REST API; if absent, routes to WebSocket. This match is on the **full entity_id** (`entry.EntityID == helperID`), so callers — including `manage_helper` update, `internal/handlers/helpers_consolidated.go` — must pass the full entity_id (e.g. `sensor.my_template`), not the bare object_id; a bare id never matches and silently falls through to `wsClientImpl.UpdateHelper`'s `<platform>/update` WS command, which config-entry domains (sensor, binary_sensor, climate, humidifier, select, group, ...) don't have (`unknown_command`). `wsClientImpl.UpdateHelper`/`DeleteHelper` accept either a full or bare id (strip via `extractPlatform` when a known WS-helper prefix is present) but reject non-WS-helper platforms outright via `isWSHelperPlatform()` rather than building a doomed command. WebSocket helper updates require **ALL mandatory fields** (not partial) at the WS-command level: name always required, entity ID without prefix via `ExtractEntityID()`, field combos like input_number needs min+max.
- **Partial update merge**: `manage_helper` `update`'s handler (`handleUpdate`/`buildKnownTypeUpdateConfig` in `helpers_consolidated.go`) now calls `mergeCurrentHelperState()` before building the config for genuine WebSocket helper types, fetching the entity's current stored config via `GetHelperConfig(ctx, platform, entityID)` (a generic `"<platform>/list"` WS call - the same `collection.StorageCollectionWebsocket` mechanism that already backs `"<platform>/update"`) and filling in any field the caller omitted — mirroring Config Entry helpers' `mergeOptionsFlowConfig`. `GetHelperConfig` replaced the schedule-only `GetScheduleConfig` after storage config turned out to be the right merge source for *every* WS helper type, not just schedule: it returns the raw config verbatim (unlike `GetState`'s entity attributes, which are a lossy, sometimes-conditional projection - e.g. `input_boolean`/`input_select`/`input_text`/`input_datetime`'s `"initial"` was previously excluded from update because `GetState` never exposed it, but storage config does). The gate is `!homeassistant.RequiresConfigEntryFlow(meta.platform)`, not `helperTypes` key presence: `group` is a `helperTypes` key but is a Config Entry Flow platform, so it deliberately skips this merge (as do `threshold`/`derivative`, whose platform names also equal their `helperTypes` map key) and keeps the older unmerged path from before this merge step was added.
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

**Pre-existing integration test failures on this HA instance** (not regression indicators): `TestSwitchAsXIntegration` → `extra keys not allowed @ data['name']` (HA API validation tightened); `TestTemplateHelperIntegration/TestTemplateSensorUpdate` → HA version-specific 400 errors. `TestGenericThermostatIntegration` was previously listed here as a generic "HA version-specific 400 error" - that was a mis-attribution; the real root cause (a missing `presets`-step case in `buildConfigForFlowStep`) is now fixed and documented under "API & Type Gotchas". `TestHelperSourceDomainIntegration/TestGenericThermostatWithFanHeater` and `.../TestGenericThermostatRejectsWrongDeviceClassTargetSensor` were also previously listed here (template *fan* fixture failing with `{"errors":{"state":"required key not provided"}}`) - root-caused to `createTemplateFan()` omitting the `state` field that HA's template `fan` `CONFIG_SCHEMA` requires (`vol.Required(CONF_STATE)`, merged in via `_SCHEMA_STATE` for the `fan` domain but not for `switch`, which is why the identically-shaped `createSourceSwitch` fixture never had this problem) - fixed in `internal/handlers/integration/helper_source_domain_tool_dispatch_integration_test.go` (issue #195). Fixing #195 unmasked two further independent fixture-only gaps in the same test, both now fixed: `TestGenericThermostatWithFanHeater` got one step further and failed in `createTemplateSensor("thermo_fan_sensor", "temperature")` with a 400 - HA's template sensor `_validate_unit()` requires `unit_of_measurement` whenever `device_class` is set, and the fixture never set one; fixed by having `createTemplateSensor` also set `unit_of_measurement` via a `deviceClassUnits` lookup whenever a device class is supplied (issue #207). `TestStatisticsOverBinarySensorSource` failed independently with `missing_max_age_or_sampling_size` - HA's statistics config flow requires `sampling_size` or `max_age`, and the fixture supplied neither, unlike the sibling tests in `statistics_integration_test.go`; fixed by adding `sampling_size: 20` to the fixture's `manage_helper create` call (issue #208).

**Zone/Person WS command prefix**: `TestZoneIntegration`/`TestPersonIntegration` failing with `unknown_command` was previously misdiagnosed here as "Zone/Person WS API unsupported on this HA version" — the real cause was `wsClientImpl.GetZones`/`GetPersons`/`Create*`/`Update*`/`Delete*` sending `config/zone/*` and `config/person/*` WebSocket commands. Home Assistant's collection helper (`helpers/collection.py`) registers these under the bare domain prefix (`zone/list`, `person/list`, ...), not `config/` — unlike the genuine `config/*_registry/*` commands (entity/device/area/label/floor), which are a different, correctly `config/`-prefixed API. `unknown_command` looks identical whether a command is missing from this HA version or was simply never registered under that name, which is why the true cause went undetected — fixed by dropping the `config/` prefix from all 8 zone/person commands. `TestWSClientImpl_ZonePersonCommands` in `ws_client_impl_test.go` unit-tests the exact 8 command strings against a mocked transport (runs in normal CI, no live HA needed) so a regression to `config/`-prefixed values is caught without depending on the integration suites.

**`person/list` response shape**: fixing the command prefix above was necessary but not sufficient — `GetPersons` still failed against live HA with `cannot unmarshal object into Go value of type []PersonRegistryEntry`. `person/list` uses a custom collection handler (`PersonStorageCollectionWebsocket`) that responds with `{"storage": [...], "config": [...]}`, separating storage-managed persons from YAML-configured ones, unlike `zone/list` and the other collection APIs, which return a plain array. `GetPersons` now unmarshals both arrays and merges them. `TestWSClientImpl_GetPersons_MergesStorageAndConfig` guards this. Both `TestZoneIntegration` and `TestPersonIntegration` (plus their tool-dispatch counterparts) are confirmed passing against a live Home Assistant instance.

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

**File Editing Tool Priority**: Always use the dedicated file tools (Write, Edit) to create or modify files. Never use Bash with Python or shell commands to manipulate file content — the Write tool rewrites a complete file cleanly, the Edit tool makes surgical replacements. Python byte-level manipulation is error-prone, harder to read, and the wrong tool for the job.

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
