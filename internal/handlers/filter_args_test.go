package handlers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// filterTypeCase is one row of a strict-filter-type test: args are sent to the
// handler as-is; want is a substring of the expected error text, or "" when the
// call must succeed. setupMock (optional) prepares the client, and gets the
// subtest's t so a mock can fail the test when HA must not be reached.
type filterTypeCase struct {
	name      string
	args      map[string]any
	want      string
	setupMock func(t *testing.T, c *UniversalMockClient)
}

// mustNotCallHA makes every client read that a strict-argument test can reach
// fail the test: an invalid argument must be rejected before Home Assistant is
// called. Use it as a filterTypeCase setupMock.
func mustNotCallHA(t *testing.T, c *UniversalMockClient) {
	t.Helper()
	fail := func() { t.Errorf("HA must not be called for an invalid argument") }
	c.GetStatesFn = func(context.Context) ([]homeassistant.Entity, error) { fail(); return nil, nil }
	c.GetHistoryFn = func(context.Context, []string, time.Time, time.Time, homeassistant.HistoryOptions) (map[string][]homeassistant.HistoryEntry, error) {
		fail()
		return nil, nil
	}
	c.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) { fail(); return nil, nil }
	c.GetEntityRegistryEntryFn = func(context.Context, string) (*homeassistant.EntityRegistryEntry, error) { fail(); return nil, nil }
	c.GetDeviceRegistryFn = func(context.Context) ([]homeassistant.DeviceRegistryEntry, error) { fail(); return nil, nil }
	c.GetConfigEntriesFn = func(context.Context, string) ([]homeassistant.ConfigEntryFull, error) { fail(); return nil, nil }
	c.GetLovelaceConfigFn = func(context.Context, string) (map[string]any, error) { fail(); return nil, nil }
	c.ListDashboardsFn = func(context.Context) ([]homeassistant.DashboardEntry, error) { fail(); return nil, nil }
	c.ListAutomationsFn = func(context.Context) ([]homeassistant.Automation, error) { fail(); return nil, nil }
	c.ListScriptsFn = func(context.Context) ([]homeassistant.Entity, error) { fail(); return nil, nil }
	c.ListScenesFn = func(context.Context) ([]homeassistant.Entity, error) { fail(); return nil, nil }
	c.GetSystemLogFn = func(context.Context) ([]homeassistant.SystemLogEntry, error) { fail(); return nil, nil }
	c.SendHACSCommandFn = func(context.Context, string, map[string]any) (any, error) { fail(); return nil, nil }
}

// runFilterTypeCases runs each case in parallel against a fresh mock client.
func runFilterTypeCases(
	t *testing.T,
	handlerFunc func(context.Context, homeassistant.Client, map[string]any) (*mcp.ToolsCallResult, error),
	cases []filterTypeCase,
) {
	t.Helper()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := &UniversalMockClient{}
			if tc.setupMock != nil {
				tc.setupMock(t, client)
			}

			result, err := handlerFunc(context.Background(), client, tc.args)
			if err != nil {
				t.Fatalf("handler returned unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("handler returned nil result")
			}

			if tc.want == "" {
				if result.IsError {
					t.Errorf("want success, got error %q", resultText(t, result))
				}
				return
			}
			if !result.IsError {
				t.Fatalf("want error containing %q, got success: %q", tc.want, resultText(t, result))
			}
			if got := resultText(t, result); !strings.Contains(got, tc.want) {
				t.Errorf("error = %q, want it to contain %q", got, tc.want)
			}
		})
	}
}
