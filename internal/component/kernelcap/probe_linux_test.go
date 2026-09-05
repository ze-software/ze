//go:build linux

// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
//
// The XFRM probe's classification, driven by errno rather than by whatever the
// host's kernel happens to hold, plus one test that runs the REAL probe on this
// host to prove it needs no privilege.

package kernelcap

import (
	"errors"
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

// withXFRMOpen swaps the socket open for the duration of a test.
func withXFRMOpen(t *testing.T, err error) {
	t.Helper()
	original := xfrmOpen
	xfrmOpen = func() error { return err }
	t.Cleanup(func() { xfrmOpen = original })
}

// VALIDATES: AC-13. The probe opens NETLINK_XFRM, which needs no CAP_NET_ADMIN,
// so an unprivileged reader on a host that HAS XFRM is told it is present.
// PREVENTS: the failure of every probe this one replaces. xfrmAvailable and
// probeXFRM both dumped the Security Policy Database, which needs CAP_NET_ADMIN,
// so an unprivileged `ze doctor` on a healthy kernel was told XFRM was missing --
// and under the refusal this gate adds, that answer would stop the daemon.
func TestXFRMCapabilityUnprivilegedReportsPresence(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("this test says something only when the process is unprivileged")
	}
	if err := openXFRMNetlink(); err != nil {
		t.Skipf("this host holds no XFRM dataplane, so presence cannot be asserted: %v", err)
	}

	result := XFRM()
	if result.State != StatePresent {
		t.Fatalf("an unprivileged probe on a host with XFRM reported %v: %v", result.State, result.Reason)
	}
}

// VALIDATES: absence is the errno that means the kernel carries no XFRM netlink
// protocol, and a permission denial is NOT absence.
// PREVENTS: an operator rebuilding a kernel for a capability the host already
// has. A probe that conflates the two answers refuses a start for a privilege
// fault, which no kernel rebuild fixes (A-2).
func TestXFRMCapabilityAbsentIsEPROTONOSUPPORT(t *testing.T) {
	for name, tc := range map[string]struct {
		err   error
		state State
	}{
		"protocol not supported":       {unix.EPROTONOSUPPORT, StateAbsent},
		"address family not supported": {unix.EAFNOSUPPORT, StateAbsent},
		"permission denied":            {unix.EPERM, StateUnknown},
		"access denied":                {unix.EACCES, StateUnknown},
		"too many open files":          {unix.EMFILE, StateUnknown},
		"an error with no errno":       {errors.New("netlink handle failed"), StateUnknown},
	} {
		t.Run(name, func(t *testing.T) {
			withXFRMOpen(t, tc.err)
			result := XFRM()
			if result.State != tc.state {
				t.Errorf("%v classified as %v, want %v", tc.err, result.State, tc.state)
			}
			if result.Reason == nil {
				t.Error("a faulty verdict carries no reason, so the message can name no action")
			}
		})
	}

	t.Run("a successful open is present", func(t *testing.T) {
		withXFRMOpen(t, nil)
		result := XFRM()
		if result.State != StatePresent {
			t.Errorf("a successful open classified as %v, want present", result.State)
		}
		if result.Reason != nil {
			t.Errorf("a present capability carries a reason: %v", result.Reason)
		}
	})
}

// VALIDATES: the test override drives all three verdicts, which is what the
// functional tests need to reach the absent and cannot-determine branches on a
// host whose kernel is healthy.
// PREVENTS: a functional test that passes only where XFRM happens to be absent,
// which is the vacuity trap of ai/rules/interop-and-goal-validation.md. A typo in
// the variable must not be read as an answer either: it leaves the real probe in
// charge rather than deciding a start on a misspelling.
func TestXFRMOverrideDrivesEveryVerdict(t *testing.T) {
	for value, want := range map[string]State{
		"present": StatePresent,
		"absent":  StateAbsent,
		"unknown": StateUnknown,
	} {
		t.Run(value, func(t *testing.T) {
			withXFRMOpen(t, errors.New("the real probe must not be consulted"))
			forced, ok := forcedXFRMFor(value)
			if !ok {
				t.Fatalf("%q was not read as an override", value)
			}
			if forced.State != want {
				t.Errorf("%q forced %v, want %v", value, forced.State, want)
			}
		})
	}

	if _, ok := forcedXFRMFor("abcent"); ok {
		t.Error("a misspelled override was read as an answer; it must leave the real probe in charge")
	}
	if _, ok := forcedXFRMFor(""); ok {
		t.Error("an unset override was read as an answer")
	}
}
