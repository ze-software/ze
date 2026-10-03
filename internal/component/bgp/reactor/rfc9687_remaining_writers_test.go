package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
)

// TestRFC9687RemainingWritersRestartSendHold covers writers with their own reset
// calls. Each sends at 0.6 SendHoldTime and survives to 1.2 SendHoldTime; the
// exact new deadline also distinguishes a restart from a stopped timer.
// MUTATION: remove resetSendHoldTimer from any writer: its deadline stays old.
// RFC requirement: RFC9687-4.3-8 positive -- SendAnnounce, SendUpdateHeld, raw and parsed fwdBatchHandler batches, and flushFwdDirty restart the live session's SendHoldTimer and survive 1.2 SendHoldTime with a send in the middle.
func TestRFC9687RemainingWritersRestartSendHold(t *testing.T) {
	for _, name := range []string{"announce", "held", "batch-raw", "batch-parsed", "dirty-flush"} {
		t.Run(name, func(t *testing.T) {
			p := rfc9687Established(t, 90)
			s := p.session
			p.clock.Add(6 * rfc9687SendHold / 10)
			var err error
			switch name {
			case "announce":
				route := bgptypes.RouteSpec{Prefix: netip.MustParsePrefix("198.51.100.0/24"), NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.99"))}
				err = s.SendAnnounce(route, netip.Addr{}, 65001, false, true, false)
			case "held":
				s.HoldWrites()
				err = s.SendUpdateHeld(&message.Update{WithdrawnRoutes: []byte{24, 10, 0, 0}})
				s.releaseWrites()
			case "batch-raw", "batch-parsed":
				peer := NewPeer(s.settings)
				peer.session = s
				item := fwdItem{peer: peer}
				if name == "batch-raw" {
					item.rawBodies = [][]byte{{0, 4, 24, 10, 0, 0, 0, 0}}
				} else {
					item.updates = []*message.Update{{WithdrawnRoutes: []byte{24, 10, 0, 0}}}
				}
				fwdBatchHandler(fwdKey{}, []fwdItem{item})
			case "dirty-flush":
				s.writeMu.Lock()
				err = s.writeRawUpdateBody([]byte{0, 4, 24, 10, 0, 0, 0, 0})
				s.writeMu.Unlock()
				source := NewSession(s.settings)
				source.appendFwdDirty(s)
				source.flushFwdDirty()
				if len(source.fwdDirty) != 0 {
					t.Fatal("the destination was not flushed")
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			want := p.clock.Now().Add(rfc9687SendHold).UnixNano()
			if got := s.sendHoldDeadline.Load(); got != want {
				t.Fatalf("send-hold deadline = %d, want %d", got, want)
			}
			p.clock.Add(6 * rfc9687SendHold / 10)
			select {
			case err := <-p.runResult:
				t.Fatalf("session ended despite successful send: %v", err)
			case <-time.After(250 * time.Millisecond):
				// Bound the absence observation after the old deadline fired.
			}
			if !p.armed() || s.State() != fsm.StateEstablished {
				t.Fatal("live writer failed to retain Established and the armed timer")
			}
		})
	}
}
