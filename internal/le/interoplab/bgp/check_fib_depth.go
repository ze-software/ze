// Design: docs/architecture/testing/interop.md -- BGP/FIB depth interop proofs (ECMP, recursive NH, IGP cost).
// Related: register_fib_depth.go -- scenario constants and specialCheckers registration.
// Related: check_fib_depth_test.go -- pure parser and predicate unit tests.
//
// Peer source contracts:
//   - FRR 10.7.1: zebra/zebra_vty.c:vty_show_ip_route and lib/nexthop.c:nexthop_json_helper
//     emit selected/destSelected/installed and per-nexthop active/fib JSON fields;
//     bgpd/bgp_route.c multipath-relax and zebra/zebra_rnh.c NHT resolve ECMP and recursive NHs.
//   - BIRD 3.4.0: nest/rt-table.c:rt_get_igp_metric reads EA_GEN_IGP_METRIC (igp_metric),
//     proto/bgp/attrs.c:bgp_rte_better compares IGP metric at RFC 4271 Section 9.1.2.2(e),
//     and nest/rt-show.c:rt_show_rte prints "[<protocol> ...] *" for the primary route.
//   - GoBGP 4.11.0: cmd/gobgp/global.go:parsePath parses origin, local-pref, med, nexthop.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// zeBestPathDecision holds the parsed verdict from `show bgp rib best | prefix <p> | reason`.
type zeBestPathDecision struct {
	Winner     netip.Addr
	Candidates []netip.Addr
	Step       string
}

// birdRouteCandidate is one candidate block from `birdc show route for <prefix> all`.
type birdRouteCandidate struct {
	Protocol   string
	Selected   bool
	Via        netip.Addr
	BGPNextHop netip.Addr
}

// zePeerSessionCounters captures the live session identity counters of a Ze BGP peer.
type zePeerSessionCounters struct {
	Established int64
	Dropped     int64
}

// checkECMPFRR validates symmetric eBGP ECMP FIB parity (AC-3) and member
// withdrawal shrinkage (AC-14) between Ze and FRR 10.7.1.
func checkECMPFRR(ctx context.Context, check *interoplab.CheckContext) error {
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, scenarioECMPFRR, assertion, cause)
	}
	if !check.Network.IPv4.IsValid() {
		return fail(1, errors.New("ECMP scenario has no selected IPv4 network"))
	}
	zeAddr := networkHostAddress(check.Network, 2)
	frrAddr := networkHostAddress(check.Network, 3)
	gobgpAddr := networkHostAddress(check.Network, 5)
	sinkAddr := networkHostAddress(check.Network, extendedSinkHost)
	gobgpIP := netip.MustParseAddr(gobgpAddr)
	sinkIP := netip.MustParseAddr(sinkAddr)
	bothHops := sortedAddrs(gobgpIP, sinkIP)
	onlyGoBGP := []netip.Addr{gobgpIP}
	onlySink := []netip.Addr{sinkIP}

	// Assertions 1-4: all four eBGP sessions (ze<->sink, ze<->gobgp, frr<->sink, frr<->gobgp) Established.
	if err := waitFRRNeighborEstablished(ctx, check.Lab, peerFRR, sinkAddr); err != nil {
		return fail(1, err)
	}
	if err := waitFRRNeighborEstablished(ctx, check.Lab, peerFRR, gobgpAddr); err != nil {
		return fail(2, err)
	}
	if err := waitFRRNeighborEstablished(ctx, check.Lab, peerFRRSink, zeAddr); err != nil {
		return fail(3, err)
	}
	if err := waitGoBGPNeighborEstablished(ctx, check.Lab, zeAddr); err != nil {
		return fail(4, err)
	}
	if err := waitGoBGPNeighborEstablished(ctx, check.Lab, frrAddr); err != nil {
		return fail(4, err)
	}
	if err := waitZePeerState(ctx, check.Lab, sinkAddr, fibDepthWaitBudget); err != nil {
		return fail(4, err)
	}
	if err := waitZePeerState(ctx, check.Lab, gobgpAddr, fibDepthWaitBudget); err != nil {
		return fail(4, err)
	}

	// Assertion 5: inject equal-cost subject and unequal-cost negative control on GoBGP.
	for _, prefix := range []string{ecmpEqualPrefix, ecmpUnequalPrefix} {
		if _, err := check.Lab.Exec(ctx, peerGoBGP, []string{
			cmdGoBGP, gobgpGlobal, gobgpRIB, gobgpAdd, prefix,
			"-a", gobgpFamilyIPv4, gobgpOrigin, fibDepthGoBGPOriginIGP, gobgpNextHop, gobgpAddr,
		}, nil); err != nil {
			return fail(5, err)
		}
	}

	// Assertions 6-8 (AC-3): equal-cost FIB parity on 10.100.0.0/24 and show ecmp-groups.
	frrEqualHops, err := waitFRRInstalledNextHops(ctx, check.Lab, ecmpEqualPrefix, bothHops)
	if err != nil {
		return fail(6, err)
	}
	zeEqualHops, err := waitLinuxKernelNextHops(ctx, check.Lab, "ze", ecmpEqualPrefix, fibDepthZeProtocol, bothHops)
	if err != nil {
		return fail(7, err)
	}
	if !slices.Equal(zeEqualHops, frrEqualHops) {
		return fail(7, fmt.Errorf("ECMP FIB parity mismatch on %s: ze=%v frr=%v", ecmpEqualPrefix, zeEqualHops, frrEqualHops))
	}
	if err := waitZeECMPGroup(ctx, check.Lab, ecmpEqualPrefix, bothHops); err != nil {
		return fail(8, err)
	}

	// Assertions 9-10: unequal-cost negative control 10.100.2.0/24 holds sole nexthop {.5} on both.
	if _, err := waitFRRInstalledNextHops(ctx, check.Lab, ecmpUnequalPrefix, onlyGoBGP); err != nil {
		return fail(9, err)
	}
	if _, err := waitLinuxKernelNextHops(ctx, check.Lab, "ze", ecmpUnequalPrefix, fibDepthZeProtocol, onlyGoBGP); err != nil {
		return fail(10, err)
	}

	// Assertions 11-13 (AC-14): withdraw GoBGP's equal-cost member; both FIBs shrink to sole {.15}.
	if _, err := check.Lab.Exec(ctx, peerGoBGP, []string{
		cmdGoBGP, gobgpGlobal, gobgpRIB, fibDepthGoBGPDel, ecmpEqualPrefix, "-a", gobgpFamilyIPv4,
	}, nil); err != nil {
		return fail(11, err)
	}
	frrShrunkHops, err := waitFRRInstalledNextHops(ctx, check.Lab, ecmpEqualPrefix, onlySink)
	if err != nil {
		return fail(12, err)
	}
	zeShrunkHops, err := waitLinuxKernelNextHops(ctx, check.Lab, "ze", ecmpEqualPrefix, fibDepthZeProtocol, onlySink)
	if err != nil {
		return fail(13, err)
	}
	if !slices.Equal(zeShrunkHops, frrShrunkHops) {
		return fail(13, fmt.Errorf("ECMP withdrawal parity mismatch on %s: ze=%v frr=%v", ecmpEqualPrefix, zeShrunkHops, frrShrunkHops))
	}
	return nil
}

// checkRecursiveNHFRR validates off-link recursive iBGP next-hop resolution (AC-2)
// and covering-route withdrawal cascade (AC-12) on Ze and FRR 10.7.1.
func checkRecursiveNHFRR(ctx context.Context, check *interoplab.CheckContext) error {
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, scenarioRecursiveNHFRR, assertion, cause)
	}
	if !check.Network.IPv4.IsValid() {
		return fail(1, errors.New("recursive next-hop scenario has no selected IPv4 network"))
	}
	zeAddr := networkHostAddress(check.Network, 2)
	frrAddr := networkHostAddress(check.Network, 3)
	gobgpAddr := networkHostAddress(check.Network, 5)
	gatewayAddr := networkHostAddress(check.Network, extendedSinkHost)
	gobgpIP := netip.MustParseAddr(gobgpAddr)
	gatewayIP := netip.MustParseAddr(gatewayAddr)
	recursiveNH := netip.MustParseAddr(recursiveResolvableNH)

	// Assertions 1-2: iBGP sessions Established and baseline Ze session counters recorded.
	if err := waitFRRNeighborEstablished(ctx, check.Lab, peerFRR, gobgpAddr); err != nil {
		return fail(1, err)
	}
	if err := waitGoBGPNeighborEstablished(ctx, check.Lab, zeAddr); err != nil {
		return fail(2, err)
	}
	if err := waitGoBGPNeighborEstablished(ctx, check.Lab, frrAddr); err != nil {
		return fail(2, err)
	}
	if err := waitZePeerState(ctx, check.Lab, gobgpAddr, fibDepthWaitBudget); err != nil {
		return fail(2, err)
	}
	beforeSession, err := queryZePeerSession(ctx, check.Lab, gobgpAddr)
	if err != nil {
		return fail(2, err)
	}

	// Assertion 3: inject recursive subject, unresolvable negative control, and direct positive control.
	for _, route := range []struct{ prefix, nextHop string }{
		{recursiveSubjectPrefix, recursiveResolvableNH},
		{recursiveUnresolvablePrefix, recursiveUnresolvableNH},
		{recursiveDirectPrefix, gobgpAddr},
	} {
		if _, err := check.Lab.Exec(ctx, peerGoBGP, []string{
			cmdGoBGP, gobgpGlobal, gobgpRIB, gobgpAdd, route.prefix,
			"-a", gobgpFamilyIPv4, gobgpOrigin, fibDepthGoBGPOriginIGP, gobgpNextHop, route.nextHop,
		}, nil); err != nil {
			return fail(3, err)
		}
	}

	// Assertions 4-9 (AC-2): both FRR and Ze resolve 10.200.0.0/24 via {.15}, install
	// direct control 10.200.2.0/24 via {.5}, and keep unresolvable 10.200.1.0/24 out of FIB.
	frrHops, err := waitFRRInstalledNextHops(ctx, check.Lab, recursiveSubjectPrefix, []netip.Addr{gatewayIP})
	if err != nil {
		return fail(4, err)
	}
	if _, err := waitLinuxKernelNextHops(ctx, check.Lab, peerFRR, recursiveSubjectPrefix, "", []netip.Addr{gatewayIP}); err != nil {
		return fail(4, err)
	}
	zeHops, err := waitLinuxKernelNextHops(ctx, check.Lab, "ze", recursiveSubjectPrefix, fibDepthZeProtocol, []netip.Addr{gatewayIP})
	if err != nil {
		return fail(5, err)
	}
	if !slices.Equal(zeHops, frrHops) {
		return fail(5, fmt.Errorf("recursive FIB parity mismatch on %s: ze=%v frr=%v", recursiveSubjectPrefix, zeHops, frrHops))
	}
	if err := waitZeNextHopResolved(ctx, check.Lab, recursiveNH, gatewayIP, recursiveSubjectPrefix); err != nil {
		return fail(6, err)
	}
	if _, err := waitFRRInstalledNextHops(ctx, check.Lab, recursiveDirectPrefix, []netip.Addr{gobgpIP}); err != nil {
		return fail(7, err)
	}
	if _, err := waitLinuxKernelNextHops(ctx, check.Lab, "ze", recursiveDirectPrefix, fibDepthZeProtocol, []netip.Addr{gobgpIP}); err != nil {
		return fail(7, err)
	}
	if err := requireFRRRouteAbsentNow(ctx, check.Lab, recursiveUnresolvablePrefix); err != nil {
		return fail(8, err)
	}
	if err := requireLinuxKernelRouteAbsentNow(ctx, check.Lab, peerFRR, recursiveUnresolvablePrefix); err != nil {
		return fail(8, err)
	}
	if err := requireLinuxKernelRouteAbsentNow(ctx, check.Lab, "ze", recursiveUnresolvablePrefix); err != nil {
		return fail(9, err)
	}

	// Assertions 10-14 (AC-12): remove covering static route on both Ze (SIGHUP) and FRR;
	// 10.200.0.0/24 vanishes from both kernel FIBs while 10.200.2.0/24 and Ze's iBGP session survive.
	if err := applyZeReload(ctx, check.Lab, zeMountedReloadConfig); err != nil {
		return fail(10, err)
	}
	var noStatic textbuf.Buffer
	noStatic.Str("no ip route ").Str(recursiveStaticCoverPrefix).Byte(' ').Str(gatewayAddr)
	if _, err := check.Lab.Exec(ctx, peerFRR, []string{
		cmdVtysh, "-c", frrConfigureTerminal, "-c", noStatic.String(),
	}, nil); err != nil {
		return fail(11, err)
	}
	if err := waitLinuxKernelRouteAbsent(ctx, check.Lab, peerFRR, recursiveSubjectPrefix); err != nil {
		return fail(12, err)
	}
	if err := waitLinuxKernelRouteAbsent(ctx, check.Lab, "ze", recursiveSubjectPrefix); err != nil {
		return fail(13, err)
	}
	if _, err := waitLinuxKernelNextHops(ctx, check.Lab, peerFRR, recursiveDirectPrefix, "", []netip.Addr{gobgpIP}); err != nil {
		return fail(14, err)
	}
	if _, err := waitLinuxKernelNextHops(ctx, check.Lab, "ze", recursiveDirectPrefix, fibDepthZeProtocol, []netip.Addr{gobgpIP}); err != nil {
		return fail(14, err)
	}
	if err := requireZePeerSessionUnchanged(ctx, check.Lab, gobgpAddr, beforeSession); err != nil {
		return fail(14, err)
	}
	return nil
}

// checkIGPCostBIRD validates iBGP IGP-cost best-path selection (AC-1) and
// cost-change reselection (AC-13) on Ze and BIRD 3.4.0.
func checkIGPCostBIRD(ctx context.Context, check *interoplab.CheckContext) error {
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, scenarioIGPCostBIRD, assertion, cause)
	}
	if !check.Network.IPv4.IsValid() {
		return fail(1, errors.New("IGP-cost scenario has no selected IPv4 network"))
	}
	zeAddr := networkHostAddress(check.Network, 2)
	frrAddr := networkHostAddress(check.Network, 3)
	birdAddr := networkHostAddress(check.Network, 4)
	gobgpAddr := networkHostAddress(check.Network, 5)
	frrIP := netip.MustParseAddr(frrAddr)
	gobgpIP := netip.MustParseAddr(gobgpAddr)
	frrNH := netip.MustParseAddr(igpCostFRRNextHop)
	gobgpNH := netip.MustParseAddr(igpCostGoBGPNextHop)
	birdProtocols := []string{igpCostBIRDPeerA, igpCostBIRDPeerB}

	// Assertions 1-4: all four iBGP sessions Established and Ze session counters snapshotted.
	if err := waitFRRNeighborEstablished(ctx, check.Lab, peerFRR, zeAddr); err != nil {
		return fail(1, err)
	}
	if err := waitFRRNeighborEstablished(ctx, check.Lab, peerFRR, birdAddr); err != nil {
		return fail(1, err)
	}
	if err := waitContains(ctx, check.Lab, peerBIRD, []string{cmdBirdc, birdcShowProtocols}, fibDepthWaitBudget, igpCostBIRDPeerA, stateEstablished); err != nil {
		return fail(2, err)
	}
	if err := waitContains(ctx, check.Lab, peerBIRD, []string{cmdBirdc, birdcShowProtocols}, fibDepthWaitBudget, igpCostBIRDPeerB, stateEstablished); err != nil {
		return fail(2, err)
	}
	if err := waitGoBGPNeighborEstablished(ctx, check.Lab, zeAddr); err != nil {
		return fail(3, err)
	}
	if err := waitGoBGPNeighborEstablished(ctx, check.Lab, birdAddr); err != nil {
		return fail(3, err)
	}
	if err := waitZePeerState(ctx, check.Lab, frrAddr, fibDepthWaitBudget); err != nil {
		return fail(4, err)
	}
	if err := waitZePeerState(ctx, check.Lab, gobgpAddr, fibDepthWaitBudget); err != nil {
		return fail(4, err)
	}
	frrSessionBefore, err := queryZePeerSession(ctx, check.Lab, frrAddr)
	if err != nil {
		return fail(4, err)
	}
	gobgpSessionBefore, err := queryZePeerSession(ctx, check.Lab, gobgpAddr)
	if err != nil {
		return fail(4, err)
	}

	// Assertion 5: inject GoBGP's candidate with matching LOCAL_PREF 100, ORIGIN igp, MED 0.
	if _, err := check.Lab.Exec(ctx, peerGoBGP, []string{
		cmdGoBGP, gobgpGlobal, gobgpRIB, gobgpAdd, igpCostSubjectPrefix,
		"-a", gobgpFamilyIPv4,
		gobgpOrigin, fibDepthGoBGPOriginIGP,
		fibDepthGoBGPLocalPref, "100",
		fibDepthGoBGPMED, "0",
		gobgpNextHop, igpCostGoBGPNextHop,
	}, nil); err != nil {
		return fail(5, err)
	}

	// Assertions 6-9 (AC-1): Phase 1 -- both candidates held, Ze selects .5 at step igp-cost,
	// Ze kernel FIB (proto 250) routes via .5, BIRD selects peer_b via .5 (bgp_next_hop 10.222.0.1).
	wantCandidates := map[netip.Addr]netip.Addr{frrIP: frrNH, gobgpIP: gobgpNH}
	if err := waitZeReceivedCandidates(ctx, check.Lab, igpCostSubjectPrefix, wantCandidates); err != nil {
		return fail(6, err)
	}
	if err := waitZeBestPathReason(ctx, check.Lab, igpCostSubjectPrefix, gobgpIP, igpCostStepName, sortedAddrs(frrIP, gobgpIP)); err != nil {
		return fail(7, err)
	}
	if _, err := waitLinuxKernelNextHops(ctx, check.Lab, "ze", igpCostSubjectPrefix, fibDepthZeProtocol, []netip.Addr{gobgpIP}); err != nil {
		return fail(8, err)
	}
	if err := waitBIRDRouteWinner(ctx, check.Lab, igpCostSubjectPrefix, igpCostBIRDPeerB, birdProtocols, gobgpIP, gobgpNH); err != nil {
		return fail(9, err)
	}

	// Assertions 10-15 (AC-13): Phase 2 -- raise 10.222.0.0/24 cost 10 -> 30 on both Ze and BIRD;
	// both Ze and BIRD flip winner to .3 without dropping any BGP session.
	if err := applyZeReload(ctx, check.Lab, zeMountedReloadConfig); err != nil {
		return fail(10, err)
	}
	if _, err := check.Lab.Exec(ctx, peerBIRD, []string{cmdBirdc, "enable " + igpCostBIRDExpensiveStat}, nil); err != nil {
		return fail(11, err)
	}
	if _, err := check.Lab.Exec(ctx, peerBIRD, []string{cmdBirdc, "disable " + igpCostBIRDCheapStatic}, nil); err != nil {
		return fail(11, err)
	}
	if err := waitZeBestPathReason(ctx, check.Lab, igpCostSubjectPrefix, frrIP, igpCostStepName, sortedAddrs(frrIP, gobgpIP)); err != nil {
		return fail(12, err)
	}
	if _, err := waitLinuxKernelNextHops(ctx, check.Lab, "ze", igpCostSubjectPrefix, fibDepthZeProtocol, []netip.Addr{frrIP}); err != nil {
		return fail(13, err)
	}
	if err := waitBIRDRouteWinner(ctx, check.Lab, igpCostSubjectPrefix, igpCostBIRDPeerA, birdProtocols, frrIP, frrNH); err != nil {
		return fail(14, err)
	}
	if err := requireZePeerSessionUnchanged(ctx, check.Lab, frrAddr, frrSessionBefore); err != nil {
		return fail(15, err)
	}
	if err := requireZePeerSessionUnchanged(ctx, check.Lab, gobgpAddr, gobgpSessionBefore); err != nil {
		return fail(15, err)
	}
	return nil
}

// --- Lab wait and query wrappers ---

func waitFRRNeighborEstablished(ctx context.Context, lab interoplab.CheckerLab, peer, neighbor string) error {
	var cmd textbuf.Buffer
	cmd.Str("show bgp neighbor ").Str(neighbor)
	return waitContains(ctx, lab, peer, []string{cmdVtysh, "-c", cmd.String()}, fibDepthWaitBudget, "BGP state = Established")
}

func waitGoBGPNeighborEstablished(ctx context.Context, lab interoplab.CheckerLab, neighbor string) error {
	return waitContainsFold(ctx, lab, peerGoBGP, []string{cmdGoBGP, gobgpNeighbor, neighbor}, fibDepthWaitBudget, peerStateEstablished)
}

func waitFRRInstalledNextHops(ctx context.Context, lab interoplab.CheckerLab, prefix string, want []netip.Addr) ([]netip.Addr, error) {
	var cmd textbuf.Buffer
	cmd.Str("show ip route ").Str(prefix).Str(" json")
	argv := []string{cmdVtysh, "-c", cmd.String()}
	var lastOutput string
	var lastHops []netip.Addr
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     fibDepthWaitBudget,
		Interval:    time.Second,
		Description: peerFRR + " installed " + fibDepthFRRProtocolBGP + " route for " + prefix,
	}, func(probeCtx context.Context) (string, error) {
		output, queryErr := lab.Query(probeCtx, peerFRR, argv, nil)
		lastOutput = output
		return output, queryErr
	}, func(output string) bool {
		hops, parseErr := parseFRRInstalledNextHops(output, prefix)
		if parseErr != nil {
			return false
		}
		lastHops = hops
		return slices.Equal(hops, want)
	})
	if err != nil {
		return nil, interoplab.WithSearchedOutput(fmt.Errorf("%s %s: want active FIB nexthops %v: %w", peerFRR, prefix, want, err), lastOutput)
	}
	return lastHops, nil
}

func requireFRRRouteAbsentNow(ctx context.Context, lab interoplab.CheckerLab, prefix string) error {
	var cmd textbuf.Buffer
	cmd.Str("show ip route ").Str(prefix).Str(" json")
	output, err := lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", cmd.String()}, nil)
	if err != nil {
		return err
	}
	if err := requireFRRInstalledRouteAbsent(output, prefix); err != nil {
		return interoplab.WithSearchedOutput(err, output)
	}
	return nil
}

func linuxKernelRouteArgv(prefix string, numeric bool) []string {
	if numeric {
		return []string{fibDepthCmdIP, fibDepthFlagIPv4, fibDepthFlagJSON, fibDepthFlagNumeric, ipObjectRoute, ipActionShow, ipOptionExact, prefix}
	}
	return []string{fibDepthCmdIP, fibDepthFlagIPv4, fibDepthFlagJSON, ipObjectRoute, ipActionShow, ipOptionExact, prefix}
}

func waitLinuxKernelNextHops(ctx context.Context, lab interoplab.CheckerLab, peer, prefix, wantProto string, want []netip.Addr) ([]netip.Addr, error) {
	argv := linuxKernelRouteArgv(prefix, wantProto != "")
	var lastOutput string
	var lastHops []netip.Addr
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     fibDepthWaitBudget,
		Interval:    time.Second,
		Description: peer + " kernel FIB route for " + prefix,
	}, func(probeCtx context.Context) (string, error) {
		output, queryErr := lab.Query(probeCtx, peer, argv, nil)
		lastOutput = output
		return output, queryErr
	}, func(output string) bool {
		hops, parseErr := parseLinuxKernelNextHops(output, prefix, wantProto)
		if parseErr != nil {
			return false
		}
		lastHops = hops
		return slices.Equal(hops, want)
	})
	if err != nil {
		return nil, interoplab.WithSearchedOutput(fmt.Errorf("%s kernel FIB %s: want nexthops %v (proto %q): %w", peer, prefix, want, wantProto, err), lastOutput)
	}
	return lastHops, nil
}

func requireLinuxKernelRouteAbsentNow(ctx context.Context, lab interoplab.CheckerLab, peer, prefix string) error {
	argv := linuxKernelRouteArgv(prefix, peer == "ze")
	output, err := lab.Query(ctx, peer, argv, nil)
	if err != nil {
		return err
	}
	if err := requireLinuxKernelRouteAbsent(output, prefix); err != nil {
		return interoplab.WithSearchedOutput(err, output)
	}
	return nil
}

func waitLinuxKernelRouteAbsent(ctx context.Context, lab interoplab.CheckerLab, peer, prefix string) error {
	argv := linuxKernelRouteArgv(prefix, peer == "ze")
	var lastOutput string
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     fibDepthWaitBudget,
		Interval:    time.Second,
		Description: peer + " kernel FIB withdrawal of " + prefix,
	}, func(probeCtx context.Context) (string, error) {
		output, queryErr := lab.Query(probeCtx, peer, argv, nil)
		lastOutput = output
		return output, queryErr
	}, func(output string) bool {
		return requireLinuxKernelRouteAbsent(output, prefix) == nil
	})
	if err != nil {
		return interoplab.WithSearchedOutput(fmt.Errorf("%s kernel FIB still holds %s: %w", peer, prefix, err), lastOutput)
	}
	return nil
}

func waitZeECMPGroup(ctx context.Context, lab interoplab.CheckerLab, prefix string, allowedHops []netip.Addr) error {
	argv := zeCommand(fibDepthZeShowECMPGroups)
	var lastOutput string
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     fibDepthWaitBudget,
		Interval:    time.Second,
		Description: "ze show ecmp-groups for " + prefix,
	}, func(probeCtx context.Context) (string, error) {
		output, queryErr := lab.Query(probeCtx, "ze", argv, queryEnvironment("ze", argv))
		lastOutput = output
		return output, queryErr
	}, func(output string) bool {
		return requireZeECMPGroup(output, prefix, allowedHops) == nil
	})
	if err != nil {
		return interoplab.WithSearchedOutput(err, lastOutput)
	}
	return nil
}

func waitZeNextHopResolved(ctx context.Context, lab interoplab.CheckerLab, nextHop, directNH netip.Addr, dependent string) error {
	argv := zeCommand(fibDepthZeShowNextHopTbl)
	var lastOutput string
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     fibDepthWaitBudget,
		Interval:    time.Second,
		Description: "ze show nexthop-table for " + nextHop.String(),
	}, func(probeCtx context.Context) (string, error) {
		output, queryErr := lab.Query(probeCtx, "ze", argv, queryEnvironment("ze", argv))
		lastOutput = output
		return output, queryErr
	}, func(output string) bool {
		return requireZeNextHopResolved(output, nextHop, directNH, dependent) == nil
	})
	if err != nil {
		return interoplab.WithSearchedOutput(err, lastOutput)
	}
	return nil
}

func waitZeReceivedCandidates(ctx context.Context, lab interoplab.CheckerLab, prefix string, want map[netip.Addr]netip.Addr) error {
	argv := zeCommand(fibDepthZeShowRIBReceived)
	var lastOutput string
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     fibDepthWaitBudget,
		Interval:    time.Second,
		Description: "ze received BGP candidates for " + prefix,
	}, func(probeCtx context.Context) (string, error) {
		output, queryErr := lab.Query(probeCtx, "ze", argv, queryEnvironment("ze", argv))
		lastOutput = output
		return output, queryErr
	}, func(output string) bool {
		return requireZeReceivedCandidates(output, prefix, want) == nil
	})
	if err != nil {
		return interoplab.WithSearchedOutput(err, lastOutput)
	}
	return nil
}

func waitZeBestPathReason(ctx context.Context, lab interoplab.CheckerLab, prefix string, wantWinner netip.Addr, wantStep string, wantCandidates []netip.Addr) error {
	var cmd textbuf.Buffer
	cmd.Str("show bgp rib best | prefix ").Str(prefix).Str(" | reason")
	argv := zeCommand(cmd.String())
	var lastOutput string
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     fibDepthWaitBudget,
		Interval:    time.Second,
		Description: "ze best-path reason for " + prefix,
	}, func(probeCtx context.Context) (string, error) {
		output, queryErr := lab.Query(probeCtx, "ze", argv, queryEnvironment("ze", argv))
		lastOutput = output
		return output, queryErr
	}, func(output string) bool {
		decision, parseErr := parseZeBestPathReason(output, prefix)
		if parseErr != nil {
			return false
		}
		if decision.Winner != wantWinner {
			return false
		}
		if decision.Step != wantStep {
			return false
		}
		return slices.Equal(decision.Candidates, wantCandidates)
	})
	if err != nil {
		return interoplab.WithSearchedOutput(fmt.Errorf("ze best-path reason for %s: want winner %s at step %q with candidates %v: %w",
			prefix, wantWinner, wantStep, wantCandidates, err), lastOutput)
	}
	return nil
}

func waitBIRDRouteWinner(ctx context.Context, lab interoplab.CheckerLab, prefix, wantProtocol string, wantProtocols []string, wantVia, wantBGPNextHop netip.Addr) error {
	var cmd textbuf.Buffer
	cmd.Str("show route for ").Str(prefix).Str(" all")
	argv := []string{cmdBirdc, cmd.String()}
	var lastOutput string
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     fibDepthWaitBudget,
		Interval:    time.Second,
		Description: "BIRD selected route for " + prefix + " via " + wantProtocol,
	}, func(probeCtx context.Context) (string, error) {
		output, queryErr := lab.Query(probeCtx, peerBIRD, argv, nil)
		lastOutput = output
		return output, queryErr
	}, func(output string) bool {
		return requireBIRDRouteWinner(output, prefix, wantProtocol, wantProtocols, wantVia, wantBGPNextHop) == nil
	})
	if err != nil {
		return interoplab.WithSearchedOutput(fmt.Errorf("BIRD %s: want selected %s via %s (bgp_next_hop %s): %w",
			prefix, wantProtocol, wantVia, wantBGPNextHop, err), lastOutput)
	}
	return nil
}

func queryZePeerSession(ctx context.Context, lab interoplab.CheckerLab, peer string) (zePeerSessionCounters, error) {
	argv := zeCommand("show bgp peer " + peer + " detail")
	output, err := lab.Query(ctx, "ze", argv, queryEnvironment("ze", argv))
	if err != nil {
		return zePeerSessionCounters{}, err
	}
	counters, err := parseZePeerSession(output, peer)
	if err != nil {
		return zePeerSessionCounters{}, interoplab.WithSearchedOutput(err, output)
	}
	return counters, nil
}

func requireZePeerSessionUnchanged(ctx context.Context, lab interoplab.CheckerLab, peer string, before zePeerSessionCounters) error {
	after, err := queryZePeerSession(ctx, lab, peer)
	if err != nil {
		return err
	}
	if after != before {
		return fmt.Errorf("ze BGP session to %s reset across reload: before=%+v after=%+v", peer, before, after)
	}
	return nil
}

// --- Pure parsers and predicates (no lab I/O) ---

// parseFRRInstalledNextHops extracts the sorted unique active+fib next-hop IPs
// of FRR 10.7.1's `show ip route <prefix> json` BGP entry for prefix.
// Selection, installation, and BGP protocol MUST hold on the same route entry,
// and each returned next hop MUST carry both active and fib true
// (zebra_vty.c:vty_show_ip_route, lib/nexthop.c:nexthop_json_helper).
func parseFRRInstalledNextHops(output, prefix string) ([]netip.Addr, error) {
	wantPrefix, err := netip.ParsePrefix(prefix)
	if err != nil {
		return nil, fmt.Errorf("invalid expected FRR prefix %q: %w", prefix, err)
	}
	var document map[string][]struct {
		Prefix       string `json:"prefix"`
		Protocol     string `json:"protocol"`
		Selected     bool   `json:"selected"`
		DestSelected bool   `json:"destSelected"`
		Installed    bool   `json:"installed"`
		Nexthops     []struct {
			IP     string `json:"ip"`
			Active bool   `json:"active"`
			FIB    bool   `json:"fib"`
		} `json:"nexthops"`
	}
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return nil, fmt.Errorf("decode FRR installed-route JSON: %w", err)
	}
	if document == nil {
		return nil, errors.New("FRR installed-route JSON is null")
	}
	for _, route := range document[prefix] {
		gotPrefix, parseErr := netip.ParsePrefix(route.Prefix)
		if parseErr != nil {
			continue
		}
		if gotPrefix != wantPrefix {
			continue
		}
		if route.Protocol != fibDepthFRRProtocolBGP {
			continue
		}
		if !route.Selected {
			continue
		}
		if !route.DestSelected {
			continue
		}
		if !route.Installed {
			continue
		}
		hops := make([]netip.Addr, 0, len(route.Nexthops))
		for _, hop := range route.Nexthops {
			if !hop.Active {
				continue
			}
			if !hop.FIB {
				continue
			}
			addr, addrErr := netip.ParseAddr(hop.IP)
			if addrErr != nil {
				return nil, fmt.Errorf("invalid FRR active FIB next hop %q: %w", hop.IP, addrErr)
			}
			hops = append(hops, addr)
		}
		if len(hops) == 0 {
			return nil, fmt.Errorf("FRR installed %s route for %s carries no active FIB next hops", fibDepthFRRProtocolBGP, prefix)
		}
		return sortedUniqueAddrs(hops), nil
	}
	return nil, fmt.Errorf("FRR has no selected, installed %s route for %s", fibDepthFRRProtocolBGP, prefix)
}

// requireFRRInstalledRouteAbsent verifies that FRR's `show ip route <prefix> json`
// holds no installed route for prefix. Non-JSON or null answers fail closed.
func requireFRRInstalledRouteAbsent(output, prefix string) error {
	var document map[string][]struct {
		Prefix    string `json:"prefix"`
		Installed bool   `json:"installed"`
	}
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return fmt.Errorf("decode FRR route absence JSON: %w", err)
	}
	if document == nil {
		return errors.New("FRR route absence JSON is null")
	}
	for _, route := range document[prefix] {
		if route.Installed {
			return fmt.Errorf("FRR still holds an installed route for %s", prefix)
		}
	}
	return nil
}

// parseLinuxKernelNextHops extracts the sorted unique gateway addresses from
// iproute2's `ip -4 -j [-N] route show exact <prefix>` JSON array. When
// wantProto is non-empty, the route's protocol field MUST equal wantProto.
// Both single-path ("gateway") and multipath ("nexthops[].gateway") shapes are
// decoded (iproute2 ip/iproute.c:print_route, print_rta_multipath).
func parseLinuxKernelNextHops(output, prefix, wantProto string) ([]netip.Addr, error) {
	wantPrefix, err := netip.ParsePrefix(prefix)
	if err != nil {
		return nil, fmt.Errorf("invalid expected kernel prefix %q: %w", prefix, err)
	}
	var routes []struct {
		Destination string          `json:"dst"`
		Gateway     string          `json:"gateway"`
		Protocol    json.RawMessage `json:"protocol"`
		Nexthops    []struct {
			Gateway string `json:"gateway"`
		} `json:"nexthops"`
	}
	if err := json.Unmarshal([]byte(output), &routes); err != nil {
		return nil, fmt.Errorf("decode Linux kernel route JSON: %w", err)
	}
	if routes == nil {
		return nil, errors.New("kernel route JSON is null")
	}
	if len(routes) != 1 {
		return nil, fmt.Errorf("kernel holds %d routes for %s, want 1", len(routes), prefix)
	}
	route := routes[0]
	gotPrefix, err := netip.ParsePrefix(route.Destination)
	if err != nil {
		return nil, fmt.Errorf("invalid kernel route dst %q: %w", route.Destination, err)
	}
	if gotPrefix != wantPrefix {
		return nil, fmt.Errorf("kernel route dst is %s, want %s", gotPrefix, wantPrefix)
	}
	if wantProto != "" {
		gotProto := strings.Trim(strings.TrimSpace(string(route.Protocol)), `"`)
		if gotProto != wantProto {
			return nil, fmt.Errorf("kernel route %s has protocol %q, want %q", prefix, gotProto, wantProto)
		}
	}
	var hops []netip.Addr
	if route.Gateway != "" {
		addr, addrErr := netip.ParseAddr(route.Gateway)
		if addrErr != nil {
			return nil, fmt.Errorf("invalid kernel gateway %q: %w", route.Gateway, addrErr)
		}
		hops = append(hops, addr)
	}
	for _, nh := range route.Nexthops {
		if nh.Gateway == "" {
			return nil, fmt.Errorf("kernel multipath route %s has a nexthop with no gateway", prefix)
		}
		addr, addrErr := netip.ParseAddr(nh.Gateway)
		if addrErr != nil {
			return nil, fmt.Errorf("invalid kernel multipath gateway %q: %w", nh.Gateway, addrErr)
		}
		hops = append(hops, addr)
	}
	if len(hops) == 0 {
		return nil, fmt.Errorf("kernel route %s carries no gateway or multipath nexthops", prefix)
	}
	return sortedUniqueAddrs(hops), nil
}

// requireLinuxKernelRouteAbsent verifies that `ip -4 -j [-N] route show exact <prefix>`
// returns a valid JSON array holding no route for prefix.
func requireLinuxKernelRouteAbsent(output, prefix string) error {
	var routes []struct {
		Destination string `json:"dst"`
	}
	if err := json.Unmarshal([]byte(output), &routes); err != nil {
		return fmt.Errorf("decode kernel route absence JSON: %w", err)
	}
	if routes == nil {
		return errors.New("kernel route absence JSON is null")
	}
	if len(routes) != 0 {
		return fmt.Errorf("kernel still holds %d route(s) for %s", len(routes), prefix)
	}
	return nil
}

// requireZeECMPGroup verifies that Ze's `show ecmp-groups` JSON array lists
// prefix with at least one equal-cost sibling path whose next-hop belongs to
// allowedHops (sysrib.go:showECMPGroups reports sibling paths excluding the primary winner).
func requireZeECMPGroup(output, prefix string, allowedHops []netip.Addr) error {
	wantPrefix, err := netip.ParsePrefix(prefix)
	if err != nil {
		return fmt.Errorf("invalid expected ECMP prefix %q: %w", prefix, err)
	}
	var groups []struct {
		Prefix string `json:"prefix"`
		Paths  []struct {
			NextHop string `json:"next-hop"`
		} `json:"paths"`
	}
	if err := json.Unmarshal([]byte(output), &groups); err != nil {
		return fmt.Errorf("decode ze show ecmp-groups JSON: %w", err)
	}
	if groups == nil {
		return errors.New("ze show ecmp-groups JSON is null")
	}
	for _, group := range groups {
		gotPrefix, parseErr := netip.ParsePrefix(group.Prefix)
		if parseErr != nil {
			continue
		}
		if gotPrefix != wantPrefix {
			continue
		}
		if len(group.Paths) == 0 {
			return fmt.Errorf("ze show ecmp-groups entry for %s has no sibling paths", prefix)
		}
		for _, path := range group.Paths {
			addr, addrErr := netip.ParseAddr(path.NextHop)
			if addrErr != nil {
				return fmt.Errorf("invalid ze ECMP sibling next hop %q: %w", path.NextHop, addrErr)
			}
			if !slices.Contains(allowedHops, addr) {
				return fmt.Errorf("ze show ecmp-groups %s sibling %s is outside %v", prefix, addr, allowedHops)
			}
		}
		return nil
	}
	return fmt.Errorf("ze show ecmp-groups does not list %s", prefix)
}

// requireZeNextHopResolved verifies that Ze's `show nexthop-table` JSON array
// tracks nextHop as resolved to directNH with dependent in its dependents list.
func requireZeNextHopResolved(output string, nextHop, directNH netip.Addr, dependent string) error {
	wantDependent, err := netip.ParsePrefix(dependent)
	if err != nil {
		return fmt.Errorf("invalid expected dependent prefix %q: %w", dependent, err)
	}
	var rows []struct {
		NextHop    string   `json:"next-hop"`
		Resolved   bool     `json:"resolved"`
		DirectNH   string   `json:"direct-nh"`
		Dependents []string `json:"dependents"`
	}
	if err := json.Unmarshal([]byte(output), &rows); err != nil {
		return fmt.Errorf("decode ze show nexthop-table JSON: %w", err)
	}
	if rows == nil {
		return errors.New("ze show nexthop-table JSON is null")
	}
	for _, row := range rows {
		gotNH, parseErr := netip.ParseAddr(row.NextHop)
		if parseErr != nil {
			continue
		}
		if gotNH != nextHop {
			continue
		}
		if !row.Resolved {
			return fmt.Errorf("ze nexthop-table reports %s unresolved", nextHop)
		}
		gotDirect, directErr := netip.ParseAddr(row.DirectNH)
		if directErr != nil {
			return fmt.Errorf("invalid ze direct-nh %q for %s: %w", row.DirectNH, nextHop, directErr)
		}
		if gotDirect != directNH {
			return fmt.Errorf("ze nexthop-table resolves %s to %s, want %s", nextHop, gotDirect, directNH)
		}
		for _, dep := range row.Dependents {
			gotDep, depErr := netip.ParsePrefix(dep)
			if depErr == nil && gotDep == wantDependent {
				return nil
			}
		}
		return fmt.Errorf("ze nexthop-table for %s omits dependent %s (have %v)", nextHop, dependent, row.Dependents)
	}
	return fmt.Errorf("ze show nexthop-table does not track %s", nextHop)
}

// requireZeReceivedCandidates verifies that Ze's `show bgp rib | received`
// piped JSON document holds every expected (peer -> nextHop) IPv4 unicast
// candidate for prefix.
func requireZeReceivedCandidates(output, prefix string, want map[netip.Addr]netip.Addr) error {
	var routes []struct {
		Peer      string               `json:"peer"`
		Direction rpc.MessageDirection `json:"direction"`
		Prefix    string               `json:"prefix"`
		Family    string               `json:"family"`
		NextHop   string               `json:"next-hop"`
	}
	if err := decodeZePipedRows(output, zeRIBRowsKey, zePipeReceived, &routes); err != nil {
		return err
	}
	found := make(map[netip.Addr]netip.Addr, len(want))
	for _, route := range routes {
		if route.Direction != rpc.DirectionReceived {
			continue
		}
		if route.Family != zeIPv4Unicast {
			continue
		}
		if route.Prefix != prefix {
			continue
		}
		peerAddr, peerErr := netip.ParseAddr(route.Peer)
		if peerErr != nil {
			continue
		}
		nhAddr, nhErr := netip.ParseAddr(route.NextHop)
		if nhErr != nil {
			return fmt.Errorf("ze received %s from %s has invalid next-hop %q: %w", prefix, peerAddr, route.NextHop, nhErr)
		}
		found[peerAddr] = nhAddr
	}
	for wantPeer, wantNH := range want {
		gotNH, ok := found[wantPeer]
		if !ok {
			return fmt.Errorf("ze received RIB for %s is missing candidate from peer %s", prefix, wantPeer)
		}
		if gotNH != wantNH {
			return fmt.Errorf("ze received %s from %s carries next-hop %s, want %s", prefix, wantPeer, gotNH, wantNH)
		}
	}
	return nil
}

// parseZeBestPathReason decodes Ze's `show bgp rib best | prefix <p> | reason`
// JSON response and extracts the winning peer, sorted candidate peer set, and
// deciding step for prefix.
func parseZeBestPathReason(output, prefix string) (zeBestPathDecision, error) {
	if strings.TrimSpace(output) == "" {
		return zeBestPathDecision{}, errors.New("ze best-path reason answer is empty")
	}
	var document struct {
		Entries []struct {
			Prefix     string   `json:"prefix"`
			WinnerPeer string   `json:"winner-peer"`
			Candidates []string `json:"candidates"`
			Steps      []struct {
				Step   string `json:"step"`
				Winner string `json:"winner"`
			} `json:"steps"`
		} `json:"best-path-reason"`
	}
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return zeBestPathDecision{}, fmt.Errorf("decode ze best-path reason JSON: %w", err)
	}
	if len(document.Entries) != 1 {
		return zeBestPathDecision{}, fmt.Errorf("ze best-path reason returned %d entries for %s, want 1", len(document.Entries), prefix)
	}
	entry := document.Entries[0]
	if entry.Prefix != prefix {
		return zeBestPathDecision{}, fmt.Errorf("ze best-path reason answered prefix %q, want %s", entry.Prefix, prefix)
	}
	winner, err := netip.ParseAddr(entry.WinnerPeer)
	if err != nil {
		return zeBestPathDecision{}, fmt.Errorf("invalid ze best-path winner %q: %w", entry.WinnerPeer, err)
	}
	if len(entry.Steps) == 0 {
		return zeBestPathDecision{}, fmt.Errorf("ze best-path reason for %s carries no comparison step", prefix)
	}
	step := entry.Steps[len(entry.Steps)-1].Step
	if step == "" {
		return zeBestPathDecision{}, fmt.Errorf("ze best-path reason for %s has an empty deciding step", prefix)
	}
	candidates := make([]netip.Addr, 0, len(entry.Candidates))
	for _, raw := range entry.Candidates {
		addr, parseErr := netip.ParseAddr(raw)
		if parseErr != nil {
			return zeBestPathDecision{}, fmt.Errorf("invalid ze best-path candidate %q: %w", raw, parseErr)
		}
		candidates = append(candidates, addr)
	}
	return zeBestPathDecision{
		Winner:     winner,
		Candidates: sortedUniqueAddrs(candidates),
		Step:       step,
	}, nil
}

// parseBIRDRouteAll parses BIRD 3.4.0's `show route for <prefix> all` output
// into per-candidate blocks (nest/rt-show.c:rt_show_rte). The first route line
// MUST name prefix as its first field so a covering route cannot stand in for
// an exact match. Tab-indented attribute lines belong to the preceding route block.
func parseBIRDRouteAll(output, prefix string) ([]birdRouteCandidate, error) {
	wantPrefix, err := netip.ParsePrefix(prefix)
	if err != nil {
		return nil, fmt.Errorf("invalid expected BIRD prefix %q: %w", prefix, err)
	}
	var candidates []birdRouteCandidate
	sawExactPrefix := false
	for line := range strings.SplitSeq(output, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, "\t") {
			if len(candidates) == 0 {
				continue
			}
			current := &candidates[len(candidates)-1]
			trimmed := strings.TrimSpace(line)
			if rest, ok := strings.CutPrefix(trimmed, "via "); ok {
				fields := strings.Fields(rest)
				if len(fields) > 0 {
					if addr, parseErr := netip.ParseAddr(fields[0]); parseErr == nil {
						current.Via = addr
					}
				}
			}
			if rest, ok := strings.CutPrefix(trimmed, "bgp_next_hop:"); ok {
				fields := strings.Fields(rest)
				if len(fields) > 0 {
					if addr, parseErr := netip.ParseAddr(fields[0]); parseErr == nil {
						current.BGPNextHop = addr
					}
				}
			}
			continue
		}
		open := strings.IndexByte(line, '[')
		if open < 0 {
			continue
		}
		shut := strings.IndexByte(line[open:], ']')
		if shut < 0 {
			continue
		}
		shut += open
		proto, _, _ := strings.Cut(strings.TrimSpace(line[open+1:shut]), " ")
		if proto == "" {
			continue
		}
		if len(candidates) == 0 {
			fields := strings.Fields(line)
			if len(fields) == 0 {
				continue
			}
			gotPrefix, parseErr := netip.ParsePrefix(fields[0])
			if parseErr != nil {
				continue
			}
			if gotPrefix != wantPrefix {
				return nil, fmt.Errorf("BIRD answered prefix %s, want %s", gotPrefix, wantPrefix)
			}
			sawExactPrefix = true
		}
		selected := strings.HasPrefix(line[shut+1:], " *")
		candidates = append(candidates, birdRouteCandidate{
			Protocol: proto,
			Selected: selected,
		})
	}
	if !sawExactPrefix {
		return nil, fmt.Errorf("BIRD route output names no exact route for %s", prefix)
	}
	return candidates, nil
}

// requireBIRDRouteWinner verifies that BIRD holds every protocol in
// wantProtocols for prefix, marks exactly one candidate as selected (*), and
// that the selected candidate matches wantProtocol, wantVia, and wantBGPNextHop.
func requireBIRDRouteWinner(output, prefix, wantProtocol string, wantProtocols []string, wantVia, wantBGPNextHop netip.Addr) error {
	candidates, err := parseBIRDRouteAll(output, prefix)
	if err != nil {
		return err
	}
	present := make(map[string]bool, len(candidates))
	var winner *birdRouteCandidate
	selectedCount := 0
	for i := range candidates {
		present[candidates[i].Protocol] = true
		if candidates[i].Selected {
			selectedCount++
			winner = &candidates[i]
		}
	}
	for _, proto := range wantProtocols {
		if !present[proto] {
			return fmt.Errorf("BIRD route table for %s is missing candidate protocol %q", prefix, proto)
		}
	}
	if selectedCount != 1 {
		return fmt.Errorf("BIRD marked %d selected routes for %s, want 1", selectedCount, prefix)
	}
	if winner.Protocol != wantProtocol {
		return fmt.Errorf("BIRD selected %s for %s, want %s", winner.Protocol, prefix, wantProtocol)
	}
	if winner.Via != wantVia {
		return fmt.Errorf("BIRD selected route %s (%s) resolves via %s, want %s", prefix, winner.Protocol, winner.Via, wantVia)
	}
	if winner.BGPNextHop != wantBGPNextHop {
		return fmt.Errorf("BIRD selected route %s (%s) has bgp_next_hop %s, want %s", prefix, winner.Protocol, winner.BGPNextHop, wantBGPNextHop)
	}
	return nil
}

// parseZePeerSession extracts established/dropped counters from Ze's
// `show bgp peer <peer> detail` JSON answer, requiring state "established".
func parseZePeerSession(output, peer string) (zePeerSessionCounters, error) {
	var document struct {
		Peers map[string]map[string]any `json:"peers"`
	}
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return zePeerSessionCounters{}, fmt.Errorf("decode ze peer detail JSON: %w", err)
	}
	row, ok := document.Peers[peer]
	if !ok {
		return zePeerSessionCounters{}, fmt.Errorf("ze peer detail omits peer %s", peer)
	}
	if row[fieldState] != peerStateEstablished {
		return zePeerSessionCounters{}, fmt.Errorf("ze peer %s state is %v, want %s", peer, row[fieldState], peerStateEstablished)
	}
	established, estOK := number(row[zeFieldConnectionsEstablished])
	if !estOK {
		return zePeerSessionCounters{}, fmt.Errorf("ze peer %s omits %s", peer, zeFieldConnectionsEstablished)
	}
	if established < 1 {
		return zePeerSessionCounters{}, fmt.Errorf("ze peer %s has %s=%d, want >= 1", peer, zeFieldConnectionsEstablished, established)
	}
	dropped, dropOK := number(row[zeFieldConnectionsDropped])
	if !dropOK {
		return zePeerSessionCounters{}, fmt.Errorf("ze peer %s omits %s", peer, zeFieldConnectionsDropped)
	}
	if dropped < 0 {
		return zePeerSessionCounters{}, fmt.Errorf("ze peer %s has negative %s=%d", peer, zeFieldConnectionsDropped, dropped)
	}
	return zePeerSessionCounters{Established: established, Dropped: dropped}, nil
}

func sortedAddrs(addrs ...netip.Addr) []netip.Addr {
	return sortedUniqueAddrs(addrs)
}

func sortedUniqueAddrs(addrs []netip.Addr) []netip.Addr {
	out := slices.Clone(addrs)
	slices.SortFunc(out, func(a, b netip.Addr) int { return a.Compare(b) })
	return slices.Compact(out)
}
