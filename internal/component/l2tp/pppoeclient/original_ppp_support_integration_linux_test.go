//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- real client dependency.

package pppoeclient

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/doctor"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/test/userns"
)

// A configured CPE discovers its actual declaration check on the public daemon
// runner. Native pair and QEMU negotiation remain independently mandatory.
func TestClientOriginalPPPSupportNativeRunner(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	if err := pppkernel.CheckOriginalPPPSupport(); err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePostConfig) {
		if check.Name != "pppoe-client-original-ppp-support" {
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
		t.Fatal("CPE check not discovered")
	}
	tree := config.NewTree()
	tree.GetOrCreateContainer("interface").AddListEntry("pppoe-client", "wan", config.NewTree())
	for _, row := range doctor.RunRuntimeChecks(tree, nil) {
		if row.Code == "doctor-ppp-original-unit-attachment" {
			t.Fatalf("actual query42 support prerequisite: %v", row)
		}
	}
}
