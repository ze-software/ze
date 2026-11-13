// Design: docs/architecture/testing/interop.md -- OSPF redistribution to eBGP.
// Related: register_ospf_redistribute_default.go -- checker registration.
// FRR: https://github.com/FRRouting/frr/tree/frr-10.7.1 --
// ospfd/ospf_lsa.c:lsa_link_ptop_set originates the loopback's configured subnet;
// bgpd/bgp_route.c:bgp_update rejects a host prefix matching a local address,
// so this scenario uses the non-host subnet instead of FRR's own /32.
// bgpd/bgp_route.c:bgp_update_martian_nexthop and bgp_update reject a next hop
// that bgpd/bgp_nexthop.c:bgp_nexthop_self finds in the local address hash.
// bgpd/bgp_route.c:route_vty_out_detail produces the accepted path's JSON.
package bgp

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	ospfRedistributeDefaultScenario = "bgp-ospf-redistribute-default-frr"
	ospfRedistributeDefaultPrefix   = "10.82.0.0/24"
	ospfRedistributeDefaultLoopback = "10.82.0.3/24"
)

// checkOSPFRedistributeDefault follows a real OSPF route through the
// redistribution consumer to a real external BGP receiver. The original
// gateway is FRR itself; preserving it as an explicit BGP next hop meets Ze's
// final send guard against peer-owned next hops. The accepted path must use
// Ze's endpoint. No session next-hop override masks the default.
func checkOSPFRedistributeDefault(ctx context.Context, check *interoplab.CheckContext) error {
	zeHop := networkHostAddress(check.Network, zeHostNumber)
	frrHop := networkHostAddress(check.Network, frrHostNumber)
	fail := func(assertion int, cause error) error {
		cause = fmt.Errorf("%w%s", cause, ospfRedistributeDefaultDiagnostics(ctx, check.Lab, zeHop, frrHop))
		return checkerFailure(ctx, check.Lab, ospfRedistributeDefaultScenario, assertion, cause)
	}
	prerequisites := []operation{
		{kind: opWaitContains, peer: peerFRR, command: []string{"ip", "-o", "-4", "address", "show", "dev", "lo"}, contains: []string{ospfRedistributeDefaultLoopback}, timeout: 30 * time.Second},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show ip ospf interface lo"}, contains: []string{"Internet Address " + ospfRedistributeDefaultLoopback, "Network Type POINTOPOINT"}, timeout: 30 * time.Second},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
		// proto 250 and the exact prefix exclude Docker's connected/default
		// routes. FRR has no BGP network or redistribute statement that could
		// originate this prefix: its source is the OSPF point-to-point stub link.
		{kind: opWaitContains, peer: "ze", command: []string{"ip", "-N", "-4", "route", "show", "exact", ospfRedistributeDefaultPrefix, "proto", "250"}, contains: []string{ospfRedistributeDefaultPrefix, "via " + frrHop + " dev eth0"}, timeout: 90 * time.Second},
		{kind: opFRRSession, argument: zeHop, timeout: 90 * time.Second},
	}
	for index := range prerequisites {
		if err := runOperation(ctx, check.Network, check.Lab, &prerequisites[index]); err != nil {
			return fail(index+1, err)
		}
	}
	// FRR owns this connected subnet, so zebra need not install the BGP
	// alternative. Its BGP RIB must nevertheless accept a valid best path
	// received from Ze with the external default's exact session-local hop.
	if err := waitFRRVerdict(ctx, check.Lab, "show bgp ipv4 unicast "+ospfRedistributeDefaultPrefix+" json", "FRR accepted OSPF redistribution with the external default next hop", func(output string) error {
		return requireRedistributedExternalPath(output, netip.MustParsePrefix(ospfRedistributeDefaultPrefix), netip.MustParseAddr(zeHop))
	}); err != nil {
		return fail(len(prerequisites)+1, err)
	}
	session := operation{kind: opFRRSession, argument: zeHop, timeout: 30 * time.Second}
	if err := runOperation(ctx, check.Network, check.Lab, &session); err != nil {
		return fail(len(prerequisites)+2, fmt.Errorf("BGP session after accepting redistributed route: %w", err))
	}
	return nil
}
