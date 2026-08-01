package handlers

import (
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/jsonpatch"
)

// =============================================================================
// matchesElement tests
// =============================================================================

func TestMatchesElement(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		elem  map[string]any
		match map[string]any
		want  bool
	}{
		{
			name:  "all criteria match",
			elem:  map[string]any{"entity_id": "binary_sensor.door", "to": "off", "platform": "state"},
			match: map[string]any{"entity_id": "binary_sensor.door", "to": "off"},
			want:  true,
		},
		{
			name:  "no criteria match",
			elem:  map[string]any{"entity_id": "binary_sensor.motion", "platform": "state"},
			match: map[string]any{"entity_id": "binary_sensor.door"},
			want:  false,
		},
		{
			name:  "partial match only - second key missing",
			elem:  map[string]any{"entity_id": "binary_sensor.door"},
			match: map[string]any{"entity_id": "binary_sensor.door", "to": "off"},
			want:  false,
		},
		{
			name:  "key missing from element",
			elem:  map[string]any{"platform": "state"},
			match: map[string]any{"entity_id": "binary_sensor.door"},
			want:  false,
		},
		{
			name:  "empty match criteria always matches",
			elem:  map[string]any{"entity_id": "binary_sensor.door"},
			match: map[string]any{},
			want:  true,
		},
		{
			name:  "float64 from JSON vs int match",
			elem:  map[string]any{"brightness": float64(255)},
			match: map[string]any{"brightness": float64(255)},
			want:  true,
		},
		{
			name:  "string value mismatch",
			elem:  map[string]any{"to": "on"},
			match: map[string]any{"to": "off"},
			want:  false,
		},
		{
			name:  "null value match",
			elem:  map[string]any{"from": nil},
			match: map[string]any{"from": nil},
			want:  true,
		},
		{
			name:  "boolean match",
			elem:  map[string]any{"enabled": true},
			match: map[string]any{"enabled": true},
			want:  true,
		},
		{
			name:  "boolean mismatch",
			elem:  map[string]any{"enabled": false},
			match: map[string]any{"enabled": true},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := matchesElement(tt.elem, tt.match)
			if got != tt.want {
				t.Errorf("matchesElement() = %v, want %v", got, tt.want)
			}
		})
	}
}

// findMatchingPaths tests

func TestFindMatchingPaths(t *testing.T) {
	t.Parallel()

	triggers := []any{
		map[string]any{"platform": "state", "entity_id": "binary_sensor.door", "to": "on"},
		map[string]any{"platform": "time", "at": "07:00"},
		map[string]any{"platform": "state", "entity_id": "binary_sensor.motion", "to": "on"},
		map[string]any{"platform": "state", "entity_id": "binary_sensor.door", "to": "off"},
	}

	tests := []struct {
		name  string
		match map[string]any
		want  []string
	}{
		{
			name:  "match by entity_id only - two matches",
			match: map[string]any{"entity_id": "binary_sensor.door"},
			want:  []string{"/triggers/0", "/triggers/3"},
		},
		{
			name:  "match by entity_id and to - single match",
			match: map[string]any{"entity_id": "binary_sensor.door", "to": "off"},
			want:  []string{"/triggers/3"},
		},
		{
			name:  "no match",
			match: map[string]any{"entity_id": "binary_sensor.unknown"},
			want:  nil,
		},
		{
			name:  "match by platform - three matches",
			match: map[string]any{"platform": "state"},
			want:  []string{"/triggers/0", "/triggers/2", "/triggers/3"},
		},
		{
			name:  "match time trigger",
			match: map[string]any{"at": "07:00"},
			want:  []string{"/triggers/1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := findMatchingPaths(triggers, "/triggers", tt.match)
			if len(got) != len(tt.want) {
				t.Fatalf("findMatchingPaths() = %v, want %v", got, tt.want)
			}
			for i, p := range got {
				if p != tt.want[i] {
					t.Errorf("findMatchingPaths()[%d] = %q, want %q", i, p, tt.want[i])
				}
			}
		})
	}
}

func TestFindMatchingPaths_SkipsNonMapNodes(t *testing.T) {
	t.Parallel()

	// Section with a non-map element mixed in
	section := []any{
		"not-a-map",
		map[string]any{"entity_id": "binary_sensor.door"},
		42,
	}

	paths := findMatchingPaths(section, "/triggers", map[string]any{"entity_id": "binary_sensor.door"})
	if len(paths) != 1 || paths[0] != "/triggers/1" {
		t.Errorf("findMatchingPaths() = %v, want [/triggers/1]", paths)
	}
}

func TestFindMatchingPaths_NestedDashboardCards(t *testing.T) {
	t.Parallel()

	// Mirrors issue #144: a chip nested 4 levels below "views" via
	// sections -> cards -> cards -> chips.
	views := []any{
		map[string]any{"title": "Home"},
		map[string]any{
			"title": "Example",
			"sections": []any{
				map[string]any{
					"cards": []any{
						map[string]any{"type": "grid"},
						map[string]any{
							"type": "vertical-stack",
							"cards": []any{
								map[string]any{
									"type": "tile",
									"chips": []any{
										map[string]any{"type": "weather"},
										map[string]any{"type": "weather"},
										map[string]any{
											"type":    "entity",
											"entity":  "device_tracker.example_phone",
											"content": "Example",
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

	match := map[string]any{"content": "Example", "entity": "device_tracker.example_phone"}
	paths := findMatchingPaths(views, "/views", match)

	want := "/views/1/sections/0/cards/1/cards/0/chips/2"
	if len(paths) != 1 || paths[0] != want {
		t.Fatalf("findMatchingPaths() = %v, want [%s]", paths, want)
	}
}

// =============================================================================
// findMatchingIndices tests
// =============================================================================

// =============================================================================
// buildResolvedPath tests
// =============================================================================

// =============================================================================
// validateSemanticOp tests
// =============================================================================

func TestValidateSemanticOp(t *testing.T) {
	t.Parallel()

	validMatch := map[string]any{"entity_id": "binary_sensor.door"}

	tests := []struct {
		name        string
		op          SemanticOperation
		wantErrFrag string
	}{
		{
			name: "valid replace op",
			op: SemanticOperation{
				Operation: jsonpatch.Operation{Op: "replace"},
				Match:     validMatch,
				Section:   "triggers",
				Field:     "for",
			},
		},
		{
			name: "valid remove op without field",
			op: SemanticOperation{
				Operation: jsonpatch.Operation{Op: "remove"},
				Match:     validMatch,
				Section:   "triggers",
			},
		},
		{
			name: "valid add op",
			op: SemanticOperation{
				Operation: jsonpatch.Operation{Op: "add"},
				Match:     validMatch,
				Section:   "triggers",
				Field:     "for",
			},
		},
		{
			name: "valid test op",
			op: SemanticOperation{
				Operation: jsonpatch.Operation{Op: "test"},
				Match:     validMatch,
				Section:   "triggers",
				Field:     "entity_id",
			},
		},
		{
			name: "path and match both set",
			op: SemanticOperation{
				Operation: jsonpatch.Operation{Op: "replace", Path: "/triggers/0/for"},
				Match:     validMatch,
				Section:   "triggers",
				Field:     "for",
			},
			wantErrFrag: "either 'path' or 'match', not both",
		},
		{
			name: "empty match",
			op: SemanticOperation{
				Operation: jsonpatch.Operation{Op: "replace"},
				Match:     map[string]any{},
				Section:   "triggers",
				Field:     "for",
			},
			wantErrFrag: "'match' must not be empty",
		},
		{
			name: "missing section",
			op: SemanticOperation{
				Operation: jsonpatch.Operation{Op: "replace"},
				Match:     validMatch,
				Field:     "for",
			},
			wantErrFrag: "'section' is required",
		},
		{
			name: "move op not supported",
			op: SemanticOperation{
				Operation: jsonpatch.Operation{Op: "move"},
				Match:     validMatch,
				Section:   "triggers",
				Field:     "for",
			},
			wantErrFrag: "move/copy operations",
		},
		{
			name: "copy op not supported",
			op: SemanticOperation{
				Operation: jsonpatch.Operation{Op: "copy"},
				Match:     validMatch,
				Section:   "triggers",
				Field:     "for",
			},
			wantErrFrag: "move/copy operations",
		},
		{
			name: "replace without field",
			op: SemanticOperation{
				Operation: jsonpatch.Operation{Op: "replace"},
				Match:     validMatch,
				Section:   "triggers",
			},
			wantErrFrag: "'field' is required for \"replace\"",
		},
		{
			name: "add without field",
			op: SemanticOperation{
				Operation: jsonpatch.Operation{Op: "add"},
				Match:     validMatch,
				Section:   "triggers",
			},
			wantErrFrag: "'field' is required for \"add\"",
		},
		{
			name: "test without field",
			op: SemanticOperation{
				Operation: jsonpatch.Operation{Op: "test"},
				Match:     validMatch,
				Section:   "triggers",
			},
			wantErrFrag: "'field' is required for \"test\"",
		},
		{
			name: "remove with non-empty field rejected",
			op: SemanticOperation{
				Operation: jsonpatch.Operation{Op: "remove"},
				Match:     validMatch,
				Section:   "triggers",
				Field:     "entity_id",
			},
			wantErrFrag: "'field' must be empty for 'remove'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateSemanticOp(tt.op, 0)
			if tt.wantErrFrag == "" {
				if err != nil {
					t.Errorf("validateSemanticOp() unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("validateSemanticOp() expected error containing %q, got nil", tt.wantErrFrag)
				}
				if !strings.Contains(err.Error(), tt.wantErrFrag) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErrFrag)
				}
			}
		})
	}
}

// =============================================================================
// resolveSemanticOps tests
// =============================================================================

func TestResolveSemanticOps_StandardPassthrough(t *testing.T) {
	t.Parallel()

	doc := map[string]any{
		"mode": "single",
	}
	ops := []SemanticOperation{
		{Operation: jsonpatch.Operation{Op: "replace", Path: "/mode", Value: "queued"}},
	}

	resolved, err := resolveSemanticOps(doc, ops)
	if err != nil {
		t.Fatalf("resolveSemanticOps() error = %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("len(resolved) = %d, want 1", len(resolved))
	}
	if resolved[0].Path != "/mode" || resolved[0].Op != "replace" {
		t.Errorf("resolved op = %+v, want path=/mode op=replace", resolved[0])
	}
}

func TestResolveSemanticOps_SingleMatch(t *testing.T) {
	t.Parallel()

	doc := map[string]any{
		"triggers": []any{
			map[string]any{"platform": "state", "entity_id": "binary_sensor.door", "to": "on"},
			map[string]any{"platform": "time", "at": "07:00"},
		},
	}

	matchIdx := 0
	ops := []SemanticOperation{
		{
			Operation:  jsonpatch.Operation{Op: "replace", Value: "00:05:00"},
			Match:      map[string]any{"entity_id": "binary_sensor.door"},
			Section:    "triggers",
			Field:      "for",
			MatchIndex: &matchIdx,
		},
	}

	resolved, err := resolveSemanticOps(doc, ops)
	if err != nil {
		t.Fatalf("resolveSemanticOps() error = %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("len(resolved) = %d, want 1", len(resolved))
	}
	if resolved[0].Path != "/triggers/0/for" {
		t.Errorf("resolved path = %q, want '/triggers/0/for'", resolved[0].Path)
	}
	if resolved[0].Value != "00:05:00" {
		t.Errorf("resolved value = %v, want '00:05:00'", resolved[0].Value)
	}
}

func TestResolveSemanticOps_MultipleMatches(t *testing.T) {
	t.Parallel()

	doc := map[string]any{
		"triggers": []any{
			map[string]any{"platform": "state", "entity_id": "binary_sensor.door", "to": "on"},
			map[string]any{"platform": "state", "entity_id": "binary_sensor.motion", "to": "on"},
			map[string]any{"platform": "state", "entity_id": "binary_sensor.door", "to": "off"},
		},
	}

	ops := []SemanticOperation{
		{
			Operation: jsonpatch.Operation{Op: "replace", Value: "binary_sensor.new_door"},
			Match:     map[string]any{"entity_id": "binary_sensor.door"},
			Section:   "triggers",
			Field:     "entity_id",
		},
	}

	resolved, err := resolveSemanticOps(doc, ops)
	if err != nil {
		t.Fatalf("resolveSemanticOps() error = %v", err)
	}
	// Two triggers match (indices 0 and 2)
	if len(resolved) != 2 {
		t.Fatalf("len(resolved) = %d, want 2", len(resolved))
	}
	if resolved[0].Path != "/triggers/0/entity_id" {
		t.Errorf("resolved[0].Path = %q, want '/triggers/0/entity_id'", resolved[0].Path)
	}
	if resolved[1].Path != "/triggers/2/entity_id" {
		t.Errorf("resolved[1].Path = %q, want '/triggers/2/entity_id'", resolved[1].Path)
	}
}

func TestResolveSemanticOps_RemoveDecendingOrder(t *testing.T) {
	t.Parallel()

	doc := map[string]any{
		"triggers": []any{
			map[string]any{"platform": "state", "entity_id": "binary_sensor.door"},
			map[string]any{"platform": "time", "at": "07:00"},
			map[string]any{"platform": "state", "entity_id": "binary_sensor.door"},
		},
	}

	ops := []SemanticOperation{
		{
			Operation: jsonpatch.Operation{Op: "remove"},
			Match:     map[string]any{"entity_id": "binary_sensor.door"},
			Section:   "triggers",
		},
	}

	resolved, err := resolveSemanticOps(doc, ops)
	if err != nil {
		t.Fatalf("resolveSemanticOps() error = %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("len(resolved) = %d, want 2", len(resolved))
	}
	// Must be descending: index 2 first, then 0
	if resolved[0].Path != "/triggers/2" {
		t.Errorf("resolved[0].Path = %q, want '/triggers/2'", resolved[0].Path)
	}
	if resolved[1].Path != "/triggers/0" {
		t.Errorf("resolved[1].Path = %q, want '/triggers/0'", resolved[1].Path)
	}
}

func TestResolveSemanticOps_MatchIndexSelection(t *testing.T) {
	t.Parallel()

	doc := map[string]any{
		"triggers": []any{
			map[string]any{"platform": "state", "entity_id": "binary_sensor.door", "to": "on"},
			map[string]any{"platform": "state", "entity_id": "binary_sensor.door", "to": "off"},
		},
	}

	idx1 := 1
	ops := []SemanticOperation{
		{
			Operation:  jsonpatch.Operation{Op: "replace", Value: "00:10:00"},
			Match:      map[string]any{"entity_id": "binary_sensor.door"},
			Section:    "triggers",
			Field:      "for",
			MatchIndex: &idx1,
		},
	}

	resolved, err := resolveSemanticOps(doc, ops)
	if err != nil {
		t.Fatalf("resolveSemanticOps() error = %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("len(resolved) = %d, want 1", len(resolved))
	}
	// MatchIndex=1 picks the second match (index 1 in the array)
	if resolved[0].Path != "/triggers/1/for" {
		t.Errorf("resolved path = %q, want '/triggers/1/for'", resolved[0].Path)
	}
}

func TestResolveSemanticOps_NoMatches(t *testing.T) {
	t.Parallel()

	doc := map[string]any{
		"triggers": []any{
			map[string]any{"platform": "time", "at": "07:00"},
		},
	}

	ops := []SemanticOperation{
		{
			Operation: jsonpatch.Operation{Op: "replace", Value: "00:05:00"},
			Match:     map[string]any{"entity_id": "binary_sensor.nonexistent"},
			Section:   "triggers",
			Field:     "for",
		},
	}

	_, err := resolveSemanticOps(doc, ops)
	if err == nil {
		t.Fatal("resolveSemanticOps() expected error for no matches, got nil")
	}
	if !strings.Contains(err.Error(), "no elements in section") {
		t.Errorf("error = %q, want to contain 'no elements in section'", err.Error())
	}
}

func TestResolveSemanticOps_SectionNotFound(t *testing.T) {
	t.Parallel()

	doc := map[string]any{"mode": "single"}

	ops := []SemanticOperation{
		{
			Operation: jsonpatch.Operation{Op: "replace", Value: "x"},
			Match:     map[string]any{"entity_id": "sensor.x"},
			Section:   "triggers",
			Field:     "for",
		},
	}

	_, err := resolveSemanticOps(doc, ops)
	if err == nil {
		t.Fatal("resolveSemanticOps() expected error for missing section, got nil")
	}
	if !strings.Contains(err.Error(), "not found in config") {
		t.Errorf("error = %q, want to contain 'not found in config'", err.Error())
	}
}

func TestResolveSemanticOps_SectionNotArray(t *testing.T) {
	t.Parallel()

	doc := map[string]any{
		"triggers": "not-an-array",
	}

	ops := []SemanticOperation{
		{
			Operation: jsonpatch.Operation{Op: "replace", Value: "x"},
			Match:     map[string]any{"entity_id": "sensor.x"},
			Section:   "triggers",
			Field:     "for",
		},
	}

	_, err := resolveSemanticOps(doc, ops)
	if err == nil {
		t.Fatal("resolveSemanticOps() expected error for non-array section, got nil")
	}
	if !strings.Contains(err.Error(), "is not an array") {
		t.Errorf("error = %q, want to contain 'is not an array'", err.Error())
	}
}

func TestResolveSemanticOps_MatchIndexOutOfRange(t *testing.T) {
	t.Parallel()

	doc := map[string]any{
		"triggers": []any{
			map[string]any{"entity_id": "binary_sensor.door"},
		},
	}

	idx5 := 5
	ops := []SemanticOperation{
		{
			Operation:  jsonpatch.Operation{Op: "replace", Value: "x"},
			Match:      map[string]any{"entity_id": "binary_sensor.door"},
			Section:    "triggers",
			Field:      "for",
			MatchIndex: &idx5,
		},
	}

	_, err := resolveSemanticOps(doc, ops)
	if err == nil {
		t.Fatal("resolveSemanticOps() expected error for out-of-range match_index, got nil")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("error = %q, want to contain 'out of range'", err.Error())
	}
}

func TestResolveSemanticOps_CrossOpRemoveDescendingOrder(t *testing.T) {
	t.Parallel()

	// Two separate semantic remove ops on the same section — must be globally sorted
	// descending so that index 2 is removed before index 0 (not the other way around).
	doc := map[string]any{
		"triggers": []any{
			map[string]any{"platform": "state", "entity_id": "binary_sensor.door"},   // index 0
			map[string]any{"platform": "time", "at": "07:00"},                        // index 1
			map[string]any{"platform": "state", "entity_id": "binary_sensor.motion"}, // index 2
		},
	}

	ops := []SemanticOperation{
		// First op matches index 0
		{
			Operation: jsonpatch.Operation{Op: "remove"},
			Match:     map[string]any{"entity_id": "binary_sensor.door"},
			Section:   "triggers",
		},
		// Second op matches index 2
		{
			Operation: jsonpatch.Operation{Op: "remove"},
			Match:     map[string]any{"entity_id": "binary_sensor.motion"},
			Section:   "triggers",
		},
	}

	resolved, err := resolveSemanticOps(doc, ops)
	if err != nil {
		t.Fatalf("resolveSemanticOps() error = %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("len(resolved) = %d, want 2", len(resolved))
	}
	// Cross-op sort: index 2 must come before index 0
	if resolved[0].Path != "/triggers/2" {
		t.Errorf("resolved[0].Path = %q, want '/triggers/2' (descending order)", resolved[0].Path)
	}
	if resolved[1].Path != "/triggers/0" {
		t.Errorf("resolved[1].Path = %q, want '/triggers/0' (descending order)", resolved[1].Path)
	}

	// Verify applying these ops leaves only the time trigger
	result, _, applyErr := applyPatchWithSemantics(doc, ops)
	if applyErr != nil {
		t.Fatalf("applyPatchWithSemantics() error = %v", applyErr)
	}
	triggers := result["triggers"].([]any)
	if len(triggers) != 1 {
		t.Fatalf("len(triggers) = %d, want 1", len(triggers))
	}
	remaining := triggers[0].(map[string]any)
	if remaining["platform"] != "time" {
		t.Errorf("remaining trigger = %v, want time trigger", remaining)
	}
}

func TestResolveSemanticOps_NestedMatch(t *testing.T) {
	t.Parallel()

	// Reproduces issue #144: target lives several levels below the named
	// section, nested through sections/cards/cards/chips.
	doc := map[string]any{
		"views": []any{
			map[string]any{
				"sections": []any{
					map[string]any{
						"cards": []any{
							map[string]any{
								"cards": []any{
									map[string]any{
										"chips": []any{
											map[string]any{
												"entity":  "device_tracker.example_phone",
												"content": "Example",
											},
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

	ops := []SemanticOperation{
		{
			Operation: jsonpatch.Operation{Op: "replace", Value: "device_tracker.example_phone_new"},
			Match:     map[string]any{"content": "Example", "entity": "device_tracker.example_phone"},
			Section:   "views",
			Field:     "entity",
		},
	}

	resolved, err := resolveSemanticOps(doc, ops)
	if err != nil {
		t.Fatalf("resolveSemanticOps() error = %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("len(resolved) = %d, want 1", len(resolved))
	}
	want := "/views/0/sections/0/cards/0/cards/0/chips/0/entity"
	if resolved[0].Path != want {
		t.Errorf("resolved path = %q, want %q", resolved[0].Path, want)
	}

	result, _, applyErr := applyPatchWithSemantics(doc, ops)
	if applyErr != nil {
		t.Fatalf("applyPatchWithSemantics() error = %v", applyErr)
	}
	chip := result["views"].([]any)[0].(map[string]any)["sections"].([]any)[0].(map[string]any)["cards"].([]any)[0].(map[string]any)["cards"].([]any)[0].(map[string]any)["chips"].([]any)[0].(map[string]any)
	if chip["entity"] != "device_tracker.example_phone_new" {
		t.Errorf("chip entity = %v, want updated value", chip["entity"])
	}
}

func TestResolveSemanticOps_NestedRemoveOrdering(t *testing.T) {
	t.Parallel()

	// Two chips at different indices within the same nested array must be
	// removed highest-index-first so earlier removals don't shift the
	// indices of not-yet-processed matches.
	doc := map[string]any{
		"views": []any{
			map[string]any{
				"cards": []any{
					map[string]any{
						"chips": []any{
							map[string]any{"type": "weather"},
							map[string]any{"type": "stale"},
							map[string]any{"type": "weather"},
							map[string]any{"type": "stale"},
						},
					},
				},
			},
		},
	}

	ops := []SemanticOperation{
		{
			Operation: jsonpatch.Operation{Op: "remove"},
			Match:     map[string]any{"type": "stale"},
			Section:   "views",
		},
	}

	result, _, err := applyPatchWithSemantics(doc, ops)
	if err != nil {
		t.Fatalf("applyPatchWithSemantics() error = %v", err)
	}
	chips := result["views"].([]any)[0].(map[string]any)["cards"].([]any)[0].(map[string]any)["chips"].([]any)
	if len(chips) != 2 {
		t.Fatalf("len(chips) = %d, want 2", len(chips))
	}
	for _, c := range chips {
		if c.(map[string]any)["type"] != "weather" {
			t.Errorf("remaining chip = %v, want weather", c)
		}
	}
}

// TestResolveSemanticOps_SiblingBranchRemovesStableOrder verifies removeBeforePaths'
// documented behavior for paths that diverge at a non-numeric segment (distinct
// object keys, not array indices): there is no defined removal order between
// independent sibling branches, so sortSemanticRemoves must leave them in their
// original DFS match order (N3) rather than reordering them. Removal safety here
// doesn't depend on order — "badges" and "cards" are separate arrays, so removing
// from one never shifts indices in the other — this test locks in the stability
// guarantee itself, which the "differing non-numeric segment" branch of
// removeBeforePaths relies on.
func TestResolveSemanticOps_SiblingBranchRemovesStableOrder(t *testing.T) {
	t.Parallel()

	doc := map[string]any{
		"views": []any{
			map[string]any{
				"badges": []any{
					map[string]any{"type": "entity", "stale": true},
				},
				"cards": []any{
					map[string]any{"type": "entity", "stale": true},
				},
			},
		},
	}

	ops := []SemanticOperation{
		{
			Operation: jsonpatch.Operation{Op: "remove"},
			Match:     map[string]any{"stale": true},
			Section:   "views",
		},
	}

	resolved, err := resolveSemanticOps(doc, ops)
	if err != nil {
		t.Fatalf("resolveSemanticOps() error = %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("len(resolved) = %d, want 2", len(resolved))
	}
	// DFS visits object keys in sorted order, so "badges" (matched first) must
	// stay before "cards" (matched second) — no non-numeric-segment reordering.
	if resolved[0].Path != "/views/0/badges/0" {
		t.Errorf("resolved[0].Path = %q, want '/views/0/badges/0'", resolved[0].Path)
	}
	if resolved[1].Path != "/views/0/cards/0" {
		t.Errorf("resolved[1].Path = %q, want '/views/0/cards/0'", resolved[1].Path)
	}
}

func TestResolveSemanticOps_MixedOps(t *testing.T) {
	t.Parallel()

	doc := map[string]any{
		"mode": "single",
		"triggers": []any{
			map[string]any{"platform": "state", "entity_id": "binary_sensor.door", "to": "on"},
		},
	}

	ops := []SemanticOperation{
		// Standard op
		{Operation: jsonpatch.Operation{Op: "replace", Path: "/mode", Value: "queued"}},
		// Semantic op
		{
			Operation: jsonpatch.Operation{Op: "replace", Value: "00:05:00"},
			Match:     map[string]any{"entity_id": "binary_sensor.door"},
			Section:   "triggers",
			Field:     "for",
		},
	}

	resolved, err := resolveSemanticOps(doc, ops)
	if err != nil {
		t.Fatalf("resolveSemanticOps() error = %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("len(resolved) = %d, want 2", len(resolved))
	}
	if resolved[0].Path != "/mode" {
		t.Errorf("resolved[0].Path = %q, want '/mode'", resolved[0].Path)
	}
	if resolved[1].Path != "/triggers/0/for" {
		t.Errorf("resolved[1].Path = %q, want '/triggers/0/for'", resolved[1].Path)
	}
}

// =============================================================================
// End-to-end: applyPatchWithSemantics
// =============================================================================

func TestApplyPatchWithSemantics_EndToEnd(t *testing.T) {
	t.Parallel()

	// Realistic automation config
	doc := map[string]any{
		"id":    "morning_routine",
		"alias": "Morning Routine",
		"mode":  "single",
		"triggers": []any{
			map[string]any{"platform": "state", "entity_id": "binary_sensor.door", "to": "on"},
			map[string]any{"platform": "time", "at": "07:00"},
		},
		"conditions": []any{
			map[string]any{"condition": "state", "entity_id": "input_boolean.vacation", "state": "off"},
		},
		"actions": []any{
			map[string]any{"action": "light.turn_on", "target": map[string]any{"entity_id": "light.kitchen"}},
		},
	}

	t.Run("add trigger for delay", func(t *testing.T) {
		t.Parallel()
		ops := []SemanticOperation{
			{
				// Use "add" since "for" doesn't exist yet in the trigger (RFC 6902: replace requires existing key)
				Operation: jsonpatch.Operation{Op: "add", Value: "00:05:00"},
				Match:     map[string]any{"entity_id": "binary_sensor.door", "to": "on"},
				Section:   "triggers",
				Field:     "for",
			},
		}
		result, _, err := applyPatchWithSemantics(doc, ops)
		if err != nil {
			t.Fatalf("applyPatchWithSemantics() error = %v", err)
		}
		triggers := result["triggers"].([]any)
		trigger0 := triggers[0].(map[string]any)
		if trigger0["for"] != "00:05:00" {
			t.Errorf("trigger 'for' = %v, want '00:05:00'", trigger0["for"])
		}
	})

	t.Run("replace condition state", func(t *testing.T) {
		t.Parallel()
		ops := []SemanticOperation{
			{
				Operation: jsonpatch.Operation{Op: "replace", Value: "on"},
				Match:     map[string]any{"entity_id": "input_boolean.vacation"},
				Section:   "conditions",
				Field:     "state",
			},
		}
		result, _, err := applyPatchWithSemantics(doc, ops)
		if err != nil {
			t.Fatalf("applyPatchWithSemantics() error = %v", err)
		}
		conditions := result["conditions"].([]any)
		cond0 := conditions[0].(map[string]any)
		if cond0["state"] != "on" {
			t.Errorf("condition state = %v, want 'on'", cond0["state"])
		}
	})

	t.Run("remove trigger by entity_id", func(t *testing.T) {
		t.Parallel()
		ops := []SemanticOperation{
			{
				Operation: jsonpatch.Operation{Op: "remove"},
				Match:     map[string]any{"entity_id": "binary_sensor.door"},
				Section:   "triggers",
			},
		}
		result, _, err := applyPatchWithSemantics(doc, ops)
		if err != nil {
			t.Fatalf("applyPatchWithSemantics() error = %v", err)
		}
		triggers := result["triggers"].([]any)
		if len(triggers) != 1 {
			t.Errorf("len(triggers) = %d, want 1 (time trigger should remain)", len(triggers))
		}
		// Only the time trigger should remain
		remaining := triggers[0].(map[string]any)
		if remaining["platform"] != "time" {
			t.Errorf("remaining trigger platform = %v, want 'time'", remaining["platform"])
		}
	})

	t.Run("combined standard and semantic ops", func(t *testing.T) {
		t.Parallel()
		ops := []SemanticOperation{
			{Operation: jsonpatch.Operation{Op: "replace", Path: "/mode", Value: "parallel"}},
			{
				Operation: jsonpatch.Operation{Op: "replace", Value: "08:00"},
				Match:     map[string]any{"at": "07:00"},
				Section:   "triggers",
				Field:     "at",
			},
		}
		result, _, err := applyPatchWithSemantics(doc, ops)
		if err != nil {
			t.Fatalf("applyPatchWithSemantics() error = %v", err)
		}
		if result["mode"] != "parallel" {
			t.Errorf("mode = %v, want 'parallel'", result["mode"])
		}
		triggers := result["triggers"].([]any)
		timeTrigger := triggers[1].(map[string]any)
		if timeTrigger["at"] != "08:00" {
			t.Errorf("at = %v, want '08:00'", timeTrigger["at"])
		}
	})
}

// =============================================================================
// parseOperations — semantic field parsing
// =============================================================================

func TestParseOperations_SemanticFields(t *testing.T) {
	t.Parallel()

	idx := 0
	tests := []struct {
		name        string
		args        map[string]any
		wantMatch   map[string]any
		wantSection string
		wantField   string
		wantMIIdx   *int
	}{
		{
			name: "semantic fields parsed",
			args: map[string]any{
				"operations": []any{
					map[string]any{
						"op":      "replace",
						"match":   map[string]any{"entity_id": "binary_sensor.door"},
						"section": "triggers",
						"field":   "for",
						"value":   "00:05:00",
					},
				},
			},
			wantMatch:   map[string]any{"entity_id": "binary_sensor.door"},
			wantSection: "triggers",
			wantField:   "for",
		},
		{
			name: "match_index parsed as float64 from JSON",
			args: map[string]any{
				"operations": []any{
					map[string]any{
						"op":          "replace",
						"match":       map[string]any{"entity_id": "binary_sensor.door"},
						"section":     "triggers",
						"field":       "for",
						"value":       "x",
						"match_index": float64(0),
					},
				},
			},
			wantMatch:   map[string]any{"entity_id": "binary_sensor.door"},
			wantSection: "triggers",
			wantField:   "for",
			wantMIIdx:   &idx,
		},
		{
			name: "standard op - no semantic fields",
			args: map[string]any{
				"operations": []any{
					map[string]any{"op": "replace", "path": "/mode", "value": "queued"},
				},
			},
			wantSection: "",
			wantField:   "",
		},
	}

	// Non-map match value must return error
	t.Run("non-map match returns error", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{
			"operations": []any{
				map[string]any{
					"op":    "replace",
					"match": "not-an-object",
					"field": "for",
					"value": "x",
				},
			},
		}
		ops, errResult := parseOperations(args)
		if ops != nil {
			t.Error("expected nil ops for non-map match")
		}
		if errResult == nil {
			t.Fatal("expected error result for non-map match")
		}
		if !strings.Contains(errResult.Content[0].Text, "'match'") {
			t.Errorf("error = %q, want to contain \"'match'\"", errResult.Content[0].Text)
		}
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ops, errResult := parseOperations(tt.args)
			if errResult != nil {
				t.Fatalf("parseOperations() error: %v", errResult.Content[0].Text)
			}
			if len(ops) != 1 {
				t.Fatalf("len(ops) = %d, want 1", len(ops))
			}
			op := ops[0]
			if op.Section != tt.wantSection {
				t.Errorf("Section = %q, want %q", op.Section, tt.wantSection)
			}
			if op.Field != tt.wantField {
				t.Errorf("Field = %q, want %q", op.Field, tt.wantField)
			}
			if tt.wantMIIdx != nil {
				if op.MatchIndex == nil || *op.MatchIndex != *tt.wantMIIdx {
					t.Errorf("MatchIndex = %v, want %d", op.MatchIndex, *tt.wantMIIdx)
				}
			} else if op.MatchIndex != nil {
				t.Errorf("MatchIndex = %v, want nil", op.MatchIndex)
			}
		})
	}
}
