// Design: docs/architecture/l2tp/subscriber-session-model.md -- terminal
// publication still owns the subscriber ID used by the address pool.
package pppoe

import (
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
)

// The real PPP event consumer can be inside an older Down while the discovery
// reader handles a PADT. Until Down returns, its pool subscriber still owns
// the address allocation under this SID. The next worker's independent pool
// request must not be able to adopt that old allocation.
func TestPPPoETerminalPublicationHoldsSIDUntilSubscribersReturn(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixture := newCoreRetentionFixture(t, 491)
		const sid = uint16(81)
		session, transport, control := fixture.admit(t, sid)
		entered := make(chan struct{})
		release := make(chan struct{})
		releaseDown := sync.OnceFunc(func() { close(release) })
		defer releaseDown()
		subevents.SessionDown.Subscribe(fixture.sub.bus, func(event *subevents.SessionDownPayload) {
			if event.Session.ID != session.SubscriberID {
				t.Error("Down did not name the original subscriber")
			}
			close(entered)
			<-release
		})
		control.Cancel()
		down, ok := fixture.next(t).(ppp.EventSessionDown)
		if !ok || down.Control != control {
			t.Fatal("the original PPP worker did not deliver its terminal event")
		}
		delivered := make(chan struct{})
		go func() {
			defer close(delivered)
			fixture.sub.handlePPPEvent(down)
		}()
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("the original Down did not enter its subscriber")
		}

		packet := Packet{SID: sid, DstMAC: fixture.server.hwAddr}
		copy(packet.SrcMAC[:], session.MAC)
		fixture.server.handlePADT(&packet)
		select {
		case <-transport.closed:
		default:
			t.Fatal("competing PADT did not gate the original transport")
		}
		if found := fixture.server.sessions.Lookup(sid); found != session {
			t.Error("competing PADT released the exact admission during shared Down")
		}
		if next, err := fixture.server.sessions.AllocSID(); !errors.Is(err, ErrMaxSessions) {
			if err == nil {
				fixture.server.sessions.freeSID(next)
			}
			t.Errorf("replacement could start while the original pool callback was held: %v", err)
		}
		select {
		case <-session.closeDone:
			t.Error("physical close signaled SID release before shared Down returned")
		default:
		}

		releaseDown()
		<-delivered
		if fixture.server.sessions.Lookup(sid) != nil {
			t.Fatal("the original completion did not release its SID")
		}
		select {
		case <-session.closeDone:
		default:
			t.Fatal("the original completion did not settle closeDone")
		}
		successor, _, nextControl := fixture.admit(t, sid)
		if successor == session || successor.SubscriberID != session.SubscriberID || nextControl == control {
			t.Fatal("the same numeric SID did not create a distinct PPP worker")
		}
		fixture.sub.handlePPPEvent(down)
		if fixture.server.sessions.Lookup(sid) != successor || !nextControl.Active() {
			t.Fatal("a late original Down changed the useful successor")
		}
		if _, found := subscriber.DefaultRegistry.Get(successor.SubscriberID); !found {
			t.Fatal("a late original Down removed the successor's registry entry")
		}
	})
}

// This fixture's bus propagates a panic through the publication call. Test the
// admission's unwinding in that case. Ze's production bus catches and reports
// individual subscriber panics before returning to the publisher.
func TestPPPoETerminalPublicationPanicKeepsOriginalRetryable(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixture := newCoreRetentionFixture(t, 492)
		const sid = uint16(82)
		session, _, control := fixture.admit(t, sid)
		cause := errors.New("subscriber callback failed before returning")
		calls := 0
		subevents.SessionDown.Subscribe(fixture.sub.bus, func(*subevents.SessionDownPayload) {
			calls++
			if calls == 1 {
				packet := Packet{SID: sid, DstMAC: fixture.server.hwAddr}
				copy(packet.SrcMAC[:], session.MAC)
				// The callback itself reaches PADT. Any wait for its own
				// delivery would deadlock this single publishing goroutine.
				fixture.server.handlePADT(&packet)
				panic(cause)
			}
		})
		control.Cancel()
		down, ok := fixture.next(t).(ppp.EventSessionDown)
		if !ok || down.Control != control {
			t.Fatal("the original PPP worker did not deliver its terminal event")
		}
		func() {
			defer func() {
				if recovered, ok := recover().(error); !ok || !errors.Is(recovered, cause) {
					t.Errorf("subscriber callback panic = %v, want the original cause", recovered)
				}
			}()
			fixture.sub.handlePPPEvent(down)
		}()
		if fixture.server.sessions.Lookup(sid) != session {
			t.Fatal("interrupted Down gave up its original admission")
		}
		fixture.sub.handlePPPEvent(down)
		if calls != 2 || fixture.server.sessions.Lookup(sid) != nil {
			t.Fatal("retry did not finish delivery and release the original admission")
		}
	})
}
