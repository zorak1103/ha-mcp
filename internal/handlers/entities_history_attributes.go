package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/zorak1103/ha-mcp/internal/handlers/formatter"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

const (
	// attributeHistoryDefaultLimit caps the change rows per entity when limit is not given.
	attributeHistoryDefaultLimit = 50
	// attributeValueMaxLen is the longest value rendered before it is cut with an ellipsis.
	attributeValueMaxLen = 80
	attributeUnsetText   = "<unset>"

	// attributeNotRecordedHint explains the other reason an attribute can be missing: integrations
	// list attributes the recorder must not store (input_number min/max/step/mode, a climate's
	// preset_modes and hvac_modes, ...), and those never reach the history.
	attributeNotRecordedHint = "the attribute may not exist on this entity, or Home Assistant's recorder may not store it (integrations exclude some attributes from history)"
)

// attributeChange is one transition of a single attribute in an entity's history.
// Old and New hold display values: nil means the attribute was absent; maps, lists and
// long strings are already rendered as short strings.
type attributeChange struct {
	At    time.Time
	Start bool // the value at the start of the range; Old is unused
	Old   any
	New   any
}

// attributeChangeJSON is one change in json output. Old and New are omitted when the
// attribute was absent.
type attributeChangeJSON struct {
	At    string `json:"at"`
	Start bool   `json:"start,omitempty"`
	Old   any    `json:"old,omitempty"`
	New   any    `json:"new,omitempty"`
}

// attributeSection is the processed attribute history of one requested entity.
type attributeSection struct {
	entityID string
	name     string
	hasRows  bool
	exists   bool // false only when the entity has no rows and GetState cannot find it
	seen     bool // the attribute appears in at least one row
	changes  []attributeChange
	total    int // changes without the start row, before the limit
}

// attributeChanges walks one entity's rows (oldest first) and returns the attribute's value
// at the start followed by every later row where it differs from the row before. seen
// reports whether any row carried the attribute. A missing or null attribute counts as
// unset, so appearing and disappearing are changes.
func attributeChanges(entries []homeassistant.HistoryEntry, attr string) (changes []attributeChange, seen bool) {
	var prev any
	prevSet := false
	for i, entry := range entries {
		cur, set := entry.Attributes[attr]
		set = set && cur != nil
		seen = seen || set

		switch {
		case i == 0:
			changes = append(changes, attributeChange{At: historyUpdatedTime(entry), Start: true, New: attributeDisplayValue(cur)})
		case set != prevSet || !reflect.DeepEqual(cur, prev):
			changes = append(changes, attributeChange{At: historyUpdatedTime(entry), Old: attributeDisplayValue(prev), New: attributeDisplayValue(cur)})
		}
		prev, prevSet = cur, set
	}
	return changes, seen
}

// historyUpdatedTime is when the row was written. For an attribute-only change HA's
// last_changed is the earlier state change, so last_updated is the time that matters.
func historyUpdatedTime(entry homeassistant.HistoryEntry) time.Time {
	if entry.LastUpdated > 0 {
		entry.LastChanged = entry.LastUpdated
	}
	return entry.LastChangedTime()
}

// attributeDisplayValue keeps numbers, booleans and nil as they are and renders everything
// else as a string of at most attributeValueMaxLen characters (maps and lists as compact json).
func attributeDisplayValue(v any) any {
	switch val := v.(type) {
	case nil, bool, float64:
		return v
	case string:
		return truncateAttributeText(val)
	default:
		encoded, err := json.Marshal(val)
		if err != nil {
			return truncateAttributeText(fmt.Sprint(val))
		}
		return truncateAttributeText(string(encoded))
	}
}

func truncateAttributeText(s string) string {
	runes := []rune(s)
	if len(runes) <= attributeValueMaxLen {
		return s
	}
	return string(runes[:attributeValueMaxLen]) + "…"
}

// limitAttributeChanges keeps the newest limit changes. The start row is kept only when
// nothing is cut: once the list is truncated it is far from the window, and the first shown
// change carries the value before it as Old. total counts the changes without the start row.
func limitAttributeChanges(changes []attributeChange, limit int) (shown []attributeChange, total int) {
	for _, change := range changes {
		if !change.Start {
			total++
		}
	}
	if limit <= 0 || total <= limit {
		return changes, total
	}
	return changes[len(changes)-limit:], total
}

// renderAttributeHistory renders mode=history with attribute set: per entity the attribute's
// transitions instead of the state rows.
func renderAttributeHistory(
	ctx context.Context,
	client homeassistant.Client,
	history map[string][]homeassistant.HistoryEntry,
	params *historyParams,
) (string, error) {
	sections := buildAttributeSections(ctx, client, history, params)
	if params.format == formatter.FormatNatural {
		return formatAttributeSectionsNatural(ctx, sections, params), nil
	}
	return formatAttributeSectionsJSON(sections, params)
}

func buildAttributeSections(
	ctx context.Context,
	client homeassistant.Client,
	history map[string][]homeassistant.HistoryEntry,
	params *historyParams,
) []attributeSection {
	limit := params.limit
	if limit == 0 {
		limit = attributeHistoryDefaultLimit
	}

	sections := make([]attributeSection, 0, len(params.entityIDs))
	for _, id := range params.entityIDs {
		rows := history[id]
		section := attributeSection{entityID: id, hasRows: len(rows) > 0, exists: true}
		if !section.hasRows {
			_, err := client.GetState(ctx, id)
			section.exists = err == nil
		}
		for _, row := range rows {
			if name := formatter.GetStringAttr(row.Attributes, "friendly_name"); name != "" {
				section.name = name
				break
			}
		}
		changes, seen := attributeChanges(rows, params.attribute)
		section.seen = seen
		if seen {
			section.changes, section.total = limitAttributeChanges(changes, limit)
		}
		sections = append(sections, section)
	}
	return sections
}

// shownChanges is the number of change rows displayed, without the start row.
func (s attributeSection) shownChanges() int {
	n := len(s.changes)
	if n > 0 && s.changes[0].Start {
		n--
	}
	return n
}

func (s attributeSection) label() string {
	if s.name != "" && s.name != s.entityID {
		return fmt.Sprintf("%s (%s)", s.name, s.entityID)
	}
	return s.entityID
}

func changeNoun(n int) string {
	if n == 1 {
		return "change"
	}
	return "changes"
}

func attributeValueText(v any) string {
	if v == nil {
		return attributeUnsetText
	}
	return fmt.Sprint(v)
}

func formatAttributeSectionsNatural(ctx context.Context, sections []attributeSection, params *historyParams) string {
	parts := make([]string, 0, len(sections))
	for _, section := range sections {
		parts = append(parts, formatAttributeSectionNatural(ctx, section, params.attribute))
	}
	return strings.Join(parts, "\n\n")
}

func formatAttributeSectionNatural(ctx context.Context, section attributeSection, attr string) string {
	if !section.hasRows {
		// The no-history and not-found texts are the ones the state view uses.
		text, err := formatter.NewNaturalFormatter().FormatHistory(ctx, section.entityID, nil, formatter.HistoryOptions{EntityExists: section.exists})
		if err != nil {
			return fmt.Sprintf("No history found for %s.", section.entityID)
		}
		return text
	}
	if !section.seen {
		return fmt.Sprintf("Attribute %s is not present in any history row of %s. %s.", attr, section.label(), attributeNotRecordedHint)
	}

	lines := make([]string, 0, len(section.changes)+2)
	lines = append(lines, fmt.Sprintf("Attribute %s of %s: %d %s.", attr, section.label(), section.total, changeNoun(section.total)))
	for _, change := range section.changes {
		ts := change.At.Format("2006-01-02 15:04")
		if change.Start {
			lines = append(lines, fmt.Sprintf("%s (start) → %s", ts, attributeValueText(change.New)))
			continue
		}
		lines = append(lines, fmt.Sprintf("%s %s → %s", ts, attributeValueText(change.Old), attributeValueText(change.New)))
	}
	text := strings.Join(lines, "\n")
	if shown := section.shownChanges(); section.total > shown {
		text += fmt.Sprintf("\n\n[Showing %d of %d changes. Use limit parameter for more.]", shown, section.total)
	}
	return text
}

func attributeSummaryLine(section attributeSection, attr string) string {
	switch {
	case !section.hasRows:
		line := fmt.Sprintf("No history for %s", section.entityID)
		if !section.exists {
			line += " (entity not found in Home Assistant)"
		}
		return line
	case !section.seen:
		return fmt.Sprintf("Attribute %s not present in any history row of %s (%s)", attr, section.entityID, attributeNotRecordedHint)
	}
	if shown := section.shownChanges(); section.total > shown {
		return fmt.Sprintf("Showing %d of %d changes of attribute %s for %s (limited)", shown, section.total, attr, section.entityID)
	}
	return fmt.Sprintf("Found %d %s of attribute %s for %s", section.total, changeNoun(section.total), attr, section.entityID)
}

func attributeChangesJSON(section attributeSection) []attributeChangeJSON {
	out := make([]attributeChangeJSON, 0, len(section.changes)) // an array, never null
	for _, change := range section.changes {
		out = append(out, attributeChangeJSON{
			At:    change.At.Format(time.RFC3339),
			Start: change.Start,
			Old:   change.Old,
			New:   change.New,
		})
	}
	return out
}

// formatAttributeSectionsJSON mirrors the state view: summary line(s), a blank line, then the
// data. One entity gives a bare array; several give {entity_id, changes} objects in request order.
func formatAttributeSectionsJSON(sections []attributeSection, params *historyParams) (string, error) {
	lines := make([]string, 0, len(sections))
	for _, section := range sections {
		lines = append(lines, attributeSummaryLine(section, params.attribute))
	}
	summary := strings.Join(lines, "\n")

	var data any
	if len(sections) == 1 {
		data = attributeChangesJSON(sections[0])
	} else {
		type block struct {
			EntityID string                `json:"entity_id"`
			Changes  []attributeChangeJSON `json:"changes"`
		}
		blocks := make([]block, 0, len(sections))
		for _, section := range sections {
			blocks = append(blocks, block{EntityID: section.entityID, Changes: attributeChangesJSON(section)})
		}
		data = blocks
	}

	body, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal attribute history: %w", err)
	}
	return summary + "\n\n" + string(body), nil
}
