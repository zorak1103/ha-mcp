package handlers

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestConfigWriteGuardError_EntryExists_Proceeds(t *testing.T) {
	client := &UniversalMockClient{
		ConfigFileEntryExistsFn: func(context.Context, string, string) (bool, error) {
			return true, nil
		},
	}

	result := configWriteGuardError(context.Background(), client, "script", "update", "example_toggle", "script.example_toggle", "example_toggle")
	if result != nil {
		t.Fatalf("expected nil (proceed) when the probe confirms the entry exists, got: %s", result.Content[0].Text)
	}
}

func TestConfigWriteGuardError_EntryMissing_Refuses(t *testing.T) {
	client := &UniversalMockClient{
		ConfigFileEntryExistsFn: func(context.Context, string, string) (bool, error) {
			return false, nil
		},
	}

	result := configWriteGuardError(context.Background(), client, "script", "update", "example_toggle", "script.example_toggle", "example_toggle")
	if result == nil {
		t.Fatal("expected a refusal when the probe confirms the entry is missing")
	}
	assertContainsAll(t, result.Content[0].Text, []string{
		"example_toggle",
		"script.example_toggle",
		"scripts.yaml",
		"script.example_toggle_2",
		"script.reload",
	})
}

func TestConfigWriteGuardError_ProbeFails_Proceeds(t *testing.T) {
	client := &UniversalMockClient{
		ConfigFileEntryExistsFn: func(context.Context, string, string) (bool, error) {
			return false, errors.New("connection reset")
		},
	}

	result := configWriteGuardError(context.Background(), client, "automation", "patch", "morning_routine", "automation.morning_routine", "morning_routine")
	if result != nil {
		t.Fatalf("expected nil (graceful degradation) when the probe itself fails, got: %s", result.Content[0].Text)
	}
}

func TestConfigWriteGuardError_UsesActionInMessage(t *testing.T) {
	client := &UniversalMockClient{
		ConfigFileEntryExistsFn: func(context.Context, string, string) (bool, error) {
			return false, nil
		},
	}

	updateResult := configWriteGuardError(context.Background(), client, "script", "update", "x", "script.x", "x")
	patchResult := configWriteGuardError(context.Background(), client, "script", "patch", "x", "script.x", "x")

	assertContainsAll(t, updateResult.Content[0].Text, []string{"cannot update"})
	assertContainsAll(t, patchResult.Content[0].Text, []string{"cannot patch"})
}

func TestConfigWriteGuardError_PatchMessage_MentionsDryRun(t *testing.T) {
	client := &UniversalMockClient{
		ConfigFileEntryExistsFn: func(context.Context, string, string) (bool, error) {
			return false, nil
		},
	}

	result := configWriteGuardError(context.Background(), client, "automation", "patch", "x", "automation.x", "x")
	if !strings.Contains(result.Content[0].Text, "dry_run") {
		t.Errorf("expected the patch refusal to point at dry_run for previewing the intended result, got: %s", result.Content[0].Text)
	}
}

func TestConfigWriteGuardError_DomainSpecificFileNames(t *testing.T) {
	client := &UniversalMockClient{
		ConfigFileEntryExistsFn: func(context.Context, string, string) (bool, error) {
			return false, nil
		},
	}

	tests := []struct {
		domain   string
		wantFile string
	}{
		{"automation", "automations.yaml"},
		{"script", "scripts.yaml"},
		{"scene", "scenes.yaml"},
	}
	for _, tt := range tests {
		result := configWriteGuardError(context.Background(), client, tt.domain, "update", "x", tt.domain+".x", "x")
		if !strings.Contains(result.Content[0].Text, tt.wantFile) {
			t.Errorf("domain %q: expected refusal to name %q, got: %s", tt.domain, tt.wantFile, result.Content[0].Text)
		}
	}
}

func TestConfigWriteGuardError_ProbeReceivesDomainAndConfigID(t *testing.T) {
	var gotDomain, gotConfigID string
	client := &UniversalMockClient{
		ConfigFileEntryExistsFn: func(_ context.Context, domain, configID string) (bool, error) {
			gotDomain, gotConfigID = domain, configID
			return true, nil
		},
	}

	configWriteGuardError(context.Background(), client, "automation", "update", "morning routine (display)", "automation.morning_routine", "morning_routine_actual_id")

	if gotDomain != "automation" {
		t.Errorf("expected probe domain %q, got %q", "automation", gotDomain)
	}
	if gotConfigID != "morning_routine_actual_id" {
		t.Errorf("expected the probe to receive the exact id the write will target, not the display id, got %q", gotConfigID)
	}
}
