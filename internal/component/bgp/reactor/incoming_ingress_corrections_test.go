// Design: docs/architecture/behavior/peer-lifecycle.md -- incoming admission deadlines.
// Related: incoming_ingress_test.go -- malformed OPEN and ordinary ingress controls.
// RFC 4271 Sections 8.1.5 and 8.2.2 -- see rfc/short/rfc4271.md.
package reactor

import (
	"bytes"
	"errors"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgpserver "github.com/ze-software/ze/internal/component/bgp/server"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/test/sim"
)

// TestIncomingExpiredOpenCannotRetireIncumbent holds the original open-wait
// callback before dispatch, then crosses its real deadline during validation
// or while collision arbitration waits for Peer.mu. The unchanged deadline
// must refuse only that candidate, even before its queued callback can run.
func TestIncomingExpiredOpenCannotRetireIncumbent(t *testing.T) {
	for _, wait := range []string{"validator", "peer-owner"} {
		t.Run(wait, func(t *testing.T) {
			peer, original, outgoing := incomingCorrectionPeer(t, 0)
			fc := &incomingCorrectionClock{
				FakeClock: sim.NewFakeClock(time.Now()),
				fired:     make(chan struct{}), release: make(chan struct{}),
			}
			releaseExpiry := sync.OnceFunc(func() { close(fc.release) })
			t.Cleanup(releaseExpiry)
			candidateSession := peer.newIncomingSession()
			candidateSession.SetClock(fc)
			var holdExpired atomic.Uint32
			candidateSession.fsm.SetCallback(func(_, _ fsm.State, event fsm.Event) {
				if event == fsm.EventHoldTimerExpires {
					holdExpired.Add(1)
				}
			})
			entered := make(chan struct{})
			proceed := make(chan struct{})
			releaseValidator := sync.OnceFunc(func() { close(proceed) })
			t.Cleanup(releaseValidator)
			candidateSession.setOpenValidator(func(_ string, _, _ *message.Open) error {
				close(entered)
				<-proceed
				return nil
			})
			incoming, local := incomingCorrectionStart(t, candidateSession, false)
			bridgeDone := make(chan struct{})
			close(bridgeDone)
			candidate := &incomingCandidate{
				peer: peer, conn: local, session: candidateSession,
				readDone: make(chan struct{}), handoffDone: make(chan struct{}),
				bridgeStop: make(chan struct{}), bridgeDone: bridgeDone,
				progress: make(chan struct{}, 1),
			}
			peer.mu.Lock()
			peer.pending = candidate
			peer.mu.Unlock()
			var advanced, resolveDone chan struct{}
			var releaseRead, releasePeer func()
			t.Cleanup(func() {
				// Real TCP drains use absolute deadlines from the fake clock.
				// Release the fixture's remote sockets before any Session closes.
				closeConnQuietly(incoming)
				closeConnQuietly(outgoing)
				releaseValidator()
				if releaseRead != nil {
					releaseRead()
				}
				if releasePeer != nil {
					releasePeer()
				}
				releaseExpiry()
				candidateSession.closeConn()
				original.closeConn()
				incomingCorrectionAwait(t, candidate.readDone, "candidate OPEN reader cleanup")
				if resolveDone != nil {
					incomingCorrectionAwait(t, resolveDone, "candidate resolver cleanup")
				}
				if advanced != nil {
					incomingCorrectionAwait(t, advanced, "clock advancement cleanup")
				}
				peer.incomingWG.Wait()
				candidateSession.stopSessionTimers()
			})
			type admissionResult struct {
				open *candidateOpen
				err  error
			}
			admitted := make(chan admissionResult, 1)
			go func() {
				defer close(candidate.readDone)
				open, err := candidate.readOpen(t.Context(), candidateSession.bufReader)
				admitted <- admissionResult{open: open, err: err}
			}()
			delayOpenWrite(t, incoming, delayOpenRemote(0x03030303))
			incomingCorrectionAwait(t, entered, "candidate validator entry")
			expires, armed := fc.NextTimer()
			if !armed {
				t.Fatal("candidate has no original open-wait timer")
			}
			advance := func() {
				advanced = make(chan struct{})
				go func() { defer close(advanced); fc.Add(expires.Sub(fc.FakeClock.Now())) }()
				incomingCorrectionAwait(t, fc.fired, "original callback dispatch gate")
				if fc.FakeClock.Now().Before(expires) {
					t.Fatal("original open-wait deadline has not expired")
				}
			}
			if wait == "validator" {
				advance()
			}
			releaseValidator()
			var admission admissionResult
			select {
			case admission = <-admitted:
			case <-time.After(5 * time.Second):
				t.Fatal("OPEN admission did not finish after validator release")
			}
			resolved := make(chan incomingDecision, 1)
			if admission.err == nil {
				var readGate *incomingCorrectionReadGate
				if wait == "peer-owner" {
					readGate = &incomingCorrectionReadGate{entered: make(chan struct{}), release: make(chan struct{})}
					fc.readGate.Store(readGate)
					peer.mu.Lock()
					releaseRead = sync.OnceFunc(func() { close(readGate.release) })
					releasePeer = sync.OnceFunc(peer.mu.Unlock)
				}
				resolveDone = make(chan struct{})
				go func() {
					defer close(resolveDone)
					resolved <- candidate.resolve(admission.open, admission.open.open.BGPIdentifier)
				}()
				if readGate != nil {
					// HoldBudget has sampled the old time under openMu. Its early
					// answer will remain Armed; only the commit-side check can see
					// the deadline crossed while Peer.mu was unavailable.
					select {
					case <-readGate.entered:
					case <-time.After(5 * time.Second):
						releaseRead()
						releasePeer()
						t.Fatal("collision resolution did not sample its early hold budget")
					}
					advance()
					releaseRead()
					releasePeer()
				}
			} else {
				if !errors.Is(admission.err, ErrHoldTimerExpired) {
					t.Fatal(admission.err)
				}
				resolved <- incomingRejected
			}
			select {
			case decision := <-resolved:
				if decision != incomingRejected {
					t.Fatalf("expired candidate decision=%d", decision)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("expired candidate resolution did not finish")
			}
			reply := delayOpenReadType(t, incoming, 3)
			if !bytes.Equal(reply[message.HeaderLen:], []byte{4, 0}) {
				t.Fatalf("expired candidate NOTIFICATION=%x, want 0400", reply[message.HeaderLen:])
			}
			if got := holdExpired.Load(); got != 1 {
				t.Fatalf("hold expiry events=%d, want 1", got)
			}
			if candidateSession.State() != fsm.StateIdle {
				t.Fatal("expiry itself did not enter Idle")
			}
			// The peer has received the complete NOTIFICATION. Its write-side
			// FIN ends Session's drain without closing the read-side EOF oracle.
			incomingTCP, ok := incoming.(*net.TCPConn)
			if !ok {
				t.Fatal("incoming listener did not return a TCP connection")
			}
			if err := incomingTCP.CloseWrite(); err != nil {
				t.Fatal(err)
			}
			candidate.retire()
			incomingIngressEOF(t, incoming)
			if candidateSession.State() != fsm.StateIdle {
				t.Fatal("expired candidate did not enter Idle")
			}
			if candidateSession.Negotiated() != nil {
				t.Fatal("expired candidate published negotiation")
			}
			if got := peer.ConnectRetryCounter(); got != 1 {
				t.Fatalf("expiry counter=%d, want 1", got)
			}
			peer.mu.RLock()
			inbound, handoff := peer.inbound, peer.collisionHandoff
			peer.mu.RUnlock()
			if inbound != nil {
				t.Fatal("expired candidate was published inbound")
			}
			if handoff {
				t.Fatal("expired candidate reserved a collision handoff")
			}
			if peer.currentSession() != original {
				t.Fatal("expired candidate replaced its incumbent")
			}
			if original.State() != fsm.StateOpenConfirm {
				t.Fatal("expired candidate retired its incumbent")
			}
			releaseExpiry()
			incomingCorrectionAwait(t, advanced, "queued expiry callback completion")
			if got := holdExpired.Load(); got != 1 {
				t.Fatalf("queued callback repeated Event10: %d", got)
			}
			if got := peer.ConnectRetryCounter(); got != 1 {
				t.Fatalf("queued callback repeated expiry: %d", got)
			}
			incomingCorrectionConfirm(t, original, outgoing)
		})
	}
}

// TestIncomingValidatorRefusalUsesOpenErrorEvent sends a real OPEN to a
// pre-TCP installed coded plugin validator, then runs the candidate retirement
// action. The OPEN error itself must own Idle and one retry-history increment;
// retirement cannot substitute TCP failure or grant an initial retry budget.
func TestIncomingValidatorRefusalUsesOpenErrorEvent(t *testing.T) {
	for _, held := range []bool{false, true} {
		name := "OpenSent"
		delay := time.Duration(0)
		if held {
			name = "Connect-held"
			delay = 240 * time.Second
		}
		t.Run(name, func(t *testing.T) {
			peer, original, outgoing := incomingCorrectionPeer(t, delay)
			session := peer.newSession()
			session.SetClock(sim.NewFakeClock(time.Now()))
			session.setOpenValidator(func(_ string, _, _ *message.Open) error {
				return &bgpserver.OpenValidationError{NotifyCode: 2, NotifySubcode: 11, Reason: "test role mismatch"}
			})
			var openErrors atomic.Uint32
			session.fsm.SetCallback(func(_, _ fsm.State, event fsm.Event) {
				if event == fsm.EventBGPOpenMsgErr {
					openErrors.Add(1)
				}
			})
			incoming, _ := incomingCorrectionStart(t, session, held)
			before := peer.ConnectRetryCounter()
			processed := incomingCorrectionProcess(t, session, incoming)
			delayOpenWrite(t, incoming, delayOpenRemote(0x03030303))
			reply := delayOpenReadType(t, incoming, 3)
			if !bytes.Equal(reply[message.HeaderLen:], []byte{2, 11}) {
				t.Fatalf("coded validator NOTIFICATION=%x, want 020b", reply[message.HeaderLen:])
			}
			select {
			case err := <-processed:
				if err == nil {
					t.Fatal("coded validator accepted OPEN")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("coded validator refusal did not finish")
			}
			// This is the unselected candidate finalizer's transport-failure
			// delivery. Event22 must already own Idle, so it does nothing more.
			session.handleConnectionClose()
			session.stopSessionTimers()
			incomingIngressEOF(t, incoming)
			if got := openErrors.Load(); got != 1 {
				t.Fatalf("OPEN error events=%d, want 1", got)
			}
			if session.State() != fsm.StateIdle {
				t.Fatalf("refusal state=%s, want Idle", session.State())
			}
			if got := peer.ConnectRetryCounter(); got != before+1 {
				t.Fatalf("refusal counter=%d, want %d", got, before+1)
			}
			if session.retryBudget != nil {
				t.Fatal("OPEN refusal obtained TCP failure's initial retry budget")
			}
			if session.timers.IsConnectRetryTimerRunning() {
				t.Fatal("OPEN refusal left ConnectRetryTimer armed")
			}
			if session.timers.IsDelayOpenTimerRunning() {
				t.Fatal("OPEN refusal left DelayOpenTimer armed")
			}
			if peer.currentSession() != original {
				t.Fatal("plugin refusal replaced its incumbent")
			}
			incomingCorrectionConfirm(t, original, outgoing)
		})
	}
}

// incomingCorrectionPeer owns two real TCP Session lifecycles. Tests MUST
// close their own gates before cleanup joins each Session's timer owner.
func incomingCorrectionPeer(t *testing.T, delay time.Duration) (*Peer, *Session, net.Conn) {
	t.Helper()
	settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101)
	settings.Connection = ConnectionBoth
	settings.DelayOpen = delay
	peer := NewPeer(settings)
	original := peer.newSession()
	original.SetClock(sim.NewFakeClock(time.Now()))
	peer.mu.Lock()
	peer.session = original
	peer.mu.Unlock()
	outgoing, _ := incomingCorrectionStart(t, original, delay != 0)
	processed := incomingCorrectionProcess(t, original, outgoing)
	delayOpenWrite(t, outgoing, delayOpenRemote(0x02020202))
	if delay != 0 {
		delayOpenReadType(t, outgoing, 1)
	}
	delayOpenReadType(t, outgoing, 4)
	select {
	case err := <-processed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("incumbent OPEN did not finish")
	}
	if original.State() != fsm.StateOpenConfirm {
		t.Fatal("incumbent did not reach OpenConfirm")
	}
	return peer, original, outgoing
}

func incomingCorrectionStart(t *testing.T, session *Session, held bool) (net.Conn, net.Conn) {
	t.Helper()
	remote, local := collisionProofTCPPair(t)
	t.Cleanup(func() {
		closeConnQuietly(remote)
		session.closeConn()
		session.stopSessionTimers()
	})
	if err := session.startDamped(); err != nil {
		t.Fatal(err)
	}
	if err := session.Accept(local); err != nil {
		t.Fatal(err)
	}
	if held {
		if !session.holdsOpen() {
			t.Fatal("candidate does not own its held TCP connection")
		}
	} else {
		delayOpenReadType(t, remote, 1)
	}
	return remote, local
}

func incomingCorrectionConfirm(t *testing.T, original *Session, outgoing net.Conn) {
	t.Helper()
	processed := incomingCorrectionProcess(t, original, outgoing)
	delayOpenWrite(t, outgoing, &message.Keepalive{})
	select {
	case err := <-processed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("useful incumbent confirmation did not finish")
	}
	if original.State() != fsm.StateEstablished {
		t.Fatalf("useful incumbent state=%s", original.State())
	}
}

// incomingCorrectionProcess owns one reader step. Cleanup MUST close the remote
// before joining, including when an assertion fails before consuming its result.
func incomingCorrectionProcess(t *testing.T, session *Session, remote net.Conn) <-chan error {
	t.Helper()
	processed := make(chan error, 1)
	done := make(chan struct{})
	t.Cleanup(func() {
		closeConnQuietly(remote)
		incomingCorrectionAwait(t, done, "Session reader cleanup")
	})
	go func() {
		defer close(done)
		processed <- session.ReadAndProcess()
	}()
	return processed
}

func incomingCorrectionAwait(t *testing.T, done <-chan struct{}, reason string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal(reason + " did not finish")
	}
}

// incomingCorrectionClock gates AfterFunc dispatch without replacing a Session
// callback. Its optional one-shot Now gate proves expiry during Peer.mu wait.
// The test MUST release both gates and join Add before returning.
type incomingCorrectionClock struct {
	*sim.FakeClock
	fired    chan struct{}
	release  chan struct{}
	once     sync.Once
	readGate atomic.Pointer[incomingCorrectionReadGate]
}

type incomingCorrectionReadGate struct {
	entered chan struct{}
	release chan struct{}
}

func (c *incomingCorrectionClock) Now() time.Time {
	now := c.FakeClock.Now()
	if gate := c.readGate.Swap(nil); gate != nil {
		close(gate.entered)
		<-gate.release
	}
	return now
}

func (c *incomingCorrectionClock) AfterFunc(d time.Duration, callback func()) clock.Timer {
	return c.FakeClock.AfterFunc(d, func() {
		c.once.Do(func() { close(c.fired) })
		<-c.release
		callback()
	})
}
