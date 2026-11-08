// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the Child SA route-change events
// Related: redistribute.go -- the emitters under test

package engine

import (
	"net"
	"net/netip"
	"slices"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/redistevents"
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
