// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- IPv6CP Restart counter renewal.

package pppoeclient

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestIPv6CPLateNakRenewsTheSpentBudget spends nine unanswered requests,
// accepts a matching Nak in each irc state, then observes a fresh silent budget.
// Removing the RCN reset stops IPv6CP on the very next configured expiry.
func TestIPv6CPLateNakRenewsTheSpentBudget(t *testing.T) {
	for _, state := range []ppp.LCPState{ppp.LCPStateReqSent, ppp.LCPStateAckSent} {
		t.Run(state.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				c := timedIPv6CP(t, 500*time.Millisecond)
				wire := &ipv6cpWire{}
				var buf [ppp.MaxFrameBufLen]byte
				if err := c.start(wire, buf[:]); err != nil {
					t.Fatal(err)
				}
				for sent := 1; sent < 9; sent++ {
					<-c.restart.C
					if err := c.tick(wire, buf[:], slogutil.DiscardLogger()); err != nil {
						t.Fatal(err)
					}
				}
				if state == ppp.LCPStateAckSent {
					peer := ppp.IPv6CPInterfaceID{2, 4, 6, 8, 10, 12, 14, 16}
					if _, err := c.handle(wire, buf[:], ppp.LCPPacket{Code: ppp.LCPConfigureRequest, Identifier: 90, Data: ipv6cpOption(peer)}, slogutil.DiscardLogger()); err != nil {
						t.Fatal(err)
					}
				}
				if c.state != state {
					t.Fatalf("state before Nak = %v, want %v", c.state, state)
				}
				began := time.Now()
				suggestion := ppp.IPv6CPInterfaceID{2, 1, 2, 3, 4, 5, 6, 7}
				if _, err := c.handle(wire, buf[:], ppp.LCPPacket{Code: ppp.LCPConfigureNak, Identifier: c.identifier, Data: ipv6cpOption(suggestion)}, slogutil.DiscardLogger()); err != nil {
					t.Fatal(err)
				}
				if c.requests != 1 {
					t.Fatalf("requests after valid Nak = %d, want 1 of fresh budget", c.requests)
				}
				for expiry := 1; expiry <= ipv6cpConfigureMax; expiry++ {
					<-c.restart.C
					if err := c.tick(wire, buf[:], slogutil.DiscardLogger()); err != nil {
						t.Fatal(err)
					}
					if expiry < ipv6cpConfigureMax {
						if c.state == ppp.LCPStateStopped {
							t.Fatalf("fresh budget stopped at expiry %d", expiry)
						}
					}
				}
				if c.state != ppp.LCPStateStopped {
					t.Fatalf("state = %v, want Stopped", c.state)
				}
				if elapsed := time.Since(began); elapsed != 5*time.Second {
					t.Fatalf("renewed budget lasted %s, want 5s", elapsed)
				}
				requests := 0
				for _, frame := range wire.frames {
					protocol, payload, _, err := ppp.ParseFrame(frame)
					if err != nil {
						t.Fatal(err)
					}
					if protocol != ppp.ProtoIPv6CP {
						t.Fatalf("wire protocol = %#x, want IPv6CP", protocol)
					}
					packet, err := ppp.ParseLCPPacket(payload)
					if err != nil {
						t.Fatal(err)
					}
					if packet.Code == ppp.LCPConfigureRequest {
						requests++
					}
				}
				if requests != 19 {
					t.Fatalf("wire Configure-Requests = %d, want spent nine plus fresh ten", requests)
				}
			})
		})
	}
}

// TestIPv6CPNakInAckReceivedKeepsTheCounter pins the RFC's contrasting scr
// edge: Ack-Rcvd does not carry irc, so a retained count must not be cleared.
func TestIPv6CPNakInAckReceivedKeepsTheCounter(t *testing.T) {
	c := timedIPv6CP(t, time.Second)
	wire := &ipv6cpWire{}
	var buf [ppp.MaxFrameBufLen]byte
	if err := c.start(wire, buf[:]); err != nil {
		t.Fatal(err)
	}
	c.state = ppp.LCPStateAckRcvd
	c.requests = 4
	suggestion := ppp.IPv6CPInterfaceID{2, 1, 2, 3, 4, 5, 6, 7}
	if _, err := c.handle(wire, buf[:], ppp.LCPPacket{Code: ppp.LCPConfigureNak, Identifier: c.identifier, Data: ipv6cpOption(suggestion)}, slogutil.DiscardLogger()); err != nil {
		t.Fatal(err)
	}
	if c.requests != 5 {
		t.Fatalf("Ack-Rcvd requests = %d, want retained four plus scr", c.requests)
	}
	if c.state != ppp.LCPStateReqSent {
		t.Fatalf("state = %v, want Req-Sent", c.state)
	}
}
