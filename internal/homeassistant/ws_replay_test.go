package homeassistant

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestIsReplaySafeWSCommand(t *testing.T) {
	t.Parallel()

	safe := []string{
		"get_states",
		"history/history_during_period",
		"automation/config",
		"script/config",
		"config/entity_registry/get",
		"media_source/browse_media",
		"lovelace/config",
		"recorder/statistics_during_period",
		"recorder/list_statistic_ids",
		"recorder/validate_statistics",
		"get_triggers_for_target",
		"get_conditions_for_target",
		"get_services_for_target",
		"extract_from_target",
		"config_entries/get",
		"config_entries/get_single",
		"hacs/info",
		"hacs/repository/info",
		"hacs/repository/releases",
		"hacs/repository/release_notes",
		"config/entity_registry/list",
		"config/device_registry/list",
		"config/area_registry/list",
		"config/label_registry/list",
		"config/floor_registry/list",
		"zone/list",
		"person/list",
		"tag/list",
		"lovelace/dashboards/list",
		"system_log/list",
		"hacs/repositories/list",
		"hacs/critical/list",
		"input_boolean/list",
	}
	unsafe := []string{
		"call_service",
		"input_boolean/create",
		"input_boolean/update",
		"input_boolean/delete",
		"config/entity_registry/update",
		"config/entity_registry/remove",
		"config/device_registry/update",
		"config/device_registry/remove_config_entry",
		"config/area_registry/create",
		"config/area_registry/update",
		"config/area_registry/delete",
		"config/label_registry/create",
		"config/label_registry/update",
		"config/label_registry/delete",
		"config/floor_registry/create",
		"config/floor_registry/update",
		"config/floor_registry/delete",
		"zone/create",
		"zone/update",
		"zone/delete",
		"person/create",
		"person/update",
		"person/delete",
		"tag/create",
		"tag/update",
		"tag/delete",
		"lovelace/config/save",
		"lovelace/dashboards/create",
		"lovelace/dashboards/update",
		"lovelace/dashboards/delete",
		"recorder/clear_statistics",
		"hacs/repository/download",
		"hacs/repository/remove",
		"hacs/repositories/add",
		"hacs/repositories/remove",
		"hacs/repository/refresh",
		"hacs/repository/beta",
		"auth/sign_path",
		"camera/stream",
		"test/echo",
		"",
		"unknown/thing",
		"list",
	}

	for _, msgType := range safe {
		t.Run("safe/"+msgType, func(t *testing.T) {
			t.Parallel()
			if !isReplaySafeWSCommand(msgType) {
				t.Errorf("isReplaySafeWSCommand(%q) = false, want true", msgType)
			}
		})
	}
	for _, msgType := range unsafe {
		t.Run("unsafe/"+msgType, func(t *testing.T) {
			t.Parallel()
			if isReplaySafeWSCommand(msgType) {
				t.Errorf("isReplaySafeWSCommand(%q) = true, want false", msgType)
			}
		})
	}
}

func TestWSRetryPredicate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		msgType string
		err     error
		want    bool
	}{
		{"not sent write", "call_service", newNotSentError(errors.New("not connected")), true},
		{"not sent read", "get_states", newNotSentError(errors.New("not connected")), true},
		{"not sent connection lost write", "call_service", newNotSentError(errConnectionLost), true},
		{"not sent connection lost read", "get_states", newNotSentError(errConnectionLost), true},
		{"connection lost write", "call_service", errConnectionLost, false},
		{"connection lost read", "get_states", errConnectionLost, true},
		{"transient message write", "call_service", errors.New("command failed: unknown_error - connection refused"), false},
		{"transient message read", "get_states", errors.New("command failed: unknown_error - connection refused"), true},
		{"canceled write", "call_service", context.Canceled, false},
		{"canceled read", "get_states", context.Canceled, false},
		{"nil write", "call_service", nil, false},
		{"nil read", "get_states", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := wsRetryPredicate(tt.msgType)(tt.err); got != tt.want {
				t.Errorf("wsRetryPredicate(%q)(%v) = %v, want %v", tt.msgType, tt.err, got, tt.want)
			}
		})
	}
}

func TestMarkOutcomeUnknown(t *testing.T) {
	t.Parallel()

	notSent := newNotSentError(errConnectionLost)
	timeout := ErrRequestTimeout
	failed := errors.New("command failed: x - y")

	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		if got := markOutcomeUnknown("call_service", nil); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})

	t.Run("write connection lost is wrapped", func(t *testing.T) {
		t.Parallel()
		got := markOutcomeUnknown("call_service", errConnectionLost)
		if !errors.Is(got, errOutcomeUnknown) {
			t.Errorf("got %v, want errOutcomeUnknown", got)
		}
		if !errors.Is(got, errConnectionLost) {
			t.Errorf("got %v, want errConnectionLost preserved", got)
		}
	})

	t.Run("write wrapped connection lost is wrapped", func(t *testing.T) {
		t.Parallel()
		got := markOutcomeUnknown("call_service", fmt.Errorf("outer: %w", errConnectionLost))
		if !errors.Is(got, errOutcomeUnknown) || !errors.Is(got, errConnectionLost) {
			t.Errorf("got %v, want both errOutcomeUnknown and errConnectionLost", got)
		}
	})

	unchanged := []struct {
		name    string
		msgType string
		err     error
	}{
		{"not sent write", "call_service", notSent},
		{"read connection lost", "get_states", errConnectionLost},
		{"write timeout", "call_service", timeout},
		{"write command failed", "call_service", failed},
	}
	for _, tt := range unchanged {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := markOutcomeUnknown(tt.msgType, tt.err)
			if got != tt.err { //nolint:errorlint // identity check is the point
				t.Errorf("got %v, want %v unchanged", got, tt.err)
			}
			if errors.Is(got, errOutcomeUnknown) {
				t.Errorf("got %v, must not be errOutcomeUnknown", got)
			}
		})
	}
}
