package formatter

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// NaturalRegistryFormatter produces human-readable registry output.
type NaturalRegistryFormatter struct{}

// NewNaturalRegistryFormatter creates a new NaturalRegistryFormatter.
func NewNaturalRegistryFormatter() *NaturalRegistryFormatter {
	return &NaturalRegistryFormatter{}
}

// FormatEntityRegistry formats entity registry entries in natural language.
func (f *NaturalRegistryFormatter) FormatEntityRegistry(
	_ context.Context,
	entries []homeassistant.EntityRegistryEntry,
	opts RegistryOptions,
) (string, error) {
	if len(entries) == 0 {
		return MsgNoEntitiesFound, nil
	}

	var result strings.Builder

	// Count enabled vs disabled
	enabledCount := 0
	for _, e := range entries {
		if e.DisabledBy == "" {
			enabledCount++
		}
	}

	// Summary line
	fmt.Fprintf(&result, "Found %d entities (%d enabled).\n\n", len(entries), enabledCount)

	// Domain breakdown
	domainCounts := countEntitiesByDomainNatural(entries, opts.IncludeDisabled)
	if len(domainCounts) > 0 {
		result.WriteString("By Domain:\n")
		writeSortedCounts(&result, domainCounts)
		result.WriteString("\n")
	}

	// Verbose: list entries
	if opts.Verbose {
		result.WriteString("Entities:\n")
		limit := opts.Limit
		if limit <= 0 {
			limit = len(entries)
		}
		for i, e := range entries {
			if i >= limit {
				fmt.Fprintf(&result, "... and %d more\n", len(entries)-limit)
				break
			}
			if !opts.IncludeDisabled && e.DisabledBy != "" {
				continue
			}
			f.writeEntityEntry(&result, e)
		}
	}

	return strings.TrimSuffix(result.String(), "\n"), nil
}

func (f *NaturalRegistryFormatter) writeEntityEntry(result *strings.Builder, e homeassistant.EntityRegistryEntry) {
	name := e.Name
	if name == "" {
		name = e.EntityID
	}
	fmt.Fprintf(result, "- %s (%s)", name, e.EntityID)

	var details []string
	if e.Platform != "" {
		details = append(details, fmt.Sprintf("platform: %s", e.Platform))
	}
	if e.AreaID != "" {
		details = append(details, fmt.Sprintf("area: %s", e.AreaID))
	}
	if e.DisabledBy != "" {
		details = append(details, "disabled")
	}

	if len(details) > 0 {
		fmt.Fprintf(result, " [%s]", strings.Join(details, ", "))
	}
	result.WriteString("\n")
}

// FormatDeviceRegistry formats device registry entries in natural language.
func (f *NaturalRegistryFormatter) FormatDeviceRegistry(
	_ context.Context,
	entries []homeassistant.DeviceRegistryEntry,
	opts RegistryOptions,
) (string, error) {
	if len(entries) == 0 {
		return "No devices found.", nil
	}

	var result strings.Builder

	// Count enabled vs disabled
	enabledCount := 0
	for _, d := range entries {
		if d.DisabledBy == "" {
			enabledCount++
		}
	}

	// Summary line
	fmt.Fprintf(&result, "Found %d devices", len(entries))
	if enabledCount != len(entries) {
		fmt.Fprintf(&result, " (%d enabled)", enabledCount)
	}
	result.WriteString(".\n\n")

	// Manufacturer breakdown
	mfgCounts := countDevicesByManufacturerNatural(entries, opts.IncludeDisabled)
	if len(mfgCounts) > 0 {
		result.WriteString("By Manufacturer:\n")
		writeSortedCounts(&result, mfgCounts)
		result.WriteString("\n")
	}

	// Verbose OR EntityMap: list entries
	if opts.Verbose || opts.EntityMap != nil {
		result.WriteString("Devices:\n")
		limit := opts.Limit
		if limit <= 0 {
			limit = len(entries)
		}
		for i, d := range entries {
			if i >= limit {
				fmt.Fprintf(&result, "... and %d more\n", len(entries)-limit)
				break
			}
			if !opts.IncludeDisabled && d.DisabledBy != "" {
				continue
			}
			f.writeDeviceEntry(&result, d, opts.EntityMap)
		}
	}

	return strings.TrimSuffix(result.String(), "\n"), nil
}

func (f *NaturalRegistryFormatter) writeDeviceEntry(result *strings.Builder, d homeassistant.DeviceRegistryEntry, entityMap map[string][]EntityInfo) {
	name := d.Name
	if name == "" {
		name = d.ID
	}
	fmt.Fprintf(result, "- %s", name)

	var details []string
	if d.Manufacturer != "" {
		details = append(details, d.Manufacturer)
	}
	if d.Model.String() != "" {
		details = append(details, d.Model.String())
	}
	if d.AreaID != "" {
		details = append(details, fmt.Sprintf("area: %s", d.AreaID))
	}
	if d.DisabledBy != "" {
		details = append(details, "disabled")
	}

	if len(details) > 0 {
		fmt.Fprintf(result, " [%s]", strings.Join(details, ", "))
	}
	result.WriteString("\n")

	// Show entities if EntityMap is provided
	if entityMap != nil {
		if entities, ok := entityMap[d.ID]; ok && len(entities) > 0 {
			fmt.Fprintf(result, "  Entities (%d):\n", len(entities))
			for _, entity := range entities {
				displayName := entity.EntityID
				if entity.FriendlyName != "" {
					displayName = fmt.Sprintf("%s (%s)", entity.FriendlyName, entity.EntityID)
				}
				fmt.Fprintf(result, "  - %s\n", displayName)
			}
		}
	}
}

// FormatAreaRegistry formats area registry entries in natural language.
func (f *NaturalRegistryFormatter) FormatAreaRegistry(
	_ context.Context,
	entries []homeassistant.AreaRegistryEntry,
) (string, error) {
	if len(entries) == 0 {
		return "No areas found.", nil
	}

	var result strings.Builder

	fmt.Fprintf(&result, "Found %d areas:\n", len(entries))

	for _, a := range entries {
		fmt.Fprintf(&result, "- %s (%s)\n", a.Name, a.AreaID)
	}

	return strings.TrimSuffix(result.String(), "\n"), nil
}

// FormatAllRegistries formats a combined summary of all registries.
func (f *NaturalRegistryFormatter) FormatAllRegistries(
	_ context.Context,
	entities []homeassistant.EntityRegistryEntry,
	devices []homeassistant.DeviceRegistryEntry,
	areas []homeassistant.AreaRegistryEntry,
	opts RegistryOptions,
) (string, error) {
	var result strings.Builder

	// Entities section
	result.WriteString("## Entities\n\n")
	enabledEntityCount := 0
	for _, e := range entities {
		if e.DisabledBy == "" {
			enabledEntityCount++
		}
	}
	fmt.Fprintf(&result, "Total: %d", len(entities))
	if !opts.IncludeDisabled {
		fmt.Fprintf(&result, " (enabled: %d)", enabledEntityCount)
	}
	result.WriteString("\n\n")

	domainCounts := countEntitiesByDomainNatural(entities, opts.IncludeDisabled)
	if len(domainCounts) > 0 {
		result.WriteString("By domain:\n")
		writeSortedCounts(&result, domainCounts)
		result.WriteString("\n")
	}

	// Devices section
	result.WriteString("## Devices\n\n")
	enabledDeviceCount := 0
	for _, d := range devices {
		if d.DisabledBy == "" {
			enabledDeviceCount++
		}
	}
	fmt.Fprintf(&result, "Total: %d", len(devices))
	if !opts.IncludeDisabled {
		fmt.Fprintf(&result, " (enabled: %d)", enabledDeviceCount)
	}
	result.WriteString("\n\n")

	mfgCounts := countDevicesByManufacturerNatural(devices, opts.IncludeDisabled)
	if len(mfgCounts) > 0 {
		result.WriteString("By manufacturer:\n")
		writeSortedCounts(&result, mfgCounts)
		result.WriteString("\n")
	}

	// Areas section
	result.WriteString("## Areas\n\n")
	fmt.Fprintf(&result, "Total: %d\n\n", len(areas))
	for _, a := range areas {
		fmt.Fprintf(&result, "- %s (%s)\n", a.Name, a.AreaID)
	}

	return strings.TrimSuffix(result.String(), "\n"), nil
}

// Helper functions for natural formatter

func countEntitiesByDomainNatural(entries []homeassistant.EntityRegistryEntry, includeDisabled bool) map[string]int {
	counts := make(map[string]int)
	for _, e := range entries {
		if !includeDisabled && e.DisabledBy != "" {
			continue
		}
		domain := ExtractDomain(e.EntityID)
		counts[domain]++
	}
	return counts
}

func countDevicesByManufacturerNatural(entries []homeassistant.DeviceRegistryEntry, includeDisabled bool) map[string]int {
	counts := make(map[string]int)
	for _, d := range entries {
		if !includeDisabled && d.DisabledBy != "" {
			continue
		}
		mfg := d.Manufacturer
		if mfg == "" {
			mfg = "(unknown)"
		}
		counts[mfg]++
	}
	return counts
}

func writeSortedCounts(result *strings.Builder, counts map[string]int) {
	// Sort by count descending, then name ascending
	type kv struct {
		Key   string
		Value int
	}
	var sorted []kv
	for k, v := range counts {
		sorted = append(sorted, kv{k, v})
	}
	slices.SortFunc(sorted, func(a, b kv) int {
		return cmp.Or(
			cmp.Compare(b.Value, a.Value),
			cmp.Compare(a.Key, b.Key),
		)
	})

	for _, kv := range sorted {
		fmt.Fprintf(result, "- %s: %d\n", kv.Key, kv.Value)
	}
}

// JSONRegistryFormatter produces JSON output for registry data.
type JSONRegistryFormatter struct{}

// NewJSONRegistryFormatter creates a new JSONRegistryFormatter.
func NewJSONRegistryFormatter() *JSONRegistryFormatter {
	return &JSONRegistryFormatter{}
}

// FormatEntityRegistry formats entity registry entries as JSON.
func (f *JSONRegistryFormatter) FormatEntityRegistry(
	_ context.Context,
	entries []homeassistant.EntityRegistryEntry,
	_ RegistryOptions,
) (string, error) {
	if entries == nil {
		entries = []homeassistant.EntityRegistryEntry{}
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal entity registry: %w", err)
	}
	return string(data), nil
}

// FormatDeviceRegistry formats device registry entries as JSON.
func (f *JSONRegistryFormatter) FormatDeviceRegistry(
	_ context.Context,
	entries []homeassistant.DeviceRegistryEntry,
	opts RegistryOptions,
) (string, error) {
	if entries == nil {
		entries = []homeassistant.DeviceRegistryEntry{}
	}

	// If EntityMap is provided, augment devices with entity information
	if opts.EntityMap != nil {
		type deviceWithEntities struct {
			homeassistant.DeviceRegistryEntry
			Entities []EntityInfo `json:"entities,omitempty"`
		}

		augmented := make([]deviceWithEntities, len(entries))
		for i, entry := range entries {
			augmented[i] = deviceWithEntities{
				DeviceRegistryEntry: entry,
				Entities:            opts.EntityMap[entry.ID],
			}
		}

		data, err := json.MarshalIndent(augmented, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal device registry: %w", err)
		}
		return string(data), nil
	}

	// Default: just marshal the entries
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal device registry: %w", err)
	}
	return string(data), nil
}

// FormatAreaRegistry formats area registry entries as JSON.
func (f *JSONRegistryFormatter) FormatAreaRegistry(
	_ context.Context,
	entries []homeassistant.AreaRegistryEntry,
) (string, error) {
	if entries == nil {
		entries = []homeassistant.AreaRegistryEntry{}
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal area registry: %w", err)
	}
	return string(data), nil
}

// FormatAllRegistries formats all registries as combined JSON.
func (f *JSONRegistryFormatter) FormatAllRegistries(
	_ context.Context,
	entities []homeassistant.EntityRegistryEntry,
	devices []homeassistant.DeviceRegistryEntry,
	areas []homeassistant.AreaRegistryEntry,
	_ RegistryOptions,
) (string, error) {
	if entities == nil {
		entities = []homeassistant.EntityRegistryEntry{}
	}
	if devices == nil {
		devices = []homeassistant.DeviceRegistryEntry{}
	}
	if areas == nil {
		areas = []homeassistant.AreaRegistryEntry{}
	}

	combined := map[string]any{
		"entities": entities,
		"devices":  devices,
		"areas":    areas,
	}

	data, err := json.MarshalIndent(combined, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal combined registry: %w", err)
	}
	return string(data), nil
}
