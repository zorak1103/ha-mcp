// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/zorak1103/ha-mcp/internal/handlers/formatter"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// findReferencesTypes are the valid values for the "types" filter parameter,
// matching the "type" field of a ConfigHit returned by the scanners.
var findReferencesTypes = []string{"automation", "script", "scene", "dashboard", "helper_template"}

// schemaTypeObject avoids adding one more raw "object" literal to the codebase's
// already-at-threshold goconst count for the JSON Schema "object" type keyword
// (see .golangci.yml goconst min-occurrences).
const schemaTypeObject = "object"

// FindReferencesHandlers provides the find_references MCP tool: a server-side
// cross-config search across automations, scripts, scenes, dashboards, and
// template-helper templates - issue #141.
type FindReferencesHandlers struct{}

// NewFindReferencesHandlers creates a new FindReferencesHandlers instance.
func NewFindReferencesHandlers() *FindReferencesHandlers {
	return &FindReferencesHandlers{}
}

// RegisterTools registers the find_references tool with the registry.
func (h *FindReferencesHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.findReferencesTool(), h.handleFindReferences)
}

func (h *FindReferencesHandlers) findReferencesTool() mcp.Tool {
	return mcp.Tool{
		Name: "find_references",
		Description: `Search for a string or entity_id across automations, scripts, scenes, dashboards, and ` +
			`template-helper templates in one call - the server-side equivalent of grepping every config type ` +
			`at once instead of listing/getting each object individually and scanning client-side (issue #141).

Returns matches grouped by type with the object id, JSON path (where applicable), context, and a snippet.`,
		InputSchema: mcp.JSONSchema{
			Type:        schemaTypeObject,
			Description: "Parameters for a cross-config reference search",
			Properties: map[string]mcp.JSONSchema{
				"search": {
					Type:        "string",
					Description: "String or entity_id to search for",
				},
				"match_mode": {
					Type: "string",
					Description: "'substring' (default) matches search as a substring anywhere, including inside " +
						"Jinja templates. 'exact' matches only leaf values equal to search.",
					Enum: []string{"substring", "exact"},
				},
				"types": {
					Type:        "array",
					Description: "Config types to search. Default: all of automation, script, scene, dashboard, helper_template.",
					Items:       &mcp.JSONSchema{Type: "string", Enum: findReferencesTypes},
				},
				"format": {
					Type:        "string",
					Description: "Output format: 'natural' (default, human-readable) or 'json' (structured data)",
					Enum:        []string{"natural", formatJSON},
				},
			},
			Required: []string{"search"},
		},
	}
}

// findReferencesResult is the format=json payload shape: hits plus which
// sources were actually scanned successfully, so a "no references found"
// result can be trusted (or explicitly not, via failed_sources).
type findReferencesResult struct {
	Hits           []ConfigHit `json:"hits"`
	ScannedSources []string    `json:"scanned_sources"`
	FailedSources  []string    `json:"failed_sources,omitempty"`
}

func (h *FindReferencesHandlers) handleFindReferences(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	search, _ := args["search"].(string)
	if search == "" {
		return errorResult("search is required"), nil
	}

	matchMode, _ := args["match_mode"].(string)
	match := searchMatchFunc(matchMode, search)

	types := findReferencesRequestedTypes(args)
	hits, scanned, failed := runRequestedScanners(ctx, client, types, match)

	format := formatter.ParseFormat(getStringArg(args, "format"))
	if format == formatter.FormatNatural {
		return successResult(formatFindReferencesNatural(search, hits, failed)), nil
	}

	result := findReferencesResult{Hits: hits, ScannedSources: scanned, FailedSources: failed}
	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return errorResult(fmt.Sprintf("error formatting results: %v", err)), nil
	}
	return successResult(string(output)), nil
}

// scannerCall pairs a source name with the scan function to run for it.
type scannerCall struct {
	source string
	run    func() ([]ConfigHit, error)
}

// buildScannerCalls returns one scannerCall per requested type, in
// findReferencesTypes order (automation, script, scene, dashboard,
// helper_template) - this fixed order is what keeps runRequestedScanners'
// output deterministic regardless of which goroutine finishes first.
func buildScannerCalls(ctx context.Context, client homeassistant.Client, types map[string]bool, match func(string) bool) []scannerCall {
	var calls []scannerCall
	if types["automation"] {
		calls = append(calls, scannerCall{"automation", func() ([]ConfigHit, error) { return scanAutomationsForReferences(ctx, client, match) }})
	}
	if types["script"] {
		calls = append(calls, scannerCall{"script", func() ([]ConfigHit, error) { return scanScriptsForReferences(ctx, client, match) }})
	}
	if types["scene"] {
		calls = append(calls, scannerCall{"scene", func() ([]ConfigHit, error) { return scanScenesForReferences(ctx, client, match) }})
	}
	if types["dashboard"] {
		calls = append(calls, scannerCall{"dashboard", func() ([]ConfigHit, error) { return scanAllDashboardsForReferences(ctx, client, match) }})
	}
	if types["helper_template"] {
		calls = append(calls, scannerCall{"helper_template", func() ([]ConfigHit, error) { return scanHelperTemplates(ctx, client, match) }})
	}
	return calls
}

// runRequestedScanners runs every scanner named in types concurrently and
// aggregates their hits plus which sources succeeded/failed. Each goroutine
// writes only to its own index in results, so no locking is needed; hits are
// concatenated in calls order (not completion order) afterward, keeping
// output deterministic regardless of which scan finishes first.
func runRequestedScanners(ctx context.Context, client homeassistant.Client, types map[string]bool, match func(string) bool) (hits []ConfigHit, scanned, failed []string) {
	calls := buildScannerCalls(ctx, client, types, match)

	results := make([]struct {
		hits []ConfigHit
		err  error
	}, len(calls))

	var wg sync.WaitGroup
	for i, c := range calls {
		wg.Add(1)
		go func(i int, run func() ([]ConfigHit, error)) {
			defer wg.Done()
			results[i].hits, results[i].err = run()
		}(i, c.run)
	}
	wg.Wait()

	var outcomes []ScanOutcome
	for i, c := range calls {
		hits = append(hits, results[i].hits...)
		outcomes = append(outcomes, ScanOutcome{Source: c.source, Err: results[i].err})
	}

	scanned, failed = splitScanOutcomes(outcomes)
	return hits, scanned, failed
}

// searchMatchFunc builds the leaf-value predicate for the requested match_mode.
// Unknown/empty match_mode defaults to substring - the more useful default for
// "where is this used" searches, since it also catches entity_ids embedded in
// Jinja template text (exact would miss those).
func searchMatchFunc(matchMode, search string) func(string) bool {
	if matchMode == "exact" {
		return func(s string) bool { return s == search }
	}
	return func(s string) bool { return strings.Contains(s, search) }
}

// findReferencesRequestedTypes returns the set of types to search, defaulting
// to all of findReferencesTypes when the "types" argument is absent or empty.
func findReferencesRequestedTypes(args map[string]any) map[string]bool {
	requested, ok := args["types"].([]any)
	if !ok || len(requested) == 0 {
		result := make(map[string]bool, len(findReferencesTypes))
		for _, t := range findReferencesTypes {
			result[t] = true
		}
		return result
	}

	result := make(map[string]bool, len(requested))
	for _, t := range requested {
		if s, ok := t.(string); ok {
			result[s] = true
		}
	}
	return result
}

// scanAutomationsForReferences scans every automation's triggers/conditions/actions.
func scanAutomationsForReferences(ctx context.Context, client homeassistant.Client, match func(string) bool) ([]ConfigHit, error) {
	automations, err := client.ListAutomations(ctx)
	if err != nil {
		return nil, err
	}

	var hits []ConfigHit
	for _, auto := range automations {
		autoID := strings.TrimPrefix(auto.EntityID, "automation.")
		full, err := client.GetAutomation(ctx, autoID)
		if err != nil || full.Config == nil {
			continue
		}
		hits = append(hits, configSectionHits("automation", auto.EntityID, match,
			namedSection{"triggers", full.Config.Triggers},
			namedSection{"conditions", full.Config.Conditions},
			namedSection{"actions", full.Config.Actions},
		)...)
	}
	return hits, nil
}

// scanScriptsForReferences scans every script's sequence.
func scanScriptsForReferences(ctx context.Context, client homeassistant.Client, match func(string) bool) ([]ConfigHit, error) {
	scripts, err := client.ListScripts(ctx)
	if err != nil {
		return nil, err
	}

	var hits []ConfigHit
	for _, script := range scripts {
		// ListScripts returns live entity state only - real Home Assistant does not
		// expose "sequence" as a state attribute, so the full config must be
		// fetched via GetScript (mirrors scanAutomationsForReferences/GetAutomation).
		full, err := client.GetScript(ctx, script.EntityID)
		if err != nil || full.Config == nil {
			continue
		}
		hits = append(hits, configSectionHits("script", script.EntityID, match,
			namedSection{"sequence", full.Config.Sequence},
		)...)
	}
	return hits, nil
}

// scanScenesForReferences scans every scene's flat entity_id list.
func scanScenesForReferences(ctx context.Context, client homeassistant.Client, match func(string) bool) ([]ConfigHit, error) {
	scenes, err := client.ListScenes(ctx)
	if err != nil {
		return nil, err
	}

	var hits []ConfigHit
	for _, scene := range scenes {
		entities, ok := scene.Attributes["entity_id"].([]any)
		if !ok {
			continue
		}
		hits = append(hits, configSectionHits("scene", scene.EntityID, match,
			namedSection{configKeyEntityID, entities},
		)...)
	}
	return hits, nil
}

// scanAllDashboardsForReferences scans every dashboard, including the default one.
func scanAllDashboardsForReferences(ctx context.Context, client homeassistant.Client, match func(string) bool) ([]ConfigHit, error) {
	urlPaths, err := allDashboardURLPaths(ctx, client)
	if err != nil {
		return nil, err
	}

	var hits []ConfigHit
	for _, urlPath := range urlPaths {
		config, err := client.GetLovelaceConfig(ctx, urlPath)
		if err != nil {
			continue
		}
		hits = append(hits, scanDashboardConfig(urlPath, config, match)...)
	}
	return hits, nil
}

// namedSection pairs a top-level config array with the pointer-path segment name it lives under.
type namedSection struct {
	name  string
	items []any
}

// configSectionHits collects match hits across a set of named top-level config
// sections (e.g. an automation's triggers/conditions/actions), tagging each with
// objectType and objectID.
func configSectionHits(objectType, objectID string, match func(string) bool, sections ...namedSection) []ConfigHit {
	var hits []ConfigHit
	for _, sec := range sections {
		for _, p := range collectMatchPaths(sec.items, "/"+sec.name, match) {
			hits = append(hits, ConfigHit{Type: objectType, ObjectID: objectID, Path: p})
		}
	}
	return hits
}

// formatFindReferencesNatural renders hits grouped by type for LLM-friendly
// output, followed by a warning listing any source that could not be scanned
// (so an empty-hits result is never mistaken for a confirmed "not used anywhere").
func formatFindReferencesNatural(search string, hits []ConfigHit, failedSources []string) string {
	var parts []string

	if len(hits) == 0 {
		parts = append(parts, fmt.Sprintf("No references found for %q.", search))
	} else {
		byType := make(map[string][]ConfigHit)
		var order []string
		for _, hit := range hits {
			if _, seen := byType[hit.Type]; !seen {
				order = append(order, hit.Type)
			}
			byType[hit.Type] = append(byType[hit.Type], hit)
		}

		parts = append(parts, fmt.Sprintf("Found %d match(es) for %q:", len(hits), search))
		for _, t := range order {
			parts = append(parts, fmt.Sprintf("\n%s (%d):", t, len(byType[t])))
			for _, hit := range byType[t] {
				line := fmt.Sprintf("  • %s", hit.ObjectID)
				if hit.Path != "" {
					line += " " + hit.Path
				}
				if hit.Context != "" {
					line += " - " + hit.Context
				}
				parts = append(parts, line)
			}
		}
	}

	if len(failedSources) > 0 {
		parts = append(parts, "", fmt.Sprintf(
			scanFailureWarningFormat,
			len(failedSources), strings.Join(failedSources, ", "),
		))
	}

	return strings.Join(parts, "\n")
}
