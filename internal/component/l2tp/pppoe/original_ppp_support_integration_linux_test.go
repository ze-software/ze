//go:build integration && linux

// Design: docs/architecture/doctor-and-health-checks.md -- real AC configured check.

package pppoe

import (
	"testing"

	"github.com/ze-software/ze/internal/component/doctor"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/test/userns"
)

// Native AC discovery reaches the actual public runner closure. A VPP
// opposite uses its real selector and never asks for kernel PPP declaration.
func TestPPPoEOriginalPPPSupportNativeRunner(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	if err := pppkernel.CheckOriginalPPPSupport(); err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePostConfig) {
		if check.Name != "pppoe-original-ppp-support" {
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
		t.Fatal("AC check not discovered")
	}
	tree := pppoeBackendTree(t, "interface { backend netlink } pppoe { enabled true; interface access0 {} }")
	for _, row := range doctor.RunRuntimeChecks(tree, nil) {
		if row.Code == "doctor-ppp-original-unit-attachment" {
			t.Fatalf("actual query42 support prerequisite: %v", row)
		}
	}
	tree = pppoeBackendTree(t, "interface { backend vpp } pppoe { enabled true; interface access0 {} }")
	required, err := pppoeOriginalPPPSupportScope(diagnostic.DoctorCheckContext{Tree: tree})
	if err != nil {
		t.Fatal(err)
	}
	if required != 0 {
		t.Fatal("VPP acquired kernel requirements")
	}
}
