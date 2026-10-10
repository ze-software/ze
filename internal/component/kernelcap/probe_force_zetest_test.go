//go:build zetest

// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier

package kernelcap

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// VALIDATES: a zetest build, the functional-test DUT, reads
// ze.test.kernelcap.force and names it in the forced row.
// PREVENTS: the functional tests that force a verdict (test/ui/doctor-*-kernelcap.ci,
// test/plugin/kernel-capability-*.ci) silently testing the host's kernel.
func TestZetestBuildReadsTheForceVariable(t *testing.T) {
	t.Setenv("ze_test_kernelcap_force", "alpha=absent")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	withEnrolment(t, capabilityFor("alpha", StatePresent, nil))

	rows := ProbeAll()
	if len(rows) != 1 || rows[0].State != "absent" || !strings.Contains(rows[0].Reason, forceEnv) {
		t.Errorf("rows = %+v, want alpha forced absent naming %s", rows, forceEnv)
	}
}
