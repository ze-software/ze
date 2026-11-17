// Design: docs/guide/bgp-policy.md -- read-only export route-kind simulation.
package policy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPolicyTestAcceptsSimulatedExportKind proves both declared route classes
// can be selected without changing the UPDATE or the policy under test.
func TestPolicyTestAcceptsSimulatedExportKind(t *testing.T) {
	for _, kind := range []string{"learned", "local"} {
		t.Run(kind, func(t *testing.T) {
			got, err := parsePolicyTestArgs([]string{"export", "route-kind", kind, "update", "AA"})
			require.NoError(t, err)
			require.Equal(t, kind, got.simulatedKind)
		})
	}
}

// TestPolicyTestRefusesImportSimulation proves the export-only argument does
// not silently change the meaning of an import test.
func TestPolicyTestRefusesImportSimulation(t *testing.T) {
	_, err := parsePolicyTestArgs([]string{"import", "route-kind", "local", "update", "AA"})
	require.Error(t, err)
}

// TestPolicyTestRefusesUnknownSimulation proves caller text cannot select a
// route class outside the two read-only choices.
func TestPolicyTestRefusesUnknownSimulation(t *testing.T) {
	_, err := parsePolicyTestArgs([]string{"export", "route-kind", "role", "update", "AA"})
	require.Error(t, err)
}

// TestPolicyTestSimulationRequiresOneValue preserves the declared once-only
// keyword and refuses a missing value before a dry run can begin.
func TestPolicyTestSimulationRequiresOneValue(t *testing.T) {
	for _, test := range []struct {
		args []string
		want error
	}{
		{[]string{"export", "update", "AA", "route-kind"}, errMissingRouteKind},
		{[]string{"export", "update", "AA", "route-kind", "learned", "route-kind", "local"}, errRepeatedRouteKind},
	} {
		_, err := parsePolicyTestArgs(test.args)
		require.ErrorIs(t, err, test.want)
	}
	require.True(t, isPolicyTestKeyword("route-kind"))
}
