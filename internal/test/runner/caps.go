// Design: docs/architecture/testing/ci-format.md -- option=needs-linux capability gating
// Related: caps_linux.go -- the real probe; record_parse.go -- the needs-linux option

package runner

// goosLinux is runtime.GOOS on Linux. Named so the several capability/netns
// guards in this package share one spelling instead of repeating the literal.
const goosLinux = "linux"

// capsNetAdmin is the only value `option=needs-linux:caps=` accepts today. It
// declares that the test performs privileged network configuration (creating
// interfaces, bringing links up, programming netlink).
const capsNetAdmin = "net-admin"

// hasNetAdmin reports whether this process may perform privileged network
// configuration. It is a package var, not a direct call, so a test can drive
// BOTH polarities of the gate on any host: asserting only the host's real
// answer makes the test vacuous in exactly the environment where the suite runs
// for real (the QEMU VM, where the capability is present and the "skips without
// it" branch is never taken). Same seam as interfaceByName in
// internal/component/ike/engine/doctor.go.
//
// This is a deliberate, precedented exception to the "no global mutable state"
// rule in ai/rules/go-standards.md: it is a test seam, never written in
// production (only probeNetAdmin's result is read), and it mirrors
// interfaceByName in internal/component/ike/engine/doctor.go and in
// internal/component/doctor/checks_listener.go. Without it the capability
// test is vacuous on any host that HAS the capability -- which is the QEMU
// runner, the one environment where the gated tests actually execute.
var hasNetAdmin = probeNetAdmin

// skipReasonNetnsLink is the skip reason applied by applyNetnsLinkGate. It names
// both targets that DO provision the links, so the reader's next step is one
// command rather than a source dive.
const skipReasonNetnsLink = "option=netns-link (needs the per-test netns launch mode; run via make ze-netns-qemu-test, or make ze-netns-test on Linux)"

// applyNetnsLinkGate skips a record that declared `option=netns-link` when the
// per-test netns launch mode is off.
//
// The option declares a PREREQUISITE, so a runner that cannot satisfy it must
// SKIP, not run. provisionNetnsLinks is reachable only from the netns arm of
// runOrchestrated (runner_exec.go), so off netns mode the interface is never
// created and the option is silently inert -- a fail-open gate
// (ai/rules/fail-closed-guards.md). The test then runs against a kernel missing
// the interface it asked for and fails on a symptom far from the cause. Two
// shapes of that in the 2026-07-25 `make ze-qemu-needs-linux-test` run, which
// sets no ZE_TEST_NETNS:
//
//   - all 8 needs-linux test/ospf and all 3 test/ospfv3 tests provision their
//     OSPF interface here (eth0/eth1/nbma0/ptmp0). Without it
//     resolveOSPFInterface (internal/plugins/ospf/transport/backend_linux.go:41)
//     fails "Link not found", the ospf engine exits 1, plugin startup never
//     completes, and each test dies on its observer's unrelated-looking TLS
//     connect timeout. Teaching OSPF to tolerate a missing active link is
//     explicitly NOT the fix (plan/learned/1264).
//   - test/policy/005-next-hop's next-hop then has no connected route, so
//     RouteAdd returns "network is unreachable" and takes policy-routes down.
//
// Provisioning the links outside netns mode is NOT the alternative: they are
// named eth0/eth1/..., so creating them would touch the caller's REAL host
// namespace -- the one thing the per-test netns launch mode (Fix B and its R-2
// host-safety gate, plan/learned/1112) exists to guarantee never happens.
//
// It replaces a weaker reason on purpose: netns mode implies Linux plus the
// runner's CAP_SYS_ADMIN, so this requirement subsumes
// needs-linux[:caps=net-admin], and the targets it names are the ones that
// actually run these tests (`make ze-qemu-needs-linux-test` never will).
func applyNetnsLinkGate(r *Record) {
	if len(r.NetnsLinks) > 0 && !netnsActive() {
		r.SkipReason = skipReasonNetnsLink
	}
}
