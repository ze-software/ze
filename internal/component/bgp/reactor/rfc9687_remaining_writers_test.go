package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
)

// TestRFC9687RemainingWritersRestartSendHold covers writers sharing the emission
// accounting in flushWrites. Each sends at 0.6 SendHoldTime and survives to 1.2
// SendHoldTime; the exact new deadline also distinguishes a restart from a stopped timer.
// MUTATION: remove the emitted-message reset in flushWrites: the deadline stays old.
// RFC requirement: RFC9687-4.3-8 positive -- SendAnnounce, SendUpdateHeld, raw and parsed fwdBatchHandler batches, and flushFwdDirty restart the live session's SendHoldTimer and survive 1.2 SendHoldTime with a send in the middle.
func TestRFC9687RemainingWritersRestartSendHold(t *testing.T) {
	for _, name := range []string{"announce", "held", "batch-raw", "batch-parsed", "dirty-flush"} {
		t.Run(name, func(t *testing.T) {
			p := rfc9687Established(t, 90)
			s := p.session
			var item fwdItem
			switch name {
			case "batch-raw", "batch-parsed":
				// Establish source ownership before advancing the timer under test.
				item = rfc9687ForwardItem(t, s)
			}
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
				// RFC 9687 Section 4.3: only this post-advance send can restart the deadline.
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

// rfc9687ForwardItem advertises the path through the real writer before returning
// the captured ownership needed by the later batch withdrawal. Callers MUST run
// this setup before advancing the fake clock, so setup cannot satisfy the timer
// restart assertion on behalf of the writer under test.
func rfc9687ForwardItem(t *testing.T, session *Session) fwdItem {
	t.Helper()
	peer := NewPeer(session.settings)
	peer.session = session
	peer.state.Store(int32(PeerStateEstablished))
	peer.setEncodingContexts(session.Negotiated())
	t.Cleanup(peer.clearEncodingContexts)

	source, _ := newAnnouncePeer(t, "192.0.2.3")
	source.session.localOpen = &message.Open{MyAS: 65000, HoldTime: 90}
	source.session.peerOpen = &message.Open{MyAS: 65001, HoldTime: 90}
	source.session.negotiateWith(nil, nil)
	source.setEncodingContexts(source.session.Negotiated())
	t.Cleanup(source.clearEncodingContexts)

	body := makeUpdateBody(nil,
		[]byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 3, 4, 192, 0, 2, 3},
		[]byte{24, 10, 0, 0})
	// RFC 4271 Section 4.3: advertise the route this source will later withdraw.
	if err := ownershipWriterForward(t, peer, source, body, false); err != nil {
		t.Fatal(err)
	}
	return fwdItem{
		peer: peer, session: session, authority: adjOutForwarded,
		receivedPeer: source, receivedGeneration: source.forwardGeneration.Load(),
		sourcePeerStr: source.addrString,
	}
}
