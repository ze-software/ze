// Design: docs/architecture/behavior/fsm.md — atomic strict DelayOpen release.
package reactor

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// strictReleaseWriteGate holds the first completed control write at its zero
// deadline reset, after Flush has put the KEEPALIVE on the real TCP socket but
// before handleBFDEvent can apply its FSM event. The owner MUST unblock it
// before joining the writer, including every test cleanup path.
type strictReleaseWriteGate struct {
	net.Conn
	parked      chan struct{}
	release     chan struct{}
	parkOnce    sync.Once
	releaseOnce sync.Once
}

func (c *strictReleaseWriteGate) SetWriteDeadline(deadline time.Time) error {
	if err := c.Conn.SetWriteDeadline(deadline); err != nil {
		return err
	}
	if deadline.IsZero() {
		c.parkOnce.Do(func() {
			close(c.parked)
			<-c.release
		})
	}
	return nil
}

// unblock MUST run before the writer is joined; it is also safe in cleanup
// after the ordinary release, so a failed assertion cannot strand the writer.
func (c *strictReleaseWriteGate) unblock() {
	c.releaseOnce.Do(func() { close(c.release) })
}

// TestSessionBFDStrictDelayOpenImmediatePeerKeepalive exercises the race the
// ordinary release matrix excludes by waiting for handleBFDEvent to finish.
// The peer receives the withheld KEEPALIVE and replies while the BFD writer is
// still inside its post-flush deadline reset. The actual receive dispatcher
// must not feed Event26 to a still-pending Connect/Active FSM.
//
// Regression discriminator: without receive-side serialization, the reply
// reaches the Connect/Active catchall, counts a failed attempt and enters Idle
// before the BFD event. Both parent states and all three release events matter.
func TestSessionBFDStrictDelayOpenImmediatePeerKeepalive(t *testing.T) {
	for _, passive := range []bool{false, true} {
		for _, event := range []fsm.Event{fsm.EventBfdUp, fsm.EventBfdAdminDown, fsm.EventBfdDisabled} {
			t.Run(fmt.Sprintf("passive=%t/%s", passive, event), func(t *testing.T) {
				s, far, transport, _, counter := strictDelaySocket(t, passive, 9, api.StateDown, true)
				strictDelayReceiveOpen(t, s, far, 9, false)
				delayOpenReadType(t, far, 1)
				parent := s.State()
				if s.fsm.BfdSubState() == fsm.SubStateNone {
					t.Fatal("OPEN did not enter strict DelayOpen pending")
				}

				gate := &strictReleaseWriteGate{
					Conn: transport.Conn, parked: make(chan struct{}), release: make(chan struct{}),
				}
				transport.Conn = gate
				writerDone := make(chan error, 1)
				go func() { writerDone <- s.handleBFDEvent(event) }()
				writerJoined := false
				t.Cleanup(func() {
					gate.unblock()
					if !writerJoined {
						select {
						case <-writerDone:
						case <-time.After(5 * time.Second):
							t.Error("BFD release writer did not stop")
						}
					}
				})
				select {
				case <-gate.parked:
				case <-time.After(5 * time.Second):
					t.Fatal("KEEPALIVE writer did not reach its post-flush barrier")
				}
				delayOpenReadType(t, far, 4)
				if s.State() != parent || s.fsm.BfdSubState() == fsm.SubStateNone {
					t.Fatal("write barrier did not retain the pending parent before the BFD event")
				}

				readObserved := make(chan struct{})
				s.onRead = func() { close(readObserved) }
				receiverDone := make(chan error, 1)
				go func() { receiverDone <- s.ReadAndProcess() }()
				receiverJoined := false
				t.Cleanup(func() {
					gate.unblock()
					if !receiverJoined {
						_ = far.Close() // Unblock the owned socket reader on an early assertion failure.
						select {
						case <-receiverDone:
						case <-time.After(5 * time.Second):
							t.Error("KEEPALIVE receiver did not stop")
						}
					}
				})
				delayOpenWrite(t, far, &message.Keepalive{})
				select {
				case <-readObserved:
				case <-time.After(5 * time.Second):
					t.Fatal("immediate peer KEEPALIVE did not reach the real receive dispatcher")
				}

				// Read completion is the readiness barrier. This window measures
				// forbidden dispatch completion while the release is deliberately
				// parked; it is not a sleep used to make the socket become ready.
				blockedWindow := time.NewTimer(500 * time.Millisecond)
				var receiveErr error
				early := false
				select {
				case receiveErr = <-receiverDone:
					receiverJoined, early = true, true
				case <-blockedWindow.C:
				}
				blockedWindow.Stop()
				gate.unblock()
				select {
				case err := <-writerDone:
					writerJoined = true
					if err != nil {
						t.Errorf("BFD release: %v", err)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("BFD release did not finish after unblocking its write")
				}
				if !receiverJoined {
					select {
					case receiveErr = <-receiverDone:
						receiverJoined = true
					case <-time.After(5 * time.Second):
						t.Fatal("peer KEEPALIVE did not finish after the BFD transition")
					}
				}
				if early {
					t.Errorf("peer KEEPALIVE completed before BFD release committed: %v", receiveErr)
				}
				if receiveErr != nil {
					t.Errorf("peer KEEPALIVE failed: %v", receiveErr)
				}
				if s.State() != fsm.StateEstablished || s.fsm.BfdSubState() != fsm.SubStateNone || counter.Load() != 0 || s.Conn() != transport || s.tearingDown.Load() {
					t.Fatalf("immediate reply left state=%s sub=%s counter=%d conn-retained=%t teardown=%t; want Established, no retry or teardown",
						s.State(), s.fsm.BfdSubState(), counter.Load(), s.Conn() == transport, s.tearingDown.Load())
				}
			})
		}
	}
}
