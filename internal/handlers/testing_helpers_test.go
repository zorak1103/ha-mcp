// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// UniversalMockClient is a flexible mock for all handler tests.
// It implements the homeassistant.Client interface with configurable function hooks.
// If a hook is nil, the method returns a sensible default (empty slice or zero value).
type UniversalMockClient struct {
	homeassistant.Client

	// Entity operations
	GetStatesFn func(ctx context.Context) ([]homeassistant.Entity, error)
	GetStateFn  func(ctx context.Context, entityID string) (*homeassistant.Entity, error)
	SetStateFn  func(ctx context.Context, entityID string, state homeassistant.StateUpdate) (*homeassistant.Entity, error)

	// History operations
	GetHistoryFn func(ctx context.Context, entityID string, start, end time.Time) ([][]homeassistant.HistoryEntry, error)

	// Automation operations
	ListAutomationsFn  func(ctx context.Context) ([]homeassistant.Automation, error)
	GetAutomationFn    func(ctx context.Context, automationID string) (*homeassistant.Automation, error)
	CreateAutomationFn func(ctx context.Context, config homeassistant.AutomationConfig) error
	UpdateAutomationFn func(ctx context.Context, automationID string, config homeassistant.AutomationConfig) error
	DeleteAutomationFn func(ctx context.Context, automationID string) error
	ToggleAutomationFn func(ctx context.Context, entityID string, enabled bool) error

	// Helper operations
	ListHelpersFn    func(ctx context.Context) ([]homeassistant.Entity, error)
	CreateHelperFn   func(ctx context.Context, helper homeassistant.HelperConfig) error
	UpdateHelperFn   func(ctx context.Context, helperID string, helper homeassistant.HelperConfig) error
	DeleteHelperFn   func(ctx context.Context, helperID string) error
	SetHelperValueFn func(ctx context.Context, entityID string, value any) error

	// Script operations
	ListScriptsFn  func(ctx context.Context) ([]homeassistant.Entity, error)
	GetScriptFn    func(ctx context.Context, scriptID string) (*homeassistant.Script, error)
	CreateScriptFn func(ctx context.Context, scriptID string, config homeassistant.ScriptConfig) error
	UpdateScriptFn func(ctx context.Context, scriptID string, config homeassistant.ScriptConfig) error
	DeleteScriptFn func(ctx context.Context, scriptID string) error

	// Scene operations
	ListScenesFn  func(ctx context.Context) ([]homeassistant.Entity, error)
	CreateSceneFn func(ctx context.Context, sceneID string, config homeassistant.SceneConfig) error
	UpdateSceneFn func(ctx context.Context, sceneID string, config homeassistant.SceneConfig) error
	DeleteSceneFn func(ctx context.Context, sceneID string) error

	// Service operations
	CallServiceFn func(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error)

	// Registry operations
	GetEntityRegistryFn func(ctx context.Context) ([]homeassistant.EntityRegistryEntry, error)
	GetDeviceRegistryFn func(ctx context.Context) ([]homeassistant.DeviceRegistryEntry, error)
	GetAreaRegistryFn   func(ctx context.Context) ([]homeassistant.AreaRegistryEntry, error)

	// Media operations
	SignPathFn        func(ctx context.Context, path string, expires int) (string, error)
	GetCameraStreamFn func(ctx context.Context, entityID string) (*homeassistant.StreamInfo, error)
	BrowseMediaFn     func(ctx context.Context, mediaContentID string) (*homeassistant.MediaBrowseResult, error)

	// Configuration operations
	GetLovelaceConfigFn func(ctx context.Context) (map[string]any, error)

	// Statistics operations
	GetStatisticsFn func(ctx context.Context, statIDs []string, period string) ([]homeassistant.StatisticsResult, error)

	// Target operations
	GetTriggersForTargetFn   func(ctx context.Context, target homeassistant.Target, expandGroup *bool) ([]string, error)
	GetConditionsForTargetFn func(ctx context.Context, target homeassistant.Target, expandGroup *bool) ([]string, error)
	GetServicesForTargetFn   func(ctx context.Context, target homeassistant.Target, expandGroup *bool) ([]string, error)
	ExtractFromTargetFn      func(ctx context.Context, target homeassistant.Target, expandGroup *bool) (*homeassistant.ExtractFromTargetResult, error)

	// Config operations
	GetScheduleConfigFn func(ctx context.Context, scheduleID string) (map[string]any, error)
}

// Entity operations implementation

func (m *UniversalMockClient) GetStates(ctx context.Context) ([]homeassistant.Entity, error) {
	if m.GetStatesFn != nil {
		return m.GetStatesFn(ctx)
	}
	return []homeassistant.Entity{}, nil
}

func (m *UniversalMockClient) GetState(ctx context.Context, entityID string) (*homeassistant.Entity, error) {
	if m.GetStateFn != nil {
		return m.GetStateFn(ctx, entityID)
	}
	return &homeassistant.Entity{EntityID: entityID, State: "unknown"}, nil
}

func (m *UniversalMockClient) SetState(ctx context.Context, entityID string, state homeassistant.StateUpdate) (*homeassistant.Entity, error) {
	if m.SetStateFn != nil {
		return m.SetStateFn(ctx, entityID, state)
	}
	return &homeassistant.Entity{EntityID: entityID, State: "updated"}, nil
}

// History operations implementation

func (m *UniversalMockClient) GetHistory(ctx context.Context, entityID string, start, end time.Time) ([][]homeassistant.HistoryEntry, error) {
	if m.GetHistoryFn != nil {
		return m.GetHistoryFn(ctx, entityID, start, end)
	}
	return [][]homeassistant.HistoryEntry{}, nil
}

// Automation operations implementation

func (m *UniversalMockClient) ListAutomations(ctx context.Context) ([]homeassistant.Automation, error) {
	if m.ListAutomationsFn != nil {
		return m.ListAutomationsFn(ctx)
	}
	return []homeassistant.Automation{}, nil
}

func (m *UniversalMockClient) GetAutomation(ctx context.Context, automationID string) (*homeassistant.Automation, error) {
	if m.GetAutomationFn != nil {
		return m.GetAutomationFn(ctx, automationID)
	}
	return &homeassistant.Automation{EntityID: "automation." + automationID}, nil
}

func (m *UniversalMockClient) CreateAutomation(ctx context.Context, config homeassistant.AutomationConfig) error {
	if m.CreateAutomationFn != nil {
		return m.CreateAutomationFn(ctx, config)
	}
	return nil
}

func (m *UniversalMockClient) UpdateAutomation(ctx context.Context, automationID string, config homeassistant.AutomationConfig) error {
	if m.UpdateAutomationFn != nil {
		return m.UpdateAutomationFn(ctx, automationID, config)
	}
	return nil
}

func (m *UniversalMockClient) DeleteAutomation(ctx context.Context, automationID string) error {
	if m.DeleteAutomationFn != nil {
		return m.DeleteAutomationFn(ctx, automationID)
	}
	return nil
}

func (m *UniversalMockClient) ToggleAutomation(ctx context.Context, entityID string, enabled bool) error {
	if m.ToggleAutomationFn != nil {
		return m.ToggleAutomationFn(ctx, entityID, enabled)
	}
	return nil
}

// Helper operations implementation

func (m *UniversalMockClient) ListHelpers(ctx context.Context) ([]homeassistant.Entity, error) {
	if m.ListHelpersFn != nil {
		return m.ListHelpersFn(ctx)
	}
	return []homeassistant.Entity{}, nil
}

func (m *UniversalMockClient) CreateHelper(ctx context.Context, helper homeassistant.HelperConfig) error {
	if m.CreateHelperFn != nil {
		return m.CreateHelperFn(ctx, helper)
	}
	return nil
}

func (m *UniversalMockClient) UpdateHelper(ctx context.Context, helperID string, helper homeassistant.HelperConfig) error {
	if m.UpdateHelperFn != nil {
		return m.UpdateHelperFn(ctx, helperID, helper)
	}
	return nil
}

func (m *UniversalMockClient) DeleteHelper(ctx context.Context, helperID string) error {
	if m.DeleteHelperFn != nil {
		return m.DeleteHelperFn(ctx, helperID)
	}
	return nil
}

func (m *UniversalMockClient) SetHelperValue(ctx context.Context, entityID string, value any) error {
	if m.SetHelperValueFn != nil {
		return m.SetHelperValueFn(ctx, entityID, value)
	}
	return nil
}

// Script operations implementation

func (m *UniversalMockClient) ListScripts(ctx context.Context) ([]homeassistant.Entity, error) {
	if m.ListScriptsFn != nil {
		return m.ListScriptsFn(ctx)
	}
	return []homeassistant.Entity{}, nil
}

func (m *UniversalMockClient) GetScript(ctx context.Context, scriptID string) (*homeassistant.Script, error) {
	if m.GetScriptFn != nil {
		return m.GetScriptFn(ctx, scriptID)
	}
	return &homeassistant.Script{EntityID: "script." + scriptID}, nil
}

func (m *UniversalMockClient) CreateScript(ctx context.Context, scriptID string, config homeassistant.ScriptConfig) error {
	if m.CreateScriptFn != nil {
		return m.CreateScriptFn(ctx, scriptID, config)
	}
	return nil
}

func (m *UniversalMockClient) UpdateScript(ctx context.Context, scriptID string, config homeassistant.ScriptConfig) error {
	if m.UpdateScriptFn != nil {
		return m.UpdateScriptFn(ctx, scriptID, config)
	}
	return nil
}

func (m *UniversalMockClient) DeleteScript(ctx context.Context, scriptID string) error {
	if m.DeleteScriptFn != nil {
		return m.DeleteScriptFn(ctx, scriptID)
	}
	return nil
}

// Scene operations implementation

func (m *UniversalMockClient) ListScenes(ctx context.Context) ([]homeassistant.Entity, error) {
	if m.ListScenesFn != nil {
		return m.ListScenesFn(ctx)
	}
	return []homeassistant.Entity{}, nil
}

func (m *UniversalMockClient) CreateScene(ctx context.Context, sceneID string, config homeassistant.SceneConfig) error {
	if m.CreateSceneFn != nil {
		return m.CreateSceneFn(ctx, sceneID, config)
	}
	return nil
}

func (m *UniversalMockClient) UpdateScene(ctx context.Context, sceneID string, config homeassistant.SceneConfig) error {
	if m.UpdateSceneFn != nil {
		return m.UpdateSceneFn(ctx, sceneID, config)
	}
	return nil
}

func (m *UniversalMockClient) DeleteScene(ctx context.Context, sceneID string) error {
	if m.DeleteSceneFn != nil {
		return m.DeleteSceneFn(ctx, sceneID)
	}
	return nil
}

// Service operations implementation

func (m *UniversalMockClient) CallService(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
	if m.CallServiceFn != nil {
		return m.CallServiceFn(ctx, domain, service, data)
	}
	return []homeassistant.Entity{}, nil
}

// Registry operations implementation

func (m *UniversalMockClient) GetEntityRegistry(ctx context.Context) ([]homeassistant.EntityRegistryEntry, error) {
	if m.GetEntityRegistryFn != nil {
		return m.GetEntityRegistryFn(ctx)
	}
	return []homeassistant.EntityRegistryEntry{}, nil
}

func (m *UniversalMockClient) GetDeviceRegistry(ctx context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
	if m.GetDeviceRegistryFn != nil {
		return m.GetDeviceRegistryFn(ctx)
	}
	return []homeassistant.DeviceRegistryEntry{}, nil
}

func (m *UniversalMockClient) GetAreaRegistry(ctx context.Context) ([]homeassistant.AreaRegistryEntry, error) {
	if m.GetAreaRegistryFn != nil {
		return m.GetAreaRegistryFn(ctx)
	}
	return []homeassistant.AreaRegistryEntry{}, nil
}

// Media operations implementation

func (m *UniversalMockClient) SignPath(ctx context.Context, path string, expires int) (string, error) {
	if m.SignPathFn != nil {
		return m.SignPathFn(ctx, path, expires)
	}
	return path + "?signed=true", nil
}

func (m *UniversalMockClient) GetCameraStream(ctx context.Context, entityID string) (*homeassistant.StreamInfo, error) {
	if m.GetCameraStreamFn != nil {
		return m.GetCameraStreamFn(ctx, entityID)
	}
	return &homeassistant.StreamInfo{URL: "http://example.com/stream"}, nil
}

func (m *UniversalMockClient) BrowseMedia(ctx context.Context, mediaContentID string) (*homeassistant.MediaBrowseResult, error) {
	if m.BrowseMediaFn != nil {
		return m.BrowseMediaFn(ctx, mediaContentID)
	}
	return &homeassistant.MediaBrowseResult{}, nil
}

// Configuration operations implementation

func (m *UniversalMockClient) GetLovelaceConfig(ctx context.Context) (map[string]any, error) {
	if m.GetLovelaceConfigFn != nil {
		return m.GetLovelaceConfigFn(ctx)
	}
	return map[string]any{}, nil
}

// Statistics operations implementation

func (m *UniversalMockClient) GetStatistics(ctx context.Context, statIDs []string, period string) ([]homeassistant.StatisticsResult, error) {
	if m.GetStatisticsFn != nil {
		return m.GetStatisticsFn(ctx, statIDs, period)
	}
	return []homeassistant.StatisticsResult{}, nil
}

// Target operations implementation

func (m *UniversalMockClient) GetTriggersForTarget(ctx context.Context, target homeassistant.Target, expandGroup *bool) ([]string, error) {
	if m.GetTriggersForTargetFn != nil {
		return m.GetTriggersForTargetFn(ctx, target, expandGroup)
	}
	return []string{}, nil
}

func (m *UniversalMockClient) GetConditionsForTarget(ctx context.Context, target homeassistant.Target, expandGroup *bool) ([]string, error) {
	if m.GetConditionsForTargetFn != nil {
		return m.GetConditionsForTargetFn(ctx, target, expandGroup)
	}
	return []string{}, nil
}

func (m *UniversalMockClient) GetServicesForTarget(ctx context.Context, target homeassistant.Target, expandGroup *bool) ([]string, error) {
	if m.GetServicesForTargetFn != nil {
		return m.GetServicesForTargetFn(ctx, target, expandGroup)
	}
	return []string{}, nil
}

func (m *UniversalMockClient) ExtractFromTarget(ctx context.Context, target homeassistant.Target, expandGroup *bool) (*homeassistant.ExtractFromTargetResult, error) {
	if m.ExtractFromTargetFn != nil {
		return m.ExtractFromTargetFn(ctx, target, expandGroup)
	}
	return &homeassistant.ExtractFromTargetResult{}, nil
}

// Config operations implementation

func (m *UniversalMockClient) GetScheduleConfig(ctx context.Context, scheduleID string) (map[string]any, error) {
	if m.GetScheduleConfigFn != nil {
		return m.GetScheduleConfigFn(ctx, scheduleID)
	}
	return map[string]any{}, nil
}

// =============================================================================
// Test Helper Functions
// =============================================================================

// handlerTestCase represents a standard test case for handler functions.
type handlerTestCase struct {
	name            string
	args            map[string]any
	setupMock       func(*UniversalMockClient)
	wantError       bool
	wantContains    []string
	wantNotContains []string
}

// runHandlerTestCases executes a set of test cases for a handler function.
func runHandlerTestCases(
	t *testing.T,
	tests []handlerTestCase,
	handlerFunc func(context.Context, homeassistant.Client, map[string]any) (*mcp.ToolsCallResult, error),
) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &UniversalMockClient{}
			if tt.setupMock != nil {
				tt.setupMock(client)
			}

			result, err := handlerFunc(context.Background(), client, tt.args)
			if err != nil {
				t.Fatalf("handler returned unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("handler returned nil result")
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Fatal("handler returned empty content")
			}

			content := result.Content[0].Text
			assertContainsAll(t, content, tt.wantContains)
			assertNotContainsAny(t, content, tt.wantNotContains)
		})
	}
}

// paramRequiredTestCases generates standard test cases for required parameters.
func paramRequiredTestCases(paramName string) []handlerTestCase {
	return []handlerTestCase{
		{
			name:         "missing " + paramName,
			args:         map[string]any{},
			wantError:    true,
			wantContains: []string{paramName + " is required"},
		},
		{
			name:         "empty " + paramName,
			args:         map[string]any{paramName: ""},
			wantError:    true,
			wantContains: []string{paramName + " is required"},
		},
	}
}

// =============================================================================
// Tool Schema Validation Helpers
// =============================================================================

// toolSchemaExpectation defines expectations for a tool's schema.
type toolSchemaExpectation struct {
	ExpectedName    string
	RequiredParams  []string
	OptionalParams  []string
	WantDescription bool
}

// verifyToolSchema validates a tool's schema against expectations.
func verifyToolSchema(t *testing.T, tool mcp.Tool, expect toolSchemaExpectation) {
	t.Helper()

	// Name check
	if tool.Name != expect.ExpectedName {
		t.Errorf("tool.Name = %q, want %q", tool.Name, expect.ExpectedName)
	}

	// Description check
	if expect.WantDescription && tool.Description == "" {
		t.Error("tool.Description is empty, want non-empty")
	}

	// Schema type check
	if tool.InputSchema.Type != testSchemaTypeObject {
		t.Errorf("InputSchema.Type = %q, want %q", tool.InputSchema.Type, testSchemaTypeObject)
	}

	// Required parameters check
	requiredMap := make(map[string]bool)
	for _, req := range tool.InputSchema.Required {
		requiredMap[req] = true
	}

	for _, param := range expect.RequiredParams {
		if !requiredMap[param] {
			t.Errorf("Required parameter %q not found in schema.Required", param)
		}
	}

	// Properties check (required + optional)
	allParams := make([]string, 0, len(expect.RequiredParams)+len(expect.OptionalParams))
	allParams = append(allParams, expect.RequiredParams...)
	allParams = append(allParams, expect.OptionalParams...)
	for _, param := range allParams {
		if _, ok := tool.InputSchema.Properties[param]; !ok {
			t.Errorf("Property %q missing from schema.Properties", param)
		}
	}
}

// =============================================================================
// Content Assertion Helpers
// =============================================================================

// assertContainsAll checks that content contains all expected strings.
func assertContainsAll(t *testing.T, content string, want []string) {
	t.Helper()
	for _, expected := range want {
		if !strings.Contains(content, expected) {
			t.Errorf("Content missing expected string %q\nGot: %s", expected, truncateForError(content))
		}
	}
}

// assertNotContainsAny checks that content does not contain any of the unwanted strings.
func assertNotContainsAny(t *testing.T, content string, notWant []string) {
	t.Helper()
	for _, unexpected := range notWant {
		if strings.Contains(content, unexpected) {
			t.Errorf("Content should not contain %q\nGot: %s", unexpected, truncateForError(content))
		}
	}
}

// truncateForError truncates long content for readable error messages.
func truncateForError(content string) string {
	const maxLen = 500
	if len(content) > maxLen {
		return content[:maxLen] + "... (truncated)"
	}
	return content
}

// =============================================================================
// Common Test Data
// =============================================================================

// testEntity creates a standard test entity.
func testEntity(entityID, state string) homeassistant.Entity {
	return homeassistant.Entity{
		EntityID: entityID,
		State:    state,
		Attributes: map[string]any{
			"friendly_name": "Test " + entityID,
		},
	}
}

// testAutomation creates a standard test automation.
func testAutomation(id, state, friendlyName string) homeassistant.Automation {
	return homeassistant.Automation{
		EntityID:      "automation." + id,
		State:         state,
		FriendlyName:  friendlyName,
		LastTriggered: "2024-01-15T10:30:00Z",
	}
}

// =============================================================================
// Tests for Testing Helpers (Self-Tests)
// =============================================================================

func TestUniversalMockClient_DefaultBehavior(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{}
	ctx := context.Background()

	// Test default entity operations
	t.Run("GetStates returns empty slice", func(t *testing.T) {
		t.Parallel()
		states, err := client.GetStates(ctx)
		if err != nil {
			t.Errorf("GetStates() error = %v", err)
		}
		if len(states) != 0 {
			t.Errorf("GetStates() len = %d, want 0", len(states))
		}
	})

	t.Run("GetState returns default entity", func(t *testing.T) {
		t.Parallel()
		entity, err := client.GetState(ctx, "test.entity")
		if err != nil {
			t.Errorf("GetState() error = %v", err)
		}
		if entity.EntityID != "test.entity" {
			t.Errorf("GetState() EntityID = %q, want 'test.entity'", entity.EntityID)
		}
	})

	t.Run("ListAutomations returns empty slice", func(t *testing.T) {
		t.Parallel()
		autos, err := client.ListAutomations(ctx)
		if err != nil {
			t.Errorf("ListAutomations() error = %v", err)
		}
		if len(autos) != 0 {
			t.Errorf("ListAutomations() len = %d, want 0", len(autos))
		}
	})
}

func TestUniversalMockClient_CustomHooks(t *testing.T) {
	t.Parallel()

	t.Run("GetStateFn hook is called", func(t *testing.T) {
		t.Parallel()

		called := false
		client := &UniversalMockClient{
			GetStateFn: func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
				called = true
				return &homeassistant.Entity{EntityID: entityID, State: "custom"}, nil
			},
		}

		entity, _ := client.GetState(context.Background(), "test.entity")
		if !called {
			t.Error("GetStateFn was not called")
		}
		if entity.State != "custom" {
			t.Errorf("GetState() State = %q, want 'custom'", entity.State)
		}
	})

	t.Run("ListAutomationsFn hook is called", func(t *testing.T) {
		t.Parallel()

		client := &UniversalMockClient{
			ListAutomationsFn: func(_ context.Context) ([]homeassistant.Automation, error) {
				return []homeassistant.Automation{
					{EntityID: "automation.test"},
				}, nil
			},
		}

		autos, _ := client.ListAutomations(context.Background())
		if len(autos) != 1 {
			t.Errorf("ListAutomations() len = %d, want 1", len(autos))
		}
	})
}

func TestTestEntity(t *testing.T) {
	t.Parallel()

	entity := testEntity("light.living_room", "on")

	if entity.EntityID != "light.living_room" {
		t.Errorf("testEntity() EntityID = %q, want 'light.living_room'", entity.EntityID)
	}
	if entity.State != "on" {
		t.Errorf("testEntity() State = %q, want 'on'", entity.State)
	}
	if entity.Attributes["friendly_name"] != "Test light.living_room" {
		t.Errorf("testEntity() friendly_name = %q, want 'Test light.living_room'", entity.Attributes["friendly_name"])
	}
}

func TestTestAutomation(t *testing.T) {
	t.Parallel()

	auto := testAutomation("morning_routine", "on", "Morning Routine")

	if auto.EntityID != "automation.morning_routine" {
		t.Errorf("testAutomation() EntityID = %q, want 'automation.morning_routine'", auto.EntityID)
	}
	if auto.State != "on" {
		t.Errorf("testAutomation() State = %q, want 'on'", auto.State)
	}
	if auto.FriendlyName != "Morning Routine" {
		t.Errorf("testAutomation() FriendlyName = %q, want 'Morning Routine'", auto.FriendlyName)
	}
}

func TestAssertContainsAll(t *testing.T) {
	t.Parallel()

	// Create a mock T to capture errors
	tests := []struct {
		name       string
		content    string
		want       []string
		shouldFail bool
	}{
		{
			name:       "all strings present",
			content:    "hello world test",
			want:       []string{"hello", "world"},
			shouldFail: false,
		},
		{
			name:       "empty want list",
			content:    "any content",
			want:       []string{},
			shouldFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Just verify no panic occurs
			assertContainsAll(t, tt.content, tt.want)
		})
	}
}

func TestAssertNotContainsAny(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		notWant []string
	}{
		{
			name:    "no unwanted strings",
			content: "hello world",
			notWant: []string{"foo", "bar"},
		},
		{
			name:    "empty not want list",
			content: "any content",
			notWant: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Just verify no panic occurs
			assertNotContainsAny(t, tt.content, tt.notWant)
		})
	}
}

func TestTruncateForError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		wantLen  int
		wantTail string
	}{
		{
			name:     "short content unchanged",
			content:  "short",
			wantLen:  5,
			wantTail: "short",
		},
		{
			name:     "long content truncated",
			content:  strings.Repeat("a", 600),
			wantLen:  515, // 500 + len("... (truncated)")
			wantTail: "... (truncated)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := truncateForError(tt.content)
			if len(result) != tt.wantLen {
				t.Errorf("truncateForError() len = %d, want %d", len(result), tt.wantLen)
			}
			if !strings.HasSuffix(result, tt.wantTail) {
				t.Errorf("truncateForError() should end with %q", tt.wantTail)
			}
		})
	}
}

func TestParamRequiredTestCases(t *testing.T) {
	t.Parallel()

	cases := paramRequiredTestCases("entity_id")

	if len(cases) != 2 {
		t.Fatalf("paramRequiredTestCases() len = %d, want 2", len(cases))
	}

	if cases[0].name != "missing entity_id" {
		t.Errorf("cases[0].name = %q, want 'missing entity_id'", cases[0].name)
	}
	if cases[1].name != "empty entity_id" {
		t.Errorf("cases[1].name = %q, want 'empty entity_id'", cases[1].name)
	}
	if !cases[0].wantError {
		t.Error("cases[0].wantError should be true")
	}
	if len(cases[0].wantContains) == 0 || cases[0].wantContains[0] != "entity_id is required" {
		t.Error("cases[0].wantContains should include 'entity_id is required'")
	}
}

func TestVerifyToolSchema(t *testing.T) {
	t.Parallel()

	tool := mcp.Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: mcp.JSONSchema{
			Type: testSchemaTypeObject,
			Properties: map[string]mcp.JSONSchema{
				"required_param": {Type: "string"},
				"optional_param": {Type: "boolean"},
			},
			Required: []string{"required_param"},
		},
	}

	// This should pass without errors
	verifyToolSchema(t, tool, toolSchemaExpectation{
		ExpectedName:    "test_tool",
		RequiredParams:  []string{"required_param"},
		OptionalParams:  []string{"optional_param"},
		WantDescription: true,
	})
}

func TestRunHandlerTestCases(t *testing.T) {
	t.Parallel()

	// Create a simple handler for testing
	handler := func(_ context.Context, _ homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
		entityID := getString(args, "entity_id")
		if entityID == "" {
			return &mcp.ToolsCallResult{
				IsError: true,
				Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			}, nil
		}
		return &mcp.ToolsCallResult{
			IsError: false,
			Content: []mcp.ContentBlock{mcp.NewTextContent("Success: " + entityID)},
		}, nil
	}

	tests := []handlerTestCase{
		{
			name:         "success case",
			args:         map[string]any{"entity_id": "light.test"},
			wantError:    false,
			wantContains: []string{"Success", "light.test"},
		},
		{
			name:         "error case - missing param",
			args:         map[string]any{},
			wantError:    true,
			wantContains: []string{"entity_id is required"},
		},
	}

	runHandlerTestCases(t, tests, handler)
}

func TestHandlerTestCase_WithMock(t *testing.T) {
	t.Parallel()

	handler := func(_ context.Context, client homeassistant.Client, _ map[string]any) (*mcp.ToolsCallResult, error) {
		states, err := client.GetStates(context.Background())
		if err != nil {
			return &mcp.ToolsCallResult{
				IsError: true,
				Content: []mcp.ContentBlock{mcp.NewTextContent("Error: " + err.Error())},
			}, err
		}
		return &mcp.ToolsCallResult{
			IsError: false,
			Content: []mcp.ContentBlock{mcp.NewTextContent("Found " + string(rune('0'+len(states))) + " states")},
		}, nil
	}

	tests := []handlerTestCase{
		{
			name: "with mock setup",
			args: map[string]any{},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{
						{EntityID: "light.one"},
						{EntityID: "light.two"},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Found 2 states"},
		},
	}

	runHandlerTestCases(t, tests, handler)
}
