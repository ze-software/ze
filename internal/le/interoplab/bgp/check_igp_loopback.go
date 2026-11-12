// Design: docs/architecture/testing/interop.md -- reciprocal IGP loopback routes.
// Related: register_igp_loopback.go -- the checker registrations.
// Related: check_rfc_predicate.go -- requireRouteInstalledVia judges FRR's FIB.
// FRR: https://github.com/FRRouting/frr/tree/frr-10.7.1 -- isisd/isis_pdu.c
// process_p2p_hello and put_hello_hdr decide the offered and negotiated levels;
// isisd/isis_adjacency.c isis_adj_print_vty renders the exact L2 adjacency.
// zebra/zebra_vty.c vty_show_ip_route and lib/nexthop.c nexthop_json_helper
// produce the installed-route and matching-next-hop evidence judged below.
// ospfd/ospf_lsa.c lsa_link_loopback_set originates FRR's loopback host route.
// iproute2: https://git.kernel.org/pub/scm/network/iproute2/iproute2.git/tree/ip/iproute.c?h=v6.1.0
// filter_nlmsg enforces the protocol filter and print_route omits its redundant
// field. The recorded lab reply verifies that printer behavior; this source
// reference makes no claim about the installed iproute2 version.
package bgp

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	isisDefaultLevelScenario = "isis-p2p-level2-default-frr"
	ospfLoopbackScenario     = "ospf-loopback-prefixes-frr"
)

func checkISISDefaultLevelLoopbacks(ctx context.Context, check *interoplab.CheckContext) error {
	prerequisites := []operation{
		{kind: opWaitContains, peer: "ze", command: []string{"ip", "-o", "-4", "address", "show", "dev", "lo"}, contains: []string{"127.0.0.1/8", "10.80.0.2/32"}, timeout: 30 * time.Second},
		// FRR 10.7.1 isis_adj_print_vty emits these fields together for one adjacency.
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show isis neighbor detail"}, contains: []string{"Interface: eth0, Level: 2, State: Up"}, timeout: 90 * time.Second},
	}
	for index := range prerequisites {
		if err := runOperation(ctx, check.Network, check.Lab, &prerequisites[index]); err != nil {
			return checkerFailure(ctx, check.Lab, isisDefaultLevelScenario, index+1, err)
		}
	}
	return checkIGPLoopbackRoutes(ctx, check, "isis", []netip.Prefix{
		netip.MustParsePrefix("10.80.0.2/32"),
	}, netip.MustParsePrefix("10.80.0.3/32"))
}

func checkOSPFLoopbackPrefixes(ctx context.Context, check *interoplab.CheckContext) error {
	prerequisites := []operation{
		{kind: opWaitContains, peer: "ze", command: []string{"ip", "-o", "-4", "address", "show", "dev", "lo"}, contains: []string{"127.0.0.1/8", "10.81.0.2/32", "10.81.1.2/32"}, timeout: 30 * time.Second},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	}
	for index := range prerequisites {
		if err := runOperation(ctx, check.Network, check.Lab, &prerequisites[index]); err != nil {
			return checkerFailure(ctx, check.Lab, ospfLoopbackScenario, index+1, err)
		}
	}
	return checkIGPLoopbackRoutes(ctx, check, "ospf", []netip.Prefix{
		netip.MustParsePrefix("10.81.0.2/32"),
		netip.MustParsePrefix("10.81.1.2/32"),
	}, netip.MustParsePrefix("10.81.0.3/32"))
}

// checkIGPLoopbackRoutes requires FRR's selected installed routes through Ze,
// Ze's exact kernel route through FRR, then a ping sourced from each advertised
// loopback in each direction. Docker's connected/default routes cannot satisfy
// either route assertion, and the source addresses require a working return path.
// Both protocol registrations declare NeedsDataPlane, so their SPF routes reach
// the kernel through the selected FIB writer rather than a checker-installed route.
func checkIGPLoopbackRoutes(ctx context.Context, check *interoplab.CheckContext, protocol string, zePrefixes []netip.Prefix, frrPrefix netip.Prefix) error {
	zeHop := networkHostAddress(check.Network, zeHostNumber)
	frrHop := networkHostAddress(check.Network, frrHostNumber)
	for _, prefix := range zePrefixes {
		if err := waitFRRVerdict(ctx, check.Lab, "show ip route "+prefix.String()+" json", "FRR's installed "+protocol+" route for "+prefix.String(), func(output string) error {
			return requireRouteInstalledVia(output, prefix.String(), zeHop, protocol)
		}); err != nil {
			return fmt.Errorf("FRR loopback route: %w", err)
		}
	}
	// -N keeps Ze's FIB protocol number independent of /etc/iproute2/rt_protos.
	// The exact-prefix selector excludes Docker's default and connected routes.
	// iproute2 filter_nlmsg enforces proto 250; print_route suppresses the
	// redundant protocol field when that filter is explicit.
	route := operation{
		kind: opWaitContains, peer: "ze",
		command:  []string{"ip", "-N", "-4", "route", "show", "exact", frrPrefix.String(), "proto", "250"},
		contains: []string{frrPrefix.Addr().String(), "via " + frrHop + " dev eth0"},
		timeout:  90 * time.Second,
	}
	if err := runOperation(ctx, check.Network, check.Lab, &route); err != nil {
		return fmt.Errorf("Ze reciprocal kernel route: %w", err)
	}
	for _, prefix := range zePrefixes {
		toFRR := operation{kind: opExec, peer: "ze", command: []string{"ping", "-n", "-c", "1", "-W", "2", "-I", prefix.Addr().String(), frrPrefix.Addr().String()}}
		if err := runOperation(ctx, check.Network, check.Lab, &toFRR); err != nil {
			return fmt.Errorf("Ze loopback %s to FRR %s: %w", prefix, frrPrefix, err)
		}
		toZe := operation{kind: opExec, peer: peerFRR, command: []string{"ping", "-n", "-c", "1", "-W", "2", "-I", frrPrefix.Addr().String(), prefix.Addr().String()}}
		if err := runOperation(ctx, check.Network, check.Lab, &toZe); err != nil {
			return fmt.Errorf("FRR loopback %s to Ze %s: %w", frrPrefix, prefix, err)
		}
	}
	return nil
}
