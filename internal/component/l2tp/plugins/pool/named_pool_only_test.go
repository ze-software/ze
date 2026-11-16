// Design: docs/guide/l2tp.md -- named pool selection without a default IPv4 pool

package l2tppool

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
)

// TestPoolNamedOnlyPoolSelection configures only a named pool and requests it
// through RADIUS Framed-Pool metadata. A request that selects no pool remains
// refused, while the selected pool supplies an address, gateway and DNS.
func TestPoolNamedOnlyPoolSelection(t *testing.T) {
	for _, tc := range []struct {
		name     string
		selected string
		accept   bool
	}{
		{name: "selected", selected: "gold", accept: true},
		{name: "unselected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := radiusTestPool()
			p.pool = nil
			if tc.selected != "" {
				l2tp.StoreSessionMetadata(subscriber.SessionID(subscriber.AccessL2TP, uint64(radiusTestTunnel), 60), &l2tp.AuthMetadata{FramedPool: tc.selected})
				t.Cleanup(func() {
					l2tp.ClearSessionMetadata(subscriber.SessionID(subscriber.AccessL2TP, uint64(radiusTestTunnel), 60))
				})
			}
			answer := p.handle(ppp.EventIPRequest{
				SubscriberID: subscriber.SessionID(subscriber.AccessL2TP, uint64(radiusTestTunnel), 60),
				TunnelID:     radiusTestTunnel, SessionID: 60, Family: ppp.AddressFamilyIPv4,
			})
			if answer.Accept != tc.accept {
				t.Fatalf("named-only request accepted = %t, want %t; reason: %s", answer.Accept, tc.accept, answer.Reason)
			}
			if !tc.accept {
				if answer.Reason == "" {
					t.Fatal("unselected named-only request returned no refusal reason")
				}
				return
			}
			if answer.Peer != netip.MustParseAddr("10.0.0.20") {
				t.Fatalf("named-only address = %s, want 10.0.0.20", answer.Peer)
			}
			if answer.Local != p.namedPools["gold"].gateway {
				t.Fatalf("named-only gateway = %s, want %s", answer.Local, p.namedPools["gold"].gateway)
			}
			if answer.DNSPrimary != p.namedPools["gold"].dnsPrimary {
				t.Fatalf("named-only DNS = %s, want %s", answer.DNSPrimary, p.namedPools["gold"].dnsPrimary)
			}
		})
	}
}
