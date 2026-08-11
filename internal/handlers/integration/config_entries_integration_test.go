//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

type ConfigEntriesIntegrationTestSuite struct {
	HelperTestSuite
}

func TestConfigEntriesIntegration(t *testing.T) {
	suite.Run(t, new(ConfigEntriesIntegrationTestSuite))
}

func (s *ConfigEntriesIntegrationTestSuite) TestListConfigEntries() {
	// List all config entries
	entries, err := s.Client().GetConfigEntries(s.Context(), "")
	s.Require().NoError(err, "Failed to list config entries")
	s.NotEmpty(entries, "Should have at least some config entries in Home Assistant")

	// Verify entries have expected fields
	for _, entry := range entries {
		s.NotEmpty(entry.EntryID, "Entry should have entry_id")
		s.NotEmpty(entry.Domain, "Entry should have domain")
	}
}

func (s *ConfigEntriesIntegrationTestSuite) TestListConfigEntriesFilterByDomain() {
	// First, create a template sensor to ensure at least one template entry exists
	templateName := GenerateTestID("cfg_entry_domain")
	templateEntityID := BuildEntityID("sensor", templateName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), templateEntityID)
	})

	// Create template sensor
	templateConfig := homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"name":  templateName,
			"state": "{{ 42 }}",
		},
	}

	err := s.Client().CreateHelper(s.Context(), templateConfig)
	s.Require().NoError(err, "Failed to create template sensor")

	_, err = s.WaitForEntity(templateEntityID, 5*time.Second)
	s.Require().NoError(err, "Template sensor did not appear")

	// List template config entries only
	entries, err := s.Client().GetConfigEntries(s.Context(), "template")
	s.Require().NoError(err, "Failed to list template config entries")
	s.NotEmpty(entries, "Should have at least one template config entry")

	// Verify all returned entries are from template domain
	for _, entry := range entries {
		s.Equal("template", entry.Domain, "Filtered entries should all be from template domain")
	}

	// Cleanup
	_ = s.Client().DeleteHelper(s.Context(), templateEntityID)
}

func (s *ConfigEntriesIntegrationTestSuite) TestGetConfigEntry() {
	// Create a template sensor to have a known config entry
	templateName := GenerateTestID("cfg_entry_get")
	templateEntityID := BuildEntityID("sensor", templateName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), templateEntityID)
	})

	// Create template sensor
	templateConfig := homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"name":  templateName,
			"state": "{{ 42 }}",
		},
	}

	err := s.Client().CreateHelper(s.Context(), templateConfig)
	s.Require().NoError(err, "Failed to create template sensor")

	_, err = s.WaitForEntity(templateEntityID, 5*time.Second)
	s.Require().NoError(err, "Template sensor did not appear")

	// Get entity registry to find config_entry_id
	registry, err := s.Client().GetEntityRegistry(s.Context())
	s.Require().NoError(err, "Failed to get entity registry")

	var configEntryID string
	for _, entry := range registry {
		if entry.EntityID == templateEntityID {
			configEntryID = entry.ConfigEntryID
			break
		}
	}
	s.Require().NotEmpty(configEntryID, "Template entity should have a config_entry_id")

	// Get the config entry
	configEntry, err := s.Client().GetConfigEntry(s.Context(), configEntryID)
	s.Require().NoError(err, "Failed to get config entry")
	s.NotNil(configEntry, "Config entry should not be nil")

	// Verify config entry fields
	s.Equal(configEntryID, configEntry.EntryID, "Entry ID should match")
	s.Equal("template", configEntry.Domain, "Domain should be template")
	s.NotEmpty(configEntry.Title, "Title should not be empty")
	s.Equal("loaded", configEntry.State, "State should be loaded")
	s.True(configEntry.SupportsOptions, "Template entries support options")
	s.True(configEntry.SupportsUnload, "Template entries support unload")

	// Note: The Options field is not populated by the WebSocket API.
	// Template definitions are stored in the config entry storage but
	// are not exposed through config_entries/get_single.

	// Cleanup
	_ = s.Client().DeleteHelper(s.Context(), templateEntityID)
}

func (s *ConfigEntriesIntegrationTestSuite) TestGetConfigEntryNotFound() {
	// Try to get a non-existent config entry
	_, err := s.Client().GetConfigEntry(s.Context(), "nonexistent_entry_id_12345")
	s.Error(err, "Getting non-existent config entry should return an error")
}

func (s *ConfigEntriesIntegrationTestSuite) TestConfigEntryOptionsDiscovery() {
	// Phase 1: Diagnostic test to verify API behavior for config entry options
	templateName := GenerateTestID("cfg_opts_discovery")
	templateEntityID := BuildEntityID("sensor", templateName)
	knownTemplate := "{{ states('sensor.test') | float }}"

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), templateEntityID)
	})

	// Create template sensor with known Jinja2 template
	templateConfig := homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"name":  templateName,
			"state": knownTemplate,
		},
	}

	err := s.Client().CreateHelper(s.Context(), templateConfig)
	s.Require().NoError(err, "Failed to create template sensor")

	_, err = s.WaitForEntity(templateEntityID, 5*time.Second)
	s.Require().NoError(err, "Template sensor did not appear")

	// Get entity registry to find config_entry_id
	registry, err := s.Client().GetEntityRegistry(s.Context())
	s.Require().NoError(err, "Failed to get entity registry")

	var configEntryID string
	for _, entry := range registry {
		if entry.EntityID == templateEntityID {
			configEntryID = entry.ConfigEntryID
			break
		}
	}
	s.Require().NotEmpty(configEntryID, "Template entity should have a config_entry_id")

	// Test 1: Check if WebSocket API returns options
	configEntry, err := s.Client().GetConfigEntry(s.Context(), configEntryID)
	s.Require().NoError(err, "Failed to get config entry")

	s.T().Logf("Config Entry Options from WS API: %v", configEntry.Options)
	if configEntry.Options == nil {
		s.T().Log("DIAGNOSTIC: WebSocket API returns nil Options (expected)")
	} else if len(configEntry.Options) == 0 {
		s.T().Log("DIAGNOSTIC: WebSocket API returns empty Options map")
	} else {
		s.T().Logf("DIAGNOSTIC: WebSocket API returns Options: %+v", configEntry.Options)
	}

	// Test 2: Try Options Flow REST API
	s.T().Log("DIAGNOSTIC: Testing Options Flow REST API")
	options, err := s.Client().GetConfigEntryOptions(s.Context(), configEntryID)
	s.Require().NoError(err, "Failed to get config entry options")
	s.T().Logf("DIAGNOSTIC: Options Flow API returned %d options", len(options))

	if stateTemplate, ok := options["state"].(string); ok {
		s.T().Logf("DIAGNOSTIC: State template found: %s", stateTemplate)
		s.Equal(knownTemplate, stateTemplate, "State template should match known template")
	} else {
		s.T().Log("DIAGNOSTIC: State template not found in options")
	}

	s.T().Logf("DIAGNOSTIC: Full options: %+v", options)
}

func (s *ConfigEntriesIntegrationTestSuite) TestGetConfigEntryOptionsViaFlow() {
	// Phase 3: Integration test for Options Flow
	templateName := GenerateTestID("cfg_opts_flow_test")
	templateEntityID := BuildEntityID("sensor", templateName)
	knownTemplate := "{{ 42 | float }}"
	knownUnit := "count"

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), templateEntityID)
	})

	// Create template sensor with known Jinja2 template
	templateConfig := homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"name":                templateName,
			"state":               knownTemplate,
			"unit_of_measurement": knownUnit,
		},
	}

	err := s.Client().CreateHelper(s.Context(), templateConfig)
	s.Require().NoError(err, "Failed to create template sensor")

	_, err = s.WaitForEntity(templateEntityID, 5*time.Second)
	s.Require().NoError(err, "Template sensor did not appear")

	// Get entity registry to find config_entry_id
	registry, err := s.Client().GetEntityRegistry(s.Context())
	s.Require().NoError(err, "Failed to get entity registry")

	var configEntryID string
	for _, entry := range registry {
		if entry.EntityID == templateEntityID {
			configEntryID = entry.ConfigEntryID
			break
		}
	}
	s.Require().NotEmpty(configEntryID, "Template entity should have a config_entry_id")

	// Call GetConfigEntryOptions via the new method
	options, err := s.Client().GetConfigEntryOptions(s.Context(), configEntryID)
	s.Require().NoError(err, "Failed to get config entry options")
	s.NotNil(options, "Options should not be nil")
	s.NotEmpty(options, "Options should not be empty")

	// Assert state option matches the created template
	stateTemplate, ok := options["state"].(string)
	s.Require().True(ok, "State option should be a string")
	s.Equal(knownTemplate, stateTemplate, "State template should match the created template")

	// Assert unit_of_measurement is present
	unit, ok := options["unit_of_measurement"].(string)
	s.Require().True(ok, "Unit of measurement should be a string")
	s.Equal(knownUnit, unit, "Unit should match the created unit")
}

// TestDeleteConfigEntryViaTool exercises the manage_config_entry delete action
// through the real handler dispatch (not just the bare client method, which
// the other tests in this suite already cover indirectly via DeleteHelper).
// A template sensor's config entry is used as the target: template helpers
// are Config Entry Flow platforms, so deleting the entry is the same
// operation Home Assistant performs when a user removes an integration.
func (s *ConfigEntriesIntegrationTestSuite) TestDeleteConfigEntryViaTool() {
	templateName := GenerateTestID("cfg_entry_delete")
	templateEntityID := BuildEntityID("sensor", templateName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), templateEntityID)
	})

	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"name":  templateName,
			"state": "{{ 42 }}",
		},
	})
	s.Require().NoError(err, "Failed to create template sensor")

	_, err = s.WaitForEntity(templateEntityID, 5*time.Second)
	s.Require().NoError(err, "Template sensor did not appear")

	registry, err := s.Client().GetEntityRegistry(s.Context())
	s.Require().NoError(err, "Failed to get entity registry")

	var configEntryID string
	for _, entry := range registry {
		if entry.EntityID == templateEntityID {
			configEntryID = entry.ConfigEntryID
			break
		}
	}
	s.Require().NotEmpty(configEntryID, "Template entity should have a config_entry_id")

	// The action under test: delete via the real manage_config_entry tool.
	result := s.CallTool("manage_config_entry", map[string]any{
		"action":   "delete",
		"entry_id": configEntryID,
	})
	s.Require().False(result.IsError, "manage_config_entry delete should succeed, got: %s", resultText(result))
	s.Contains(resultText(result), configEntryID, "success message should name the deleted entry_id")

	err = s.WaitForEntityGone(templateEntityID, 5*time.Second)
	s.Require().NoError(err, "Template sensor entity should be gone after config entry deletion")

	_, err = s.Client().GetConfigEntry(s.Context(), configEntryID)
	s.Error(err, "Config entry should no longer be retrievable after deletion")
}
