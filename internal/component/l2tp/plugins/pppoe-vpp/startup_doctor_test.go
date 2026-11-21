// Design: docs/architecture/l2tp/bng-5-pppoe.md -- VPP startup prerequisites

package pppoevpp

import (
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// The real feature import must publish a complete native doctor registration,
// including its live socket dependency and metadata used by show diagnostic.
func TestVPPPPPoEStartupDoctorRegistration(t *testing.T) {
	const code = "doctor-vpp-pppoe-plugin"
	if diagnostic.Lookup(code) == nil {
		t.Fatal("PPPoE VPP diagnostic metadata missing")
	}
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePostConfig) {
		if check.Name != "vpp-startup-pppoe" {
			continue
		}
		if check.Component != "vpp" {
			t.Fatalf("component=%s", check.Component)
		}
		if len(check.Dependencies) != 1 {
			t.Fatalf("dependencies=%v", check.Dependencies)
		}
		if check.Dependencies[0] != "vpp-api-socket" {
			t.Fatalf("dependency=%s", check.Dependencies[0])
		}
		if len(check.Codes) != 1 {
			t.Fatalf("codes=%v", check.Codes)
		}
		if check.Codes[0] != code {
			t.Fatalf("code=%s", check.Codes[0])
		}
		if check.Check == nil {
			t.Fatal("doctor callback missing")
		}
		return
	}
	t.Fatal("PPPoE VPP startup doctor check missing")
}
