// Design: docs/architecture/doctor-and-health-checks.md -- the runtime phase: checks only the daemon runs
// Related: spd_policy.go -- spdInstallFixRestart, the restart fix of the SPD findings
// Related: doctor_ike_socket_bypass.go -- checkIKESocketBypass, whose fix is transport.BypassFix
// Related: unmatched.go -- catchAllInstallHint and unmatchedStartRefusal
// Related: spd_forward_bypass.go -- spdForwardBypassRefusal, which names a restart when a read failed

package engine

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/privilege"
)

// VALIDATES: every IPsec operator text that tells the operator to restart ze names the
// restart for this process: `sudo systemctl restart ze` when ze runs under ze.service,
// and the stop-and-start steps (ze signal stop, then ze start) for a ze started another
// way, with no systemctl. Neither ever names `ze signal restart`, which exits without
// starting a new process.
// METHOD: fix the membership through privilege.SetServiceMembershipForTest, then read
// each producer's text: the ipsec-ike-socket-bypass finding for each cause that ends in
// a restart (a missing capability ends in the unit reinstall, which
// TestRefusedIKESocketBypassNamesTheRestartForThisProcess pins), the SPD start refusals
// (catch-all, interface catch-all), the override-guard finding, the MOBIKE shared-socket
// failure, the owned catch-all hint and the forward-bypass refusal whose read failed.
// PREVENTS: an operator whose ze runs outside a supervisor told to run a command that
// stops ze and leaves it stopped, or told to run systemctl for a ze systemd did not start.
func TestIPsecRestartFixNamesTheRestartForThisProcess(t *testing.T) {
	cause := errors.New("operation not permitted")
	producers := []struct {
		name string
		// exact is the whole text under ze.service; empty when the restart is a part of a
		// longer text, which is then read for the restart alone.
		exact string
		text  func() string
	}{
		{
			name:  "ipsec-ike-socket-bypass finding, kernel without XFRM",
			exact: "boot a kernel built with CONFIG_XFRM (Ze's own kernel carries it), then restart ze: sudo systemctl restart ze",
			text:  func() string { return ikeSocketBypassFix(syscall.ENOPROTOOPT) },
		},
		{
			name: "ipsec-ike-socket-bypass finding, security module",
			exact: "find the denial with sudo journalctl -b --grep 'avc:|apparmor=\"DENIED\"', allow ze the " +
				"IP_XFRM_POLICY socket option in that policy, then restart ze: sudo systemctl restart ze",
			text: func() string { return ikeSocketBypassFix(syscall.EACCES) },
		},
		{
			name: "ipsec-ike-socket-bypass finding, unclassified",
			exact: "if ze lacks CAP_NET_ADMIN: sudo systemctl stop ze && sudo ze install systemd --force --start; " +
				"if the kernel lacks CONFIG_XFRM: boot a kernel built with CONFIG_XFRM (Ze's own kernel carries it), " +
				"then restart ze: sudo systemctl restart ze",
			text: func() string { return ikeSocketBypassFix(syscall.EINVAL) },
		},
		{
			name:  "SPD catch-all start refusal",
			exact: "sudo systemctl restart ze",
			text:  func() string { return unmatchedStartRefusal(dataplane.SPActionDiscard, cause).Fix() },
		},
		{
			name:  "interface catch-all start refusal",
			exact: "sudo systemctl restart ze",
			text:  func() string { return interfaceCatchAllStartRefusal("xfrm0", cause).Fix() },
		},
		{
			name:  "override-guard finding",
			exact: "sudo systemctl restart ze",
			text:  func() string { return overrideGuardDiagnostic("the guard reads detached").Fix },
		},
		{
			name:  "MOBIKE shared-socket failure",
			exact: "sudo systemctl restart ze",
			text:  func() string { return mobikeSocketNotSharedFailure("site-a", "192.0.2.1").Fix() },
		},
		{
			name: "owned catch-all hint",
			text: func() string {
				return catchAllInstallHint(&dataplane.PolicyOwnedError{HeldBy: "peer-a", Wanted: "catch-all"})
			},
		},
		{
			name: "forward-bypass refusal after a failed read",
			text: func() string {
				p := ipsec.SPDPolicy{Name: "web", LocalPort: ipsec.AnyPort(), RemotePort: ipsec.AnyPort()}
				return spdForwardBypassRefusal(p, errors.New("read failed"), nil).Why()
			},
		},
	}
	for _, producer := range producers {
		t.Run(producer.name+"/ze.service", func(t *testing.T) {
			t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))
			got := producer.text()
			if producer.exact != "" && got != producer.exact {
				t.Fatalf("under ze.service the text is %q, want %q", got, producer.exact)
			}
			if !strings.Contains(got, "sudo systemctl restart ze") {
				t.Errorf("under ze.service the text does not name sudo systemctl restart ze: %q", got)
			}
			if strings.Contains(got, "ze signal restart") {
				t.Errorf("the text names ze signal restart, which leaves ze stopped: %q", got)
			}
		})
		t.Run(producer.name+"/other", func(t *testing.T) {
			t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipOther, nil))
			got := producer.text()
			if strings.Contains(got, "systemctl") {
				t.Errorf("outside ze.service the text names systemctl: %q", got)
			}
			if strings.Contains(got, "ze signal restart") {
				t.Errorf("the text names ze signal restart, which leaves ze stopped: %q", got)
			}
			for _, want := range []string{"ze signal stop", "ze start"} {
				if !strings.Contains(got, want) {
					t.Errorf("outside ze.service the text does not name %q: %q", want, got)
				}
			}
		})
	}
}

// ikeSocketBypassFix answers the fix of the ipsec-ike-socket-bypass finding for one IKE
// socket whose bypass the kernel refused with errno, wrapped as
// dataplane.BypassIKESocket wraps it.
func ikeSocketBypassFix(errno syscall.Errno) string {
	refusal := fmt.Errorf("transport: setsockopt IP_XFRM_POLICY, direction 1: %w", errno)
	return checkIKESocketBypass([]ikeSocketBypassFailure{{address: "192.0.2.1:500", err: refusal}})[0].Fix
}
