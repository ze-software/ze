// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the Child SA route-change events
// Related: redistribute.go -- the emitters under test

package engine

import (
	"net"
	"net/netip"
	"slices"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/iprule"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rtproto"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// routeChange is one route-change entry as a subscriber saw it, with its batch's family.
type routeChange struct {
	afi    uint16
	safi   uint8
	action redistevents.RouteAction
	prefix netip.Prefix
}

// routeBus records the ipsec route-change entries the engine emits. It copies each
// entry during Emit, because the emitter releases the batch to its pool once Emit
// returns, exactly as an engine subscriber must.
type routeBus struct {
	mu      sync.Mutex
	changes []routeChange
}

func (b *routeBus) Emit(namespace, eventType string, payload any) (int, error) {
	if namespace != ipsecName {
		return 0, nil
	}
	if eventType != redistevents.EventType {
		return 0, nil
	}
	batch, ok := payload.(*redistevents.RouteChangeBatch)
	if !ok {
		return 0, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, entry := range batch.Entries {
		b.changes = append(b.changes, routeChange{afi: batch.AFI, safi: batch.SAFI, action: entry.Action, prefix: entry.Prefix})
	}
	return 1, nil
}

func (b *routeBus) Subscribe(_, _ string, _ func(any)) func() { return func() {} }

func (b *routeBus) take() []routeChange {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := b.changes
	b.changes = nil
	return out
}

// VALIDATES: a Child SA that negotiated an IPv4 pair first and an IPv6 pair second
// announces both remote prefixes on the route-change bus when it comes up, each in a
// batch of its own family, and withdraws both when it goes.
// PREVENTS: emitRouteAdd and emitRouteRemove publishing child.TSRemote alone. TSRemote
// holds only the first pair, so the IPv6 remote prefix of a dual-family route-based
// tunnel never reached the redistribute consumers (BGP, OSPF, IS-IS).
func TestChildSARouteEventsCarryEveryRemoteSelector(t *testing.T) {
	log := slogutil.DiscardLogger()
	remote4 := &net.IPNet{IP: net.ParseIP("10.255.2.0").To4(), Mask: net.CIDRMask(24, 32)}
	child := &ChildSA{
		IfID:     vti0IfID,
		TSLocal:  &net.IPNet{IP: net.ParseIP("10.255.1.0").To4(), Mask: net.CIDRMask(24, 32)},
		TSRemote: remote4,
		Selectors: []tsPair{
			{
				I: tsSelector{Net: &net.IPNet{IP: net.ParseIP("10.255.1.0").To4(), Mask: net.CIDRMask(24, 32)}, Port: ipsec.AnyPort()},
				R: tsSelector{Net: remote4, Port: ipsec.AnyPort()},
			},
			{
				I: tsSelector{Net: &net.IPNet{IP: net.ParseIP("fd00:db8:1::"), Mask: net.CIDRMask(64, 128)}, Port: ipsec.AnyPort()},
				R: tsSelector{Net: &net.IPNet{IP: net.ParseIP("fd00:db8:2::"), Mask: net.CIDRMask(64, 128)}, Port: ipsec.AnyPort()},
			},
		},
		SelectorsLocalIsTSi: true,
	}
	bus := &routeBus{}

	emitRouteAdd(bus, child, log)
	want := []routeChange{
		{afi: 1, safi: 1, action: redistevents.ActionAdd, prefix: netip.MustParsePrefix("10.255.2.0/24")},
		{afi: 2, safi: 1, action: redistevents.ActionAdd, prefix: netip.MustParsePrefix("fd00:db8:2::/64")},
	}
	if got := bus.take(); !slices.Equal(got, want) {
		t.Fatalf("add: got %v, want %v", got, want)
	}

	emitRouteRemove(bus, child, log)
	want = []routeChange{
		{afi: 1, safi: 1, action: redistevents.ActionRemove, prefix: netip.MustParsePrefix("10.255.2.0/24")},
		{afi: 2, safi: 1, action: redistevents.ActionRemove, prefix: netip.MustParsePrefix("fd00:db8:2::/64")},
	}
	if got := bus.take(); !slices.Equal(got, want) {
		t.Fatalf("remove: got %v, want %v", got, want)
	}
}

// VALIDATES: a Child SA with no negotiated selector set still announces and withdraws
// TSRemote, one route-change entry in one batch, as before the selector set was read.
// PREVENTS: the multi-selector fix dropping the route of a Child SA that carries only
// TSRemote.
func TestChildSARouteEventsWithoutSelectorsCarryTSRemote(t *testing.T) {
	log := slogutil.DiscardLogger()
	child := &ChildSA{
		TSLocal:  &net.IPNet{IP: net.ParseIP("192.0.2.0").To4(), Mask: net.CIDRMask(24, 32)},
		TSRemote: &net.IPNet{IP: net.ParseIP("198.51.100.0").To4(), Mask: net.CIDRMask(24, 32)},
	}
	bus := &routeBus{}

	emitRouteAdd(bus, child, log)
	emitRouteRemove(bus, child, log)
	want := []routeChange{
		{afi: 1, safi: 1, action: redistevents.ActionAdd, prefix: netip.MustParsePrefix("198.51.100.0/24")},
		{afi: 1, safi: 1, action: redistevents.ActionRemove, prefix: netip.MustParsePrefix("198.51.100.0/24")},
	}
	if got := bus.take(); !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// rekeyTestChild builds a Child SA on vti0 whose local side is 10.255.1.0/24 and
// fd00:db8:1::/64 and whose remote side is the given prefixes, one selector pair each,
// in order. The local half of each pair matches the remote half's family.
func rekeyTestChild(t *testing.T, remotes ...string) *ChildSA {
	t.Helper()
	child := &ChildSA{IfID: vti0IfID, SelectorsLocalIsTSi: true}
	for _, remote := range remotes {
		prefix := netip.MustParsePrefix(remote)
		local := netip.MustParsePrefix("10.255.1.0/24")
		if prefix.Addr().Is6() {
			local = netip.MustParsePrefix("fd00:db8:1::/64")
		}
		pair := tsPair{
			I: tsSelector{Net: prefixIPNet(local), Port: ipsec.AnyPort()},
			R: tsSelector{Net: prefixIPNet(prefix), Port: ipsec.AnyPort()},
		}
		child.Selectors = append(child.Selectors, pair)
		if child.TSRemote == nil {
			child.TSLocal, child.TSRemote = pair.I.Net, pair.R.Net
		}
	}
	return child
}

func prefixIPNet(prefix netip.Prefix) *net.IPNet {
	bits := 32
	if prefix.Addr().Is6() {
		bits = 128
	}
	return &net.IPNet{IP: net.IP(prefix.Addr().AsSlice()), Mask: net.CIDRMask(prefix.Bits(), bits)}
}

// rekeyGuard stands vti0 up under the DISCARD default with a guarded IPv6 tunnel
// address, as TestChildSAGuardsEveryNegotiatedIPv6Selector does, and answers the stub
// and vti0's guard table, so a test can read which tunnel routes the guard holds.
func rekeyGuard(t *testing.T) (*xfrmInterfaceStub, uint32) {
	t.Helper()
	dp := &catchAllDP{}
	loadFailClosedBackend(t, dp)
	stub := stubXFRMInterfaceReads(t)
	stub.facts["vti0"] = xfrmInterfaceFacts{device: "vti0", ifIndex: 7, ifID: vti0IfID, addresses: []netip.Addr{mustAddr(t, "fd00:db8:1::1")}}
	state := applyTestState(t)
	if err := state.applyConfig(vtiBoundConfig(dataplane.SPActionDiscard), applyReload); err != nil {
		t.Fatalf("discard apply: %v", err)
	}
	return stub, stub.guard.rules[netip.MustParsePrefix("fd00:db8:1::1/128")]
}

func guardHolds(stub *xfrmInterfaceStub, table uint32, prefix string) bool {
	return stub.guard.routes[iprule.Route{Table: table, Dst: netip.MustParsePrefix(prefix), LinkIndex: 7, Protocol: rtproto.IPsecSourceGuard}]
}

// VALIDATES: a Child SA rekey that changes the selector set withdraws, from the
// route-change bus and from the tunnel-source guard, exactly the prefixes the retired
// Child SA carried and the successor does not, announces only the ones the successor
// adds, and emits nothing for a prefix both carry.
// PREVENTS: the owner loop announcing the successor's set and never withdrawing the
// retired one, which left a prefix the rekey narrowed away announced to BGP, OSPF and
// IS-IS and routed into the interface by the guard table, for as long as the tunnel ran.
func TestChildSARekeyMovesOnlyChangedTunnelRoutes(t *testing.T) {
	tests := []struct {
		name      string
		successor []string
		want      []routeChange
		guardKept []string
		guardGone []string
	}{
		{
			name:      "narrowed to IPv4",
			successor: []string{"10.255.2.0/24"},
			want: []routeChange{
				{afi: 2, safi: 1, action: redistevents.ActionRemove, prefix: netip.MustParsePrefix("fd00:db8:2::/64")},
			},
			guardGone: []string{"fd00:db8:2::/64"},
		},
		{
			name:      "IPv6 selector replaced",
			successor: []string{"10.255.2.0/24", "fd00:db8:3::/64"},
			want: []routeChange{
				{afi: 2, safi: 1, action: redistevents.ActionAdd, prefix: netip.MustParsePrefix("fd00:db8:3::/64")},
				{afi: 2, safi: 1, action: redistevents.ActionRemove, prefix: netip.MustParsePrefix("fd00:db8:2::/64")},
			},
			guardKept: []string{"fd00:db8:3::/64"},
			guardGone: []string{"fd00:db8:2::/64"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub, table := rekeyGuard(t)
			log := slogutil.DiscardLogger()
			retired := rekeyTestChild(t, "10.255.2.0/24", "fd00:db8:2::/64")
			successor := rekeyTestChild(t, tt.successor...)
			bus := &routeBus{}
			guardTunnelRoute(true, "swan", retired, log)

			rekeyTunnelRoutes(bus, "swan", retired, successor, log)
			if got := bus.take(); !slices.Equal(got, tt.want) {
				t.Errorf("route changes: got %v, want %v", got, tt.want)
			}
			for _, prefix := range tt.guardKept {
				if !guardHolds(stub, table, prefix) {
					t.Errorf("guard table lacks %s, which the successor carries: %v", prefix, stub.guard.routes)
				}
			}
			for _, prefix := range tt.guardGone {
				if guardHolds(stub, table, prefix) {
					t.Errorf("guard table still holds %s, which the rekey dropped: %v", prefix, stub.guard.routes)
				}
			}
		})
	}
}

// VALIDATES: a Child SA rekey that keeps the selector set emits no route change and
// leaves the guard table's tunnel route in place.
// PREVENTS: a rekey flapping every tunnel route (withdraw, then announce) on each
// lifetime expiry, which every redistribute consumer would propagate.
func TestChildSARekeyKeepsUnchangedTunnelRoutes(t *testing.T) {
	stub, table := rekeyGuard(t)
	log := slogutil.DiscardLogger()
	retired := rekeyTestChild(t, "10.255.2.0/24", "fd00:db8:2::/64")
	successor := rekeyTestChild(t, "10.255.2.0/24", "fd00:db8:2::/64")
	bus := &routeBus{}
	guardTunnelRoute(true, "swan", retired, log)

	rekeyTunnelRoutes(bus, "swan", retired, successor, log)
	if got := bus.take(); len(got) != 0 {
		t.Errorf("route changes for an unchanged selector set: %v, want none", got)
	}
	if !guardHolds(stub, table, "fd00:db8:2::/64") {
		t.Errorf("guard table lost the kept IPv6 tunnel route: %v", stub.guard.routes)
	}
}
