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
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
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
	if !strings.Contains(r.SkipReason, "./le test qemu all") {
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

// The net-bind token maps to CAP_NET_BIND_SERVICE and gates in BOTH polarities.
//
// VALIDATES: `caps=net-bind` skips without the capability, runs with it, and
// asks the probe about the bit its NAME claims.
// PREVENTS: two failures with one shape. Declaring `caps=net-admin` on a test
// whose only privileged act is binding port 53 would skip a host that can bind
// it and cannot program nftables, deleting coverage; checking no bit at all
// would let the test run unprivileged, where `le mock dns` fails to bind and
// the daemon's lookups time out against nothing.
func TestCapsNetBindGateBothPolarities(t *testing.T) {
	bits := capsRequired[capsNetBind]
	if !slices.Contains(bits, capNetBindService) {
		t.Errorf("capsRequired[%q] = %v, missing CAP_NET_BIND_SERVICE (bit %d)", capsNetBind, bits, capNetBindService)
	}
	if slices.Contains(bits, capNetAdmin) {
		t.Errorf("capsRequired[%q] = %v also requires CAP_NET_ADMIN (bit %d): binding a port programs no netlink, "+
			"so the gate would skip a host that could run the test", capsNetBind, bits, capNetAdmin)
	}

	var seen []string
	withCapsRecording(t, false, &seen)

	absent := parseNeedsLinux(t, "option=needs-linux:caps=net-bind")
	if runtime.GOOS != goosLinux {
		return // the GOOS skip fires first, and is covered by the bare-option test
	}
	if absent.SkipReason == "" {
		t.Fatal("caps=net-bind ran without CAP_NET_BIND_SERVICE: the stub cannot bind port 53 and the test fails on the host, not on ze")
	}
	if !slices.Contains(seen, capsNetBind) {
		t.Errorf("the probe was never asked about %q (seen: %v)", capsNetBind, seen)
	}
	if !strings.Contains(absent.SkipReason, capsNetBind) {
		t.Errorf("skip reason %q does not name the missing capability", absent.SkipReason)
	}

	withCaps(t, true)
	present := parseNeedsLinux(t, "option=needs-linux:caps=net-bind")
	if present.SkipReason != "" {
		t.Fatalf("caps=net-bind skipped WITH the capability present: %q -- the DNS stub tests would never run, even under QEMU", present.SkipReason)
	}
}

// TestEveryCapsBitHasADockerName pins the Docker spelling of each caps= bit
// against a table written out here, independent of capDockerName.
//
// VALIDATES: capDockerName holds exactly the bits some caps= token requires,
// each under the kernel's CAP_ name with the prefix dropped, and
// ContainerCapsDockerNames answers exactly BPF, NET_ADMIN, NET_BIND_SERVICE
// and NET_RAW: every token's grant except sys-time, whose test sets the clock
// of the host the container shares.
// PREVENTS: a wrong spelling or a stray grant (SYS_RESOURCE, which no token
// requires) reaching the release-candidate evidence container
// (internal/le/verify/evidence), and SYS_TIME reaching it, which would let a
// verify run set the Docker host's clock.
func TestEveryCapsBitHasADockerName(t *testing.T) {
	spelled := map[int]string{
		10: "NET_BIND_SERVICE",
		12: "NET_ADMIN",
		13: "NET_RAW",
		25: "SYS_TIME",
		39: "BPF",
	}
	if len(capDockerName) != len(spelled) {
		t.Errorf("capDockerName = %v, want exactly %v", capDockerName, spelled)
	}
	for bit, want := range spelled {
		if got := capDockerName[bit]; got != want {
			t.Errorf("capDockerName[%d] = %q, want %q", bit, got, want)
		}
	}
	for token, bits := range capsRequired {
		for _, bit := range bits {
			if _, ok := capDockerName[bit]; !ok {
				t.Errorf("caps=%s requires bit %d, which has no Docker name", token, bit)
			}
		}
	}

	got, err := ContainerCapsDockerNames()
	if err != nil {
		t.Fatalf("ContainerCapsDockerNames: %v", err)
	}
	want := []string{"BPF", "NET_ADMIN", "NET_BIND_SERVICE", "NET_RAW"}
	if !slices.Equal(got, want) {
		t.Errorf("ContainerCapsDockerNames() = %v, want exactly %v", got, want)
	}
}

// TestNeedsLinuxRefusesUnknownKeys drives the key validation of
// option=needs-linux, and the removed opens-device option.
//
// VALIDATES: an unknown needs-linux key (the misspelled cpas=, a devices= key)
// and an empty caps= are each refused on every host with the accepted
// spelling; option=opens-device, whose per-test device declaration had no
// reader and was removed, is refused as an unknown option type.
// PREVENTS: a typo leaving a test's capability need undeclared or silently
// ignored, and a device declaration reappearing that decides nothing.
func TestNeedsLinuxRefusesUnknownKeys(t *testing.T) {
	withCaps(t, true)

	for line, want := range map[string]string{
		"option=needs-linux:device=ppp":                  "unknown key",
		"option=needs-linux:caps=net-admin:devices=ppp":  "unknown key",
		"option=needs-linux:caps=":                       "caps= is empty",
		"option=needs-linux:caps=net-admin:cpas=net-raw": "unknown key",
		"option=opens-device:name=ppp":                   `unknown option type "opens-device"`,
	} {
		et := &EncodingTests{}
		err := et.parseLine(newRecord("bad-key"), "test/l2tp/fake.ci", line)
		if err == nil {
			t.Errorf("%s was accepted", line)
			continue
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%s refused as %q, want it to say %q", line, err, want)
		}
	}
}

// TestContainerDevicesCoverWhatPPPOpens reads every non-test Go file under
// internal/component/l2tp, the subsystem the L2TP and PPPoE .ci tests run,
// collects each "/dev/..." literal passed to an open call, and checks that
// ContainerDevicePaths holds every one and nothing else. The opened set is
// derived from the source, not listed: ppp.DevPPPSetup opens /dev/ppp through
// openDevPPP (devppp_linux.go).
//
// VALIDATES: the release-candidate evidence container is granted every device
// node the L2TP and PPPoE paths open, and no node they do not; the walk found
// at least one open.
// PREVENTS: a new device open in that subsystem leaving the container without
// the node, which fails the session open and reads as a protocol failure, and
// a stale grant surviving the open's removal.
func TestContainerDevicesCoverWhatPPPOpens(t *testing.T) {
	root := filepath.Join("..", "..", "component", "l2tp")
	opened := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (selector.Sel.Name != "Open" && selector.Sel.Name != "OpenFile") {
				return true
			}
			literal, ok := call.Args[0].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(literal.Value)
			if err == nil && strings.HasPrefix(value, "/dev/") {
				opened[value] = true
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(opened) == 0 {
		t.Fatal("no open of a /dev node found under internal/component/l2tp; the walk read the wrong tree")
	}
	want := make([]string, 0, len(opened))
	for path := range opened {
		want = append(want, path)
	}
	slices.Sort(want)
	if got := ContainerDevicePaths(); !slices.Equal(got, want) {
		t.Errorf("ContainerDevicePaths() = %v, want exactly the nodes the L2TP and PPPoE paths open, %v", got, want)
	}
	for _, path := range want {
		if module, ok := DeviceHostModule(path); !ok || module == "" {
			t.Errorf("DeviceHostModule(%q) = %q, %v; every granted node names the module that creates it", path, module, ok)
		}
	}
}
