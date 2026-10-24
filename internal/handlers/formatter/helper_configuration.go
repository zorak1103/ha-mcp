package formatter

import (
	"slices"
	"strings"
)

// DetailKeyConfiguration is the detail-map key under which get_details stores
// a helper's stored configuration (raw Home Assistant keys mapped to their
// values). The natural renderers show it as a trailing "Configuration:"
// section; the JSON formatter marshals it as a nested object.
const DetailKeyConfiguration = "configuration"

// appendConfigurationSection appends a "Configuration:" section to out when
// detail carries a non-empty map[string]any under DetailKeyConfiguration: one
// "Key: value" line per entry in sorted key order, values rendered by
// FormatDetailValue (newlines sanitized, long values truncated). Entries whose
// value is nil or renders empty are skipped. In every other case, including a
// block whose entries are all skipped, out is returned unchanged.
func appendConfigurationSection(out string, detail map[string]any) string {
	block, ok := detail[DetailKeyConfiguration].(map[string]any)
	if !ok || len(block) == 0 {
		return out
	}

	keys := make([]string, 0, len(block))
	for k := range block {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	var lines strings.Builder
	for _, k := range keys {
		if block[k] == nil {
			continue
		}
		rendered := FormatDetailValue(block[k])
		if rendered == "" {
			continue
		}
		lines.WriteString("\n" + sentenceCaseKey(k) + ": " + rendered)
	}
	if lines.Len() == 0 {
		return out
	}

	return out + "\n\nConfiguration:" + lines.String()
}
