package handlers

import (
	"encoding/json"
	"slices"
	"strings"
)

// automationOwnedKeys are the top-level automation keys manage_automation
// update has an argument for. Every other key of the stored config is carried
// over unchanged by update.
var automationOwnedKeys = map[string]struct{}{
	"id": {}, "alias": {}, "description": {}, "mode": {}, "max": {},
	"triggers": {}, "conditions": {}, "actions": {},
}

// scriptOwnedKeys are the top-level script keys manage_script update has an
// argument for. Every other key of the stored config is carried over unchanged
// by update.
var scriptOwnedKeys = map[string]struct{}{
	"alias": {}, "description": {}, "mode": {}, "max": {},
	"icon": {}, "fields": {}, "sequence": {},
}

// keptKeysNote lists the top-level keys of the written config that update
// carried over without the caller naming them: every key of cfg except
// ownedKeys and replaced. It returns "" when nothing is left, or when cfg
// cannot be marshaled (the note is informational; the write already succeeded).
func keptKeysNote(cfg any, ownedKeys map[string]struct{}, replaced ...string) string {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return ""
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return ""
	}
	var kept []string
	for key := range top {
		if _, owned := ownedKeys[key]; owned || slices.Contains(replaced, key) {
			continue
		}
		kept = append(kept, key)
	}
	if len(kept) == 0 {
		return ""
	}
	slices.Sort(kept)
	return " (kept from previous config: " + strings.Join(kept, ", ") + ")"
}
