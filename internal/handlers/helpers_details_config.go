package handlers

import (
	"context"
	"errors"
	"fmt"
	"maps"

	"github.com/zorak1103/ha-mcp/internal/handlers/formatter"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// helperSourceConfig is a helper's stored configuration as Home Assistant
// keeps it: WS storage helpers' "<platform>/list" entry, or a config-entry
// helper's (flat) options. platform is the integration platform that owns it.
type helperSourceConfig struct {
	platform string
	config   map[string]any
}

// fetchHelperSourceConfig reads the stored configuration behind a helper
// entity. It returns (nil, nil) when the entity has no helper-backed stored
// config: a YAML-defined WS helper, an entity missing from the registry, one
// without a config entry, or one whose config entry belongs to a non-helper
// integration. The last case matters: get_details reaches every entity whose
// domain is in helperEntityDomains (sensor, light, ...), and starting an
// arbitrary integration's options flow from a read is not side-effect-free,
// so options are only read for helper platforms (RequiresConfigEntryFlow).
func fetchHelperSourceConfig(ctx context.Context, client homeassistant.Client, entityID, entityDomain string) (*helperSourceConfig, error) {
	if meta, ok := helperTypes[entityDomain]; ok && !homeassistant.RequiresConfigEntryFlow(meta.platform) {
		cfg, err := client.GetHelperConfig(ctx, meta.platform, entityID)
		if errors.Is(err, homeassistant.ErrHelperNotFoundInStorage) {
			return nil, nil //nolint:nilnil // documented contract: (nil, nil) = no helper-backed stored config (YAML-defined)
		}
		if err != nil {
			return nil, err
		}
		return &helperSourceConfig{platform: meta.platform, config: cfg}, nil
	}

	registry, err := client.GetEntityRegistry(ctx)
	if err != nil {
		return nil, err
	}
	var entry *homeassistant.EntityRegistryEntry
	for i := range registry {
		if registry[i].EntityID == entityID {
			entry = &registry[i]
			break
		}
	}
	if entry == nil || entry.ConfigEntryID == "" || !homeassistant.RequiresConfigEntryFlow(entry.Platform) {
		return nil, nil //nolint:nilnil // documented contract: (nil, nil) = not a config-entry helper, nothing to read
	}

	options, err := client.GetConfigEntryOptions(ctx, entry.ConfigEntryID)
	if err != nil {
		return nil, err
	}
	// Cloned so the icon merge below never mutates a map the client may share.
	options = maps.Clone(options)
	if options == nil {
		options = map[string]any{}
	}
	// A config-entry helper's icon lives in the entity registry, not in its
	// options (manage_helper sets it there after the flow completes).
	if _, has := options["icon"]; !has && entry.Icon != "" {
		options["icon"] = entry.Icon
	}
	return &helperSourceConfig{platform: entry.Platform, config: options}, nil
}

// applyTemplateEnrichment renders a template helper's options into details
// with the dedicated template readers and returns the option keys they
// consumed, so the configuration block does not repeat them.
func applyTemplateEnrichment(entityDomain string, options, details map[string]any) map[string]bool {
	if len(options) == 0 {
		return nil
	}
	// templateSubtypeDomains and {sensor, binary_sensor} are disjoint.
	if templateSubtypeDomains[entityDomain] {
		return addTemplateSubtypeOptionsToDetails(entityDomain, options, details)
	}
	if entityDomain == platformSensorEntity || entityDomain == platformBinarySensorEntity {
		return addTemplateOptionsToDetails(options, details)
	}
	return nil
}

// configurationSkipKeys are stored-config keys never shown in the
// configuration block: the storage id and the template subtype selector are
// internal identifiers, not fields update can write.
var configurationSkipKeys = map[string]bool{"id": true, "template_type": true}

// shownAtTopLevel reports whether value is already shown in details under the
// same key (or, for "name", as friendly_name). Comparison is via fmt.Sprint,
// since several detail builders store numbers as strings ("0" vs. 0.0).
func shownAtTopLevel(details map[string]any, key string, value any) bool {
	detailKey := key
	if key == "name" {
		detailKey = "friendly_name"
	}
	existing, ok := details[detailKey]
	if !ok || existing == nil {
		return false
	}
	return fmt.Sprint(existing) == fmt.Sprint(value)
}

// attachConfigurationBlock stores the cfg entries not already shown elsewhere
// under details[formatter.DetailKeyConfiguration], keeping raw HA keys (it is
// its own namespace, so it deliberately bypasses putDetail's renames). Keys in
// consumed or configurationSkipKeys, nil values and values already shown at
// top level are left out; nothing is stored when none remain. Callers run
// moveConfigurationAttributeAside first, so the key is free.
func attachConfigurationBlock(cfg, details map[string]any, consumed map[string]bool) {
	block := map[string]any{}
	for k, v := range cfg {
		if consumed[k] || configurationSkipKeys[k] || v == nil || shownAtTopLevel(details, k, v) {
			continue
		}
		block[k] = v
	}
	if len(block) == 0 {
		return
	}
	details[formatter.DetailKeyConfiguration] = block
}

// moveConfigurationAttributeAside renames a detail literally keyed
// "configuration" (a state attribute of that name) via putDetail's collision
// policy (-> attr_configuration), whether or not a block is attached later.
// Afterwards details[formatter.DetailKeyConfiguration] can only ever be the
// stored-configuration block, which the renderers treat specially.
func moveConfigurationAttributeAside(details map[string]any) {
	existing, ok := details[formatter.DetailKeyConfiguration]
	if !ok {
		return
	}
	putDetail(details, formatter.DetailKeyConfiguration, existing)
	delete(details, formatter.DetailKeyConfiguration)
}

// attachHelperConfiguration fetches the helper's stored configuration and
// attaches it to details. A fetch failure never fails get_details: the
// returned warning (empty on success) is appended to the result instead.
func attachHelperConfiguration(ctx context.Context, client homeassistant.Client, entityID, entityDomain string, details map[string]any) string {
	moveConfigurationAttributeAside(details)
	src, err := fetchHelperSourceConfig(ctx, client, entityID, entityDomain)
	if err != nil {
		return configurationFetchWarning(err)
	}
	if src == nil {
		return ""
	}
	var consumed map[string]bool
	if src.platform == platformTemplate {
		consumed = applyTemplateEnrichment(entityDomain, src.config, details)
	}
	attachConfigurationBlock(src.config, details, consumed)
	return ""
}

// configurationFetchWarning renders a configuration fetch error as a
// single-line, length-bounded warning.
func configurationFetchWarning(err error) string {
	reason := formatter.TruncateRunes(scanFailureLineBreaks.Replace(err.Error()), maxScanErrorReasonChars)
	return "configuration could not be read: " + reason
}

// scheduleWeekdays are the schedule storage keys rendered as the schedule
// section itself.
var scheduleWeekdays = []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}

// attachScheduleConfiguration reads a schedule's storage config (already
// fetched by the caller) into details: every weekday key is consumed, since
// the schedule section renders them (an empty day is simply absent there).
// It returns the warning for a fetch error other than YAML-defined.
func attachScheduleConfiguration(cfg map[string]any, fetchErr error, details map[string]any) string {
	moveConfigurationAttributeAside(details)
	if errors.Is(fetchErr, homeassistant.ErrHelperNotFoundInStorage) {
		return ""
	}
	if fetchErr != nil {
		return configurationFetchWarning(fetchErr)
	}
	consumed := make(map[string]bool, len(scheduleWeekdays))
	for _, day := range scheduleWeekdays {
		consumed[day] = true
	}
	attachConfigurationBlock(cfg, details, consumed)
	return ""
}
