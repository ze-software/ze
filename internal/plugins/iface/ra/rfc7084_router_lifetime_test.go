package ifacera

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/ndp"
	"github.com/ze-software/ze/internal/core/routewatch"
)

var (
	testNow       = time.Unix(1_800_000_000, 0)
	testSubnet    = netip.MustParsePrefix("2001:db8:aa00:1::1/64")
	testDelegated = netip.MustParsePrefix("2001:db8:aa00::/56")
)

// ceSpec is a CE-role unit numbering from pppoe0, with the given delegated
// valid and preferred lifetimes left at testNow (none when valid is 0) and
// optionally one operator prefix.
func ceSpec(t *testing.T, valid, preferred time.Duration, operatorPrefix bool) iface.RASenderSpec {
	t.Helper()
	spec := iface.RASenderSpec{
		Interface:     "eth1",
		Unit:          "0",
		WANs:          []string{"pppoe0"},
		Advertisement: ndp.RAConfig{RouterLifetime: 1800},
	}
	if valid > 0 {
		spec.Delegated = []iface.DelegatedAdvert{testAdvert(t, valid, preferred)}
	}
	if operatorPrefix {
		spec.Advertisement.Prefixes = []ndp.PrefixInfo{{Prefix: netip.MustParsePrefix("2001:db8:1::/64"), OnLink: true, Autonomous: true, ValidLifetime: 86400, PreferredLifetime: 14400}}
	}
	return spec
}

// testAdvert builds the advert the iface component would hand the sender,
// with deadlines valid and preferred after testNow.
func testAdvert(t *testing.T, valid, preferred time.Duration) iface.DelegatedAdvert {
	t.Helper()
	return iface.NewDelegatedAdvert("pppoe0", testSubnet, testDelegated, testNow.Add(valid), testNow.Add(preferred))
}

func findPrefix(cfg ndp.RAConfig, p netip.Prefix) (ndp.PrefixInfo, bool) {
	for _, pi := range cfg.Prefixes {
		if pi.Prefix == p {
			return pi, true
		}
	}
	return ndp.PrefixInfo{}, false
}

func findRoute(cfg ndp.RAConfig, p netip.Prefix) (ndp.RouteInfo, bool) {
	for _, ri := range cfg.Routes {
		if ri.Prefix == p {
			return ri, true
		}
	}
	return ndp.RouteInfo{}, false
}

// VALIDATES: the Router Lifetime condition matrix (RFC 7084 G-4
// RFC7084-4.1-4, L-4 RFC7084-4.3-7, RFC 4861 Section 6.2.3), spec AC-9 and
// AC-15: one case per row, PIO and RIO kept while the delegation holds (L-3),
// and a unit with no WAN keeps its configured value whatever the WAN state.
// METHOD: build the advertisement for each W and P with a fixed clock.
// PREVENTS: a CE advertising itself as a default router with no way out, or
// with no prefix for its hosts to source from.
func TestRFC7084RouterLifetimeMatrix(t *testing.T) {
	cases := []struct {
		name     string
		w        bool
		valid    time.Duration
		operator bool
		want     uint16
		wantPIO  bool
	}{
		{"W yes, P yes (delegated)", true, time.Hour, false, 1800, true},
		{"W yes, P yes (operator prefix only)", true, 0, true, 1800, false},
		{"W yes, P no", true, 0, false, 0, false},
		{"W no, P yes", false, time.Hour, false, 0, true},
		{"W no, P no", false, 0, false, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			adv := &advertiser{spec: ceSpec(t, tc.valid, tc.valid/2, tc.operator)}
			cfg := adv.build(testNow, tc.w)
			if cfg.RouterLifetime != tc.want {
				t.Fatalf("Router Lifetime = %d, want %d", cfg.RouterLifetime, tc.want)
			}
			_, pio := findPrefix(cfg, testSubnet.Masked())
			_, rio := findRoute(cfg, testDelegated)
			if pio != tc.wantPIO {
				t.Fatalf("delegated PIO present = %v, want %v", pio, tc.wantPIO)
			}
			if rio != tc.wantPIO {
				t.Fatalf("delegated RIO present = %v, want %v (L-3: independent of WAN connectivity)", rio, tc.wantPIO)
			}
		})
	}

	t.Run("no WAN keeps the configured value (AC-15)", func(t *testing.T) {
		adv := &advertiser{spec: iface.RASenderSpec{Advertisement: ndp.RAConfig{RouterLifetime: 1800}}}
		for _, w := range []bool{true, false} {
			if got := adv.build(testNow, w).RouterLifetime; got != 1800 {
				t.Fatalf("W=%v: Router Lifetime = %d, want the configured 1800", w, got)
			}
		}
	})
}

// VALIDATES: RFC 9915 Section 6.3 (RFC9915-6.3-2) and RFC 7084 L-3
// (RFC7084-4.3-6), spec AC-8 and AC-10: the advertised PIO lifetimes are what
// remains at the moment of the send, preferred never above valid, with A and
// L set, and the RIO lifetime is the delegation's remaining valid lifetime.
// PREVENTS: an advertisement that keeps the lease's original lifetimes while
// the delegation runs down.
func TestRFC9915AdvertisedLifetimeBound(t *testing.T) {
	adv := &advertiser{spec: ceSpec(t, time.Hour, 2*time.Hour, false)}
	later := testNow.Add(1500*time.Second + 700*time.Millisecond)
	cfg := adv.build(later, true)
	pio, ok := findPrefix(cfg, testSubnet.Masked())
	if !ok {
		t.Fatal("no PIO for the delegated /64")
	}
	if pio.ValidLifetime != 2099 {
		t.Fatalf("valid = %d, want 2099 (3600 - 1500.7 rounded down)", pio.ValidLifetime)
	}
	if pio.PreferredLifetime != 2099 {
		t.Fatalf("preferred = %d, want 2099 (never above valid)", pio.PreferredLifetime)
	}
	if !pio.OnLink || !pio.Autonomous {
		t.Fatal("the delegated PIO must carry L and A (RFC 7084 L-6)")
	}
	rio, ok := findRoute(cfg, testDelegated)
	if !ok {
		t.Fatal("no RIO for the delegated prefix")
	}
	if rio.Lifetime != 2099 {
		t.Fatalf("RIO lifetime = %d, want 2099", rio.Lifetime)
	}
}

// VALIDATES: RFC 7084 G-5 (RFC7084-4.1-5), spec AC-11 and AC-14: losing W or
// P after a non-zero Router Lifetime went out asks for an advertisement at
// once, which carries 0 and keeps the PIO and RIO while P holds; a dropped
// delegation is withdrawn once with zero lifetimes.
// METHOD: drive the advertiser through sent, a WAN loss, and an update.
// PREVENTS: hosts keeping Ze as their default router until the next periodic
// advertisement, up to MaxRtrAdvInterval after the WAN went away.
func TestRFC7084G5ImmediateZeroLifetime(t *testing.T) {
	adv := &advertiser{spec: ceSpec(t, time.Hour, time.Hour, false)}
	first := adv.build(testNow, true)
	adv.sent(first)
	if first.RouterLifetime != 1800 {
		t.Fatalf("setup: Router Lifetime = %d, want 1800", first.RouterLifetime)
	}
	if adv.urgent(testNow, true) {
		t.Fatal("nothing changed, nothing is urgent")
	}

	if !adv.urgent(testNow, false) {
		t.Fatal("losing the WAN default route must send at once (G-5)")
	}
	lost := adv.build(testNow, false)
	if lost.RouterLifetime != 0 {
		t.Fatalf("Router Lifetime after WAN loss = %d, want 0", lost.RouterLifetime)
	}
	if _, ok := findPrefix(lost, testSubnet.Masked()); !ok {
		t.Fatal("the PIO must stay while the delegation holds")
	}
	adv.sent(lost)
	if !adv.urgent(testNow, true) {
		t.Fatal("W and P both true again must send at once (RFC 4861 Section 6.2.4)")
	}

	adv.update(ceSpec(t, 0, 0, false))
	if !adv.urgent(testNow, false) {
		t.Fatal("a dropped delegation must be withdrawn at once")
	}
	gone := adv.build(testNow, false)
	pio, ok := findPrefix(gone, testSubnet.Masked())
	if !ok || pio.ValidLifetime != 0 || pio.PreferredLifetime != 0 {
		t.Fatalf("the dropped /64 must be advertised once with zero lifetimes, got %+v (present %v)", pio, ok)
	}
	adv.sent(gone)
	if _, ok := findPrefix(adv.build(testNow, false), testSubnet.Masked()); ok {
		t.Fatal("the withdrawn /64 must not be advertised again")
	}
}

// VALIDATES: RFC 7084 G-4 input W, spec A-5 and AC-14: a main-table ::/0
// through the WAN device sets W, its removal clears it, a default through
// another device or another table does not count, and a seed read before a
// removal cannot restore the removed route.
// PREVENTS: Router Lifetime following a default route that is not the WAN's.
func TestRFC7084G4RouterLifetimeFollowsWANDefault(t *testing.T) {
	defaultRoute := func(ifindex int, table uint32, action routewatch.Action) routewatch.RouteEvent {
		return routewatch.RouteEvent{
			Prefix:     netip.MustParsePrefix("::/0"),
			NextHop:    netip.MustParseAddr("fe80::1"),
			TableID:    table,
			OutIfindex: ifindex,
			Action:     action,
		}
	}
	d := newWANDefaults()
	d.seed(nil)

	if !d.observe(defaultRoute(7, mainRouteTable, routewatch.ActionAdd)) {
		t.Fatal("adding the WAN default must report a change")
	}
	if !d.holds(7) {
		t.Fatal("W must hold through ifindex 7")
	}
	if d.holds(8) {
		t.Fatal("a default through 7 says nothing about 8")
	}
	if d.observe(defaultRoute(8, 100, routewatch.ActionAdd)) || d.holds(8) {
		t.Fatal("a default in another table is not the WAN default")
	}
	if d.observe(routewatch.RouteEvent{Prefix: netip.MustParsePrefix("2001:db8::/32"), TableID: mainRouteTable, OutIfindex: 8, Action: routewatch.ActionAdd}) {
		t.Fatal("a non-default route is ignored")
	}
	if !d.observe(defaultRoute(7, mainRouteTable, routewatch.ActionRemove)) || d.holds(7) {
		t.Fatal("removing the WAN default must clear W")
	}

	late := newWANDefaults()
	late.observe(defaultRoute(9, mainRouteTable, routewatch.ActionRemove))
	late.seed([]routewatch.RouteEvent{defaultRoute(9, mainRouteTable, routewatch.ActionAdd)})
	if late.holds(9) {
		t.Fatal("a seed read before a removal must not restore the route")
	}
}
