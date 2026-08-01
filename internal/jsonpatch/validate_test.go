package jsonpatch

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ops     []Operation
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty ops",
			ops:     []Operation{},
			wantErr: false,
		},
		{
			name: "valid add",
			ops:  []Operation{{Op: "add", Path: "/foo", Value: "bar"}},
		},
		{
			name: "valid remove",
			ops:  []Operation{{Op: "remove", Path: "/foo"}},
		},
		{
			name: "valid replace",
			ops:  []Operation{{Op: "replace", Path: "/foo", Value: 42.0}},
		},
		{
			name: "valid move",
			ops:  []Operation{{Op: "move", From: "/foo", Path: "/bar"}},
		},
		{
			name: "valid copy",
			ops:  []Operation{{Op: "copy", From: "/foo", Path: "/bar"}},
		},
		{
			name: "valid test",
			ops:  []Operation{{Op: "test", Path: "/foo", Value: "val"}},
		},
		{
			name: "multiple valid ops",
			ops: []Operation{
				{Op: "replace", Path: "/mode", Value: "queued"},
				{Op: "add", Path: "/actions/-", Value: map[string]any{"action": "light.turn_on"}},
				{Op: "remove", Path: "/conditions/0"},
			},
		},
		{
			name:    "invalid op",
			ops:     []Operation{{Op: "update", Path: "/foo"}},
			wantErr: true,
			errMsg:  "invalid operation",
		},
		{
			name:    "empty op",
			ops:     []Operation{{Op: "", Path: "/foo"}},
			wantErr: true,
			errMsg:  "op is required",
		},
		{
			name:    "move missing from",
			ops:     []Operation{{Op: "move", Path: "/bar"}},
			wantErr: true,
			errMsg:  "from is required",
		},
		{
			name:    "copy missing from",
			ops:     []Operation{{Op: "copy", Path: "/bar"}},
			wantErr: true,
			errMsg:  "from is required",
		},
		{
			name:    "index out of range in error message",
			ops:     []Operation{{Op: "invalid_op", Path: "/foo"}},
			wantErr: true,
			errMsg:  "index 0",
		},
		{
			name: "second op invalid",
			ops: []Operation{
				{Op: "remove", Path: "/foo"},
				{Op: "badop", Path: "/bar"},
			},
			wantErr: true,
			errMsg:  "index 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := Validate(tt.ops)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %q, want to contain %q", err, tt.errMsg)
				}
			}
		})
	}
}
