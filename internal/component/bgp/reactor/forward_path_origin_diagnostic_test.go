// Design: docs/architecture/doctor-and-health-checks.md -- original mapping provenance refusal.
package reactor

import (
	"errors"
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"strings"
	"testing"
)

// An actual refusal enters the shared doctor event ring once per peer, carries
// its cause and recovery, and stays bounded for an oversized independent cause.
func TestForwardPathOriginDiagnosticKeepsCauseAndRecovery(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	require.NotNil(t, diagnostic.Lookup(codeForwardPathOrigin))
	require.Contains(t, diagnostic.EventCodes(), codeForwardPathOrigin)
	reportForwardPathOriginFailure("192.0.2.143", errors.New("original connection is missing"))
	reportForwardPathOriginFailure("192.0.2.143", errors.New("repeated origin refusal"))
	events := report.Errors(0)
	require.Len(t, events, 1)
	require.Equal(t, codeForwardPathOrigin, events[0].Code)
	require.Contains(t, events[0].Message, "original connection is missing")
	require.Equal(t, "ze request restart", events[0].Fix)
	reportForwardPathOriginFailure("192.0.2.144", errors.New(strings.Repeat("x", report.MessageMax+1)))
	events = report.Errors(0)
	require.Len(t, events, 2)
	for _, event := range events {
		require.LessOrEqual(t, len(event.Message), report.MessageMax)
	}
	require.Empty(t, report.Warnings())
}
