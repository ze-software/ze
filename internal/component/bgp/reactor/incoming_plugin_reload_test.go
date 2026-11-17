// Design: docs/architecture/behavior/peer-lifecycle.md -- frozen incoming OPEN proposals.
// Related: session_plugin_preparation_test.go -- the proposal capture boundary.
// Related: peer_settings_plugin_capability_test.go -- actual plugin-only reloads.
package reactor

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/test/sim"
)

// TestIncomingPluginReloadPreservesUsefulIncumbent holds the candidate's real
// constructor after its old plugin snapshot, then reloads while the original
// OPEN pair has no use for that capability. Only a stale, in-use incoming
// offer must be refused; unused or unchanged offers remain valid winners.
func TestIncomingPluginReloadPreservesUsefulIncumbent(t *testing.T) {
	for _, tc := range []struct {
		name    string
		changed bool
		used    bool
	}{
		{"changed-used", true, true},
		{"changed-unused", true, false},
		{"unchanged-used", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := NewPeerSettings(mustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101)
			settings.Connection = ConnectionBoth
			r, peer := newSwapTestReactor(t, settings, settings)
			cleanupPendingFixturePeer(t, peer)
			store := &pluginStore{}
			declaration := func(restart byte) plugin.InjectedCapability {
				return plugin.InjectedCapability{Code: 64, Value: []byte{0, restart}, Plugin: "reload-probe"}
			}
			store.set(declaration(120))
			captured, proceed := make(chan struct{}), make(chan struct{})
			release := sync.OnceFunc(func() { close(proceed) })
			t.Cleanup(release)
			var holdNext atomic.Bool
			r.pluginCapabilitySeam = func(selectors ...string) []plugin.InjectedCapability {
				current := store.get(selectors...)
				if holdNext.CompareAndSwap(true, false) {
					close(captured)
					<-proceed
				}
				return current
			}
			original := peer.newSession()
			original.SetClock(sim.NewFakeClock(time.Now()))
			peer.mu.Lock()
			peer.session = original
			peer.mu.Unlock()
			outgoing, _ := incomingCorrectionStart(t, original, false)
			processed := incomingCorrectionProcess(t, original, outgoing)
			delayOpenWrite(t, outgoing, delayOpenRemote(0x02020202))
			delayOpenReadType(t, outgoing, 4)
			if err := <-processed; err != nil {
				t.Fatal(err)
			}
			if original.State() != fsm.StateOpenConfirm {
				t.Fatal("incumbent did not reach useful OpenConfirm")
			}
			originalConn := original.Conn()
			incoming, local := collisionProofTCPPair(t)
			t.Cleanup(func() { release(); closeConnQuietly(incoming) })
			holdNext.Store(true)
			r.acceptOrReject(local, peer, nil)
			incomingCorrectionAwait(t, captured, "incoming proposal's old plugin snapshot")
			if tc.changed {
				store.set(declaration(90))
			}
			if err := (&reactorAPIAdapter{r: r}).Reload(); err != nil {
				t.Fatal(err)
			}
			r.mu.RLock()
			kept := r.peers[settings.PeerKey()] == peer
			r.mu.RUnlock()
			if !kept {
				t.Fatal("reload bounced an incumbent that never used the changed capability")
			}
			release()
			openPacket := delayOpenReadType(t, incoming, 1)
			open, err := message.UnpackOpen(openPacket[message.HeaderLen:])
			if err != nil {
				t.Fatal(err)
			}
			caps, err := capability.ParseFromOptionalParams(open.OptionalParams, open.ExtendedParams)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, cap := range caps {
				if cap.Code() != capability.CodeGracefulRestart {
					continue
				}
				wire := make([]byte, cap.Len())
				cap.WriteTo(wire, 0)
				if !bytes.Equal(wire, []byte{64, 2, 0, 120}) {
					t.Fatalf("incoming constructor changed its frozen OPEN: %x", wire)
				}
				found = true
			}
			if !found {
				t.Fatal("incoming OPEN did not carry the old captured GR declaration")
			}
			var candidate *incomingCandidate
			delayOpenAwait(t, func() bool {
				peer.mu.RLock()
				defer peer.mu.RUnlock()
				var ok bool
				candidate, ok = peer.pending.(*incomingCandidate)
				return ok
			})
			remote := delayOpenRemote(0x03030303)
			if tc.used {
				remote.OptionalParams = []byte{2, 4, 64, 2, 0, 90}
			}
			delayOpenWrite(t, incoming, remote)
			if tc.changed && tc.used {
				delayOpenAwait(t, func() bool { return original.Conn() == nil || candidate.session.Conn() == nil })
				if original.Conn() != originalConn {
					t.Fatal("stale incoming offer retired the useful incumbent")
				}
				packet := delayOpenReadType(t, incoming, 3)
				if !bytes.Equal(packet[message.HeaderLen:], []byte{6, 5}) {
					t.Fatalf("stale incoming refusal=%x, want 0605", packet[message.HeaderLen:])
				}
				var octet [1]byte
				if n, err := incoming.Read(octet[:]); n != 0 || !errors.Is(err, io.EOF) {
					t.Fatalf("stale incoming socket remains open: bytes=%d error=%v", n, err)
				}
				incomingCorrectionAwait(t, candidate.readDone, "stale candidate's joined retirement")
				incomingCorrectionConfirm(t, original, outgoing)
				fresh, freshLocal := collisionProofTCPPair(t)
				t.Cleanup(func() { closeConnQuietly(fresh) })
				r.acceptOrReject(freshLocal, peer, nil)
				delayOpenReadType(t, fresh, 1)
				delayOpenWrite(t, fresh, remote)
				delayOpenReadCollisionClose(t, fresh)
				if peer.currentSession() != original {
					t.Fatal("fresh collision displaced the healthy incumbent")
				}
				return
			}
			delayOpenAwait(t, func() bool { return original.Conn() == nil })
			peer.mu.RLock()
			winner := inboundCandidate(peer.inbound)
			peer.mu.RUnlock()
			if winner == nil {
				t.Fatal("an unchanged or unused incoming offer was not permitted to win")
			}
			if winner.conn != local {
				t.Fatal("the useful incoming winner lost its original connection")
			}
		})
	}
}
