// Design: docs/architecture/testing/ci-format.md -- option=needs-linux capability gating
// Related: caps_linux.go -- the real probe; record_parse.go -- the needs-linux option

package runner

import (
	"slices"
	"strings"
)

// goosLinux is runtime.GOOS on Linux. Named so the several capability/netns
// guards in this package share one spelling instead of repeating the literal.
const goosLinux = "linux"

// Capability bit positions in the Linux capability bitmask
// (include/uapi/linux/capability.h). Declared here rather than in caps_linux.go
// so capsRequired below -- which every platform parses -- can name them: a `.ci`
// typo must be rejected on macOS too, or the author who wrote it gets no
// feedback until someone runs on Linux.
const (
	capNetAdmin    = 12 // CAP_NET_ADMIN: interfaces, netlink, nftables
	capNetRaw      = 13 // CAP_NET_RAW: raw and packet sockets (ICMP ping, traceroute)
	capSysResource = 24 // CAP_SYS_RESOURCE: raise rlimits (RLIMIT_MEMLOCK for eBPF)
	capBPF         = 39 // CAP_BPF: load eBPF programs and create maps (>= 5.8)
)

// Values `option=needs-linux:caps=` accepts, as a comma-separated list.
//
//   - net-admin: privileged network configuration -- creating interfaces,
//     bringing links up, programming netlink and nftables.
//
//   - bpf: loading eBPF programs and creating maps. CAP_BPF alone, deliberately
//     NOT CAP_SYS_RESOURCE. rlimit.RemoveMemlock
//     (vendor/github.com/cilium/ebpf/rlimit/rlimit_linux.go:109-127) only
//     reaches its prlimit(2) fallback -- the part that needs CAP_SYS_RESOURCE --
//     when memcg BPF accounting is unavailable, which is kernels older than
//     5.11. Every kernel this gate is ever evaluated on is far past that: the ze
//     appliance builds 7.1.4, the CI runner is 6.x, the QEMU Alpine VM likewise.
//     On all of them a process holding CAP_BPF makes RemoveMemlock return early
//     and CAP_SYS_RESOURCE is never consulted.
//
//     The CI log that motivated this token read "need CAP_BPF/CAP_SYS_RESOURCE",
//     which invites requiring both. That message is a CASCADE, not two
//     independent requirements: without CAP_BPF the memcg probe (BPF_MAP_CREATE)
//     fails, so the library falls back to prlimit, which is then also denied.
//     Requiring the second bit would make the gate over-strict, and an
//     over-strict gate SKIPS a test the host could actually run -- the coverage
//     deletion this whole mechanism exists to prevent.
//
//   - net-raw: opening a raw or packet socket. `resolve ping` / `show ping` and
//     traceroute build ICMP themselves through net.ListenPacket("ip4:icmp", ...)
//     (internal/component/ping/cmd/ping.go doPingCtx), which the kernel refuses
//     without CAP_NET_RAW. Unprivileged, that surfaces as a StatusError response
//     whose text already names the capability -- so the test does not hang, it
//     fails with a readable reason that is nonetheless about the HOST and not
//     about ze. CAP_NET_RAW alone: nothing on this path needs NET_ADMIN, and
//     requiring both would skip a host that can genuinely run the test.
const (
	capsNetAdmin = "net-admin"
	capsNetRaw   = "net-raw"
	capsBPF      = "bpf"
)

// capsRequired maps each accepted token to the capability bits a host must hold
// for it. Deriving the probe from this table rather than from a per-token `if`
// keeps the declared name and the tested bits in one place: the failure this
// prevents is a token whose NAME says one capability while the probe checks
// another, which is a guard that cannot evaluate what it claims to
// (ai/rules/evidence.md).
var capsRequired = map[string][]int{
	capsNetAdmin: {capNetAdmin},
	capsNetRaw:   {capNetRaw},
	capsBPF:      {capBPF},
}

// capsAccepted lists the accepted tokens for an error message, derived from the
// table so a new capability cannot be added without the diagnostic naming it
// (ai/rules/evidence.md).
func capsAccepted() string {
	names := make([]string, 0, len(capsRequired))
	for name := range capsRequired {
		names = append(names, name)
	}
	slices.Sort(names)
	return strings.Join(names, ", ")
}

// hasCaps reports whether this process holds every capability the named tokens
// require. It is a package var, not a direct call, so a test can drive BOTH
// polarities of the gate on any host: asserting only the host's real answer
// makes the test vacuous in exactly the environment where the suite runs for
// real (the QEMU VM, where the capabilities are present and the "skips without
// them" branch is never taken). Same seam as interfaceByName in
// internal/component/ike/engine/doctor.go.
//
// This is a deliberate, precedented exception to the "no global mutable state"
// rule in ai/rules/go-standards.md: it is a test seam, never written in
// production (only probeCaps's result is read), and it mirrors interfaceByName
// in internal/component/ike/engine/doctor.go and in
// internal/component/doctor/checks_listener.go.
var hasCaps = func(tokens []string) bool {
	n := 0
	for _, t := range tokens {
		n += len(capsRequired[t])
	}
	if n == 0 {
		return true
	}
	bits := make([]int, 0, n)
	for _, t := range tokens {
		bits = append(bits, capsRequired[t]...)
	}
	return probeCaps(bits...)
}

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
// (ai/rules/evidence.md). The test then runs against a kernel missing
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
//     explicitly NOT the fix.
//   - test/policy/005-next-hop's next-hop then has no connected route, so
//     RouteAdd returns "network is unreachable" and takes policy-routes down.
//
// Provisioning the links outside netns mode is NOT the alternative: they are
// named eth0/eth1/..., so creating them would touch the caller's REAL host
// namespace -- the one thing the per-test netns launch mode and its host-safety
// gate exist to guarantee never happens.
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
