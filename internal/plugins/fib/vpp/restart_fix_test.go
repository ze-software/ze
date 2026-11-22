// Design: docs/architecture/fib/fib-vpp-route-ownership.md -- the FIB source that marks Ze's IP routes in VPP
// Related: doctor_route.go -- staleRouteFix, the fix of the fib-vpp-stale-routes findings
// Related: doctor_label.go -- staleLabelFix, the fix of the stale-label findings

package fibvpp

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/privilege"
)

// VALIDATES: the fib-vpp stale-route and stale-label findings name the restart for
// this process: `sudo systemctl restart ze` when ze runs under ze.service, and the
// stop-and-start steps (ze signal stop, then ze start) for a ze started another way,
// with no systemctl. Neither names `ze signal restart`, which exits without starting
// a new process.
// METHOD: fix the membership through privilege.SetServiceMembershipForTest, then read
// the fix of a stale-route finding answered by the fib-vpp-stale-routes check over a
// refused FIB source, and of a stale-label finding.
// PREVENTS: an operator whose ze runs outside a supervisor told to run a command that
// stops ze and leaves the stale routes forwarding, or told to run systemctl for a ze
// systemd did not start.
func TestFibVPPStaleFixNamesTheRestartForThisProcess(t *testing.T) {
	producers := []struct {
		name string
		fix  func(t *testing.T) string
	}{
		{
			name: "stale-route finding",
			fix: func(t *testing.T) string {
				t.Helper()
				findings, err := answerDoctorCheck(staleRoutesCheckName, nil, fmt.Errorf("%w: retval -1", errFibSource))
				if err != nil {
					t.Fatalf("answerDoctorCheck: %v", err)
				}
				if len(findings) != 1 {
					t.Fatalf("findings = %+v, want one for the refused FIB source", findings)
				}
				return findings[0].Fix
			},
		},
		{
			name: "stale-label finding",
			fix:  func(*testing.T) string { return staleLabelFinding("in-label 100").Fix },
		},
	}
	for _, producer := range producers {
		t.Run(producer.name+"/ze.service", func(t *testing.T) {
			t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))
			if got := producer.fix(t); got != "sudo systemctl restart ze" {
				t.Fatalf("under ze.service the fix is %q, want %q", got, "sudo systemctl restart ze")
			}
		})
		t.Run(producer.name+"/other", func(t *testing.T) {
			t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipOther, nil))
			got := producer.fix(t)
			if strings.Contains(got, "systemctl") {
				t.Errorf("outside ze.service the fix names systemctl: %q", got)
			}
			if strings.Contains(got, "ze signal restart") {
				t.Errorf("the fix names ze signal restart, which leaves ze stopped: %q", got)
			}
			for _, want := range []string{"ze signal stop", "ze start"} {
				if !strings.Contains(got, want) {
					t.Errorf("outside ze.service the fix does not name %q: %q", want, got)
				}
			}
		})
	}
}
