//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// DeviceManageIntegrationTestSuite tests manage_device at the API level.
// Devices cannot be created or deleted via API — tests find an existing device,
// apply changes, and restore original values in cleanup.
type DeviceManageIntegrationTestSuite struct {
	IntegrationTestSuite
}

func TestDeviceManageIntegration(t *testing.T) {
	suite.Run(t, new(DeviceManageIntegrationTestSuite))
}

// findAnyDevice returns the first non-disabled device from the registry, or nil
// if the registry is empty.
func (s *DeviceManageIntegrationTestSuite) findAnyDevice() *homeassistant.DeviceRegistryEntry {
	devices, err := s.Client().GetDeviceRegistry(s.Context())
	if err != nil || len(devices) == 0 {
		return nil
	}
	for i := range devices {
		if devices[i].DisabledBy == "" {
			return &devices[i]
		}
	}
	return nil
}

// findDeviceByID returns a device entry from the registry, or nil if not found.
func (s *DeviceManageIntegrationTestSuite) findDeviceByID(deviceID string) *homeassistant.DeviceRegistryEntry {
	devices, err := s.Client().GetDeviceRegistry(s.Context())
	if err != nil {
		return nil
	}
	for i := range devices {
		if devices[i].ID == deviceID {
			return &devices[i]
		}
	}
	return nil
}

// TestDeviceRegistryList verifies that GetDeviceRegistry returns a parseable result.
func (s *DeviceManageIntegrationTestSuite) TestDeviceRegistryList() {
	devices, err := s.Client().GetDeviceRegistry(s.Context())
	s.Require().NoError(err, "GetDeviceRegistry should succeed")
	s.T().Logf("Found %d device(s) in registry", len(devices))

	// Basic structural checks on whatever devices are present
	for _, d := range devices {
		s.NotEmpty(d.ID, "every device entry should have an ID")
	}
}

// TestDeviceUpdateNameByUser sets a custom display name on a device, verifies it,
// then restores the original name.
func (s *DeviceManageIntegrationTestSuite) TestDeviceUpdateNameByUser() {
	device := s.findAnyDevice()
	if device == nil {
		s.T().Skip("no devices available in registry")
	}

	deviceID := device.ID
	originalName := device.NameByUser

	// Restore original name on cleanup
	s.RegisterCleanup(func() {
		_, _ = s.Client().UpdateDeviceRegistryEntry(s.Context(), deviceID,
			homeassistant.DeviceRegistryUpdateConfig{NameByUser: &originalName})
	})

	customName := "mcptest_device_name"
	updated, err := s.Client().UpdateDeviceRegistryEntry(s.Context(), deviceID,
		homeassistant.DeviceRegistryUpdateConfig{NameByUser: &customName})
	s.Require().NoError(err, "UpdateDeviceRegistryEntry should succeed")

	// API may return empty entry — verify via registry if needed
	if updated != nil && updated.ID != "" {
		s.Equal(customName, updated.NameByUser, "NameByUser should be updated")
	} else {
		time.Sleep(500 * time.Millisecond)
		d := s.findDeviceByID(deviceID)
		s.Require().NotNil(d, "device should exist in registry")
		s.Equal(customName, d.NameByUser, "NameByUser should be reflected in registry")
	}

	// Restore
	_, err = s.Client().UpdateDeviceRegistryEntry(s.Context(), deviceID,
		homeassistant.DeviceRegistryUpdateConfig{NameByUser: &originalName})
	s.Require().NoError(err, "restoring original name should succeed")
}

// TestDeviceUpdateAreaAssignment assigns a test area to a device, verifies it,
// then clears the area assignment and deletes the test area.
func (s *DeviceManageIntegrationTestSuite) TestDeviceUpdateAreaAssignment() {
	device := s.findAnyDevice()
	if device == nil {
		s.T().Skip("no devices available in registry")
	}

	deviceID := device.ID
	originalAreaID := device.AreaID

	// Create a test area to assign
	areaName := GenerateTestID("dev_area")
	createdArea, err := s.Client().CreateArea(s.Context(), homeassistant.AreaConfig{Name: areaName})
	s.Require().NoError(err, "failed to create test area")
	testAreaID := createdArea.AreaID

	// Restore original area assignment and delete test area on cleanup
	s.RegisterCleanup(func() {
		_, _ = s.Client().UpdateDeviceRegistryEntry(s.Context(), deviceID,
			homeassistant.DeviceRegistryUpdateConfig{AreaID: &originalAreaID})
		_ = s.Client().DeleteArea(s.Context(), testAreaID)
	})

	time.Sleep(500 * time.Millisecond)

	// Assign test area to device
	updated, err := s.Client().UpdateDeviceRegistryEntry(s.Context(), deviceID,
		homeassistant.DeviceRegistryUpdateConfig{AreaID: &testAreaID})
	s.Require().NoError(err, "assigning area to device should succeed")

	if updated != nil && updated.ID != "" {
		s.Equal(testAreaID, updated.AreaID, "AreaID should be updated")
	} else {
		time.Sleep(500 * time.Millisecond)
		d := s.findDeviceByID(deviceID)
		s.Require().NotNil(d, "device should exist in registry")
		s.Equal(testAreaID, d.AreaID, "AreaID should be reflected in registry")
	}

	// Clear area assignment
	emptyAreaID := ""
	_, err = s.Client().UpdateDeviceRegistryEntry(s.Context(), deviceID,
		homeassistant.DeviceRegistryUpdateConfig{AreaID: &emptyAreaID})
	s.Require().NoError(err, "clearing area assignment should succeed")

	time.Sleep(500 * time.Millisecond)
	d := s.findDeviceByID(deviceID)
	s.Require().NotNil(d)
	s.Empty(d.AreaID, "AreaID should be cleared")

	// Restore original area
	_, err = s.Client().UpdateDeviceRegistryEntry(s.Context(), deviceID,
		homeassistant.DeviceRegistryUpdateConfig{AreaID: &originalAreaID})
	s.Require().NoError(err, "restoring original area should succeed")
	_ = s.Client().DeleteArea(s.Context(), testAreaID)
}

// TestDeviceUpdateLabels sets labels on a device, verifies them, replaces them,
// then restores the original labels.
func (s *DeviceManageIntegrationTestSuite) TestDeviceUpdateLabels() {
	device := s.findAnyDevice()
	if device == nil {
		s.T().Skip("no devices available in registry")
	}

	deviceID := device.ID
	originalLabels := device.Labels

	// Create two test labels to use on the device
	label1ID := s.CreateTestLabel("dev_lbl1")
	label2ID := s.CreateTestLabel("dev_lbl2")

	// Restore original labels on cleanup
	s.RegisterCleanup(func() {
		_, _ = s.Client().UpdateDeviceRegistryEntry(s.Context(), deviceID,
			homeassistant.DeviceRegistryUpdateConfig{Labels: originalLabels})
	})

	time.Sleep(500 * time.Millisecond)

	// Set two labels on the device
	twoLabels := []string{label1ID, label2ID}
	updated, err := s.Client().UpdateDeviceRegistryEntry(s.Context(), deviceID,
		homeassistant.DeviceRegistryUpdateConfig{Labels: twoLabels})
	s.Require().NoError(err, "setting labels should succeed")

	if updated != nil && updated.ID != "" {
		s.ElementsMatch(twoLabels, updated.Labels, "both labels should be set")
	} else {
		time.Sleep(500 * time.Millisecond)
		d := s.findDeviceByID(deviceID)
		s.Require().NotNil(d)
		s.ElementsMatch(twoLabels, d.Labels, "both labels should be reflected in registry")
	}

	// Replace with only one label (replace semantics at API level)
	oneLabel := []string{label2ID}
	_, err = s.Client().UpdateDeviceRegistryEntry(s.Context(), deviceID,
		homeassistant.DeviceRegistryUpdateConfig{Labels: oneLabel})
	s.Require().NoError(err, "replacing labels should succeed")

	time.Sleep(500 * time.Millisecond)
	d := s.findDeviceByID(deviceID)
	s.Require().NotNil(d)
	s.ElementsMatch(oneLabel, d.Labels, "label should be replaced")
	s.NotContains(d.Labels, label1ID, "first label should no longer be present")

	// Restore original labels
	_, err = s.Client().UpdateDeviceRegistryEntry(s.Context(), deviceID,
		homeassistant.DeviceRegistryUpdateConfig{Labels: originalLabels})
	s.Require().NoError(err, "restoring labels should succeed")
}
