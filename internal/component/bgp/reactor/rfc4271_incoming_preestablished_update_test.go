// Design: docs/architecture/behavior/peer-lifecycle.md -- private incoming admission.
// Related: rfc4271_incoming_listener_test.go -- actual listener and lifecycle helpers.
package reactor

import (
	"bytes"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// preEstablishedUpdateProbe observes both the factory's immutable receive
// callback and the real delivery worker. A callback cannot disappear merely
// because the selected Session is not ready for a worker yet.
type preEstablishedUpdateProbe struct {
	callbacks atomic.Uint32
	delivered atomic.Uint32
	withdrawn atomic.Uint32
	fenced    atomic.Bool
	present   atomic.Bool
}

func (probe *preEstablishedUpdateProbe) configure(t *testing.T, peer *Peer) {
	t.Helper()
	acceptingPolicies(t, peer.settings)
	prior := peer.messageCallback
	peer.messageCallback = func(captured *Session, addr netip.Addr, kind msgtype.MessageType, body []byte, update *wireu.WireUpdate, ctx bgpctx.ContextID, direction rpc.MessageDirection, buffer BufHandle, meta map[string]any, source string, id uint64) bool {
		if kind == msgtype.TypeUPDATE && direction == rpc.DirectionReceived {
			probe.callbacks.Add(1)
		}
		return prior(captured, addr, kind, body, update, ctx, direction, buffer, meta, source, id)
	}
}

func (probe *preEstablishedUpdateProbe) install(r *Reactor) {
	r.setMessageReceiver(&testDeliveryReceiver{onReceived: func(_ plugin.PeerInfo, msg bgptypes.RawMessage) {
		if msg.Type != msgtype.TypeUPDATE {
			return
		}
		probe.delivered.Add(1)
		switch {
		case bytes.Equal(msg.RawBytes, []byte{0, 4, 24, 192, 0, 2, 0, 0}):
			probe.present.Store(false)
			probe.withdrawn.Add(1)
		case bytes.Equal(msg.RawBytes, []byte{0, 4, 24, 198, 51, 100, 0, 0}):
			// The worker's FIFO delivery of this legal incumbent message
			// fences any earlier erroneously queued candidate withdrawal.
			probe.fenced.Store(true)
		case len(msg.RawBytes) >= 4 && msg.RawBytes[0] == 0 && msg.RawBytes[1] == 0 && bytes.HasSuffix(msg.RawBytes, []byte{24, 192, 0, 2}):
			probe.present.Store(true)
		}
	}})
}

func preEstablishedUpdateMode(t *testing.T, coalesced string) {
	t.Helper()
	t.Setenv("ze.bgp.reactor.coalesce", coalesced)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

// TestIncomingBeforeOpenUpdateCannotPublishIncumbentWithdrawal proves a second
// actual listener socket cannot withdraw a useful Established session's route
// while its own OPEN is held. The legal incumbent withdrawal remains accepted.
// Route presence is modeled from actual worker deliveries; this is not an
// Adj-RIB-In or Loc-RIB assertion.
// RFC requirement: RFC4271-8.2.1-3 positive -- an admitted incoming TCP connection still in Active before OPEN rejects its UPDATE without entering the received-UPDATE callback or consumer queue, while the same Established sibling socket retains its modeled route and delivers the legal FIFO fence and legitimate withdrawal exactly once.
func TestIncomingBeforeOpenUpdateCannotPublishIncumbentWithdrawal(t *testing.T) {
	for _, coalesced := range []string{"false", "true"} {
		t.Run("coalesce="+coalesced, func(t *testing.T) {
			preEstablishedUpdateMode(t, coalesced)
			probe := &preEstablishedUpdateProbe{}
			r, peer, original, _, _ := incomingListenerPeer(t, false, time.Minute, func(peer *Peer) { probe.configure(t, peer) })
			probe.install(r)
			incumbent := incomingListenerDial(t, r)
			delayOpenWrite(t, incumbent, delayOpenRemote(0x02020202))
			delayOpenReadType(t, incumbent, 1)
			delayOpenReadType(t, incumbent, 4)
			delayOpenWrite(t, incumbent, &message.Keepalive{})
			delayOpenAwait(t, func() bool { return original.State() == fsm.StateEstablished })
			delayOpenWrite(t, incumbent, &message.Update{PathAttributes: firstASAttrs(2, 65001), NLRI: []byte{24, 192, 0, 2}})
			delayOpenAwait(t, probe.present.Load)
			if probe.callbacks.Load() != 1 || probe.delivered.Load() != 1 {
				t.Fatal("useful incumbent did not establish its one-route receive baseline")
			}
			incoming := incomingListenerDial(t, r)
			var candidate *incomingCandidate
			delayOpenAwait(t, func() bool {
				peer.mu.RLock()
				defer peer.mu.RUnlock()
				candidate, _ = peer.pending.(*incomingCandidate)
				return candidate != nil && candidate.session.holdsOpen()
			})
			if candidate.session.State() != fsm.StateActive {
				t.Fatalf("private held state %s, want Active", candidate.session.State())
			}
			delayOpenNoBytes(t, incoming)
			delayOpenWrite(t, incoming, &message.Update{WithdrawnRoutes: []byte{24, 192, 0, 2}})
			incomingReadPoolClosed(t, incoming)
			incomingCorrectionAwait(t, candidate.readDone, "rejected private UPDATE reader")
			delayOpenWrite(t, incumbent, &message.Update{WithdrawnRoutes: []byte{24, 198, 51, 100}})
			delayOpenAwait(t, probe.fenced.Load)
			if probe.callbacks.Load() != 2 || probe.delivered.Load() != 2 || probe.withdrawn.Load() != 0 || !probe.present.Load() {
				t.Fatalf("private UPDATE escaped admission: callbacks=%d delivered=%d target-withdrawals=%d route-present=%t", probe.callbacks.Load(), probe.delivered.Load(), probe.withdrawn.Load(), probe.present.Load())
			}
			if peer.currentSession() != original || original.State() != fsm.StateEstablished {
				t.Fatal("private UPDATE retired the useful incumbent")
			}
			delayOpenWrite(t, incumbent, &message.Update{WithdrawnRoutes: []byte{24, 192, 0, 2}})
			delayOpenAwait(t, func() bool { return probe.withdrawn.Load() == 1 })
			if probe.present.Load() || probe.callbacks.Load() != 3 || probe.delivered.Load() != 3 || original.State() != fsm.StateEstablished {
				t.Fatal("the same Established socket did not deliver its legitimate withdrawal once")
			}
		})
	}
}

// TestActiveDelayOpenUpdateNeverPublishes covers the ordinary selected read
// rail, whose worker must join before absence of consumer delivery is checked.
func TestActiveDelayOpenUpdateNeverPublishes(t *testing.T) {
	for _, coalesced := range []string{"false", "true"} {
		t.Run("coalesce="+coalesced, func(t *testing.T) {
			preEstablishedUpdateMode(t, coalesced)
			probe := &preEstablishedUpdateProbe{}
			r, peer, original, _, _ := incomingListenerPeer(t, false, time.Minute, func(peer *Peer) { probe.configure(t, peer) })
			probe.install(r)
			conn := incomingListenerDial(t, r)
			delayOpenAwait(t, original.holdsOpen)
			if original.State() != fsm.StateActive {
				t.Fatalf("ordinary held state %s, want Active", original.State())
			}
			delayOpenWrite(t, conn, &message.Update{WithdrawnRoutes: []byte{24, 192, 0, 2}})
			incomingReadPoolClosed(t, conn)
			delayOpenAwait(t, func() bool { return peer.currentSession() != original })
			if probe.callbacks.Load() != 0 || probe.delivered.Load() != 0 {
				t.Fatalf("ordinary pre-OPEN UPDATE published: callbacks=%d delivered=%d", probe.callbacks.Load(), probe.delivered.Load())
			}
		})
	}
}

// TestConnectDelayOpenUpdateNeverPublishes covers the ordinary dialing rail
// while its accepted TCP connection is still held in Connect by DelayOpen.
func TestConnectDelayOpenUpdateNeverPublishes(t *testing.T) {
	for _, coalesced := range []string{"false", "true"} {
		t.Run("coalesce="+coalesced, func(t *testing.T) {
			preEstablishedUpdateMode(t, coalesced)
			probe := &preEstablishedUpdateProbe{}
			r, peer, original, conn, _ := incomingListenerPeer(t, true, time.Minute, func(peer *Peer) { probe.configure(t, peer) })
			probe.install(r)
			delayOpenAwait(t, original.holdsOpen)
			if original.State() != fsm.StateConnect {
				t.Fatalf("ordinary held state %s, want Connect", original.State())
			}
			delayOpenWrite(t, conn, &message.Update{WithdrawnRoutes: []byte{24, 192, 0, 2}})
			incomingReadPoolClosed(t, conn)
			delayOpenAwait(t, func() bool { return peer.currentSession() != original })
			if probe.callbacks.Load() != 0 || probe.delivered.Load() != 0 {
				t.Fatalf("ordinary Connect UPDATE published: callbacks=%d delivered=%d", probe.callbacks.Load(), probe.delivered.Load())
			}
		})
	}
}

// TestIncomingStrictPendingUpdateNeverPublishes covers the separate private
// read rail after a valid OPEN, with strict BFD keeping this dialing peer's
// candidate in Connect. Its incoming TCP socket retains that configured mode.
func TestIncomingStrictPendingUpdateNeverPublishes(t *testing.T) {
	probe := &preEstablishedUpdateProbe{}
	r, peer, original, outgoing, clock := incomingListenerPeer(t, true, time.Minute, func(peer *Peer) {
		probe.configure(t, peer)
		peer.settings.BFD = &BFDSettings{Enabled: true, Strict: true, HoldTime: 5}
		peer.settings.Capabilities = []capability.Capability{&capability.BFDStrictMode{}}
	})
	probe.install(r)
	delayOpenAwait(t, original.holdsOpen)
	clock.Add(time.Minute)
	delayOpenReadType(t, outgoing, 1)
	delayOpenAwait(t, func() bool { return original.State() == fsm.StateOpenSent })
	incoming := incomingListenerDial(t, r)
	var candidate *incomingCandidate
	delayOpenAwait(t, func() bool {
		peer.mu.RLock()
		defer peer.mu.RUnlock()
		candidate, _ = peer.pending.(*incomingCandidate)
		return candidate != nil && candidate.session.holdsOpen()
	})
	body := openBodyWithBFDStrict(9)
	body[1], body[2] = 0xfd, 0xe9
	frame := append(bytes.Repeat([]byte{0xff}, 16), 0, byte(message.HeaderLen+len(body)), 1)
	if _, err := incoming.Write(append(frame, body...)); err != nil {
		t.Fatal(err)
	}
	delayOpenReadType(t, incoming, 1)
	delayOpenAwait(t, func() bool { return candidate.session.fsm.BfdSubState() != fsm.SubStateNone })
	if candidate.session.State() != fsm.StateConnect || peer.currentSession() != original {
		t.Fatalf("valid OPEN did not leave a private Connect strict-pending candidate: state=%s", candidate.session.State())
	}
	delayOpenNoBytes(t, incoming)
	delayOpenWrite(t, incoming, &message.Update{WithdrawnRoutes: []byte{24, 192, 0, 2}})
	incomingReadPoolClosed(t, incoming)
	incomingCorrectionAwait(t, candidate.readDone, "strict-pending UPDATE reader")
	if probe.callbacks.Load() != 0 || probe.delivered.Load() != 0 || peer.currentSession() != original || original.State() != fsm.StateOpenSent {
		t.Fatalf("strict-pending UPDATE escaped: callbacks=%d delivered=%d incumbent=%s", probe.callbacks.Load(), probe.delivered.Load(), original.State())
	}
}
