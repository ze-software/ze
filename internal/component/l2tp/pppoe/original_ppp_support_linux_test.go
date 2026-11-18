//go:build linux

// Design: docs/architecture/doctor-and-health-checks.md -- selected AC dependency.

package pppoe

import (
	"testing"

	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// Scope uses the real schema-backed backend selector: a configured VPP AC
// and an inactive AC must never acquire native PPP requirements.
func TestPPPoEOriginalPPPSupportRegisteredScope(t *testing.T) {
	var checks int
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePostConfig) {
		if check.Name == "pppoe-original-ppp-support" {
			checks++
		}
	}
	if checks != 1 {
		t.Fatalf("PPPoE registrations %d", checks)
	}
	for _, test := range []struct {
		name   string
		source string
		want   uint32
	}{
		{"empty", "", 0},
		{"inactive", "pppoe { enabled false; interface access0 {} }", 0},
		{"no-access", "pppoe { enabled true }", 0},
		{"default", "pppoe { enabled true; interface access0 {} }", pppkernel.OriginalPPPSupportRequirements(false)},
		{"native", "interface { backend netlink } pppoe { enabled true; interface access0 {} }", pppkernel.OriginalPPPSupportRequirements(false)},
		{"vpp", "interface { backend vpp } pppoe { enabled true; interface access0 {} }", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			tree := pppoeBackendTree(t, test.source)
			got, err := pppoeOriginalPPPSupportScope(diagnostic.DoctorCheckContext{Tree: tree})
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("required %#x want %#x", got, test.want)
			}
		})
	}
}
