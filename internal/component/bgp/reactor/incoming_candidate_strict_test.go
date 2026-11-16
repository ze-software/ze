// Design: docs/architecture/behavior/peer-lifecycle.md -- independent incoming BFD lifecycle.
// Related: rfc4271_incoming_listener_test.go -- actual TCP listener and outgoing control.
// RFC: rfc/drafts/draft-ietf-idr-bgp-bfd-strict-mode.txt -- Sections 8.3.1 and 8.3.5.
package reactor

import (
	"bytes"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// TestIncomingCandidateBFDReleaseKeepsOriginalSession proves the actual
// incoming listener's private FSM receives BFD release, then transfers its
// existing socket/reader to the selected epoch without a second OPEN.
func TestIncomingCandidateBFDReleaseKeepsOriginalSession(t *testing.T) {
	r, peer, original, outgoing, timerClock := incomingListenerPeer(t, true, time.Minute, func(peer *Peer) {
		peer.settings.BFD = &BFDSettings{Enabled: true, Strict: true, HoldTime: 5}
		peer.settings.Capabilities = []capability.Capability{&capability.BFDStrictMode{}}
	})
	delayOpenAwait(t, original.holdsOpen)
	timerClock.Add(time.Minute)
	delayOpenReadType(t, outgoing, 1)
	delayOpenAwait(t, func() bool { return original.State() == fsm.StateOpenSent })
	incoming := incomingListenerDial(t, r)
	var candidate *incomingCandidate
	delayOpenAwait(t, func() bool {
		peer.mu.RLock()
		defer peer.mu.RUnlock()
		var ok bool
		candidate, ok = peer.pending.(*incomingCandidate)
		return ok && candidate.session.holdsOpen()
	})
	reader := candidate.session.bufReader
	body := openBodyWithBFDStrict(9)
	body[1], body[2] = 0xfd, 0xe9 // Configured remote AS 65001.
	frame := append(bytes.Repeat([]byte{0xff}, 16), 0, byte(19+len(body)), 1)
	if _, err := incoming.Write(append(frame, body...)); err != nil {
		t.Fatal(err)
	}
	delayOpenReadType(t, incoming, 1)
	delayOpenAwait(t, func() bool { return candidate.session.fsm.BfdSubState() != fsm.SubStateNone })
	if peer.currentSession() != original {
		t.Fatal("BFD-pending candidate replaced the useful OpenSent incumbent")
	}
	delayOpenNoBytes(t, incoming)
	peer.deliverBFDEvent(fsm.EventBfdUp)
	delayOpenReadType(t, incoming, 4)
	delayOpenWrite(t, incoming, &message.Keepalive{})
	delayOpenAwait(t, func() bool {
		return peer.currentSession() == candidate.session && candidate.session.State() == fsm.StateEstablished
	})
	if candidate.session.bufReader != reader {
		t.Fatal("promotion replaced the incoming buffered reader")
	}
	if got := peer.Stats().OpensSent; got != 2 {
		t.Fatalf("promotion emitted a duplicate local OPEN: total=%d, want original plus candidate", got)
	}
	if candidate.session.fsm.BfdSubState() != fsm.SubStateNone {
		t.Fatal("promoted candidate retained its BFD pending sub-state")
	}
}
