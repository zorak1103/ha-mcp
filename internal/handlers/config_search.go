// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/jsonpatch"
)

// ConfigHit is a single location where a search predicate matched inside a
// Home Assistant config object (a dashboard card/chip, a template-helper
// template, ...). Used by find_references, analyze_entity's dashboard/helper
// coverage, and manage_dashboard's find action.
type ConfigHit struct {
	Type     string `json:"type"`
	ObjectID string `json:"object_id"`
	Path     string `json:"path,omitempty"`
	Context  string `json:"context,omitempty"`
	Snippet  string `json:"snippet,omitempty"`
}

// collectMatchPaths recursively walks node looking for string leaves that satisfy
// match, returning the RFC 6901 pointer path to each match, rooted at prefix.
// Map keys are visited in sorted order for deterministic output. This is the
// generic core behind collectEntityPaths (exact-match) and the substring-based
// dashboard/free-text scanners below.
func collectMatchPaths(node any, prefix string, match func(string) bool) []string {
	if node == nil {
		return nil
	}
	switch v := node.(type) {
	case string:
		if match(v) {
			return []string{prefix}
		}
		return nil
	case []any:
		var paths []string
		for i, item := range v {
			sub := collectMatchPaths(item, prefix+"/"+strconv.Itoa(i), match)
			paths = append(paths, sub...)
		}
		return paths
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var paths []string
		for _, k := range keys {
			sub := collectMatchPaths(v[k], prefix+"/"+jsonpatch.EscapeSegment(k), match)
			paths = append(paths, sub...)
		}
		return paths
	}
	return nil
}

// scanDashboardConfig walks a dashboard config's views - including nested
// sections/cards/chips at any depth - for string values satisfying match,
// returning one ConfigHit per match with a compact card-context label.
func scanDashboardConfig(urlPath string, config map[string]any, match func(string) bool) []ConfigHit {
	views, _ := config["views"].([]any)
	var hits []ConfigHit
	for i, v := range views {
		prefix := fmt.Sprintf("/views/%d", i)
		for _, p := range collectMatchPaths(v, prefix, match) {
			hits = append(hits, ConfigHit{
				Type:     "dashboard",
				ObjectID: urlPath,
				Path:     p,
				Context:  dashboardPathContext(config, p),
			})
		}
	}
	return hits
}

// dashboardPathContext returns a compact "card: <type> (<entity>)" label for the
// nearest enclosing card/chip with a "type" field, by walking path from config's
// root down to the match and tracking the most recently seen type/entity.
func dashboardPathContext(config map[string]any, path string) string {
	segs, err := jsonpatch.Segments(path)
	if err != nil {
		return ""
	}
	var node any = config
	cardType, entity := "", ""
	for _, seg := range segs {
		switch v := node.(type) {
		case map[string]any:
			if t, ok := v["type"].(string); ok {
				cardType = t
			}
			if e, ok := v["entity"].(string); ok {
				entity = e
			}
			node = v[seg]
		case []any:
			idx, convErr := strconv.Atoi(seg)
			if convErr != nil || idx < 0 || idx >= len(v) {
				return buildCardContext(cardType, entity)
			}
			node = v[idx]
		default:
			return buildCardContext(cardType, entity)
		}
	}
	return buildCardContext(cardType, entity)
}

func buildCardContext(cardType, entity string) string {
	switch {
	case cardType != "" && entity != "":
		return fmt.Sprintf("card: %s (%s)", cardType, entity)
	case cardType != "":
		return "card: " + cardType
	default:
		return ""
	}
}

// scanHelperTemplates scans every template-helper's Jinja templates (state and
// availability) for text satisfying match. Entities that are not template
// helpers, and registry/options-flow lookup failures, are skipped rather than
// treated as errors - a helper without a config_entry_id simply isn't scanned.
func scanHelperTemplates(ctx context.Context, client homeassistant.Client, match func(string) bool) ([]ConfigHit, error) {
	entries, err := client.GetEntityRegistry(ctx)
	if err != nil {
		return nil, err
	}

	var hits []ConfigHit
	for _, entry := range entries {
		if entry.Platform != platformTemplate || entry.ConfigEntryID == "" {
			continue
		}
		options, err := client.GetConfigEntryOptions(ctx, entry.ConfigEntryID)
		if err != nil {
			continue
		}
		hits = append(hits, templateOptionHits(entry.EntityID, options, match)...)
	}
	return hits, nil
}

// scanFailureWarningFormat is the shared natural-language warning appended by
// find_references (this task) and analyze_entity (Task 6) when one or more
// sources could not be scanned - both tools must use identical wording so
// the failure signal reads the same way everywhere. Defined here, in its
// first consumer's task, rather than earlier, so it's never an unused
// package-level const (golangci-lint's `unused` check would fail on that).
const scanFailureWarningFormat = "⚠ %d source(s) could not be scanned: %s — results may be incomplete."

// templateOptionHits checks the state/availability template fields of a single
// template helper's config-entry options against match.
func templateOptionHits(entityID string, options map[string]any, match func(string) bool) []ConfigHit {
	var hits []ConfigHit
	for _, field := range []string{"state", "availability"} {
		tmpl, ok := options[field].(string)
		if !ok || tmpl == "" || !match(tmpl) {
			continue
		}
		hits = append(hits, ConfigHit{
			Type:     "helper_template",
			ObjectID: entityID,
			Context:  field,
			Snippet:  tmpl,
		})
	}
	return hits
}

// ScanOutcome names one source's scan attempt and its error, if any (nil = success).
// Used by analyze_entity and find_references to build a scanned/failed source
// list that reflects what actually happened, not what should have happened.
type ScanOutcome struct {
	Source string
	Err    error
}

// splitScanOutcomes splits a list of per-source scan attempts into the names
// that succeeded and the names that failed, preserving input order in each list.
func splitScanOutcomes(outcomes []ScanOutcome) (scanned, failed []string) {
	for _, o := range outcomes {
		if o.Err != nil {
			failed = append(failed, o.Source)
			continue
		}
		scanned = append(scanned, o.Source)
	}
	return scanned, failed
}

// scanFailures returns only the failed entries from outcomes, preserving order.
func scanFailures(outcomes []ScanOutcome) []ScanOutcome {
	var failed []ScanOutcome
	for _, o := range outcomes {
		if o.Err != nil {
			failed = append(failed, o)
		}
	}
	return failed
}

// formatScanFailureWarning renders the shared "could not be scanned" warning,
// including each failed source's underlying error so a failure is diagnosable
// from the tool's own output without needing server-side logs.
func formatScanFailureWarning(failed []ScanOutcome) string {
	if len(failed) == 0 {
		return ""
	}
	details := make([]string, 0, len(failed))
	for _, o := range failed {
		reason := "unknown error"
		if o.Err != nil {
			reason = strings.ReplaceAll(o.Err.Error(), "\n", " ")
		}
		details = append(details, fmt.Sprintf("%s (%s)", o.Source, reason))
	}
	return fmt.Sprintf(scanFailureWarningFormat, len(failed), strings.Join(details, ", "))
}

// allDashboardURLPaths returns the url_paths to search for "every dashboard"
// scans: "" (the default dashboard) followed by every registered dashboard's
// URLPath. Returns the ListDashboards error, if any, so callers can record a
// failed "dashboards" scan instead of silently searching zero dashboards.
func allDashboardURLPaths(ctx context.Context, client homeassistant.Client) ([]string, error) {
	dashboards, err := client.ListDashboards(ctx)
	if err != nil {
		return nil, err
	}
	urlPaths := make([]string, 0, len(dashboards)+1)
	urlPaths = append(urlPaths, "")
	for _, d := range dashboards {
		urlPaths = append(urlPaths, d.URLPath)
	}
	return urlPaths, nil
}
