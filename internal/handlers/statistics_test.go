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

func TestManageStatisticsToolSchema(t *testing.T) {
	t.Parallel()

	h := NewStatisticsHandlers()
	tool := h.manageStatisticsTool()
	verifyToolSchema(t, tool, toolSchemaExpectation{
		ExpectedName:    "manage_statistics",
		WantDescription: true,
		RequiredParams:  []string{"action"},
		OptionalParams:  []string{"statistic_type", "statistic_ids", "limit", "format"},
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
}

func TestManageStatistics_DispatchValidation(t *testing.T) {
	t.Parallel()

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
		assertContainsAll(t, content, []string{"sensor.mcptest_battery", "Showing 1 of 2"})
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
		assertNotContainsAny(t, content, []string{"Showing"})
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
}

func TestManageStatistics_ClearAction(t *testing.T) {
	t.Parallel()

	t.Run("valid ids forwarded to client", func(t *testing.T) {
		t.Parallel()

		var gotIDs []string
		client := &UniversalMockClient{}
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
}

func TestFormatStatisticListNatural(t *testing.T) {
	t.Parallel()

	t.Run("legacy has_mean/has_sum shape", func(t *testing.T) {
		t.Parallel()

		out := formatStatisticListNatural(statTestLegacyMeta())
		assertContainsAll(t, out, []string{
			"- sensor.mcptest_battery",
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

		out := formatStatisticListNatural(statTestModernMeta())
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
		out := formatStatisticListNatural(metas)
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
		out := formatStatisticListNatural(metas)
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
		out := formatStatisticListNatural(metas)
		assertContainsAll(t, out, []string{"- sensor.empty_stat_unit", "unit: A"})
	})

	t.Run("empty list", func(t *testing.T) {
		t.Parallel()

		out := formatStatisticListNatural(nil)
		if !strings.Contains(out, "No statistic ids found") {
			t.Errorf("want empty-list message, got:\n%s", out)
		}
	})
}

func TestFormatStatisticListJSON(t *testing.T) {
	t.Parallel()

	b, err := formatStatisticListJSON(statTestLegacyMeta())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var entries []map[string]any
	if err := json.Unmarshal([]byte(b), &entries); err != nil {
		t.Fatalf("output is not a JSON array: %v\n%s", err, b)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0]["statistic_id"] != "sensor.mcptest_battery" {
		t.Errorf("entries[0][statistic_id] = %v, want sensor.mcptest_battery", entries[0]["statistic_id"])
	}
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
		out := formatValidateNatural(issues)
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
		out := formatValidateNatural(issues)
		assertContainsAll(t, out, []string{"sensor.mcptest_weird", "exotic_issue"})
	})

	t.Run("empty map", func(t *testing.T) {
		t.Parallel()

		out := formatValidateNatural(map[string][]homeassistant.StatisticValidationIssue{})
		assertContainsAll(t, out, []string{"no issues"})
	})
}

func TestFormatValidateJSON(t *testing.T) {
	t.Parallel()

	issues := map[string][]homeassistant.StatisticValidationIssue{
		"sensor.mcptest_orphaned": {
			{Type: "no_state", Data: map[string]any{"start": "2026-01-01T00:00:00+00:00"}},
		},
	}
	b, err := formatValidateJSON(issues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(b), &parsed); err != nil {
		t.Fatalf("output is not a JSON object: %v\n%s", err, b)
	}
	if parsed["sensor.mcptest_orphaned"] == nil {
		t.Errorf("missing sensor.mcptest_orphaned key in JSON:\n%s", b)
	}
}
