package homeassistant

import "testing"

func TestSlugify_MatchesLegacy(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"umlaut", "Küche", "kuche"},
		{"collapses separators", "A  -b", "a_b"},
		{"empty", "", ""},
		{"trailing separator", "abc -", "abc"},
		{"leading separator", "  abc", "abc"},
		{"punctuation dropped", "Hello, World!", "hello_world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Slugify(tt.in); got != tt.want {
				t.Errorf("Slugify(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
