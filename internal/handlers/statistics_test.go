package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// Unique fixture helpers to avoid clashing with pointer helpers in other
// test files of this package.
func statTestStr(s string) *string { return &s }
func statTestBool(b bool) *bool    { return &b }
func statTestInt(i int) *int       { return &i }

// statTestLegacyMeta uses the pre-mean_type HA response shape
// (has_mean/has_sum booleans).
func statTestLegacyMeta() []homeassistant.StatisticMeta {
	return []homeassistant.StatisticMeta{
		{
			StatisticID:    "sensor.mcptest_battery",
			Source:         "recorder",
			DisplayUnit:    statTestStr("mV"),
			StatisticsUnit: statTestStr("mV"),
			HasMean:        statTestBool(true),
			HasSum:         statTestBool(false),
			Name:           statTestStr("Battery"),
		},
		{
			StatisticID:    "sensor.mcptest_energy",
			Source:         "recorder",
			StatisticsUnit: statTestStr("kWh"),
			HasMean:        statTestBool(false),
			HasSum:         statTestBool(true),
		},
	}
}

// statTestModernMeta uses the HA >= 2024.11 shape where has_mean is
// replaced by mean_type (StatisticMeanType: NONE=0, ARITHMETIC=1,
// CIRCULAR=2) while has_sum stays a plain boolean.
func statTestModernMeta() []homeassistant.StatisticMeta {
	return []homeassistant.StatisticMeta{
		{
			StatisticID:    "sensor.mcptest_battery",
			Source:         "recorder",
			StatisticsUnit: statTestStr("mV"),
			MeanType:       statTestInt(1),
			HasSum:         statTestBool(false),
		},
	}
}

// statTestManyMeta generates n distinct StatisticMeta entries, used to
// exercise the default output cap (issue C1) without hand-writing a large
// fixture.
func statTestManyMeta(n int) []homeassistant.StatisticMeta {
	metas := make([]homeassistant.StatisticMeta, n)
	for i := range metas {
		metas[i] = homeassistant.StatisticMeta{
			StatisticID: fmt.Sprintf("sensor.mcptest_bulk_%03d", i),
			Source:      "recorder",
		}
	}
	return metas
}

// statTestManyIssues generates n distinct single-issue validation entries,
// used to exercise the default output cap (issue C1) for action=validate.
func statTestManyIssues(n int) map[string][]homeassistant.StatisticValidationIssue {
	issues := make(map[string][]homeassistant.StatisticValidationIssue, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("sensor.mcptest_bulk_%03d", i)
		issues[id] = []homeassistant.StatisticValidationIssue{{Type: "no_state"}}
	}
	return issues
}

// statTestMetaForIDs returns minimal StatisticMeta entries for the given
// ids, marking them "known to the recorder" for action=clear's pre-check
// (resolveKnownStatisticIDs).
func statTestMetaForIDs(ids ...string) []homeassistant.StatisticMeta {
	metas := make([]homeassistant.StatisticMeta, 0, len(ids))
	for _, id := range ids {
		metas = append(metas, homeassistant.StatisticMeta{StatisticID: id, Source: "recorder"})
	}
	return metas
}

func TestManageStatisticsToolSchema(t *testing.T) {
	t.Parallel()

	h := NewStatisticsHandlers()
	tool := h.manageStatisticsTool()
	verifyToolSchema(t, tool, toolSchemaExpectation{
		ExpectedName:    "manage_statistics",
		WantDescription: true,
		RequiredParams:  []string{"action"},
		OptionalParams:  []string{"statistic_type", "statistic_ids", "limit", "format", "dry_run"},
	})

	actionEnum := tool.InputSchema.Properties["action"].Enum
	if len(actionEnum) != 3 {
		t.Errorf("action enum has %d values, want 3", len(actionEnum))
	}
	wantActions := map[string]bool{statActionList: true, statActionValidate: true, statActionClear: true}
	for _, a := range actionEnum {
		if !wantActions[a] {
			t.Errorf("unexpected action enum value %q", a)
			continue
		}
		delete(wantActions, a)
	}
	if len(wantActions) != 0 {
		t.Errorf("action enum missing values: %v", wantActions)
	}

	statTypeEnum := tool.InputSchema.Properties["statistic_type"].Enum
	if len(statTypeEnum) != 2 {
		t.Fatalf("statistic_type enum has %d values, want 2", len(statTypeEnum))
	}
	if statTypeEnum[0] != "mean" || statTypeEnum[1] != "sum" {
		t.Errorf("statistic_type enum = %v, want [mean sum]", statTypeEnum)
	}

	formatEnum := tool.InputSchema.Properties["format"].Enum
	if len(formatEnum) != 2 {
		t.Fatalf("format enum has %d values, want 2", len(formatEnum))
	}
	if formatEnum[0] != formatNatural || formatEnum[1] != formatJSON {
		t.Errorf("format enum = %v, want [%s %s]", formatEnum, formatNatural, formatJSON)
	}

	// statistic_ids must be an array of strings
	statisticIDsProp := tool.InputSchema.Properties["statistic_ids"]
	if statisticIDsProp.Type != "array" {
		t.Errorf("statistic_ids.Type = %q, want %q", statisticIDsProp.Type, "array")
	}
	if statisticIDsProp.Items == nil || statisticIDsProp.Items.Type != "string" {
		t.Errorf("statistic_ids.Items missing or not string-typed")
	}

	limitProp, ok := tool.InputSchema.Properties["limit"]
	if !ok {
		t.Fatal("missing limit property in schema")
	}
	if limitProp.Type != "integer" {
		t.Errorf("limit.Type = %q, want integer", limitProp.Type)
	}

	dryRunProp, ok := tool.InputSchema.Properties["dry_run"]
	if !ok {
		t.Fatal("missing dry_run property in schema")
	}
	if dryRunProp.Type != "boolean" {
		t.Errorf("dry_run.Type = %q, want boolean", dryRunProp.Type)
	}
}

func TestManageStatistics_DispatchValidation(t *testing.T) {
	t.Parallel()

	tooManyIDs := make([]any, maxClearStatisticIDs+1)
	for i := range tooManyIDs {
		tooManyIDs[i] = fmt.Sprintf("sensor.mcptest_%d", i)
	}

	tests := []handlerTestCase{
		{
			name:         "missing action",
			args:         map[string]any{},
			wantError:    true,
			wantContains: []string{"action"},
		},
		{
			name:         "unknown action lists valid actions",
			args:         map[string]any{"action": "bogus"},
			wantError:    true,
			wantContains: []string{"list", "validate", "clear"},
		},
		{
			name:         "invalid statistic_type rejected",
			args:         map[string]any{"action": statActionList, "statistic_type": "bogus"},
			wantError:    true,
			wantContains: []string{"mean", "sum"},
		},
		{
			name:         "list with non-integer limit rejected",
			args:         map[string]any{"action": statActionList, "limit": "10"},
			wantError:    true,
			wantContains: []string{"limit"},
		},
		{
			name:         "list with negative limit rejected",
			args:         map[string]any{"action": statActionList, "limit": float64(-1)},
			wantError:    true,
			wantContains: []string{"limit"},
		},
		{
			name:         "validate with non-integer limit rejected",
			args:         map[string]any{"action": statActionValidate, "limit": "10"},
			wantError:    true,
			wantContains: []string{"limit"},
		},
		{
			name:         "clear without statistic_ids rejected",
			args:         map[string]any{"action": statActionClear},
			wantError:    true,
			wantContains: []string{"statistic_ids"},
		},
		{
			name:         "clear with non-array statistic_ids rejected",
			args:         map[string]any{"action": statActionClear, "statistic_ids": "sensor.a"},
			wantError:    true,
			wantContains: []string{"array"},
		},
		{
			name:         "clear with empty statistic_ids rejected",
			args:         map[string]any{"action": statActionClear, "statistic_ids": []any{}},
			wantError:    true,
			wantContains: []string{"at least one"},
		},
		{
			name:         "clear with non-string element rejected",
			args:         map[string]any{"action": statActionClear, "statistic_ids": []any{"sensor.a", float64(42)}},
			wantError:    true,
			wantContains: []string{"statistic_ids[1]", "string"},
		},
		{
			name:         "clear with empty string element rejected",
			args:         map[string]any{"action": statActionClear, "statistic_ids": []any{""}},
			wantError:    true,
			wantContains: []string{"statistic_ids[0]", "empty"},
		},
		{
			name:         "clear with whitespace-only element rejected",
			args:         map[string]any{"action": statActionClear, "statistic_ids": []any{"   "}},
			wantError:    true,
			wantContains: []string{"statistic_ids[0]", "empty"},
		},
		{
			name:         "clear with malformed id rejected",
			args:         map[string]any{"action": statActionClear, "statistic_ids": []any{"not-a-valid-id"}},
			wantError:    true,
			wantContains: []string{"not-a-valid-id", "valid statistic id"},
		},
		{
			name:         "clear with too many ids rejected",
			args:         map[string]any{"action": statActionClear, "statistic_ids": tooManyIDs},
			wantError:    true,
			wantContains: []string{"too many", fmt.Sprintf("%d", maxClearStatisticIDs)},
		},
	}
	runHandlerTestCases(t, tests, (&StatisticsHandlers{}).handleManageStatistics)
}

func TestManageStatistics_ListAction(t *testing.T) {
	t.Parallel()

	h := &StatisticsHandlers{}

	t.Run("no filter passes empty statistic_type", func(t *testing.T) {
		t.Parallel()

		var gotType string
		client := &UniversalMockClient{}
		client.ListStatisticIDsFn = func(_ context.Context, statisticType string) ([]homeassistant.StatisticMeta, error) {
			gotType = statisticType
			return statTestLegacyMeta(), nil
		}

		result, err := h.handleManageStatistics(context.Background(), client, map[string]any{"action": statActionList})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected IsError result: %s", result.Content[0].Text)
		}
		if gotType != "" {
			t.Errorf("ListStatisticIDs called with statisticType %q, want empty", gotType)
		}
		content := result.Content[0].Text
		assertContainsAll(t, content, []string{"sensor.mcptest_battery", "sensor.mcptest_energy"})
	})

	t.Run("statistic_type=mean forwarded", func(t *testing.T) {
		t.Parallel()

		var gotType string
		client := &UniversalMockClient{}
		client.ListStatisticIDsFn = func(_ context.Context, statisticType string) ([]homeassistant.StatisticMeta, error) {
			gotType = statisticType
			return []homeassistant.StatisticMeta{}, nil
		}

		result, err := h.handleManageStatistics(context.Background(), client, map[string]any{
			"action":         statActionList,
			"statistic_type": "mean",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected IsError result: %s", result.Content[0].Text)
		}
		if gotType != "mean" {
			t.Errorf("ListStatisticIDs called with statisticType %q, want %q", gotType, "mean")
		}
	})

	t.Run("empty result message", func(t *testing.T) {
		t.Parallel()

		client := &UniversalMockClient{}
		client.ListStatisticIDsFn = func(_ context.Context, _ string) ([]homeassistant.StatisticMeta, error) {
			return []homeassistant.StatisticMeta{}, nil
		}

		result, err := h.handleManageStatistics(context.Background(), client, map[string]any{"action": statActionList})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected IsError result: %s", result.Content[0].Text)
		}
		assertContainsAll(t, result.Content[0].Text, []string{"No statistic ids found"})
	})

	t.Run("limit truncates with notice", func(t *testing.T) {
		t.Parallel()

		client := &UniversalMockClient{}
		client.ListStatisticIDsFn = func(_ context.Context, _ string) ([]homeassistant.StatisticMeta, error) {
			return statTestLegacyMeta(), nil
		}

		result, err := h.handleManageStatistics(context.Background(), client, map[string]any{
			"action": statActionList,
			"limit":  float64(1),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		content := result.Content[0].Text
		assertContainsAll(t, content, []string{"sensor.mcptest_battery", "(showing 1)"})
		assertNotContainsAny(t, content, []string{"sensor.mcptest_energy"})
	})

	t.Run("limit equal to total does not truncate", func(t *testing.T) {
		t.Parallel()

		client := &UniversalMockClient{}
		client.ListStatisticIDsFn = func(_ context.Context, _ string) ([]homeassistant.StatisticMeta, error) {
			return statTestLegacyMeta(), nil
		}

		result, err := h.handleManageStatistics(context.Background(), client, map[string]any{
			"action": statActionList,
			"limit":  float64(2),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		content := result.Content[0].Text
		assertContainsAll(t, content, []string{"sensor.mcptest_battery", "sensor.mcptest_energy"})
		assertNotContainsAny(t, content, []string{"showing"})
	})

	t.Run("client error surfaces as IsError", func(t *testing.T) {
		t.Parallel()

		client := &UniversalMockClient{}
		client.ListStatisticIDsFn = func(_ context.Context, _ string) ([]homeassistant.StatisticMeta, error) {
			return nil, fmt.Errorf("ws not connected")
		}

		result, err := h.handleManageStatistics(context.Background(), client, map[string]any{"action": statActionList})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Fatalf("want IsError, got: %s", result.Content[0].Text)
		}
		assertContainsAll(t, result.Content[0].Text, []string{"ws not connected"})
	})

	t.Run("no limit arg applies default cap (issue C1)", func(t *testing.T) {
		t.Parallel()

		client := &UniversalMockClient{}
		client.ListStatisticIDsFn = func(_ context.Context, _ string) ([]homeassistant.StatisticMeta, error) {
			return statTestManyMeta(defaultStatisticListLimit + 50), nil
		}

		result, err := h.handleManageStatistics(context.Background(), client, map[string]any{"action": statActionList})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected IsError result: %s", result.Content[0].Text)
		}
		content := result.Content[0].Text
		assertContainsAll(t, content, []string{
			fmt.Sprintf("%d statistic ids", defaultStatisticListLimit+50),
			fmt.Sprintf("(showing %d)", defaultStatisticListLimit),
		})
	})

	t.Run("explicit limit 0 returns everything (issue C1)", func(t *testing.T) {
		t.Parallel()

		total := defaultStatisticListLimit + 50
		client := &UniversalMockClient{}
		client.ListStatisticIDsFn = func(_ context.Context, _ string) ([]homeassistant.StatisticMeta, error) {
			return statTestManyMeta(total), nil
		}

		result, err := h.handleManageStatistics(context.Background(), client, map[string]any{
			"action": statActionList,
			"limit":  float64(0),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected IsError result: %s", result.Content[0].Text)
		}
		content := result.Content[0].Text
		assertContainsAll(t, content, []string{fmt.Sprintf("%d statistic ids", total)})
		assertNotContainsAny(t, content, []string{"showing"})
	})
}

func TestManageStatistics_ClearAction(t *testing.T) {
	t.Parallel()

	t.Run("valid ids forwarded to client", func(t *testing.T) {
		t.Parallel()

		var gotIDs []string
		client := &UniversalMockClient{}
		client.ListStatisticIDsFn = func(_ context.Context, _ string) ([]homeassistant.StatisticMeta, error) {
			return statTestMetaForIDs("sensor.mcptest_a", "sensor.mcptest_b"), nil
		}
		client.ClearStatisticsFn = func(_ context.Context, ids []string) error {
			gotIDs = ids
			return nil
		}

		result, err := (&StatisticsHandlers{}).handleManageStatistics(context.Background(), client, map[string]any{
			"action":        statActionClear,
			"statistic_ids": []any{"sensor.mcptest_a", "sensor.mcptest_b"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected IsError result: %s", result.Content[0].Text)
		}
		if len(gotIDs) != 2 || gotIDs[0] != "sensor.mcptest_a" || gotIDs[1] != "sensor.mcptest_b" {
			t.Errorf("ClearStatistics called with %v, want [sensor.mcptest_a sensor.mcptest_b]", gotIDs)
		}
		assertContainsAll(t, result.Content[0].Text, []string{"sensor.mcptest_a", "sensor.mcptest_b", "2"})
	})

	t.Run("duplicate ids deduplicated before forwarding", func(t *testing.T) {
		t.Parallel()

		var gotIDs []string
		client := &UniversalMockClient{}
		client.ListStatisticIDsFn = func(_ context.Context, _ string) ([]homeassistant.StatisticMeta, error) {
			return statTestMetaForIDs("sensor.mcptest_a", "sensor.mcptest_b"), nil
		}
		client.ClearStatisticsFn = func(_ context.Context, ids []string) error {
			gotIDs = ids
			return nil
		}

		result, err := (&StatisticsHandlers{}).handleManageStatistics(context.Background(), client, map[string]any{
			"action":        statActionClear,
			"statistic_ids": []any{"sensor.mcptest_a", "sensor.mcptest_a", "sensor.mcptest_b"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected IsError result: %s", result.Content[0].Text)
		}
		if len(gotIDs) != 2 || gotIDs[0] != "sensor.mcptest_a" || gotIDs[1] != "sensor.mcptest_b" {
			t.Errorf("ClearStatistics called with %v, want deduped [sensor.mcptest_a sensor.mcptest_b]", gotIDs)
		}
		assertContainsAll(t, result.Content[0].Text, []string{"Cleared statistics for 2"})
	})

	t.Run("whitespace trimmed before forwarding", func(t *testing.T) {
		t.Parallel()

		var gotIDs []string
		client := &UniversalMockClient{}
		client.ListStatisticIDsFn = func(_ context.Context, _ string) ([]homeassistant.StatisticMeta, error) {
			return statTestMetaForIDs("sensor.mcptest_a"), nil
		}
		client.ClearStatisticsFn = func(_ context.Context, ids []string) error {
			gotIDs = ids
			return nil
		}

		_, err := (&StatisticsHandlers{}).handleManageStatistics(context.Background(), client, map[string]any{
			"action":        statActionClear,
			"statistic_ids": []any{"  sensor.mcptest_a  "},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(gotIDs) != 1 || gotIDs[0] != "sensor.mcptest_a" {
			t.Errorf("ClearStatistics called with %v, want [sensor.mcptest_a]", gotIDs)
		}
	})

	t.Run("external statistic id (domain:key) accepted", func(t *testing.T) {
		t.Parallel()

		var gotIDs []string
		client := &UniversalMockClient{}
		client.ListStatisticIDsFn = func(_ context.Context, _ string) ([]homeassistant.StatisticMeta, error) {
			return statTestMetaForIDs("growatt:total_energy"), nil
		}
		client.ClearStatisticsFn = func(_ context.Context, ids []string) error {
			gotIDs = ids
			return nil
		}

		_, err := (&StatisticsHandlers{}).handleManageStatistics(context.Background(), client, map[string]any{
			"action":        statActionClear,
			"statistic_ids": []any{"growatt:total_energy"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(gotIDs) != 1 || gotIDs[0] != "growatt:total_energy" {
			t.Errorf("ClearStatistics called with %v, want [growatt:total_energy]", gotIDs)
		}
	})

	t.Run("external statistic id with digit-leading domain accepted (issue N1)", func(t *testing.T) {
		t.Parallel()

		var gotIDs []string
		client := &UniversalMockClient{}
		client.ListStatisticIDsFn = func(_ context.Context, _ string) ([]homeassistant.StatisticMeta, error) {
			return statTestMetaForIDs("17track:total_distance"), nil
		}
		client.ClearStatisticsFn = func(_ context.Context, ids []string) error {
			gotIDs = ids
			return nil
		}

		result, err := (&StatisticsHandlers{}).handleManageStatistics(context.Background(), client, map[string]any{
			"action":        statActionClear,
			"statistic_ids": []any{"17track:total_distance"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected IsError result: %s", result.Content[0].Text)
		}
		if len(gotIDs) != 1 || gotIDs[0] != "17track:total_distance" {
			t.Errorf("ClearStatistics called with %v, want [17track:total_distance]", gotIDs)
		}
	})

	t.Run("client error surfaces as IsError", func(t *testing.T) {
		t.Parallel()

		client := &UniversalMockClient{}
		client.ClearStatisticsFn = func(_ context.Context, _ []string) error {
			return fmt.Errorf("clear_statistics timed out")
		}

		result, err := (&StatisticsHandlers{}).handleManageStatistics(context.Background(), client, map[string]any{
			"action":        statActionClear,
			"statistic_ids": []any{"sensor.mcptest_a"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Fatalf("want IsError, got: %s", result.Content[0].Text)
		}
		assertContainsAll(t, result.Content[0].Text, []string{"clear_statistics timed out"})
	})

	t.Run("unknown ids reported as skipped, not falsely cleared (issue W2)", func(t *testing.T) {
		t.Parallel()

		var gotIDs []string
		client := &UniversalMockClient{}
		client.ListStatisticIDsFn = func(_ context.Context, _ string) ([]homeassistant.StatisticMeta, error) {
			return statTestMetaForIDs("sensor.mcptest_a"), nil // mcptest_b is not known
		}
		client.ClearStatisticsFn = func(_ context.Context, ids []string) error {
			gotIDs = ids
			return nil
		}

		result, err := (&StatisticsHandlers{}).handleManageStatistics(context.Background(), client, map[string]any{
			"action":        statActionClear,
			"statistic_ids": []any{"sensor.mcptest_a", "sensor.mcptest_b"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected IsError result: %s", result.Content[0].Text)
		}
		// Both ids are still forwarded to HA - it does its own filtering -
		// but the message must distinguish known from unknown.
		if len(gotIDs) != 2 {
			t.Errorf("ClearStatistics called with %v, want both requested ids forwarded", gotIDs)
		}
		content := result.Content[0].Text
		assertContainsAll(t, content, []string{
			"Cleared statistics for 1 statistic id: sensor.mcptest_a",
			"1 id not present in the recorder database (skipped): sensor.mcptest_b",
		})
	})

	t.Run("all ids unknown reports zero cleared without a dangling list", func(t *testing.T) {
		t.Parallel()

		client := &UniversalMockClient{}
		client.ListStatisticIDsFn = func(_ context.Context, _ string) ([]homeassistant.StatisticMeta, error) {
			return []homeassistant.StatisticMeta{}, nil
		}
		client.ClearStatisticsFn = func(_ context.Context, _ []string) error {
			return nil
		}

		result, err := (&StatisticsHandlers{}).handleManageStatistics(context.Background(), client, map[string]any{
			"action":        statActionClear,
			"statistic_ids": []any{"sensor.mcptest_a"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected IsError result: %s", result.Content[0].Text)
		}
		assertContainsAll(t, result.Content[0].Text, []string{
			"Cleared statistics for 0 statistic ids: none of the requested ids were present",
		})
	})

	t.Run("dry_run previews without clearing (issue W3)", func(t *testing.T) {
		t.Parallel()

		cleared := false
		client := &UniversalMockClient{}
		client.ListStatisticIDsFn = func(_ context.Context, _ string) ([]homeassistant.StatisticMeta, error) {
			return statTestMetaForIDs("sensor.mcptest_a"), nil
		}
		client.ClearStatisticsFn = func(_ context.Context, _ []string) error {
			cleared = true
			return nil
		}

		result, err := (&StatisticsHandlers{}).handleManageStatistics(context.Background(), client, map[string]any{
			"action":        statActionClear,
			"statistic_ids": []any{"sensor.mcptest_a", "sensor.mcptest_b"},
			"dry_run":       true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected IsError result: %s", result.Content[0].Text)
		}
		if cleared {
			t.Error("dry_run must not call ClearStatistics")
		}
		content := result.Content[0].Text
		assertContainsAll(t, content, []string{
			"Would clear statistics for 1 statistic id: sensor.mcptest_a",
			"1 id not present in the recorder database (skipped): sensor.mcptest_b",
		})
	})

	t.Run("pre-check failure degrades clear with a warning (issue W2)", func(t *testing.T) {
		t.Parallel()

		var gotIDs []string
		client := &UniversalMockClient{}
		client.ListStatisticIDsFn = func(_ context.Context, _ string) ([]homeassistant.StatisticMeta, error) {
			return nil, fmt.Errorf("ws not connected")
		}
		client.ClearStatisticsFn = func(_ context.Context, ids []string) error {
			gotIDs = ids
			return nil
		}

		result, err := (&StatisticsHandlers{}).handleManageStatistics(context.Background(), client, map[string]any{
			"action":        statActionClear,
			"statistic_ids": []any{"sensor.mcptest_a"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected IsError result: %s", result.Content[0].Text)
		}
		if len(gotIDs) != 1 || gotIDs[0] != "sensor.mcptest_a" {
			t.Errorf("ClearStatistics called with %v, want [sensor.mcptest_a]", gotIDs)
		}
		assertContainsAll(t, result.Content[0].Text, []string{"Cleared statistics for 1", "WARNING", "could not verify"})
	})

	t.Run("pre-check failure on dry_run reports a warning without clearing (issue W2+W3)", func(t *testing.T) {
		t.Parallel()

		cleared := false
		client := &UniversalMockClient{}
		client.ListStatisticIDsFn = func(_ context.Context, _ string) ([]homeassistant.StatisticMeta, error) {
			return nil, fmt.Errorf("ws not connected")
		}
		client.ClearStatisticsFn = func(_ context.Context, _ []string) error {
			cleared = true
			return nil
		}

		result, err := (&StatisticsHandlers{}).handleManageStatistics(context.Background(), client, map[string]any{
			"action":        statActionClear,
			"statistic_ids": []any{"sensor.mcptest_a"},
			"dry_run":       true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected IsError result: %s", result.Content[0].Text)
		}
		if cleared {
			t.Error("dry_run must not call ClearStatistics even when the pre-check fails")
		}
		assertContainsAll(t, result.Content[0].Text, []string{"Would attempt to clear statistics for 1", "WARNING", "could not verify"})
	})
}

func TestManageStatistics_ValidateAction(t *testing.T) {
	t.Parallel()

	h := &StatisticsHandlers{}

	t.Run("issues grouped in natural output", func(t *testing.T) {
		t.Parallel()

		client := &UniversalMockClient{}
		client.ValidateStatisticsFn = func(_ context.Context) (map[string][]homeassistant.StatisticValidationIssue, error) {
			return map[string][]homeassistant.StatisticValidationIssue{
				"sensor.mcptest_orphaned": {
					{Type: "no_state", Data: map[string]any{"start": "2026-01-01T00:00:00+00:00"}},
				},
				"sensor.mcptest_changing": {
					{Type: "units_changed", Data: map[string]any{"statistic_unit": "kWh", "state_unit": "Wh"}},
				},
			}, nil
		}

		result, err := h.handleManageStatistics(context.Background(), client, map[string]any{"action": statActionValidate})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected IsError result: %s", result.Content[0].Text)
		}
		content := result.Content[0].Text
		assertContainsAll(t, content, []string{
			"sensor.mcptest_orphaned", "no_state",
			"sensor.mcptest_changing", "units_changed",
			"statistic_unit", "2026-01-01T00:00:00+00:00",
		})
	})

	t.Run("no issues message", func(t *testing.T) {
		t.Parallel()

		client := &UniversalMockClient{}
		client.ValidateStatisticsFn = func(_ context.Context) (map[string][]homeassistant.StatisticValidationIssue, error) {
			return map[string][]homeassistant.StatisticValidationIssue{}, nil
		}

		result, err := h.handleManageStatistics(context.Background(), client, map[string]any{"action": statActionValidate})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected IsError result: %s", result.Content[0].Text)
		}
		assertContainsAll(t, result.Content[0].Text, []string{"no issues"})
	})

	t.Run("client error surfaces as IsError", func(t *testing.T) {
		t.Parallel()

		client := &UniversalMockClient{}
		client.ValidateStatisticsFn = func(_ context.Context) (map[string][]homeassistant.StatisticValidationIssue, error) {
			return nil, fmt.Errorf("recorder not running")
		}

		result, err := h.handleManageStatistics(context.Background(), client, map[string]any{"action": statActionValidate})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Fatalf("want IsError, got: %s", result.Content[0].Text)
		}
		assertContainsAll(t, result.Content[0].Text, []string{"recorder not running"})
	})

	t.Run("limit truncates issue ids with notice, issue and id counts kept separate (issue W1)", func(t *testing.T) {
		t.Parallel()

		client := &UniversalMockClient{}
		client.ValidateStatisticsFn = func(_ context.Context) (map[string][]homeassistant.StatisticValidationIssue, error) {
			return map[string][]homeassistant.StatisticValidationIssue{
				// sensor.mcptest_a carries 2 issues so the shown-issue count
				// (2) differs from the shown-id count (1) - a header that
				// conflated the two populations (issue W1) would misreport
				// one of them.
				"sensor.mcptest_a": {{Type: "no_state", Data: nil}, {Type: "units_changed", Data: nil}},
				"sensor.mcptest_b": {{Type: "no_state", Data: nil}},
			}, nil
		}

		result, err := h.handleManageStatistics(context.Background(), client, map[string]any{
			"action": statActionValidate,
			"limit":  float64(1),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected IsError result: %s", result.Content[0].Text)
		}
		content := result.Content[0].Text
		assertContainsAll(t, content, []string{
			"3 issues", "2 statistic ids", "showing 2 issues / 1 id", "sensor.mcptest_a",
		})
		assertNotContainsAny(t, content, []string{"sensor.mcptest_b"})
	})

	t.Run("limit truncates in json format with metadata", func(t *testing.T) {
		t.Parallel()

		client := &UniversalMockClient{}
		client.ValidateStatisticsFn = func(_ context.Context) (map[string][]homeassistant.StatisticValidationIssue, error) {
			return map[string][]homeassistant.StatisticValidationIssue{
				"sensor.mcptest_a": {{Type: "no_state", Data: nil}, {Type: "units_changed", Data: nil}},
				"sensor.mcptest_b": {{Type: "no_state", Data: nil}},
			}, nil
		}

		result, err := h.handleManageStatistics(context.Background(), client, map[string]any{
			"action": statActionValidate,
			"limit":  float64(1),
			"format": formatJSON,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected IsError result: %s", result.Content[0].Text)
		}

		var parsed statisticValidateJSONResponse
		if err := json.Unmarshal([]byte(result.Content[0].Text), &parsed); err != nil {
			t.Fatalf("output is not the expected JSON shape: %v\n%s", err, result.Content[0].Text)
		}
		if parsed.TotalIDs != 2 || parsed.ReturnedIDs != 1 || !parsed.Truncated {
			t.Errorf("got (total_ids=%d, returned_ids=%d, truncated=%v), want (2, 1, true)",
				parsed.TotalIDs, parsed.ReturnedIDs, parsed.Truncated)
		}
		if parsed.TotalIssues != 3 || parsed.ReturnedIssues != 2 {
			t.Errorf("got (total_issues=%d, returned_issues=%d), want (3, 2)",
				parsed.TotalIssues, parsed.ReturnedIssues)
		}
	})

	t.Run("no limit arg applies default cap (issue C1)", func(t *testing.T) {
		t.Parallel()

		client := &UniversalMockClient{}
		client.ValidateStatisticsFn = func(_ context.Context) (map[string][]homeassistant.StatisticValidationIssue, error) {
			return statTestManyIssues(defaultStatisticListLimit + 50), nil
		}

		result, err := h.handleManageStatistics(context.Background(), client, map[string]any{"action": statActionValidate})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected IsError result: %s", result.Content[0].Text)
		}
		content := result.Content[0].Text
		assertContainsAll(t, content, []string{
			fmt.Sprintf("%d statistic ids", defaultStatisticListLimit+50),
			fmt.Sprintf("showing %d issues / %d ids", defaultStatisticListLimit, defaultStatisticListLimit),
		})
	})

	t.Run("explicit limit 0 returns everything (issue C1)", func(t *testing.T) {
		t.Parallel()

		total := defaultStatisticListLimit + 50
		client := &UniversalMockClient{}
		client.ValidateStatisticsFn = func(_ context.Context) (map[string][]homeassistant.StatisticValidationIssue, error) {
			return statTestManyIssues(total), nil
		}

		result, err := h.handleManageStatistics(context.Background(), client, map[string]any{
			"action": statActionValidate,
			"limit":  float64(0),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected IsError result: %s", result.Content[0].Text)
		}
		content := result.Content[0].Text
		assertContainsAll(t, content, []string{fmt.Sprintf("%d statistic ids", total)})
		assertNotContainsAny(t, content, []string{"showing"})
	})
}

func TestFormatStatisticListNatural(t *testing.T) {
	t.Parallel()

	t.Run("legacy has_mean/has_sum shape", func(t *testing.T) {
		t.Parallel()

		metas := statTestLegacyMeta()
		out := formatStatisticListNatural(metas, len(metas))
		assertContainsAll(t, out, []string{
			"- sensor.mcptest_battery (Battery)",
			"recorder",
			"mV",
			"mean",
		})
		// energy is has_mean=false / has_sum=true
		if !strings.Contains(out, "sensor.mcptest_energy") {
			t.Errorf("missing sensor.mcptest_energy in output:\n%s", out)
		}
		energyLine := out[strings.Index(out, "sensor.mcptest_energy"):]
		if strings.Contains(energyLine, "mean") {
			t.Errorf("energy entry should not claim mean capability:\n%s", energyLine)
		}
		if !strings.Contains(energyLine, "sum") {
			t.Errorf("energy entry should claim sum capability:\n%s", energyLine)
		}
	})

	t.Run("modern mean_type-only shape", func(t *testing.T) {
		t.Parallel()

		metas := statTestModernMeta()
		out := formatStatisticListNatural(metas, len(metas))
		assertContainsAll(t, out, []string{"- sensor.mcptest_battery", "mean"})
	})

	t.Run("modern mean_type circular and none", func(t *testing.T) {
		t.Parallel()

		metas := []homeassistant.StatisticMeta{
			{
				StatisticID: "sensor.circular",
				Source:      "recorder",
				MeanType:    statTestInt(2), // CIRCULAR
			},
			{
				StatisticID: "sensor.none",
				Source:      "recorder",
				MeanType:    statTestInt(0), // NONE
			},
		}
		out := formatStatisticListNatural(metas, len(metas))
		assertContainsAll(t, out, []string{"- sensor.circular", "circular mean", "- sensor.none"})
		noneLine := out[strings.Index(out, "sensor.none"):]
		if strings.Contains(noneLine, "mean") {
			t.Errorf("none entry should not claim mean capability:\n%s", noneLine)
		}
	})

	t.Run("display_unit fallback when statistics_unit is nil", func(t *testing.T) {
		t.Parallel()

		metas := []homeassistant.StatisticMeta{
			{
				StatisticID: "sensor.display_only",
				Source:      "recorder",
				DisplayUnit: statTestStr("W"),
			},
		}
		out := formatStatisticListNatural(metas, len(metas))
		assertContainsAll(t, out, []string{"- sensor.display_only", "unit: W"})
	})

	t.Run("display_unit fallback when statistics_unit is empty", func(t *testing.T) {
		t.Parallel()

		metas := []homeassistant.StatisticMeta{
			{
				StatisticID:    "sensor.empty_stat_unit",
				Source:         "recorder",
				StatisticsUnit: statTestStr(""),
				DisplayUnit:    statTestStr("A"),
			},
		}
		out := formatStatisticListNatural(metas, len(metas))
		assertContainsAll(t, out, []string{"- sensor.empty_stat_unit", "unit: A"})
	})

	t.Run("empty list", func(t *testing.T) {
		t.Parallel()

		out := formatStatisticListNatural(nil, 0)
		if !strings.Contains(out, "No statistic ids found") {
			t.Errorf("want empty-list message, got:\n%s", out)
		}
	})

	t.Run("includes friendly name when present", func(t *testing.T) {
		t.Parallel()

		metas := []homeassistant.StatisticMeta{
			{
				StatisticID: "sensor.living_room_temp",
				Name:        statTestStr("Living Room Temperature"),
				Source:      "recorder",
			},
		}

		out := formatStatisticListNatural(metas, len(metas))
		assertContainsAll(t, out, []string{"sensor.living_room_temp (Living Room Temperature)"})
	})

	t.Run("header indicates total when truncated", func(t *testing.T) {
		t.Parallel()

		metas := []homeassistant.StatisticMeta{
			{StatisticID: "sensor.a", Source: "recorder"},
		}

		out := formatStatisticListNatural(metas, 5)
		assertContainsAll(t, out, []string{"Found 5 statistic ids in the recorder database (showing 1):"})
	})

	t.Run("a name containing a newline does not forge an extra list entry", func(t *testing.T) {
		t.Parallel()

		metas := []homeassistant.StatisticMeta{
			{
				StatisticID: "sensor.mcptest_forged",
				Source:      "recorder",
				Name:        statTestStr("evil\n- sensor.thermostat_battery (source: recorder)"),
			},
		}

		out := formatStatisticListNatural(metas, len(metas))
		if strings.Contains(out, "sensor.thermostat_battery (source: recorder)") {
			t.Errorf("newline in name forged an extra list entry:\n%s", out)
		}
		if strings.Count(out, "\n") != 2 {
			t.Errorf("want exactly 2 lines (header + 1 entry), got:\n%s", out)
		}
	})

	t.Run("statistic_id or source containing a newline does not forge an extra line", func(t *testing.T) {
		t.Parallel()

		metas := []homeassistant.StatisticMeta{
			{
				StatisticID: "sensor.mcptest_forged\n- sensor.thermostat_battery (source: recorder)",
				Source:      "recorder\nfake_source",
			},
		}

		out := formatStatisticListNatural(metas, len(metas))
		if strings.Contains(out, "sensor.thermostat_battery (source: recorder)") {
			t.Errorf("newline in statistic_id forged an extra line:\n%s", out)
		}
		if strings.Contains(out, "fake_source") == false {
			t.Errorf("expected sanitized source text to still be present on one line:\n%s", out)
		}
		if strings.Count(out, "\n") != 2 {
			t.Errorf("want exactly 2 lines (header + 1 entry), got:\n%s", out)
		}
	})
}

func TestFormatStatisticListJSON(t *testing.T) {
	t.Parallel()

	t.Run("untruncated response shape", func(t *testing.T) {
		t.Parallel()

		metas := statTestLegacyMeta()
		b, err := formatStatisticListJSON(metas, len(metas), false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var parsed statisticListJSONResponse
		if err := json.Unmarshal([]byte(b), &parsed); err != nil {
			t.Fatalf("output is not the expected JSON shape: %v\n%s", err, b)
		}
		if len(parsed.StatisticIDs) != 2 {
			t.Fatalf("got %d entries, want 2", len(parsed.StatisticIDs))
		}
		if parsed.StatisticIDs[0].StatisticID != "sensor.mcptest_battery" {
			t.Errorf("StatisticIDs[0].StatisticID = %v, want sensor.mcptest_battery", parsed.StatisticIDs[0].StatisticID)
		}
		if parsed.Total != 2 || parsed.Returned != 2 || parsed.Truncated {
			t.Errorf("got (total=%d, returned=%d, truncated=%v), want (2, 2, false)",
				parsed.Total, parsed.Returned, parsed.Truncated)
		}
	})

	t.Run("truncated response carries total and truncated flag", func(t *testing.T) {
		t.Parallel()

		metas := statTestLegacyMeta()
		b, err := formatStatisticListJSON(metas[:1], len(metas), true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var parsed statisticListJSONResponse
		if err := json.Unmarshal([]byte(b), &parsed); err != nil {
			t.Fatalf("output is not the expected JSON shape: %v\n%s", err, b)
		}
		if parsed.Total != 2 || parsed.Returned != 1 || !parsed.Truncated {
			t.Errorf("got (total=%d, returned=%d, truncated=%v), want (2, 1, true)",
				parsed.Total, parsed.Returned, parsed.Truncated)
		}
	})
}

func TestFormatValidateNatural(t *testing.T) {
	t.Parallel()

	t.Run("groups issues by type with orphan hint", func(t *testing.T) {
		t.Parallel()

		issues := map[string][]homeassistant.StatisticValidationIssue{
			"sensor.mcptest_orphaned": {
				{Type: "no_state", Data: map[string]any{"start": "2026-01-01T00:00:00+00:00"}},
			},
			"sensor.mcptest_changing": {
				{Type: "units_changed", Data: map[string]any{"statistic_unit": "kWh", "state_unit": "Wh"}},
			},
		}
		_, summary := applyValidateLimit(issues, 0)
		out := formatValidateNatural(issues, summary)
		assertContainsAll(t, out, []string{
			"sensor.mcptest_orphaned",
			"no_state",
			"no longer exists",
			"sensor.mcptest_changing",
			"units_changed",
			"statistic_unit",
		})
	})

	t.Run("unknown issue types render generically", func(t *testing.T) {
		t.Parallel()

		issues := map[string][]homeassistant.StatisticValidationIssue{
			"sensor.mcptest_weird": {
				{Type: "exotic_issue", Data: nil},
			},
		}
		_, summary := applyValidateLimit(issues, 0)
		out := formatValidateNatural(issues, summary)
		assertContainsAll(t, out, []string{"sensor.mcptest_weird", "exotic_issue"})
	})

	t.Run("empty map", func(t *testing.T) {
		t.Parallel()

		issues := map[string][]homeassistant.StatisticValidationIssue{}
		_, summary := applyValidateLimit(issues, 0)
		out := formatValidateNatural(issues, summary)
		assertContainsAll(t, out, []string{"no issues"})
	})

	t.Run("duplicate issue types on same id are deduplicated in id list", func(t *testing.T) {
		t.Parallel()

		issues := map[string][]homeassistant.StatisticValidationIssue{
			"sensor.duplicate_issues": {
				{Type: "unsupported_unit", Data: map[string]any{"unit": "foo"}},
				{Type: "unsupported_unit", Data: map[string]any{"unit": "bar"}},
			},
		}

		_, summary := applyValidateLimit(issues, 0)
		out := formatValidateNatural(issues, summary)
		// Should count 1 id, not 2 ids
		assertContainsAll(t, out, []string{"unsupported_unit (1 id)", "sensor.duplicate_issues"})
		if strings.Count(out, "sensor.duplicate_issues") != 1 {
			t.Errorf("expected sensor.duplicate_issues to appear once, got output:\n%s", out)
		}
	})

	t.Run("truncated header separates issue and id counts (issue W1)", func(t *testing.T) {
		t.Parallel()

		// The shown map has 1 id with 2 issues; the full population has 5
		// ids and 9 issues. A header that summed issues over the shown map
		// alone (the pre-fix bug) would report "2 issues across 5 ids"
		// instead of correctly attributing all 9 issues to all 5 ids.
		shown := map[string][]homeassistant.StatisticValidationIssue{
			"sensor.mcptest_a": {{Type: "no_state", Data: nil}, {Type: "units_changed", Data: nil}},
		}
		summary := validateSummary{
			ShownIDs: 1, TotalIDs: 5,
			ShownIssues: 2, TotalIssues: 9,
			Truncated: true,
		}
		out := formatValidateNatural(shown, summary)
		assertContainsAll(t, out, []string{"9 issues", "5 statistic ids", "showing 2 issues / 1 id"})
	})

	t.Run("a statistic id containing a newline does not forge an extra line", func(t *testing.T) {
		t.Parallel()

		issues := map[string][]homeassistant.StatisticValidationIssue{
			"sensor.mcptest_forged\n- sensor.thermostat_battery (source: recorder)": {
				{Type: "no_state", Data: nil},
			},
		}
		_, summary := applyValidateLimit(issues, 0)
		out := formatValidateNatural(issues, summary)
		if strings.Contains(out, "sensor.thermostat_battery (source: recorder)") {
			t.Errorf("newline in statistic id forged an extra line:\n%s", out)
		}
	})

	t.Run("issue data value containing a newline does not forge an extra line", func(t *testing.T) {
		t.Parallel()

		issues := map[string][]homeassistant.StatisticValidationIssue{
			"sensor.mcptest_a": {
				{Type: "no_state", Data: map[string]any{"start": "evil\n      fake_key: fake_value"}},
			},
		}
		_, summary := applyValidateLimit(issues, 0)
		out := formatValidateNatural(issues, summary)
		// The forged text may still appear (sanitization collapses the
		// newline to a space, it doesn't remove the value's content) - what
		// must not happen is it landing on its own indented line.
		if strings.Contains(out, "\n      fake_key: fake_value\n") {
			t.Errorf("newline in issue data value forged an extra line:\n%s", out)
		}
		wantLines := 5 // header, blank+type header, id line, data line
		if got := strings.Count(out, "\n"); got != wantLines {
			t.Errorf("want %d newlines (no forged extra line), got %d:\n%s", wantLines, got, out)
		}
	})
}

func TestFormatValidateJSON(t *testing.T) {
	t.Parallel()

	issues := map[string][]homeassistant.StatisticValidationIssue{
		"sensor.mcptest_orphaned": {
			{Type: "no_state", Data: map[string]any{"start": "2026-01-01T00:00:00+00:00"}},
		},
	}
	_, summary := applyValidateLimit(issues, 0)
	b, err := formatValidateJSON(issues, summary)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed statisticValidateJSONResponse
	if err := json.Unmarshal([]byte(b), &parsed); err != nil {
		t.Fatalf("output is not the expected JSON shape: %v\n%s", err, b)
	}
	if parsed.Issues["sensor.mcptest_orphaned"] == nil {
		t.Errorf("missing sensor.mcptest_orphaned key in JSON:\n%s", b)
	}
	if parsed.TotalIDs != 1 || parsed.ReturnedIDs != 1 || parsed.Truncated {
		t.Errorf("got (total=%d, returned=%d, truncated=%v), want (1, 1, false)",
			parsed.TotalIDs, parsed.ReturnedIDs, parsed.Truncated)
	}
	if parsed.TotalIssues != 1 || parsed.ReturnedIssues != 1 {
		t.Errorf("got (total_issues=%d, returned_issues=%d), want (1, 1)",
			parsed.TotalIssues, parsed.ReturnedIssues)
	}
}

func TestApplyStatisticLimit(t *testing.T) {
	t.Parallel()

	metas := statTestLegacyMeta()

	t.Run("limit less than total truncates", func(t *testing.T) {
		t.Parallel()

		sliced, total, truncated := applyStatisticLimit(metas, 1)
		if len(sliced) != 1 || total != 2 || !truncated {
			t.Errorf("got (%d, %d, %v), want (1, 2, true)", len(sliced), total, truncated)
		}
	})

	t.Run("limit equal to total does not truncate", func(t *testing.T) {
		t.Parallel()

		sliced, total, truncated := applyStatisticLimit(metas, 2)
		if len(sliced) != 2 || total != 2 || truncated {
			t.Errorf("got (%d, %d, %v), want (2, 2, false)", len(sliced), total, truncated)
		}
	})

	t.Run("limit zero does not truncate", func(t *testing.T) {
		t.Parallel()

		sliced, total, truncated := applyStatisticLimit(metas, 0)
		if len(sliced) != 2 || total != 2 || truncated {
			t.Errorf("got (%d, %d, %v), want (2, 2, false)", len(sliced), total, truncated)
		}
	})
}

// TestApplyValidateLimit mirrors TestApplyStatisticLimit's boundary coverage
// for the analogous validate-side limiter.
func TestApplyValidateLimit(t *testing.T) {
	t.Parallel()

	issues := map[string][]homeassistant.StatisticValidationIssue{
		"sensor.mcptest_a": {{Type: "no_state", Data: nil}},
		"sensor.mcptest_b": {{Type: "no_state", Data: nil}, {Type: "units_changed", Data: nil}},
	}

	t.Run("limit less than total truncates", func(t *testing.T) {
		t.Parallel()

		limited, summary := applyValidateLimit(issues, 1)
		if len(limited) != 1 || summary.TotalIDs != 2 || !summary.Truncated {
			t.Errorf("got (len=%d, totalIDs=%d, truncated=%v), want (1, 2, true)",
				len(limited), summary.TotalIDs, summary.Truncated)
		}
		if summary.TotalIssues != 3 {
			t.Errorf("summary.TotalIssues = %d, want 3", summary.TotalIssues)
		}
		if summary.ShownIssues != countIssues(limited) {
			t.Errorf("summary.ShownIssues = %d, want %d (countIssues(limited))", summary.ShownIssues, countIssues(limited))
		}
	})

	t.Run("limit equal to total does not truncate", func(t *testing.T) {
		t.Parallel()

		limited, summary := applyValidateLimit(issues, 2)
		if len(limited) != 2 || summary.TotalIDs != 2 || summary.Truncated {
			t.Errorf("got (len=%d, totalIDs=%d, truncated=%v), want (2, 2, false)",
				len(limited), summary.TotalIDs, summary.Truncated)
		}
		if summary.ShownIssues != 3 || summary.TotalIssues != 3 {
			t.Errorf("got (shownIssues=%d, totalIssues=%d), want (3, 3)", summary.ShownIssues, summary.TotalIssues)
		}
	})

	t.Run("limit zero does not truncate", func(t *testing.T) {
		t.Parallel()

		limited, summary := applyValidateLimit(issues, 0)
		if len(limited) != 2 || summary.TotalIDs != 2 || summary.Truncated {
			t.Errorf("got (len=%d, totalIDs=%d, truncated=%v), want (2, 2, false)",
				len(limited), summary.TotalIDs, summary.Truncated)
		}
	})
}

// TestCountIssues pins countIssues' boundary at an empty map (returns 0, not
// a nil-map panic) alongside the multi-id summation the other tests already
// exercise indirectly through applyValidateLimit.
func TestCountIssues(t *testing.T) {
	t.Parallel()

	if got := countIssues(map[string][]homeassistant.StatisticValidationIssue{}); got != 0 {
		t.Errorf("countIssues(empty) = %d, want 0", got)
	}

	issues := map[string][]homeassistant.StatisticValidationIssue{
		"sensor.mcptest_a": {{Type: "no_state"}},
		"sensor.mcptest_b": {{Type: "no_state"}, {Type: "units_changed"}},
	}
	if got := countIssues(issues); got != 3 {
		t.Errorf("countIssues(issues) = %d, want 3", got)
	}
}

// TestValidateLimitArg pins the boundary between "no limit" (0, or the key
// absent) and a rejected value, plus the non-numeric/negative/non-integer
// rejection cases.
func TestValidateLimitArg(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    map[string]any
		wantErr bool
	}{
		{name: "key absent", args: map[string]any{}, wantErr: false},
		{name: "zero is valid (unlimited)", args: map[string]any{"limit": float64(0)}, wantErr: false},
		{name: "positive integer is valid", args: map[string]any{"limit": float64(10)}, wantErr: false},
		{name: "negative is rejected", args: map[string]any{"limit": float64(-1)}, wantErr: true},
		{name: "non-integer is rejected", args: map[string]any{"limit": float64(1.5)}, wantErr: true},
		{name: "non-numeric is rejected", args: map[string]any{"limit": "10"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateLimitArg(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateLimitArg(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
		})
	}
}

// TestResolveLimitArg pins the C1 fix's key distinction: an absent limit key
// gets defaultStatisticListLimit, while an explicit 0 keeps meaning
// "unlimited" (getInt alone can't tell these apart, since it returns 0 for
// both).
func TestResolveLimitArg(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args map[string]any
		want int
	}{
		{name: "key absent gets the default", args: map[string]any{}, want: defaultStatisticListLimit},
		{name: "explicit zero stays unlimited", args: map[string]any{"limit": float64(0)}, want: 0},
		{name: "explicit positive value passes through", args: map[string]any{"limit": float64(5)}, want: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := resolveLimitArg(tt.args); got != tt.want {
				t.Errorf("resolveLimitArg(%v) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}

// TestResolveKnownStatisticIDs covers the known/unknown split action=clear
// relies on (issue W2), plus the checked=false degradation path on a
// pre-check fetch failure.
func TestResolveKnownStatisticIDs(t *testing.T) {
	t.Parallel()

	t.Run("splits known from unknown", func(t *testing.T) {
		t.Parallel()

		client := &UniversalMockClient{}
		client.ListStatisticIDsFn = func(_ context.Context, _ string) ([]homeassistant.StatisticMeta, error) {
			return statTestMetaForIDs("sensor.mcptest_a"), nil
		}

		known, unknown, checked := resolveKnownStatisticIDs(context.Background(), client, []string{"sensor.mcptest_a", "sensor.mcptest_b"})
		if !checked {
			t.Fatal("checked = false, want true")
		}
		if len(known) != 1 || known[0] != "sensor.mcptest_a" {
			t.Errorf("known = %v, want [sensor.mcptest_a]", known)
		}
		if len(unknown) != 1 || unknown[0] != "sensor.mcptest_b" {
			t.Errorf("unknown = %v, want [sensor.mcptest_b]", unknown)
		}
	})

	t.Run("fetch failure reports checked=false", func(t *testing.T) {
		t.Parallel()

		client := &UniversalMockClient{}
		client.ListStatisticIDsFn = func(_ context.Context, _ string) ([]homeassistant.StatisticMeta, error) {
			return nil, fmt.Errorf("ws not connected")
		}

		known, unknown, checked := resolveKnownStatisticIDs(context.Background(), client, []string{"sensor.mcptest_a"})
		if checked {
			t.Fatal("checked = true, want false on fetch failure")
		}
		if known != nil || unknown != nil {
			t.Errorf("got known=%v unknown=%v, want both nil when checked=false", known, unknown)
		}
	})
}

// TestFormatClearMessage pins the three shapes formatClearMessage can
// produce: all known, a known/unknown mix, and all unknown (which must not
// leave a dangling "for 0 ids: " with an empty list, issue W2).
func TestFormatClearMessage(t *testing.T) {
	t.Parallel()

	t.Run("all known", func(t *testing.T) {
		t.Parallel()

		got := formatClearMessage("Cleared", []string{"sensor.mcptest_a", "sensor.mcptest_b"}, nil)
		assertContainsAll(t, got, []string{"Cleared statistics for 2 statistic ids: sensor.mcptest_a, sensor.mcptest_b"})
		assertNotContainsAny(t, got, []string{"skipped"})
	})

	t.Run("mixed known and unknown", func(t *testing.T) {
		t.Parallel()

		got := formatClearMessage("Would clear", []string{"sensor.mcptest_a"}, []string{"sensor.mcptest_b"})
		assertContainsAll(t, got, []string{
			"Would clear statistics for 1 statistic id: sensor.mcptest_a",
			"1 id not present in the recorder database (skipped): sensor.mcptest_b",
		})
	})

	t.Run("all unknown", func(t *testing.T) {
		t.Parallel()

		got := formatClearMessage("Cleared", nil, []string{"sensor.mcptest_a"})
		assertContainsAll(t, got, []string{
			"Cleared statistics for 0 statistic ids: none of the requested ids were present in the recorder database.",
			"1 id not present in the recorder database (skipped): sensor.mcptest_a",
		})
	})
}

// TestParseStatisticIDsStrict_MaxBoundary pins the boundary between an
// accepted batch (exactly maxClearStatisticIDs) and a rejected one (one
// more), matching the same off-by-one guard style as
// TestApplyStatisticLimit's boundary case.
func TestParseStatisticIDsStrict_MaxBoundary(t *testing.T) {
	t.Parallel()

	t.Run("exactly max ids accepted", func(t *testing.T) {
		t.Parallel()

		ids := make([]any, maxClearStatisticIDs)
		for i := range ids {
			ids[i] = fmt.Sprintf("sensor.mcptest_%d", i)
		}

		got, err := parseStatisticIDsStrict(map[string]any{"statistic_ids": ids})
		if err != nil {
			t.Fatalf("unexpected error at exactly max: %v", err)
		}
		if len(got) != maxClearStatisticIDs {
			t.Errorf("got %d ids, want %d", len(got), maxClearStatisticIDs)
		}
	})

	t.Run("one over max rejected", func(t *testing.T) {
		t.Parallel()

		ids := make([]any, maxClearStatisticIDs+1)
		for i := range ids {
			ids[i] = fmt.Sprintf("sensor.mcptest_%d", i)
		}

		_, err := parseStatisticIDsStrict(map[string]any{"statistic_ids": ids})
		if err == nil {
			t.Fatal("want error at max+1, got nil")
		}
	})
}
