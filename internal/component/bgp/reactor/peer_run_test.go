// Design: docs/architecture/behavior/fsm.md -- Reactor Notification: the down follows the buffered deliveries
// Related: peer_session_end.go -- the leave arm and runOnce's down
// Related: rfc7705_live_behavior_test.go -- lowLiveRouter, real sessions over net.Pipe
package reactor

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/packet"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgpserver "github.com/ze-software/ze/internal/component/bgp/server"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/exabgp/bridge"
	"github.com/ze-software/ze/internal/test/sim"
)

// downOrderReceiver wraps the reactor's receiver. For the held source it
// blocks each batch until release closes, then publishes it and counts the
// UPDATEs it published. The delivery worker is the only caller.
type downOrderReceiver struct {
	MessageReceiver
	source    netip.Addr
	entered   chan struct{}
	enterOnce sync.Once
	release   chan struct{}
	published atomic.Int64
}

func (receiver *downOrderReceiver) OnMessageBatchReceived(peer *plugin.PeerInfo, messages []bgptypes.RawMessage) []int {
	if peer.Address != receiver.source {
		return receiver.MessageReceiver.OnMessageBatchReceived(peer, messages)
	}
	receiver.enterOnce.Do(func() { close(receiver.entered) })
	<-receiver.release
	counts := receiver.MessageReceiver.OnMessageBatchReceived(peer, messages)
	receiver.published.Add(int64(len(messages)))
	return counts
}

// downOrderObserver records each down with what the test needs to judge its
// place: how many UPDATEs were published when it arrived, the peer state and
// the session's up phase at that moment.
type downOrderObserver struct {
	mu        sync.Mutex
	receiver  *downOrderReceiver
	downs     []downOrderRecord
	downAdded chan struct{}
}

type downOrderRecord struct {
	published int64
	state     PeerState
	closing   bool
	reason    string
}

func (observer *downOrderObserver) OnPeerEstablished(*Peer) {}

func (observer *downOrderObserver) OnPeerClosed(peer *Peer, closed sessionClose) {
	record := downOrderRecord{state: peer.State(), reason: closed.reason}
	if observer.receiver != nil {
		record.published = observer.receiver.published.Load()
	}
	peer.mu.RLock()
	record.closing = peer.upPhaseLocked(peer.session) == sessionUpClosing
	peer.mu.RUnlock()
	observer.mu.Lock()
	observer.downs = append(observer.downs, record)
	observer.mu.Unlock()
	select {
	case observer.downAdded <- struct{}{}:
	default: // The test reads the records; the signal only wakes it.
	}
}

func (observer *downOrderObserver) records() []downOrderRecord {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	return append([]downOrderRecord(nil), observer.downs...)
}

// sessionEndCase is one live session whose end a subtest drives. The
// reactor's receiver is wrapped so the delivery worker holds the session's
// UPDATEs, an observer records each down, and the session-end hooks record
// every leave arm and close drained once the delivery worker has exited.
type sessionEndCase struct {
	v        *lowLivePeer
	p        *Peer
	r        *Reactor
	session  *Session
	receiver *downOrderReceiver
	observer *downOrderObserver
	drained  chan struct{}
	released sync.Once

	mu sync.Mutex
	// armPaused holds, for each leave arm, whether the session's read loop
	// was paused when the arm started. A paused read loop cannot be the
	// goroutine running the arm, so true proves another goroutine drained it.
	armPaused []bool
}

func newSessionEndCase(t *testing.T, settings *PeerSettings) *sessionEndCase {
	t.Helper()
	peers := lowLiveRouter(t, settings)
	c := &sessionEndCase{v: peers[0], p: peers[0].peer, drained: make(chan struct{})}
	c.r = c.p.reactor
	c.session = c.p.currentSession()

	c.r.mu.RLock()
	prior := c.r.messageReceiver
	c.r.mu.RUnlock()
	c.receiver = &downOrderReceiver{MessageReceiver: prior, source: c.p.Settings().Address,
		entered: make(chan struct{}), release: make(chan struct{})}
	c.r.setMessageReceiver(c.receiver)
	t.Cleanup(func() {
		c.releaseReceiver()
		c.r.setMessageReceiver(prior)
	})
	c.observer = &downOrderObserver{receiver: c.receiver, downAdded: make(chan struct{}, 1)}
	c.r.addPeerObserver(c.observer)
	return c
}

// install sets the session-end hooks, wrapped so every leave arm is recorded
// and drained is closed once.
func (c *sessionEndCase) install(t *testing.T, hooks sessionEndHooks) {
	t.Helper()
	leaveArm := hooks.leaveArm
	hooks.leaveArm = func() {
		c.mu.Lock()
		c.armPaused = append(c.armPaused, c.session.IsPaused())
		c.mu.Unlock()
		if leaveArm != nil {
			leaveArm()
		}
	}
	drained := hooks.drained
	var once sync.Once
	hooks.drained = func() {
		once.Do(func() { close(c.drained) })
		if drained != nil {
			drained()
		}
	}
	c.p.sessionEndHooks.Store(&hooks)
	t.Cleanup(func() { c.p.sessionEndHooks.Store(nil) })
}

func (c *sessionEndCase) releaseReceiver() {
	c.released.Do(func() { close(c.receiver.release) })
}

func (c *sessionEndCase) accepted() uint64 {
	publication := c.p.receivePublication.Load()
	if publication == nil {
		return 0
	}
	return publication.accepted.Load()
}

// sendHeld sends n UPDATEs and returns once the read goroutine accepted them
// all and the delivery worker blocked on the first. With pause set, it then
// parks the read loop at its pause gate (parkReadLoop).
func (c *sessionEndCase) sendHeld(t *testing.T, n int, pause bool) {
	t.Helper()
	attrs := firstASAttrs(4, 65002) // RFC 4271 Section 4.3, RFC 6793 Section 3.
	for i := range n {
		prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 20, byte(i), 0}), 24)
		ownershipLiveSend(t, c.v, nil, attrs, prefix)
	}
	lowEventually(t, func() bool { return c.accepted() >= uint64(n) }, "all UPDATEs accepted")
	select {
	case <-c.receiver.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the delivery worker never reached the held receiver")
	}
	if pause {
		c.parkReadLoop()
	}
}

// parkReadLoop pauses the read loop so it never reads the connection again on
// its own, and so never fires an FSM event on a closed connection. The pause
// gate sits before the read, so a loop blocked in its read would still read
// once more: the KEEPALIVE sent here is that read, after which the loop stops
// at the gate. A loop already past its read and heading for the gate leaves
// the KEEPALIVE unread, and the write stays blocked until its deadline or the
// connection closes, which is why its error is not judged.
func (c *sessionEndCase) parkReadLoop() {
	c.session.Pause()
	written := make(chan struct{})
	keepalive := message.PackTo(message.NewKeepalive(), nil) // RFC 4271 Section 4.4.
	go func() {
		defer close(written)
		_, _ = c.v.remote.Write(keepalive) // An unread KEEPALIVE fails at close, see above.
	}()
	select {
	case <-written: // The loop read it and stops at the gate after processing it.
	case <-time.After(200 * time.Millisecond): // A loop in its read takes it at once; this one is at its gate.
	}
}

// leftWithoutDown waits for the leave arm to leave PeerStateEstablished and
// asserts the closing mark is set while no down has arrived: the receiver
// still holds the session's UPDATEs.
func (c *sessionEndCase) leftWithoutDown(t *testing.T) {
	t.Helper()
	lowEventually(t, func() bool { return c.p.State() != PeerStateEstablished }, "leave arm ran")
	c.p.mu.RLock()
	closing := c.p.upPhaseLocked(c.session) == sessionUpClosing
	c.p.mu.RUnlock()
	if !closing {
		t.Fatal("the leave arm returned without marking the session closing")
	}
	time.Sleep(50 * time.Millisecond) // A down that overtakes the held UPDATEs lands here.
	if got := c.observer.records(); len(got) != 0 {
		t.Fatalf("down announced while UPDATEs were still held: %+v", got)
	}
}

// finish releases the receiver and asserts the one down: after every
// published UPDATE, after the leave arm, with the delivery worker gone and
// runOnce returned.
func (c *sessionEndCase) finish(t *testing.T, published int64) {
	t.Helper()
	c.releaseReceiver()
	select {
	case <-c.observer.downAdded:
	case <-time.After(5 * time.Second):
		t.Fatal("no down after the held UPDATEs were released")
	}
	lowEventually(t, func() bool { return c.p.currentSession() != c.session }, "runOnce returned")
	c.oneDown(t, published)
}

// oneDown asserts exactly one down, placed after the leave arm and after the
// delivery worker exited, and the history row it owes.
func (c *sessionEndCase) oneDown(t *testing.T, published int64) {
	t.Helper()
	select {
	case <-c.drained:
	default:
		t.Fatal("the down returned before the delivery worker exited")
	}
	time.Sleep(20 * time.Millisecond) // A second down lands here.
	downs := c.observer.records()
	if len(downs) != 1 {
		t.Fatalf("downs = %d, want exactly 1: %+v", len(downs), downs)
	}
	if downs[0].published != published {
		t.Fatalf("down arrived after %d published UPDATEs, want %d", downs[0].published, published)
	}
	if downs[0].state == PeerStateEstablished {
		t.Fatal("down arrived while State() still read Established")
	}
	if !downs[0].closing {
		t.Fatal("down arrived without the closing mark")
	}
	c.mu.Lock()
	arms := len(c.armPaused)
	c.mu.Unlock()
	if arms != 1 {
		t.Fatalf("leave arms = %d, want exactly 1", arms)
	}
	sessionDownHistoryRow(t, c.p, downs[0].reason)
}

// armOffReadGoroutine asserts the one leave arm ran while the read loop was
// paused, so on a goroutine other than runOnce's.
func (c *sessionEndCase) armOffReadGoroutine(t *testing.T) {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.armPaused) != 1 || !c.armPaused[0] {
		t.Fatalf("leave arm read-loop paused = %v, want one arm run off the read goroutine", c.armPaused)
	}
}

// sendTruncatedUpdate writes a header announcing a 40-octet UPDATE and only
// three octets of its body, then closes the connection, so the read loop's
// body read fails with the FSM still Established.
//
// RFC 4271 Section 4.1: "Marker: This 16-octet field is included for
// compatibility; it MUST be set to all ones."
//
//	Byte offset: | 0-15: Marker | 16-17: Length | 18: Type | 19..: body |
func (c *sessionEndCase) sendTruncatedUpdate(t *testing.T) {
	t.Helper()
	frame := make([]byte, 0, 22)
	for range 16 {
		frame = append(frame, 0xff)
	}
	frame = append(frame, 0, 40, byte(msgtype.TypeUPDATE), 0, 0, 0)
	c.v.send(t, frame)
	_ = c.v.remote.Close()
}

// TestSessionDownFollowsBufferedDeliveries proves that a session's down is
// announced only after every UPDATE it buffered was published, and after the
// FSM leave arm marked the session closing and left PeerStateEstablished,
// whatever ends the session and whichever goroutine drains the leave arm.
//
// Method: one real session over net.Pipe per subtest. The reactor's receiver
// is held, so the delivery worker blocks on the first batch while the read
// goroutine accepts the rest. The subtest ends the session its own way. The
// leave arm runs (the closing mark is set, State() left Established) while no
// down has arrived; once the receiver is released, exactly one down arrives,
// after the delivery worker exited, carrying every published UPDATE, and the
// history row for leaving Established carries the close reason. Where the
// subtest names a draining goroutine other than the read goroutine, the read
// loop is paused at its gate, and the arm is recorded as starting while it
// was still paused. What each subtest showed at the parent of the commit
// that moved the down is recorded in the Design Insights of
// spec-bgp-rs-replacement-on-withdrawal.md.
//
// VALIDATES: the down follows the buffered deliveries (spec Data Flow 5,
// round 7 I-2, round 8 B-1 and I-3, rounds 9 and 10).
// PREVENTS: a plugin hearing a session down before that session's UPDATEs,
// a session that ends with no down, and a leave arm that blocks its drainer.
//
// test-asserts-nothing: this function only dispatches. Each subtest is a
// named function passed to t.Run, and it fails through the sessionEndCase
// methods (leftWithoutDown, finish); the sensitivity scan follows a closure
// and one helper call, not a function value, so it cannot see them.
func TestSessionDownFollowsBufferedDeliveries(t *testing.T) {
	t.Run("remote close", sessionDownRemoteClose)
	t.Run("contended local teardown", sessionDownContendedTeardown)
	t.Run("hold timer expiry", sessionDownHoldTimer)
	t.Run("run returns established on a body-read error", sessionDownBodyReadError)
	t.Run("run returns established from the pause gate on cancel", sessionDownPausedCancel)
	t.Run("read goroutine panic", sessionDownReadPanic)
	t.Run("teardown races the down to idle", sessionDownIdleRace)
	t.Run("bfd down drained on the subscriber", sessionDownBFDSubscriber)
	t.Run("reactor stop with a paused session", sessionDownReactorStop)
	t.Run("gr congestion teardown on the forward worker", sessionDownCongestionWorker)
}

// sessionDownRemoteClose is the uncontended case: the remote closes with no
// NOTIFICATION and the read goroutine drains its own leave arm.
func sessionDownRemoteClose(t *testing.T) {
	c := newSessionEndCase(t, lowLiveSettings("10.0.0.1", 65000, 65002))
	c.install(t, sessionEndHooks{})
	c.sendHeld(t, 3, false)
	_ = c.v.remote.Close() // The remote ends the session with no NOTIFICATION.
	c.leftWithoutDown(t)
	c.finish(t, 3)
}

// sessionDownContendedTeardown is case (1): Session.Teardown runs on another
// goroutine while the read loop is paused, and the leave arm on that
// goroutine is held until runOnce's Session.Run returned and the delivery
// worker exited. The down must still wait on the leave signal.
func sessionDownContendedTeardown(t *testing.T) {
	c := newSessionEndCase(t, lowLiveSettings("10.0.0.1", 65000, 65002))
	armEntered := make(chan struct{})
	var heldDowns atomic.Int64
	heldDowns.Store(-1)
	c.install(t, sessionEndHooks{leaveArm: func() {
		close(armEntered)
		select {
		case <-c.drained:
		case <-time.After(5 * time.Second):
			return // The test fails on the missing down.
		}
		time.Sleep(50 * time.Millisecond) // A down that skipped the leave signal lands here.
		heldDowns.Store(int64(len(c.observer.records())))
	}})
	c.sendHeld(t, 2, true)

	teardownDone := make(chan error, 1)
	go func() { teardownDone <- c.session.Teardown(message.NotifyCeaseAdminShutdown, "") }()
	select {
	case <-armEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("the teardown never ran the leave arm")
	}
	// The read loop leaves its gate only now, after the arm started on the
	// teardown goroutine: Run returns the teardown's close reason.
	c.session.Resume()
	c.finish(t, 2)
	if err := <-teardownDone; err != nil {
		t.Fatalf("teardown: %v", err)
	}
	if got := heldDowns.Load(); got != 0 {
		t.Fatalf("downs while the leave arm was held after the drain = %d, want 0", got)
	}
	c.armOffReadGoroutine(t)
}

// sessionDownHoldTimer is case (2): the hold timer expires while the read
// loop is paused, so the timer goroutine drains the leave arm; the remote
// closes after reading the NOTIFICATION.
func sessionDownHoldTimer(t *testing.T) {
	settings := lowLiveSettings("10.0.0.1", 65000, 65002)
	settings.ReceiveHoldTime = 3 * time.Second // RFC 4271 Section 4.2: the smallest non-zero Hold Time.
	c := newSessionEndCase(t, settings)
	c.install(t, sessionEndHooks{})
	c.sendHeld(t, 2, true)

	// RFC 4271 Section 6.5: "If a system does not receive successive
	// KEEPALIVE, UPDATE, and/or NOTIFICATION messages within the period
	// specified in the Hold Time field of the OPEN message, then the
	// NOTIFICATION message with the Hold Timer Expired Error Code is sent".
	lowEventually(t, func() bool {
		c.v.mu.Lock()
		defer c.v.mu.Unlock()
		for _, frame := range c.v.frames {
			if frame[18] == byte(msgtype.TypeNOTIFICATION) && frame[19] == byte(message.NotifyHoldTimerExpired) {
				return true
			}
		}
		return false
	}, "Hold Timer Expired NOTIFICATION")
	c.leftWithoutDown(t)
	_ = c.v.remote.Close() // The peer closes after the NOTIFICATION.
	c.finish(t, 2)
	c.armOffReadGoroutine(t)
}

// sessionDownBodyReadError is case (3), first path: Session.Run returns on a
// body-read error with the FSM still Established, so the down fires the leave
// itself.
func sessionDownBodyReadError(t *testing.T) {
	c := newSessionEndCase(t, lowLiveSettings("10.0.0.1", 65000, 65002))
	c.install(t, sessionEndHooks{})
	c.sendHeld(t, 2, false)
	c.sendTruncatedUpdate(t)
	c.leftWithoutDown(t)
	c.finish(t, 2)
}

// sessionDownPausedCancel is case (3), second path: the peer's context is
// canceled while the read loop waits at its pause gate, so waitForResume
// returns and Run returns with the FSM still Established.
func sessionDownPausedCancel(t *testing.T) {
	c := newSessionEndCase(t, lowLiveSettings("10.0.0.1", 65000, 65002))
	c.install(t, sessionEndHooks{})
	c.sendHeld(t, 2, true)
	c.p.Stop() // Cancels the peer context only: no NOTIFICATION, no FSM event.
	c.leftWithoutDown(t)
	c.finish(t, 2)
}

// sessionDownReadPanic is case (4): the read goroutine panics after the
// route-server fast path returned and before the hand-off to the delivery
// worker. The panic reaches safeRunOnce's recover through the down, which
// still drains and announces, and the receive publication reads failed.
func sessionDownReadPanic(t *testing.T) {
	c := newSessionEndCase(t, lowLiveSettings("10.0.0.1", 65000, 65002))
	var handOffs atomic.Int64
	c.install(t, sessionEndHooks{beforeHandOff: func() {
		if handOffs.Add(1) == 3 {
			panic("test: read goroutine panic before the delivery hand-off")
		}
	}})
	publication := c.p.receivePublication.Load()
	attrs := firstASAttrs(4, 65002) // RFC 4271 Section 4.3, RFC 6793 Section 3.
	for i := range 3 {
		prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 20, byte(i), 0}), 24)
		ownershipLiveSend(t, c.v, nil, attrs, prefix)
	}
	c.leftWithoutDown(t)
	c.finish(t, 2)
	if !publication.failed.Load() {
		t.Fatal("the down's publication does not read failed after the read-goroutine panic")
	}
}

// sessionDownIdleRace is case (5): on case (3)'s path, a local teardown moves
// the FSM to Idle after the down read Established and before it fires its
// event, so that event finds Idle and exactly one leave arm runs.
func sessionDownIdleRace(t *testing.T) {
	c := newSessionEndCase(t, lowLiveSettings("10.0.0.1", 65000, 65002))
	teardownErr := make(chan error, 1)
	c.install(t, sessionEndHooks{beforeLeaveFire: func() {
		teardownErr <- c.session.Teardown(message.NotifyCeaseAdminShutdown, "")
	}})
	c.sendHeld(t, 2, false)
	c.sendTruncatedUpdate(t)
	c.leftWithoutDown(t)
	c.finish(t, 2)
	select {
	case err := <-teardownErr:
		if err != nil {
			t.Fatalf("teardown: %v", err)
		}
	default:
		t.Fatal("the down never reached the hook between reading Established and firing")
	}
}

// sessionDownBFDSubscriber is case (6): a non-strict BFD peer receives a BFD
// Down while its read loop is paused, so the BFD subscriber goroutine drains
// the leave arm. The subscriber must exit and the down must arrive.
func sessionDownBFDSubscriber(t *testing.T) {
	svc := &fakeBFDService{}
	api.SetService(svc)
	t.Cleanup(func() { api.SetService(nil) })
	settings := lowLiveSettings("10.0.0.1", 65000, 65002)
	settings.BFD = &BFDSettings{Enabled: true}
	c := newSessionEndCase(t, settings)
	c.install(t, sessionEndHooks{})

	var handle *fakeBFDHandle
	var done chan struct{}
	lowEventually(t, func() bool {
		c.p.bfd.mu.Lock()
		defer c.p.bfd.mu.Unlock()
		h, ok := c.p.bfd.handle.(*fakeBFDHandle)
		if !ok {
			return false
		}
		handle, done = h, c.p.bfd.done
		return done != nil
	}, "BFD client started on Established")
	c.sendHeld(t, 2, true)

	// RFC 5882 Section 3.2: "When a BFD session transitions from Up to Down,
	// action SHOULD be taken in the control protocol to signal the lack of
	// connectivity for the path over which BFD is running."
	handle.emit(t, packet.StateDown, packet.DiagControlDetectExpired)
	c.leftWithoutDown(t)
	c.finish(t, 2)
	c.armOffReadGoroutine(t)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the BFD subscriber goroutine never exited")
	}
}

// sessionDownReactorStop is case (7): StopForRestart while the session waits
// at its pause gate. Reactor.Wait returns within cleanup's whole r.mu hold,
// then the peer's own Wait returns, and exactly one down arrived for the
// session.
func sessionDownReactorStop(t *testing.T) {
	c := newSessionEndCase(t, lowLiveSettings("10.0.0.1", 65000, 65002))
	c.install(t, sessionEndHooks{})
	c.sendHeld(t, 2, true)
	c.releaseReceiver() // The stop, not held UPDATEs, is under test here.
	lowEventually(t, func() bool { return c.receiver.published.Load() == 2 }, "UPDATEs published")

	c.r.StopForRestart()
	// cleanup's Phase 1 plus Phase 2's 2 s, with a margin; the test reactor
	// borrows its plugin server, so Phase 1 stops no r.api.
	reactorCtx, reactorCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer reactorCancel()
	if err := c.r.Wait(reactorCtx); err != nil {
		t.Fatalf("Reactor.Wait: %v", err)
	}
	peerCtx, peerCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer peerCancel()
	if err := c.p.Wait(peerCtx); err != nil {
		t.Fatalf("Peer.Wait: %v", err)
	}
	c.oneDown(t, 2)
}

// sessionDownCongestionWorker is case (8): a forward-pool worker runs
// congestionTeardownPeer's GR branch from checkTeardown with the grace
// expired, while the read loop is paused, so the worker drains the leave arm.
// The worker then goes on to its next batch and the down arrives.
func sessionDownCongestionWorker(t *testing.T) {
	c := newSessionEndCase(t, lowLiveSettings("10.0.0.1", 65000, 65002))
	c.install(t, sessionEndHooks{})
	c.sendHeld(t, 2, true)

	handled := make(chan int, 4)
	pool := newFwdPool(func(_ fwdKey, items []fwdItem) { handled <- len(items) },
		fwdPoolConfig{chanSize: 16, idleTimeout: time.Minute})
	t.Cleanup(pool.Stop)
	address := c.p.Settings().Address
	key := fwdKey{peerAddr: netip.AddrPortFrom(address, 179)}
	pool.registerOutgoingPool(key, message.MaxMsgLen)
	weights := newWeightTracker(nil)
	weights.AddPeer(address.String(), 10, 1)
	fake := sim.NewFakeClock(time.Now())
	pool.congestion = newCongestionController(congestionConfig{
		gracePeriod:    time.Second,
		poolUsedRatio:  func() float64 { return 0.99 },
		overflowDepths: func() map[string]int { return map[string]int{address.String(): 1_000_000} },
		weights:        weights,
		clock:          fake,
		peerGRCapable:  func(string) bool { return true },
		onTeardown:     congestionTeardownPeer(func(netip.AddrPort) *Peer { return c.p }),
	})
	batch := func(what string) {
		t.Helper()
		pool.Dispatch(key, &fwdItem{})
		select {
		case <-handled:
		case <-time.After(5 * time.Second):
			t.Fatalf("the forward worker never ran the %s batch", what)
		}
	}
	batch("first")
	// The handler returns before the worker's checkTeardown runs, so wait
	// for that check to start the grace before the clock moves past it.
	lowEventually(t, func() bool {
		pool.congestion.mu.Lock()
		defer pool.congestion.mu.Unlock()
		return pool.congestion.teardownPeer == address.String()
	}, "congestion grace started")
	fake.Add(2 * time.Second)
	batch("second") // checkTeardown fires the GR teardown after this batch.

	c.leftWithoutDown(t)
	c.armOffReadGoroutine(t)
	// The GR branch closes the connection and fires Event 18 without
	// signaling Run, so the read loop leaves its gate only when resumed.
	c.session.Resume()
	batch("third") // The worker went on past the teardown.
	c.finish(t, 2)
}

// sessionDownHistoryRow asserts that the newest history row leaving
// Established carries the down's reason.
func sessionDownHistoryRow(t *testing.T, p *Peer, reason string) {
	t.Helper()
	rows := p.history.snapshot()
	for _, row := range slices.Backward(rows) {
		if row.From != fsm.StateEstablished.String() {
			continue
		}
		if row.Reason != reason {
			t.Fatalf("leave-Established history reason = %q, want %q", row.Reason, reason)
		}
		return
	}
	t.Fatalf("no leave-Established history row in %+v", rows)
}

// newNegotiatedLifetimePeer wires a real peer, dispatcher and IPC subscriber.
func newNegotiatedLifetimePeer(t *testing.T, settings *PeerSettings) (*Peer, *stateLog, *mockLifecycleObserver) {
	t.Helper()
	if err := EnsureProcessBinding(settings, "monitor", "open-received negotiated state", ""); err != nil {
		t.Fatal(err)
	}
	r := New(&Config{ListenAddr: "127.0.0.1:0"})
	if err := r.AddPeer(settings); err != nil {
		t.Fatal(err)
	}
	p := r.peers[settings.PeerKey()]
	p.ctx, p.cancel = context.WithCancel(t.Context())
	t.Cleanup(p.cancel)
	srv := flowForwardPluginServer(t, r)
	r.api = srv
	dispatcher := bgpserver.NewEventDispatcher(srv)
	r.eventDispatcher = dispatcher
	r.setMessageReceiver(dispatcher)
	observer := &mockLifecycleObserver{}
	r.addPeerObserver(observer)
	r.addPeerObserver(&apiStateObserver{dispatcher: dispatcher, reactor: r})

	engineSide, pluginSide := net.Pipe()
	proc := process.NewProcess(plugin.PluginConfig{Name: "monitor"})
	proc.SetConn(plugipc.NewPluginConn(engineSide, engineSide))
	proc.StartDelivery(t.Context())
	t.Cleanup(func() {
		proc.Stop()
		if err := pluginSide.Close(); err != nil {
			t.Log(err)
		}
	})
	for _, kind := range []string{bgpevents.EventOpen, bgpevents.EventNegotiated, bgpevents.EventState} {
		direction := events.DirUnspecified
		if kind == bgpevents.EventOpen {
			direction = events.DirReceived
		}
		srv.Subscriptions().Add(proc, &pluginserver.Subscription{
			Namespace: events.LookupNamespaceID(bgpevents.Namespace),
			EventType: events.LookupEventTypeID(kind),
			Direction: direction,
		})
	}
	r.mu.Lock()
	r.publishDeliveryGraphLocked()
	r.mu.Unlock()
	log := &stateLog{}
	go answerStateEvents(t.Context(), plugipc.NewPluginConn(pluginSide, pluginSide), log, nil)
	return p, log, observer
}

// TestOpenConfirmFailureEndsNegotiatedCache drives two real runOnce handshakes
// through the dispatcher and replays the process's events through the bridge.
// The first fails before Established; the next OPEN must carry no old block.
// A third connection fails before negotiation and must not invent another down.
func TestOpenConfirmFailureEndsNegotiatedCache(t *testing.T) {
	p, log, observer := newNegotiatedLifetimePeer(t, lowLiveSettings("192.0.2.1", 65000, 65002))

	for cycle := range 3 {
		done := make(chan error, 1)
		go func() { done <- p.runOnce() }()
		lowEventually(t, func() bool { return p.SessionState() == fsm.StateActive }, "passive session")
		server, remote := net.Pipe()
		v := &lowLivePeer{peer: p, remote: remote, stopped: make(chan struct{})}
		v.startCapture(t)
		t.Cleanup(func() {
			_ = remote.Close()
			_ = server.Close()
			<-v.stopped
		})
		if err := p.acceptConnection(server); err != nil {
			t.Fatal(err)
		}
		if cycle < 2 {
			open := &message.Open{Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x0a000001,
				OptionalParams: []byte{2, 6, 65, 4, 0, 0, 0xfd, 0xea, 2, 6, 1, 4, 0, 1, 0, 1}}
			v.send(t, message.PackTo(open, nil))
			// Reading the answering KEEPALIVE proves negotiated delivery returned,
			// rather than merely seeing the FSM state before its callback ran.
			lowEventually(t, func() bool {
				v.mu.Lock()
				defer v.mu.Unlock()
				for _, frame := range v.frames {
					if frame[18] == byte(msgtype.TypeKEEPALIVE) {
						return true
					}
				}
				return false
			}, "OpenConfirm KEEPALIVE")
		}
		if err := remote.Close(); err != nil {
			t.Fatal(err)
		}
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("session teardown did not return")
		}
	}
	if observer.establishedCount() != 0 || observer.closedCount() != 0 {
		t.Fatal("pre-Established failure called Established lifecycle observers")
	}

	var sessions bridge.Sessions
	opens, negotiated, downs := 0, 0, 0
	for _, payload := range log.recorded() {
		var batch struct {
			Events []string `json:"events"`
		}
		if err := json.Unmarshal([]byte(payload), &batch); err != nil {
			t.Fatal(err)
		}
		for _, line := range batch.Events {
			var data map[string]any
			if err := json.Unmarshal([]byte(line), &data); err != nil {
				t.Fatal(err)
			}
			event := bridge.ReadEvent(data)
			sessions.Note(&event)
			switch event.Kind {
			case "open":
				opens++
				if event.Negotiated != nil {
					t.Fatalf("OPEN %d inherited previous session capabilities: %v", opens, event.Negotiated)
				}
			case "negotiated":
				negotiated++
				probe := bridge.Event{Kind: "keepalive", Peer: event.Peer}
				sessions.Note(&probe)
				if probe.Negotiated == nil {
					t.Fatal("negotiated event did not populate bridge cache")
				}
			case "state":
				if event.Payload["state"] == "down" {
					downs++
				} else {
					t.Fatalf("unexpected pre-Established state: %v", event.Payload)
				}
			}
		}
	}
	if opens != 2 || negotiated != 2 || downs != 2 {
		t.Fatalf("events: OPEN=%d negotiated=%d down=%d; want 2 each", opens, negotiated, downs)
	}
}

// TestRetainedOpenFailureEndsNegotiatedCache fails the answering KEEPALIVE on
// the retained collision-winner rail, before runOnce starts its read worker.
func TestRetainedOpenFailureEndsNegotiatedCache(t *testing.T) {
	p, log, observer := newNegotiatedLifetimePeer(t, lowLiveSettings("192.0.2.1", 65000, 65002))
	open := &message.Open{Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x0a000001,
		OptionalParams: []byte{2, 6, 65, 4, 0, 0, 0xfd, 0xea}}
	conn := &failAfterConn{writesLeft: 1}
	p.mu.Lock()
	p.inbound = inboundConnection{conn: conn, open: open, wire: message.PackTo(open, nil)}
	p.collisionHandoff = true
	p.mu.Unlock()
	if err := p.runOnce(); err == nil {
		t.Fatal("retained OPEN's answering KEEPALIVE did not fail")
	}
	if observer.establishedCount() != 0 || observer.closedCount() != 0 {
		t.Fatal("retained OPEN failure called Established lifecycle observers")
	}
	requireNegotiatedLifetimeEnd(t, log, p.addrString)
}

// TestBFDNegotiatedPublicationJoinsSessionEnd pauses the actual BFD subscriber
// after publication admission but before its phase mark. Closing the socket
// must join that publisher, then send down, and reject a queued late callback.
func TestBFDNegotiatedPublicationJoinsSessionEnd(t *testing.T) {
	svc := &fakeBFDService{}
	api.SetService(svc)
	t.Cleanup(func() { api.SetService(nil) })
	settings := lowLiveSettings("192.0.2.1", 65000, 65002)
	settings.BFD = &BFDSettings{Enabled: true, Strict: true, HoldTime: 90}
	settings.Capabilities = append(settings.Capabilities, &capability.BFDStrictMode{})
	p, log, observer := newNegotiatedLifetimePeer(t, settings)
	t.Cleanup(p.stopBFDClient)
	entered, release, drained := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	p.sessionEndHooks.Store(&sessionEndHooks{
		beforeNegotiated: func() {
			close(entered)
			<-release
		},
		drained: func() { close(drained) },
	})
	done := make(chan error, 1)
	go func() { done <- p.runOnce() }()
	lowEventually(t, func() bool { return p.SessionState() == fsm.StateActive }, "strict passive session")
	session := p.currentSession()
	server, remote := net.Pipe()
	v := &lowLivePeer{peer: p, remote: remote, stopped: make(chan struct{})}
	v.startCapture(t)
	t.Cleanup(func() {
		_ = remote.Close()
		_ = server.Close()
		<-v.stopped
	})
	if err := p.acceptConnection(server); err != nil {
		t.Fatal(err)
	}
	open := &message.Open{Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x0a000001,
		OptionalParams: []byte{2, 8, 65, 4, 0, 0, 0xfd, 0xea, 74, 0}}
	v.send(t, message.PackTo(open, nil))
	lowEventually(t, func() bool {
		return session.fsm.BfdSubState() == fsm.SubStateOpenSentBfdUpPending
	}, "OPEN waiting for BFD")
	p.bfd.mu.Lock()
	handle, ok := p.bfd.handle.(*fakeBFDHandle)
	p.bfd.mu.Unlock()
	if !ok {
		t.Fatal("strict peer did not start the BFD subscriber")
	}
	handle.emit(t, packet.StateUp, packet.DiagNone)
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("BFD subscriber did not enter negotiation publication")
	}
	if err := remote.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-drained:
	case <-time.After(5 * time.Second):
		t.Fatal("read loop did not finish while publication was paused")
	}
	lowEventually(t, func() bool {
		session.mu.RLock()
		defer session.mu.RUnlock()
		return session.negotiationEnded
	}, "publication admission sealed")
	select {
	case <-done:
		t.Fatal("runOnce returned ahead of its admitted negotiation publication")
	default:
	}
	unblock()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("session end did not join the released publisher")
	}
	if observer.establishedCount() != 0 || observer.closedCount() != 0 {
		t.Fatal("BFD OpenConfirm failure called Established lifecycle observers")
	}
	requireNegotiatedLifetimeEnd(t, log, p.addrString)
	before := len(log.recorded())
	p.announceNegotiated(p.reactor, session, session.Negotiated())
	if len(log.recorded()) != before {
		t.Fatal("queued late callback republished negotiation after session end")
	}
	// A callback still queued before admission must also be refused, not
	// just a duplicate of the publication that already completed above.
	unpublished := NewSession(settings)
	p.endNegotiatedSession(unpublished)
	p.announceNegotiated(p.reactor, unpublished, session.Negotiated())
	if len(log.recorded()) != before {
		t.Fatal("callback admitted after an unpublished session ended")
	}
}

// requireNegotiatedLifetimeEnd feeds the actual IPC events through the bridge,
// proving positive cache population, ordered down, and an empty reconnect OPEN.
func requireNegotiatedLifetimeEnd(t *testing.T, log *stateLog, peer string) {
	t.Helper()
	var sessions bridge.Sessions
	negotiated, downs := 0, 0
	for _, payload := range log.recorded() {
		var batch struct {
			Events []string `json:"events"`
		}
		if err := json.Unmarshal([]byte(payload), &batch); err != nil {
			t.Fatal(err)
		}
		for _, line := range batch.Events {
			var data map[string]any
			if err := json.Unmarshal([]byte(line), &data); err != nil {
				t.Fatal(err)
			}
			event := bridge.ReadEvent(data)
			sessions.Note(&event)
			switch event.Kind {
			case "negotiated":
				negotiated++
				if downs != 0 {
					t.Fatal("negotiation published after down")
				}
				probe := bridge.Event{Kind: "keepalive", Peer: peer}
				sessions.Note(&probe)
				if probe.Negotiated == nil {
					t.Fatal("negotiation did not populate the bridge cache")
				}
			case "state":
				if event.Payload["state"] != "down" {
					t.Fatalf("unexpected state: %v", event.Payload)
				}
				downs++
			}
		}
	}
	if negotiated != 1 || downs != 1 {
		t.Fatalf("negotiated=%d down=%d, want one each", negotiated, downs)
	}
	open := bridge.Event{Kind: "open", Peer: peer}
	sessions.Note(&open)
	if open.Negotiated != nil {
		t.Fatal("reconnect OPEN inherited ended session capabilities")
	}
}
