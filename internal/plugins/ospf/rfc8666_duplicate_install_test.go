// Design: docs/architecture/wire/ospf.md -- OSPF Segment Routing reception->install.
// Related: sr_install.go -- installRoutes, which skips a prefix marked Duplicate.
// Related: sr_prefix_sid_per_router_test.go -- the detector half of the same requirement.
//
// VALIDATES: RFC 8666 Section 6, "If an OSPFv3 router advertises multiple Prefix-SIDs for the
// same prefix, topology, and algorithm, all of them MUST be ignored", to its consequence: the
// repeated Prefix-SIDs of one router, detected from real LSAs, reach no mpls-fib entry, while
// the same router's single Prefix-SID for that prefix does.
// PREVENTS: an installer that ignores the Duplicate verdict and programs one of the
// conflicting bindings into the forwarding plane.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// TestRFC8666OneRouterRepeatedPrefixSIDInstallsNothing installs, on one engine, an OSPFv3
// E-Intra-Area-Prefix LSA from 5.5.5.5 carrying one Prefix-SID for 2001:db8::5/128, and on
// another the same LSA plus a second one from 5.5.5.5 repeating that Prefix-SID; each result
// feeds the installer with the same route, SRGBs and algorithms.
func TestRFC8666OneRouterRepeatedPrefixSIDInstallsNothing(t *testing.T) {
	r5 := types.RouterID{5, 5, 5, 5}
	loop := netip.MustParsePrefix("2001:db8::5/128")
	five := []sr.PrefixSIDConfig{{Prefix: loop, Index: 5}}
	install := func(eng *engine) int {
		t.Helper()
		sids := eng.srRemotePrefixSIDsV6()
		bus := &srCaptureBus{}
		inst := newTestInstallerV6(bus)
		routes := []srRoute{{Prefix: loop, Origin: r5,
			NextHops: []srNextHop{{Addr: netip.MustParseAddr("fe80::5"), Router: r5}}}}
		caps := map[types.RouterID]sr.SRGB{r5: sr.NewSRGB([]sr.LabelRange{{Base: 16000, Size: 200}})}
		algos := map[types.RouterID][]uint8{r5: {0}}
		inst.installRoutes(routes, sids, caps, algos, sr.NewSRGB([]sr.LabelRange{{Base: 18000, Size: 200}}))
		return len(bus.entries)
	}

	// RFC requirement: RFC8666-6-7 negative -- a router that advertises ONE Prefix-SID for the
	// prefix is not ignored: its binding reaches the forwarding plane (so the installer below
	// is able to install this prefix, and only the repetition stops it).
	single := newV6RIEngine(t)
	installRemoteV6EPrefix(t, single, types.BackboneArea, r5, 1, five)
	if n := install(single); n == 0 {
		t.Fatal("a router's single Prefix-SID for the prefix installed no forwarding entry")
	}

	// RFC requirement: RFC8666-6-7 positive -- when the same router advertises the Prefix-SID
	// for that prefix and algorithm twice (two LSAs), all of them are ignored: no forwarding
	// entry is installed for the prefix.
	repeated := newV6RIEngine(t)
	installRemoteV6EPrefix(t, repeated, types.BackboneArea, r5, 1, five)
	installRemoteV6EPrefix(t, repeated, types.BackboneArea, r5, 2, five)
	if n := install(repeated); n != 0 {
		t.Fatalf("one router's repeated Prefix-SID installed %d forwarding entries, want none", n)
	}
}
