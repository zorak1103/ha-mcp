package homeassistant

import (
	"errors"
	"fmt"
	"strings"
)

// errOutcomeUnknown marks a write command that was sent but whose connection
// closed before Home Assistant answered. Such a command is never re-sent
// automatically, because it may already have been applied.
var errOutcomeUnknown = errors.New("the command was sent but the connection closed before Home Assistant answered; it may or may not have been applied - check the current state before retrying")

// replaySafeWSCommands lists the non-"/list" WebSocket commands that only read
// state. Re-sending one after the connection dropped cannot duplicate a side
// effect.
var replaySafeWSCommands = map[string]struct{}{
	"get_states":                        {},
	"history/history_during_period":     {},
	"automation/config":                 {},
	"script/config":                     {},
	"config/entity_registry/get":        {},
	"media_source/browse_media":         {},
	"lovelace/config":                   {},
	"recorder/statistics_during_period": {},
	"recorder/list_statistic_ids":       {},
	"recorder/validate_statistics":      {},
	"get_triggers_for_target":           {},
	"get_conditions_for_target":         {},
	"get_services_for_target":           {},
	"extract_from_target":               {},
	"config_entries/get":                {},
	"config_entries/get_single":         {},
	"hacs/info":                         {},
	"hacs/repository/info":              {},
	"hacs/repository/releases":          {},
	"hacs/repository/release_notes":     {},
	"trace/get":                         {},
	"update/release_notes":              {},
}

// isReplaySafeWSCommand reports whether msgType only reads state and may be
// re-sent after it was written. Every "<domain>/list" command is a read; all
// other names must be listed explicitly. The default is deny: a new read
// command must be added to replaySafeWSCommands to get retries.
func isReplaySafeWSCommand(msgType string) bool {
	if strings.HasSuffix(msgType, "/list") {
		return true
	}
	_, ok := replaySafeWSCommands[msgType]
	return ok
}

// wsRetryPredicate returns the retry predicate for a command of type msgType.
// Replay-safe commands retry on any transient error. Every other command
// retries only when it demonstrably never reached the wire (notSentError).
func wsRetryPredicate(msgType string) func(error) bool {
	if isReplaySafeWSCommand(msgType) {
		return IsRetryableError
	}
	return func(err error) bool {
		var notSent *notSentError
		return errors.As(err, &notSent)
	}
}

// markOutcomeUnknown tags a non-replay-safe command whose connection was lost
// after it had been sent with errOutcomeUnknown (the original error stays in
// the chain). All other errors are returned unchanged.
func markOutcomeUnknown(msgType string, err error) error {
	if err == nil || isReplaySafeWSCommand(msgType) {
		return err
	}
	var notSent *notSentError
	if errors.As(err, &notSent) || !errors.Is(err, errConnectionLost) {
		return err
	}
	return fmt.Errorf("%w: %w", errOutcomeUnknown, err)
}
