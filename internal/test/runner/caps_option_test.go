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
	"slices"
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

// withCaps forces the capability probe's answer for one test.
func withCaps(t *testing.T, present bool) {
	t.Helper()
	original := hasCaps
	hasCaps = func(_ []string) bool { return present }
	t.Cleanup(func() { hasCaps = original })
}

// withCapsRecording forces the answer AND records the tokens the parser asked
// about, so a test can assert the gate checks what the .ci file declared rather
// than something else.
func withCapsRecording(t *testing.T, present bool, seen *[]string) {
	t.Helper()
	original := hasCaps
	hasCaps = func(tokens []string) bool { *seen = append(*seen, tokens...); return present }
	t.Cleanup(func() { hasCaps = original })
}

// A bare needs-linux must NOT consult capabilities: on Linux it runs, whatever
// privileges the process holds. Skipping it would delete coverage.
func TestNeedsLinuxWithoutCapsIgnoresCapabilities(t *testing.T) {
	withCaps(t, false)

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
	withCaps(t, false)

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
	withCaps(t, true)

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
	if !strings.Contains(err.Error(), capsNetAdmin) || !strings.Contains(err.Error(), capsBPF) {
		t.Fatalf("error %q does not name every supported value (%s)", err, capsAccepted())
	}
}

// A multi-capability declaration must gate on EVERY token, and must ask the
// probe about the ones the file declared.
//
// VALIDATES: `caps=net-admin,bpf` parses as two tokens and reaches the probe.
// PREVENTS: the fail-open shape this list exists to close. The three ddos tests
// need CAP_BPF + CAP_SYS_RESOURCE (the ebpf rlimit fallback,
// vendor/github.com/cilium/ebpf/rlimit/rlimit_linux.go) as well as CAP_NET_ADMIN;
// declaring only net-admin let a host holding just that one PASS a gate it
// cannot satisfy, so the test failed on the eBPF memlock rlimit exactly as it
// did with no marker at all.
func TestNeedsLinuxCapsListGatesOnEveryToken(t *testing.T) {
	var seen []string
	withCapsRecording(t, false, &seen)

	r := parseNeedsLinux(t, "option=needs-linux:caps=net-admin,bpf")
	if runtime.GOOS != goosLinux {
		return // the GOOS skip fires first
	}
	if r.SkipReason == "" {
		t.Fatal("caps=net-admin,bpf ran with the capabilities absent")
	}
	for _, want := range []string{capsNetAdmin, capsBPF} {
		if !slices.Contains(seen, want) {
			t.Errorf("the probe was never asked about %q; declared capabilities are not all checked (seen: %v)", want, seen)
		}
	}
	if !strings.Contains(r.SkipReason, capsBPF) {
		t.Errorf("skip reason %q does not name the bpf requirement, so the operator cannot tell which capability is missing", r.SkipReason)
	}
}

// The bpf token maps to CAP_BPF and NOTHING ELSE.
//
// VALIDATES: capsRequired[bpf] is exactly {CAP_BPF}.
// PREVENTS: re-adding CAP_SYS_RESOURCE. The CI message that motivated this
// token read "need CAP_BPF/CAP_SYS_RESOURCE", which reads like two
// requirements and is one cascade: without CAP_BPF the memcg probe
// (BPF_MAP_CREATE) fails, so rlimit.RemoveMemlock falls back to prlimit(2),
// which is then also denied. That fallback exists for kernels older than 5.11
// (vendor/github.com/cilium/ebpf/rlimit/rlimit_linux.go:108), and every kernel
// this gate runs on is far past it -- the ze appliance builds 7.1.4, CI is 6.x.
// Requiring the second bit makes the gate over-strict, and an over-strict gate
// SKIPS a test the host could have run, which is the coverage deletion the
// whole mechanism exists to prevent.
func TestBPFTokenIsCapBPFOnly(t *testing.T) {
	bits := capsRequired[capsBPF]
	if !slices.Contains(bits, capBPF) {
		t.Errorf("capsRequired[%q] = %v, missing CAP_BPF (bit %d)", capsBPF, bits, capBPF)
	}
	if slices.Contains(bits, capSysResource) {
		t.Errorf("capsRequired[%q] = %v still requires CAP_SYS_RESOURCE (bit %d): "+
			"no kernel this runs on needs it, so the gate would over-skip", capsBPF, bits, capSysResource)
	}
}
