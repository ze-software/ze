// Design: docs/architecture/behavior/peer-lifecycle.md -- scoped candidate publication ownership.
// Related: bfd_delay_open_test.go -- TCP-backed Session Start, OPEN and KEEPALIVE helpers.
package reactor

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
)

// TestIncomingAdoptionPanicReleasesOpenOwner completes a real private handshake
// before its publication callback fails. Its retiring owner must still acquire
// openMu and retire timers; a panic must not strand the next connection cycle.
func TestIncomingAdoptionPanicReleasesOpenOwner(t *testing.T) {
	session, far, _, _, _ := strictDelaySocket(t, true, 9, api.StateUp, false)
	// This standalone Session has no Peer statistics publisher. Install the
	// optional negotiation observer before either OPEN rail can call it.
	session.onNegotiated = func(_, _ uint32) {}
	strictDelayReceiveOpen(t, session, far, 9, false)
	delayOpenReadType(t, far, 1)
	keepalive := delayOpenReadType(t, far, 4)
	bfdInitialReceive(t, session, far, keepalive)
	if session.State() != fsm.StateEstablished {
		t.Fatal("private candidate did not complete its useful handshake")
	}
	const failure = "publication failed"
	var contained any
	func() {
		defer func() { contained = recover() }()
		_ = session.adoptIncoming(func(_, _ fsm.State, _ fsm.Event) {
			panic(failure)
		}, &sessionLeave{}, true)
	}()
	if contained != failure {
		t.Fatalf("publication panic was not exercised: %v", contained)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		session.stopSessionTimers()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("publication panic stranded openMu before timer retirement")
	}
	if session.timers.IsHoldTimerRunning() || session.timers.IsKeepaliveTimerRunning() {
		t.Fatal("timer retirement returned with the private candidate still armed")
	}
}
