package routewatch

import (
	"net/netip"
	"testing"
)

// TestRouteEventCarriesOutputDevice checks that the output interface index a
// route event carries reaches a registered handler unchanged. Method: deliver
// one event through the same path the netlink loop uses and read it back.
func TestRouteEventCarriesOutputDevice(t *testing.T) {
	w := New()
	var got []RouteEvent
	unregister := w.Register(func(ev RouteEvent) { got = append(got, ev) })
	defer unregister()

	w.deliver(RouteEvent{
		Prefix:     netip.MustParsePrefix("::/0"),
		NextHop:    netip.MustParseAddr("fe80::5a"),
		OutIfindex: 7,
		Action:     ActionAdd,
	})

	if len(got) != 1 {
		t.Fatalf("handler saw %d events, want 1", len(got))
	}
	if got[0].OutIfindex != 7 {
		t.Fatalf("OutIfindex %d, want 7", got[0].OutIfindex)
	}
}
