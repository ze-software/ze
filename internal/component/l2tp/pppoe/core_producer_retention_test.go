// Design: docs/architecture/l2tp/bng-5-pppoe.md -- retained exact producer cleanup
package pppoe

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/core/report"
)

func TestCorePPPoEProducerCloseRetainsUntilSameOwnerRecovery(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	for _, kind := range []string{"before transfer", "close panic", "unsent restore"} {
		t.Run(kind, func(t *testing.T) {
			fixture := newCoreRetentionFixture(t, 7)
			cause := errors.New("original producer close failed")
			session, transport, _ := fixture.prepare(t, 88, true, cause)
			opened := session.Opened
			lease := session.unusedLease
			if kind == "close panic" {
				transport.owner.mu.Lock()
				transport.owner.panicFailure = cause
				transport.owner.mu.Unlock()
			}
			if kind == "unsent restore" {
				taken := fixture.server.sessions.takePPPLease(session)
				if taken != lease {
					t.Fatal("unsent handoff changed the actual retained lease")
				}
				fixture.server.sessions.restoreUnsentPPPLease(session, taken)
			}
			func() {
				if kind == "close panic" {
					defer func() {
						if recovered, ok := recover().(error); !ok || !errors.Is(recovered, cause) {
							t.Fatal("producer panic changed its original cause", recovered)
						}
					}()
				}
				fixture.server.closeAdmitted(session)
			}()
			if fixture.server.sessions.Lookup(88) != session || session.Opened != opened || session.unusedLease != lease {
				t.Fatal("failed producer cleanup dropped its exact owner roots")
			}
			select {
			case <-session.closeDone:
				t.Fatal("failed producer cleanup freed its completion/SID")
			default:
			}
			if _, err := fixture.server.sessions.AllocSID(); !errors.Is(err, ErrMaxSessions) {
				t.Fatal("failed producer cleanup reopened its held slot", err)
			}
			select {
			case <-transport.closed:
			default:
				t.Fatal("producer failure left physical wire ungated")
			}
			transport.owner.mu.Lock()
			transport.owner.failure = nil
			transport.owner.panicFailure = nil
			transport.owner.mu.Unlock()
			fixture.server.closeAdmitted(session)
			select {
			case <-session.closeDone:
			default:
				t.Fatal("same recovered owner did not finish original closeDone")
			}
			if fixture.server.sessions.Lookup(88) != nil {
				t.Fatal("same recovered owner kept its completed table slot")
			}
			if transport.owner.closes.Load() != 2 {
				t.Fatal("producer cleanup did not retry exactly the same failed owner", transport.owner.closes.Load())
			}
			fixture.server.closeAdmitted(session)
			if transport.owner.closes.Load() != 2 {
				t.Fatal("completed producer cleanup closed native owner again")
			}
		})
	}
}
