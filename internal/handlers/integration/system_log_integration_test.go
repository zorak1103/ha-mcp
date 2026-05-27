//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
)

type SystemLogIntegrationTestSuite struct {
	HelperTestSuite
}

func TestSystemLogIntegration(t *testing.T) {
	suite.Run(t, new(SystemLogIntegrationTestSuite))
}

func (s *SystemLogIntegrationTestSuite) TestGetSystemLog() {
	ctx := context.Background()
	entries, err := s.Client().GetSystemLog(ctx)
	s.Require().NoError(err, "GetSystemLog should not return an error")

	// Entries may be empty on a quiet HA instance, but the call must succeed.
	for _, e := range entries {
		s.NotEmpty(e.Name, "each entry should have a non-empty Name")
		s.NotEmpty(e.Level, "each entry should have a non-empty Level")
		s.NotEmpty(e.Message, "each entry should have a non-empty Message slice")
		s.Greater(e.Timestamp, float64(0), "each entry should have a positive Timestamp")
	}
}

func (s *SystemLogIntegrationTestSuite) TestClearSystemLog() {
	ctx := context.Background()

	// Clear the log buffer.
	err := s.Client().ClearSystemLog(ctx)
	s.Require().NoError(err, "ClearSystemLog should not return an error")

	// After clearing, list should be empty (or at least not fail).
	entries, err := s.Client().GetSystemLog(ctx)
	s.Require().NoError(err, "GetSystemLog after clear should not error")
	s.Empty(entries, "system log should be empty immediately after clear")
}
