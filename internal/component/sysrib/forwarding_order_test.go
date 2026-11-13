// Design: docs/architecture/rib/unified-locrib.md -- publication order.
// Related: forwarding_order.go -- orderForwarding.

package sysrib

import (
	"net/netip"
	"testing"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/internal/core/rib/routetype"
)

func orderGateway(prefix, gateway string) outgoingChange {
	return outgoingChange{Action: routeaction.Add, Prefix: netip.MustParsePrefix(prefix), NextHop: netip.MustParseAddr(gateway), Interface: "port1", RouteType: routetype.Unicast}
}

func orderDevice(prefix, device string) outgoingChange {
	return outgoingChange{Action: routeaction.Add, Prefix: netip.MustParsePrefix(prefix), Interface: device, RouteType: routetype.Unicast}
}

func orderWithdraw(prefix string) outgoingChange {
	return outgoingChange{Action: routeaction.Withdraw, Prefix: netip.MustParsePrefix(prefix)}
}

// orderedPrefixes flattens the batches into one sequence of prefixes, the
// order a FIB writer applies them in.
func orderedPrefixes(t *testing.T, batches []outgoingBatch) []string {
	t.Helper()
	var prefixes []string
	for i := range batches {
		for j := range batches[i].Changes {
			prefixes = append(prefixes, batches[i].Changes[j].Prefix.String())
		}
	}
	return prefixes
}

func requireOrder(t *testing.T, batches []outgoingBatch, want ...string) {
	t.Helper()
	got := orderedPrefixes(t, batches)
	if len(got) != len(want) {
		t.Fatalf("publication order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("publication order = %v, want %v", got, want)
		}
	}
}

// Goal: a route that precedes the device route its gateway lies in comes out
// after it. Method: one IPv4 publication [dependent, cover], built explicitly
// in that order, so the input order is never a map's.
func TestOrderForwardingDependentAfterCover(t *testing.T) {
	batches, err := orderForwarding(map[family.Family][]outgoingChange{family.IPv4Unicast: {
		orderGateway("10.249.3.0/24", "10.248.0.3"),
		orderDevice("10.248.0.0/24", "zesrmetric"),
	}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	requireOrder(t, batches, "10.248.0.0/24", "10.249.3.0/24")
	if len(batches) != 1 || batches[0].Family != family.IPv4Unicast {
		t.Fatalf("one family must stay one batch: %+v", batches)
	}
}

// Goal: a chain A via B via C orders fully. Sysrib programs the resolved
// gateway, so A and B each reach the writer via a gateway inside the device
// route C, and both follow C while keeping their own order.
func TestOrderForwardingChainFollowsTerminalCover(t *testing.T) {
	batches, err := orderForwarding(map[family.Family][]outgoingChange{family.IPv4Unicast: {
		orderGateway("10.249.1.0/24", "192.0.2.7"),
		orderGateway("10.250.0.0/24", "192.0.2.7"),
		orderDevice("192.0.2.0/24", "port1"),
	}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	requireOrder(t, batches, "192.0.2.0/24", "10.249.1.0/24", "10.250.0.0/24")
}

// Goal: in a mixed page only an install that precedes its provider moves, and
// no withdrawal moves: a cover withdrawn ahead of its dependent stays ahead,
// because the kernel deletes either first (TestFIBKernelDeletesCoverBeforeDependent).
func TestOrderForwardingMixedAddAndWithdraw(t *testing.T) {
	batches, err := orderForwarding(map[family.Family][]outgoingChange{family.IPv4Unicast: {
		orderGateway("198.18.10.0/24", "198.51.100.9"),
		orderWithdraw("203.0.113.0/24"),
		orderWithdraw("198.18.20.0/24"),
		orderDevice("198.51.100.0/24", "port2"),
	}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	requireOrder(t, batches, "203.0.113.0/24", "198.18.20.0/24", "198.51.100.0/24", "198.18.10.0/24")
}

// Goal: a publication with no in-publication dependency keeps its exact order
// and its own storage. Method: gateway routes whose gateways no device route
// in the publication contains, a device route, and an on-link route that
// precedes the device route its gateway lies in.
func TestOrderForwardingKeepsIndependentOrder(t *testing.T) {
	onLink := orderGateway("198.18.30.0/24", "192.0.2.9")
	onLink.OnLink = true
	input := []outgoingChange{
		orderGateway("198.18.31.0/24", "203.0.113.1"),
		onLink,
		orderGateway("198.18.32.0/24", "203.0.113.2"),
		orderDevice("192.0.2.0/24", "port1"),
		orderWithdraw("198.18.33.0/24"),
	}
	batches, err := orderForwarding(map[family.Family][]outgoingChange{family.IPv4Unicast: input}, 0)
	if err != nil {
		t.Fatal(err)
	}
	requireOrder(t, batches, "198.18.31.0/24", "198.18.30.0/24", "198.18.32.0/24", "192.0.2.0/24", "198.18.33.0/24")
	if &batches[0].Changes[0] != &input[0] {
		t.Fatal("an already ordered publication was copied")
	}
	none, err := orderForwarding(map[family.Family][]outgoingChange{family.IPv4Unicast: input[:3]}, 0)
	if err != nil {
		t.Fatal(err)
	}
	requireOrder(t, none, "198.18.31.0/24", "198.18.30.0/24", "198.18.32.0/24")
}

// Goal: a dependency across families orders across batches. Method: an IPv4
// route via an IPv6 gateway, published beside the IPv6 device route that
// reaches it. IPv4 sorts first, so the IPv6 cover's batch must come first.
func TestOrderForwardingAcrossFamilies(t *testing.T) {
	batches, err := orderForwarding(map[family.Family][]outgoingChange{
		family.IPv4Unicast: {orderGateway("198.18.40.0/24", "2001:db8:40::9"), orderDevice("198.18.41.0/24", "port1")},
		family.IPv6Unicast: {orderDevice("2001:db8:40::/64", "port1")},
	}, replay.Broadcast)
	if err != nil {
		t.Fatal(err)
	}
	requireOrder(t, batches, "198.18.41.0/24", "2001:db8:40::/64", "198.18.40.0/24")
	if len(batches) != 3 {
		t.Fatalf("batches = %+v, want IPv4, IPv6, IPv4", batches)
	}
	for i, want := range []family.Family{family.IPv4Unicast, family.IPv6Unicast, family.IPv4Unicast} {
		if batches[i].Family != want {
			t.Fatalf("batch %d family = %v, want %v", i, batches[i].Family, want)
		}
		if batches[i].ReplayID != replay.Broadcast {
			t.Fatalf("batch %d lost the replay identity: %+v", i, batches[i])
		}
	}
}

// Goal: every member gateway counts, and an install of the same prefix keeps
// its order behind the earlier entry for that prefix.
func TestOrderForwardingMembersAndRepeatedPrefix(t *testing.T) {
	member := orderGateway("198.18.50.0/24", "203.0.113.5")
	member.ECMPPaths = []sysribevents.ECMPPath{{NextHop: netip.MustParseAddr("192.0.2.5"), Interface: "port1"}}
	update := orderGateway("198.18.50.0/24", "192.0.2.6")
	update.Action = routeaction.Update
	batches, err := orderForwarding(map[family.Family][]outgoingChange{family.IPv4Unicast: {
		member,
		orderDevice("192.0.2.0/24", "port1"),
		update,
	}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	requireOrder(t, batches, "192.0.2.0/24", "198.18.50.0/24", "198.18.50.0/24")
	if batches[0].Changes[1].Action != routeaction.Add || batches[0].Changes[2].Action != routeaction.Update {
		t.Fatalf("two entries for one prefix changed order: %+v", batches[0].Changes)
	}
}

// Goal: a publication no order can apply is refused, never emitted unordered.
// Method: each of two prefixes appears first with a gateway inside the other
// and then as a device route, which makes the predecessors a cycle.
func TestOrderForwardingRefusesCycle(t *testing.T) {
	first := orderDevice("10.1.0.0/16", "port1")
	first.Action = routeaction.Update
	second := orderDevice("10.2.0.0/16", "port1")
	second.Action = routeaction.Update
	_, err := orderForwarding(map[family.Family][]outgoingChange{family.IPv4Unicast: {
		orderGateway("10.1.0.0/16", "10.2.0.1"),
		orderGateway("10.2.0.0/16", "10.1.0.1"),
		first,
		second,
	}}, 0)
	if err == nil {
		t.Fatal("a cyclic publication was ordered")
	}
}
