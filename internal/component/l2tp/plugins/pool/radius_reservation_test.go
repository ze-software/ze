// Design: docs/architecture/l2tp/subscriber-session-model.md -- pool ownership of RADIUS assignments

package l2tppool

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
)

const radiusTestTunnel uint16 = 49150

func radiusTestPool() *poolPlugin {
	return &poolPlugin{
		pool: newIPv4Pool(
			netip.MustParseAddr("10.0.0.1"),
			netip.MustParseAddr("10.0.0.10"),
			netip.MustParseAddr("10.0.0.11"),
			netip.MustParseAddr("192.0.2.53"), netip.Addr{},
		),
		namedPools: map[string]*ipv4Pool{
			"gold": newIPv4Pool(
				netip.MustParseAddr("10.0.0.2"),
				netip.MustParseAddr("10.0.0.20"),
				netip.MustParseAddr("10.0.0.21"),
				netip.MustParseAddr("198.51.100.53"), netip.Addr{},
			),
		},
	}
}

func radiusTestRequest(t *testing.T, p *poolPlugin, session uint16, address, selected string) ppp.IPResponseArgs {
	t.Helper()
	l2tp.StoreSessionMetadata(subscriber.SessionID(subscriber.AccessL2TP, uint64(radiusTestTunnel), session), &l2tp.AuthMetadata{
		FramedIP: netip.MustParseAddr(address), FramedPool: selected,
	})
	t.Cleanup(func() {
		l2tp.ClearSessionMetadata(subscriber.SessionID(subscriber.AccessL2TP, uint64(radiusTestTunnel), session))
	})
	return p.handle(ppp.EventIPRequest{
		SubscriberID: subscriber.SessionID(subscriber.AccessL2TP, uint64(radiusTestTunnel), session),
		TunnelID:     radiusTestTunnel, SessionID: session, Family: ppp.AddressFamilyIPv4,
	})
}

func radiusTestSessionDown(p *poolPlugin) {
	const session uint16 = 1
	p.onSessionDown(&subevents.SessionDownPayload{Session: subscriber.Session{
		ID:         subscriber.SessionID(subscriber.AccessL2TP, uint64(radiusTestTunnel), session),
		AccessType: subscriber.AccessL2TP, TunnelID: radiusTestTunnel, SessionID: session,
	}})
}

// TestPoolRADIUSAddressInRangeIsReserved asks the real pool handler for a RADIUS
// address and proves that the containing bitmap excludes it. Pool selection
// still supplies the gateway and DNS, even when another pool owns the address.
func TestPoolRADIUSAddressInRangeIsReserved(t *testing.T) {
	for _, tc := range []struct {
		name     string
		address  string
		selected string
		owner    string
	}{
		{name: "default-address-default-profile", address: "10.0.0.10"},
		{name: "named-address-default-profile", address: "10.0.0.20", owner: "gold"},
		{name: "default-address-named-profile", address: "10.0.0.10", selected: "gold"},
		{name: "named-address-named-profile", address: "10.0.0.20", selected: "gold", owner: "gold"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := radiusTestPool()
			answer := radiusTestRequest(t, p, 1, tc.address, tc.selected)
			if !answer.Accept {
				t.Fatalf("RADIUS address refused: %s", answer.Reason)
			}
			address := netip.MustParseAddr(tc.address)
			if answer.Peer != address {
				t.Fatalf("RADIUS peer = %s, want %s", answer.Peer, address)
			}
			selected := p.pool
			if tc.selected != "" {
				selected = p.namedPools[tc.selected]
			}
			if answer.Local != selected.gateway {
				t.Fatalf("RADIUS gateway = %s, want selected pool's %s", answer.Local, selected.gateway)
			}
			if answer.DNSPrimary != selected.dnsPrimary {
				t.Fatalf("RADIUS DNS = %s, want selected pool's %s", answer.DNSPrimary, selected.dnsPrimary)
			}
			owner := p.pool
			if tc.owner != "" {
				owner = p.namedPools[tc.owner]
			}
			if _, allocated, _ := owner.stats(); allocated != 1 {
				t.Fatalf("containing pool allocated = %d, want 1", allocated)
			}
			if tc.owner != "" {
				l2tp.StoreSessionMetadata(subscriber.SessionID(subscriber.AccessL2TP, uint64(radiusTestTunnel), 2), &l2tp.AuthMetadata{FramedPool: tc.owner})
				t.Cleanup(func() {
					l2tp.ClearSessionMetadata(subscriber.SessionID(subscriber.AccessL2TP, uint64(radiusTestTunnel), 2))
				})
			}
			next := allocate(t, p, radiusTestTunnel, 2, subscriber.AccessL2TP)
			if next == address {
				t.Fatalf("pool handed the RADIUS address %s to another subscriber", next)
			}
			if want := address.Next(); next != want {
				t.Fatalf("next pool address = %s, want %s", next, want)
			}
		})
	}
}

// TestPoolRADIUSAddressAlreadyHeldIsRefused tries a RADIUS assignment after an
// ordinary or RADIUS subscriber owns that address. The refusal identifies the
// address, leaves its owner intact, and creates no second session record.
func TestPoolRADIUSAddressAlreadyHeldIsRefused(t *testing.T) {
	for _, holder := range []string{"pool", "radius"} {
		t.Run(holder, func(t *testing.T) {
			p := radiusTestPool()
			address := netip.MustParseAddr("10.0.0.10")
			if holder == "pool" {
				if got := allocate(t, p, radiusTestTunnel, 1, subscriber.AccessL2TP); got != address {
					t.Fatalf("first pool address = %s, want %s", got, address)
				}
			} else {
				if answer := radiusTestRequest(t, p, 1, address.String(), ""); !answer.Accept {
					t.Fatalf("first RADIUS address refused: %s", answer.Reason)
				}
			}
			answer := radiusTestRequest(t, p, 2, address.String(), "gold")
			if answer.Accept {
				t.Fatal("RADIUS received an address another subscriber holds")
			}
			if !strings.Contains(answer.Reason, address.String()) {
				t.Fatalf("refusal %q does not name %s", answer.Reason, address)
			}
			if !strings.Contains(answer.Reason, "in use") {
				t.Fatalf("refusal %q does not explain the address is in use", answer.Reason)
			}
			if _, exists := p.sessionAddrs.Load(subscriber.SessionID(subscriber.AccessL2TP, uint64(radiusTestTunnel), 2)); exists {
				t.Fatal("refused RADIUS assignment created a session record")
			}
			if _, allocated, _ := p.pool.stats(); allocated != 1 {
				t.Fatalf("original allocation count = %d, want 1", allocated)
			}
		})
	}
}

// TestPoolRADIUSAddressOutsideRangesPreservesProfile proves an address outside
// every range remains admissible and uses the selected pool's gateway and DNS.
// It must not consume a bitmap slot that another subscriber can use.
func TestPoolRADIUSAddressOutsideRangesPreservesProfile(t *testing.T) {
	p := radiusTestPool()
	answer := radiusTestRequest(t, p, 1, "203.0.113.10", "gold")
	if !answer.Accept {
		t.Fatalf("outside RADIUS address refused: %s", answer.Reason)
	}
	if answer.Peer != netip.MustParseAddr("203.0.113.10") {
		t.Fatalf("outside RADIUS peer = %s", answer.Peer)
	}
	if answer.Local != p.namedPools["gold"].gateway {
		t.Fatalf("outside RADIUS gateway = %s", answer.Local)
	}
	if answer.DNSPrimary != p.namedPools["gold"].dnsPrimary {
		t.Fatalf("outside RADIUS DNS = %s", answer.DNSPrimary)
	}
	for name, pool := range map[string]*ipv4Pool{"default": p.pool, "gold": p.namedPools["gold"]} {
		if _, allocated, _ := pool.stats(); allocated != 0 {
			t.Fatalf("outside RADIUS address consumed %s bitmap slot: %d", name, allocated)
		}
	}
	radiusTestSessionDown(p)
	if got := allocate(t, p, radiusTestTunnel, 2, subscriber.AccessL2TP); got != netip.MustParseAddr("10.0.0.10") {
		t.Fatalf("default pool address after outside teardown = %s", got)
	}
}

// TestPoolRADIUSReservationReleasedOnSessionDown ends a RADIUS subscriber whose
// selected profile differs from its address's pool, then reallocates its slot.
func TestPoolRADIUSReservationReleasedOnSessionDown(t *testing.T) {
	p := radiusTestPool()
	answer := radiusTestRequest(t, p, 1, "10.0.0.10", "gold")
	if !answer.Accept {
		t.Fatalf("RADIUS address refused: %s", answer.Reason)
	}
	if _, allocated, _ := p.pool.stats(); allocated != 1 {
		t.Fatalf("RADIUS reservation count = %d, want 1", allocated)
	}
	radiusTestSessionDown(p)
	if _, allocated, _ := p.pool.stats(); allocated != 0 {
		t.Fatalf("after RADIUS teardown allocated = %d, want 0", allocated)
	}
	if got := allocate(t, p, radiusTestTunnel, 2, subscriber.AccessL2TP); got != answer.Peer {
		t.Fatalf("pool returned %s after RADIUS teardown, want %s", got, answer.Peer)
	}
}

// TestPoolRADIUSReservationSurvivesReloadRollback carries a live RADIUS address
// into a wider pool and back again. Allocation on each side skips the held
// address, and teardown releases it into the restored pool.
func TestPoolRADIUSReservationSurvivesReloadRollback(t *testing.T) {
	p := &poolPlugin{}
	tx := p.configTransaction()
	if err := tx.Configure(reloadPoolSection(t, map[string]any{
		"ipv4": reloadIPv4("10.0.0.10", "10.0.0.11"),
	})); err != nil {
		t.Fatal(err)
	}
	answer := radiusTestRequest(t, p, 1, "10.0.0.11", "")
	if !answer.Accept {
		t.Fatalf("RADIUS address refused: %s", answer.Reason)
	}
	if err := tx.Verify(reloadPoolSection(t, map[string]any{
		"ipv4": reloadIPv4("10.0.0.10", "10.0.0.12"),
	})); err != nil {
		t.Fatal(err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatal(err)
	}
	if got := allocate(t, p, radiusTestTunnel, 2, subscriber.AccessL2TP); got != netip.MustParseAddr("10.0.0.10") {
		t.Fatalf("first dynamic address after reload = %s", got)
	}
	if got := allocate(t, p, radiusTestTunnel, 3, subscriber.AccessL2TP); got != netip.MustParseAddr("10.0.0.12") {
		t.Fatalf("dynamic allocation did not skip RADIUS reservation: %s", got)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	total, allocated, available := p.pool.stats()
	if total != 2 {
		t.Fatalf("restored pool total = %d, want 2", total)
	}
	if allocated != 2 {
		t.Fatalf("restored pool allocated = %d, want 2", allocated)
	}
	if available != 0 {
		t.Fatalf("restored pool available = %d, want 0", available)
	}
	radiusTestSessionDown(p)
	if got := allocate(t, p, radiusTestTunnel, 4, subscriber.AccessL2TP); got != answer.Peer {
		t.Fatalf("restored pool returned %s after RADIUS teardown, want %s", got, answer.Peer)
	}
}

// TestPoolRADIUSAddressOutsideRangesAlreadyHeldIsRefused gives two subscribers
// the same external RADIUS address. A valid unique external address remains
// allowed, but the second request must not create another live owner.
func TestPoolRADIUSAddressOutsideRangesAlreadyHeldIsRefused(t *testing.T) {
	p := radiusTestPool()
	first := radiusTestRequest(t, p, 1, "203.0.113.10", "")
	if !first.Accept {
		t.Fatalf("first external RADIUS address refused: %s", first.Reason)
	}
	second := radiusTestRequest(t, p, 2, "203.0.113.10", "gold")
	if second.Accept {
		t.Fatal("RADIUS assigned the same external address to two subscribers")
	}
	if !strings.Contains(second.Reason, "203.0.113.10") {
		t.Fatalf("refusal %q does not name the held external address", second.Reason)
	}
	if !strings.Contains(second.Reason, "in use") {
		t.Fatalf("refusal %q does not explain the external address is in use", second.Reason)
	}
	radiusTestSessionDown(p)
	third := radiusTestRequest(t, p, 3, "203.0.113.10", "")
	if !third.Accept {
		t.Fatalf("released external RADIUS address refused: %s", third.Reason)
	}
}

// TestPoolRADIUSOrphanAddressAlreadyHeldIsRefused removes a live subscriber's
// range, then asks RADIUS for the carried address. Removing the range must not
// remove the subscriber's exclusive ownership before its session ends.
func TestPoolRADIUSOrphanAddressAlreadyHeldIsRefused(t *testing.T) {
	p := &poolPlugin{}
	tx := p.configTransaction()
	if err := tx.Configure(reloadPoolSection(t, map[string]any{
		"ipv4": reloadIPv4("10.0.0.10", "10.0.0.11"),
	})); err != nil {
		t.Fatal(err)
	}
	first := allocate(t, p, radiusTestTunnel, 1, subscriber.AccessL2TP)
	if err := tx.Verify(reloadPoolSection(t, map[string]any{
		"ipv4": reloadIPv4("10.0.0.20", "10.0.0.21"),
	})); err != nil {
		t.Fatal(err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatal(err)
	}
	second := radiusTestRequest(t, p, 2, first.String(), "")
	if second.Accept {
		t.Fatalf("RADIUS assigned the live orphan %s to another subscriber", first)
	}
	if !strings.Contains(second.Reason, first.String()) {
		t.Fatalf("refusal %q does not name the live orphan %s", second.Reason, first)
	}
	radiusTestSessionDown(p)
	third := radiusTestRequest(t, p, 3, first.String(), "")
	if !third.Accept {
		t.Fatalf("released orphan RADIUS address refused: %s", third.Reason)
	}
}
