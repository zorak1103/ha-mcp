# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ha-mcp is a Model Context Protocol (MCP) server that provides AI assistants with access to Home Assistant. It uses a hybrid architecture: WebSocket for most operations, REST API for automation/script/scene CRUD (create/update/delete). Translates MCP tool calls into Home Assistant API commands.

**Requirements:** Go 1.26+, golangci-lint v2

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
- Implementation: `buildConfigForFlowStep()` maps step_id to required fields, `createHelperViaConfigFlow()` loops until create_entry
- threshold, derivative, integration, group, template use HTTP-based Config Entry Flow (automatically handled by HybridClient)

**Config Entry API Field Mapping**: Some platforms use different API field names than user-facing names:
- `generic_thermostat`: heater_entity_id → heater, target_sensor_entity_id → target_sensor, ac_mode required
- `generic_hygrostat`: humidifier_entity_id → humidifier, target_sensor_entity_id → target_sensor, device_class required
- Mapping done in config builders, original fields filtered by `platformSkipFields` map in `shouldSkipConfigField()`

**Config Entry Source Entity Requirements**: Many Config Entry helpers validate source entity domains:
- utility_meter, statistics, trend, filter require sensor entities (not input_number) - use template sensor wrapper
- generic_thermostat, generic_hygrostat, switch_as_x require switch entities (not input_boolean) - use template switch wrapper
- Integration tests: Create helper functions (createSourceSensor, createSourceSwitch) that build input + template wrapper

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
- `manage_automation`, `manage_script`, `manage_scene`, `manage_dashboard` all support `action=patch` with an `operations` array. `manage_dashboard` also supports `action=find` (search a string/entity_id across all views/nested cards, issue #143)
- **Standard ops** follow RFC 6902: `{"op": "replace", "path": "/mode", "value": "queued"}`
- **Semantic ops** use property-based addressing: `{"op": "add", "match": {"entity_id": "binary_sensor.door"}, "section": "triggers", "field": "for", "value": "00:05:00"}`
  - `match`: key-value pairs to identify element(s) — mutually exclusive with `path`
  - `section`: array to search (`triggers`, `conditions`, `actions`, `sequence`, `views`, …)
  - `field`: field within matched element(s) — required for `add`/`replace`/`test`; omit for `remove` (deletes whole element)
  - `match_index`: optional 0-based index to select specific match when multiple elements match
  - Semantic remove ops are automatically sorted descending per section to prevent index shifting
- Supported ops: `add`, `remove`, `replace`, `test` (standard: also `move`, `copy`)
- Atomic: if any operation fails, the config is not modified
- Standard paths use RFC 6901 JSON Pointer syntax: `/triggers/0/entity_id`, `/actions/-` (append)
- **Nested action blocks (issue #124):** `then`/`else` are siblings of `if`, NOT nested inside it — `/actions/0/then/0`, not `/actions/0/if/0/then/0`. `choose` nests `conditions`/`sequence` per option (`/actions/0/choose/0/sequence/-`); `default` is a sibling of `choose` (`/actions/0/default/-`). `section` in semantic ops only addresses top-level arrays — nested blocks require standard `path` ops. See `.claude/skills/ha-mcp/ha-mcp-patching/SKILL.md` ("Nested Action Structures").
- Implemented in `internal/jsonpatch/` (RFC 6902 engine) + `internal/handlers/patch_semantic.go` (semantic layer). A `key not found` error reports the prefix actually navigated (not the full submitted path) plus a structural hint for `then`/`else`/`sequence`/`default` misses (`internal/jsonpatch/pointer.go`: `navigatedPrefix`, `actionBlockKeyHint`, `keyNotFoundError`)

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
- **Reads are WebSocket, writes are REST** (`GetAutomation`/`GetScript` via `automation/config`/`script/config` WS commands; `Update*` via REST `/api/config/...`) — a REST config write does not refresh HA's running config until a domain reload runs. `manage_automation`/`manage_script` `update` and `patch` call `reloadDomain(ctx, client, domain)` (`internal/handlers/waiter.go`) after every successful write for this reason — without it, an immediate `get` after `update`/`patch` returns stale pre-write config (issue #126). Reload failure (rare) appends a warning to the success message rather than failing the call, since the config write itself already succeeded.
- **`manage_script` delete registry fallback**: The storage-config `DELETE /api/config/script/config/{id}` endpoint only knows storage-managed scripts. YAML-defined scripts and orphan `_2`-suffixed duplicates (HA appends `_2` on a `unique_id` collision) have a storage key that differs from the entity's `object_id`, so the delete 404s/400s with "Resource not found" even though the entity is readable via `get`/`list` (issue #123). `handleDelete` (`scripts.go`) detects this via `isNotFoundError()` and falls back to `deleteScriptViaRegistry()` — the same entity-registry `RemoveEntityRegistryEntry()` path `manage_entity delete` uses — rather than surfacing the raw HA error.

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
- **Client interface method additions**: When adding methods to `Client` interface, must update 12 files total: (1) client.go interface, (2-3) ws_client_impl.go + rest_client.go implementations, (4-5) hybrid_client.go WSOperations/RESTOperations interfaces + delegation, (6) cached_client.go delegation, (7-12) 6 test mock files (testing_helpers_test.go, server_test.go, cached_client_test.go, factory_test.go, client_pool_test.go, hybrid_client_test.go)
- **wsClientImpl REST-only method stubs**: When adding REST-only methods to Client interface, `wsClientImpl` needs stub returning informative error
- **ContentBlock custom MarshalJSON**: `ContentBlock` has a `MarshalJSON` method in `internal/mcp/types.go` that emits only type-specific fields. Do NOT remove it or add `omitempty` back to `Text` — the MCP spec requires `"text"` key always present on text blocks even for empty strings; `omitempty` silently drops it and causes `invalid_union` Zod errors in MCP clients (e.g., `render_template` returning empty string). Uses anonymous struct literals per case to avoid infinite MarshalJSON recursion.
- **Template helper type field**: `type` field in template config determines sensor vs binary_sensor subtype for Config Entry Flow menu selection, but must be filtered by `shouldSkipConfigField` before API submission
- **HACS download API field**: `hacs/repository/download` command requires `repository` field (not `repository_id`)
- **Automation/Scene entity ID derivation**: Home Assistant derives entity IDs from alias/name field (slugified), NOT from config ID. Integration tests: Use matching alias/name and config ID for predictable entity IDs. Handlers: Use `generateAutomationID(alias)` to create matching config IDs
- **List operations don't populate Config**: `ListAutomations()`, `ListScenes()`, `ListScripts()` return entities with State/EntityID but NOT Config field - use `Get*()` for full config
- **Script entities never expose `sequence` as a state attribute**: `analyze_entity`'s and `find_references`'s script-reference scanners used to read `script.Attributes["sequence"]` from `ListScripts()`'s state list - which works against unit-test mocks (they set `sequence` directly on `Attributes`) but silently returns zero results against real Home Assistant, since real script entity attributes are only `current`, `friendly_name`, `last_triggered`, `mode`. Found via a live-HA integration test (#141) that unit tests could not catch. Fixed by fetching the full config via `GetScript(ctx, script.EntityID)` per script (`internal/handlers/analysis.go`'s `findScriptReferences`, `internal/handlers/find_references.go`'s `scanScriptsForReferences`), mirroring how automation-reference scanning already used `GetAutomation`.
- **Trace list API requirement**: `trace/list` WebSocket command requires `domain` parameter (automation or script) - not optional despite handler schema making it optional. Uses `SendHACSCommand` for generic WS dispatch. `wait=true` opt-in parameter polls `trace/list` until traces appear (HA records asynchronously — may lag a fresh `automation.trigger` call)
- **Todo uid→item API field rename**: User param `uid` maps to `data["item"]` in HA `todo.update_item`/`todo.remove_item` service calls — not `data["uid"]`
- **Empty array parameter handling**: Distinguish missing array parameter from empty array in validation errors. Create operations reject empty arrays (validation error), update operations allow empty arrays to clear fields. Automation triggers: empty array `[]` auto-fills with manual-only placeholder trigger
- **Format parameter constants**: Always use existing `formatNatural` (entities_manage.go) and `formatJSON` (areas.go) constants in new tools — avoid hardcoded strings (goconst linter)
- **MCP Registry API**: Use `registry.RegisterTool(tool, handler)` NOT `registry.AddTool(tool)` (method doesn't exist)
- **InputSchema type**: Use `mcp.JSONSchema` with `Properties: map[string]mcp.JSONSchema`, NOT `map[string]any`
- **AutomationConfig.UnmarshalJSON field enumeration:** The custom UnmarshalJSON (types.go:165) explicitly handles each field. Any new field added to `AutomationConfig` MUST also get an unmarshal branch — otherwise the value is silently dropped when reading from HA's API. `ScriptConfig` uses default unmarshalling and only needs the struct field.
- **YAML-defined scripts/automations are not writable via the config API (#122):** `POST /api/config/{script,automation}/config/<id>` only edits storage/UI-managed entities. Writing a YAML-defined entity's id still returns 200, but HA creates a *new* storage entity with the same object_id, disambiguated as `<entity>_2` — leaving the original YAML entity untouched while the tool reports false success (same storage-vs-YAML root cause as the delete fallback above). `manage_script`/`manage_automation` `update`/`patch` now guard against this: before writing, `isYAMLDefinedEntity()` (`yaml_defined.go`) checks the entity-registry entry's `unique_id` (empty or missing entry ⇒ YAML-defined ⇒ refuse with an actionable error) via the same `GetEntityRegistry` access the delete fallback uses. A registry lookup failure is *not* treated as YAML-defined — the write proceeds (graceful degradation) rather than blocking a legitimate edit.
- **`buildConfigEntryUpdateConfig()` leaked the tool's top-level `entity_id` into config-entry update payloads:** `internal/handlers/helpers_consolidated.go`'s config-entry `update` builder forwarded `args["entity_id"]` - the required identifier of *which* helper to update - into the config sent to Home Assistant's Options Flow, as if it were a per-platform config field. For helpers without an `entity_id` config field (template, group, ...) this made every update fail with `"extra keys not allowed @ data['entity_id']"`; for helpers where `entity_id` *is* a legitimate field (threshold, derivative, integral - the monitored source entity), it silently overwrote the source with the helper's own id. Unreachable until the #135 routing fix let config-entry update calls reach Options Flow submission for the first time. Fixed by removing the forwarding - the top-level identifier is never also a legitimate per-platform config value.
- **`addExtendedConfigEntryFields()` defaulted `device_class` to `"humidifier"` for every config-entry helper's update, not just `generic_hygrostat`:** the one-size-fits-all update builder (`internal/handlers/helpers_config_builders_extended.go`) has no per-platform dispatch, so this default - meant only for `generic_hygrostat` - corrupted or rejected updates for template, threshold, group, and every other config-entry type whenever the caller didn't explicitly supply a `device_class`. Fixed by threading `platform` (already computed via `ParseHelperEntityID` in the caller, `internal/handlers/helpers_consolidated.go`) through and gating the default on `platform == "humidifier"`.
- **`extractOptionsFromSchema()` propagated a literal JSON `null` for unset optional Options Flow fields:** an optional field left unset at helper creation (e.g. a threshold's `lower` bound, when only `upper` was set) reports `suggested_value: null` in Home Assistant's Options Flow schema - key present, value nil. `internal/homeassistant/hybrid_client.go`'s schema extractor stored this literal `nil`, and `mergeOptionsFlowConfig()` resubmitted it verbatim on any subsequent update; HA rejects an explicitly-submitted `null` for an optional numeric field as a type error (`"expected float"}`) - the real HA frontend omits the key instead. Fixed by also requiring `suggestedValue != nil` before including the field, matching how a genuinely-absent field was already omitted.

### Pattern Reference

**Options Flow (config entry options read + write)**: `config_entries/get_single` WS API returns empty Options field. `GetConfigEntryOptions()` initiates Options Flow REST API (init → navigate menu if needed → extract `suggested_value` from `data_schema` → abort). For **updates**: init flow → navigate menu → extract current values via `mergeOptionsFlowConfig()` (preserves unchanged fields) → submit. Template helpers show sensor/binary_sensor menu requiring navigation via `findEntityDomainForConfigEntry()`. See `updateHelperViaOptionsFlow()` in `hybrid_client.go`. Icons are extracted before Options Flow submission and set via Entity Registry after success.

**Helper update routing**: `UpdateHelper` in `HybridClient` checks entity registry for `config_entry_id` — if present, routes to Options Flow REST API; if absent, routes to WebSocket. This match is on the **full entity_id** (`entry.EntityID == helperID`), so callers — including `manage_helper` update, `internal/handlers/helpers_consolidated.go` — must pass the full entity_id (e.g. `sensor.my_template`), not the bare object_id; a bare id never matches and silently falls through to `wsClientImpl.UpdateHelper`'s `<platform>/update` WS command, which config-entry domains (sensor, binary_sensor, climate, humidifier, select, group, ...) don't have (`unknown_command`, issue #135). `wsClientImpl.UpdateHelper`/`DeleteHelper` accept either a full or bare id (strip via `extractPlatform` when a known WS-helper prefix is present) but reject non-WS-helper platforms outright via `isWSHelperPlatform()` rather than building a doomed command. WebSocket helper updates require **ALL mandatory fields** (not partial): name always required, entity ID without prefix via `ExtractEntityID()`, field combos like input_number needs min+max.

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

**Pre-existing integration test failures on this HA instance** (not regression indicators): `TestSwitchAsXIntegration` → `extra keys not allowed @ data['name']` (HA API validation tightened); `TestGenericThermostatIntegration`, `TestTemplateHelperIntegration/TestTemplateSensorUpdate` → HA version-specific 400 errors.

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

**Roborev Review Gate**: This repo has a roborev post-commit hook (`.git/hooks/post-commit`) that asynchronously enqueues an AI review after every commit — the hook only enqueues, it does NOT block. After committing changes destined for push or a PR, run `roborev wait` (blocks on HEAD's review; exit 0 = pass, exit 1 = fail/no job found) before pushing. If it exits non-zero, run `/roborev-fix` to address the findings, then re-commit and `roborev wait` again before proceeding. Do not push or open a PR while a roborev review for the pushed commit is still outstanding.

**File Editing Tool Priority**: Always use the dedicated file tools (Write, Edit) to create or modify files. Never use Bash with Python or shell commands to manipulate file content — the Write tool rewrites a complete file cleanly, the Edit tool makes surgical replacements. Python byte-level manipulation is error-prone, harder to read, and the wrong tool for the job.

**CRLF Line Endings (Windows)**: `git config core.autocrlf=true` means working-directory files have CRLF line endings. The Edit tool silently fails to match multi-line strings in such files. When Edit fails unexpectedly on a file with many changes needed, use Write to rewrite the whole file instead of diagnosing the mismatch.

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
