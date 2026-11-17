// Design: docs/architecture/behavior/peer-lifecycle.md -- adoption retains original terminal bounds.
// Related: incoming_ingress_corrections_test.go -- controlled original timer dispatch.
package reactor

import (
	"bytes"
	"errors"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgpserver "github.com/ze-software/ze/internal/component/bgp/server"
	"github.com/ze-software/ze/internal/test/sim"
)

// TestIncomingOpenSelectionRechecksTerminal blocks the selected validation
// callback, then refuses policy, expires the original budget, or closes TCP.
// None may publish an incoming session after that blocking boundary.
func TestIncomingOpenSelectionRechecksTerminal(t *testing.T) {
	for _, outcome := range []string{"policy-refused", "expired", "closed"} {
		t.Run(outcome, func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101)
			peer := NewPeer(settings)
			session := peer.newIncomingSession()
			fc := &incomingCorrectionClock{FakeClock: sim.NewFakeClock(time.Now()), fired: make(chan struct{}), release: make(chan struct{})}
			session.SetClock(fc)
			entered := make(chan struct{})
			proceed := make(chan struct{})
			releaseValidation := sync.OnceFunc(func() { close(proceed) })
			releaseTimer := sync.OnceFunc(func() { close(fc.release) })
			var selectionDone, advanceDone chan struct{}
			var calls atomic.Uint32
			session.setOpenValidator(func(_ string, _, _ *message.Open) error {
				if calls.Add(1) == 1 {
					return nil
				}
				close(entered)
				<-proceed
				if outcome == "policy-refused" {
					return &bgpserver.OpenValidationError{NotifyCode: 2, NotifySubcode: 11, Reason: "selected policy changed"}
				}
				return nil
			})
			remote, _ := incomingCorrectionStart(t, session, false)
			t.Cleanup(func() {
				closeConnQuietly(remote)
				releaseValidation()
				releaseTimer()
				if selectionDone != nil {
					incomingCorrectionAwait(t, selectionDone, "selected OPEN callback cleanup")
				}
				if advanceDone != nil {
					incomingCorrectionAwait(t, advanceDone, "selection deadline clock cleanup")
				}
			})
			wire := message.PackTo(delayOpenRemote(0x03030303), nil)
			if _, err := session.admitOpen(wire[message.HeaderLen:]); err != nil {
				t.Fatal(err)
			}
			var published atomic.Uint32
			leave := &sessionLeave{}
			result := make(chan error, 1)
			selectionDone = make(chan struct{})
			go func() {
				defer close(selectionDone)
				result <- session.adoptIncoming(func(_, _ fsm.State, _ fsm.Event) { published.Add(1) }, leave, false)
			}()
			select {
			case <-entered:
			case err := <-result:
				t.Fatalf("selection bypassed the retained OPEN's validation: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatal("selected OPEN validator was not reached")
			}
			if outcome == "expired" {
				expires := session.timers.HoldBudget().Expires
				advanceDone = make(chan struct{})
				go func() { defer close(advanceDone); fc.Add(expires.Sub(fc.FakeClock.Now())) }()
				incomingCorrectionAwait(t, fc.fired, "original hold callback before dispatch")
				if fc.FakeClock.Now().Before(expires) {
					t.Fatal("the original hold budget has not elapsed")
				}
			}
			if outcome == "closed" {
				session.closeConn()
			}
			releaseValidation()
			// Drain the bounded refusal before joining its synchronous writer.
			if outcome != "closed" {
				packet := delayOpenReadType(t, remote, 3)
				want := []byte{2, 11}
				if outcome == "expired" {
					want = []byte{4, 0}
				}
				if !bytes.Equal(packet[message.HeaderLen:], want) {
					t.Fatalf("terminal selection NOTIFICATION=%x, want %x", packet[message.HeaderLen:], want)
				}
			}
			select {
			case err := <-result:
				if err == nil {
					t.Fatal("terminal selection was accepted")
				}
				if outcome == "expired" {
					if !errors.Is(err, ErrHoldTimerExpired) {
						t.Fatalf("expired selection returned %v", err)
					}
				}
			case <-time.After(5 * time.Second):
				t.Fatal("terminal selection did not return")
			}
			if leave.selected {
				t.Fatal("terminal candidate installed selected publication")
			}
			if published.Load() != 0 {
				t.Fatal("terminal candidate published an FSM transition")
			}
		})
	}
}
