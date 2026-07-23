//go:build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// ToolDispatchHarnessTestSuite is a minimal, permanent canary proving CallTool
// actually reaches the real HybridClient through the registry+handler layer.
// If this ever breaks, every *_tool_dispatch_integration_test.go file is
// silently not testing what it claims to.
type ToolDispatchHarnessTestSuite struct {
	IntegrationTestSuite
}

func TestToolDispatchHarness(t *testing.T) {
	suite.Run(t, new(ToolDispatchHarnessTestSuite))
}

// TestCallToolReachesRealClient calls the read-only get_state tool on
// sun.sun - present on every default Home Assistant installation - so this
// test never creates, modifies, or deletes anything.
func (s *ToolDispatchHarnessTestSuite) TestCallToolReachesRealClient() {
	// format=json is requested explicitly so the response is guaranteed to
	// echo back the entity_id - the default natural-language format renders
	// the friendly name (e.g. "Sun") instead, which wouldn't prove much.
	result := s.CallTool("get_state", map[string]any{"entity_id": "sun.sun", "format": "json"})
	s.False(result.IsError, "get_state on sun.sun should succeed: %s", resultText(result))
	s.Contains(resultText(result), "sun.sun")
}
