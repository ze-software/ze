// Design: docs/architecture/testing/interop.md -- real daemon neighbor-watch and FRR observations.
// Related: check_rfc2545_third_party.go -- topology, kernel identity and foreign decoder helpers.
// Related: plan/release/fixes/spec-rfc2545-readvertise-on-condition-flip.md -- the re-advertisement owner.
// RFC naming: untagged -- the missing watch-triggered re-advertisement remains a defect probe.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/network"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	rfc2545NeighborFlipScenario = "bgp-rfc2545-neighbor-condition-flip-frr"
	neighborFlipUniqueControl   = "2001:db8:5640::/48"
	neighborFlipAmbigControl    = "2001:db8:5641::/48"
	neighborFlipSubject         = "2001:db8:5642::/48"
	neighborFlipFreshControl    = "2001:db8:5643::/48"
	neighborFlipDevice          = "eth0"
	neighborFlipIPNeighbor      = "neigh"
	neighborFlipFRRRoutes       = "show ipv6 route bgp json"
)

// checkRFC2545NeighborConditionFlip sends the subject exactly once, while two
// real link-local neighbors make its third-party identity ambiguous. Removing
// the extra identity must update that retained route through the real watch,
// without another subject command or a session reset. Distinct setup controls
// may be repeated; only FRR queries run during the decisive transition wait.
func checkRFC2545NeighborConditionFlip(ctx context.Context, check *interoplab.CheckContext) error {
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, rfc2545NeighborFlipScenario, assertion, cause)
	}
	if !check.Network.IPv6.IsValid() {
		return fail(1, errors.New("the neighbor-flip scenario has no selected IPv6 network"))
	}
	zeAddress := networkHostAddress6(check.Network, zeHostNumber)
	session := operation{kind: opFRRSession, argument: zeAddress}
	if err := runOperation(ctx, check.Network, check.Lab, &session); err != nil {
		return fail(1, err)
	}
	generation, err := queryFRRSessionGeneration(ctx, check.Lab, zeAddress)
	if err != nil {
		return fail(1, err)
	}
	global := netip.MustParseAddr(networkHostAddress6(check.Network, extendedSinkHost))
	linkLocal, mac, err := thirdPartyIdentity(ctx, check.Lab, neighborFlipDevice)
	if err != nil {
		return fail(2, err)
	}
	if _, err := untilDecoded(ctx, func() (struct{}, error) {
		return struct{}{}, resolveThirdParty(ctx, check.Lab, neighborFlipDevice, global, linkLocal, mac)
	}); err != nil {
		return fail(2, err)
	}
	if err := neighborFlipControl(ctx, check.Lab, neighborFlipUniqueControl, global, linkLocal); err != nil {
		return fail(3, fmt.Errorf("unique-neighbor setup control: %w", err))
	}

	// This is a real second address on the same router, not a fictitious MAC
	// binding. Its successful ping must populate Ze's actual neighbor table.
	secondary := netip.MustParseAddr("fe80::2545:5642")
	if secondary == linkLocal {
		return fail(4, errors.New("the secondary link-local equals the router's original address"))
	}
	if err := neighborFlipExec(ctx, check.Lab, peerFRRSink,
		[]string{"ip", "-6", ipObjectAddr, ipActionAdd, secondary.String() + "/64", ipOptionDevice, neighborFlipDevice, "nodad"}); err != nil {
		return fail(4, err)
	}
	if _, err := check.Lab.Query(ctx, "ze", []string{cmdPing, "-c", "1", "-W", "2", "-I", neighborFlipDevice, secondary.String()}, nil); err != nil {
		return fail(4, err)
	}
	table, err := neighborFlipTable(ctx, check.Lab)
	if err != nil {
		return fail(4, err)
	}
	if err := requireNeighborFlipOutcome(table, global, linkLocal, mac, network.LinkLocalAmbiguous); err != nil {
		return fail(4, fmt.Errorf("the actual ND replies did not establish an ambiguous router identity: %w", err))
	}
	// Pin only mappings already established by the real ND responses above.
	// This prevents reachability aging from changing a test-controlled phase.
	for _, address := range []netip.Addr{global, linkLocal, secondary} {
		if err := neighborFlipExec(ctx, check.Lab, "ze", []string{"ip", "-6", neighborFlipIPNeighbor, "replace", address.String(),
			"lladdr", mac.String(), "nud", "permanent", ipOptionDevice, neighborFlipDevice}); err != nil {
			return fail(4, err)
		}
	}
	table, err = neighborFlipTable(ctx, check.Lab)
	if err != nil {
		return fail(4, err)
	}
	if err := requireNeighborFlipOutcome(table, global, linkLocal, mac, network.LinkLocalAmbiguous); err != nil {
		return fail(4, err)
	}
	if err := neighborFlipControl(ctx, check.Lab, neighborFlipAmbigControl, global, netip.Addr{}); err != nil {
		return fail(5, fmt.Errorf("ambiguous-neighbor setup control: %w", err))
	}

	// There is ONE subject send. Every later observation of it is read-only.
	if err := announceNeighborFlipRoute(ctx, check.Lab, neighborFlipSubject, global); err != nil {
		return fail(6, err)
	}
	if err := waitNeighborFlipRoute(ctx, check.Lab, neighborFlipSubject, global, netip.Addr{}); err != nil {
		return fail(6, fmt.Errorf("initial global-only subject: %w", err))
	}
	if _, err := untilDecoded(ctx, func() (struct{}, error) {
		command := zeCommand("show bgp rib | sent")
		answer, err := check.Lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
		if err != nil {
			return struct{}{}, err
		}
		return struct{}{}, requireNeighborFlipRetained(answer,
			networkHostAddress6(check.Network, frrHostNumber), global)
	}); err != nil {
		return fail(7, fmt.Errorf("the daemon retained no sent subject for re-advertisement: %w", err))
	}

	if err := neighborFlipExec(ctx, check.Lab, peerFRRSink,
		[]string{"ip", "-6", ipObjectAddr, ipRouteDelete, secondary.String() + "/64", ipOptionDevice, neighborFlipDevice}); err != nil {
		return fail(8, err)
	}
	if err := neighborFlipExec(ctx, check.Lab, "ze",
		[]string{"ip", "-6", neighborFlipIPNeighbor, ipRouteDelete, secondary.String(), ipOptionDevice, neighborFlipDevice}); err != nil {
		return fail(8, err)
	}
	table, err = neighborFlipTable(ctx, check.Lab)
	if err != nil {
		return fail(8, err)
	}
	if err := requireNeighborPair(table, global, linkLocal, mac); err != nil {
		return fail(8, err)
	}

	// No Ze command, route injection, reload or callback call occurs while
	// waiting for this transition. A new API send must not mask missing replay.
	transitionErr := waitNeighborFlipRoute(ctx, check.Lab, neighborFlipSubject, global, linkLocal)
	// Run this independent control even after a transition red. Its pair proves
	// the daemon's current watched facts can construct the new form; it cannot
	// count as the retained subject's re-advertisement.
	if err := neighborFlipControl(ctx, check.Lab, neighborFlipFreshControl, global, linkLocal); err != nil {
		return fail(10, fmt.Errorf("fresh post-change control did not acquire the current neighbor identity: %w", err))
	}
	after, err := queryFRRSessionGeneration(ctx, check.Lab, zeAddress)
	if err != nil {
		return fail(11, err)
	}
	if err := requireSameBGPGeneration("the neighbor condition change", generation, after); err != nil {
		return fail(11, err)
	}
	if transitionErr != nil {
		return fail(9, fmt.Errorf("the retained subject stayed global-only after the neighbor became unique, while the fresh control acquired its pair: %w", transitionErr))
	}
	if _, err := untilDecoded(ctx, func() (struct{}, error) {
		routes, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", neighborFlipFRRRoutes}, nil)
		if err != nil {
			return struct{}{}, err
		}
		return struct{}{}, requireRouteInstalledVia(routes, neighborFlipSubject, linkLocal.String(), "bgp")
	}); err != nil {
		return fail(12, err)
	}
	return nil
}

// neighborFlipExec measures mutation by command completion and exit status.
// Successful ip mutations are silent; the following kernel queries separately
// verify their effect before any route observation can claim a condition flip.
func neighborFlipExec(ctx context.Context, lab interoplab.CheckerLab, peer string, command []string) error {
	result, err := lab.Exec(ctx, peer, command, nil)
	if err != nil {
		return fmt.Errorf("neighbor mutation on %s: %w", peer, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("neighbor mutation on %s exited %d: stdout=%q stderr=%q", peer, result.ExitCode, result.Stdout, result.Stderr)
	}
	return nil
}

func announceNeighborFlipRoute(ctx context.Context, lab interoplab.CheckerLab, prefix string, global netip.Addr) error {
	var command textbuf.Buffer
	argv := zeCommand(command.Str("send bgp * update text origin igp nhop ").Addr(global).
		Str(" mp ipv6/unicast add ").Str(prefix).String())
	_, err := lab.Query(ctx, "ze", argv, queryEnvironment("ze", argv))
	return err
}

// neighborFlipControl repeats only a distinct control prefix while the kernel
// watch publishes its facts. It is never called with neighborFlipSubject.
func neighborFlipControl(ctx context.Context, lab interoplab.CheckerLab, prefix string, global, linkLocal netip.Addr) error {
	if prefix == neighborFlipSubject {
		return errors.New("a repeated setup control cannot use the retained subject prefix")
	}
	_, err := untilDecoded(ctx, func() (struct{}, error) {
		if err := announceNeighborFlipRoute(ctx, lab, prefix, global); err != nil {
			return struct{}{}, err
		}
		answer, err := lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show bgp ipv6 unicast " + prefix + " json"}, nil)
		if err != nil {
			return struct{}{}, err
		}
		return struct{}{}, requireNeighborFlipRoute(answer, prefix, global, linkLocal)
	})
	return err
}

// waitNeighborFlipRoute is observation only. Its polling loop cannot send a
// route to Ze, so a passing transition needs an actual unsolicited UPDATE.
func waitNeighborFlipRoute(ctx context.Context, lab interoplab.CheckerLab, prefix string, global, linkLocal netip.Addr) error {
	answer, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 30 * time.Second, Interval: time.Second,
		Description: "FRR's retained next-hop transition for " + prefix,
	}, func(probe context.Context) (string, error) {
		return lab.Query(probe, peerFRR, []string{cmdVtysh, "-c", "show bgp ipv6 unicast " + prefix + " json"}, nil)
	}, func(output string) bool { return requireNeighborFlipRoute(output, prefix, global, linkLocal) == nil })
	if err != nil {
		return neighborFlipWaitFailure(err, requireNeighborFlipRoute(answer, prefix, global, linkLocal))
	}
	return nil
}

// neighborFlipWaitFailure preserves the failed wait and its last nonnil route
// verdict. If final validation supplies no additional error, leave the wait
// failure intact rather than manufacturing a diagnostic that wraps nil.
func neighborFlipWaitFailure(cause, verdict error) error {
	if verdict == nil {
		return cause
	}
	return fmt.Errorf("%w; last FRR route verdict: %w", cause, verdict)
}

func neighborFlipTable(ctx context.Context, lab interoplab.CheckerLab) (string, error) {
	return lab.Query(ctx, "ze", []string{"ip", "-j", "-6", neighborFlipIPNeighbor, ipActionShow, ipOptionDevice, neighborFlipDevice}, nil)
}

// requireNeighborFlipOutcome uses the same state classifier and pairing
// producer as the real interface reader. It asserts the kernel input to the
// watch independently of the foreign peer's BGP observation.
func requireNeighborFlipOutcome(output string, global, linkLocal netip.Addr, mac net.HardwareAddr, want network.LinkLocalOutcome) error {
	var entries []ipNeighbor
	if err := json.Unmarshal([]byte(output), &entries); err != nil {
		return fmt.Errorf("decode the actual neighbor table: %w", err)
	}
	neighbors := make([]network.Neighbor, 0, len(entries))
	globalBound := false
	for _, entry := range entries {
		address, err := netip.ParseAddr(entry.Destination)
		if err != nil {
			return err
		}
		if entry.LinkLayer != "" {
			hardware, err := net.ParseMAC(entry.LinkLayer)
			if err != nil {
				return err
			}
			entry.LinkLayer = hardware.String()
		}
		binding := network.NeighborBindingInvalid
		for _, state := range entry.State {
			if iface.NeighborStateResolved(strings.ToLower(state)) {
				binding = network.NeighborBindingValid
			}
		}
		if address == global {
			if binding != network.NeighborBindingValid {
				return errors.New("the subject's global neighbor has no usable binding")
			}
			if entry.LinkLayer != mac.String() {
				return fmt.Errorf("the global neighbor names %s, want the actual router's MAC %s", entry.LinkLayer, mac)
			}
			globalBound = true
		}
		neighbors = append(neighbors, network.Neighbor{Address: address, LinkAddress: entry.LinkLayer,
			Device: neighborFlipDevice, Binding: binding})
	}
	if !globalBound {
		return errors.New("the actual neighbor table has no bound global next hop")
	}
	found, outcome := network.NewNeighborLinkLocals(neighbors).LinkLocalOf(global)
	if outcome != want {
		return fmt.Errorf("the actual kernel neighbor pairing has outcome %d, want %d", outcome, want)
	}
	if want == network.LinkLocalFound {
		if found != linkLocal {
			return fmt.Errorf("the actual neighbor pairing names %s, want %s", found, linkLocal)
		}
	}
	return nil
}

func requireNeighborFlipRoute(output, prefix string, global, linkLocal netip.Addr) error {
	var answer struct {
		Prefix string `json:"prefix"`
	}
	if err := json.Unmarshal([]byte(output), &answer); err != nil {
		return err
	}
	if answer.Prefix != prefix {
		return fmt.Errorf("FRR answered route %q, want %s", answer.Prefix, prefix)
	}
	hops, err := parseFRRNextHops(output)
	if err != nil {
		return err
	}
	return requireNextHopShape(hops, global, linkLocal)
}

func requireNeighborFlipRetained(output, peer string, global netip.Addr) error {
	// zeCommand's --format json lowers to | json. The line carries | sent, so
	// the CLI prints the routes envelope beside the chain that shaped it
	// (decodeZePipedRows), and not the bare array a chainless line unwraps to.
	var rows []struct {
		Peer      string `json:"peer"`
		Direction string `json:"direction"`
		Family    string `json:"family"`
		Prefix    string `json:"prefix"`
		NextHop   string `json:"next-hop"`
	}
	if err := decodeZePipedRows(output, zeRIBRowsKey, zePipeSent, &rows); err != nil {
		return err
	}
	count := 0
	for _, route := range rows {
		if route.Peer != peer {
			continue
		}
		if route.Direction != rpc.DirectionSent.String() {
			continue
		}
		if route.Family != "ipv6/unicast" {
			continue
		}
		if route.Prefix != neighborFlipSubject {
			continue
		}
		nextHop, err := netip.ParseAddr(route.NextHop)
		if err != nil {
			return err
		}
		if nextHop != global {
			return fmt.Errorf("the retained subject next hop is %s, want %s", nextHop, global)
		}
		count++
	}
	if count != 1 {
		return fmt.Errorf("the daemon retains %d sent subject entries for FRR, want exactly 1", count)
	}
	return nil
}
