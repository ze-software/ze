// Design: docs/architecture/testing/interop.md -- RFC 2545 Section 3 for a third-party next hop.
// Related: check_rfc2545_third_party_predicate.go -- the pure predicates this body decides with.
// Related: register_rfc2545_third_party.go -- the registration of this checker.
// Related: test/interop/scenarios/bgp-rfc2545-third-party-linklocal-frr/ze.conf -- the topology.
package bgp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// rfc2545ThirdPartyScenario is the scenario checkRFC2545ThirdPartyLinkLocal
// judges.
const rfc2545ThirdPartyScenario = "bgp-rfc2545-third-party-linklocal-frr"

// checkRFC2545ThirdPartyLinkLocal judges RFC 2545 Section 3 for a next hop that
// names another router: the global next hop is frr-sink's (host 15) on the /64
// the speaker shares with FRR, and FRR must decode frr-sink's global then
// frr-sink's own link-local, read from the speaker's neighbor table, and install
// the route through that link-local.
//
// RFC requirement: RFC2545-3-3 positive -- the global next hop names another router (frr-sink, host 15) on the /64 the speaker shares with FRR, and FRR decodes that router's global then that router's own link-local, read from the speaker's neighbor table, and installs the route through that link-local.
// RFC requirement: RFC2545-3-6 positive -- a route Ze announces through another router's global on the common subnet reaches FRR with that router's link-local as its second next hop, not Ze's own.
func checkRFC2545ThirdPartyLinkLocal(ctx context.Context, check *interoplab.CheckContext) error {
	const (
		// The scenario literal, which the conformance recorder reads to pick
		// the scenario a break of this checker runs (rfc-conformance-gates.md).
		name   = "bgp-rfc2545-third-party-linklocal-frr"
		prefix = "2001:db8:5603::/48"
		device = "eth0"
	)
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, name, assertion, cause)
	}
	if !check.Network.IPv6.IsValid() {
		return fail(1, errors.New("RFC 2545 third-party scenario has no selected IPv6 network"))
	}

	// Assertion 1. FRR's IPv6 session with Ze is up.
	session := operation{kind: opFRRSession, argument: networkHostAddress6(check.Network, zeHostNumber)}
	if err := runOperation(ctx, check.Network, check.Lab, &session); err != nil {
		return fail(1, err)
	}

	// Assertion 2. The third-party router owns exactly one link-local, and its
	// MAC is known. Docker gives frr-sink host 15 on the lab /64.
	sinkGlobal := netip.MustParseAddr(networkHostAddress6(check.Network, extendedSinkHost))
	sinkLinkLocal, sinkMAC, err := thirdPartyIdentity(ctx, check.Lab, device)
	if err != nil {
		return fail(2, err)
	}

	// Assertion 3. Ze's kernel resolves both of frr-sink's addresses, and its
	// neighbor table binds them to one MAC: the state Ze's reader pairs.
	if _, err := untilDecoded(ctx, func() (struct{}, error) {
		return struct{}{}, resolveThirdParty(ctx, check.Lab, device, sinkGlobal, sinkLinkLocal, sinkMAC)
	}); err != nil {
		return fail(3, err)
	}

	// Assertion 4. Ze announces a route through frr-sink's global, and FRR
	// decodes frr-sink's global then frr-sink's link-local. The announce is
	// repeated each round: the neighbor-changed event that refreshes Ze's
	// table read races the first send, and a resend is an implicit replace.
	// A Ze that never adds the third party's link-local fails every round.
	var announce textbuf.Buffer
	command := zeCommand(announce.Str("send bgp * update text origin igp nhop ").Addr(sinkGlobal).
		Str(" mp ipv6/unicast add ").Str(prefix).String())
	if _, err := untilDecoded(ctx, func() (struct{}, error) {
		if _, err := check.Lab.Query(ctx, "ze", command, queryEnvironment("ze", command)); err != nil {
			return struct{}{}, err
		}
		select {
		case <-ctx.Done():
			return struct{}{}, ctx.Err()
		case <-time.After(time.Second):
		}
		hops, err := queryFRRNextHops(ctx, check.Lab, prefix)
		if err != nil {
			return struct{}{}, err
		}
		return struct{}{}, requireNextHopShape(hops, sinkGlobal, sinkLinkLocal)
	}); err != nil {
		return fail(4, fmt.Errorf("third-party route: %w", err))
	}

	// Assertion 5. FRR installs the route through frr-sink's link-local, so the
	// second address was used and not merely parsed.
	routes, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show ipv6 route bgp"}, nil)
	if err != nil {
		return fail(5, err)
	}
	if err := requireRouteInstalledVia(routes, prefix, sinkLinkLocal.String()); err != nil {
		return fail(5, err)
	}

	// Assertion 6. The session survived the UPDATE.
	if err := runOperation(ctx, check.Network, check.Lab, &session); err != nil {
		return fail(6, err)
	}
	return nil
}

// thirdPartyIdentity reads frr-sink's one link-local on device and its MAC.
func thirdPartyIdentity(ctx context.Context, lab interoplab.CheckerLab, device string) (netip.Addr, net.HardwareAddr, error) {
	addresses, err := lab.Query(ctx, peerFRRSink, []string{"cat", "/proc/net/if_inet6"}, nil)
	if err != nil {
		return netip.Addr{}, nil, err
	}
	linkLocal, err := onlyLinkLocal(addresses, device)
	if err != nil {
		return netip.Addr{}, nil, err
	}
	output, err := lab.Query(ctx, peerFRRSink, []string{"cat", "/sys/class/net/" + device + "/address"}, nil)
	if err != nil {
		return netip.Addr{}, nil, err
	}
	mac, err := parseContainerMAC(output)
	if err != nil {
		return netip.Addr{}, nil, err
	}
	return linkLocal, mac, nil
}

// resolveThirdParty pings frr-sink's global and link-local from Ze's container,
// which makes Ze's kernel resolve both by Neighbor Discovery, then requires
// Ze's neighbor table to bind both to frr-sink's MAC.
func resolveThirdParty(ctx context.Context, lab interoplab.CheckerLab, device string, global, linkLocal netip.Addr, mac net.HardwareAddr) error {
	if _, err := lab.Query(ctx, "ze", []string{"ping", "-c", "1", "-W", "2", global.String()}, nil); err != nil {
		return fmt.Errorf("ping %s from ze: %w", global, err)
	}
	if _, err := lab.Query(ctx, "ze", []string{"ping", "-c", "1", "-W", "2", "-I", device, linkLocal.String()}, nil); err != nil {
		return fmt.Errorf("ping %s from ze: %w", linkLocal, err)
	}
	table, err := lab.Query(ctx, "ze", []string{"ip", "-j", "-6", "neigh", "show", "dev", device}, nil)
	if err != nil {
		return err
	}
	return requireNeighborPair(table, global, linkLocal, mac)
}
