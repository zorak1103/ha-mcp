// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// buildExtendedHelperProperties returns schema properties for the 10 new Config Entry helper types.
// These properties are merged into the main manage_helper tool schema.
//
//nolint:funlen // Large schema definition with many properties for extended helper types
func buildExtendedHelperProperties() map[string]mcp.JSONSchema {
	return map[string]mcp.JSONSchema{
		// utility_meter specific fields
		"source": {
			Type:        "string",
			Description: "Source entity for utility_meter, derivative, integral, statistics, trend, or filter helpers",
		},
		"cycle": {
			Type:        "string",
			Description: "Reset cycle for utility_meter: quarter-hourly, hourly, daily, weekly, monthly, bimonthly, quarterly, yearly",
			Enum: []string{
				"quarter-hourly", "hourly", "daily", "weekly", "monthly",
				"bimonthly", "quarterly", "yearly",
			},
		},
		"offset": {
			Type:        "number",
			Description: "Offset in days for utility_meter cycle reset",
		},
		"delta_values": {
			Type:        "boolean",
			Description: "Whether source values are delta values (utility_meter)",
		},
		"net_consumption": {
			Type:        "boolean",
			Description: "Enable net consumption tracking (utility_meter)",
		},
		"periodically_resetting": {
			Type:        "boolean",
			Description: "Whether source meter resets periodically (utility_meter)",
		},
		"tariffs": {
			Type:        "array",
			Description: "List of tariff names for utility_meter",
			Items:       &mcp.JSONSchema{Type: "string"},
		},

		// min_max specific fields
		"entity_ids": {
			Type:        "array",
			Description: "List of entity IDs to aggregate (min_max, required)",
			Items:       &mcp.JSONSchema{Type: "string"},
		},
		"round_digits": {
			Type:        "number",
			Description: "Number of decimal places to round to (min_max)",
		},
		"min_max_type": {
			Type:        "string",
			Description: "Calculation to perform (min_max, required): min, max, mean, median, last, range, sum",
			Enum:        []string{"min", "max", "mean", "median", "last", "range", "sum"},
		},

		// statistics specific fields
		"state_characteristic": {
			Type:        "string",
			Description: "Statistic to calculate (statistics, required): count, mean, median, stdev, variance, total, min, max, value_min, value_max, datetime_newest, datetime_oldest, datetime_value_max, datetime_value_min, change, change_sample, change_second, average_linear, average_step, average_timeless",
			Enum: []string{
				"count", "mean", "median", "stdev", "variance", "total",
				"min", "max", "value_min", "value_max",
				"datetime_newest", "datetime_oldest", "datetime_value_max", "datetime_value_min",
				"change", "change_sample", "change_second",
				"average_linear", "average_step", "average_timeless",
			},
		},
		"sampling_size": {
			Type:        "number",
			Description: "Maximum number of samples to keep (statistics)",
		},
		"max_age": {
			Type:        "string",
			Description: "Maximum age of samples (statistics, e.g., '00:05:00' for 5 minutes)",
		},
		"percentile": {
			Type:        "number",
			Description: "Percentile value (0-100) for percentile characteristic (statistics)",
		},
		"precision": {
			Type:        "number",
			Description: "Number of decimal places (statistics)",
		},

		// trend specific fields
		"min_gradient": {
			Type:        "number",
			Description: "Minimum gradient to detect trend (trend)",
		},
		"min_samples": {
			Type:        "number",
			Description: "Minimum number of samples to calculate trend (trend)",
		},
		"sample_duration": {
			Type:        "number",
			Description: "Duration of sampling window in seconds (trend)",
		},
		"max_samples": {
			Type:        "number",
			Description: "Maximum number of samples to keep (trend)",
		},
		"invert": {
			Type:        "boolean",
			Description: "Invert the trend detection (trend) or switch behavior (switch_as_x)",
		},

		// random specific fields (minimum/maximum already defined in schema)

		// filter specific fields
		"filter": {
			Type:        "string",
			Description: "Filter type (filter, required): outlier, lowpass, time_simple_moving_average, time_throttle, throttle",
		},
		"filters": {
			Type:        "array",
			Description: "List of filters to apply (filter, optional - for multiple filters)",
			Items: &mcp.JSONSchema{
				Type: "object",
			},
		},

		// tod (Time of Day) specific fields
		"after_time": {
			Type:        "string",
			Description: "Start time in HH:MM:SS format (tod, required)",
		},
		"before_time": {
			Type:        "string",
			Description: "End time in HH:MM:SS format (tod, required)",
		},
		"after_offset": {
			Type:        "string",
			Description: "Offset for after_time (tod, e.g., '00:30:00')",
		},
		"before_offset": {
			Type:        "string",
			Description: "Offset for before_time (tod, e.g., '00:30:00')",
		},

		// generic_thermostat specific fields
		"heater_entity_id": {
			Type:        "string",
			Description: "Entity ID of the heater switch (generic_thermostat, required)",
		},
		"target_sensor_entity_id": {
			Type:        "string",
			Description: "Entity ID of the temperature sensor (generic_thermostat, required) or humidity sensor (generic_hygrostat, required)",
		},
		"ac_mode": {
			Type:        "boolean",
			Description: "Air conditioning mode (generic_thermostat)",
		},
		"min_temp": {
			Type:        "number",
			Description: "Minimum temperature setpoint (generic_thermostat)",
		},
		"max_temp": {
			Type:        "number",
			Description: "Maximum temperature setpoint (generic_thermostat)",
		},
		"target_temp": {
			Type:        "number",
			Description: "Initial target temperature (generic_thermostat)",
		},
		"cold_tolerance": {
			Type:        "number",
			Description: "Cold tolerance in degrees (generic_thermostat)",
		},
		"hot_tolerance": {
			Type:        "number",
			Description: "Hot tolerance in degrees (generic_thermostat)",
		},

		// switch_as_x specific fields
		"target_domain": {
			Type:        "string",
			Description: "Target domain for switch_as_x (required): cover, fan, light, lock, siren, valve",
			Enum:        []string{"cover", "fan", "light", "lock", "siren", "valve"},
		},

		// generic_hygrostat specific fields
		"humidifier_entity_id": {
			Type:        "string",
			Description: "Entity ID of the humidifier switch (generic_hygrostat, required)",
		},
		"min_humidity": {
			Type:        "number",
			Description: "Minimum humidity setpoint (generic_hygrostat)",
		},
		"max_humidity": {
			Type:        "number",
			Description: "Maximum humidity setpoint (generic_hygrostat)",
		},
		"target_humidity": {
			Type:        "number",
			Description: "Initial target humidity (generic_hygrostat)",
		},
		"dry_tolerance": {
			Type:        "number",
			Description: "Dry tolerance in percentage points (generic_hygrostat)",
		},
		"wet_tolerance": {
			Type:        "number",
			Description: "Wet tolerance in percentage points (generic_hygrostat)",
		},
	}
}
