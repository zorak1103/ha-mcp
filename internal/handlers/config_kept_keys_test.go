package handlers

import "testing"

func TestKeptKeysNote(t *testing.T) {
	t.Parallel()

	owned := map[string]struct{}{"alias": {}, "mode": {}}
	tests := []struct {
		name     string
		cfg      any
		replaced []string
		want     string
	}{
		{
			name: "nothing outside owned keys",
			cfg:  map[string]any{"alias": "a", "mode": "single"},
			want: "",
		},
		{
			name: "extras are listed sorted",
			cfg:  map[string]any{"alias": "a", "trace": 1, "variables": 2, "initial_state": true},
			want: " (kept from previous config: initial_state, trace, variables)",
		},
		{
			name:     "replaced key is dropped",
			cfg:      map[string]any{"alias": "a", "trace": 1, "variables": 2},
			replaced: []string{"variables"},
			want:     " (kept from previous config: trace)",
		},
		{
			name:     "only replaced extras leaves no note",
			cfg:      map[string]any{"alias": "a", "variables": 2},
			replaced: []string{"variables"},
			want:     "",
		},
		{
			name: "unmarshalable value yields no note",
			cfg:  map[string]any{"trace": make(chan int)},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := keptKeysNote(tt.cfg, owned, tt.replaced...); got != tt.want {
				t.Errorf("keptKeysNote() = %q, want %q", got, tt.want)
			}
		})
	}
}
