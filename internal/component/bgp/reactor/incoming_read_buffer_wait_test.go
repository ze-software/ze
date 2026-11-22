// Design: docs/architecture/forward-congestion-pool.md -- read-pool exhaustion is backpressure.
// Related: incoming_candidate.go -- private incoming reader and cancellation bridge.
// Related: rfc4271_incoming_listener_test.go -- actual listener and OpenSent incumbent.
package reactor

import (
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// incomingReadPoolProbe fences the first candidate Get after the fixture has
// taken every free handle. The production pool already exposes this probe;
// it is installed before any session starts, not changed under a reader.
type incomingReadPoolProbe struct {
	armed atomic.Bool
	once  sync.Once
	read  chan struct{}
}

func (probe *incomingReadPoolProbe) get() {
	if probe.armed.Load() {
		probe.once.Do(func() { close(probe.read) })
	}
}

// incomingReadPools gives this serialized pool test fresh, ordinary muxes.
// The Reactor is constructed afterwards: New installs their combined budget.
// Cleanup restores the original muxes only after all later socket, held-handle
// and Reactor Stop/Wait cleanups. No pool reset is used to clear a product leak.
func incomingReadPools(t *testing.T) (*incomingReadPoolProbe, *incomingReadPoolProbe) {
	t.Helper()
	bufMuxGlobalMu.Lock()
	oldStd, oldExt := bufMuxStd, bufMuxExt
	standard := &incomingReadPoolProbe{read: make(chan struct{})}
	extended := &incomingReadPoolProbe{read: make(chan struct{})}
	bufMuxStd = newProbedPool(message.MaxMsgLen, bufMuxBlockSize)
	bufMuxExt = newProbedPool(message.ExtMsgLen, bufMuxBlockSize)
	bufMuxStd.setProbe(standard.get)
	bufMuxExt.setProbe(extended.get)
	t.Cleanup(func() {
		_, stdUsed := bufMuxStd.Stats()
		_, extUsed := bufMuxExt.Stats()
		bufMuxStd, bufMuxExt = oldStd, oldExt
		bufMuxGlobalMu.Unlock()
		if stdUsed != 0 || extUsed != 0 {
			t.Errorf("incoming fixture retained read handles: standard=%d extended=%d", stdUsed, extUsed)
		}
	})
	return standard, extended
}

func incomingReadPoolExhaustExtended(t *testing.T) func() {
	t.Helper()
	// Prime one standard 4K block before capping the shared budget so the
	// incoming candidate can still read its standard-size OPEN before blocking
	// on the exhausted extended pool for the post-OPEN KEEPALIVE.
	std := bufMuxStd.Get()
	if std.Buf == nil {
		t.Fatal("standard read pool refused its initial handle")
	}
	bufMuxStd.Return(std)
	budget := bufMuxExt.mux.budget
	previous := budget.maxBytes.Load()
	held := []BufHandle{bufMuxExt.Get()}
	if held[0].Buf == nil {
		t.Fatal("extended read pool refused its first handle")
	}
	updateBufMuxBudget(budget.allocatedBytes())
	for {
		handle := bufMuxExt.Get()
		if handle.Buf == nil {
			break
		}
		held = append(held, handle)
	}
	release := sync.OnceFunc(func() {
		for _, handle := range held {
			bufMuxExt.Return(handle)
		}
		updateBufMuxBudget(previous)
	})
	t.Cleanup(release)
	return release
}

func incomingReadPoolCandidate(t *testing.T, peer *Peer) *incomingCandidate {
	t.Helper()
	peer.mu.RLock()
	defer peer.mu.RUnlock()
	candidate, ok := peer.pending.(*incomingCandidate)
	if !ok {
		t.Fatalf("incoming admission is %T, want the same live candidate while buffers are held", peer.pending)
	}
	return candidate
}

func incomingReadPoolClosed(t *testing.T, conn net.Conn) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var octet [1]byte
	count, err := conn.Read(octet[:])
	if count != 0 || !errors.Is(err, io.EOF) {
		t.Fatalf("retired candidate produced %d bytes, error %v; want EOF", count, err)
	}
}

// TestIncomingCandidateReadPoolBackpressure drives both private incoming read
// rails over the actual TCP listener. Standard-pool exhaustion stops the first
// OPEN; negotiated Extended Message exhaustion stops the following KEEPALIVE
// deterministically, without racing a handle already taken for that read.
// Returning the handles must establish the same candidate and original reader,
// with one OPEN exchange. The outgoing OpenSent session remains selected until
// that exchange succeeds; a reset or replay cannot pass.
func TestIncomingCandidateReadPoolBackpressure(t *testing.T) {
	for _, afterOpen := range []bool{false, true} {
		name := "before-OPEN"
		if afterOpen {
			name = "after-OPEN"
		}
		t.Run(name, func(t *testing.T) {
			standard, extended := incomingReadPools(t)
			r, peer, original, outgoing, _ := incomingListenerPeer(t, true, 0, func(peer *Peer) {
				if afterOpen {
					peer.settings.Capabilities = append(peer.settings.Capabilities, &capability.ExtendedMessage{})
				}
			})
			delayOpenReadType(t, outgoing, 1)
			delayOpenAwait(t, func() bool { return original.State() == fsm.StateOpenSent })
			probe := standard
			var release func()
			if afterOpen {
				probe = extended
				release = incomingReadPoolExhaustExtended(t)
			} else {
				release = exhaustStdReadPool(t)
			}
			probe.armed.Store(true)
			incoming := incomingListenerDial(t, r)
			delayOpenReadType(t, incoming, 1)
			open := delayOpenRemote(0x02020202)
			if afterOpen {
				open.OptionalParams = []byte{2, 2, 6, 0}
			}
			delayOpenWrite(t, incoming, open)
			if afterOpen {
				delayOpenReadType(t, incoming, 4)
			}
			incomingCorrectionAwait(t, probe.read, "exhausted incoming pool read")
			delayOpenNoBytes(t, incoming)
			candidate := incomingReadPoolCandidate(t, peer)
			reader := candidate.session.bufReader
			if peer.currentSession() != original || original.State() != fsm.StateOpenSent {
				t.Fatal("pool pressure replaced the outgoing OpenSent incumbent")
			}
			release()
			if !afterOpen {
				delayOpenReadType(t, incoming, 4)
			}
			delayOpenWrite(t, incoming, &message.Keepalive{})
			delayOpenAwait(t, func() bool {
				return peer.currentSession() == candidate.session && candidate.session.State() == fsm.StateEstablished
			})
			if candidate.session.bufReader != reader {
				t.Fatal("buffer resumption replaced the candidate's reader")
			}
			if stats := peer.Stats(); stats.OpensSent != 2 || stats.OpensReceived != 1 {
				t.Fatalf("buffer resumption repeated OPEN: sent=%d received=%d", stats.OpensSent, stats.OpensReceived)
			}
		})
	}
}

// TestIncomingCandidateReadPoolWaitEnds proves held buffers cannot extend the
// already-installed first-OPEN deadline or prevent peer cancellation. The short
// configured deadline exercises the same absolute ReceiveHoldTime budget whose
// zero-value fallback is 90 seconds; the ordinary OpenSent hold is much longer.
func TestIncomingCandidateReadPoolWaitEnds(t *testing.T) {
	for _, cancelPeer := range []bool{false, true} {
		name := "first-OPEN-deadline"
		if cancelPeer {
			name = "peer-cancel"
		}
		t.Run(name, func(t *testing.T) {
			standard, _ := incomingReadPools(t)
			r, peer, original, outgoing, _ := incomingListenerPeer(t, true, 0, func(peer *Peer) {
				if !cancelPeer {
					peer.settings.ReceiveHoldTime = 3 * time.Second
				}
			})
			delayOpenReadType(t, outgoing, 1)
			delayOpenAwait(t, func() bool { return original.State() == fsm.StateOpenSent })
			exhaustStdReadPool(t)
			standard.armed.Store(true)
			incoming := incomingListenerDial(t, r)
			delayOpenReadType(t, incoming, 1)
			incomingCorrectionAwait(t, standard.read, "exhausted incoming pool read")
			delayOpenNoBytes(t, incoming)
			candidate := incomingReadPoolCandidate(t, peer)
			if cancelPeer {
				peer.Stop()
			}
			incomingReadPoolClosed(t, incoming)
			incomingCorrectionAwait(t, candidate.readDone, "incoming pool wait retirement")
			peer.mu.RLock()
			pending := peer.pending
			peer.mu.RUnlock()
			if pending != nil {
				t.Fatalf("retired pool wait retained admission %T", pending)
			}
			if !cancelPeer && (peer.currentSession() != original || original.State() != fsm.StateOpenSent) {
				t.Fatal("candidate read deadline retired the OpenSent incumbent")
			}
		})
	}
}
