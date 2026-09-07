//go:build integration

package integration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
)

// CallServiceIntegrationTestSuite exercises call_service through the real tool
// registry, handler, client, and formatter layers.
type CallServiceIntegrationTestSuite struct {
	IntegrationTestSuite
}

func TestCallServiceIntegration(t *testing.T) {
	suite.Run(t, new(CallServiceIntegrationTestSuite))
}

func (s *CallServiceIntegrationTestSuite) TestResponseTypeServiceThroughTool() {
	states, err := s.Client().GetStates(s.Context())
	s.Require().NoError(err, "failed to get states")

	var weatherEntityID string
	for _, state := range states {
		if len(state.EntityID) > len("weather.") && state.EntityID[:len("weather.")] == "weather." {
			weatherEntityID = state.EntityID
			break
		}
	}
	if weatherEntityID == "" {
		s.T().Skip("no weather entities found in Home Assistant")
	}

	services, err := s.Client().GetServices(s.Context())
	s.Require().NoError(err, "failed to get Home Assistant services")
	var hasForecastService bool
	for _, serviceDomain := range services {
		if serviceDomain.Domain != "weather" {
			continue
		}
		_, hasForecastService = serviceDomain.Services["get_forecasts"]
		break
	}
	if !hasForecastService {
		s.T().Skip("weather.get_forecasts is unavailable in Home Assistant")
	}

	result := s.CallTool("call_service", map[string]any{
		"domain":          "weather",
		"service":         "get_forecasts",
		"data":            map[string]any{"entity_id": weatherEntityID, "type": "daily"},
		"format":          "json",
		"return_response": true,
	})
	s.False(result.IsError, "response-type call should succeed: %s", resultText(result))

	var output map[string]any
	s.Require().NoError(json.Unmarshal([]byte(resultText(result)), &output), "response output should be valid JSON")
	s.Equal(true, output["success"])
	s.Equal("weather", output["domain"])
	s.Equal("get_forecasts", output["service"])
	s.Contains(output, "response")

	withoutFlag := s.CallTool("call_service", map[string]any{
		"domain":  "weather",
		"service": "get_forecasts",
		"data":    map[string]any{"entity_id": weatherEntityID, "type": "daily"},
	})
	s.True(withoutFlag.IsError, "response-type service without return_response should fail: %s", resultText(withoutFlag))
}
