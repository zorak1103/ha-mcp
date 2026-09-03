//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// HelperSourceDomainToolDispatchTestSuite verifies manage_helper's
// source-entity domain/device_class preflight (checkSourceEntityDomain,
// helpers_consolidated.go) against a real Home Assistant instance. That
// preflight is a handler-level feature reached only via the manage_helper
// tool dispatch (handleCreate/handleUpdate) - unlike the other integration
// suites in this package, which call s.Client().CreateHelper directly and
// so never exercise it (see CLAUDE.md's "Integration test scope"), every
// helper under test here is created through s.CallTool("manage_helper", ...).
//
// This suite exists because an adversarial review found the preflight's
// domain allowlist for statistics/trend/generic_thermostat/generic_hygrostat
// was narrower than Home Assistant's own EntitySelector - a unit test that
// asserts the allowlist against a hand-copied mirror of itself can never
// catch that; only a live create against real HA can.
type HelperSourceDomainToolDispatchTestSuite struct {
	HelperTestSuite
}

func TestHelperSourceDomainIntegration(t *testing.T) {
	suite.Run(t, new(HelperSourceDomainToolDispatchTestSuite))
}

// createTemplateBinarySensor wraps an input_boolean as a binary_sensor via
// the template platform, for use as a statistics source.
func (s *HelperSourceDomainToolDispatchTestSuite) createTemplateBinarySensor(prefix string) (string, string) {
	boolName := GenerateTestID(prefix + "_bool")
	boolEntityID := BuildEntityID("input_boolean", boolName)

	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "input_boolean",
		Config:   map[string]any{"name": boolName, "initial": true},
	})
	s.Require().NoError(err, "failed to create input_boolean")
	_, err = s.WaitForEntity(boolEntityID, 5*time.Second)
	s.Require().NoError(err, "input_boolean did not appear")

	sensorName := GenerateTestID(prefix + "_bsensor")
	sensorEntityID := BuildEntityID("binary_sensor", sensorName)

	err = s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"name":  sensorName,
			"state": "{{ states('" + boolEntityID + "') == 'on' }}",
			"type":  "binary_sensor",
		},
	})
	s.Require().NoError(err, "failed to create template binary_sensor")
	_, err = s.WaitForEntity(sensorEntityID, 5*time.Second)
	s.Require().NoError(err, "template binary_sensor did not appear")

	return boolEntityID, sensorEntityID
}

// createTemplateFan wraps an input_boolean as a fan via the template
// platform, for use as a generic_thermostat heater.
func (s *HelperSourceDomainToolDispatchTestSuite) createTemplateFan(prefix string) (string, string) {
	boolName := GenerateTestID(prefix + "_bool")
	boolEntityID := BuildEntityID("input_boolean", boolName)

	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "input_boolean",
		Config:   map[string]any{"name": boolName, "initial": false},
	})
	s.Require().NoError(err, "failed to create input_boolean")
	_, err = s.WaitForEntity(boolEntityID, 5*time.Second)
	s.Require().NoError(err, "input_boolean did not appear")

	fanName := GenerateTestID(prefix + "_fan")
	fanEntityID := BuildEntityID("fan", fanName)

	err = s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"name":     fanName,
			"state":    "{{ states('" + boolEntityID + "') == 'on' }}",
			"turn_on":  map[string]any{"service": "input_boolean.turn_on", "data": map[string]any{"entity_id": boolEntityID}},
			"turn_off": map[string]any{"service": "input_boolean.turn_off", "data": map[string]any{"entity_id": boolEntityID}},
			"type":     "fan",
		},
	})
	s.Require().NoError(err, "failed to create template fan")
	_, err = s.WaitForEntity(fanEntityID, 5*time.Second)
	s.Require().NoError(err, "template fan did not appear")

	return boolEntityID, fanEntityID
}

// deviceClassUnits maps a sensor device_class to a unit Home Assistant accepts
// for it. HA's template config_flow _validate_unit() rejects a template sensor
// that sets device_class without a matching unit_of_measurement.
var deviceClassUnits = map[string]string{
	"temperature": "°C",
}

// createTemplateSensor wraps an input_number as a sensor via the template
// platform, optionally carrying deviceClass, for use as a generic_thermostat
// target_sensor_entity_id.
func (s *HelperSourceDomainToolDispatchTestSuite) createTemplateSensor(prefix string, deviceClass string) (string, string) {
	inputName := GenerateTestID(prefix + "_input")
	inputEntityID := BuildEntityID("input_number", inputName)

	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "input_number",
		Config:   map[string]any{"name": inputName, "min": 0.0, "max": 50.0, "initial": 20.0},
	})
	s.Require().NoError(err, "failed to create input_number")
	_, err = s.WaitForEntity(inputEntityID, 5*time.Second)
	s.Require().NoError(err, "input_number did not appear")

	sensorName := GenerateTestID(prefix + "_sensor")
	sensorEntityID := BuildEntityID("sensor", sensorName)

	config := map[string]any{
		"name":  sensorName,
		"state": "{{ states('" + inputEntityID + "') | float }}",
	}
	if deviceClass != "" {
		config["device_class"] = deviceClass
		unit, ok := deviceClassUnits[deviceClass]
		s.Require().True(ok, "no unit_of_measurement mapped for device_class %q", deviceClass)
		config["unit_of_measurement"] = unit
	}

	err = s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{Platform: "template", Config: config})
	s.Require().NoError(err, "failed to create template sensor")
	_, err = s.WaitForEntity(sensorEntityID, 5*time.Second)
	s.Require().NoError(err, "template sensor did not appear")

	return inputEntityID, sensorEntityID
}

// TestStatisticsOverBinarySensorSource proves the widened statistics domain
// allowlist (sensor, binary_sensor) matches Home Assistant's own
// EntitySelector (domain=[BINARY_SENSOR_DOMAIN, SENSOR_DOMAIN]) - before the
// fix, this call was rejected client-side even though HA has always accepted
// it, and count_on/count_off exist specifically for binary_sensor sources.
func (s *HelperSourceDomainToolDispatchTestSuite) TestStatisticsOverBinarySensorSource() {
	boolEntityID, sourceEntityID := s.createTemplateBinarySensor("stat_bin")
	statName := GenerateTestID("stat_bin_stats")
	statEntityID := BuildEntityID("sensor", statName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), statEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
		_ = s.Client().DeleteHelper(s.Context(), boolEntityID)
	})

	result := s.CallTool("manage_helper", map[string]any{
		"action":               "create",
		"type":                 "statistics",
		"id":                   statName,
		"name":                 statName,
		"entity_id":            sourceEntityID,
		"state_characteristic": "count_on",
	})
	s.Require().False(result.IsError, "statistics over a binary_sensor source should be accepted, got: %s", resultText(result))

	_, err := s.WaitForEntity(statEntityID, 5*time.Second)
	s.Require().NoError(err, "statistics sensor did not appear")
}

// TestTrendOverCounterSource proves the widened trend domain allowlist
// (sensor, counter) matches Home Assistant's own EntitySelector
// (ALLOWED_DOMAINS = [COUNTER_DOMAIN, SENSOR_DOMAIN]). A counter is a
// first-class WS helper entity, so no wrapper is needed - it is itself the
// source.
func (s *HelperSourceDomainToolDispatchTestSuite) TestTrendOverCounterSource() {
	counterName := GenerateTestID("trend_counter")
	counterEntityID := BuildEntityID("counter", counterName)

	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "counter",
		Config:   map[string]any{"name": counterName, "initial": 0.0},
	})
	s.Require().NoError(err, "failed to create counter")
	_, err = s.WaitForEntity(counterEntityID, 5*time.Second)
	s.Require().NoError(err, "counter did not appear")

	trendName := GenerateTestID("trend_over_counter")
	trendEntityID := BuildEntityID("binary_sensor", trendName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), trendEntityID)
		_ = s.Client().DeleteHelper(s.Context(), counterEntityID)
	})

	result := s.CallTool("manage_helper", map[string]any{
		"action":    "create",
		"type":      "trend",
		"id":        trendName,
		"name":      trendName,
		"entity_id": counterEntityID,
	})
	s.Require().False(result.IsError, "trend over a counter source should be accepted, got: %s", resultText(result))

	_, err = s.WaitForEntity(trendEntityID, 5*time.Second)
	s.Require().NoError(err, "trend binary_sensor did not appear")
}

// TestGenericThermostatWithFanHeater proves the widened generic_thermostat
// heater_entity_id allowlist (switch, fan) matches Home Assistant's own
// EntitySelector (domain=[fan.DOMAIN, switch.DOMAIN]) - a common case for AC
// units modeled as fan entities. The target sensor still requires
// device_class=temperature (Task 2), so it is created with that set.
func (s *HelperSourceDomainToolDispatchTestSuite) TestGenericThermostatWithFanHeater() {
	fanBoolEntityID, fanEntityID := s.createTemplateFan("thermo_fan")
	sensorInputEntityID, sensorEntityID := s.createTemplateSensor("thermo_fan_sensor", "temperature")

	thermoName := GenerateTestID("thermo_fan_heater")
	thermoEntityID := BuildEntityID("climate", thermoName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), thermoEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sensorEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sensorInputEntityID)
		_ = s.Client().DeleteHelper(s.Context(), fanEntityID)
		_ = s.Client().DeleteHelper(s.Context(), fanBoolEntityID)
	})

	result := s.CallTool("manage_helper", map[string]any{
		"action":                  "create",
		"type":                    "generic_thermostat",
		"id":                      thermoName,
		"name":                    thermoName,
		"heater_entity_id":        fanEntityID,
		"target_sensor_entity_id": sensorEntityID,
		"ac_mode":                 true,
	})
	s.Require().False(result.IsError, "generic_thermostat with a fan heater should be accepted, got: %s", resultText(result))

	_, err := s.WaitForEntity(thermoEntityID, 5*time.Second)
	s.Require().NoError(err, "generic_thermostat did not appear")
}

// TestGenericThermostatRejectsWrongDeviceClassTargetSensor is the negative
// counterpart to Task 2: a sensor.* target that lacks device_class
// "temperature" must be rejected by the preflight with an actionable
// message, rather than reaching HA's opaque config-flow error.
func (s *HelperSourceDomainToolDispatchTestSuite) TestGenericThermostatRejectsWrongDeviceClassTargetSensor() {
	boolEntityID, heaterEntityID := s.createTemplateFan("thermo_dc_reject")
	sensorInputEntityID, sensorEntityID := s.createTemplateSensor("thermo_dc_reject", "")

	thermoName := GenerateTestID("thermo_dc_reject")
	thermoEntityID := BuildEntityID("climate", thermoName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), thermoEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sensorEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sensorInputEntityID)
		_ = s.Client().DeleteHelper(s.Context(), heaterEntityID)
		_ = s.Client().DeleteHelper(s.Context(), boolEntityID)
	})

	result := s.CallTool("manage_helper", map[string]any{
		"action":                  "create",
		"type":                    "generic_thermostat",
		"id":                      thermoName,
		"name":                    thermoName,
		"heater_entity_id":        heaterEntityID,
		"target_sensor_entity_id": sensorEntityID,
		"ac_mode":                 true,
	})
	s.Require().True(result.IsError, "generic_thermostat with a non-temperature target sensor should be rejected")
	text := resultText(result)
	s.Contains(text, "device_class", "error should mention device_class")
	s.Contains(text, "temperature", "error should mention the required device_class")

	err := s.WaitForEntityGone(thermoEntityID, 2*time.Second)
	s.Require().NoError(err, "generic_thermostat should not have been created")
}
