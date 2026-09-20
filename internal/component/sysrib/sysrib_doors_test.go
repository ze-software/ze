// VALIDATES: the three doors onto one install decision answer the same for a
// prefix whose gateway the resolver cannot reach -- a route arriving live
// (recomputeBest), the permission sweep (fibStateChange) and a cascade
// (cascadeRecompute). Each leaves the prefix programmed over the target its
// producer named, because the Loc-RIB is not the router's whole picture of
// reachability: an OSPF or IS-IS next-hop on a link whose connected route no
// plugin inserted is on-link all the same (test/ospf/ospf-route-install.ci).
// PREVENTS: plan/journal/guard-added-to-one-half-of-a-pair.md, both rows. The
// cascade read the same verdict as a path LOST and withdrew a prefix the live
// path had deliberately programmed. The withdraw was PERMANENT: an identical
// re-announcement is a no-op at recomputeBest, and a cascade fires only when a
// covering route changes, so a gateway the Loc-RIB never covers leaves the
// prefix out of the FIB for the life of the process.

package sysrib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// unresolvableGateway is a gateway no test here gives a covering route, so
// every door meets the one state they used to answer differently for.
var unresolvableGateway = netip.MustParseAddr("192.0.2.1")

// TestEveryDoorProgramsAnUnresolvableGatewayTheSameWay drives one state
// through the three doors and compares what the FIB is left holding. The
// doors run on their own system RIB each, because the question is what one
// door does with the state rather than what three of them do in sequence.
func TestEveryDoorProgramsAnUnresolvableGatewayTheSameWay(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	wireLocRIB(t)
	pfx := netip.MustParsePrefix("10.70.0.0/24")
	key := prefixKey{family: family.IPv4Unicast, prefix: pfx}

	live := liveDoor(t, pfx)
	sweep := sweepDoor(t, pfx)
	cascade := cascadeDoor(t, pfx)

	for _, door := range []struct {
		name   string
		sysrib *sysRIB
	}{{"the live change", live}, {"the permission sweep", sweep}, {"the cascade", cascade}} {
		if !door.sysrib.programmedByZe(key) {
			t.Errorf("%s holds no install for %s over %s, and the other doors do: the Loc-RIB "+
				"not covering a gateway is the ordinary state of an OSPF or IS-IS next-hop",
				door.name, pfx, unresolvableGateway)
			continue
		}
		if door.sysrib.installed[key].nextHop != unresolvableGateway {
			t.Errorf("%s programmed %s over %s, want the target its producer named, %s",
				door.name, pfx, door.sysrib.installed[key].nextHop, unresolvableGateway)
		}
	}
}

// liveDoor is recomputeBest: the route arrives from its producer.
func liveDoor(t *testing.T, pfx netip.Prefix) *sysRIB {
	t.Helper()
	bus := newTestEventBus()
	s := configuredSysRIB(t, bus)
	addPath(s, "bgp", pfx, unresolvableGateway, "eth0", 20)

	changes := publishedChanges(t, bus)
	if len(changes) != 1 || changes[0].Action != routeaction.Add {
		t.Fatalf("the live change published %+v, want one add for %s", changes, pfx)
	}
	return s
}

// sweepDoor is fibStateChange: the operator permits the protocol again, which
// is a fresh install decision and no news about any path.
func sweepDoor(t *testing.T, pfx netip.Prefix) *sysRIB {
	t.Helper()
	bus := newTestEventBus()
	s := configuredSysRIB(t, bus, "bgp")
	addPath(s, "bgp", pfx, unresolvableGateway, "eth0", 20)

	if withheld := publishedChanges(t, bus); len(withheld) != 0 {
		t.Fatalf("the withheld winner published %+v, want nothing", withheld)
	}

	reconfigure(t, s)

	permitted := publishedChanges(t, bus)
	if len(permitted) != 1 || permitted[0].Action != routeaction.Add {
		t.Fatalf("the permission sweep published %+v, want one add for %s", permitted, pfx)
	}
	return s
}

// cascadeDoor is cascadeRecompute: a covering route moved and the prefix is
// re-resolved. The gateway resolves to nothing before and after, so the
// cascade is news about some other prefix and this one owes nothing.
func cascadeDoor(t *testing.T, pfx netip.Prefix) *sysRIB {
	t.Helper()
	bus := newTestEventBus()
	s := configuredSysRIB(t, bus)
	addPath(s, "bgp", pfx, unresolvableGateway, "eth0", 20)

	s.processCascade([]netip.Addr{unresolvableGateway})

	cascaded := publishedChanges(t, bus)[1:]
	if len(cascaded) != 0 {
		t.Errorf("the cascade published %+v, want nothing: %s resolved to nothing when the "+
			"prefix was programmed and it still does, so no path was lost",
			cascaded, unresolvableGateway)
	}
	return s
}

// TestCascadeWithdrawsAPathItLaterProved is the transition the install record
// exists for. A prefix programmed while the resolver proved nothing is left
// alone by a cascade, because nothing about it changed. Once a covering route
// makes its gateway resolvable the install IS proved, and losing that cover
// loses the path, so the same prefix is withdrawn.
//
// The two states are the same ADDRESS throughout: a gateway inside a connected
// covering route resolves to itself, so the entry the cascade computes is
// identical and nothing is published when the cover appears. What the cascade
// reads is the proof, which is recorded whether or not an entry is emitted.
func TestCascadeWithdrawsAPathItLaterProved(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	connectedID := redistevents.RegisterProtocol("connected")
	loc := wireLocRIB(t)

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus)

	pfx := netip.MustParsePrefix("10.71.0.0/24")
	addPath(s, "bgp", pfx, unresolvableGateway, "eth0", 20)

	covering := netip.MustParsePrefix("192.0.2.0/24")
	loc.Insert(family.IPv4Unicast, covering, locrib.Path{Source: connectedID})
	s.processCascade([]netip.Addr{unresolvableGateway})

	if proved := publishedChanges(t, bus)[1:]; len(proved) != 0 {
		t.Fatalf("setup: the cascade published %+v when %s became resolvable, want nothing: the "+
			"entry is the one the FIB already holds", proved, unresolvableGateway)
	}

	loc.Remove(family.IPv4Unicast, covering, connectedID, 0)
	s.processCascade([]netip.Addr{unresolvableGateway})

	lost := publishedChanges(t, bus)[1:]
	if len(lost) != 1 || lost[0].Action != routeaction.Withdraw || lost[0].Prefix != pfx {
		t.Errorf("the cascade published %+v, want a withdraw of %s: %s was covered when the "+
			"cascade last ran and is not now, so the prefix lost the path it held",
			lost, pfx, unresolvableGateway)
	}
}
