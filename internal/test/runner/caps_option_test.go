// VALIDATES: option=needs-linux[:caps=net-admin] parsing -- a bare needs-linux
// keeps its previous meaning, caps=net-admin gates on the capability in BOTH
// polarities, and an unknown caps value is rejected on every host.
// PREVENTS: the regression this option was added to fix. Seven reload tests
// applied interface config on an unprivileged Linux host, where the interface
// plugin dies with "operation not permitted" during its stage-3 handshake. The
// daemon then never reached the state the tests assert, so they did not fail --
// they HUNG until the suite timeout, and were mistaken for load-sensitive
// flakes. Silently dropping the caps gate would restore that hang; silently
// skipping a bare needs-linux test would delete coverage instead.
//
// The probe is injected rather than read from the host. An earlier version of
// this file asserted only the host's real answer, which made the gate's own
// regression test VACUOUS on a machine that HAS the capability -- precisely the
// QEMU runner, the one environment where these tests execute for real.

package runner

import (
	"runtime"
	"strings"
	"testing"
)

func parseNeedsLinux(t *testing.T, line string) *Record {
	t.Helper()
	et := &EncodingTests{}
	r := newRecord("needs-linux-test")
	if err := et.parseLine(r, "test/reload/fake.ci", line); err != nil {
		t.Fatalf("parseLine(%q): %v", line, err)
	}
	return r
}

// withNetAdmin forces the capability probe's answer for one test.
func withNetAdmin(t *testing.T, present bool) {
	t.Helper()
	original := hasNetAdmin
	hasNetAdmin = func() bool { return present }
	t.Cleanup(func() { hasNetAdmin = original })
}

// A bare needs-linux must NOT consult capabilities: on Linux it runs, whatever
// privileges the process holds. Skipping it would delete coverage.
func TestNeedsLinuxWithoutCapsIgnoresCapabilities(t *testing.T) {
	withNetAdmin(t, false)

	r := parseNeedsLinux(t, "option=needs-linux")
	if !r.NeedsLinux {
		t.Fatal("NeedsLinux not set")
	}
	if runtime.GOOS != goosLinux {
		return
	}
	if r.SkipReason != "" {
		t.Fatalf("bare needs-linux skipped on Linux with no capability: %q -- that deletes coverage", r.SkipReason)
	}
}

// caps=net-admin must SKIP when the capability is absent, and the reason must
// name the runner that does have it.
func TestNeedsLinuxNetAdminSkipsWithoutCapability(t *testing.T) {
	withNetAdmin(t, false)

	r := parseNeedsLinux(t, "option=needs-linux:caps=net-admin")
	if runtime.GOOS != goosLinux {
		return // the GOOS skip fires first, and is covered by the bare-option test
	}
	if r.SkipReason == "" {
		t.Fatal("caps=net-admin ran without CAP_NET_ADMIN: the test will hang instead of skipping")
	}
	if !strings.Contains(r.SkipReason, "ze-qemu-needs-linux-test") {
		t.Fatalf("skip reason %q does not name the runner that can run this test", r.SkipReason)
	}
}

// ...and must RUN when the capability is present. Without this polarity the
// gate could skip unconditionally and no test would notice.
func TestNeedsLinuxNetAdminRunsWithCapability(t *testing.T) {
	withNetAdmin(t, true)

	r := parseNeedsLinux(t, "option=needs-linux:caps=net-admin")
	if runtime.GOOS != goosLinux {
		return
	}
	if r.SkipReason != "" {
		t.Fatalf("caps=net-admin skipped WITH the capability present: %q -- the seven gated tests would never run, even in QEMU", r.SkipReason)
	}
}

// An unrecognized caps value must be rejected on EVERY host. A typo is a
// property of the .ci file, not of the machine parsing it: validating it after
// the GOOS early-return silently accepted `caps=net-admn` on macOS, so an author
// working there got no feedback and the gate was quietly disabled.
func TestNeedsLinuxRejectsUnknownCaps(t *testing.T) {
	et := &EncodingTests{}
	r := newRecord("needs-linux-bad-caps")
	err := et.parseLine(r, "test/reload/fake.ci", "option=needs-linux:caps=net-admn")
	if err == nil {
		t.Fatalf("a misspelled caps value was accepted on %s, silently disabling the capability gate", runtime.GOOS)
	}
	if !strings.Contains(err.Error(), capsNetAdmin) {
		t.Fatalf("error %q does not name the supported value", err)
	}
}
