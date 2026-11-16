// Design: docs/architecture/bgp/egress-attribute-rules.md -- observable marked-route refusals.
// Related: filter_atomic_diagnostic.go -- the bounded report producer.

package reactor

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// TestAtomicAggregatePolicyDiagnostic reports a real refusal twice and checks
// the shared doctor/event boundary: one historical event, a described code,
// an actionable editor command, and no persistent active warning.
func TestAtomicAggregatePolicyDiagnostic(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	metadata := diagnostic.Lookup(codeAtomicAggregatePolicy)
	require.NotNil(t, metadata)
	require.NotEmpty(t, metadata.Description)
	require.Contains(t, diagnostic.EventCodes(), codeAtomicAggregatePolicy)
	require.LessOrEqual(t, len(atomicAggregatePolicyFix), report.FixMax)

	reportAtomicAggregatePolicyRefusal("192.0.2.9", directionExport, nil)
	reportAtomicAggregatePolicyRefusal("192.0.2.9", directionExport, errors.New("invalid policy output"))
	issues := report.Errors(0)
	require.Len(t, issues, 1, "a looping filter must not flood the shared event ring")
	require.Equal(t, codeAtomicAggregatePolicy, issues[0].Code)
	require.Equal(t, "192.0.2.9", issues[0].Subject)
	require.Equal(t, "ze config edit", issues[0].Fix)
	require.Contains(t, issues[0].Message, "source-route removal is requested")
	require.Empty(t, report.Warnings(), "recovery must not leave a stale active warning")
}

// TestAtomicAggregateRetirementDiagnostic preserves the preparation cause in
// one bounded historical event per peer, without claiming a withdrawal ran.
func TestAtomicAggregateRetirementDiagnostic(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	metadata := diagnostic.Lookup(codeAtomicAggregateRetirement)
	require.NotNil(t, metadata)
	require.Contains(t, diagnostic.EventCodes(), codeAtomicAggregateRetirement)
	require.NotEmpty(t, metadata.Description)
	require.LessOrEqual(t, len(atomicAggregatePolicyFix), report.FixMax)

	reportAtomicAggregateRetirementFailure("192.0.2.9", errPolicyRemovalEncoding)
	reportAtomicAggregateRetirementFailure("192.0.2.9", errors.New("repeated preparation failure"))
	issues := report.Errors(0)
	require.Len(t, issues, 1, "repeated preparation failures must not flood the event ring")
	require.Equal(t, reportSourceBGP, issues[0].Source)
	require.Equal(t, codeAtomicAggregateRetirement, issues[0].Code)
	require.Equal(t, "192.0.2.9", issues[0].Subject)
	require.Equal(t, atomicAggregatePolicyFix, issues[0].Fix)
	require.Contains(t, issues[0].Message, errPolicyRemovalEncoding.Error())
	require.Contains(t, issues[0].Message, "previously advertised routes may remain")
	require.NotContains(t, issues[0].Message, "source routes are withdrawn")

	reportAtomicAggregateRetirementFailure("192.0.2.10", errors.New(strings.Repeat("x", report.MessageMax+1)))
	issues = report.Errors(0)
	require.Len(t, issues, 2, "an oversized cause must still produce an event for another peer")
	for _, issue := range issues {
		require.LessOrEqual(t, len(issue.Message), report.MessageMax)
	}
	require.Empty(t, report.Warnings(), "a transient preparation failure must not leave an active warning")
}
