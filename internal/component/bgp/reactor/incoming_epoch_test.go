// Design: docs/architecture/behavior/peer-lifecycle.md -- selected negotiation ownership.
// Related: held_wiring_test.go -- retained generation parsing and Peer-owned hold.
// RFC 4724 Section 4.2 -- see rfc/short/rfc4724.md.
package reactor

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// TestIncomingPrivateOpenKeepsSelectedGeneration negotiates a real private
// candidate beside an OpenSent incumbent carrying a previous held generation.
// The candidate cannot erase that hold before selection; whichever connection
// subsequently becomes useful must begin exactly one new selected generation.
func TestIncomingPrivateOpenKeepsSelectedGeneration(t *testing.T) {
	for _, selectCandidate := range []bool{false, true} {
		name := "failed candidate leaves original useful"
		if selectCandidate {
			name = "selected candidate begins new generation"
		}
		t.Run(name, func(t *testing.T) {
			r, peer, original, outgoing, _ := incomingListenerPeer(t, true, 0)
			delayOpenReadType(t, outgoing, 1)
			// Seed a previous genuine parser-produced generation using the same
			// Peer-owned hold that survives reconnect. No FSM state is fabricated.
			previous := heldWiringSession(t, peer, true)
			t.Cleanup(previous.stopSessionTimers)
			wire := mupWireNLRI(3, t1stBody(2, 0x00)...)
			holdT1STGeneration(t, previous, mupFam, wire)
			peer.typedNLRIHeld.mu.Lock()
			before := peer.typedNLRIHeld.generation
			peer.typedNLRIHeld.mu.Unlock()
			incoming := incomingListenerDial(t, r)
			delayOpenReadType(t, incoming, 1)
			delayOpenWrite(t, incoming, delayOpenRemote(0x03030303))
			delayOpenReadType(t, incoming, 4)
			if peer.currentSession() != original || original.State() != fsm.StateOpenSent {
				t.Fatal("private OPEN selected or retired the incumbent")
			}
			peer.typedNLRIHeld.mu.Lock()
			generation := peer.typedNLRIHeld.generation
			count := len(peer.typedNLRIHeld.routes)
			var retained []byte
			for _, route := range peer.typedNLRIHeld.routes {
				retained = route.nlri
			}
			peer.typedNLRIHeld.mu.Unlock()
			if generation != before || count != 1 || !bytes.Equal(retained, wire) {
				t.Fatalf("private negotiation changed selected hold: generation=%d want=%d routes=%d NLRI=%x", generation, before, count, retained)
			}
			if selectCandidate {
				delayOpenWrite(t, incoming, &message.Keepalive{})
				refusal := delayOpenReadType(t, outgoing, 3)
				if !bytes.Equal(refusal[message.HeaderLen:], []byte{6, 7}) {
					t.Fatalf("retired incumbent collision notification=%x", refusal)
				}
				incomingIngressEOF(t, outgoing)
				delayOpenAwait(t, func() bool {
					return peer.currentSession() != original && peer.SessionState() == fsm.StateEstablished
				})
			} else {
				closeConnQuietly(incoming)
				delayOpenAwait(t, func() bool {
					peer.mu.RLock()
					defer peer.mu.RUnlock()
					return peer.pending == nil
				})
				delayOpenWrite(t, outgoing, delayOpenRemote(0x02020202))
				delayOpenReadType(t, outgoing, 4)
				delayOpenWrite(t, outgoing, &message.Keepalive{})
				delayOpenAwait(t, func() bool { return original.State() == fsm.StateEstablished })
				if peer.currentSession() != original {
					t.Fatal("failed candidate replaced the original connection")
				}
			}
			peer.typedNLRIHeld.mu.Lock()
			generation = peer.typedNLRIHeld.generation
			count = len(peer.typedNLRIHeld.routes)
			peer.typedNLRIHeld.mu.Unlock()
			if generation != before+1 || count != 0 {
				t.Fatalf("selected no-GR negotiation did not begin its generation: generation=%d want=%d routes=%d", generation, before+1, count)
			}
		})
	}
}
