// Design: docs/architecture/testing/interop.md -- a route server acts on, then removes, its large control communities.
// Related: check_special.go -- registry that dispatches this checker.
// Related: check_rfc_predicate.go -- requireBIRDASPath, the one-path AS_PATH reading this checker relies on.
// RFC: rfc/short/rfc7947.md -- Section 2.2.2.1, the prepend is applied only on the client's request.
package bgp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	routeServerControlScenario = "bgp-route-server-control-community-frr"
	// routeServerControlPrependPrefix carries 65001:101:65003 in frr.conf: the
	// route server (AS 65001) is asked to prepend its AS once toward BIRD (AS 65003).
	routeServerControlPrependPrefix = "10.99.0.0/24"
	// routeServerControlOtherPrefix carries 65001:101:65099, a prepend request
	// naming a client that is not BIRD, so BIRD owes the path FRR sent.
	routeServerControlOtherPrefix = "10.98.0.0/24"
	// routeServerControlPrependedPath is the path BIRD decodes for the requested
	// route: the route server's AS once, then the originating client's.
	routeServerControlPrependedPath = "65001 65002"
	// routeServerControlClientPath is the path of the route that asked nothing of
	// BIRD's session: the originating client's AS alone, as on the plain relay.
	routeServerControlClientPath = "65002"
	// routeServerControlForeignLarge is the value in FRR's own namespace both
	// routes carry. It MUST reach BIRD: the route server removes only values whose
	// Global Administrator is its own AS.
	routeServerControlForeignLarge = "(65002, 1, 1)"
	// routeServerControlOwnGA opens every large community in the route server's
	// namespace as BIRD prints it. None may reach BIRD.
	routeServerControlOwnGA = "(65001,"
)

// checkRouteServerControlCommunity proves, at BIRD, that Ze's route server acts
// on the large prepend request a client sends and removes it before the route
// leaves. The prepended path and the absent control value are both read from
// what BIRD decoded off the wire Ze emitted. The second prefix names another
// client, so a route server that prepended on every request, or on none, fails
// one of the two path assertions.
func checkRouteServerControlCommunity(ctx context.Context, check *interoplab.CheckContext) error {
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, routeServerControlScenario, assertion, cause)
	}
	if !check.Network.IPv4.IsValid() {
		return fail(1, errors.New("route server control scenario has no selected IPv4 network"))
	}
	zeAddress := networkHostAddress(check.Network, 2)

	// Assertions 1 to 4. Both sessions, then both relayed routes at the far side.
	relay := []operation{
		{kind: opFRRSession, argument: zeAddress},
		{kind: opBIRDSession, argument: birdZeProtocol},
		{kind: opBIRDRoute, argument: routeServerControlPrependPrefix, timeout: 60 * time.Second},
		{kind: opBIRDRoute, argument: routeServerControlOtherPrefix, timeout: 60 * time.Second},
	}
	for index := range relay {
		if err := runOperation(ctx, check.Network, check.Lab, &relay[index]); err != nil {
			return fail(index+1, err)
		}
	}

	// Assertions 5 to 8. The path and the large communities of each route.
	routes := []struct {
		prefix string
		path   string
	}{
		{routeServerControlPrependPrefix, routeServerControlPrependedPath},
		{routeServerControlOtherPrefix, routeServerControlClientPath},
	}
	for index, route := range routes {
		output, err := check.Lab.Query(ctx, peerBIRD, []string{cmdBirdc, "show route for " + route.prefix + " all"}, nil)
		if err != nil {
			return fail(5+2*index, err)
		}
		if err := requireBIRDASPath(output, route.prefix, route.path); err != nil {
			return fail(5+2*index, err)
		}
		if err := requireRouteServerControlRemoved(output, route.prefix); err != nil {
			return fail(6+2*index, err)
		}
	}

	// Assertions 9 and 10. Both sessions survived the relay.
	for index := range relay[:2] {
		if err := runOperation(ctx, check.Network, check.Lab, &relay[index]); err != nil {
			return fail(index+9, err)
		}
	}
	return nil
}

// requireRouteServerControlRemoved reports whether BIRD's route dump for prefix
// carries the client's own large community and no value in the route server's
// namespace. The foreign value is the positive half: a route server that dropped
// the whole attribute would otherwise pass as one that removed only its own.
func requireRouteServerControlRemoved(routeText, prefix string) error {
	large, ok := birdAttributeLine(routeText, birdLargeCommunityLine)
	if !ok {
		return interoplab.WithSearchedOutput(fmt.Errorf("BIRD route %s has no %s line", prefix, birdLargeCommunityLine), routeText)
	}
	if !strings.Contains(large, routeServerControlForeignLarge) {
		return fmt.Errorf("BIRD route %s lost the client's own large community %s: %s", prefix, routeServerControlForeignLarge, strings.TrimSpace(large))
	}
	if strings.Contains(large, routeServerControlOwnGA) {
		return fmt.Errorf("BIRD route %s received a large community in the route server's namespace: %s", prefix, strings.TrimSpace(large))
	}
	return nil
}

// birdLargeCommunityLine opens the LARGE_COMMUNITY line of a BIRD route dump.
const birdLargeCommunityLine = "bgp_large_community:"
