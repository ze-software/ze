// Design: docs/architecture/doctor-and-health-checks.md -- the readiness check this component owns
// Related: doctor.go -- missingStateDiagnostic, whose message names the restart

package gtsm

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/privilege"
)

// VALIDATES: the doctor-gtsm-kernel-state finding names the restart for this process:
// `sudo systemctl restart ze` when ze runs under ze.service, and the stop-and-start
// steps (ze signal stop, then ze start) for a ze started another way, with no
// systemctl. Neither names `ze signal restart`, which exits without starting a new
// process.
// METHOD: fix the membership through privilege.SetServiceMembershipForTest, then read
// the message missingStateDiagnostic builds for a peer whose host route is missing.
// PREVENTS: an operator whose ze runs outside a supervisor told to run a command that
// stops ze and leaves the GTSM state unpublished, or told to run systemctl for a ze
// systemd did not start.
func TestGTSMDoctorNamesTheRestartForThisProcess(t *testing.T) {
	message := func() string {
		return missingStateDiagnostic("192.0.2.1", []string{"the host route carrying hop limit 255"}).Message
	}

	t.Run("ze.service", func(t *testing.T) {
		t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))
		const want = "gtsm: 192.0.2.1: the kernel does not hold the host route carrying hop limit 255, " +
			"so a packet below its TTL floor is not dropped before it reaches BGP; " +
			"restart ze to republish the GTSM kernel state: sudo systemctl restart ze"
		if got := message(); got != want {
			t.Fatalf("under ze.service the finding is %q, want %q", got, want)
		}
	})
	t.Run("other", func(t *testing.T) {
		t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipOther, nil))
		got := message()
		if strings.Contains(got, "systemctl") {
			t.Errorf("outside ze.service the finding names systemctl: %q", got)
		}
		if strings.Contains(got, "ze signal restart") {
			t.Errorf("the finding names ze signal restart, which leaves ze stopped: %q", got)
		}
		for _, want := range []string{"ze signal stop", "ze start"} {
			if !strings.Contains(got, want) {
				t.Errorf("outside ze.service the finding does not name %q: %q", want, got)
			}
		}
	})
}
