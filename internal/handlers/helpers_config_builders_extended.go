// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

// buildUtilityMeterConfig builds configuration for utility_meter helper.
func buildUtilityMeterConfig(config, args map[string]any) error {
	// Required: source
	if source, ok := args["source"].(string); ok {
		config["source"] = source
	}

	// Optional fields
	addOptionalString(config, args, "cycle")
	addOptionalFloat(config, args, "offset")
	addOptionalBool(config, args, "delta_values")
	addOptionalBool(config, args, "net_consumption")
	addOptionalBool(config, args, "periodically_resetting")

	// Tariffs array
	if tariffs, ok := args["tariffs"].([]any); ok {
		config["tariffs"] = convertToStringSlice(tariffs)
	}

	return nil
}

// buildMinMaxConfig builds configuration for min_max helper.
func buildMinMaxConfig(config, args map[string]any) error {
	// Required: entity_ids
	if entityIDs, ok := args["entity_ids"].([]any); ok {
		config["entity_ids"] = convertToStringSlice(entityIDs)
	}

	// Required: min_max_type maps to HA's "type" config field. Kept distinct
	// from the tool's own top-level helper-type selector ("type": "min_max"),
	// which shares the same args map - reading args["type"] here would always
	// see the selector value instead of the caller's intended calculation.
	if minMaxType, ok := args["min_max_type"].(string); ok && minMaxType != "" {
		config["type"] = minMaxType
	}

	// Optional fields
	addOptionalInt(config, args, "round_digits")

	return nil
}

// buildStatisticsConfig builds configuration for statistics helper.
func buildStatisticsConfig(config, args map[string]any) error {
	// Required: entity_id, state_characteristic
	addOptionalString(config, args, "entity_id")
	addOptionalString(config, args, "state_characteristic")

	// Optional fields
	addOptionalInt(config, args, "sampling_size")
	addOptionalString(config, args, "max_age")
	addOptionalFloat(config, args, "percentile")
	addOptionalInt(config, args, "precision")

	return nil
}

// buildTrendConfig builds configuration for trend helper.
func buildTrendConfig(config, args map[string]any) error {
	// Required: entity_id
	addOptionalString(config, args, "entity_id")

	// Optional fields
	addOptionalFloat(config, args, "min_gradient")
	addOptionalInt(config, args, "min_samples")
	addOptionalFloat(config, args, "sample_duration")
	addOptionalInt(config, args, "max_samples")
	addOptionalBool(config, args, "invert")

	return nil
}

// buildRandomSensorConfig builds configuration for random sensor helper.
func buildRandomSensorConfig(config, args map[string]any) error {
	// Set type routing field for menu navigation
	config["type"] = "sensor"

	// Optional fields
	addOptionalFloat(config, args, "minimum")
	addOptionalFloat(config, args, "maximum")

	return nil
}

// buildRandomBinarySensorConfig builds configuration for random binary_sensor helper.
func buildRandomBinarySensorConfig(config, _ map[string]any) error {
	// Set type routing field for menu navigation
	config["type"] = "binary_sensor"

	return nil
}

// buildFilterConfig builds configuration for filter helper.
func buildFilterConfig(config, args map[string]any) error {
	// Required: entity_id, filter (filter type name)
	addOptionalString(config, args, "entity_id")
	addOptionalString(config, args, "filter")

	// Optional: filters array (pass through as-is)
	if filters, ok := args["filters"].([]any); ok {
		config["filters"] = filters
	}

	return nil
}

// buildTodConfig builds configuration for tod (Time of Day) helper.
func buildTodConfig(config, args map[string]any) error {
	// Required: after_time, before_time
	addOptionalString(config, args, "after_time")
	addOptionalString(config, args, "before_time")

	// Optional fields
	addOptionalString(config, args, "after_offset")
	addOptionalString(config, args, "before_offset")

	return nil
}

// buildGenericThermostatConfig builds configuration for generic_thermostat helper.
func buildGenericThermostatConfig(config, args map[string]any) error {
	// Map user-friendly field names to API field names
	// API expects: heater, target_sensor, ac_mode (not heater_entity_id, target_sensor_entity_id)
	addRenamedOptionalString(config, args, "heater_entity_id", "heater")
	addRenamedOptionalString(config, args, "target_sensor_entity_id", "target_sensor")

	// ac_mode is required by API
	if acMode, ok := args["ac_mode"].(bool); ok {
		config["ac_mode"] = acMode
	} else {
		config["ac_mode"] = false // Default to heating mode
	}

	// Optional fields
	addOptionalFloat(config, args, "min_temp")
	addOptionalFloat(config, args, "max_temp")
	addOptionalFloat(config, args, "target_temp")
	addOptionalFloat(config, args, "cold_tolerance")
	addOptionalFloat(config, args, "hot_tolerance")

	return nil
}

// buildSwitchAsXConfig builds configuration for switch_as_x helper.
func buildSwitchAsXConfig(config, args map[string]any) error {
	// Required: entity_id (switch entity)
	// Note: name field should NOT be sent to API
	addOptionalString(config, args, "entity_id")

	// target_domain is routing field for menu navigation (stored but filtered by shouldSkipConfigField)
	if targetDomain, ok := args["target_domain"].(string); ok {
		config["target_domain"] = targetDomain
	}

	// Optional fields
	addOptionalBool(config, args, "invert")

	return nil
}

// buildGenericHygrostatConfig builds configuration for generic_hygrostat helper.
func buildGenericHygrostatConfig(config, args map[string]any) error {
	const deviceClassHumidifierValue = "humidifier"

	// Map user-friendly field names to API field names
	// API expects: humidifier, target_sensor, device_class (not humidifier_entity_id, target_sensor_entity_id)
	addRenamedOptionalString(config, args, "humidifier_entity_id", "humidifier")
	addRenamedOptionalString(config, args, "target_sensor_entity_id", "target_sensor")

	// device_class is required by API
	config["device_class"] = deviceClassHumidifierValue

	// Optional fields
	addOptionalFloat(config, args, "min_humidity")
	addOptionalFloat(config, args, "max_humidity")
	addOptionalFloat(config, args, "target_humidity")
	addOptionalFloat(config, args, "dry_tolerance")
	addOptionalFloat(config, args, "wet_tolerance")

	return nil
}

// addExtendedConfigEntryFields adds extended helper fields to update config.
// This function is called by buildConfigEntryUpdateConfig to handle new helper types.
//
//nolint:funlen // Large number of fields for extended helper types
func addExtendedConfigEntryFields(config, args map[string]any, entityDomain, minMaxPlatform string) {
	// utility_meter fields
	addOptionalString(config, args, "source")
	addOptionalString(config, args, "cycle")
	addOptionalFloat(config, args, "offset")
	addOptionalBool(config, args, "delta_values")
	addOptionalBool(config, args, "net_consumption")
	addOptionalBool(config, args, "periodically_resetting")
	if tariffs, ok := args["tariffs"].([]any); ok {
		config["tariffs"] = convertToStringSlice(tariffs)
	}

	// min_max fields
	if entityIDs, ok := args["entity_ids"].([]any); ok {
		config["entity_ids"] = convertToStringSlice(entityIDs)
	}
	addMinMaxTypeField(config, args, minMaxPlatform)
	addOptionalInt(config, args, "round_digits")

	// statistics fields
	addOptionalString(config, args, "state_characteristic")
	addOptionalInt(config, args, "sampling_size")
	addOptionalString(config, args, "max_age")
	addOptionalFloat(config, args, "percentile")
	addOptionalInt(config, args, "precision")

	// trend fields
	addOptionalFloat(config, args, "min_gradient")
	addOptionalInt(config, args, "min_samples")
	addOptionalFloat(config, args, "sample_duration")
	addOptionalInt(config, args, "max_samples")
	addOptionalBool(config, args, "invert")

	// random fields
	addOptionalFloat(config, args, "minimum")
	addOptionalFloat(config, args, "maximum")

	// filter fields
	if filters, ok := args["filters"].([]any); ok {
		config["filters"] = filters
	}

	// tod fields
	addOptionalString(config, args, "after_time")
	addOptionalString(config, args, "before_time")
	addOptionalString(config, args, "after_offset")
	addOptionalString(config, args, "before_offset")

	// generic_thermostat fields (map to API field names)
	addRenamedOptionalString(config, args, "heater_entity_id", "heater")
	addRenamedOptionalString(config, args, "target_sensor_entity_id", "target_sensor")
	if acMode, ok := args["ac_mode"].(bool); ok {
		config["ac_mode"] = acMode
	}
	addOptionalFloat(config, args, "min_temp")
	addOptionalFloat(config, args, "max_temp")
	addOptionalFloat(config, args, "target_temp")
	addOptionalFloat(config, args, "cold_tolerance")
	addOptionalFloat(config, args, "hot_tolerance")

	// switch_as_x fields
	addOptionalString(config, args, "target_domain")

	// generic_hygrostat fields (map to API field names)
	addRenamedOptionalString(config, args, "humidifier_entity_id", "humidifier")
	addRenamedOptionalString(config, args, "target_sensor_entity_id", "target_sensor")
	// device_class is required for hygrostat - only default it when the
	// helper being updated is actually a generic_hygrostat (its entity domain
	// is "humidifier"), not every config-entry helper type that shares this
	// one-size-fits-all update builder.
	if entityDomain == entityDomainHumidifier {
		const deviceClassHumidifier = "humidifier"
		if _, exists := config["device_class"]; !exists {
			config["device_class"] = deviceClassHumidifier
		}
	}
	addOptionalFloat(config, args, "min_humidity")
	addOptionalFloat(config, args, "max_humidity")
	addOptionalFloat(config, args, "target_humidity")
	addOptionalFloat(config, args, "dry_tolerance")
	addOptionalFloat(config, args, "wet_tolerance")
}

// addMinMaxTypeField writes config["type"] from args["min_max_type"] only
// when minMaxPlatform == platformMinMax - i.e. handleUpdate has already
// verified, via resolveConfigEntryPlatformForMinMaxType, that the entity
// being updated is actually a min_max helper. addExtendedConfigEntryFields
// is a one-size-fits-all update builder shared by every config-entry helper
// type (template, threshold, sensor-domain group, statistics, ...); an
// unconditional write here previously let min_max_type leak into "type" for
// any of them, silently changing e.g. a sensor group's aggregation type
// since HA's group CONF_TYPE enum is a superset of min_max's.
func addMinMaxTypeField(config, args map[string]any, minMaxPlatform string) {
	if minMaxPlatform != platformMinMax {
		return
	}
	if minMaxType, ok := args["min_max_type"].(string); ok && minMaxType != "" {
		config["type"] = minMaxType
	}
}
