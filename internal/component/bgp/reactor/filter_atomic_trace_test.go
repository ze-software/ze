// Design: docs/architecture/bgp/egress-attribute-rules.md -- dry runs show source retirement.
// Related: policy_dryrun_library.go -- the operator's policy-test renderer.
package reactor

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
)

// TestAtomicRetirementDryRunShowsWithdrawal exercises the same trace entry
// point as show policy test: the excluded A is shown before useful B, and the
// returned NLRI display includes both effects for changed-attrs calculation.
func TestAtomicRetirementDryRunShowsWithdrawal(t *testing.T) {
	first, second := []byte{24, 10, 42, 0}, []byte{24, 10, 43, 0}
	for _, direction := range []string{directionImport, directionExport} {
		t.Run(direction, func(t *testing.T) {
			f := newAIGPReplayFixture(t, nil)
			input := rfc4271ProofRelayBody(append(bytes.Clone(first), second...), true, 10)
			output := rfc4271ProofRelayBody(second, true, 77)
			f.r.filterTransportSeam = rawAnswerTransport(output)
			policy := answerPolicy(t, routepolicy.FinalAccept, "trace")
			update := wireu.NewWireUpdate(input, f.ctxID)
			before, err := displayUpdate(update)
			require.NoError(t, err)
			dry := libraryDryRun{wireUpdate: update, payload: input, ctxID: f.ctxID,
				direction: direction, peer: f.destination.Settings().Address.String(),
				session: policyview.Session{EBGP: true}, intent: libraryIntentContext{asn4: true, asPathEditable: true}}
			result, after, trace, _ := f.r.traceLibraryChain(policy.Library(), &dry, &before)
			require.Equal(t, PolicyAccept, result.Action)
			require.False(t, result.Failed)
			require.False(t, result.Withdrawn, "the retained B is still an accepted announcement")
			require.NotNil(t, after)
			require.Len(t, trace, 1)
			lines := strings.Split(result.Text, "\n")
			require.Len(t, lines, 2, "the display must show the actual preceding withdrawal")
			require.Equal(t, "nlri del 10.42.0.0/24", lines[0])
			require.Contains(t, lines[1], "med 77")
			require.Contains(t, lines[1], "nlri add 10.43.0.0/24")
			require.Equal(t, 1, strings.Count(result.Text, "10.42.0.0/24"))
			require.Equal(t, 1, strings.Count(result.Text, "10.43.0.0/24"))
			require.Contains(t, string(after.nlri), "nlri del 10.42.0.0/24")
			require.Contains(t, string(after.nlri), "nlri add 10.43.0.0/24")
			require.Contains(t, before.changed(after), "nlri")
			require.Zero(t, policy.Library().Refused(), "display-only evaluation must not count refusals")
		})
	}
}
