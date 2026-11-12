// Design: docs/architecture/mpls/mpls-kernel.md -- label-switching entries on the VPP data plane, doctor wiring tests
package fibvpp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestFibVPPRegistrationDeclaresLabelCheck proves the Stage 1 registration
// fib-vpp's runner sends declares the fib-vpp-label-entries runtime check
// with both codes it reports, so `show doctor` asks fib-vpp's process for it
// and the doctor-coverage gate finds each code declared.
//
// VALIDATES: Phase 13 -- the declared fib-vpp check is wired, not only
// written.
// METHOD: engineRegistration, the value runFibVPPPlugin passes to p.Run.
func TestFibVPPRegistrationDeclaresLabelCheck(t *testing.T) {
	reg := engineRegistration()
	require.Len(t, reg.DoctorChecks, 2)
	check := reg.DoctorChecks[0]
	assert.Equal(t, labelEntriesCheckName, check.Name)
	assert.Equal(t, rpc.DoctorPhaseRuntime, check.Phase)
	assert.ElementsMatch(t, []string{codePathMTUUnenforced, codeStaleLabel}, check.Codes)
	routes := reg.DoctorChecks[1]
	assert.Equal(t, staleRoutesCheckName, routes.Name)
	assert.Equal(t, rpc.DoctorPhaseRuntime, routes.Phase)
	assert.Equal(t, []string{codeStaleRoute}, routes.Codes)
	assert.Equal(t, commandDecls(), reg.Commands, "the runner still sends the commands init declares")
}

// TestFibVPPDoctorCallbackAnswers proves the doctor callback answers the
// declared check from the current writer, answers no finding while there is
// no writer, and refuses a check name fib-vpp did not declare.
//
// VALIDATES: Phase 13 -- the fib-vpp-label-entries check reaches the
// writer's findings.
// METHOD: answerLabelDoctorCheck, which the OnDoctorCheck callback calls
// with the writer read under fibMu.
func TestFibVPPDoctorCallbackAnswers(t *testing.T) {
	findings, err := answerLabelDoctorCheck(labelEntriesCheckName, nil)
	require.NoError(t, err)
	assert.Empty(t, findings, "no writer holds no label state")

	f := newFibVPP(&mockBackend{})
	f.warnPathMTULocked(&mplsfib.Entry{InLabel: 18000, PathMTU: 1400})
	findings, err = answerLabelDoctorCheck(labelEntriesCheckName, f)
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, codePathMTUUnenforced, findings[0].Code)

	_, err = answerLabelDoctorCheck("ldp-label-entries", f)
	require.Error(t, err)
}
