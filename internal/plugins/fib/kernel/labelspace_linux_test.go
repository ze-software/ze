//go:build linux

package fibkernel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
)

// procRootEnv names the /proc root every doctor-tier probe and this repair read.
// It is registered by internal/component/kernelcap, which owns ProcPath.
const procRootEnv = "ze.test.doctor.procfs-root"

// withLabelSpace builds a fake /proc whose net.mpls.platform_labels holds value,
// points the repair at it, and hands back the path so a case can read it again.
func withLabelSpace(t *testing.T, value string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "sys", "net", "mpls")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	path := filepath.Join(dir, "platform_labels")
	require.NoError(t, os.WriteFile(path, []byte(value), 0o644))

	old := env.Get(procRootEnv)
	require.NoError(t, env.Set(procRootEnv, root))
	t.Cleanup(func() { _ = env.Set(procRootEnv, old) })
	return path
}

// TestMPLSPlatformLabelsWritten proves AC-10.
//
// VALIDATES: a kernel whose AF_MPLS table exists with a label space of 0 gets a
// non-zero label space written before ze programs a label. Ze does not refuse and
// does not merely warn.
// PREVENTS: the state every appliance boots in staying dead. ze's runtime kernel
// builds MPLS in, so the table exists and the sysctl still defaults to 0, which
// disables MPLS entirely: ze would program labels the kernel refuses.
func TestMPLSPlatformLabelsWritten(t *testing.T) {
	path := withLabelSpace(t, "0\n")

	repairLabelSpace()

	written, err := os.ReadFile(path) //nolint:gosec // a path this test created
	require.NoError(t, err)
	assert.Equal(t, "1048575", string(written),
		"a zero label space must be replaced by one that covers every 20-bit label")
}

// VALIDATES: an operator's own label space is never overwritten, and a table that
// is already sized is left alone.
// PREVENTS: ze silently undoing a `net.mpls.platform_labels` an operator set in
// the sysctl {} block, which is the surface the repair defers to.
func TestMPLSPlatformLabelsLeavesAnOperatorValueAlone(t *testing.T) {
	path := withLabelSpace(t, "4096\n")

	repairLabelSpace()

	written, err := os.ReadFile(path) //nolint:gosec // a path this test created
	require.NoError(t, err)
	assert.Equal(t, "4096\n", string(written), "an operator's label space must survive the repair")
}

// VALIDATES: a kernel with no AF_MPLS table is left untouched. The capability
// gate has already refused the start for it, and the repair must not create the
// file that would make an absent capability look present.
// PREVENTS: the repair defeating the gate that runs before it.
func TestMPLSPlatformLabelsNotCreatedWhenTheTableIsAbsent(t *testing.T) {
	root := t.TempDir()
	old := env.Get(procRootEnv)
	require.NoError(t, env.Set(procRootEnv, root))
	t.Cleanup(func() { _ = env.Set(procRootEnv, old) })

	repairLabelSpace()

	_, err := os.Stat(kernelcap.MPLSPlatformLabelsPath())
	assert.True(t, os.IsNotExist(err), "the repair created a sysctl the kernel does not hold: %v", err)
	assert.Equal(t, kernelcap.StateAbsent, kernelcap.MPLS().State,
		"the capability must still read as absent after the repair declined to act")
}

// VALIDATES: the fib kernel plugin enrols the MPLS capability, so ze doctor
// reports it, the startup gate refuses on it and ze explain resolves its codes.
// PREVENTS: the capability existing as dead code, which is what an unregistered
// enrolment is (ai/rules/completion.md). It also pins the ownership: a VPP or P4
// backend never links this package, so it never carries this requirement.
func TestMPLSCapabilityEnrolled(t *testing.T) {
	assert.Contains(t, kernelcap.Enrolled(), "mpls",
		"the plugin that programs kernel labels must enrol the kernel requirement")

	var found *diagnostic.DoctorCheck
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePostConfig) {
		if check.Name == "kernel-capability-mpls" {
			found = &check
			break
		}
	}
	require.NotNil(t, found, "no kernel-capability-mpls doctor check was registered")
	require.NotNil(t, found.Check, "kernel-capability-mpls has a nil Check function")
	assert.Contains(t, found.Codes, diagnosticMPLSUnavailable)
	assert.Contains(t, found.Codes, diagnosticMPLSUnknown)
}
