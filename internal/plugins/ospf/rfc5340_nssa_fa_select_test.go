// Design: docs/architecture/ospf/ospfv3-5-nssa-redist.md -- OSPFv3 NSSA forwarding address.
// Related: origination_v6_nssa.go -- externalScopeV6For and forwardingAddressForAF, the selector.
// Related: interface_addr.go -- v6UsableForwardingAddress, the global-address filter.
//
// VALIDATES: RFC 5340 Appendix A.4.8, "A global IPv6 address MUST be selected as forwarding
// address for NSSA-LSAs that are to be propagated by NSSA area border routers", at the
// SELECTION: the per-NSSA attachment externalScopeV6For builds (whose address the P-bit
// NSSA-LSA carries) takes an interface's global address and never its link-local one.
// PREVENTS: a selector that hands a link-local, loopback or unspecified address to a
// propagated NSSA-LSA, which no router outside the link can forward to.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// TestRFC5340NSSAForwardingAddressSelectsGlobal walks the OSPFv3 NSSA scope over interfaces
// whose address seam answers a link-local, a loopback, the unspecified or a global address.
func TestRFC5340NSSAForwardingAddressSelectsGlobal(t *testing.T) {
	e := newEngineWithCodecAF(nil, v6Codec{}, afIPv6Unicast)
	t.Cleanup(e.shutdown)
	nssa := types.AreaID{0, 0, 0, 9}
	cfg := ospfConfig{Areas: []areaConfig{{AreaID: nssa, AreaType: types.AreaTypeNSSA}}}
	addrs := map[string]netip.Addr{
		"eth0": netip.MustParseAddr("fe80::1"),
		"eth1": netip.MustParseAddr("::1"),
		"eth2": netip.IPv6Unspecified(),
		"eth3": netip.MustParseAddr("2001:db8:9::1"),
	}
	e.forwardingAddress = func(name string) (netip.Addr, bool) {
		addr, ok := addrs[name]
		return addr, ok
	}
	scope := func(names ...string) nssaAttachmentV6 {
		t.Helper()
		running := make([]interfaceConfig, 0, len(names))
		for _, name := range names {
			running = append(running, interfaceConfig{Name: name, AreaID: nssa, Enabled: true})
		}
		nssas, _ := e.externalScopeV6For(cfg, running, nil)
		if len(nssas) != 1 || nssas[0].area != nssa {
			t.Fatalf("NSSA scope over %v = %+v, want one attachment to %s", names, nssas, nssa)
		}
		return nssas[0]
	}

	// RFC requirement: RFC5340-A.4.8-1 negative -- a non-global address is never selected: an
	// NSSA reached only over interfaces whose address is link-local, loopback or unspecified
	// gets no forwarding address at all (so no propagated NSSA-LSA is originated).
	for _, name := range []string{"eth0", "eth1", "eth2"} {
		if got := scope(name); got.hasFA {
			t.Fatalf("%s (%s) was selected as the NSSA forwarding address %v", name, addrs[name], netip.AddrFrom16(got.fa))
		}
	}
	if got := scope("eth0", "eth1", "eth2"); got.hasFA {
		t.Fatalf("a non-global address was selected as the NSSA forwarding address %v", netip.AddrFrom16(got.fa))
	}

	// RFC requirement: RFC5340-A.4.8-1 positive -- the global address is selected: with a
	// link-local interface first in name order and a global one after it, the NSSA's
	// forwarding address is the global 2001:db8:9::1.
	got := scope("eth0", "eth3")
	if !got.hasFA {
		t.Fatal("no forwarding address was selected although eth3 holds a global address")
	}
	if netip.AddrFrom16(got.fa) != addrs["eth3"] {
		t.Fatalf("NSSA forwarding address = %v, want the global %v", netip.AddrFrom16(got.fa), addrs["eth3"])
	}
}
