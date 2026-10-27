package bgp

import (
	"strings"
	"testing"
)

// TestRouteServerControlCommunityPredicates drives both polarities of the two
// readings checkRouteServerControlCommunity makes of BIRD's route dump, with no
// container: the AS_PATH of the requested and the unrequested route, and the
// large-community line. Each break a route server could ship (no prepend, a
// prepend on every route, the control value left in, the whole attribute
// dropped) must be refused.
func TestRouteServerControlCommunityPredicates(t *testing.T) {
	dump := func(prefix, path, large string) string {
		text := prefix + "           unicast [ze_peer 03:18:06.271] * (100) [AS65002i]\n" +
			"\tvia 172.30.0.2 on eth0\n" +
			"\tbgp_origin: IGP\n" +
			"\tbgp_path: " + path + "\n" +
			"\tbgp_next_hop: 172.30.0.2\n"
		if large != "" {
			text += "\tbgp_large_community: " + large + "\n"
		}
		return text
	}
	const foreign = "(65002, 1, 1)"

	t.Run("requested route", func(t *testing.T) {
		prefix := routeServerControlPrependPrefix
		good := dump(prefix, routeServerControlPrependedPath, foreign)
		if err := requireBIRDASPath(good, prefix, routeServerControlPrependedPath); err != nil {
			t.Fatalf("the prepended path was refused: %v", err)
		}
		if err := requireRouteServerControlRemoved(good, prefix); err != nil {
			t.Fatalf("a route carrying only the client's value was refused: %v", err)
		}
		unprepended := dump(prefix, routeServerControlClientPath, foreign)
		if requireBIRDASPath(unprepended, prefix, routeServerControlPrependedPath) == nil {
			t.Fatal("a route server that ignored the prepend request passed")
		}
		twice := dump(prefix, "65001 65001 65002", foreign)
		if requireBIRDASPath(twice, prefix, routeServerControlPrependedPath) == nil {
			t.Fatal("a route server that prepended twice for function 101 passed")
		}
	})

	t.Run("unrequested route", func(t *testing.T) {
		prefix := routeServerControlOtherPrefix
		if err := requireBIRDASPath(dump(prefix, routeServerControlClientPath, foreign), prefix, routeServerControlClientPath); err != nil {
			t.Fatalf("the client's own path was refused: %v", err)
		}
		if requireBIRDASPath(dump(prefix, routeServerControlPrependedPath, foreign), prefix, routeServerControlClientPath) == nil {
			t.Fatal("a route server that prepended toward a client the request did not name passed")
		}
	})

	t.Run("large communities", func(t *testing.T) {
		prefix := routeServerControlPrependPrefix
		leaked := dump(prefix, routeServerControlPrependedPath, "(65001, 101, 65003) "+foreign)
		err := requireRouteServerControlRemoved(leaked, prefix)
		if err == nil || !strings.Contains(err.Error(), "route server's namespace") {
			t.Fatalf("a route still carrying the control value passed: %v", err)
		}
		lost := dump(prefix, routeServerControlPrependedPath, "(65003, 1, 1)")
		if requireRouteServerControlRemoved(lost, prefix) == nil {
			t.Fatal("a route that lost the client's own value passed")
		}
		if requireRouteServerControlRemoved(dump(prefix, routeServerControlPrependedPath, ""), prefix) == nil {
			t.Fatal("a route with no large community at all passed")
		}
	})
}
