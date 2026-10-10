//go:build !zetest

// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier

package kernelcap

import (
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// VALIDATES: a build without the zetest tag, which is every shipped ze, ignores
// ze.test.kernelcap.force in both its spellings and runs the real probe.
// METHOD: set the variable in the process environment the way an operator
// would, then probe a capability whose real answer is absent.
// PREVENTS: the forced answer becoming the operator override the owner refused
// (2026-08-14): a shipped daemon told "present" by its environment would start
// on a kernel that lacks the feature.
func TestShippedBuildIgnoresTheForceVariable(t *testing.T) {
	for _, spelling := range []string{forceEnv, "ze_test_kernelcap_force"} {
		t.Run(spelling, func(t *testing.T) {
			t.Setenv(spelling, "alpha=present")
			env.ResetCache()
			t.Cleanup(env.ResetCache)
			withEnrolment(t, capabilityFor("alpha", StateAbsent, nil))

			rows := ProbeAll()
			if len(rows) != 1 || rows[0].State != "absent" {
				t.Errorf("rows = %+v, want alpha absent from the real probe", rows)
			}
		})
	}
}
