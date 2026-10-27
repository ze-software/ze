// Design: docs/architecture/behavior/peer-lifecycle.md -- why a session left Established
// Related: peer_close.go -- sessionCloseFor, the publication-failure input
// Related: peer_session_end.go -- sessionDown, which derives the close after the drain
// Related: recovery_owed_test.go -- the teardown half, publicationPanicReceiver
// Related: peer_run_test.go -- sessionEndCase, downOrderObserver
package reactor

import (
	"errors"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// errInjectedWrite is the write failure the unsent-Cease case injects.
var errInjectedWrite = errors.New("test: injected write failure")

// unsentCeaseReceiver retires the session's writer, as a failed socket write
// does (Session.retireWrite), then panics, so the Cease the delivery worker's
// teardown builds is never written.
type unsentCeaseReceiver struct {
	MessageReceiver
	session *Session
}

func (receiver unsentCeaseReceiver) OnMessageBatchReceived(*plugin.PeerInfo, []bgptypes.RawMessage) []int {
	receiver.session.writeMu.Lock()
	receiver.session.retireWrite(errInjectedWrite)
	receiver.session.writeMu.Unlock()
	panic("BUG: test receiver panics after its session's writer failed")
}

// lateFailureReceiver holds the session's first batch, as downOrderReceiver
// does, and panics once released, so the publication fails after whatever
// the subtest did meanwhile.
type lateFailureReceiver struct{ *downOrderReceiver }

func (receiver lateFailureReceiver) OnMessageBatchReceived(*plugin.PeerInfo, []bgptypes.RawMessage) []int {
	receiver.enterOnce.Do(func() { close(receiver.entered) })
	<-receiver.release
	panic("BUG: test receiver panics after the session ended")
}

// TestOwedKeyPublicationFailureForbidsRetention is the GR-capable half of
// AC-17 (spec-bgp-rs-replacement-on-withdrawal, Phase 2b second half): a
// session whose receive publication failed goes down with a reason that
// forbids Graceful Restart retention, whether the Cease reached the wire or
// not, and whenever the failure happened. The teardown half is
// TestOwedKeyPublicationFailureTearsDownSource; the GR plugin's answer to the
// reason is TestAdjRIBInInvalidCloseRetainsNothing.
//
// Method: one live session over net.Pipe per subtest, through the delivery
// worker's receiver seam or the read goroutine's hand-off seam. The down every
// peer observer receives is recorded with its reason.
//
// VALIDATES: Cease delivered: rpc.ReasonNotification. Cease unsent:
// rpc.ReasonAdjRIBInInvalid, never "notification". Late failure (the worker
// fails after a transport loss ended the session): rpc.ReasonAdjRIBInInvalid.
// The read goroutine panicking before the hand-off: its guard writes Cease /
// Unspecific before the session ends (spec-improve-5-panic-boundaries AC-1),
// so the down is rpc.ReasonNotification, which forbids retention as well.
// Each forbids retention
// (rpc.ReasonForbidsRetention), and the history row carries the same reason.
// PREVENTS: a GR helper keeping, for a restart time, routes from an Adj-RIB-In
// ze knows to be invalid.
func TestOwedKeyPublicationFailureForbidsRetention(t *testing.T) {
	t.Run("cease delivered", publicationFailureCeaseDelivered)
	t.Run("cease unsent", publicationFailureCeaseUnsent)
	t.Run("late failure after a transport loss", publicationFailureAfterTransportLoss)
	t.Run("read goroutine panic before the hand-off", publicationFailureReadPanic)
}

// publicationFailureCeaseDelivered: the worker's receiver panics and its
// teardown writes Cease / Unspecific (RFC 4271 Section 4.5: a panic has no
// defined subcode).
func publicationFailureCeaseDelivered(t *testing.T) {
	v, observer := publicationFailureSession(t)
	v.peer.sessionEndHooks.Store(&sessionEndHooks{deliveryReceiver: func(prior MessageReceiver) MessageReceiver {
		return publicationPanicReceiver{MessageReceiver: prior}
	}})
	t.Cleanup(func() { v.peer.sessionEndHooks.Store(nil) })
	publicationFailureSend(t, v)

	// RFC 4271 Section 4.5: Cease (6) with subcode Unspecific (0).
	lowEventually(t, func() bool { return ceaseSent(v, message.NotifyCeaseUnspecific) }, "Cease / Unspecific on the wire")
	publicationFailureDown(t, v.peer, observer, rpc.ReasonNotification)
}

// publicationFailureCeaseUnsent: the session's writer fails before the
// worker's teardown, so the Cease is built and never written
// (recordNotificationUnsent) and no NOTIFICATION was exchanged.
func publicationFailureCeaseUnsent(t *testing.T) {
	v, observer := publicationFailureSession(t)
	session := v.peer.currentSession()
	v.peer.sessionEndHooks.Store(&sessionEndHooks{deliveryReceiver: func(prior MessageReceiver) MessageReceiver {
		return unsentCeaseReceiver{MessageReceiver: prior, session: session}
	}})
	t.Cleanup(func() { v.peer.sessionEndHooks.Store(nil) })
	publicationFailureSend(t, v)

	publicationFailureDown(t, v.peer, observer, rpc.ReasonAdjRIBInInvalid)
	// The receiver panics, so the Cease its teardown builds is Unspecific.
	if ceaseSent(v, message.NotifyCeaseUnspecific) {
		t.Fatal("the Cease reached the wire although the session's writer had failed")
	}
}

// publicationFailureAfterTransportLoss is the late failure of round 7, I-2:
// the session ends on a body-read error while the worker holds its UPDATEs,
// the leave arm runs, and only then does the worker fail on the held batch.
func publicationFailureAfterTransportLoss(t *testing.T) {
	c := newSessionEndCase(t, lowLiveSettings("10.0.0.1", 65000, 65002))
	c.install(t, sessionEndHooks{deliveryReceiver: func(MessageReceiver) MessageReceiver {
		return lateFailureReceiver{downOrderReceiver: c.receiver}
	}})
	c.sendHeld(t, 2, false)
	c.sendTruncatedUpdate(t)
	c.leftWithoutDown(t)
	c.releaseReceiver()
	select {
	case <-c.observer.downAdded:
	case <-time.After(5 * time.Second):
		t.Fatal("no down after the held batch failed")
	}
	publicationFailureDown(t, c.p, c.observer, rpc.ReasonAdjRIBInInvalid)
}

// publicationFailureReadPanic is round 8, I-3: the read goroutine panics
// between acceptedReceive and the hand-off, with no lock held, after the
// worker published the UPDATEs before it.
func publicationFailureReadPanic(t *testing.T) {
	c := newSessionEndCase(t, lowLiveSettings("10.0.0.1", 65000, 65002))
	var handOffs atomic.Int64
	c.install(t, sessionEndHooks{beforeHandOff: func() {
		if handOffs.Add(1) == 3 {
			panic("test: read goroutine panic before the delivery hand-off")
		}
	}})
	attrs := firstASAttrs(4, 65002) // RFC 4271 Section 4.3, RFC 6793 Section 3.
	for i := range 3 {
		prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 20, byte(i), 0}), 24)
		ownershipLiveSend(t, c.v, nil, attrs, prefix)
	}
	c.leftWithoutDown(t)
	c.releaseReceiver()
	select {
	case <-c.observer.downAdded:
	case <-time.After(5 * time.Second):
		t.Fatal("no down after the read goroutine panicked")
	}
	// The read guard writes Cease / Unspecific before the session ends
	// (spec-improve-5-panic-boundaries AC-1), so a NOTIFICATION was exchanged.
	if !ceaseSent(c.v, message.NotifyCeaseUnspecific) {
		t.Fatal("the read guard sent no Cease / Unspecific")
	}
	publicationFailureDown(t, c.p, c.observer, rpc.ReasonNotification)
}

// publicationFailureSession starts one live session and records its downs.
func publicationFailureSession(t *testing.T) (*lowLivePeer, *downOrderObserver) {
	t.Helper()
	v := lowLiveRouter(t, lowLiveSettings("10.0.0.1", 65000, 65002))[0]
	observer := &downOrderObserver{downAdded: make(chan struct{}, 1)}
	v.peer.reactor.addPeerObserver(observer)
	return v, observer
}

// publicationFailureSend sends the one UPDATE whose publication fails.
func publicationFailureSend(t *testing.T, v *lowLivePeer) {
	t.Helper()
	attrs := firstASAttrs(4, 65002)                                            // RFC 6793 Section 3.
	ownershipLiveSend(t, v, nil, attrs, netip.MustParsePrefix("10.20.0.0/24")) // RFC 4271 Section 4.3.
}

// publicationFailureDown asserts the session's one down carries want, that
// want forbids retention, and that the history row names the same reason.
func publicationFailureDown(t *testing.T, p *Peer, observer *downOrderObserver, want string) {
	t.Helper()
	lowEventually(t, func() bool { return len(observer.records()) == 1 }, "the source's down")
	down := observer.records()[0]
	if down.reason != want {
		t.Fatalf("down reason = %q, want %q", down.reason, want)
	}
	if !rpc.ReasonForbidsRetention(down.reason) {
		t.Fatalf("down reason %q allows Graceful Restart retention", down.reason)
	}
	// The down appends the row after it told the observers.
	lowEventually(t, func() bool {
		for _, row := range p.history.snapshot() {
			if row.From == fsm.StateEstablished.String() {
				return true
			}
		}
		return false
	}, "the leave-Established row in the FSM history")
	sessionDownHistoryRow(t, p, want)
}
