// Design: docs/architecture/l2tp/bng-1-radius-attributes.md -- Framed-IP-Address 0xFFFFFFFF pool bypass
// Related: register.go -- poolPlugin.handle honors AuthMetadata.FramedIPNegotiated
//
// VALIDATES: when AuthMetadata.FramedIPNegotiated is true (RADIUS
// Framed-IP-Address == 0xFFFFFFFF), the pool handler does not consume a pool
// slot, returns PeerNegotiated=true with Peer unset and the selected pool's
// local gateway and DNS servers, and succeeds even when the pool is exhausted.
// PREVENTS: pool allocation overriding peer-selected IP negotiation or failing
// a negotiated session on an exhausted range.

package l2tppool

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
)

// TestPoolHandleFramedIPNegotiatedBypassesAllocation verifies that a
// peer-negotiated session takes no pool address, carries PeerNegotiated=true
// with Local gateway and DNS, and works even when the pool has no free slots.
func TestPoolHandleFramedIPNegotiatedBypassesAllocation(t *testing.T) {
	dns1 := netip.MustParseAddr("1.1.1.1")
	dns2 := netip.MustParseAddr("8.8.8.8")
	pool := newIPv4Pool(testGW, netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.1"), dns1, dns2)

	p := &poolPlugin{pool: pool}

	// Exhaust the single-slot pool first with a normal session.
	firstID := subscriber.SessionID(subscriber.AccessL2TP, uint64(10), 1)
	first := p.handle(ppp.EventIPRequest{
		SubscriberID: firstID,
		TunnelID:     10,
		SessionID:    1,
		Family:       ppp.AddressFamilyIPv4,
	})
	if !first.Accept || first.PeerNegotiated || first.Peer != netip.MustParseAddr("10.0.0.1") {
		t.Fatalf("control pool allocation = %+v, want Accept=true Peer=10.0.0.1 PeerNegotiated=false", first)
	}
	if _, allocated, avail := pool.stats(); allocated != 1 || avail != 0 {
		t.Fatalf("after control allocation: allocated=%d avail=%d, want 1 and 0", allocated, avail)
	}

	// Now a peer-negotiated session (0xFFFFFFFF) arrives on the exhausted pool.
	negID := subscriber.SessionID(subscriber.AccessL2TP, uint64(10), 2)
	l2tp.StoreSessionMetadata(negID, &l2tp.AuthMetadata{FramedIPNegotiated: true})
	t.Cleanup(func() { l2tp.ClearSessionMetadata(negID) })

	got := p.handle(ppp.EventIPRequest{
		SubscriberID: negID,
		TunnelID:     10,
		SessionID:    2,
		Family:       ppp.AddressFamilyIPv4,
	})
	if !got.Accept {
		t.Fatalf("negotiated session refused: %s", got.Reason)
	}
	if !got.PeerNegotiated {
		t.Error("PeerNegotiated = false, want true")
	}
	if got.Peer.IsValid() {
		t.Errorf("Peer = %v, want unset for PeerNegotiated", got.Peer)
	}
	if got.Local != testGW {
		t.Errorf("Local = %v, want %v", got.Local, testGW)
	}
	if got.AllocationID != 0 {
		t.Errorf("AllocationID = %d, want 0 (no pool reservation owned)", got.AllocationID)
	}
	if got.DNSPrimary != dns1 || got.DNSSecondary != dns2 {
		t.Errorf("DNS = (%v, %v), want (%v, %v)", got.DNSPrimary, got.DNSSecondary, dns1, dns2)
	}
	if _, exists := p.sessionAddrs.Load(negID); exists {
		t.Error("peer-negotiated session unexpectedly created a pool sessionAddrs record before admission")
	}
	if got.AdmitPeerIPv4 == nil {
		t.Fatal("AdmitPeerIPv4 = nil, want pool admission callback")
	}
	if got.ReleasePeerIPv4 == nil {
		t.Fatal("ReleasePeerIPv4 = nil, want pool release callback")
	}
}

// TestPoolHandleFramedIPNegotiatedUsesNamedPoolProfile verifies that
// Framed-Pool alongside Framed-IP-Address == 0xFFFFFFFF supplies the named
// pool's gateway and DNS without allocating from either pool.
func TestPoolHandleFramedIPNegotiatedUsesNamedPoolProfile(t *testing.T) {
	defaultPool := newIPv4Pool(testGW, netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.5"),
		netip.MustParseAddr("9.9.9.9"), netip.Addr{})
	goldGW := netip.MustParseAddr("10.1.0.254")
	goldDNS := netip.MustParseAddr("198.51.100.53")
	goldPool := newIPv4Pool(goldGW, netip.MustParseAddr("10.1.0.1"), netip.MustParseAddr("10.1.0.5"),
		goldDNS, netip.Addr{})

	p := &poolPlugin{
		pool:       defaultPool,
		namedPools: map[string]*ipv4Pool{"gold": goldPool},
	}

	id := subscriber.SessionID(subscriber.AccessL2TP, uint64(11), 3)
	l2tp.StoreSessionMetadata(id, &l2tp.AuthMetadata{
		FramedIPNegotiated: true,
		FramedPool:         "gold",
	})
	t.Cleanup(func() { l2tp.ClearSessionMetadata(id) })

	got := p.handle(ppp.EventIPRequest{
		SubscriberID: id,
		TunnelID:     11,
		SessionID:    3,
		Family:       ppp.AddressFamilyIPv4,
	})
	if !got.Accept {
		t.Fatalf("named-pool negotiated session refused: %s", got.Reason)
	}
	if !got.PeerNegotiated {
		t.Error("PeerNegotiated = false, want true")
	}
	if got.Peer.IsValid() {
		t.Errorf("Peer = %v, want unset", got.Peer)
	}
	if got.Local != goldGW {
		t.Errorf("Local = %v, want gold gateway %v", got.Local, goldGW)
	}
	if got.DNSPrimary != goldDNS {
		t.Errorf("DNSPrimary = %v, want gold DNS %v", got.DNSPrimary, goldDNS)
	}
	if _, alloc, _ := defaultPool.stats(); alloc != 0 {
		t.Errorf("default pool allocated = %d, want 0", alloc)
	}
	if _, alloc, _ := goldPool.stats(); alloc != 0 {
		t.Errorf("gold pool allocated = %d, want 0", alloc)
	}
}

// TestPoolPeerNegotiatedAdmissionExcludesPoolsAndActiveSessions verifies that
// AdmitPeerIPv4 enforces the same exclusivity rules as static assignments:
// addresses inside any configured pool (default or named) or already held by
// another live session (pool, static RADIUS, or peer-negotiated) are refused,
// while a free external unicast address is recorded in sessionAddrs and freed
// cleanly by ReleasePeerIPv4 or session teardown.
func TestPoolPeerNegotiatedAdmissionExcludesPoolsAndActiveSessions(t *testing.T) {
	defaultPool := newIPv4Pool(testGW, netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.5"),
		netip.MustParseAddr("1.1.1.1"), netip.Addr{})
	goldPool := newIPv4Pool(netip.MustParseAddr("10.1.0.254"), netip.MustParseAddr("10.1.0.1"), netip.MustParseAddr("10.1.0.5"),
		netip.Addr{}, netip.Addr{})

	p := &poolPlugin{
		pool:       defaultPool,
		namedPools: map[string]*ipv4Pool{"gold": goldPool},
	}

	// 1. Allocate one pool address and one external static RADIUS address.
	poolID := subscriber.SessionID(subscriber.AccessL2TP, uint64(20), 1)
	poolAns := p.handle(ppp.EventIPRequest{
		SubscriberID: poolID, TunnelID: 20, SessionID: 1, Family: ppp.AddressFamilyIPv4,
	})
	if !poolAns.Accept {
		t.Fatalf("pool allocation refused: %s", poolAns.Reason)
	}

	staticAddr := netip.MustParseAddr("203.0.113.10")
	staticID := subscriber.SessionID(subscriber.AccessL2TP, uint64(20), 2)
	l2tp.StoreSessionMetadata(staticID, &l2tp.AuthMetadata{FramedIP: staticAddr})
	t.Cleanup(func() { l2tp.ClearSessionMetadata(staticID) })
	if ans := p.handle(ppp.EventIPRequest{
		SubscriberID: staticID, TunnelID: 20, SessionID: 2, Family: ppp.AddressFamilyIPv4,
	}); !ans.Accept {
		t.Fatalf("static RADIUS allocation refused: %s", ans.Reason)
	}

	// 2. Peer-negotiated session A.
	negAID := subscriber.SessionID(subscriber.AccessL2TP, uint64(20), 3)
	l2tp.StoreSessionMetadata(negAID, &l2tp.AuthMetadata{FramedIPNegotiated: true})
	t.Cleanup(func() { l2tp.ClearSessionMetadata(negAID) })
	negA := p.handle(ppp.EventIPRequest{
		SubscriberID: negAID, TunnelID: 20, SessionID: 3, Family: ppp.AddressFamilyIPv4,
	})
	if !negA.Accept || negA.CheckPeerIPv4 == nil || negA.AdmitPeerIPv4 == nil || negA.ReleasePeerIPv4 == nil {
		t.Fatalf("negA answer = %+v, want Accept with callbacks", negA)
	}

	// Refused: inside default pool (allocated and free), inside named pool,
	// and held by a static RADIUS session.
	for _, tc := range []struct {
		name string
		addr netip.Addr
	}{
		{"default-pool-allocated", poolAns.Peer},
		{"default-pool-free", netip.MustParseAddr("10.0.0.5")},
		{"named-pool-free", netip.MustParseAddr("10.1.0.3")},
		{"static-radius-active", staticAddr},
	} {
		if negA.CheckPeerIPv4(tc.addr) {
			t.Errorf("CheckPeerIPv4(%s, %s) = true, want false", tc.name, tc.addr)
		}
		if negA.AdmitPeerIPv4(tc.addr) {
			t.Errorf("AdmitPeerIPv4(%s, %s) = true, want false", tc.name, tc.addr)
		}
	}

	// CheckPeerIPv4 is read-only: checking a free address returns true without
	// reserving it in sessionAddrs.
	chosenA := netip.MustParseAddr("198.51.100.42")
	if !negA.CheckPeerIPv4(chosenA) {
		t.Fatalf("CheckPeerIPv4(%s) = false, want true", chosenA)
	}
	if _, exists := p.sessionAddrs.Load(negAID); exists {
		t.Fatal("CheckPeerIPv4 reserved an address in sessionAddrs; must be read-only")
	}

	// Admitted: free external unicast address.
	if !negA.AdmitPeerIPv4(chosenA) {
		t.Fatalf("AdmitPeerIPv4(%s) = false, want true", chosenA)
	}
	// Idempotent re-admission of the same address for the same session.
	if !negA.AdmitPeerIPv4(chosenA) {
		t.Fatalf("idempotent AdmitPeerIPv4(%s) = false, want true", chosenA)
	}
	stored, ok := p.sessionAddrs.Load(negAID)
	if !ok {
		t.Fatal("admitted peer-negotiated address not recorded in sessionAddrs")
	}
	if sa, ok := stored.(sessionAddr); !ok || sa.addr != chosenA || sa.fromPool || !sa.peerNegotiated {
		t.Fatalf("sessionAddrs[%s] = %+v, want addr=%s fromPool=false peerNegotiated=true", negAID, stored, chosenA)
	}

	// Peer-negotiated session B cannot steal chosenA.
	negBID := subscriber.SessionID(subscriber.AccessL2TP, uint64(20), 4)
	l2tp.StoreSessionMetadata(negBID, &l2tp.AuthMetadata{FramedIPNegotiated: true})
	t.Cleanup(func() { l2tp.ClearSessionMetadata(negBID) })
	negB := p.handle(ppp.EventIPRequest{
		SubscriberID: negBID, TunnelID: 20, SessionID: 4, Family: ppp.AddressFamilyIPv4,
	})
	if negB.AdmitPeerIPv4(chosenA) {
		t.Errorf("second peer-negotiated session admitted active address %s", chosenA)
	}

	// Static RADIUS session C also cannot steal chosenA.
	staticCID := subscriber.SessionID(subscriber.AccessL2TP, uint64(20), 5)
	l2tp.StoreSessionMetadata(staticCID, &l2tp.AuthMetadata{FramedIP: chosenA})
	t.Cleanup(func() { l2tp.ClearSessionMetadata(staticCID) })
	if ans := p.handle(ppp.EventIPRequest{
		SubscriberID: staticCID, TunnelID: 20, SessionID: 5, Family: ppp.AddressFamilyIPv4,
	}); ans.Accept {
		t.Errorf("static RADIUS session stole peer-negotiated address %s", chosenA)
	}

	// ReleasePeerIPv4 frees chosenA so session B can now claim it.
	negA.ReleasePeerIPv4()
	if _, exists := p.sessionAddrs.Load(negAID); exists {
		t.Error("ReleasePeerIPv4 left sessionAddrs record behind")
	}
	if !negB.AdmitPeerIPv4(chosenA) {
		t.Fatalf("session B could not claim %s after session A released it", chosenA)
	}

	// onSessionDown also releases session B's negotiated address.
	p.onSessionDown(&subevents.SessionDownPayload{
		Session: subscriber.Session{
			ID: negBID, AccessType: subscriber.AccessL2TP, TunnelID: 20, SessionID: 4,
		},
	})
	if _, exists := p.sessionAddrs.Load(negBID); exists {
		t.Error("onSessionDown left peer-negotiated sessionAddrs record behind")
	}
	if !negA.AdmitPeerIPv4(chosenA) {
		t.Fatalf("session A could not reclaim %s after session B went down", chosenA)
	}
}

// TestPoolPeerNegotiatedSurvivesConfigReload verifies that a live
// peer-negotiated sessionAddr carries cleanly across a pool reload, including
// reserving a bitmap slot when a reloaded range expands to cover it.
func TestPoolPeerNegotiatedSurvivesConfigReload(t *testing.T) {
	p := &poolPlugin{}
	tx := p.configTransaction()
	if err := tx.Configure(reloadPoolSection(t, map[string]any{
		"ipv4": reloadIPv4("10.0.0.10", "10.0.0.11"),
	})); err != nil {
		t.Fatal(err)
	}

	negID := subscriber.SessionID(subscriber.AccessL2TP, uint64(21), 1)
	l2tp.StoreSessionMetadata(negID, &l2tp.AuthMetadata{FramedIPNegotiated: true})
	t.Cleanup(func() { l2tp.ClearSessionMetadata(negID) })
	ans := p.handle(ppp.EventIPRequest{
		SubscriberID: negID, TunnelID: 21, SessionID: 1, Family: ppp.AddressFamilyIPv4,
	})
	chosen := netip.MustParseAddr("10.0.0.20")
	if !ans.AdmitPeerIPv4(chosen) {
		t.Fatalf("AdmitPeerIPv4(%s) = false, want true", chosen)
	}

	// Reload so the pool range now includes the live peer-negotiated address.
	if err := tx.Verify(reloadPoolSection(t, map[string]any{
		"ipv4": reloadIPv4("10.0.0.20", "10.0.0.21"),
	})); err != nil {
		t.Fatal(err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatal(err)
	}

	// Dynamic allocation must skip the carried peer-negotiated address.
	next := allocate(t, p, 21, 2, subscriber.AccessL2TP)
	if next == chosen {
		t.Fatalf("dynamic pool handed carried peer-negotiated address %s to another session", chosen)
	}
	if want := netip.MustParseAddr("10.0.0.21"); next != want {
		t.Fatalf("next pool address = %s, want %s", next, want)
	}

	// Releasing the peer-negotiated session frees the carried slot in the new pool.
	ans.ReleasePeerIPv4()
	reclaimed := allocate(t, p, 21, 3, subscriber.AccessL2TP)
	if reclaimed != chosen {
		t.Fatalf("reclaimed address = %s, want %s", reclaimed, chosen)
	}
}
