//go:build integration && linux

// Design: docs/architecture/doctor-and-health-checks.md -- real L2TP-only check.

package l2tp

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/doctor"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/test/userns"
)

// The actual registered L2TP closure runs through the public daemon runner.
// Requiring the actual support probe first prevents a missing scope from
// masquerading as positive support. Existing real-pair tests prove mutation.
func TestL2TPOriginalPPPSupportNativeRunner(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	if err := pppkernel.CheckOriginalPPPSupport(); err != nil {
		t.Fatal(err)
	}
	tree := config.NewTree()
	tree.GetOrCreateContainer("l2tp")
	var found bool
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePostConfig) {
		if check.Name != "l2tp-original-ppp-support" {
			continue
		}
		found = true
		check.Name += "-native-runner"
		check.Phase = diagnostic.DoctorPhaseRuntime
		if err := diagnostic.RegisterDoctorCheck(check); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { diagnostic.UnregisterDoctorCheckForTest(check.Name) })
	}
	if !found {
		t.Fatal("L2TP-only check not discovered")
	}
	for _, row := range doctor.RunRuntimeChecks(tree, nil) {
		if row.Code == "doctor-ppp-original-unit-attachment" {
			t.Fatalf("actual query42 support prerequisite: %v", row)
		}
	}
}
