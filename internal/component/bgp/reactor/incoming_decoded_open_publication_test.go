// Design: docs/architecture/behavior/peer-lifecycle.md -- selected decoded OPEN publication.
// Related: rfc4271_incoming_listener_test.go -- actual listener and connection ownership.
// RFC 4271 Sections 6.8 and 8.2.1 -- unselected connections retain independent learned state.
package reactor

import (
	"bytes"
	"net/netip"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type incomingDecodedOpenObservation struct {
	direction rpc.MessageDirection
	body      []byte
}

type incomingDecodedOpenObservations struct {
	mu     sync.Mutex
	events []incomingDecodedOpenObservation
}

func (observed *incomingDecodedOpenObservations) attach(peer *Peer) {
	prior := peer.messageCallback
	peer.messageCallback = func(captured *Session, addr netip.Addr, kind msgtype.MessageType, body []byte,
		update *wireu.WireUpdate, ctx bgpctx.ContextID, direction rpc.MessageDirection,
		buffer BufHandle, meta map[string]any, source string, id uint64) bool {
		if kind == msgtype.TypeOPEN {
			observed.mu.Lock()
			observed.events = append(observed.events, incomingDecodedOpenObservation{direction: direction, body: bytes.Clone(body)})
			observed.mu.Unlock()
		}
		return prior(captured, addr, kind, body, update, ctx, direction, buffer, meta, source, id)
	}
}

func (observed *incomingDecodedOpenObservations) snapshot() []incomingDecodedOpenObservation {
	observed.mu.Lock()
	defer observed.mu.Unlock()
	return append([]incomingDecodedOpenObservation(nil), observed.events...)
}

func (observed *incomingDecodedOpenObservations) clear() {
	observed.mu.Lock()
	defer observed.mu.Unlock()
	observed.events = nil
}

// TestIncomingLosingOpenPublishesNoLearnedMessages observes both decoded OPEN
// directions on actual sockets, while physical OPEN counters still include the
// losing or invalid exchange. The original Established connection survives.
func TestIncomingLosingOpenPublishesNoLearnedMessages(t *testing.T) {
	for _, rejected := range []bool{false, true} {
		name := "collision"
		if rejected {
			name = "invalid-peer-as"
		}
		t.Run(name, func(t *testing.T) {
			observed := &incomingDecodedOpenObservations{}
			r, peer, original, _, _ := incomingListenerPeer(t, false, 0, observed.attach)
			incumbent := incomingListenerDial(t, r)
			delayOpenReadType(t, incumbent, 1)
			incomingListenerEstablish(t, peer, incumbent)
			delayOpenAwait(t, func() bool { return len(observed.snapshot()) == 2 })
			observed.clear()
			incoming := incomingListenerDial(t, r)
			delayOpenReadType(t, incoming, 1)
			remote := delayOpenRemote(0x09090909)
			code, sub := byte(6), byte(7)
			if rejected {
				remote.MyAS = 65002
				code, sub = 2, 2
			}
			delayOpenWrite(t, incoming, remote)
			packet := delayOpenReadType(t, incoming, 3)
			body := packet[message.HeaderLen:]
			if !bytes.Equal(body, []byte{code, sub}) {
				t.Fatalf("private rejection %x, want %d/%d", body, code, sub)
			}
			incomingReadPoolClosed(t, incoming)
			delayOpenAwait(t, func() bool {
				peer.mu.RLock()
				defer peer.mu.RUnlock()
				return peer.pending == nil
			})
			if got := observed.snapshot(); len(got) != 0 {
				t.Fatalf("unselected OPEN published learned messages: %+v", got)
			}
			if peer.currentSession() != original {
				t.Fatal("private OPEN replaced the original Session")
			}
			if original.State() != fsm.StateEstablished {
				t.Fatalf("original Session is %s, want Established", original.State())
			}
			if got := peer.Stats().OpensSent; got != 2 {
				t.Fatalf("physical local OPEN counter=%d, want both actual exchanges", got)
			}
			if got := peer.Stats().OpensReceived; got != 2 {
				t.Fatalf("physical remote OPEN counter=%d, want both actual exchanges", got)
			}
		})
	}
}

// TestIncomingWinningOpenPublishesItsObservedPairOnce contrasts the private
// handshake interval with selection. A selected candidate publishes its exact
// actual local and remote OPEN bodies once, without counting another wire send.
func TestIncomingWinningOpenPublishesItsObservedPairOnce(t *testing.T) {
	observed := &incomingDecodedOpenObservations{}
	r, peer, original, outgoing, _ := incomingListenerPeer(t, true, 0, observed.attach)
	delayOpenReadType(t, outgoing, 1)
	delayOpenAwait(t, func() bool { return len(observed.snapshot()) == 1 })
	observed.clear()
	incoming := incomingListenerDial(t, r)
	localPacket := delayOpenReadType(t, incoming, 1)
	local := localPacket[message.HeaderLen:]
	var candidate *incomingCandidate
	delayOpenAwait(t, func() bool {
		peer.mu.RLock()
		defer peer.mu.RUnlock()
		candidate, _ = peer.pending.(*incomingCandidate)
		return candidate != nil
	})
	remote := delayOpenRemote(0x09090909)
	remotePacket := message.PackTo(remote, nil)
	delayOpenWrite(t, incoming, remote)
	delayOpenReadType(t, incoming, 4)
	if peer.currentSession() != original {
		t.Fatal("candidate was selected before its private handshake completed")
	}
	if got := observed.snapshot(); len(got) != 0 {
		t.Fatalf("private handshake published OPEN before selection: %+v", got)
	}
	delayOpenWrite(t, incoming, &message.Keepalive{})
	delayOpenAwait(t, func() bool {
		return peer.currentSession() == candidate.session && candidate.session.State() == fsm.StateEstablished
	})
	delayOpenAwait(t, func() bool { return len(observed.snapshot()) >= 2 })
	events := observed.snapshot()
	if len(events) != 2 {
		t.Fatalf("selected OPEN publication count=%d, want exact pair", len(events))
	}
	if events[0].direction != rpc.DirectionSent {
		t.Fatalf("first actual OPEN direction=%v, want sent", events[0].direction)
	}
	if !bytes.Equal(events[0].body, local) {
		t.Fatalf("selected local OPEN=%x, want actual %x", events[0].body, local)
	}
	if events[1].direction != rpc.DirectionReceived {
		t.Fatalf("second actual OPEN direction=%v, want received", events[1].direction)
	}
	if !bytes.Equal(events[1].body, remotePacket[message.HeaderLen:]) {
		t.Fatalf("selected remote OPEN=%x, want actual %x", events[1].body, remotePacket[message.HeaderLen:])
	}
	if got := peer.Stats().OpensSent; got != 2 {
		t.Fatalf("selection counted physical OPEN again: %d", got)
	}
	if got := peer.Stats().OpensReceived; got != 1 {
		t.Fatalf("selection counted received OPEN again: %d", got)
	}
}
