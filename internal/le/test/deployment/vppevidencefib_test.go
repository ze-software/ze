package testdeployment

import (
	"errors"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

// vppFIBECMPOutput is `show ip fib 10.28.0.0/24` in the shape VPP's
// format_fib_entry writes for a two-path recursive ECMP route installed under
// FIB source ze-fib-vpp.
const vppFIBECMPOutput = `ipv4-VRF:0, fib_index:0, flow hash:[src dst sport dport proto flowlabel ] epoch:0 flags:none locks:[adjacency:1, default-route:1, ]
10.28.0.0/24 fib:0 index:24 locks:2
  ze-fib-vpp refs:1 src-flags:added,contributing,active,
    path-list:[32] locks:2 flags:shared, uPRF-list:27 len:2 itfs:[2, 3, ]
      path:[40] pl-index:32 ip4 weight=1 pref=0 recursive:  oper-flags:resolved,
        via 10.28.99.1 in fib:0 via-fib:0 via-dpo:[dpo-load-balance:20]
      path:[41] pl-index:32 ip4 weight=1 pref=0 recursive:  oper-flags:resolved,
        via 10.28.98.1 in fib:0 via-fib:0 via-dpo:[dpo-load-balance:19]
 forwarding:   unicast-ip4-chain
  [@0]: dpo-load-balance: [proto:ip4 index:26 buckets:2 uRPF:27 to:[0:0]]
    [0] [@5]: ipv4 via 10.28.98.1 loop1: mtu:9000 next:3 flags:[]
    [1] [@5]: ipv4 via 10.28.99.1 loop2: mtu:9000 next:3 flags:[]
`

// vppFIBSinglePathOutput is `show ip fib 10.28.0.0/24` after withdrawing
// 10.28.99.1, leaving only 10.28.98.1 under ze-fib-vpp.
const vppFIBSinglePathOutput = `ipv4-VRF:0, fib_index:0, flow hash:[src dst sport dport proto flowlabel ] epoch:0 flags:none locks:[adjacency:1, default-route:1, ]
10.28.0.0/24 fib:0 index:24 locks:2
  ze-fib-vpp refs:1 src-flags:added,contributing,active,
    path-list:[33] locks:2 flags:shared, uPRF-list:28 len:1 itfs:[2, ]
      path:[42] pl-index:33 ip4 weight=1 pref=0 recursive:  oper-flags:resolved,
        via 10.28.98.1 in fib:0 via-fib:0 via-dpo:[dpo-load-balance:19]
 forwarding:   unicast-ip4-chain
  [@0]: dpo-load-balance: [proto:ip4 index:26 buckets:1 uRPF:28 to:[0:0]]
    [0] [@5]: ipv4 via 10.28.98.1 loop1: mtu:9000 next:3 flags:[]
`

// vppFIBDefaultFallbackOutput is what `show ip fib 10.29.0.0/24` returns after
// 10.29.0.0/24 has been removed from VPP: an LPM lookup match on 0.0.0.0/0.
const vppFIBDefaultFallbackOutput = `ipv4-VRF:0, fib_index:0, flow hash:[src dst sport dport proto flowlabel ] epoch:0 flags:none locks:[adjacency:1, default-route:1, ]
0.0.0.0/0 fib:0 index:0 locks:2
  default-route refs:1 entry-flags:drop, src-flags:added,contributing,active,
    path-list:[0] locks:2 flags:drop, uPRF-list:0 len:0 itfs:[]
      path:[0] pl-index:0 ip4 weight=1 pref=0 special:  cfg-flags:drop,
        [@0]: dpo-drop ip4
 forwarding:   unicast-ip4-chain
  [@0]: dpo-load-balance: [proto:ip4 index:0 buckets:1 uRPF:0 to:[0:0]]
    [0] [@0]: dpo-drop ip4
`

// TestVPPFIBSourceNextHopsParsesECMPAndWithdrawal validates the `show ip fib`
// parser across two-path ECMP, single-path survival, CRLF endings, foreign
// sources, wrong prefixes and forwarding-section isolation.
//
// VALIDATES: only exact-prefix paths under the named FIB source count, both
// ECMP next-hops are extracted and sorted, withdrawal leaves one surviving
// path, and LPM fallback to 0.0.0.0/0 reports the prefix absent.
// PREVENTS: a foreign source (API/CLI), a different prefix, a single-path
// route, or tokens in the forwarding DPO section passing an ECMP or sweep
// check.
func TestVPPFIBSourceNextHopsParsesECMPAndWithdrawal(t *testing.T) {
	both := []string{VPPECMPNextHopA, VPPECMPNextHopB}
	if !vppFIBHasSourceNextHops(vppFIBECMPOutput, VPPECMPPrefix, both) {
		t.Fatalf("vppFIBHasSourceNextHops missed two-path ECMP in:\n%s", vppFIBECMPOutput)
	}
	// vppctl ends lines with CRLF; the parser must handle both LF and CRLF.
	crlf := strings.ReplaceAll(vppFIBECMPOutput, "\n", "\r\n")
	if !vppFIBHasSourceNextHops(crlf, VPPECMPPrefix, []string{VPPECMPNextHopB, VPPECMPNextHopA}) {
		t.Fatal("vppFIBHasSourceNextHops failed on CRLF output or unsorted expected slice")
	}

	if vppFIBHasSourceNextHops(vppFIBSinglePathOutput, VPPECMPPrefix, both) {
		t.Fatal("single-path output satisfied a two-path ECMP requirement")
	}
	if !vppFIBHasSourceNextHops(vppFIBSinglePathOutput, VPPECMPPrefix, []string{VPPECMPNextHopA}) {
		t.Fatalf("single surviving path %s not matched in:\n%s", VPPECMPNextHopA, vppFIBSinglePathOutput)
	}

	if vppFIBHasSource(vppFIBDefaultFallbackOutput, VPPStaleSweepPrefix) {
		t.Fatal("default-route LPM fallback reported stale prefix present")
	}

	foreignSource := strings.Replace(vppFIBECMPOutput, "  ze-fib-vpp refs:1", "  API refs:1", 1)
	if vppFIBHasSource(foreignSource, VPPECMPPrefix) {
		t.Fatal("API source counted as ze-fib-vpp")
	}
	if apiHops, found := vppFIBSourceNextHops(foreignSource, VPPECMPPrefix, "API"); !found || !slices.Equal(apiHops, both) {
		t.Fatalf("vppFIBSourceNextHops(API) = %v, %v; want %v, true", apiHops, found, both)
	}

	wrongPrefix := strings.Replace(vppFIBECMPOutput, "10.28.0.0/24 fib:0", "10.28.1.0/24 fib:0", 1)
	if vppFIBHasSource(wrongPrefix, VPPECMPPrefix) {
		t.Fatal("another prefix counted as 10.28.0.0/24")
	}

	// Next-hops appearing only under `forwarding:` must not satisfy ze-fib-vpp.
	sourceWithoutPaths := "10.28.0.0/24 fib:0 index:24 locks:2\n" +
		"  ze-fib-vpp refs:1 src-flags:added,contributing,active,\n" +
		" forwarding:   unicast-ip4-chain\n" +
		"      path:[40] pl-index:32 ip4 weight=1 pref=0 recursive:\n" +
		"        via 10.28.98.1 in fib:0\n"
	got, found := vppFIBSourceNextHops(sourceWithoutPaths, VPPECMPPrefix, VPPFIBSourceName)
	if !found {
		t.Fatal("ze-fib-vpp source header not recognized")
	}
	if len(got) != 0 {
		t.Fatalf("forwarding section leaked next-hops %v into ze-fib-vpp", got)
	}

	// An attached-nexthop detail line without leading "via" is parsed too.
	attached := "10.29.0.0/24 fib:0 index:9 locks:1\n" +
		"  ze-fib-vpp refs:1\n" +
		"    path-list:[4] locks:1\n" +
		"      path:[5] pl-index:4 ip4 weight=1 pref=0 attached-nexthop:\n" +
		"        10.29.99.1 loop1\n" +
		" forwarding: unicast-ip4-chain\n"
	if !vppFIBHasSourceNextHops(attached, VPPStaleSweepPrefix, []string{VPPStaleSweepNextHop}) {
		t.Fatal("attached-nexthop format not parsed under ze-fib-vpp")
	}
}

// TestVPPStaticFIBConfigRendersRoutesAndOmitsEmptyStatic validates the config
// generator shared by fib-ecmp and fib-stale-sweep.
//
// VALIDATES: fib { vpp { enabled true; } } is always emitted, multiple hops
// render under table default next { ... }, and an empty route slice omits the
// static block for reload withdrawal.
// PREVENTS: malformed static config or leftover static blocks breaking reload
// cleanup in real VPP runs.
func TestVPPStaticFIBConfigRendersRoutesAndOmitsEmptyStatic(t *testing.T) {
	ecmp := vppStaticFIBConfig([]vppStaticRoute{
		{prefix: VPPECMPPrefix, nextHops: []string{VPPECMPNextHopA, VPPECMPNextHopB}},
	})
	for _, needle := range []string{
		"fib {\n    vpp { enabled true; }\n}",
		"static {\n    table default {",
		"route 10.28.0.0/24 {",
		"hop 10.28.98.1 { }",
		"hop 10.28.99.1 { }",
	} {
		if !strings.Contains(ecmp, needle) {
			t.Fatalf("ECMP config missing %q:\n%s", needle, ecmp)
		}
	}

	empty := vppStaticFIBConfig(nil)
	if strings.Contains(empty, "static {") {
		t.Fatalf("empty route config kept a static block:\n%s", empty)
	}
	if !strings.Contains(empty, "fib {\n    vpp { enabled true; }\n}") {
		t.Fatalf("empty route config dropped fib-vpp:\n%s", empty)
	}
}

// TestKillAwaitingExitSendsSIGKILLAndRecordsSurvivors validates abrupt daemon
// termination used by fib-stale-sweep.
//
// VALIDATES: killAwaitingExit sends SIGKILL directly (never SIGTERM), returns
// nil once the process exits, and records a stop failure if the process
// survives SIGKILL.
// PREVENTS: fib-stale-sweep triggering graceful static shutdown via SIGTERM or
// silently ignoring a stuck process after SIGKILL.
func TestKillAwaitingExitSendsSIGKILLAndRecordsSurvivors(t *testing.T) {
	proc, received := fakeDaemon(t, syscall.SIGKILL, 20*time.Millisecond)
	if err := proc.killAwaitingExit(time.Second); err != nil {
		t.Fatalf("killAwaitingExit of a process that exited on SIGKILL: %v", err)
	}
	if !proc.exited() {
		t.Fatal("killAwaitingExit answered before the process exited")
	}
	if !slices.Equal(*received, []syscall.Signal{syscall.SIGKILL}) {
		t.Fatalf("signals sent = %v, want [SIGKILL]", *received)
	}

	v := &VPP{vppIface: &vppIface{}}
	stuck, _ := fakeDaemon(t, 0, 0)
	if err := stuck.killAwaitingExit(50 * time.Millisecond); !errors.Is(err, errProcessSurvived) {
		t.Fatalf("killAwaitingExit of survivor = %v, want errProcessSurvived", err)
	}
	v.recordStopFailure(errProcessSurvived)
	if !errors.Is(v.checkDaemonsStopped(), errProcessSurvived) {
		t.Fatalf("checkDaemonsStopped = %v, want errProcessSurvived", v.checkDaemonsStopped())
	}
}
