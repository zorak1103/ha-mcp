package handlers

import (
	"context"
	"strings"
	"testing"

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
