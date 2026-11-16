// Design: docs/architecture/l2tp/subscriber-session-model.md -- address-answer ownership tokens
// Related: register.go -- exhaustion refusal before either IPv4 allocation path

package l2tppool

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
)

// TestPoolAllocationIdentifiersDoNotWrap drives each IPv4 admission branch at
// the final token, then checks refusal, cached retry and exact-owner cleanup.
func TestPoolAllocationIdentifiersDoNotWrap(t *testing.T) {
	for _, tc := range []struct {
		name   string
		radius bool
		peer   netip.Addr
	}{
		{name: "dynamic", peer: netip.MustParseAddr("10.0.0.10")},
		{name: "radius", radius: true, peer: netip.MustParseAddr("10.0.0.11")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &poolPlugin{
				pool: newIPv4Pool(netip.MustParseAddr("10.0.0.1"),
					netip.MustParseAddr("10.0.0.10"), netip.MustParseAddr("10.0.0.11"),
					netip.Addr{}, netip.Addr{}),
				nextAllocationID: ^uint64(0) - 1,
			}
			firstID := subscriber.SessionID(subscriber.AccessL2TP, 49153, 1)
			secondID := subscriber.SessionID(subscriber.AccessL2TP, 49153, 2)
			if tc.radius {
				l2tp.StoreSessionMetadata(firstID, &l2tp.AuthMetadata{
					FramedIP: tc.peer,
				})
				l2tp.StoreSessionMetadata(secondID, &l2tp.AuthMetadata{
					FramedIP: netip.MustParseAddr("10.0.0.10"),
				})
				t.Cleanup(func() {
					l2tp.ClearSessionMetadata(firstID)
					l2tp.ClearSessionMetadata(secondID)
				})
			}
			firstReq := ppp.EventIPRequest{
				SubscriberID: firstID, TunnelID: 49153, SessionID: 1,
				Family: ppp.AddressFamilyIPv4,
			}
			first := p.handle(firstReq)
			if !first.Accept {
				t.Fatalf("final allocation refused: %s", first.Reason)
			}
			if first.Peer != tc.peer {
				t.Fatalf("final allocation peer = %s, want %s", first.Peer, tc.peer)
			}
			if first.AllocationID != ^uint64(0) {
				t.Fatalf("final allocation token = %d, want maximum", first.AllocationID)
			}
			original, present := p.sessionAddrs.Load(firstID)
			if !present {
				t.Fatal("final allocation has no owner record")
			}
			originalAllocation, ok := original.(sessionAddr)
			if !ok {
				t.Fatalf("owner record has type %T, want sessionAddr", original)
			}
			if originalAllocation.allocationID != first.AllocationID {
				t.Fatal("owner record does not retain the final answer token")
			}
			if _, used, _ := p.pool.stats(); used != 1 {
				t.Fatalf("final allocation reserved %d slots, want 1", used)
			}
			p.pool.mu.Lock()
			bitmap := p.pool.bitmap[0] // The two-address fixture fits in one word.
			p.pool.mu.Unlock()

			second := p.handle(ppp.EventIPRequest{
				SubscriberID: secondID, TunnelID: 49153, SessionID: 2,
				Family: ppp.AddressFamilyIPv4,
			})
			if second.Accept {
				t.Fatalf("exhausted identifier counter admitted another subscriber: %+v", second)
			}
			if second.Reason == "" {
				t.Fatal("identifier exhaustion refusal has no reason")
			}
			if second.AllocationID != 0 {
				t.Fatalf("refusal owns an allocation token: %d", second.AllocationID)
			}
			if p.nextAllocationID != ^uint64(0) {
				t.Fatalf("identifier counter wrapped to %d", p.nextAllocationID)
			}
			if _, used, _ := p.pool.stats(); used != 1 {
				t.Fatalf("refusal changed reserved slots to %d, want 1", used)
			}
			p.pool.mu.Lock()
			unchanged := p.pool.bitmap[0] == bitmap
			p.pool.mu.Unlock()
			if !unchanged {
				t.Fatal("identifier exhaustion changed the reservation bitmap")
			}
			if _, present := p.sessionAddrs.Load(secondID); present {
				t.Fatal("refused subscriber acquired an owner record")
			}
			retained, present := p.sessionAddrs.Load(firstID)
			if !present {
				t.Fatal("refusal deleted the original owner record")
			}
			retainedAllocation, ok := retained.(sessionAddr)
			if !ok {
				t.Fatalf("retained owner record has type %T, want sessionAddr", retained)
			}
			if retainedAllocation != originalAllocation {
				t.Fatal("refusal changed the original owner record")
			}

			repeated := p.handle(firstReq)
			if !repeated.Accept {
				t.Fatalf("existing owner retry refused at exhaustion: %s", repeated.Reason)
			}
			if repeated.Peer != first.Peer {
				t.Fatalf("retry peer = %s, want original %s", repeated.Peer, first.Peer)
			}
			if repeated.AllocationID != 0 {
				t.Fatalf("retry acquired cleanup ownership: %d", repeated.AllocationID)
			}

			p.onPoolAnswerUndelivered(&subevents.PoolAnswerUndeliveredPayload{
				SubscriberID: firstID, TunnelID: 49153, SessionID: 1,
				AllocationID: first.AllocationID,
			})
			if _, used, _ := p.pool.stats(); used != 0 {
				t.Fatalf("exact final-token release left %d slots reserved", used)
			}
			if _, present := p.sessionAddrs.Load(firstID); present {
				t.Fatal("exact final-token release left the owner record")
			}
			p.pool.mu.Lock()
			empty := p.pool.bitmap[0] == 0
			p.pool.mu.Unlock()
			if !empty {
				t.Fatal("exact final-token release left a reserved bitmap bit")
			}
		})
	}
}
