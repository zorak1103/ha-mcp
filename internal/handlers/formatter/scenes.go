package formatter

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// Message constants for scene formatter.
const (
	MsgNoScenesFound = "No scenes found."
)

// SceneInfo represents scene information for list output.
type SceneInfo struct {
	EntityID     string   `json:"entity_id"`
	State        string   `json:"state"`
	FriendlyName string   `json:"friendly_name,omitempty"`
	EntityIDs    []string `json:"entity_ids,omitempty"`
}

// SceneListOptions configures scene list formatting.
type SceneListOptions struct {
	Verbose bool
	Limit   int
}

// SceneFormatter defines the interface for formatting scene responses.
type SceneFormatter interface {
	// FormatList formats a list of scenes.
	FormatList(ctx context.Context, scenes []SceneInfo, opts SceneListOptions) (string, error)

	// FormatDetail formats a single scene with full details.
	FormatDetail(ctx context.Context, scene homeassistant.Entity) (string, error)
}

// NewSceneFormatter creates a new SceneFormatter for the specified format.
func NewSceneFormatter(format Format) SceneFormatter {
	switch format {
	case FormatJSON:
		return NewJSONSceneFormatter()
	case FormatNatural:
		return NewNaturalSceneFormatter()
	default:
		return NewNaturalSceneFormatter()
	}
}

// =============================================================================
// Natural Language Formatter
// =============================================================================

// NaturalSceneFormatter produces human-readable scene output.
type NaturalSceneFormatter struct{}

// NewNaturalSceneFormatter creates a new NaturalSceneFormatter.
func NewNaturalSceneFormatter() *NaturalSceneFormatter {
	return &NaturalSceneFormatter{}
}

// FormatList formats a list of scenes in natural language.
func (f *NaturalSceneFormatter) FormatList(
	_ context.Context,
	scenes []SceneInfo,
	opts SceneListOptions,
) (string, error) {
	if len(scenes) == 0 {
		return MsgNoScenesFound, nil
	}

	var result strings.Builder

	// Summary line
	fmt.Fprintf(&result, "%d scenes\n\n", len(scenes))

	// Domain breakdown
	domainCounts := f.countByDomain(scenes)
	if len(domainCounts) > 0 {
		result.WriteString("By affected domains: ")
		result.WriteString(f.formatDomainCounts(domainCounts))
		result.WriteString("\n\n")
	}

	// Scene list
	result.WriteString("Scenes:\n")
	for _, scene := range scenes {
		f.writeSceneLine(&result, scene, opts.Verbose)
	}

	return strings.TrimSuffix(result.String(), "\n"), nil
}

// FormatDetail formats a single scene with full details.
func (f *NaturalSceneFormatter) FormatDetail(
	_ context.Context,
	scene homeassistant.Entity,
) (string, error) {
	var result strings.Builder

	// Header: Name
	name := scene.EntityID
	if friendlyName, ok := scene.Attributes["friendly_name"].(string); ok && friendlyName != "" {
		name = friendlyName
	}
	fmt.Fprintf(&result, "Scene: %s\n", name)

	// Icon
	if icon, ok := scene.Attributes["icon"].(string); ok && icon != "" {
		fmt.Fprintf(&result, "Icon: %s\n", icon)
	}

	// Entity list
	entityIDs := f.extractEntityIDs(scene)
	if len(entityIDs) > 0 {
		result.WriteString("\n")
		f.writeEntitiesSection(&result, entityIDs)
	}

	return strings.TrimSuffix(result.String(), "\n"), nil
}

// Helper methods for NaturalSceneFormatter

func (f *NaturalSceneFormatter) countByDomain(scenes []SceneInfo) map[string]int {
	counts := make(map[string]int)
	for _, scene := range scenes {
		for _, entityID := range scene.EntityIDs {
			domain := ExtractDomain(entityID)
			if domain != "unknown" {
				counts[domain]++
			}
		}
	}
	return counts
}

func (f *NaturalSceneFormatter) formatDomainCounts(counts map[string]int) string {
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

	var parts []string
	for _, kv := range sorted {
		parts = append(parts, fmt.Sprintf("%s: %d", kv.Key, kv.Value))
	}
	return strings.Join(parts, ", ")
}

func (f *NaturalSceneFormatter) writeSceneLine(result *strings.Builder, scene SceneInfo, verbose bool) {
	name := scene.FriendlyName
	if name == "" {
		name = strings.TrimPrefix(scene.EntityID, "scene.")
	}

	fmt.Fprintf(result, "- %s", name)

	// Add entity count with domain breakdown
	if len(scene.EntityIDs) > 0 {
		domainCounts := make(map[string]int)
		for _, entityID := range scene.EntityIDs {
			domain := ExtractDomain(entityID)
			if domain != "unknown" {
				domainCounts[domain]++
			}
		}

		fmt.Fprintf(result, " - affects %d entities", len(scene.EntityIDs))
		if verbose && len(domainCounts) > 0 {
			var domainParts []string
			for domain, count := range domainCounts {
				domainParts = append(domainParts, fmt.Sprintf("%d %s", count, domain))
			}
			sort.Strings(domainParts)
			fmt.Fprintf(result, " (%s)", strings.Join(domainParts, ", "))
		}
	}

	result.WriteString("\n")
}

func (f *NaturalSceneFormatter) extractEntityIDs(scene homeassistant.Entity) []string {
	if entityIDs, ok := scene.Attributes["entity_id"].([]any); ok {
		result := make([]string, 0, len(entityIDs))
		for _, item := range entityIDs {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func (f *NaturalSceneFormatter) writeEntitiesSection(result *strings.Builder, entityIDs []string) {
	fmt.Fprintf(result, "Entities (%d):\n", len(entityIDs))

	// Group by domain
	domainEntities := make(map[string][]string)
	for _, entityID := range entityIDs {
		domain := ExtractDomain(entityID)
		domainEntities[domain] = append(domainEntities[domain], entityID)
	}

	// Sort domains
	var domains []string
	for domain := range domainEntities {
		domains = append(domains, domain)
	}
	sort.Strings(domains)

	for _, domain := range domains {
		entities := domainEntities[domain]
		fmt.Fprintf(result, "\n%s (%d):\n", domain, len(entities))
		for _, entityID := range entities {
			fmt.Fprintf(result, "- %s\n", entityID)
		}
	}
}

// =============================================================================
// JSON Formatter
// =============================================================================

// JSONSceneFormatter produces JSON output for scene data.
type JSONSceneFormatter struct{}

// NewJSONSceneFormatter creates a new JSONSceneFormatter.
func NewJSONSceneFormatter() *JSONSceneFormatter {
	return &JSONSceneFormatter{}
}

// FormatList formats a list of scenes as JSON.
func (f *JSONSceneFormatter) FormatList(
	_ context.Context,
	scenes []SceneInfo,
	_ SceneListOptions,
) (string, error) {
	if scenes == nil {
		scenes = []SceneInfo{}
	}

	data, err := json.MarshalIndent(scenes, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal scenes: %w", err)
	}
	return string(data), nil
}

// FormatDetail formats a single scene as JSON.
func (f *JSONSceneFormatter) FormatDetail(
	_ context.Context,
	scene homeassistant.Entity,
) (string, error) {
	data, err := json.MarshalIndent(scene, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal scene: %w", err)
	}
	return string(data), nil
}
