//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/handlers"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// TestConfig holds configuration for integration tests.
type TestConfig struct {
	URL     string
	Token   string
	Timeout time.Duration
}

// LoadTestConfig loads test configuration from environment variables.
// It first attempts to load from .env.integration file in the project root.
// Returns nil and skips the test if required variables are not set.
func LoadTestConfig(t *testing.T) *TestConfig {
	t.Helper()

	// Try to load .env.integration file from various locations
	envFiles := []string{
		".env.integration",
		filepath.Join("..", "..", "..", ".env.integration"), // From integration test dir to project root
	}
	for _, envFile := range envFiles {
		if err := godotenv.Load(envFile); err == nil {
			t.Logf("Loaded environment from %s", envFile)
			break
		}
	}

	url := os.Getenv("HA_INTEGRATION_TEST_URL")
	token := os.Getenv("HA_INTEGRATION_TEST_TOKEN")

	if url == "" || token == "" {
		t.Skip("Skipping integration test: HA_INTEGRATION_TEST_URL and HA_INTEGRATION_TEST_TOKEN must be set")
		return nil
	}

	timeout := 5 * time.Minute
	if timeoutStr := os.Getenv("HA_INTEGRATION_TEST_TIMEOUT"); timeoutStr != "" {
		if d, err := time.ParseDuration(timeoutStr); err == nil {
			timeout = d
		}
	}

	return &TestConfig{
		URL:     url,
		Token:   token,
		Timeout: timeout,
	}
}

// IntegrationTestSuite is the base test suite for integration tests.
type IntegrationTestSuite struct {
	suite.Suite
	client   homeassistant.Client
	registry *mcp.Registry
	config   *TestConfig
	ctx      context.Context
	cancel   context.CancelFunc
}

// SetupSuite runs before all tests in the suite.
func (s *IntegrationTestSuite) SetupSuite() {
	s.config = LoadTestConfig(s.T())
	if s.config == nil {
		return
	}

	s.ctx, s.cancel = context.WithTimeout(context.Background(), s.config.Timeout)

	// Create client using factory - use the suite context so connection lives for the duration of tests
	client, err := homeassistant.NewDefaultWSClient(s.ctx, s.config.URL, s.config.Token)
	require.NoError(s.T(), err, "Failed to create Home Assistant client")
	s.client = client

	// Build the real tool registry once per suite - this is the layer where a
	// handler argument-parsing bug (manage_helper update passing the wrong
	// identifier to config-entry routing) once failed silently, which no
	// prior integration test exercised.
	s.registry = mcp.NewRegistry()
	handlers.RegisterAllTools(s.registry)

	// Pre-test cleanup: Remove any leftover test entities from previous runs
	s.T().Log("Running pre-test cleanup...")
	if err := CleanupAllTestEntities(s.ctx, s.client); err != nil {
		s.T().Logf("Pre-test cleanup warning: %v", err)
	}
}

// TearDownSuite runs after all tests in the suite.
func (s *IntegrationTestSuite) TearDownSuite() {
	if s.client == nil {
		return
	}

	// Final cleanup
	s.T().Log("Running post-test cleanup...")
	cleanupCtx, cancel := context.WithTimeout(context.Background(), CleanupTimeout)
	defer cancel()

	if err := CleanupAllTestEntities(cleanupCtx, s.client); err != nil {
		s.T().Logf("Post-test cleanup warning: %v", err)
	}

	// Verify no test entities remain
	count, entities, err := CountTestEntities(cleanupCtx, s.client)
	if err != nil {
		s.T().Logf("Failed to verify cleanup: %v", err)
	} else if count > 0 {
		s.T().Errorf("Test entities still remain after cleanup: %v", entities)
	}

	// Verify no test areas remain
	areaCount, areas, err := CountTestAreas(cleanupCtx, s.client)
	if err != nil {
		s.T().Logf("Failed to verify area cleanup: %v", err)
	} else if areaCount > 0 {
		s.T().Errorf("Test areas still remain after cleanup: %v", areas)
	}

	// Verify no test dashboards remain
	dashboardCount, dashboards, err := CountTestDashboards(cleanupCtx, s.client)
	if err != nil {
		s.T().Logf("Failed to verify dashboard cleanup: %v", err)
	} else if dashboardCount > 0 {
		s.T().Errorf("Test dashboards still remain after cleanup: %v", dashboards)
	}

	// Verify no test labels remain
	labelCount, labels, err := CountTestLabels(cleanupCtx, s.client)
	if err != nil {
		s.T().Logf("Failed to verify label cleanup: %v", err)
	} else if labelCount > 0 {
		s.T().Errorf("Test labels still remain after cleanup: %v", labels)
	}

	// Verify no test floors remain
	floorCount, floors, err := CountTestFloors(cleanupCtx, s.client)
	if err != nil {
		s.T().Logf("Failed to verify floor cleanup: %v", err)
	} else if floorCount > 0 {
		s.T().Errorf("Test floors still remain after cleanup: %v", floors)
	}

	// Verify no test tags remain
	tagCount, tags, err := CountTestTags(cleanupCtx, s.client)
	if err != nil {
		s.T().Logf("Failed to verify tag cleanup: %v", err)
	} else if tagCount > 0 {
		s.T().Errorf("Test tags still remain after cleanup: %v", tags)
	}

	// Verify no test zones remain
	zoneCount, zones, err := CountTestZones(cleanupCtx, s.client)
	if err != nil {
		s.T().Logf("Failed to verify zone cleanup: %v", err)
	} else if zoneCount > 0 {
		s.T().Errorf("Test zones still remain after cleanup: %v", zones)
	}

	// Verify no test persons remain
	personCount, persons, err := CountTestPersons(cleanupCtx, s.client)
	if err != nil {
		s.T().Logf("Failed to verify person cleanup: %v", err)
	} else if personCount > 0 {
		s.T().Errorf("Test persons still remain after cleanup: %v", persons)
	}

	if s.cancel != nil {
		s.cancel()
	}

	// Close client using the factory helper
	_ = homeassistant.CloseClient(s.client)
}

// Context returns the test context.
func (s *IntegrationTestSuite) Context() context.Context {
	return s.ctx
}

// Client returns the Home Assistant client.
func (s *IntegrationTestSuite) Client() homeassistant.Client {
	return s.client
}

// CallTool dispatches a tool call through the real registry + handler layer
// (tool name -> registry lookup -> handler -> real HybridClient), using the
// suite's live Home Assistant client. This is the layer where the
// manage_helper update argument-passing bug lived but no prior integration
// test exercised - every existing test in this package calls Client()
// methods directly instead.
func (s *IntegrationTestSuite) CallTool(name string, args map[string]any) *mcp.ToolsCallResult {
	handler, ok := s.registry.GetHandler(name)
	s.Require().True(ok, "tool %q is not registered", name)

	result, err := handler(s.ctx, s.client, args)
	s.Require().NoError(err, "handler for %q returned a Go error - handlers must report domain errors via IsError, not a Go error", name)

	return result
}

// resultText extracts the primary text content from a tool call result,
// mirroring the pattern used by internal/handlers unit test helpers.
func resultText(r *mcp.ToolsCallResult) string {
	if len(r.Content) == 0 {
		return ""
	}
	return r.Content[0].Text
}

// RegisterCleanup registers a cleanup function to be called after the test.
// This ensures cleanup happens even if the test fails.
func (s *IntegrationTestSuite) RegisterCleanup(cleanupFn func()) {
	s.T().Cleanup(cleanupFn)
}

// WaitForEntity waits for an entity to appear with a specific state.
// This is useful after creating entities that take time to initialize.
func (s *IntegrationTestSuite) WaitForEntity(entityID string, timeout time.Duration) (*homeassistant.Entity, error) {
	ctx, cancel := context.WithTimeout(s.ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			entity, err := s.client.GetState(ctx, entityID)
			if err == nil && entity != nil {
				return entity, nil
			}
		}
	}
}

// WaitForEntityGone waits for an entity to disappear.
// This is useful after deleting entities.
func (s *IntegrationTestSuite) WaitForEntityGone(entityID string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(s.ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			_, err := s.client.GetState(ctx, entityID)
			if err != nil {
				// Entity not found, which is what we want
				return nil
			}
		}
	}
}

// HelperTestSuite is a specialized suite for helper tests.
type HelperTestSuite struct {
	IntegrationTestSuite
}

// AutomationTestSuite is a specialized suite for automation tests.
type AutomationTestSuite struct {
	IntegrationTestSuite
}

// WaitForAutomation waits for an automation to appear in ListAutomations().
// This is different from WaitForEntity because automations may not be immediately
// available via GetState, but are visible via ListAutomations.
// After finding the automation, it calls GetAutomation to fetch the full config.
func (s *AutomationTestSuite) WaitForAutomation(automationID string, timeout time.Duration) (*homeassistant.Automation, error) {
	ctx, cancel := context.WithTimeout(s.ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	attempts := 0
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			attempts++
			automations, err := s.client.ListAutomations(ctx)
			if err != nil {
				continue
			}
			// Check both Config.ID and EntityID (without prefix)
			// Note: ListAutomations doesn't populate Config, so only EntityID check works
			for _, automation := range automations {
				expectedEntityID := "automation." + automationID
				if automation.EntityID == expectedEntityID {
					// Found it! Now fetch full config via GetAutomation
					fullAuto, err := s.client.GetAutomation(ctx, automationID)
					if err != nil {
						continue // Might not be ready yet, keep waiting
					}
					return fullAuto, nil
				}
			}
			// Log every 10 attempts (~5 seconds)
			if attempts%10 == 0 {
				fmt.Printf("  [WaitForAutomation] Still waiting after %d attempts, total automations: %d\n", attempts, len(automations))
			}
		}
	}
}

// ScriptTestSuite is a specialized suite for script tests.
type ScriptTestSuite struct {
	IntegrationTestSuite
}

// SceneTestSuite is a specialized suite for scene tests.
type SceneTestSuite struct {
	IntegrationTestSuite
}

// WaitForScene waits for a scene to appear in ListScenes().
// This is different from WaitForEntity because scenes may not be immediately
// available via GetState, but are visible via ListScenes.
func (s *SceneTestSuite) WaitForScene(sceneID string, timeout time.Duration) (*homeassistant.Entity, error) {
	ctx, cancel := context.WithTimeout(s.ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// Build expected entity ID
	expectedEntityID := "scene." + sceneID

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			scenes, err := s.client.ListScenes(ctx)
			if err != nil {
				continue
			}
			for _, scene := range scenes {
				if scene.EntityID == expectedEntityID {
					return &scene, nil
				}
			}
		}
	}
}

// AreaTestSuite is a specialized suite for area tests.
type AreaTestSuite struct {
	IntegrationTestSuite
}

// FindAreaByID finds an area by ID in the area registry.
func (s *AreaTestSuite) FindAreaByID(areaID string) (*homeassistant.AreaRegistryEntry, error) {
	areas, err := s.Client().GetAreaRegistry(s.Context())
	if err != nil {
		return nil, err
	}

	for _, area := range areas {
		if area.AreaID == areaID {
			return &area, nil
		}
	}

	return nil, fmt.Errorf("area %s not found in registry", areaID)
}

// DashboardTestSuite is a specialized suite for dashboard tests.
type DashboardTestSuite struct {
	IntegrationTestSuite
}

// FindDashboardByURLPath finds a dashboard by URL path in the dashboard list.
func (s *DashboardTestSuite) FindDashboardByURLPath(urlPath string) (*homeassistant.DashboardEntry, error) {
	dashboards, err := s.Client().ListDashboards(s.Context())
	if err != nil {
		return nil, err
	}

	for _, dashboard := range dashboards {
		if dashboard.URLPath == urlPath {
			return &dashboard, nil
		}
	}

	return nil, fmt.Errorf("dashboard with url_path %s not found", urlPath)
}

// LabelTestSuite is a specialized suite for label tests.
type LabelTestSuite struct {
	IntegrationTestSuite
}

// FindLabelByID finds a label by ID in the label registry.
func (s *LabelTestSuite) FindLabelByID(labelID string) (*homeassistant.LabelRegistryEntry, error) {
	labels, err := s.Client().GetLabelRegistry(s.Context())
	if err != nil {
		return nil, err
	}

	for _, label := range labels {
		if label.LabelID == labelID {
			return &label, nil
		}
	}

	return nil, fmt.Errorf("label %s not found in registry", labelID)
}

// FloorTestSuite is a specialized suite for floor tests.
type FloorTestSuite struct {
	IntegrationTestSuite
}

// FindFloorByID finds a floor by ID in the floor registry.
func (s *FloorTestSuite) FindFloorByID(floorID string) (*homeassistant.FloorRegistryEntry, error) {
	floors, err := s.Client().GetFloorRegistry(s.Context())
	if err != nil {
		return nil, err
	}

	for _, floor := range floors {
		if floor.FloorID == floorID {
			return &floor, nil
		}
	}

	return nil, fmt.Errorf("floor %s not found in registry", floorID)
}

// TagTestSuite is a specialized suite for tag tests.
type TagTestSuite struct {
	IntegrationTestSuite
}

// FindTagByID finds a tag by ID in the tag registry.
func (s *TagTestSuite) FindTagByID(tagID string) (*homeassistant.TagRegistryEntry, error) {
	tags, err := s.Client().GetTags(s.Context())
	if err != nil {
		return nil, err
	}

	for _, tag := range tags {
		if tag.TagID == tagID {
			return &tag, nil
		}
	}

	return nil, fmt.Errorf("tag %s not found in registry", tagID)
}

// ZoneTestSuite is a specialized suite for zone tests.
type ZoneTestSuite struct {
	IntegrationTestSuite
}

// FindZoneByID finds a zone by ID in the zone registry.
func (s *ZoneTestSuite) FindZoneByID(zoneID string) (*homeassistant.ZoneRegistryEntry, error) {
	zones, err := s.Client().GetZones(s.Context())
	if err != nil {
		return nil, err
	}

	for _, zone := range zones {
		if zone.ID == zoneID {
			return &zone, nil
		}
	}

	return nil, fmt.Errorf("zone %s not found in registry", zoneID)
}

// PersonTestSuite is a specialized suite for person tests.
type PersonTestSuite struct {
	IntegrationTestSuite
}

// FindPersonByID finds a person by ID in the person registry.
func (s *PersonTestSuite) FindPersonByID(personID string) (*homeassistant.PersonRegistryEntry, error) {
	persons, err := s.Client().GetPersons(s.Context())
	if err != nil {
		return nil, err
	}

	for _, person := range persons {
		if person.ID == personID {
			return &person, nil
		}
	}

	return nil, fmt.Errorf("person %s not found in registry", personID)
}
