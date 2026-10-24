//go:build integration

package integration

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// fakeCleanupClient embeds the Client interface (nil) so only the methods the cleanup helpers
// call need overriding; any other call panics, which makes an unexpected call a loud failure.
type fakeCleanupClient struct {
	homeassistant.Client

	automations []homeassistant.Automation

	deleteErr   error
	registryErr error

	deleteCalls     int
	registryRemoved []string
}

func (f *fakeCleanupClient) ListAutomations(context.Context) ([]homeassistant.Automation, error) {
	return f.automations, nil
}

func (f *fakeCleanupClient) DeleteAutomation(context.Context, string) error {
	f.deleteCalls++
	return f.deleteErr
}

func (f *fakeCleanupClient) DeleteScript(context.Context, string) error {
	f.deleteCalls++
	return f.deleteErr
}

func (f *fakeCleanupClient) RemoveEntityRegistryEntry(_ context.Context, entityID string) error {
	f.registryRemoved = append(f.registryRemoved, entityID)
	return f.registryErr
}

var (
	errResourceNotFound400 = errors.New(`home assistant api error (status 400): unexpected status 400: {"message":"Resource not found"}`)
	errAPINotFound404      = &homeassistant.APIError{StatusCode: http.StatusNotFound, Message: "gone"}
	errAPIServerError500   = &homeassistant.APIError{StatusCode: http.StatusInternalServerError, Message: "boom"}
)

type fallbackCase struct {
	name             string
	id               string
	entityID         string // automation only; "" means derive from id
	deleteErr        error
	registryErr      error
	wantViaRegistry  bool
	wantErrContains  []string
	wantDeleteCalls  int
	wantRegistryDels []string
}

func fallbackCases(domain string) []fallbackCase {
	const testID = "mcptest_1a2b3c4d_x"
	testEntity := BuildEntityID(domain, testID)

	return []fallbackCase{
		{
			name: "not_found_400_falls_back_once", id: testID, deleteErr: errResourceNotFound400,
			wantViaRegistry: true, wantDeleteCalls: 1, wantRegistryDels: []string{testEntity},
		},
		{
			name: "api_error_404_falls_back", id: testID, deleteErr: errAPINotFound404,
			wantViaRegistry: true, wantDeleteCalls: 1, wantRegistryDels: []string{testEntity},
		},
		{
			name: "other_error_never_touches_registry", id: testID, deleteErr: errAPIServerError500,
			wantErrContains: []string{"boom"}, wantDeleteCalls: 3,
		},
		{
			name: "registry_not_found_is_success", id: testID, deleteErr: errResourceNotFound400,
			registryErr:     errors.New("entity not found"),
			wantViaRegistry: true, wantDeleteCalls: 1, wantRegistryDels: []string{testEntity},
		},
		{
			name: "registry_failure_reports_both", id: testID, deleteErr: errResourceNotFound400,
			registryErr:     errors.New("registry exploded"),
			wantErrContains: []string{"Resource not found", "registry exploded"},
			wantDeleteCalls: 1, wantRegistryDels: []string{testEntity},
		},
		{
			name: "non_test_id_rejected", id: "plain_" + domain, deleteErr: errResourceNotFound400,
			wantErrContains: []string{"refusing to operate on non-test entity"},
		},
		{
			name: "delete_success_no_fallback", id: testID,
			wantDeleteCalls: 1,
		},
	}
}

func assertFallbackResult(t *testing.T, tc fallbackCase, fake *fakeCleanupClient, viaRegistry bool, err error) {
	t.Helper()

	if len(tc.wantErrContains) == 0 {
		require.NoError(t, err)
	} else {
		require.Error(t, err)
		for _, want := range tc.wantErrContains {
			assert.Contains(t, err.Error(), want)
		}
	}
	assert.Equal(t, tc.wantViaRegistry, viaRegistry, "viaRegistry")
	assert.Equal(t, tc.wantDeleteCalls, fake.deleteCalls, "delete calls")
	assert.Equal(t, tc.wantRegistryDels, fake.registryRemoved, "registry removals")
}

func TestDeleteAutomationWithRetry_RegistryFallback(t *testing.T) {
	cases := append(fallbackCases("automation"), fallbackCase{
		// UI-created automations have a config id that differs from the entity_id suffix.
		name: "uses_listed_entity_id", id: "mcptest_1a2b3c4d_x", entityID: "automation.mcptest_1a2b3c4d_renamed",
		deleteErr: errResourceNotFound400, wantViaRegistry: true, wantDeleteCalls: 1,
		wantRegistryDels: []string{"automation.mcptest_1a2b3c4d_renamed"},
	})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeCleanupClient{deleteErr: tc.deleteErr, registryErr: tc.registryErr}

			viaRegistry, err := deleteAutomationWithRetry(context.Background(), fake, tc.id, tc.entityID)

			assertFallbackResult(t, tc, fake, viaRegistry, err)
		})
	}
}

func TestDeleteScriptWithRetry_RegistryFallback(t *testing.T) {
	for _, tc := range fallbackCases("script") {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeCleanupClient{deleteErr: tc.deleteErr, registryErr: tc.registryErr}

			viaRegistry, err := deleteScriptWithRetry(context.Background(), fake, tc.id)

			assertFallbackResult(t, tc, fake, viaRegistry, err)
		})
	}
}

func TestCleanupTestAutomations_RegistryFallbackUsesListedEntityID(t *testing.T) {
	fake := &fakeCleanupClient{
		deleteErr: errResourceNotFound400,
		automations: []homeassistant.Automation{{
			EntityID: "automation.mcptest_1a2b3c4d_renamed",
			Config:   &homeassistant.AutomationConfig{ID: "mcptest_1a2b3c4d_x"},
		}},
	}

	err := cleanupTestAutomations(context.Background(), fake)

	require.NoError(t, err)
	assert.Equal(t, []string{"automation.mcptest_1a2b3c4d_renamed"}, fake.registryRemoved)
}
