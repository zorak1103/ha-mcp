package homeassistant

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Slugify converts s into a lowercase ASCII slug: accents are stripped,
// letters and digits are kept, runs of whitespace/hyphen/underscore collapse
// into a single underscore, and everything else is dropped. Leading and
// trailing separators are removed.
func Slugify(s string) string {
	// NFD decomposes e.g. u (U+00FC) into u (U+0075) + combining umlaut (U+0308)
	normalized := norm.NFD.String(s)
	var result strings.Builder
	prevUnderscore := false

	for _, r := range normalized {
		if unicode.Is(unicode.Mn, r) {
			continue // skip combining marks (accents, umlauts, etc.)
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			result.WriteRune(unicode.ToLower(r))
			prevUnderscore = false
		} else if unicode.IsSpace(r) || r == '-' || r == '_' {
			if !prevUnderscore && result.Len() > 0 {
				result.WriteRune('_')
				prevUnderscore = true
			}
		}
	}

	return strings.TrimSuffix(result.String(), "_")
}
