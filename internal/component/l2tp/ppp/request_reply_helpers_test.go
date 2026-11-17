// Design: docs/research/l2tpv2-ze-integration.md -- request-bound protocol fixture replies

package ppp

import (
	"sync"
	"testing"
)

// replyNextAuthRequest supplies a former pre-phase decision through the first
// actual emitted request. Install it before the phase starts. It forwards every
// event unchanged to the original assertion channel, in order, before answering.
// It never retries the decision against a later request after expiration.
// The returned stop function MUST run only after that phase completed or its
// worker joined; cleanup provides the same idempotent stop and join automatically.
func replyNextAuthRequest(t *testing.T, session *pppSession, decision authResponseMsg) func() {
	t.Helper()
	observed := session.authEventsOut
	emitted := make(chan AuthEvent)
	session.authEventsOut = emitted
	stop := make(chan struct{})
	done := make(chan struct{})
	var once sync.Once
	stopRelay := func() {
		once.Do(func() {
			close(stop)
			<-done
			if session.authEventsOut == emitted {
				session.authEventsOut = observed
			}
		})
	}
	t.Cleanup(stopRelay)
	go func() {
		defer close(done)
		answered := false
		for {
			select {
			case <-stop:
				return
			case event, open := <-emitted:
				if !open {
					return
				}
				// Forward a ready observation before honoring a cleanup signal.
				// The unbuffered relay holds no extra queue of source events.
				select {
				case observed <- event:
				default:
					select {
					case observed <- event:
					case <-stop:
						return
					}
				}
				if answered {
					continue
				}
				request, ok := event.(EventAuthRequest)
				if !ok {
					continue
				}
				answered = true
				if err := request.answer(decision); err != nil {
					t.Errorf("reply to the captured authentication request: %v", err)
				}
			}
		}
	}()
	return stopRelay
}

// replyNextIPRequest supplies a former pre-phase decision through the first
// actual emitted request. Install it before the phase starts. It forwards every
// event unchanged to the original assertion channel, in order, before answering.
// It never retries the decision against a later request after expiration.
// The returned stop function MUST run only after that phase completed or its
// worker joined; cleanup provides the same idempotent stop and join automatically.
func replyNextIPRequest(t *testing.T, session *pppSession, decision ipResponseMsg) func() {
	t.Helper()
	observed := session.ipEventsOut
	emitted := make(chan IPEvent)
	session.ipEventsOut = emitted
	stop := make(chan struct{})
	done := make(chan struct{})
	var once sync.Once
	stopRelay := func() {
		once.Do(func() {
			close(stop)
			<-done
			if session.ipEventsOut == emitted {
				session.ipEventsOut = observed
			}
		})
	}
	t.Cleanup(stopRelay)
	go func() {
		defer close(done)
		answered := false
		for {
			select {
			case <-stop:
				return
			case event, open := <-emitted:
				if !open {
					return
				}
				// Forward a ready observation before honoring a cleanup signal.
				// The unbuffered relay holds no extra queue of source events.
				select {
				case observed <- event:
				default:
					select {
					case observed <- event:
					case <-stop:
						return
					}
				}
				if answered {
					continue
				}
				request, ok := event.(EventIPRequest)
				if !ok {
					continue
				}
				answered = true
				if err := request.answer(decision); err != nil {
					t.Errorf("reply to the captured address request: %v", err)
				}
			}
		}
	}()
	return stopRelay
}

// answerAuthRequestForTest replies through the exact event already observed by
// a protocol fixture. The fixture never manufactures a pending capability.
func answerAuthRequestForTest(t *testing.T, request EventAuthRequest, decision authResponseMsg) {
	t.Helper()
	if err := request.answer(decision); err != nil {
		t.Fatalf("answer the observed authentication request: %v", err)
	}
}

// answerIPRequestForTest preserves the observed family and its exact wait.
func answerIPRequestForTest(t *testing.T, request EventIPRequest, decision ipResponseMsg) {
	t.Helper()
	if err := request.answer(decision); err != nil {
		t.Fatalf("answer the observed address request: %v", err)
	}
}
