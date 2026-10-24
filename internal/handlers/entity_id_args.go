package handlers

import "fmt"

const (
	// attrEntityIDs is the tool argument carrying a list of entity IDs.
	attrEntityIDs = "entity_ids"

	// maxEntityIDsPerCall bounds how many entity IDs one call may supply, so a
	// single request cannot build an unbounded upstream query.
	maxEntityIDsPerCall = 100

	// bothEntityIDArgsMsg is user-facing text, identical to get_state's.
	bothEntityIDArgsMsg = "Cannot specify both entity_id and entity_ids. Use one or the other."
)

// userMessageError is an error whose text is shown to the tool caller as-is, so
// it keeps sentence capitalisation and punctuation that Go error strings avoid.
type userMessageError string

func (e userMessageError) Error() string { return string(e) }

// parseNonEmptyStringArray requires raw to be a non-empty array whose elements
// are all non-empty strings. key only names the argument in error messages.
func parseNonEmptyStringArray(key string, raw any) ([]string, error) {
	arr, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", key)
	}
	if len(arr) == 0 {
		return nil, fmt.Errorf("%s array cannot be empty", key)
	}
	out := make([]string, 0, len(arr))
	for i, el := range arr {
		s, ok := el.(string)
		if !ok || s == "" {
			return nil, fmt.Errorf("%s[%d] must be a non-empty string, got %T", key, i, el)
		}
		out = append(out, s)
	}
	return out, nil
}

// parseStrictEntityIDList parses an entity ID array strictly: every element
// must be a non-empty string, at most maxEntityIDsPerCall elements may be
// supplied (counted before deduplication), and duplicates are dropped keeping
// the first occurrence's position.
func parseStrictEntityIDList(key string, raw any) ([]string, error) {
	ids, err := parseNonEmptyStringArray(key, raw)
	if err != nil {
		return nil, err
	}
	if len(ids) > maxEntityIDsPerCall {
		return nil, fmt.Errorf("%s has %d elements; at most %d are allowed per call", key, len(ids), maxEntityIDsPerCall)
	}
	seen := make(map[string]struct{}, len(ids))
	unique := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique, nil
}

// parseEntityIDOrIDs reads the entity_id / entity_ids argument pair. Absent
// placeholders (see isAbsentArg) count as not supplied; supplying neither
// returns nil, nil. Supplying both is an error, as is a malformed value.
func parseEntityIDOrIDs(args map[string]any) ([]string, error) {
	single, hasSingle := args[attrEntityID]
	multi, hasMulti := args[attrEntityIDs]
	hasSingle = hasSingle && !isAbsentArg(single)
	hasMulti = hasMulti && !isAbsentArg(multi)

	switch {
	case hasSingle && hasMulti:
		return nil, userMessageError(bothEntityIDArgsMsg)
	case hasSingle:
		id, ok := single.(string)
		if !ok {
			return nil, fmt.Errorf("invalid entity_id: expected a string, got %T", single)
		}
		return []string{id}, nil
	case hasMulti:
		return parseStrictEntityIDList(attrEntityIDs, multi)
	default:
		return nil, nil
	}
}
