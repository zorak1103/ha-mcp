> [README](../README.md) | [Configuration](configuration.md) | [Tools](tools.md) | [Access Control](access-control.md) | [Architecture](architecture.md) | [Troubleshooting](troubleshooting.md) | [Feature Comparison](feature-comparison.md) | [Integration Tests](integration-tests.md)

# Integration Tests

This document describes how to run integration tests against a real Home Assistant instance.

## Overview

Integration tests verify that the MCP server correctly interacts with Home Assistant by:
- Creating, modifying, and deleting helpers (input_boolean, counters, timers, etc.)
- Creating and executing automations, scripts, and scenes
- Calling Home Assistant services

**Safety Guarantees:**
- All test entities use a unique prefix (`__mcptest_`) to avoid conflicts with production data
- Tests never modify existing entities without the test prefix
- Pre-test and post-test cleanup ensures no test data is left behind

## Prerequisites

1. A running Home Assistant instance (version 2023.1+)
2. A long-lived access token with full API access
3. Go 1.26+ installed

## Configuration

Set the following environment variables:

```bash
export HA_INTEGRATION_TEST_URL=http://homeassistant.local:8123
export HA_INTEGRATION_TEST_TOKEN=<your-long-lived-access-token>
export HA_INTEGRATION_TEST_TIMEOUT=5m  # optional, default 5 minutes
```

Alternatively, create a `.env.integration` file:

```env
HA_INTEGRATION_TEST_URL=http://homeassistant.local:8123
HA_INTEGRATION_TEST_TOKEN=<your-long-lived-access-token>
HA_INTEGRATION_TEST_TIMEOUT=5m
```

### Creating a Long-Lived Access Token

1. Go to your Home Assistant instance
2. Click on your profile (bottom left)
3. Scroll down to "Long-Lived Access Tokens"
4. Click "Create Token"
5. Give it a name (e.g., "MCP Integration Tests")
6. Copy the token and save it securely

## Running Tests

### Run All Integration Tests

```bash
go test -tags=integration -v ./internal/handlers/integration/...
```

### Run Specific Test Suites

```bash
# Run only counter tests
go test -tags=integration -v ./internal/handlers/integration/... -run TestCounter

# Run only automation tests
go test -tags=integration -v ./internal/handlers/integration/... -run TestAutomation

# Run a specific test
go test -tags=integration -v ./internal/handlers/integration/... -run TestCounterIntegration/TestCounterLifecycle
```

### Run with Verbose Output

```bash
go test -tags=integration -v ./internal/handlers/integration/... 2>&1 | tee test-output.log
```

## Test Categories

### Helper Tests (WebSocket-based helpers)

| Test Suite | Operations Tested |
|------------|-------------------|
| `TestCounterIntegration` | create, increment, decrement, set_value, reset, delete |
| `TestInputBooleanIntegration` | create, turn_on, turn_off, toggle, delete |
| `TestInputNumberIntegration` | create, set_value, increment, decrement, delete |
| `TestInputTextIntegration` | create, set_value, delete |
| `TestInputSelectIntegration` | create, select_option, set_options, select_first/last/next/previous, delete |
| `TestInputDatetimeIntegration` | create, set_datetime, delete |
| `TestInputButtonIntegration` | create, press, delete |
| `TestTimerIntegration` | create, start, pause, cancel, finish, change, delete |
| `TestGroupIntegration` | create, set_entities, delete |
| `TestScheduleIntegration` | create, delete |
| `TestThresholdIntegration` | create, delete |
| `TestIntegralIntegration` | create, reset, delete |
| `TestDerivativeIntegration` | create, delete |
| `TestTemplateHelperIntegration` | create_sensor, create_binary_sensor, delete |
| `TestAutomationIntegration` | create, update, toggle, trigger, delete |
| `TestScriptIntegration` | create, update, execute, delete |
| `TestSceneIntegration` | create, update, activate, delete |

### Advanced Feature Tests

| Test Suite | Operations Tested |
|------------|-------------------|
| `TestTodoIntegration` | list, get_items, add_item, update_item (status), remove_item (full CRUD with status filtering) |
| `TestCalendarIntegration` | list, get_events, create_event (datetime + all-day), delete_event (with writable calendar detection) |
| `TestTraceIntegration` | list automation traces, list script traces (execution history) |
| `TestUpdateBlueprintIntegration` | list updates (pending filter), release_notes, list blueprints (automation/script) |
| `TestCameraIntegration`    | list cameras, stream (HLS URL via manage_camera), get_snapshot (binary image data)                  |
| `TestSystemLogIntegration` | GetSystemLog (list), ClearSystemLog (clear + verify empty)                                           |

**Documented Exception (unit tests only):**

| Tool        | Scope           | Reason                                                                                                                                     |
| ----------- | --------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| `get_skill` | Unit tests only | No HA interaction — content is embedded markdown; integration test not technically applicable (documented exception per CLAUDE.md policy). |

**Note:** Read-only tests (traces, updates, blueprints, cameras) verify API integration and response parsing. They skip gracefully if no entities exist or features are unavailable.

## Test Entity Naming Convention

All test entities follow the pattern:

```
__mcptest_<8-char-uuid>_<descriptive-name>
```

Example entity IDs:
- `counter.__mcptest_a1b2c3d4_counter_test`
- `input_boolean.__mcptest_e5f6g7h8_auto_target`
- `automation.__mcptest_i9j0k1l2_automation`

This convention ensures:
1. Test entities are easily identifiable
2. No conflicts with production entities
3. Safe cleanup after test runs

## Cleanup

Tests automatically clean up after themselves using:

1. **Pre-test cleanup**: Removes any leftover test entities from previous runs
2. **Per-test cleanup**: Uses `t.Cleanup()` to ensure cleanup even on test failure
3. **Post-suite verification**: Validates no test entities remain after all tests

If you need to manually clean test entities (e.g., after a crashed test run):

```bash
# Use the HA API or UI to find and delete entities starting with __mcptest_
```

## Troubleshooting

### Tests are skipped

If all tests show as skipped, check that the environment variables are set:

```bash
echo $HA_INTEGRATION_TEST_URL
echo $HA_INTEGRATION_TEST_TOKEN
```

### Connection errors

- Verify the Home Assistant URL is accessible
- Check that the token has not expired
- Ensure no firewall is blocking the connection

### Timeout errors

Increase the timeout:

```bash
export HA_INTEGRATION_TEST_TIMEOUT=10m
```

### Entity creation failures

- Check Home Assistant logs for errors
- Verify the token has sufficient permissions
- Ensure Home Assistant is not in safe mode

## CI/CD Integration

To run integration tests in CI/CD:

```yaml
# GitHub Actions example
integration-tests:
  runs-on: ubuntu-latest
  if: github.event_name == 'workflow_dispatch'  # Manual trigger only
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
      with:
        go-version: '1.26'
    - name: Run Integration Tests
      env:
        HA_INTEGRATION_TEST_URL: ${{ secrets.HA_TEST_URL }}
        HA_INTEGRATION_TEST_TOKEN: ${{ secrets.HA_TEST_TOKEN }}
      run: go test -tags=integration -v ./internal/handlers/integration/...
```

**Important:** Integration tests should not run on every commit. Use manual triggers or schedule them for off-peak hours to avoid overwhelming your Home Assistant instance.
