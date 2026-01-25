//go:build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// TestConfig holds configuration for integration tests.
type TestConfig struct {
	URL     string
	Token   string
	Timeout time.Duration
}

// LoadTestConfig loads test configuration from environment variables.
// Returns nil and skips the test if required variables are not set.
func LoadTestConfig(t *testing.T) *TestConfig {
	t.Helper()
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
	client homeassistant.Client
	config *TestConfig
	ctx    context.Context
	cancel context.CancelFunc
}

// SetupSuite runs before all tests in the suite.
func (s *IntegrationTestSuite) SetupSuite() {
	s.config = LoadTestConfig(s.T())
	if s.config == nil {
		return
	}

	s.ctx, s.cancel = context.WithTimeout(context.Background(), s.config.Timeout)

	// Create client using factory - use a connection context
	connCtx, connCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer connCancel()

	client, err := homeassistant.NewDefaultWSClient(connCtx, s.config.URL, s.config.Token)
	require.NoError(s.T(), err, "Failed to create Home Assistant client")
	s.client = client

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

// ScriptTestSuite is a specialized suite for script tests.
type ScriptTestSuite struct {
	IntegrationTestSuite
}

// SceneTestSuite is a specialized suite for scene tests.
type SceneTestSuite struct {
	IntegrationTestSuite
}
