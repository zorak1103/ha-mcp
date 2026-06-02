package jsonpatch

import (
	"fmt"
	"strings"
	"testing"
)

func TestSegments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    string
		want    []string
		wantErr bool
	}{
		{
			name: "empty path is root",
			path: "",
			want: []string{},
		},
		{
			name: "root slash only",
			path: "/",
			want: []string{""},
		},
		{
			name: "single segment",
			path: "/foo",
			want: []string{"foo"},
		},
		{
			name: "multiple segments",
			path: "/foo/bar/baz",
			want: []string{"foo", "bar", "baz"},
		},
		{
			name: "numeric segment",
			path: "/items/2",
			want: []string{"items", "2"},
		},
		{
			name: "array append marker",
			path: "/items/-",
			want: []string{"items", "-"},
		},
		{
			name: "tilde-one escape",
			path: "/a~1b",
			want: []string{"a/b"},
		},
		{
			name: "tilde-zero escape",
			path: "/a~0b",
			want: []string{"a~b"},
		},
		{
			name: "combined escapes",
			path: "/a~0~1b",
			want: []string{"a~/b"},
		},
		{
			name: "nested path",
			path: "/triggers/2/entity_id",
			want: []string{"triggers", "2", "entity_id"},
		},
		{
			name:    "no leading slash",
			path:    "foo",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := Segments(tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Segments(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if len(got) != len(tt.want) {
				t.Fatalf("Segments(%q) = %v (len %d), want %v (len %d)", tt.path, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Segments(%q)[%d] = %q, want %q", tt.path, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestGet(t *testing.T) {
	t.Parallel()

	doc := map[string]any{
		"mode": "single",
		"triggers": []any{
			map[string]any{"trigger": "time", "at": "07:00"},
			map[string]any{"trigger": "state", "entity_id": "binary_sensor.motion"},
		},
		"nested": map[string]any{
			"deep": "value",
		},
	}

	tests := []struct {
		name       string
		path       string
		want       any
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "root returns whole doc",
			path: "",
			want: doc,
		},
		{
			name: "top-level string",
			path: "/mode",
			want: "single",
		},
		{
			name: "array element",
			path: "/triggers/0",
			want: doc["triggers"].([]any)[0],
		},
		{
			name: "nested map key",
			path: "/triggers/1/entity_id",
			want: "binary_sensor.motion",
		},
		{
			name: "deep nested",
			path: "/nested/deep",
			want: "value",
		},
		{
			name:       "missing key",
			path:       "/notexist",
			wantErr:    true,
			wantErrMsg: "available keys: [mode nested triggers]",
		},
		{
			name:    "index out of bounds",
			path:    "/triggers/5",
			wantErr: true,
		},
		{
			name:    "index into non-array",
			path:    "/mode/0",
			wantErr: true,
		},
		{
			name:    "invalid path",
			path:    "noslash",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := Get(doc, tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Get(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
			if tt.wantErr {
				if tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("expected error to contain %q, got %q", tt.wantErrMsg, err.Error())
				}
				return
			}

			if tt.path == "" {
				// Root: just check no error
				return
			}
			// Use string representation for comparison (handles maps and slices).
			gotStr := fmt.Sprintf("%v", got)
			wantStr := fmt.Sprintf("%v", tt.want)
			if gotStr != wantStr {
				t.Errorf("Get(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestEscapeSegment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain segment", "entity_id", "entity_id"},
		{"slash in key", "a/b", "a~1b"},
		{"tilde in key", "a~b", "a~0b"},
		{"tilde-slash combo", "a~/b", "a~0~1b"},
		{"empty string", "", ""},
		{"numeric", "42", "42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := EscapeSegment(tt.in)
			if got != tt.want {
				t.Errorf("EscapeSegment(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestEscapeSegmentRoundTrip(t *testing.T) {
	t.Parallel()

	cases := []string{"entity_id", "a/b", "a~b", "a~/b", "target", "~1crazy~0key"}
	for _, s := range cases {
		escaped := EscapeSegment(s)
		roundtripped := unescapeSegment(escaped)
		if roundtripped != s {
			t.Errorf("round-trip failed for %q: EscapeSegment → %q → unescapeSegment → %q", s, escaped, roundtripped)
		}
	}
}

func TestParseIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		seg     string
		length  int
		want    int
		wantErr bool
	}{
		{"valid index 0", "0", 3, 0, false},
		{"valid index 2", "2", 3, 2, false},
		{"out of bounds", "3", 3, 0, true},
		{"negative", "-1", 3, 0, true},
		{"not a number", "abc", 3, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseIndex(tt.seg, tt.length, "/test")
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseIndex(%q, %d) error = %v, wantErr %v", tt.seg, tt.length, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseIndex(%q, %d) = %d, want %d", tt.seg, tt.length, got, tt.want)
			}
		})
	}
}
