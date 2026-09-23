// Design: internal/component/l2tp/ppp/session_run.go -- the LCP Restart counter and Restart timer
// Related: internal/component/l2tp/ppp/ppp_fsm.go -- the transition table these tests drive
// Related: plan/journal/silent-fall-through.md -- the row this file closes

package ppp

import (
	"testing"
	"testing/synctest"
	"time"
)

// confRequestsIn counts the Configure-Requests among the recorded frames, which
// is the number of transmissions the Restart counter is meant to bound.
func confRequestsIn(t *testing.T, rec *frameRecorder) int {
	t.Helper()
	count := 0
	for _, d := range decodeFrames(t, rec) {
		if d.Proto == ProtoLCP && d.Pkt.Code == LCPConfigureRequest {
			count++
		}
	}
	return count
}

// VALIDATES: a peer's Configure-Request loads the Restart counter with
//
//	Max-Configure, each transmission spends one, and the Timeout event that
//	finds the counter at zero ends the negotiation instead of retransmitting
//	for ever.
//
// PREVENTS: the bare three-second ticker this replaced, which retransmitted a
//
//	Configure-Request while the state was Req-Sent or Ack-Sent and counted
//	nothing, so TO- was unreachable and only the 30-second negotiation timeout
//	ever stopped the session.
//
// RFC 1661 Section 4.4, Initialize-Restart-Count: "This action sets the Restart
// counter to the appropriate value (Max-Terminate or Max-Configure). The
// counter is decremented for each transmission, including the first." Section
// 4.3, Timeout: "The TO- event indicates that the Restart counter is not
// greater than zero, and no more packets need to be retransmitted".
func TestLCPRestartCounterBoundsConfigureRequests(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateStopped)
	req := lcpFrame(ProtoLCP, LCPConfigureRequest, 0x01,
		optStream(mruOption(1400), magicOption(0xAABBCCDD)))

	if term := s.handleFrame(req); term {
		t.Fatal("handleFrame terminated the session on an acceptable Configure-Request")
	}
	if st := s.currentState(); st != LCPStateAckSent {
		t.Fatalf("state = %s, want ack-sent (Stopped + RCR+ runs irc, scr, sca)", st)
	}
	if s.restartCount != defaultMaxConfigure-1 {
		t.Fatalf("restart counter = %d after the first Configure-Request, want %d: irc loads "+
			"Max-Configure and the transmission spends one",
			s.restartCount, defaultMaxConfigure-1)
	}

	// Every expiry while the counter is above zero is TO+, which retransmits.
	for spent := 1; spent < defaultMaxConfigure; spent++ {
		if done := s.handleRestartTimeout(); done {
			t.Fatalf("the session ended after %d Configure-Requests, want %d transmissions first",
				spent, defaultMaxConfigure)
		}
		if want := defaultMaxConfigure - spent - 1; s.restartCount != want {
			t.Fatalf("restart counter = %d after %d transmissions, want %d", s.restartCount, spent+1, want)
		}
	}
	if got := confRequestsIn(t, rec); got != defaultMaxConfigure {
		t.Fatalf("wrote %d Configure-Requests, want Max-Configure = %d", got, defaultMaxConfigure)
	}

	// The counter is now zero, so the next expiry is TO-, which stops.
	if done := s.handleRestartTimeout(); !done {
		t.Fatal("the Timeout event with the Restart counter at zero must end the session (tlf)")
	}
	if st := s.currentState(); st != LCPStateStopped {
		t.Fatalf("state = %s, want stopped (Ack-Sent + TO- runs tlf)", st)
	}
	if got := confRequestsIn(t, rec); got != defaultMaxConfigure {
		t.Fatalf("wrote %d Configure-Requests, want no transmission after TO-: %d",
			got, defaultMaxConfigure)
	}
}

// VALIDATES: a peer's Terminate-Request received in Opened zeroes the Restart
//
//	counter, so the Restart timer it arms expires as TO- and carries the
//	automaton to Stopped without sending a Terminate-Request of its own.
//
// PREVENTS: a Zero-Restart-Count action left as a no-op, which is what stood
//
//	here: the pause RFC 1661 asks for never ended, because nothing armed a
//	timer and nothing read a counter.
//
// RFC 1661 Section 4.4, Zero-Restart-Count: "This action sets the Restart
// counter to zero. This action enables the FSA to pause before proceeding to
// the desired final state, allowing traffic to be processed by the peer."
// Section 4.1: "Only the Send-Configure-Request, Send-Terminate-Request and
// Zero-Restart-Count actions start or re-start the Restart timer".
func TestLCPPeerTerminateRequestZeroesRestartCounter(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	s.restartCount = 7 // a live counter, so the zeroing below is visible

	if term := s.handleFrame(lcpFrame(ProtoLCP, LCPTerminateRequest, 0x07, nil)); term {
		t.Fatal("a peer Terminate-Request must leave the session running until the pause ends")
	}
	if st := s.currentState(); st != LCPStateStopping {
		t.Fatalf("state = %s, want stopping (Opened + RTR runs tld, zrc, sta)", st)
	}
	if s.restartCount != 0 {
		t.Fatalf("restart counter = %d, want 0 (zrc)", s.restartCount)
	}
	if c := rec.count(); c != 1 {
		t.Fatalf("wrote %d frames, want 1 Terminate-Ack", c)
	}
	if _, ok := findCode(t, rec, LCPTerminateAck); !ok {
		t.Fatal("the answer to a Terminate-Request must be a Terminate-Ack")
	}

	// The counter is zero, so the pause ends in TO- rather than in a
	// Terminate-Request retransmission.
	if done := s.handleRestartTimeout(); !done {
		t.Fatal("the Timeout event after zrc must end the session (tlf)")
	}
	if st := s.currentState(); st != LCPStateStopped {
		t.Fatalf("state = %s, want stopped (Stopping + TO- runs tlf)", st)
	}
	if c := rec.count(); c != 1 {
		t.Fatalf("wrote %d frames, want no Terminate-Request after a zeroed counter", c)
	}
}

// RFC requirement: RFC1661-4.4-2 positive -- an Opened peer termination arms the real Restart timer; the session stays Stopping for the grace period, then its timer event reaches Stopped.
// RFC requirement: RFC1661-4.4-2 negative -- a Terminate-Request in Closed sends its Ack without arming a grace timer, because that transition has no zrc action.
func TestLCPPeerTerminateRestartTimer(t *testing.T) {
	for _, initial := range []LCPState{LCPStateOpened, LCPStateClosed} {
		t.Run(initial.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s, rec, _ := newRFC1661Session(initial)
				s.restartTimer = time.NewTimer(time.Hour)
				s.restartTimer.Stop()
				defer s.restartTimer.Stop()
				s.handleFrame(lcpFrame(ProtoLCP, LCPTerminateRequest, 7, nil))
				ack, ok := findCode(t, rec, LCPTerminateAck)
				if !ok || ack.Identifier != 7 {
					t.Fatal("peer termination did not receive its matching Ack")
				}
				time.Sleep(defaultRestartTimer-time.Nanosecond)
				select {
				case <-s.restartTimer.C:
					t.Fatal("Restart timer expired before the termination grace period")
				default:
				}
				if initial == LCPStateOpened && s.currentState() != LCPStateStopping {
					t.Fatal("session did not remain Stopping during the grace period")
				}
				time.Sleep(time.Nanosecond)
				select {
				case <-s.restartTimer.C:
					if initial != LCPStateOpened {
						t.Fatal("transition without zrc armed the Restart timer")
					}
					if !s.handleRestartTimeout() || s.currentState() != LCPStateStopped {
						t.Fatal("expired grace timer did not terminate the session")
					}
				default:
					if initial == LCPStateOpened {
						t.Fatal("zrc did not arm a Restart timer")
					}
				}
			})
		})
	}
}
