// Design: docs/architecture/testing/interop.md -- the route server keeps a non-ADD-PATH client on the elected route.
// Related: check_special.go -- registry that dispatches this checker.
// Related: check_rfc_predicate.go -- requireBIRDASPath, the one-path AS_PATH reading this checker relies on.
// RFC: rfc/short/rfc4271.md -- Section 9.2, a withdrawn best is replaced by the remaining feasible route.
// RFC: rfc/short/rfc7947.md -- Section 2.3.2, a non-ADD-PATH client holds the route server's elected route.
package bgp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	routeServerReplacementScenario = "bgp-route-server-replacement-frr"
	// routeServerReplacementPrefix is the one prefix both sources announce.
	routeServerReplacementPrefix = "10.98.0.0/24"
	// routeServerReplacementFRRPath is FRR's route as BIRD decodes it: Ze runs
	// rs-client toward BIRD, so nothing is prepended and the path is the source's
	// own AS. One AS, so it is the better route by AS_PATH length.
	routeServerReplacementFRRPath = "65002"
	// routeServerReplacementGoBGPPath is GoBGP's route as BIRD decodes it: the
	// checker injects AS_PATH 65004 and GoBGP prepends its own AS on the eBGP
	// session, so the path is one AS longer than FRR's and loses the election.
	routeServerReplacementGoBGPPath = "65004 65004"
	// routeServerReplacementFirstCommunity and routeServerReplacementSecondCommunity
	// make GoBGP's re-announcement a different revision of the same route, so Ze
	// receives a later UPDATE for a prefix whose owner at BIRD is FRR.
	routeServerReplacementFirstCommunity  = "65004:1"
	routeServerReplacementSecondCommunity = "65004:2"
	// routeServerReplacementWindow is how long an ownership that must NOT change
	// is watched. The forward a broken route server sends leaves at once, so a
	// window measured in seconds is ample, and BIRD is read every second inside it.
	routeServerReplacementWindow  = 20 * time.Second
	routeServerReplacementTimeout = 90 * time.Second
	routeServerReplacementPoll    = time.Second
	// birdADDPathCapability is the line BIRD prints under a protocol's local or
	// neighbor capabilities when ADD-PATH was advertised (bgp_show_capabilities).
	birdADDPathCapability        = "ADD-PATH"
	birdNeighborCapabilitiesHead = "Neighbor capabilities"
)

// checkRouteServerReplacement proves that a route server keeps a client that
// negotiated no ADD-PATH on the RIB-elected route, on announcement and on
// withdrawal (spec-bgp-rs-replacement-on-withdrawal, AC-4). Two sources announce
// one prefix and BIRD, holding one path per prefix, is the foreign witness:
//
//   - GoBGP announces it with one prepended AS, and BIRD holds GoBGP's path.
//   - FRR announces it, a better and later route, and BIRD moves to FRR's path.
//   - GoBGP re-announces it, still worse, with a changed community. The later
//     route must not displace the better one, so BIRD keeps FRR's path through a
//     stability window. A forward-all route server sends GoBGP's revision and
//     BIRD replaces FRR's path with it.
//   - FRR withdraws it. BIRD must move to GoBGP's path and never lose the prefix.
//     A route server that relays the owner's withdrawal bare leaves BIRD with
//     nothing, though GoBGP's route is still feasible.
//
// RFC 4271 Section 9.2 and RFC 7947 Section 2.3.2 are the requirements this
// exercises through a foreign implementation; the tagged carriers live in the
// reactor's RFC test files.
func checkRouteServerReplacement(ctx context.Context, check *interoplab.CheckContext) error {
	const name = routeServerReplacementScenario
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, name, assertion, cause)
	}
	if !check.Network.IPv4.IsValid() {
		return fail(1, errors.New("route server replacement scenario has no selected IPv4 network"))
	}
	zeAddress := networkHostAddress(check.Network, 2)
	frrSession := operation{kind: opFRRSession, argument: zeAddress}
	gobgpSession := operation{kind: opGoBGPSession, argument: zeAddress}
	birdSession := operation{kind: opBIRDSession, argument: birdZeProtocol}

	// Assertions 2 to 4. Every session is up before any route moves, so each
	// later reading is about the route and never about a session still opening.
	if err := runOperation(ctx, check.Network, check.Lab, &frrSession); err != nil {
		return fail(2, err)
	}
	if err := runOperation(ctx, check.Network, check.Lab, &gobgpSession); err != nil {
		return fail(3, err)
	}
	if err := runOperation(ctx, check.Network, check.Lab, &birdSession); err != nil {
		return fail(4, err)
	}

	// Assertion 5. The client negotiated no ADD-PATH, read from BIRD itself. With
	// ADD-PATH BIRD would hold both paths and every ownership reading below would
	// be about BIRD's own selection rather than Ze's.
	protocol, err := check.Lab.Query(ctx, peerBIRD, []string{cmdBirdc, birdShowZeProtocolAll}, nil)
	if err != nil {
		return fail(5, err)
	}
	if err := requireBIRDNoAddPath(protocol); err != nil {
		return fail(5, err)
	}

	// Assertions 6 and 7. GoBGP is the first and only source, so BIRD holds its
	// path. This is also what makes the later readings discriminate: BIRD can only
	// show FRR's path because Ze replaced GoBGP's.
	gobgpNext := networkHostAddress(check.Network, 5)
	if err := routeServerReplacementGoBGPAnnounce(ctx, check.Lab, gobgpNext, routeServerReplacementFirstCommunity); err != nil {
		return fail(6, err)
	}
	if err := waitBIRDASPath(ctx, check.Lab, routeServerReplacementGoBGPPath); err != nil {
		return fail(7, err)
	}

	// Assertions 8 and 9. FRR's better, later route displaces GoBGP's at BIRD.
	if err := routeServerReplacementFRRNetwork(ctx, check.Lab, "network "+routeServerReplacementPrefix); err != nil {
		return fail(8, err)
	}
	if err := waitBIRDASPath(ctx, check.Lab, routeServerReplacementFRRPath); err != nil {
		return fail(9, err)
	}

	// Assertions 10 and 11. GoBGP re-announces, still worse. Ze's RIB must hold
	// the new revision as received: it is the proof the re-announcement reached
	// the route server, so the window below cannot pass because GoBGP never sent
	// anything. The sources receive nothing from Ze (ze.conf says why), so no
	// foreign daemon other than BIRD can witness it.
	if err := routeServerReplacementGoBGPAnnounce(ctx, check.Lab, gobgpNext, routeServerReplacementSecondCommunity); err != nil {
		return fail(10, err)
	}
	received := zeCommand("show bgp rib received community " + routeServerReplacementSecondCommunity)
	if err := waitContains(ctx, check.Lab, "ze", received, routeServerReplacementTimeout, routeServerReplacementPrefix); err != nil {
		return fail(11, err)
	}

	// Assertion 12. The worse later route does not displace the better one: BIRD
	// keeps FRR's path for the whole window. Reverting the announcement half of the
	// route server (Data Flow 3a) fails here.
	if err := holdBIRDASPath(ctx, check.Lab, routeServerReplacementFRRPath); err != nil {
		return fail(12, err)
	}

	// Assertions 13 to 15. FRR, the owner of BIRD's route, withdraws. BIRD moves
	// to GoBGP's path without ever losing the prefix, then keeps it. Reverting the
	// withdrawal half (Data Flow 3) fails at 14: the bare withdrawal empties BIRD.
	if err := routeServerReplacementFRRNetwork(ctx, check.Lab, "no network "+routeServerReplacementPrefix); err != nil {
		return fail(13, err)
	}
	if err := waitBIRDReplacement(ctx, check.Lab); err != nil {
		return fail(14, err)
	}
	if err := holdBIRDASPath(ctx, check.Lab, routeServerReplacementGoBGPPath); err != nil {
		return fail(15, err)
	}

	// Assertions 16 to 18. Every session survived. A route server that answered
	// any step with a NOTIFICATION would leave BIRD's last reading meaningless.
	if err := runOperation(ctx, check.Network, check.Lab, &frrSession); err != nil {
		return fail(16, err)
	}
	if err := runOperation(ctx, check.Network, check.Lab, &gobgpSession); err != nil {
		return fail(17, err)
	}
	if err := runOperation(ctx, check.Network, check.Lab, &birdSession); err != nil {
		return fail(18, err)
	}
	return nil
}

// routeServerReplacementGoBGPAnnounce adds (or replaces) GoBGP's route for the
// prefix with AS_PATH 65004, to which GoBGP prepends its own AS, and community.
func routeServerReplacementGoBGPAnnounce(ctx context.Context, lab interoplab.CheckerLab, nextHop, community string) error {
	_, err := lab.Exec(ctx, peerGoBGP, []string{
		cmdGoBGP, gobgpGlobal, gobgpRIB, gobgpAdd, routeServerReplacementPrefix, "-a", gobgpFamilyIPv4,
		gobgpNextHop, nextHop, "aspath", "65004", "community", community,
	}, nil)
	return err
}

// routeServerReplacementFRRNetwork runs one `network` statement under FRR's BGP
// instance, which announces (or with `no`, withdraws) the prefix.
func routeServerReplacementFRRNetwork(ctx context.Context, lab interoplab.CheckerLab, statement string) error {
	_, err := lab.Exec(ctx, peerFRR, []string{
		cmdVtysh, "-c", frrConfigureTerminal, "-c", policyModifyMPRouter,
		"-c", frrAddressFamilyIPv4Unicast, "-c", statement,
	}, nil)
	return err
}

// birdReplacementRoute reads BIRD's one route for the prefix. BIRD answers a
// prefix it does not hold with "Network not found" and birdc exits 1, so a
// query failure here is the prefix missing at BIRD (or BIRD unreachable), and
// either way is not a reading of the route.
func birdReplacementRoute(ctx context.Context, lab interoplab.CheckerLab) (string, error) {
	return lab.Query(ctx, peerBIRD, []string{cmdBirdc, "show route for " + routeServerReplacementPrefix + " all"}, nil)
}

// waitBIRDASPath waits until BIRD holds exactly one route for the prefix and its
// AS_PATH is want.
func waitBIRDASPath(ctx context.Context, lab interoplab.CheckerLab, want string) error {
	route, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     routeServerReplacementTimeout,
		Interval:    routeServerReplacementPoll,
		Description: "BIRD route " + routeServerReplacementPrefix + " with AS_PATH " + want,
	}, func(probeCtx context.Context) (string, error) {
		return birdReplacementRoute(probeCtx, lab)
	}, func(output string) bool {
		return requireBIRDASPath(output, routeServerReplacementPrefix, want) == nil
	})
	if err != nil {
		return interoplab.WithSearchedOutput(err, route)
	}
	return nil
}

// holdBIRDASPath reads BIRD every second for the stability window and fails on
// the first reading whose route is missing or whose AS_PATH is not want.
func holdBIRDASPath(ctx context.Context, lab interoplab.CheckerLab, want string) error {
	deadline := time.Now().Add(routeServerReplacementWindow)
	for reading := 1; ; reading++ {
		route, err := birdReplacementRoute(ctx, lab)
		if err != nil {
			return fmt.Errorf("BIRD reading %d of the window holds no route for %s: %w", reading, routeServerReplacementPrefix, err)
		}
		if err := requireBIRDASPath(route, routeServerReplacementPrefix, want); err != nil {
			return interoplab.WithSearchedOutput(fmt.Errorf("BIRD reading %d of the window: %w", reading, err), route)
		}
		if !time.Now().Before(deadline) {
			return nil
		}
		if err := routeServerReplacementSleep(ctx); err != nil {
			return err
		}
	}
}

// waitBIRDReplacement watches BIRD after FRR's withdrawal. Until the withdrawal
// is processed BIRD still shows FRR's path; the first reading with GoBGP's path
// ends the wait. Any reading with no route at all fails at once, because a
// client that loses the prefix while another source still offers it is the
// defect this scenario exists for.
func waitBIRDReplacement(ctx context.Context, lab interoplab.CheckerLab) error {
	deadline := time.Now().Add(routeServerReplacementTimeout)
	for reading := 1; ; reading++ {
		route, err := birdReplacementRoute(ctx, lab)
		if err != nil {
			return fmt.Errorf("BIRD lost %s at reading %d after FRR withdrew it, though GoBGP still announces it: %w", routeServerReplacementPrefix, reading, err)
		}
		replaced := requireBIRDASPath(route, routeServerReplacementPrefix, routeServerReplacementGoBGPPath)
		if replaced == nil {
			return nil
		}
		if err := requireBIRDASPath(route, routeServerReplacementPrefix, routeServerReplacementFRRPath); err != nil {
			return interoplab.WithSearchedOutput(fmt.Errorf("BIRD reading %d after FRR's withdrawal: %w", reading, replaced), route)
		}
		if !time.Now().Before(deadline) {
			return interoplab.WithSearchedOutput(fmt.Errorf("BIRD still holds FRR's withdrawn path after %s", routeServerReplacementTimeout), route)
		}
		if err := routeServerReplacementSleep(ctx); err != nil {
			return err
		}
	}
}

func routeServerReplacementSleep(ctx context.Context) error {
	timer := time.NewTimer(routeServerReplacementPoll)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// requireBIRDNoAddPath judges `birdc show protocols all ze_peer`: the session is
// up, BIRD printed its neighbor capabilities, and no ADD-PATH line appears in
// either direction. The two positive needles stop an empty or truncated answer
// from passing as absence.
func requireBIRDNoAddPath(output string) error {
	return requireAbsentWithProof(output, []string{birdADDPathCapability}, []string{stateEstablished, birdNeighborCapabilitiesHead})
}
