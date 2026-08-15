// Package homeassistant provides client factories and management for Home Assistant API.
// coverage-exempt: caching wrapper with 330 lines of delegation pass-throughs tested via integration tests
package homeassistant

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/zorak1103/ha-mcp/internal/config"
	"github.com/zorak1103/ha-mcp/internal/logging"
)

// cacheEntry holds a cached value with its expiration time.
type cacheEntry struct {
	data      any
	expiresAt time.Time
}

// isExpired returns true if the cache entry has expired.
func (e *cacheEntry) isExpired() bool {
	return time.Now().After(e.expiresAt)
}

// CachedClient wraps a Client and provides caching for static data.
// It caches services, config, and registry data to reduce API calls.
// Thread-safe for concurrent access using singleflight to prevent thundering herd.
type CachedClient struct {
	client Client
	mu     sync.RWMutex

	servicesCache       *cacheEntry
	configCache         *cacheEntry
	entityRegistryCache *cacheEntry
	deviceRegistryCache *cacheEntry
	areaRegistryCache   *cacheEntry
	labelRegistryCache  *cacheEntry
	floorRegistryCache  *cacheEntry

	// singleflight groups prevent duplicate API calls during concurrent access
	servicesGroup       singleflight.Group
	configGroup         singleflight.Group
	entityRegistryGroup singleflight.Group
	deviceRegistryGroup singleflight.Group
	areaRegistryGroup   singleflight.Group
	labelRegistryGroup  singleflight.Group
	floorRegistryGroup  singleflight.Group

	config config.CacheConfig
	logger *logging.Logger
}

// NewCachedClient wraps a Client with caching capabilities.
// If caching is disabled in config, the underlying client is returned directly.
func NewCachedClient(client Client, cfg config.CacheConfig, logger *logging.Logger) Client {
	if !cfg.Enabled {
		return client
	}

	if logger == nil {
		logger = logging.New(logging.LevelInfo)
	}

	logger.Info("Caching enabled",
		"services_ttl_min", cfg.ServicesTTLMin,
		"config_ttl_min", cfg.ConfigTTLMin,
		"entity_reg_ttl_min", cfg.EntityRegTTLMin,
		"device_reg_ttl_min", cfg.DeviceRegTTLMin,
		"area_reg_ttl_min", cfg.AreaRegTTLMin)

	return &CachedClient{
		client: client,
		config: cfg,
		logger: logger,
	}
}

// servicesTTL returns the TTL for services cache.
func (c *CachedClient) servicesTTL() time.Duration {
	if c.config.ServicesTTLMin <= 0 {
		return 60 * time.Minute
	}
	return time.Duration(c.config.ServicesTTLMin) * time.Minute
}

// configTTL returns the TTL for config cache.
func (c *CachedClient) configTTL() time.Duration {
	if c.config.ConfigTTLMin <= 0 {
		return 30 * time.Minute
	}
	return time.Duration(c.config.ConfigTTLMin) * time.Minute
}

// entityRegTTL returns the TTL for entity registry cache.
func (c *CachedClient) entityRegTTL() time.Duration {
	if c.config.EntityRegTTLMin <= 0 {
		return 10 * time.Minute
	}
	return time.Duration(c.config.EntityRegTTLMin) * time.Minute
}

// deviceRegTTL returns the TTL for device registry cache.
func (c *CachedClient) deviceRegTTL() time.Duration {
	if c.config.DeviceRegTTLMin <= 0 {
		return 10 * time.Minute
	}
	return time.Duration(c.config.DeviceRegTTLMin) * time.Minute
}

// areaRegTTL returns the TTL for area registry cache.
func (c *CachedClient) areaRegTTL() time.Duration {
	if c.config.AreaRegTTLMin <= 0 {
		return 30 * time.Minute
	}
	return time.Duration(c.config.AreaRegTTLMin) * time.Minute
}

// InvalidateRegistryCaches invalidates entity and device registry caches.
// Should be called after creating or deleting helpers.
func (c *CachedClient) InvalidateRegistryCaches() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entityRegistryCache = nil
	c.deviceRegistryCache = nil
	c.areaRegistryCache = nil
	c.labelRegistryCache = nil
	c.floorRegistryCache = nil
	c.logger.Debug("Registry caches invalidated")
}

// GetServices returns cached services or fetches from API.
// Uses singleflight to prevent duplicate API calls during concurrent access.
func (c *CachedClient) GetServices(ctx context.Context) ([]Service, error) {
	// Check cache first with read lock
	c.mu.RLock()
	if c.servicesCache != nil && !c.servicesCache.isExpired() {
		data := c.servicesCache.data.([]Service)
		c.mu.RUnlock()
		c.logger.Trace("Services cache hit")
		return data, nil
	}
	c.mu.RUnlock()

	// Use singleflight to ensure only one goroutine fetches data
	result, err, _ := c.servicesGroup.Do("services", func() (any, error) {
		// Double-check cache after acquiring singleflight lock
		c.mu.RLock()
		if c.servicesCache != nil && !c.servicesCache.isExpired() {
			data := c.servicesCache.data.([]Service)
			c.mu.RUnlock()
			return data, nil
		}
		c.mu.RUnlock()

		// Fetch from API
		services, err := c.client.GetServices(ctx)
		if err != nil {
			return nil, err
		}

		// Store in cache
		c.mu.Lock()
		c.servicesCache = &cacheEntry{
			data:      services,
			expiresAt: time.Now().Add(c.servicesTTL()),
		}
		c.mu.Unlock()
		c.logger.Debug("Services cached", "count", len(services), "ttl", c.servicesTTL())

		return services, nil
	})

	if err != nil {
		return nil, err
	}
	return result.([]Service), nil
}

// GetConfig returns cached config or fetches from API.
// Uses singleflight to prevent duplicate API calls during concurrent access.
func (c *CachedClient) GetConfig(ctx context.Context) (*Config, error) {
	// Check cache first with read lock
	c.mu.RLock()
	if c.configCache != nil && !c.configCache.isExpired() {
		data := c.configCache.data.(*Config)
		c.mu.RUnlock()
		c.logger.Trace("Config cache hit")
		return data, nil
	}
	c.mu.RUnlock()

	// Use singleflight to ensure only one goroutine fetches data
	result, err, _ := c.configGroup.Do("config", func() (any, error) {
		// Double-check cache after acquiring singleflight lock
		c.mu.RLock()
		if c.configCache != nil && !c.configCache.isExpired() {
			data := c.configCache.data.(*Config)
			c.mu.RUnlock()
			return data, nil
		}
		c.mu.RUnlock()

		// Fetch from API
		cfg, err := c.client.GetConfig(ctx)
		if err != nil {
			return nil, err
		}

		// Store in cache
		c.mu.Lock()
		c.configCache = &cacheEntry{
			data:      cfg,
			expiresAt: time.Now().Add(c.configTTL()),
		}
		c.mu.Unlock()
		c.logger.Debug("Config cached", "ttl", c.configTTL())

		return cfg, nil
	})

	if err != nil {
		return nil, err
	}
	return result.(*Config), nil
}

// GetEntityRegistry returns cached entity registry or fetches from API.
// Uses singleflight to prevent duplicate API calls during concurrent access.
func (c *CachedClient) GetEntityRegistry(ctx context.Context) ([]EntityRegistryEntry, error) {
	// Check cache first with read lock
	c.mu.RLock()
	if c.entityRegistryCache != nil && !c.entityRegistryCache.isExpired() {
		data := c.entityRegistryCache.data.([]EntityRegistryEntry)
		c.mu.RUnlock()
		c.logger.Trace("Entity registry cache hit")
		return data, nil
	}
	c.mu.RUnlock()

	// Use singleflight to ensure only one goroutine fetches data
	result, err, _ := c.entityRegistryGroup.Do("entity_registry", func() (any, error) {
		// Double-check cache after acquiring singleflight lock
		c.mu.RLock()
		if c.entityRegistryCache != nil && !c.entityRegistryCache.isExpired() {
			data := c.entityRegistryCache.data.([]EntityRegistryEntry)
			c.mu.RUnlock()
			return data, nil
		}
		c.mu.RUnlock()

		// Fetch from API
		entities, err := c.client.GetEntityRegistry(ctx)
		if err != nil {
			return nil, err
		}

		// Store in cache
		c.mu.Lock()
		c.entityRegistryCache = &cacheEntry{
			data:      entities,
			expiresAt: time.Now().Add(c.entityRegTTL()),
		}
		c.mu.Unlock()
		c.logger.Debug("Entity registry cached", "count", len(entities), "ttl", c.entityRegTTL())

		return entities, nil
	})

	if err != nil {
		return nil, err
	}
	return result.([]EntityRegistryEntry), nil
}

// GetEntityRegistryEntry retrieves a single entity registry entry, bypassing the cache: the
// underlying WS call is already a targeted single-entity fetch (unlike GetEntityRegistry, which
// exists precisely to amortize the cost of the full-registry list call), so adding a per-entity
// cache here would duplicate that machinery for a call that is already cheap.
func (c *CachedClient) GetEntityRegistryEntry(ctx context.Context, entityID string) (*EntityRegistryEntry, error) {
	return c.client.GetEntityRegistryEntry(ctx, entityID)
}

// GetDeviceRegistry returns cached device registry or fetches from API.
// Uses singleflight to prevent duplicate API calls during concurrent access.
func (c *CachedClient) GetDeviceRegistry(ctx context.Context) ([]DeviceRegistryEntry, error) {
	// Check cache first with read lock
	c.mu.RLock()
	if c.deviceRegistryCache != nil && !c.deviceRegistryCache.isExpired() {
		data := c.deviceRegistryCache.data.([]DeviceRegistryEntry)
		c.mu.RUnlock()
		c.logger.Trace("Device registry cache hit")
		return data, nil
	}
	c.mu.RUnlock()

	// Use singleflight to ensure only one goroutine fetches data
	result, err, _ := c.deviceRegistryGroup.Do("device_registry", func() (any, error) {
		// Double-check cache after acquiring singleflight lock
		c.mu.RLock()
		if c.deviceRegistryCache != nil && !c.deviceRegistryCache.isExpired() {
			data := c.deviceRegistryCache.data.([]DeviceRegistryEntry)
			c.mu.RUnlock()
			return data, nil
		}
		c.mu.RUnlock()

		// Fetch from API
		devices, err := c.client.GetDeviceRegistry(ctx)
		if err != nil {
			return nil, err
		}

		// Store in cache
		c.mu.Lock()
		c.deviceRegistryCache = &cacheEntry{
			data:      devices,
			expiresAt: time.Now().Add(c.deviceRegTTL()),
		}
		c.mu.Unlock()
		c.logger.Debug("Device registry cached", "count", len(devices), "ttl", c.deviceRegTTL())

		return devices, nil
	})

	if err != nil {
		return nil, err
	}
	return result.([]DeviceRegistryEntry), nil
}

// GetAreaRegistry returns cached area registry or fetches from API.
// Uses singleflight to prevent duplicate API calls during concurrent access.
func (c *CachedClient) GetAreaRegistry(ctx context.Context) ([]AreaRegistryEntry, error) {
	// Check cache first with read lock
	c.mu.RLock()
	if c.areaRegistryCache != nil && !c.areaRegistryCache.isExpired() {
		data := c.areaRegistryCache.data.([]AreaRegistryEntry)
		c.mu.RUnlock()
		c.logger.Trace("Area registry cache hit")
		return data, nil
	}
	c.mu.RUnlock()

	// Use singleflight to ensure only one goroutine fetches data
	result, err, _ := c.areaRegistryGroup.Do("area_registry", func() (any, error) {
		// Double-check cache after acquiring singleflight lock
		c.mu.RLock()
		if c.areaRegistryCache != nil && !c.areaRegistryCache.isExpired() {
			data := c.areaRegistryCache.data.([]AreaRegistryEntry)
			c.mu.RUnlock()
			return data, nil
		}
		c.mu.RUnlock()

		// Fetch from API
		areas, err := c.client.GetAreaRegistry(ctx)
		if err != nil {
			return nil, err
		}

		// Store in cache
		c.mu.Lock()
		c.areaRegistryCache = &cacheEntry{
			data:      areas,
			expiresAt: time.Now().Add(c.areaRegTTL()),
		}
		c.mu.Unlock()
		c.logger.Debug("Area registry cached", "count", len(areas), "ttl", c.areaRegTTL())

		return areas, nil
	})

	if err != nil {
		return nil, err
	}
	return result.([]AreaRegistryEntry), nil
}

// GetLabelRegistry returns cached label registry or fetches from API.
// Uses singleflight to prevent duplicate API calls during concurrent access.
func (c *CachedClient) GetLabelRegistry(ctx context.Context) ([]LabelRegistryEntry, error) {
	// Check cache first with read lock
	c.mu.RLock()
	if c.labelRegistryCache != nil && !c.labelRegistryCache.isExpired() {
		data := c.labelRegistryCache.data.([]LabelRegistryEntry)
		c.mu.RUnlock()
		c.logger.Trace("Label registry cache hit")
		return data, nil
	}
	c.mu.RUnlock()

	// Use singleflight to ensure only one goroutine fetches data
	result, err, _ := c.labelRegistryGroup.Do("label_registry", func() (any, error) {
		// Double-check cache after acquiring singleflight lock
		c.mu.RLock()
		if c.labelRegistryCache != nil && !c.labelRegistryCache.isExpired() {
			data := c.labelRegistryCache.data.([]LabelRegistryEntry)
			c.mu.RUnlock()
			return data, nil
		}
		c.mu.RUnlock()

		// Fetch from API
		labels, err := c.client.GetLabelRegistry(ctx)
		if err != nil {
			return nil, err
		}

		// Store in cache
		c.mu.Lock()
		c.labelRegistryCache = &cacheEntry{
			data:      labels,
			expiresAt: time.Now().Add(c.areaRegTTL()),
		}
		c.mu.Unlock()
		c.logger.Debug("Label registry cached", "count", len(labels), "ttl", c.areaRegTTL())

		return labels, nil
	})

	if err != nil {
		return nil, err
	}
	return result.([]LabelRegistryEntry), nil
}

// GetFloorRegistry returns cached floor registry or fetches from API.
// Uses singleflight to prevent duplicate API calls during concurrent access.
func (c *CachedClient) GetFloorRegistry(ctx context.Context) ([]FloorRegistryEntry, error) {
	// Check cache first with read lock
	c.mu.RLock()
	if c.floorRegistryCache != nil && !c.floorRegistryCache.isExpired() {
		data := c.floorRegistryCache.data.([]FloorRegistryEntry)
		c.mu.RUnlock()
		c.logger.Trace("Floor registry cache hit")
		return data, nil
	}
	c.mu.RUnlock()

	// Use singleflight to ensure only one goroutine fetches data
	result, err, _ := c.floorRegistryGroup.Do("floor_registry", func() (any, error) {
		// Double-check cache after acquiring singleflight lock
		c.mu.RLock()
		if c.floorRegistryCache != nil && !c.floorRegistryCache.isExpired() {
			data := c.floorRegistryCache.data.([]FloorRegistryEntry)
			c.mu.RUnlock()
			return data, nil
		}
		c.mu.RUnlock()

		// Fetch from API
		floors, err := c.client.GetFloorRegistry(ctx)
		if err != nil {
			return nil, err
		}

		// Store in cache
		c.mu.Lock()
		c.floorRegistryCache = &cacheEntry{
			data:      floors,
			expiresAt: time.Now().Add(c.areaRegTTL()),
		}
		c.mu.Unlock()
		c.logger.Debug("Floor registry cached", "count", len(floors), "ttl", c.areaRegTTL())

		return floors, nil
	})

	if err != nil {
		return nil, err
	}
	return result.([]FloorRegistryEntry), nil
}

// CreateHelper creates a helper and invalidates registry caches.
func (c *CachedClient) CreateHelper(ctx context.Context, helper HelperConfig) error {
	err := c.client.CreateHelper(ctx, helper)
	if err == nil {
		c.InvalidateRegistryCaches()
	}
	return err
}

// DeleteHelper deletes a helper and invalidates registry caches.
func (c *CachedClient) DeleteHelper(ctx context.Context, helperID string) error {
	err := c.client.DeleteHelper(ctx, helperID)
	if err == nil {
		c.InvalidateRegistryCaches()
	}
	return err
}

// RemoveEntityRegistryEntry removes an entity and invalidates registry caches.
func (c *CachedClient) RemoveEntityRegistryEntry(ctx context.Context, entityID string) error {
	err := c.client.RemoveEntityRegistryEntry(ctx, entityID)
	if err == nil {
		c.InvalidateRegistryCaches()
	}
	return err
}

// RemoveDeviceConfigEntry removes a config entry from a device and invalidates registry caches.
func (c *CachedClient) RemoveDeviceConfigEntry(ctx context.Context, deviceID, configEntryID string) error {
	err := c.client.RemoveDeviceConfigEntry(ctx, deviceID, configEntryID)
	if err == nil {
		c.InvalidateRegistryCaches()
	}
	return err
}

// UpdateEntityRegistryEntry updates an entity and invalidates entity registry cache.
func (c *CachedClient) UpdateEntityRegistryEntry(ctx context.Context, entityID string, cfg EntityRegistryUpdateConfig) (*EntityRegistryEntry, error) {
	entry, err := c.client.UpdateEntityRegistryEntry(ctx, entityID, cfg)
	if err == nil {
		c.invalidateEntityRegistryCache()
	}
	return entry, err
}

// UpdateDeviceRegistryEntry updates a device and invalidates device registry cache.
func (c *CachedClient) UpdateDeviceRegistryEntry(ctx context.Context, deviceID string, cfg DeviceRegistryUpdateConfig) (*DeviceRegistryEntry, error) {
	entry, err := c.client.UpdateDeviceRegistryEntry(ctx, deviceID, cfg)
	if err == nil {
		c.invalidateDeviceRegistryCache()
	}
	return entry, err
}

// CreateArea creates an area and invalidates area registry cache.
func (c *CachedClient) CreateArea(ctx context.Context, cfg AreaConfig) (*AreaRegistryEntry, error) {
	entry, err := c.client.CreateArea(ctx, cfg)
	if err == nil {
		c.invalidateAreaRegistryCache()
	}
	return entry, err
}

// UpdateArea updates an area and invalidates area registry cache.
func (c *CachedClient) UpdateArea(ctx context.Context, areaID string, cfg AreaConfig) (*AreaRegistryEntry, error) {
	entry, err := c.client.UpdateArea(ctx, areaID, cfg)
	if err == nil {
		c.invalidateAreaRegistryCache()
	}
	return entry, err
}

// DeleteArea deletes an area and invalidates area registry cache.
func (c *CachedClient) DeleteArea(ctx context.Context, areaID string) error {
	err := c.client.DeleteArea(ctx, areaID)
	if err == nil {
		c.invalidateAreaRegistryCache()
	}
	return err
}

// CreateLabel creates a label and invalidates label registry cache.
func (c *CachedClient) CreateLabel(ctx context.Context, cfg LabelConfig) (*LabelRegistryEntry, error) {
	entry, err := c.client.CreateLabel(ctx, cfg)
	if err == nil {
		c.invalidateLabelRegistryCache()
	}
	return entry, err
}

// UpdateLabel updates a label and invalidates label registry cache.
func (c *CachedClient) UpdateLabel(ctx context.Context, labelID string, cfg LabelConfig) (*LabelRegistryEntry, error) {
	entry, err := c.client.UpdateLabel(ctx, labelID, cfg)
	if err == nil {
		c.invalidateLabelRegistryCache()
	}
	return entry, err
}

// DeleteLabel deletes a label and invalidates label registry cache.
func (c *CachedClient) DeleteLabel(ctx context.Context, labelID string) error {
	err := c.client.DeleteLabel(ctx, labelID)
	if err == nil {
		c.invalidateLabelRegistryCache()
	}
	return err
}

// CreateFloor creates a floor and invalidates floor registry cache.
func (c *CachedClient) CreateFloor(ctx context.Context, cfg FloorConfig) (*FloorRegistryEntry, error) {
	entry, err := c.client.CreateFloor(ctx, cfg)
	if err == nil {
		c.invalidateFloorRegistryCache()
	}
	return entry, err
}

// UpdateFloor updates a floor and invalidates floor registry cache.
func (c *CachedClient) UpdateFloor(ctx context.Context, floorID string, cfg FloorConfig) (*FloorRegistryEntry, error) {
	entry, err := c.client.UpdateFloor(ctx, floorID, cfg)
	if err == nil {
		c.invalidateFloorRegistryCache()
	}
	return entry, err
}

// DeleteFloor deletes a floor and invalidates floor registry cache.
func (c *CachedClient) DeleteFloor(ctx context.Context, floorID string) error {
	err := c.client.DeleteFloor(ctx, floorID)
	if err == nil {
		c.invalidateFloorRegistryCache()
	}
	return err
}

// invalidateAreaRegistryCache clears the area registry cache.
func (c *CachedClient) invalidateAreaRegistryCache() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.areaRegistryCache = nil
	c.logger.Debug("Area registry cache invalidated")
}

// invalidateLabelRegistryCache clears the label registry cache.
func (c *CachedClient) invalidateLabelRegistryCache() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.labelRegistryCache = nil
	c.logger.Debug("Label registry cache invalidated")
}

// invalidateFloorRegistryCache clears the floor registry cache.
func (c *CachedClient) invalidateFloorRegistryCache() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.floorRegistryCache = nil
	c.logger.Debug("Floor registry cache invalidated")
}

func (c *CachedClient) invalidateEntityRegistryCache() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entityRegistryCache = nil
	c.logger.Debug("Entity registry cache invalidated")
}

func (c *CachedClient) invalidateDeviceRegistryCache() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deviceRegistryCache = nil
	c.logger.Debug("Device registry cache invalidated")
}

// GetZones delegates to the underlying client (no caching - zones change dynamically).
func (c *CachedClient) GetZones(ctx context.Context) ([]ZoneRegistryEntry, error) {
	return c.client.GetZones(ctx)
}

// CreateZone delegates to the underlying client.
func (c *CachedClient) CreateZone(ctx context.Context, cfg ZoneConfig) (*ZoneRegistryEntry, error) {
	return c.client.CreateZone(ctx, cfg)
}

// UpdateZone delegates to the underlying client.
func (c *CachedClient) UpdateZone(ctx context.Context, zoneID string, cfg ZoneConfig) (*ZoneRegistryEntry, error) {
	return c.client.UpdateZone(ctx, zoneID, cfg)
}

// DeleteZone delegates to the underlying client.
func (c *CachedClient) DeleteZone(ctx context.Context, zoneID string) error {
	return c.client.DeleteZone(ctx, zoneID)
}

// GetPersons delegates to the underlying client (no caching - persons change dynamically).
func (c *CachedClient) GetPersons(ctx context.Context) ([]PersonRegistryEntry, error) {
	return c.client.GetPersons(ctx)
}

// CreatePerson delegates to the underlying client.
func (c *CachedClient) CreatePerson(ctx context.Context, cfg PersonConfig) (*PersonRegistryEntry, error) {
	return c.client.CreatePerson(ctx, cfg)
}

// UpdatePerson delegates to the underlying client.
func (c *CachedClient) UpdatePerson(ctx context.Context, personID string, cfg PersonConfig) (*PersonRegistryEntry, error) {
	return c.client.UpdatePerson(ctx, personID, cfg)
}

// DeletePerson delegates to the underlying client.
func (c *CachedClient) DeletePerson(ctx context.Context, personID string) error {
	return c.client.DeletePerson(ctx, personID)
}

// GetTags delegates to the underlying client (no caching - tags change when scanned).
func (c *CachedClient) GetTags(ctx context.Context) ([]TagRegistryEntry, error) {
	return c.client.GetTags(ctx)
}

// CreateTag delegates to the underlying client.
func (c *CachedClient) CreateTag(ctx context.Context, cfg TagConfig) (*TagRegistryEntry, error) {
	return c.client.CreateTag(ctx, cfg)
}

// UpdateTag delegates to the underlying client.
func (c *CachedClient) UpdateTag(ctx context.Context, tagID string, cfg TagConfig) (*TagRegistryEntry, error) {
	return c.client.UpdateTag(ctx, tagID, cfg)
}

// DeleteTag delegates to the underlying client.
func (c *CachedClient) DeleteTag(ctx context.Context, tagID string) error {
	return c.client.DeleteTag(ctx, tagID)
}

// Delegated methods - all other Client interface methods delegate to the underlying client.
// These methods are pass-through implementations that don't require caching.

//nolint:revive // Delegated method
func (c *CachedClient) GetStates(ctx context.Context) ([]Entity, error) {
	return c.client.GetStates(ctx)
}

//nolint:revive // Delegated method
func (c *CachedClient) GetState(ctx context.Context, entityID string) (*Entity, error) {
	return c.client.GetState(ctx, entityID)
}

//nolint:revive // Delegated method
func (c *CachedClient) SetState(ctx context.Context, entityID string, state StateUpdate) (*Entity, error) {
	return c.client.SetState(ctx, entityID, state)
}

//nolint:revive // Delegated method
func (c *CachedClient) GetHistory(ctx context.Context, entityID string, start, end time.Time) ([][]HistoryEntry, error) {
	return c.client.GetHistory(ctx, entityID, start, end)
}

//nolint:revive // Delegated method
func (c *CachedClient) ListAutomations(ctx context.Context) ([]Automation, error) {
	return c.client.ListAutomations(ctx)
}

//nolint:revive // Delegated method
func (c *CachedClient) GetAutomation(ctx context.Context, automationID string) (*Automation, error) {
	return c.client.GetAutomation(ctx, automationID)
}

//nolint:revive // Delegated method
func (c *CachedClient) CreateAutomation(ctx context.Context, automation AutomationConfig) error {
	return c.client.CreateAutomation(ctx, automation)
}

//nolint:revive // Delegated method
func (c *CachedClient) UpdateAutomation(ctx context.Context, automationID string, automation AutomationConfig) error {
	return c.client.UpdateAutomation(ctx, automationID, automation)
}

//nolint:revive // Delegated method
func (c *CachedClient) DeleteAutomation(ctx context.Context, automationID string) error {
	return c.client.DeleteAutomation(ctx, automationID)
}

//nolint:revive // Delegated method
func (c *CachedClient) ToggleAutomation(ctx context.Context, entityID string, enabled bool) error {
	return c.client.ToggleAutomation(ctx, entityID, enabled)
}

//nolint:revive // Delegated method
func (c *CachedClient) ListHelpers(ctx context.Context) ([]Entity, error) {
	return c.client.ListHelpers(ctx)
}

//nolint:revive // Delegated method
func (c *CachedClient) UpdateHelper(ctx context.Context, helperID string, helper HelperConfig) error {
	return c.client.UpdateHelper(ctx, helperID, helper)
}

//nolint:revive // Delegated method
func (c *CachedClient) SetHelperValue(ctx context.Context, entityID string, value any) error {
	return c.client.SetHelperValue(ctx, entityID, value)
}

//nolint:revive // Delegated method
func (c *CachedClient) ListScripts(ctx context.Context) ([]Entity, error) {
	return c.client.ListScripts(ctx)
}

//nolint:revive // Delegated method
func (c *CachedClient) GetScript(ctx context.Context, scriptID string) (*Script, error) {
	return c.client.GetScript(ctx, scriptID)
}

//nolint:revive // Delegated method
func (c *CachedClient) CreateScript(ctx context.Context, scriptID string, script ScriptConfig) error {
	return c.client.CreateScript(ctx, scriptID, script)
}

//nolint:revive // Delegated method
func (c *CachedClient) UpdateScript(ctx context.Context, scriptID string, script ScriptConfig) error {
	return c.client.UpdateScript(ctx, scriptID, script)
}

//nolint:revive // Delegated method
func (c *CachedClient) DeleteScript(ctx context.Context, scriptID string) error {
	return c.client.DeleteScript(ctx, scriptID)
}

//nolint:revive // Delegated method
func (c *CachedClient) ListScenes(ctx context.Context) ([]Entity, error) {
	return c.client.ListScenes(ctx)
}

//nolint:revive // Delegated method
func (c *CachedClient) GetScene(ctx context.Context, sceneID string) (*Scene, error) {
	return c.client.GetScene(ctx, sceneID)
}

//nolint:revive // Delegated method
func (c *CachedClient) CreateScene(ctx context.Context, sceneID string, scene SceneConfig) error {
	return c.client.CreateScene(ctx, sceneID, scene)
}

//nolint:revive // Delegated method
func (c *CachedClient) UpdateScene(ctx context.Context, sceneID string, scene SceneConfig) error {
	return c.client.UpdateScene(ctx, sceneID, scene)
}

//nolint:revive // Delegated method
func (c *CachedClient) DeleteScene(ctx context.Context, sceneID string) error {
	return c.client.DeleteScene(ctx, sceneID)
}

// ConfigFileEntryExists is intentionally never cached: a stale "exists" answer would defeat
// the exact hazard this probe exists to catch (#122, #164).
func (c *CachedClient) ConfigFileEntryExists(ctx context.Context, domain, configID string) (bool, error) {
	return c.client.ConfigFileEntryExists(ctx, domain, configID)
}

//nolint:revive // Delegated method
func (c *CachedClient) CallService(ctx context.Context, domain, service string, data map[string]any) ([]Entity, error) {
	return c.client.CallService(ctx, domain, service, data)
}

//nolint:revive // Delegated method
func (c *CachedClient) CallServiceWithResponse(ctx context.Context, domain, service string, data map[string]any) (map[string]any, error) {
	return c.client.CallServiceWithResponse(ctx, domain, service, data)
}

//nolint:revive // Delegated method
func (c *CachedClient) SignPath(ctx context.Context, path string, expires int) (string, error) {
	return c.client.SignPath(ctx, path, expires)
}

//nolint:revive // Delegated method
func (c *CachedClient) GetCameraStream(ctx context.Context, entityID string) (*StreamInfo, error) {
	return c.client.GetCameraStream(ctx, entityID)
}

//nolint:revive // Delegated method
func (c *CachedClient) BrowseMedia(ctx context.Context, mediaContentID string) (*MediaBrowseResult, error) {
	return c.client.BrowseMedia(ctx, mediaContentID)
}

//nolint:revive // Delegated method
func (c *CachedClient) GetLovelaceConfig(ctx context.Context, urlPath string) (map[string]any, error) {
	return c.client.GetLovelaceConfig(ctx, urlPath)
}

//nolint:revive // Delegated method
func (c *CachedClient) SaveLovelaceConfig(ctx context.Context, urlPath string, cfg map[string]any) error {
	return c.client.SaveLovelaceConfig(ctx, urlPath, cfg)
}

//nolint:revive // Delegated method
func (c *CachedClient) ListDashboards(ctx context.Context) ([]DashboardEntry, error) {
	return c.client.ListDashboards(ctx)
}

//nolint:revive // Delegated method
func (c *CachedClient) CreateDashboard(ctx context.Context, cfg DashboardConfig) (*DashboardEntry, error) {
	return c.client.CreateDashboard(ctx, cfg)
}

//nolint:revive // Delegated method
func (c *CachedClient) UpdateDashboard(ctx context.Context, dashboardID string, cfg DashboardConfig) (*DashboardEntry, error) {
	return c.client.UpdateDashboard(ctx, dashboardID, cfg)
}

//nolint:revive // Delegated method
func (c *CachedClient) DeleteDashboard(ctx context.Context, dashboardID string) error {
	return c.client.DeleteDashboard(ctx, dashboardID)
}

//nolint:revive // Delegated method
func (c *CachedClient) GetStatistics(ctx context.Context, statIDs []string, period string) ([]StatisticsResult, error) {
	return c.client.GetStatistics(ctx, statIDs, period)
}

//nolint:revive // Delegated method
func (c *CachedClient) GetTriggersForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error) {
	return c.client.GetTriggersForTarget(ctx, target, expandGroup)
}

//nolint:revive // Delegated method
func (c *CachedClient) GetConditionsForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error) {
	return c.client.GetConditionsForTarget(ctx, target, expandGroup)
}

//nolint:revive // Delegated method
func (c *CachedClient) GetServicesForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error) {
	return c.client.GetServicesForTarget(ctx, target, expandGroup)
}

//nolint:revive // Delegated method
func (c *CachedClient) ExtractFromTarget(ctx context.Context, target Target, expandGroup *bool) (*ExtractFromTargetResult, error) {
	return c.client.ExtractFromTarget(ctx, target, expandGroup)
}

//nolint:revive // Delegated method
func (c *CachedClient) GetHelperConfig(ctx context.Context, platform, entityID string) (map[string]any, error) {
	return c.client.GetHelperConfig(ctx, platform, entityID)
}

//nolint:revive // Delegated method
func (c *CachedClient) RenderTemplate(ctx context.Context, template string) (string, error) {
	return c.client.RenderTemplate(ctx, template)
}

//nolint:revive // Delegated method
func (c *CachedClient) GetLogbook(ctx context.Context, startTime, endTime, entityID string) ([]LogbookEntry, error) {
	return c.client.GetLogbook(ctx, startTime, endTime, entityID)
}

//nolint:revive // Delegated method
func (c *CachedClient) CheckConfig(ctx context.Context) (*ConfigCheckResult, error) {
	return c.client.CheckConfig(ctx)
}

//nolint:revive // Delegated method
func (c *CachedClient) GetCalendars(ctx context.Context) ([]CalendarEntry, error) {
	return c.client.GetCalendars(ctx)
}

//nolint:revive // Delegated method
func (c *CachedClient) GetCalendarEvents(ctx context.Context, entityID, start, end string) ([]CalendarEvent, error) {
	return c.client.GetCalendarEvents(ctx, entityID, start, end)
}

//nolint:revive // Delegated method
func (c *CachedClient) GetCameraSnapshot(ctx context.Context, entityID string) ([]byte, string, error) {
	return c.client.GetCameraSnapshot(ctx, entityID)
}

//nolint:revive // Delegated method
func (c *CachedClient) GetSystemLog(ctx context.Context) ([]SystemLogEntry, error) {
	return c.client.GetSystemLog(ctx)
}

//nolint:revive // Delegated method
func (c *CachedClient) ClearSystemLog(ctx context.Context) error {
	return c.client.ClearSystemLog(ctx)
}

//nolint:revive // Delegated method
func (c *CachedClient) GetConfigEntries(ctx context.Context, domain string) ([]ConfigEntryFull, error) {
	return c.client.GetConfigEntries(ctx, domain)
}

//nolint:revive // Delegated method
func (c *CachedClient) GetConfigEntry(ctx context.Context, entryID string) (*ConfigEntryFull, error) {
	return c.client.GetConfigEntry(ctx, entryID)
}

// GetConfigEntryOptions retrieves config entry options via live Options Flow (no caching).
func (c *CachedClient) GetConfigEntryOptions(ctx context.Context, entryID string) (map[string]any, error) {
	return c.client.GetConfigEntryOptions(ctx, entryID)
}

// DeleteConfigEntry deletes a config entry and invalidates registry caches, since
// removing a config entry removes its associated devices and entities.
func (c *CachedClient) DeleteConfigEntry(ctx context.Context, entryID string) (bool, error) {
	requireRestart, err := c.client.DeleteConfigEntry(ctx, entryID)
	if err == nil {
		c.InvalidateRegistryCaches()
	}
	return requireRestart, err
}

//nolint:revive // Delegated method
func (c *CachedClient) SendHACSCommand(ctx context.Context, command string, data map[string]any) (any, error) {
	return c.client.SendHACSCommand(ctx, command, data)
}

// Close implements ClientCloser interface.
func (c *CachedClient) Close() error {
	return CloseClient(c.client)
}

// IsConnected delegates to the underlying client.
func (c *CachedClient) IsConnected() bool {
	if checker, ok := c.client.(interface{ IsConnected() bool }); ok {
		return checker.IsConnected()
	}
	return true
}

// Ensure CachedClient implements Client and ClientCloser interfaces.
var (
	_ Client       = (*CachedClient)(nil)
	_ ClientCloser = (*CachedClient)(nil)
)
