package homeassistant

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// maxEchoedFieldNameLen and maxEchoedFieldCount bound BoundedFieldList's
// rendering of caller-supplied field names: those names are arbitrary JSON
// object keys nothing in the stack validates or length-limits before they
// reach LLM-facing text (manage_helper's update success message, and the
// options/config flow "field(s) X were not accepted" errors).
const (
	maxEchoedFieldNameLen = 60
	maxEchoedFieldCount   = 20
)

// BoundedFieldList renders a list of caller-supplied field names for
// inclusion in tool-facing text, bounding both the total count
// (maxEchoedFieldCount, with an "and N more" marker) and each individual
// name's length (maxEchoedFieldNameLen, truncated on whole runes). Every
// name is rendered with %q, which escapes newlines and other control
// characters - a hostile or malformed field name must not be able to inject
// content that reads as new instructions into output the calling model
// treats as trusted.
//
// The caller is responsible for sorting names first if a stable order
// matters; this function preserves input order.
func BoundedFieldList(names []string) string {
	shown := names
	omitted := 0
	// `>` vs `>=` here is a proven-equivalent mutant (verified by hand): at
	// len(shown) == maxEchoedFieldCount exactly, shown[:maxEchoedFieldCount]
	// is a no-op slice of an already-that-length slice and omitted computes
	// to 0 either way, so which branch runs makes no observable difference.
	// No test can kill it - both forms produce byte-identical output for
	// every input.
	if len(shown) > maxEchoedFieldCount { //mutest:skip
		omitted = len(shown) - maxEchoedFieldCount
		shown = shown[:maxEchoedFieldCount]
	}
	rendered := make([]string, len(shown))
	for i, name := range shown {
		rendered[i] = fmt.Sprintf("%q", truncateRunes(name, maxEchoedFieldNameLen))
	}
	joined := strings.Join(rendered, ", ")
	if omitted > 0 {
		joined = fmt.Sprintf("%s, and %d more", joined, omitted)
	}
	return joined
}

// truncateRunes truncates s to at most maxRunes runes, appending "..." when
// truncated. Slicing a string by byte index (s[:n]) can split a multi-byte
// UTF-8 rune in half, producing invalid UTF-8 - this counts runes instead.
func truncateRunes(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxRunes]) + "..."
}
