# Home Assistant Client Interface
> Detail reference moved out of AGENTS.md. Read before touching `internal/homeassistant/` (client, WebSocket/REST routing, retry, cache) or ID handling for automations/scripts/scenes.

The `homeassistant.Client` interface abstracts all HA operations. Implementation uses a hybrid approach:
- **HybridClient** (`hybrid_client.go`): Routes to WebSocket or REST based on operation type
- **WebSocket** (`ws_client_impl.go` over `ws_client.go`/`ws_session.go`): Persistent connection with auto-reconnect (1s → 60s backoff); each authenticated connection is an immutable `wsSession` (own pending map and read loop) swapped atomically on reconnect
- **REST** (`rest_client.go`): Used for automation/script/scene CRUD operations
- **Retry** (`retry.go`): Automatic retries with exponential backoff for transient failures; REST retries GET/HEAD on 429/500/502/503/504/network errors (not client timeouts) and other methods only on 429 and dial errors; WebSocket retries a command after it was written only if it is read-only (`ws_replay.go`); pre-send failures retry for every command
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
- **Script client method ID asymmetry**: `CreateScript(ctx, scriptID, cfg)` and `DeleteScript(ctx, scriptID)` take the bare ID (e.g. `morning_routine`); `GetScript(ctx, entityID)` takes the full entity ID (e.g. `script.morning_routine`), while `UpdateScript(ctx, scriptID, cfg)` takes the bare ID like create/delete (the handlers pass `configID` from `normalizeScriptID`). Integration tests: derive both with `scriptID := GenerateTestID("x")` and `scriptEntityID := BuildEntityID("script", scriptID)`.
