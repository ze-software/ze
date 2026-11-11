package fibvpp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/mplsfib"
)

// TestFibVPPLabelDoctorReportsPathMTU proves the fib-vpp-label-entries check
// reports, once an entry carried a path MTU VPP cannot install, a warning
// under doctor-fib-vpp-mpls-path-mtu with the command that bounds labeled
// frames, and reports nothing before.
//
// VALIDATES: AC-26 -- the dropped path MTU reaches `show doctor`, not only
// the warn-once log line; the code is described by `ze show diagnostic`.
// METHOD: warnPathMTULocked over an entry with and without PathMTU, then
// labelDoctorFindings; the code's explanation is looked up in the registry.
func TestFibVPPLabelDoctorReportsPathMTU(t *testing.T) {
	f := &fibVPP{}
	f.warnPathMTULocked(&mplsfib.Entry{InLabel: 18000})
	require.Empty(t, f.labelDoctorFindings(), "an entry with no path MTU loses nothing")

	f.warnPathMTULocked(&mplsfib.Entry{InLabel: 18000, PathMTU: 1400})
	findings := f.labelDoctorFindings()
	require.Len(t, findings, 1)
	assert.Equal(t, codePathMTUUnenforced, findings[0].Code)
	assert.Equal(t, string(diagnostic.SeverityWarning), findings[0].Severity)
	assert.Contains(t, findings[0].Fix, "mtu")
	assert.NotNil(t, diagnostic.Lookup(codePathMTUUnenforced), "ze show diagnostic describes the code")
	assert.NotNil(t, diagnostic.Lookup(mplsfib.CodeDoctorNoWriter), "the writer's refusal code is described too")
}
