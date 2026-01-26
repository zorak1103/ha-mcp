// Package homeassistant provides client factories and management for Home Assistant API.
package homeassistant

import (
	"context"
	"sync"
	"time"

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
// Thread-safe for concurrent access.
type CachedClient struct {
	client Client
	mu     sync.RWMutex

	servicesCache       *cacheEntry
	configCache         *cacheEntry
	entityRegistryCache *cacheEntry
	deviceRegistryCache *cacheEntry
	areaRegistryCache   *cacheEntry

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
	c.logger.Debug("Registry caches invalidated")
}

// GetServices returns cached services or fetches from API.
func (c *CachedClient) GetServices(ctx context.Context) ([]Service, error) {
	c.mu.RLock()
	if c.servicesCache != nil && !c.servicesCache.isExpired() {
		data := c.servicesCache.data.([]Service)
		c.mu.RUnlock()
		c.logger.Trace("Services cache hit")
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
}

// GetConfig returns cached config or fetches from API.
func (c *CachedClient) GetConfig(ctx context.Context) (*Config, error) {
	c.mu.RLock()
	if c.configCache != nil && !c.configCache.isExpired() {
		data := c.configCache.data.(*Config)
		c.mu.RUnlock()
		c.logger.Trace("Config cache hit")
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
}

// GetEntityRegistry returns cached entity registry or fetches from API.
func (c *CachedClient) GetEntityRegistry(ctx context.Context) ([]EntityRegistryEntry, error) {
	c.mu.RLock()
	if c.entityRegistryCache != nil && !c.entityRegistryCache.isExpired() {
		data := c.entityRegistryCache.data.([]EntityRegistryEntry)
		c.mu.RUnlock()
		c.logger.Trace("Entity registry cache hit")
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
}

// GetDeviceRegistry returns cached device registry or fetches from API.
func (c *CachedClient) GetDeviceRegistry(ctx context.Context) ([]DeviceRegistryEntry, error) {
	c.mu.RLock()
	if c.deviceRegistryCache != nil && !c.deviceRegistryCache.isExpired() {
		data := c.deviceRegistryCache.data.([]DeviceRegistryEntry)
		c.mu.RUnlock()
		c.logger.Trace("Device registry cache hit")
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
}

// GetAreaRegistry returns cached area registry or fetches from API.
func (c *CachedClient) GetAreaRegistry(ctx context.Context) ([]AreaRegistryEntry, error) {
	c.mu.RLock()
	if c.areaRegistryCache != nil && !c.areaRegistryCache.isExpired() {
		data := c.areaRegistryCache.data.([]AreaRegistryEntry)
		c.mu.RUnlock()
		c.logger.Trace("Area registry cache hit")
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

//nolint:revive // Delegated method
func (c *CachedClient) CallService(ctx context.Context, domain, service string, data map[string]any) ([]Entity, error) {
	return c.client.CallService(ctx, domain, service, data)
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
func (c *CachedClient) GetLovelaceConfig(ctx context.Context) (map[string]any, error) {
	return c.client.GetLovelaceConfig(ctx)
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
func (c *CachedClient) GetScheduleConfig(ctx context.Context, scheduleID string) (map[string]any, error) {
	return c.client.GetScheduleConfig(ctx, scheduleID)
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
