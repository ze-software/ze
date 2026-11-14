// Design: docs/architecture/rib/unified-locrib.md -- publication order.
// Related: forwarding_order.go -- orderForwarding, forwardingGateways, forwardingReady.
// Related: forwarding_order_test.go -- the order helpers these tests share.

package sysrib

import (
	"net/netip"
	"testing"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/family"
)

// Goal: an ECMP member marked on-link is reachable on its interface without
// any route, so it never makes its route wait for a device route. Method: one
// publication [on-link-member route, gateway-member route, device route] whose
// two member gateways both lie in the device route. The gateway member is the
// control: it must move behind the device route, which proves the topological
// order ran, while the on-link member's route keeps its first place.
func TestOrderForwardingOnLinkMemberIsNoDependency(t *testing.T) {
	onLink := orderGateway("198.18.60.0/24", "203.0.113.60")
	onLink.ECMPPaths = []sysribevents.ECMPPath{{NextHop: netip.MustParseAddr("192.0.2.60"), Interface: "port1", OnLink: true}}
	gatewayed := orderGateway("198.18.61.0/24", "203.0.113.61")
	gatewayed.ECMPPaths = []sysribevents.ECMPPath{{NextHop: netip.MustParseAddr("192.0.2.61"), Interface: "port1"}}
	batches, err := orderForwarding(map[family.Family][]outgoingChange{family.IPv4Unicast: {
		onLink,
		gatewayed,
		orderDevice("192.0.2.0/24", "port1"),
	}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	requireOrder(t, batches, "198.18.60.0/24", "192.0.2.0/24", "198.18.61.0/24")
}

// Goal: once a dependent is released behind its cover, the routes still
// waiting keep their publication order: the earliest original position goes
// first. Method: [dependent, device, independent, independent]. The device
// route pops first and releases the dependent at position 0 into a ready heap
// that still holds positions 2 and 3, so the heap must sift the lower position
// up for the dependent to come out next.
func TestOrderForwardingReleasedDependentPrecedesLaterRoutes(t *testing.T) {
	batches, err := orderForwarding(map[family.Family][]outgoingChange{family.IPv4Unicast: {
		orderGateway("198.18.70.0/24", "192.0.2.70"),
		orderDevice("192.0.2.0/24", "port1"),
		orderGateway("198.18.71.0/24", "203.0.113.71"),
		orderGateway("198.18.72.0/24", "203.0.113.72"),
	}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	requireOrder(t, batches, "192.0.2.0/24", "198.18.70.0/24", "198.18.71.0/24", "198.18.72.0/24")
}

// Goal: two families with one AFI go out in SAFI order on every run, so the
// FIB writer sees one batch order whatever the map iteration order. Method:
// IPv4 labeled unicast (SAFI 4, a family the BGP RIB feeds the Loc-RIB) beside
// IPv4 unicast (SAFI 1), ordered 64 times from fresh maps: Go randomizes map
// iteration, so an order the comparator does not decide differs across runs.
func TestOrderForwardingSameAFIFamiliesInSAFIOrder(t *testing.T) {
	labeled := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}
	for run := range 64 {
		batches, err := orderForwarding(map[family.Family][]outgoingChange{
			labeled:            {orderDevice("198.18.80.0/24", "port1")},
			family.IPv4Unicast: {orderDevice("198.18.81.0/24", "port1")},
		}, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(batches) != 2 {
			t.Fatalf("run %d: batches = %+v, want one per family", run, batches)
		}
		if batches[0].Family != family.IPv4Unicast || batches[1].Family != labeled {
			t.Fatalf("run %d: batch families = %v, %v, want SAFI 1 before SAFI 4", run, batches[0].Family, batches[1].Family)
		}
	}
}
