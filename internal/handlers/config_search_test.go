package handlers

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// --- collectMatchPaths ---

func TestCollectMatchPaths_ExactMatch(t *testing.T) {
	t.Parallel()

	got := collectMatchPaths("sensor.x", "/prefix", func(s string) bool { return s == "sensor.x" })
	want := []string{"/prefix"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCollectMatchPaths_SubstringMatch(t *testing.T) {
	t.Parallel()

	node := map[string]any{
		"icon_color": "{{ presence_icon_formatter('device_tracker.example_phone') }}",
	}
	got := collectMatchPaths(node, "/chip", func(s string) bool {
		return strings.Contains(s, "device_tracker.example_phone")
	})
	want := []string{"/chip/icon_color"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCollectMatchPaths_NoMatch(t *testing.T) {
	t.Parallel()

	got := collectMatchPaths("sensor.y", "/prefix", func(s string) bool { return s == "sensor.x" })
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

// --- scanDashboardConfig ---

func nestedDashboardConfig() map[string]any {
	return map[string]any{
		"views": []any{
			map[string]any{
				"title": "Home",
				"sections": []any{
					map[string]any{
						"cards": []any{
							map[string]any{
								"type": "vertical-stack",
								"cards": []any{
									map[string]any{
										"type": "tile",
										"chips": []any{
											map[string]any{"type": "entity", "entity": "light.desk"},
											map[string]any{"type": "entity", "entity": "device_tracker.example_phone"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestScanDashboardConfig_FindsDeeplyNestedChip(t *testing.T) {
	t.Parallel()

	config := nestedDashboardConfig()
	match := func(s string) bool { return s == "device_tracker.example_phone" }
	hits := scanDashboardConfig("lovelace", config, match)

	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d: %+v", len(hits), hits)
	}
	want := "/views/0/sections/0/cards/0/cards/0/chips/1/entity"
	if hits[0].Path != want {
		t.Errorf("path = %q, want %q", hits[0].Path, want)
	}
	if hits[0].Type != "dashboard" {
		t.Errorf("type = %q, want %q", hits[0].Type, "dashboard")
	}
	if hits[0].ObjectID != "lovelace" {
		t.Errorf("object_id = %q, want %q", hits[0].ObjectID, "lovelace")
	}
	wantContext := "card: entity (device_tracker.example_phone)"
	if hits[0].Context != wantContext {
		t.Errorf("context = %q, want %q", hits[0].Context, wantContext)
	}
}

func TestScanDashboardConfig_NoMatch(t *testing.T) {
	t.Parallel()

	config := nestedDashboardConfig()
	match := func(s string) bool { return s == "device_tracker.nonexistent" }
	hits := scanDashboardConfig("lovelace", config, match)
	if len(hits) != 0 {
		t.Errorf("expected no hits, got %+v", hits)
	}
}

func TestScanDashboardConfig_SubstringMatchInTemplatedField(t *testing.T) {
	t.Parallel()

	config := map[string]any{
		"views": []any{
			map[string]any{
				"cards": []any{
					map[string]any{
						"type":       "tile",
						"icon_color": "{{ presence_icon_formatter('device_tracker.example_phone') }}",
					},
				},
			},
		},
	}
	match := func(s string) bool { return strings.Contains(s, "device_tracker.example_phone") }
	hits := scanDashboardConfig("lovelace", config, match)
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d: %+v", len(hits), hits)
	}
	if hits[0].Path != "/views/0/cards/0/icon_color" {
		t.Errorf("path = %q", hits[0].Path)
	}
	if hits[0].Context != "card: tile" {
		t.Errorf("context = %q, want %q", hits[0].Context, "card: tile")
	}
}

func TestScanDashboardConfig_MissingViews(t *testing.T) {
	t.Parallel()

	hits := scanDashboardConfig("lovelace", map[string]any{}, func(string) bool { return true })
	if len(hits) != 0 {
		t.Errorf("expected no hits for missing views, got %+v", hits)
	}
}

// --- scanHelperTemplates ---

func TestScanHelperTemplates_MatchesStateTemplate(t *testing.T) {
	t.Parallel()

	mock := &UniversalMockClient{
		GetEntityRegistryFn: func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
			return []homeassistant.EntityRegistryEntry{
				{EntityID: "sensor.occupancy_count", Platform: "template", ConfigEntryID: "entry1"},
				{EntityID: "sensor.other", Platform: "sensor", ConfigEntryID: ""},
			}, nil
		},
		GetConfigEntryOptionsFn: func(_ context.Context, entryID string) (map[string]any, error) {
			if entryID != "entry1" {
				t.Fatalf("unexpected entry id %q", entryID)
			}
			return map[string]any{
				"state": "{{ states.device_tracker | selectattr('entity_id', 'eq', 'device_tracker.example_phone') | list | count }}",
			}, nil
		},
	}

	hits, err := scanHelperTemplates(context.Background(), mock, func(s string) bool {
		return strings.Contains(s, "device_tracker.example_phone")
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d: %+v", len(hits), hits)
	}
	if hits[0].Type != "helper_template" || hits[0].ObjectID != "sensor.occupancy_count" || hits[0].Context != "state" {
		t.Errorf("unexpected hit: %+v", hits[0])
	}
}

func TestScanHelperTemplates_SkipsNonTemplateEntities(t *testing.T) {
	t.Parallel()

	mock := &UniversalMockClient{
		GetEntityRegistryFn: func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
			return []homeassistant.EntityRegistryEntry{
				{EntityID: "sensor.other", Platform: "sensor", ConfigEntryID: "entry1"},
			}, nil
		},
		GetConfigEntryOptionsFn: func(context.Context, string) (map[string]any, error) {
			t.Fatal("should not be called for a non-template entity")
			return nil, nil
		},
	}

	hits, err := scanHelperTemplates(context.Background(), mock, func(string) bool { return true })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("expected no hits, got %+v", hits)
	}
}

func TestScanHelperTemplates_RegistryErrorReturnsError(t *testing.T) {
	t.Parallel()

	mock := &UniversalMockClient{
		GetEntityRegistryFn: func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
			return nil, errors.New("registry unavailable")
		},
	}

	hits, err := scanHelperTemplates(context.Background(), mock, func(string) bool { return true })
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if hits != nil {
		t.Errorf("expected nil hits, got %+v", hits)
	}
}

// --- templateScanFields ---

// TestTemplateScanFields pins the scanned template fields per entity domain.
// The expected lists are derived from templateSubtypes: only tplTemplate
// fields (never actions) under their HA config key, plus availability.
func TestTemplateScanFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		domain string
		want   []string
	}{
		{"sensor", []string{"availability", "state"}},
		{"binary_sensor", []string{"availability", "state"}},
		{"switch", []string{"availability", "value_template"}},
		{"cover", []string{"availability", "position", "state"}},
		{"light", []string{"availability", "hs", "level", "state", "temperature"}},
	}
	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			t.Parallel()

			got := templateScanFields(tt.domain)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("templateScanFields(%q) = %v, want %v", tt.domain, got, tt.want)
			}
		})
	}
}

func TestScanHelperTemplates_MatchesSubtypeTemplateField(t *testing.T) {
	t.Parallel()

	mock := &UniversalMockClient{
		GetEntityRegistryFn: func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
			return []homeassistant.EntityRegistryEntry{
				{EntityID: "switch.tpl", Platform: "template", ConfigEntryID: "ce"},
			}, nil
		},
		GetConfigEntryOptionsFn: func(context.Context, string) (map[string]any, error) {
			return map[string]any{"value_template": "{{ is_state('input_boolean.src','on') }}"}, nil
		},
	}

	hits, err := scanHelperTemplates(context.Background(), mock, func(s string) bool {
		return strings.Contains(s, "input_boolean.src")
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d: %+v", len(hits), hits)
	}
	if hits[0].ObjectID != "switch.tpl" || hits[0].Context != "value_template" {
		t.Errorf("unexpected hit: %+v", hits[0])
	}
}

func TestScanHelperTemplates_OptionsErrorReportedWithPartialHits(t *testing.T) {
	t.Parallel()

	mock := &UniversalMockClient{
		GetEntityRegistryFn: func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
			return []homeassistant.EntityRegistryEntry{
				{EntityID: "sensor.ok", Platform: "template", ConfigEntryID: "ce-ok"},
				{EntityID: "sensor.bad", Platform: "template", ConfigEntryID: "ce-bad"},
			}, nil
		},
		GetConfigEntryOptionsFn: func(_ context.Context, entryID string) (map[string]any, error) {
			if entryID == "ce-bad" {
				return nil, errors.New("flow aborted")
			}
			return map[string]any{"state": "{{ states('sensor.src') }}"}, nil
		},
	}

	hits, err := scanHelperTemplates(context.Background(), mock, func(s string) bool {
		return strings.Contains(s, "sensor.src")
	})
	if err == nil {
		t.Fatal("expected an error for the unreadable helper, got nil")
	}
	if !strings.Contains(err.Error(), "sensor.bad") || !strings.Contains(err.Error(), "flow aborted") {
		t.Errorf("error should name the failing helper and cause, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "1 template helper") {
		t.Errorf("error should report the failure count, got %q", err.Error())
	}
	if len(hits) != 1 || hits[0].ObjectID != "sensor.ok" {
		t.Errorf("hits = %+v, want exactly the sensor.ok hit", hits)
	}
}

// --- splitScanOutcomes ---

func TestSplitScanOutcomes_AllSucceed(t *testing.T) {
	t.Parallel()

	scanned, failed := splitScanOutcomes([]ScanOutcome{
		{Source: "automation", Err: nil},
		{Source: "script", Err: nil},
	})
	if !reflect.DeepEqual(scanned, []string{"automation", "script"}) {
		t.Errorf("scanned = %v, want [automation script]", scanned)
	}
	if len(failed) != 0 {
		t.Errorf("failed = %v, want empty", failed)
	}
}

func TestSplitScanOutcomes_SomeFail(t *testing.T) {
	t.Parallel()

	scanned, failed := splitScanOutcomes([]ScanOutcome{
		{Source: "automation", Err: nil},
		{Source: "dashboard", Err: errors.New("connection failed")},
		{Source: "helper_template", Err: errors.New("registry unavailable")},
	})
	if !reflect.DeepEqual(scanned, []string{"automation"}) {
		t.Errorf("scanned = %v, want [automation]", scanned)
	}
	if !reflect.DeepEqual(failed, []string{"dashboard", "helper_template"}) {
		t.Errorf("failed = %v, want [dashboard helper_template]", failed)
	}
}

func TestSplitScanOutcomes_Empty(t *testing.T) {
	t.Parallel()

	scanned, failed := splitScanOutcomes(nil)
	if len(scanned) != 0 || len(failed) != 0 {
		t.Errorf("expected both empty, got scanned=%v failed=%v", scanned, failed)
	}
}

func TestScanFailures(t *testing.T) {
	t.Parallel()

	outcomes := []ScanOutcome{
		{Source: "automation", Err: nil},
		{Source: "script", Err: errors.New("timeout")},
		{Source: "scene", Err: nil},
		{Source: "dashboard", Err: errors.New("connection reset")},
	}
	failed := scanFailures(outcomes)
	if len(failed) != 2 {
		t.Fatalf("expected 2 failures, got %d", len(failed))
	}
	if failed[0].Source != "script" || failed[0].Err.Error() != "timeout" {
		t.Errorf("unexpected first failure: %+v", failed[0])
	}
	if failed[1].Source != "dashboard" || failed[1].Err.Error() != "connection reset" {
		t.Errorf("unexpected second failure: %+v", failed[1])
	}

	if len(scanFailures(nil)) != 0 {
		t.Errorf("expected empty result for nil input")
	}
}

func TestFormatScanFailureWarning(t *testing.T) {
	t.Parallel()

	if got := formatScanFailureWarning(nil); got != "" {
		t.Errorf("expected empty string for nil input, got %q", got)
	}

	failed := []ScanOutcome{
		{Source: "automation", Err: errors.New("websocket: not connected")},
		{Source: "script", Err: errors.New("multiline\nerror\ndetails")},
	}
	got := formatScanFailureWarning(failed)
	want := "⚠ 2 source(s) could not be scanned: automation (websocket: not connected), script (multiline error details) — results may be incomplete."
	if got != want {
		t.Errorf("formatScanFailureWarning() =\n%q\nwant:\n%q", got, want)
	}

	single := []ScanOutcome{
		{Source: "dashboard", Err: errors.New("timeout")},
	}
	gotSingle := formatScanFailureWarning(single)
	wantSingle := "⚠ 1 source(s) could not be scanned: dashboard (timeout) — results may be incomplete."
	if gotSingle != wantSingle {
		t.Errorf("formatScanFailureWarning() =\n%q\nwant:\n%q", gotSingle, wantSingle)
	}
}

func TestFormatScanFailureWarning_TruncatesLongReasons(t *testing.T) {
	t.Parallel()

	longReason := strings.Repeat("x", 5000)
	failed := []ScanOutcome{{Source: "automation", Err: errors.New(longReason)}}

	got := formatScanFailureWarning(failed)

	if len(got) > 1000 {
		t.Fatalf("expected the reason to be bounded, got a %d-char message", len(got))
	}
	if !strings.Contains(got, "...") {
		t.Errorf("expected a truncation marker, got: %s", got)
	}
}

func TestFormatScanFailureWarning_FlattensCarriageReturns(t *testing.T) {
	t.Parallel()

	failed := []ScanOutcome{{Source: "automation", Err: errors.New("line1\rline2")}}

	got := formatScanFailureWarning(failed)

	if strings.Contains(got, "\r") {
		t.Errorf("expected \\r to be flattened, got: %q", got)
	}
}

// --- allDashboardURLPaths ---

func TestAllDashboardURLPaths_IncludesDefaultAndListed(t *testing.T) {
	t.Parallel()

	mock := &UniversalMockClient{
		ListDashboardsFn: func(context.Context) ([]homeassistant.DashboardEntry, error) {
			return []homeassistant.DashboardEntry{{URLPath: "energy"}, {URLPath: "leak-sensors"}}, nil
		},
	}

	got, err := allDashboardURLPaths(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"", "energy", "leak-sensors"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAllDashboardURLPaths_ListDashboardsError(t *testing.T) {
	t.Parallel()

	mock := &UniversalMockClient{
		ListDashboardsFn: func(context.Context) ([]homeassistant.DashboardEntry, error) {
			return nil, errors.New("connection failed")
		},
	}

	got, err := allDashboardURLPaths(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if got != nil {
		t.Errorf("expected nil paths on error, got %v", got)
	}
}
