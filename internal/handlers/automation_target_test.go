package handlers

import "testing"

func TestResolveAutomationTarget(t *testing.T) {
	t.Parallel()

	const notAutomation = "entity_id \"light.kitchen\" is not an automation: action=get takes the automation as automation_id or entity_id 'automation.<id>'"
	const required = "automation_id is required for get action (or pass entity_id 'automation.<id>')"

	tests := []struct {
		name       string
		hint       string
		args       map[string]any
		wantTarget string
		wantErr    string
	}{
		{name: "automation_id bare", hint: "HINT", args: map[string]any{"automation_id": "morning"}, wantTarget: "morning"},
		{name: "automation_id prefixed", hint: "HINT", args: map[string]any{"automation_id": "automation.morning"}, wantTarget: "automation.morning"},
		{name: "automation_id alias with dot", hint: "HINT", args: map[string]any{"automation_id": "Heating v2.0"}, wantTarget: "Heating v2.0"},
		{name: "entity_id only", hint: "HINT", args: map[string]any{"entity_id": "automation.morning"}, wantTarget: "automation.morning"},
		{name: "bare id and matching entity_id", hint: "HINT", args: map[string]any{"automation_id": "morning", "entity_id": "automation.morning"}, wantTarget: "morning"},
		{name: "prefixed id and matching entity_id", hint: "HINT", args: map[string]any{"automation_id": "automation.morning", "entity_id": "automation.morning"}, wantTarget: "automation.morning"},
		{
			name: "conflicting ids", hint: "HINT",
			args:    map[string]any{"automation_id": "Morning routine", "entity_id": "automation.morning"},
			wantErr: "automation_id \"Morning routine\" conflicts with entity_id \"automation.morning\": pass only one of them",
		},
		{name: "entity_id not an automation", hint: "HINT", args: map[string]any{"entity_id": "light.kitchen"}, wantErr: notAutomation},
		{name: "prefix check before conflict check", hint: "HINT", args: map[string]any{"automation_id": "morning", "entity_id": "light.kitchen"}, wantErr: notAutomation},
		{name: "numeric automation_id", hint: "HINT", args: map[string]any{"automation_id": float64(17)}, wantErr: "automation_id must be a string, got float64"},
		{name: "type errors come first", hint: "HINT", args: map[string]any{"automation_id": float64(17), "entity_id": "automation.x"}, wantErr: "automation_id must be a string, got float64"},
		{name: "boolean entity_id", hint: "HINT", args: map[string]any{"entity_id": true}, wantErr: "entity_id must be a string, got bool"},
		{name: "nothing given", hint: "HINT", args: map[string]any{}, wantErr: required + ". HINT"},
		{name: "placeholders only", hint: "HINT", args: map[string]any{"automation_id": "", "entity_id": nil}, wantErr: required + ". HINT"},
		{name: "empty hint", hint: "", args: map[string]any{}, wantErr: required},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			target, errResult := resolveAutomationTarget("get", tt.hint, tt.args)
			if tt.wantErr != "" {
				if errResult == nil {
					t.Fatalf("expected error %q, got target %q", tt.wantErr, target)
				}
				if !errResult.IsError {
					t.Error("expected IsError to be true")
				}
				if got := errResult.Content[0].Text; got != tt.wantErr {
					t.Errorf("error text = %q, want %q", got, tt.wantErr)
				}
				if target != "" {
					t.Errorf("target = %q, want empty on error", target)
				}
				return
			}
			if errResult != nil {
				t.Fatalf("unexpected error: %s", errResult.Content[0].Text)
			}
			if target != tt.wantTarget {
				t.Errorf("target = %q, want %q", target, tt.wantTarget)
			}
		})
	}
}

func TestStringIDValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		v         any
		wantValue string
		wantErr   string
	}{
		{name: "nil", v: nil},
		{name: "empty string", v: ""},
		{name: "empty array", v: []any{}},
		{name: "string", v: "x", wantValue: "x"},
		{name: "number", v: float64(1), wantErr: "id must be a string, got float64"},
		{name: "map", v: map[string]any{}, wantErr: "id must be a string, got map[string]interface {}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			value, errMsg := stringIDValue("id", tt.v)
			if value != tt.wantValue || errMsg != tt.wantErr {
				t.Errorf("stringIDValue(%v) = (%q, %q), want (%q, %q)", tt.v, value, errMsg, tt.wantValue, tt.wantErr)
			}
		})
	}
}
