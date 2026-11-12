// Design: docs/architecture/l2tp/subscriber-session-model.md -- the order of one session's notifications

// VALIDATES: spec-single-owner-lifecycle-5-session-end-locals AC-7, AC-8 at
// the subscriber bridge, AC-10, AC-18, A-5 and R-3: the notifications of one
// L2TP session reach their readers in the order the session's state changed
// under tunnelsMu, no lock is held while a reader runs, and every order ticket
// a path takes is finished.
// PREVENTS: a teardown on another goroutine publishing SessionDown before the
// SessionIPAssigned or SessionUp of the same session. The late event then
// re-creates the subscriber entry, starts an accounting session nobody stops,
// and re-injects the subscriber route.
//
// Method: a reader holds the first notification of one path on that path's
// own goroutine, the way a slow reader holds the reactor goroutine. A second
// path then changes the same session on another goroutine, as an operator,
// timeout or CoA teardown does. A reader that receives the second path's
// notification while the first is still held records it as overtaking. The
// interleaving cannot be forced through a daemon, so this is the controlled
// seam `ai/rules/testing.md` allows.

package l2tp

import (
	"log/slog"
	"net/netip"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	"github.com/ze-software/ze/pkg/ze"
)

// orderTestBound bounds every wait in this file. Reaching it is a hang the
// test reports, never a timing a test depends on: each green path returns as
// soon as its channel closes.
const orderTestBound = 10 * time.Second

// orderTestTID and orderTestSID name the session whose first notification a
// gate holds.
const (
	orderTestTID uint16 = 11
	orderTestSID uint16 = 7
)

var (
	orderTestPeer      = netip.MustParseAddrPort("10.0.0.9:1701")
	orderTestOtherPeer = netip.MustParseAddrPort("10.0.0.10:1701")
)

// awaitOrderEvent waits until ch closes, and fails the test when it has not
// closed within orderTestBound.
func awaitOrderEvent(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(orderTestBound):
		t.Fatalf("%s did not happen within %s", what, orderTestBound)
	}
}

// startOrderPath runs path on a goroutine of its own, as the reactor
// goroutine and an operator, timeout or CoA goroutine each run one, and
// returns a channel that closes when path returns.
func startOrderPath(path func()) <-chan struct{} {
	done := make(chan struct{})
	go runOrderPath(done, path)
	return done
}

func runOrderPath(done chan<- struct{}, path func()) {
	defer close(done)
	path()
}

// sessionOrderTopic is the (namespace, event type) pair a handler subscribed to.
type sessionOrderTopic struct {
	namespace string
	eventType string
}

// sessionOrderBus is a synchronous EventBus. Emit runs every handler
// subscribed to its namespace and event type, in subscription order, on the
// emitting goroutine, as the engine bus runs an in-process reader.
// Safe for concurrent use.
type sessionOrderBus struct {
	mu       sync.Mutex
	handlers map[sessionOrderTopic][]func(any)
}

var _ ze.EventBus = (*sessionOrderBus)(nil)

func newSessionOrderBus() *sessionOrderBus {
	return &sessionOrderBus{handlers: map[sessionOrderTopic][]func(any){}}
}

func (b *sessionOrderBus) Emit(namespace, eventType string, payload any) (int, error) {
	b.mu.Lock()
	handlers := slices.Clone(b.handlers[sessionOrderTopic{namespace: namespace, eventType: eventType}])
	b.mu.Unlock()
	for _, handler := range handlers {
		handler(payload)
	}
	return 0, nil
}

func (b *sessionOrderBus) Subscribe(namespace, eventType string, handler func(any)) func() {
	topic := sessionOrderTopic{namespace: namespace, eventType: eventType}
	b.mu.Lock()
	b.handlers[topic] = append(b.handlers[topic], handler)
	b.mu.Unlock()
	return func() {}
}

// sessionNotice is one session notification as a reader received it.
type sessionNotice struct {
	event     string
	tunnelID  uint16
	sessionID uint16
}

// sessionNoticeLog is a reader of the three L2TP session events that records
// what it received, in order. Subscribed after the other readers, it receives
// each notification after they did. Safe for concurrent use.
type sessionNoticeLog struct {
	mu   sync.Mutex
	seen []sessionNotice
}

func subscribeSessionNoticeLog(bus ze.EventBus) *sessionNoticeLog {
	notices := &sessionNoticeLog{}
	l2tpevents.SessionIPAssigned.Subscribe(bus, func(p *l2tpevents.SessionIPAssignedPayload) {
		notices.add(l2tpevents.SessionIPAssignedEvent, p.TunnelID, p.SessionID)
	})
	l2tpevents.SessionUp.Subscribe(bus, func(p *l2tpevents.SessionUpPayload) {
		notices.add(l2tpevents.SessionUpEvent, p.TunnelID, p.SessionID)
	})
	l2tpevents.SessionDown.Subscribe(bus, func(p *l2tpevents.SessionDownPayload) {
		notices.add(l2tpevents.SessionDownEvent, p.TunnelID, p.SessionID)
	})
	return notices
}

func (l *sessionNoticeLog) add(event string, tunnelID, sessionID uint16) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seen = append(l.seen, sessionNotice{event: event, tunnelID: tunnelID, sessionID: sessionID})
}

// of returns the events received for one session, in the order received.
func (l *sessionNoticeLog) of(tunnelID, sessionID uint16) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var events []string
	for _, n := range l.seen {
		if n.tunnelID == tunnelID && n.sessionID == sessionID {
			events = append(events, n.event)
		}
	}
	return events
}

// sessionOrderGate holds the first notification of one tunnel at the reader
// that calls hold, and records each later notification of that tunnel that
// reaches a reader calling observe while the first is still held. Such a
// notification has overtaken the held one, so observe also lets the held one
// go: the test then fails on what was recorded rather than hanging.
// Safe for concurrent use.
type sessionOrderGate struct {
	tunnelID uint16
	entered  chan struct{} // closed when the held notification reached hold
	release  chan struct{} // closed by open, to let the held notification go on
	openOnce sync.Once

	mu        sync.Mutex
	taken     bool     // the first notification of the tunnel was held already
	holding   bool     // the held notification waits in hold
	overtaken []string // later notifications that arrived while it waited
}

// newSessionOrderGate builds a gate for tunnel orderTestTID. A test that fails
// before it opens the gate still releases the held goroutine when it ends.
func newSessionOrderGate(t *testing.T) *sessionOrderGate {
	t.Helper()
	g := &sessionOrderGate{
		tunnelID: orderTestTID,
		entered:  make(chan struct{}),
		release:  make(chan struct{}),
	}
	t.Cleanup(g.open)
	return g
}

// hold waits until open is called, for the first notification of the gate's
// tunnel only. Every other call returns at once.
func (g *sessionOrderGate) hold(tunnelID uint16) {
	g.mu.Lock()
	if tunnelID != g.tunnelID {
		g.mu.Unlock()
		return
	}
	if g.taken {
		g.mu.Unlock()
		return
	}
	g.taken = true
	g.holding = true
	g.mu.Unlock()

	close(g.entered)
	<-g.release

	g.mu.Lock()
	g.holding = false
	g.mu.Unlock()
}

// observe records what as overtaking when a notification of the gate's
// tunnel is held, and then opens the gate.
func (g *sessionOrderGate) observe(what string, tunnelID uint16) {
	g.mu.Lock()
	overtook := g.holding && tunnelID == g.tunnelID
	if overtook {
		g.overtaken = append(g.overtaken, what)
	}
	g.mu.Unlock()
	if overtook {
		g.open()
	}
}

func (g *sessionOrderGate) open() {
	g.openOnce.Do(func() { close(g.release) })
}

func (g *sessionOrderGate) overtakers() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return slices.Clone(g.overtaken)
}

// openWhenIssued is the gate's second release: it opens the gate once the
// gate's tunnel in r has issued two order tickets, the held notification's
// and the later path's. The later path issues its ticket under tunnelsMu, so
// by then it has changed the session and is about to wait behind the held
// notification. A reactor that orders its notifications never lets the later
// one overtake, so observe never opens the gate and the held notification
// would wait for ever; a reactor that does not order them lets the later one
// through to observe, which records it, whichever release came first. Call it
// once the held notification has entered the gate: its path has issued the
// tunnel's first ticket by then.
func (g *sessionOrderGate) openWhenIssued(t *testing.T, r *l2tpReactor) {
	t.Helper()
	r.tunnelsMu.Lock()
	tun := r.tunnelsByLocalID[g.tunnelID]
	require.NotNil(t, tun, "tunnel %d is not in the reactor", g.tunnelID)
	order := tun.noticeOrder
	r.tunnelsMu.Unlock()
	require.NotNil(t, order, "the held notification of tunnel %d took no order ticket", g.tunnelID)

	watch := &issueWatch{order: order, tickets: 2}
	go watch.openAfterIssue(g)
	t.Cleanup(watch.stop)
}

// issueWatch waits for a tunnel's order to issue a number of tickets. Its
// stopped flag is guarded by the order's own mutex, on whose condition the
// watch waits, so stop wakes it.
type issueWatch struct {
	order   *sessionNoticeOrder
	tickets uint64
	stopped bool
}

func (w *issueWatch) openAfterIssue(g *sessionOrderGate) {
	w.order.mu.Lock()
	for w.order.issued < w.tickets && !w.stopped {
		w.order.turn.Wait()
	}
	reached := w.order.issued >= w.tickets
	w.order.mu.Unlock()
	if reached {
		g.open()
	}
}

func (w *issueWatch) stop() {
	w.order.mu.Lock()
	w.stopped = true
	w.order.turn.Broadcast()
	w.order.mu.Unlock()
}

// ticketCounts returns how many tickets tun's order issued and finished.
func ticketCounts(r *l2tpReactor, tun *L2TPTunnel) (issued, finished uint64) {
	r.tunnelsMu.Lock()
	order := tun.noticeOrder
	r.tunnelsMu.Unlock()
	if order == nil {
		return 0, 0
	}
	order.mu.Lock()
	defer order.mu.Unlock()
	return order.issued, order.finished
}

// gatedRouteObserver is a route observer that holds an IP-up at its gate and
// reports each withdrawal to it, before it hands both to the real observer.
type gatedRouteObserver struct {
	gate  *sessionOrderGate
	inner *subscriberRouteObserver
}

func (o *gatedRouteObserver) OnSessionIPUp(tunnelID, sessionID uint16, username string, addr netip.Addr) {
	o.gate.hold(tunnelID)
	o.inner.OnSessionIPUp(tunnelID, sessionID, username, addr)
}

func (o *gatedRouteObserver) OnSessionDown(tunnelID, sessionID uint16) {
	o.gate.observe("route withdrawal", tunnelID)
	o.inner.OnSessionDown(tunnelID, sessionID)
}

// newReactorForEventOrder builds a reactor that publishes on bus and has a
// listener for the CDN and StopCCN its teardown paths send. The listener is
// never started, so a send fails and is logged, which no test here reads.
func newReactorForEventOrder(t *testing.T, bus ze.EventBus) *l2tpReactor {
	t.Helper()
	r := newReactorForSnapshot(t)
	r.listener = newUDPListener(netip.MustParseAddrPort("127.0.0.1:0"), slog.Default())
	r.eventBus = bus
	return r
}

// orderTestSession is an established session with its PPP interface, so each
// of its three notifications is published.
func orderTestSession(sid uint16, pppInterface string) *L2TPSession {
	return &L2TPSession{
		localSID:     sid,
		remoteSID:    sid + 100,
		state:        L2TPSessionEstablished,
		username:     "grace",
		pppInterface: pppInterface,
	}
}

// orderTestIPv4 is what PPP reports when IPCP assigns session orderTestSID of
// tunnel orderTestTID its address.
func orderTestIPv4() ppp.EventSessionIPAssigned {
	return ppp.EventSessionIPAssigned{
		TunnelID:  orderTestTID,
		SessionID: orderTestSID,
		Username:  "grace",
		Family:    ppp.AddressFamilyIPv4,
		Peer:      netip.MustParseAddr("192.0.2.7"),
	}
}

// acknowledgeableIPAssigned gives ev the acknowledgement channel PPP's
// sendIPAssigned gives it, and returns that channel. ppp keeps the field
// unexported so that only PPP makes one. This test stands in for PPP, and
// writes the field the one way Go allows from another package.
func acknowledgeableIPAssigned(t *testing.T, ev ppp.EventSessionIPAssigned) (ppp.EventSessionIPAssigned, <-chan struct{}) {
	t.Helper()
	done := make(chan struct{})
	field := reflect.ValueOf(&ev).Elem().FieldByName("done")
	require.True(t, field.IsValid(), "ppp.EventSessionIPAssigned no longer holds its acknowledgement channel in the field done")
	//nolint:gosec // G103: the test stands in for PPP, the one writer of the unexported acknowledgement channel.
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(done))
	return ev, done
}

// TestSessionIPAssignedIsPublishedBeforeTheTeardownThatFollowsIt proves that a
// teardown on another goroutine delivers its SessionDown only after the
// SessionIPAssigned that changed the session before it. A reader holds the
// IP-assigned publication; the operator teardown of the same session then
// runs on its own goroutine.
func TestSessionIPAssignedIsPublishedBeforeTheTeardownThatFollowsIt(t *testing.T) {
	bus := newSessionOrderBus()
	gate := newSessionOrderGate(t)
	l2tpevents.SessionIPAssigned.Subscribe(bus, func(p *l2tpevents.SessionIPAssignedPayload) { gate.hold(p.TunnelID) })
	l2tpevents.SessionDown.Subscribe(bus, func(p *l2tpevents.SessionDownPayload) {
		gate.observe(l2tpevents.SessionDownEvent, p.TunnelID)
	})
	notices := subscribeSessionNoticeLog(bus)
	r := newReactorForEventOrder(t, bus)
	insertEstablishedTunnel(t, r, 11, 22, orderTestPeer, orderTestSession(7, "ppp7"))

	assigned := startOrderPath(func() { r.handleSessionIPAssigned(orderTestIPv4()) })
	awaitOrderEvent(t, gate.entered, "the SessionIPAssigned publication reaching its first reader")
	gate.openWhenIssued(t, r)
	var teardownErr error
	torn := startOrderPath(func() {
		teardownErr = r.teardownSessionOnTunnelWith(11, 7, cdnResultAdministrative, l2tpevents.TerminateCauseAdminReset)
	})
	awaitOrderEvent(t, torn, "the operator teardown")
	awaitOrderEvent(t, assigned, "the SessionIPAssigned publication")

	require.NoError(t, teardownErr)
	require.Empty(t, gate.overtakers(),
		"a later notification of tunnel 11 reached the readers while its SessionIPAssigned was still being delivered")
	require.Equal(t, []string{l2tpevents.SessionIPAssignedEvent, l2tpevents.SessionDownEvent}, notices.of(11, 7),
		"readers MUST receive the notifications of one session in the order its state changed")
}

// TestSessionUpIsPublishedBeforeTheTeardownThatFollowsIt is the same question
// for SessionUp, which the reactor goroutine publishes when every NCP is up.
func TestSessionUpIsPublishedBeforeTheTeardownThatFollowsIt(t *testing.T) {
	bus := newSessionOrderBus()
	gate := newSessionOrderGate(t)
	l2tpevents.SessionUp.Subscribe(bus, func(p *l2tpevents.SessionUpPayload) { gate.hold(p.TunnelID) })
	l2tpevents.SessionDown.Subscribe(bus, func(p *l2tpevents.SessionDownPayload) {
		gate.observe(l2tpevents.SessionDownEvent, p.TunnelID)
	})
	notices := subscribeSessionNoticeLog(bus)
	r := newReactorForEventOrder(t, bus)
	insertEstablishedTunnel(t, r, 11, 22, orderTestPeer, orderTestSession(7, "ppp7"))

	up := startOrderPath(func() { r.handleSessionUp(ppp.EventSessionUp{TunnelID: 11, SessionID: 7}) })
	awaitOrderEvent(t, gate.entered, "the SessionUp publication reaching its first reader")
	gate.openWhenIssued(t, r)
	var teardownErr error
	torn := startOrderPath(func() {
		teardownErr = r.teardownSessionOnTunnelWith(11, 7, cdnResultAdministrative, l2tpevents.TerminateCauseAdminReset)
	})
	awaitOrderEvent(t, torn, "the operator teardown")
	awaitOrderEvent(t, up, "the SessionUp publication")

	require.NoError(t, teardownErr)
	require.Empty(t, gate.overtakers(),
		"a later notification of tunnel 11 reached the readers while its SessionUp was still being delivered")
	require.Equal(t, []string{l2tpevents.SessionUpEvent, l2tpevents.SessionDownEvent}, notices.of(11, 7),
		"readers MUST receive the notifications of one session in the order its state changed")
}

// TestRouteObserverSeesIPUpBeforeTheDownThatFollowsIt proves the route
// observer is ordered too: an IP-up told after the withdrawal would leave a
// record, and an injected subscriber route, that nothing removes. The real
// observer sits behind a wrapper that holds the IP-up.
func TestRouteObserverSeesIPUpBeforeTheDownThatFollowsIt(t *testing.T) {
	bus := newSessionOrderBus()
	gate := newSessionOrderGate(t)
	inner := newSubscriberRouteObserver(slog.Default(), bus)
	r := newReactorForEventOrder(t, bus)
	r.routeObserver = &gatedRouteObserver{gate: gate, inner: inner}
	insertEstablishedTunnel(t, r, 11, 22, orderTestPeer, orderTestSession(7, "ppp7"))

	assigned := startOrderPath(func() { r.handleSessionIPAssigned(orderTestIPv4()) })
	awaitOrderEvent(t, gate.entered, "the IP-up reaching the route observer")
	gate.openWhenIssued(t, r)
	var teardownErr error
	torn := startOrderPath(func() {
		teardownErr = r.teardownSessionOnTunnelWith(11, 7, cdnResultAdministrative, l2tpevents.TerminateCauseAdminReset)
	})
	awaitOrderEvent(t, torn, "the operator teardown")
	awaitOrderEvent(t, assigned, "the IP-up")

	require.NoError(t, teardownErr)
	inner.mu.Lock()
	_, kept := inner.records[7]
	inner.mu.Unlock()
	require.False(t, kept,
		"the route observer still holds session 7 after its teardown: the withdrawal reached it before the IP-up")
}

// TestTeardownDuringIPAssignedLeavesNoSubscriberEntry proves AC-8 at the
// subscriber bridge: a session cleared while its IP-assigned event is being
// handled leaves no entry in the subscriber registry.
func TestTeardownDuringIPAssignedLeavesNoSubscriberEntry(t *testing.T) {
	bus := newSessionOrderBus()
	gate := newSessionOrderGate(t)
	l2tpevents.SessionIPAssigned.Subscribe(bus, func(p *l2tpevents.SessionIPAssignedPayload) { gate.hold(p.TunnelID) })
	l2tpevents.SessionDown.Subscribe(bus, func(p *l2tpevents.SessionDownPayload) {
		gate.observe(l2tpevents.SessionDownEvent, p.TunnelID)
	})
	registry := subscriber.NewRegistry()
	bridge := newSubscriberBridge(registry, bus, slog.Default())
	t.Cleanup(bridge.stop)
	r := newReactorForEventOrder(t, bus)
	insertEstablishedTunnel(t, r, 11, 22, orderTestPeer, orderTestSession(7, "ppp7"))

	assigned := startOrderPath(func() { r.handleSessionIPAssigned(orderTestIPv4()) })
	awaitOrderEvent(t, gate.entered, "the SessionIPAssigned publication reaching its first reader")
	gate.openWhenIssued(t, r)
	var teardownErr error
	torn := startOrderPath(func() {
		teardownErr = r.teardownSessionOnTunnelWith(11, 7, cdnResultAdministrative, l2tpevents.TerminateCauseAdminReset)
	})
	awaitOrderEvent(t, torn, "the operator teardown")
	awaitOrderEvent(t, assigned, "the SessionIPAssigned publication")

	require.NoError(t, teardownErr)
	_, registered := registry.Get(l2tpSessionID(11, 7))
	require.False(t, registered,
		"the subscriber registry still holds session 7 after its teardown: the SessionDown reached the bridge before the SessionIPAssigned")
}

// TestTunnelTeardownDownsFollowEarlierEventsOfItsSessions proves the order for
// a tunnel teardown, which ends every session of the tunnel in one path: the
// SessionDown of a session whose IP-assigned event is still being delivered
// waits for it.
func TestTunnelTeardownDownsFollowEarlierEventsOfItsSessions(t *testing.T) {
	bus := newSessionOrderBus()
	gate := newSessionOrderGate(t)
	l2tpevents.SessionIPAssigned.Subscribe(bus, func(p *l2tpevents.SessionIPAssignedPayload) { gate.hold(p.TunnelID) })
	l2tpevents.SessionDown.Subscribe(bus, func(p *l2tpevents.SessionDownPayload) {
		gate.observe(l2tpevents.SessionDownEvent, p.TunnelID)
	})
	notices := subscribeSessionNoticeLog(bus)
	r := newReactorForEventOrder(t, bus)
	insertEstablishedTunnel(t, r, 11, 22, orderTestPeer, orderTestSession(7, "ppp7"), orderTestSession(8, "ppp8"))

	assigned := startOrderPath(func() { r.handleSessionIPAssigned(orderTestIPv4()) })
	awaitOrderEvent(t, gate.entered, "the SessionIPAssigned publication reaching its first reader")
	gate.openWhenIssued(t, r)
	var teardownErr error
	torn := startOrderPath(func() {
		teardownErr = r.teardownTunnel(11, ResultCodeValue{Result: resultAdministrative}, l2tpevents.TerminateCauseAdminReset)
	})
	awaitOrderEvent(t, torn, "the tunnel teardown")
	awaitOrderEvent(t, assigned, "the SessionIPAssigned publication")

	require.NoError(t, teardownErr)
	require.Empty(t, gate.overtakers(),
		"a SessionDown of tunnel 11 reached the readers while the SessionIPAssigned of one of its sessions was still being delivered")
	require.Equal(t, []string{l2tpevents.SessionIPAssignedEvent, l2tpevents.SessionDownEvent}, notices.of(11, 7),
		"readers MUST receive the notifications of one session in the order its state changed")
}

// TestIPAssignedAcknowledgesAfterReadersReturn proves AC-10: PPP, which
// enables forwarding on the acknowledgement, is acknowledged after the last
// reader of the IP-assigned event returned, and not before. Each reader looks
// at the acknowledgement channel as the last thing it does.
func TestIPAssignedAcknowledgesAfterReadersReturn(t *testing.T) {
	bus := newSessionOrderBus()
	ev, acknowledged := acknowledgeableIPAssigned(t, orderTestIPv4())
	var acknowledgedEarly []string
	lookAtAcknowledgement := func(reader string) {
		select {
		case <-acknowledged:
			acknowledgedEarly = append(acknowledgedEarly, reader)
		default:
		}
	}
	l2tpevents.SessionIPAssigned.Subscribe(bus, func(*l2tpevents.SessionIPAssignedPayload) { lookAtAcknowledgement("first reader") })
	l2tpevents.SessionIPAssigned.Subscribe(bus, func(*l2tpevents.SessionIPAssignedPayload) { lookAtAcknowledgement("last reader") })
	r := newReactorForEventOrder(t, bus)
	insertEstablishedTunnel(t, r, 11, 22, orderTestPeer, orderTestSession(7, "ppp7"))

	r.handleSessionIPAssigned(ev)

	require.Empty(t, acknowledgedEarly, "PPP was acknowledged while a reader of the assignment still ran")
	select {
	case <-acknowledged:
	default:
		t.Fatal("handleSessionIPAssigned returned without acknowledging the assignment")
	}
}

// TestSessionUpStartsTimeoutsAfterPublication validates A-5: the RADIUS
// session timeout is armed after the SessionUp publication, never during it.
// A reader looks at the session's timer while it handles the event.
func TestSessionUpStartsTimeoutsAfterPublication(t *testing.T) {
	bus := newSessionOrderBus()
	r := newReactorForEventOrder(t, bus)
	sess := orderTestSession(7, "ppp7")
	insertEstablishedTunnel(t, r, 11, 22, orderTestPeer, sess)
	StoreSessionMetadata(11, 7, &AuthMetadata{SessionTimeout: 3600})
	t.Cleanup(func() { ClearSessionMetadata(11, 7) })
	t.Cleanup(func() {
		r.tunnelsMu.Lock()
		cancelSessionTimeouts(sess)
		r.tunnelsMu.Unlock()
	})
	armedDuringPublication := false
	l2tpevents.SessionUp.Subscribe(bus, func(*l2tpevents.SessionUpPayload) {
		r.tunnelsMu.Lock()
		armedDuringPublication = sess.sessionTimeoutCancel != nil
		r.tunnelsMu.Unlock()
	})

	r.handleSessionUp(ppp.EventSessionUp{TunnelID: 11, SessionID: 7})

	r.tunnelsMu.Lock()
	armed := sess.sessionTimeoutCancel != nil
	r.tunnelsMu.Unlock()
	require.False(t, armedDuringPublication, "the session timeout was armed before the SessionUp readers ran")
	require.True(t, armed, "handleSessionUp returned without arming the RADIUS session timeout")
}

// TestSlowReaderDelaysNoStateChange proves AC-18: while a reader of one
// tunnel's event is held, a teardown on another tunnel completes and its
// SessionDown reaches the readers, and a session lookup answers.
func TestSlowReaderDelaysNoStateChange(t *testing.T) {
	bus := newSessionOrderBus()
	gate := newSessionOrderGate(t)
	l2tpevents.SessionIPAssigned.Subscribe(bus, func(p *l2tpevents.SessionIPAssignedPayload) { gate.hold(p.TunnelID) })
	notices := subscribeSessionNoticeLog(bus)
	r := newReactorForEventOrder(t, bus)
	insertEstablishedTunnel(t, r, 11, 22, orderTestPeer, orderTestSession(7, "ppp7"))
	insertEstablishedTunnel(t, r, 12, 23, orderTestOtherPeer, orderTestSession(9, "ppp9"))

	assigned := startOrderPath(func() { r.handleSessionIPAssigned(orderTestIPv4()) })
	awaitOrderEvent(t, gate.entered, "the SessionIPAssigned publication of tunnel 11 reaching its first reader")
	var teardownErr error
	torn := startOrderPath(func() {
		teardownErr = r.teardownSessionOnTunnelWith(12, 9, cdnResultAdministrative, l2tpevents.TerminateCauseAdminReset)
	})
	awaitOrderEvent(t, torn, "the teardown on tunnel 12 while a reader of tunnel 11 is held")
	var found bool
	looked := startOrderPath(func() { _, found = r.LookupSession(7) })
	awaitOrderEvent(t, looked, "a session lookup while a reader of tunnel 11 is held")
	gate.open()
	awaitOrderEvent(t, assigned, "the held publication")

	require.NoError(t, teardownErr)
	require.True(t, found, "the lookup did not find session 7, which is still up")
	require.Equal(t, []string{l2tpevents.SessionDownEvent}, notices.of(12, 9),
		"the SessionDown of tunnel 12 did not reach the readers while a reader of tunnel 11 was held")
}

// reactorTripwire is an L2TP Service whose every method records that it was
// called. Published as the running service, it is the reactor a reader would
// reach through LookupService. Safe for concurrent use.
type reactorTripwire struct {
	mu     sync.Mutex
	called []string
}

var _ Service = (*reactorTripwire)(nil)

func (w *reactorTripwire) trip(method string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.called = append(w.called, method)
}

func (w *reactorTripwire) calls() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Clone(w.called)
}

func (w *reactorTripwire) Snapshot() Snapshot { w.trip("Snapshot"); return Snapshot{} }
func (w *reactorTripwire) LookupTunnel(uint16) (TunnelSnapshot, bool) {
	w.trip("LookupTunnel")
	return TunnelSnapshot{}, false
}
func (w *reactorTripwire) LookupSession(uint16) (SessionSnapshot, bool) {
	w.trip("LookupSession")
	return SessionSnapshot{}, false
}
func (w *reactorTripwire) Listeners() []ListenerSnapshot { w.trip("Listeners"); return nil }
func (w *reactorTripwire) EffectiveConfig() ConfigSnapshot {
	w.trip("EffectiveConfig")
	return ConfigSnapshot{}
}
func (w *reactorTripwire) TeardownTunnel(uint16) error  { w.trip("TeardownTunnel"); return nil }
func (w *reactorTripwire) TeardownSession(uint16) error { w.trip("TeardownSession"); return nil }
func (w *reactorTripwire) TeardownAllTunnels() int      { w.trip("TeardownAllTunnels"); return 0 }
func (w *reactorTripwire) TeardownAllSessions() int     { w.trip("TeardownAllSessions"); return 0 }
func (w *reactorTripwire) SessionEvents(uint16) []ObserverEvent {
	w.trip("SessionEvents")
	return nil
}
func (w *reactorTripwire) LoginSamples(string) []CQMBucket    { w.trip("LoginSamples"); return nil }
func (w *reactorTripwire) SessionSummaries() []SessionSummary { w.trip("SessionSummaries"); return nil }
func (w *reactorTripwire) LoginSummaries() []LoginSummary     { w.trip("LoginSummaries"); return nil }
func (w *reactorTripwire) EchoState(string) *LoginEchoState   { w.trip("EchoState"); return nil }
func (w *reactorTripwire) ReliableStats(uint16) *ReliableStats {
	w.trip("ReliableStats")
	return nil
}
func (w *reactorTripwire) TunnelFSMHistory(uint16) []FSMTransition {
	w.trip("TunnelFSMHistory")
	return nil
}
func (w *reactorTripwire) SessionFSMHistory(uint16) []FSMTransition {
	w.trip("SessionFSMHistory")
	return nil
}
func (w *reactorTripwire) CaptureSnapshot(int, uint16, string) []CaptureEntry {
	w.trip("CaptureSnapshot")
	return nil
}
func (w *reactorTripwire) EnableRawCapture()  { w.trip("EnableRawCapture") }
func (w *reactorTripwire) DisableRawCapture() { w.trip("DisableRawCapture") }
func (w *reactorTripwire) RawCaptureSnapshot(int) []RawCaptureEntry {
	w.trip("RawCaptureSnapshot")
	return nil
}
func (w *reactorTripwire) RecordDisconnect(uint16, string, string, uint32) {
	w.trip("RecordDisconnect")
}
func (w *reactorTripwire) PlaceOutgoingCall(string, string) (OutgoingCallResult, error) {
	w.trip("PlaceOutgoingCall")
	return OutgoingCallResult{}, nil
}

// sessionNestingBus is a sessionOrderBus that records each L2TP session event
// published while another one is still being delivered. Such a publication
// comes from inside a reader. Safe for concurrent use.
type sessionNestingBus struct {
	*sessionOrderBus

	mu         sync.Mutex
	delivering int
	nested     []string
}

func (b *sessionNestingBus) Emit(namespace, eventType string, payload any) (int, error) {
	if namespace != l2tpevents.Namespace || !isL2TPSessionEvent(eventType) {
		return b.sessionOrderBus.Emit(namespace, eventType, payload)
	}
	b.mu.Lock()
	if b.delivering > 0 {
		b.nested = append(b.nested, eventType)
	}
	b.delivering++
	b.mu.Unlock()
	defer b.leave()
	return b.sessionOrderBus.Emit(namespace, eventType, payload)
}

func (b *sessionNestingBus) leave() {
	b.mu.Lock()
	b.delivering--
	b.mu.Unlock()
}

func (b *sessionNestingBus) nestedPublications() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.nested)
}

func isL2TPSessionEvent(eventType string) bool {
	return slices.Contains([]string{
		l2tpevents.SessionIPAssignedEvent, l2tpevents.SessionUpEvent, l2tpevents.SessionDownEvent,
	}, eventType)
}

// TestSessionEventReadersDoNotCallBackIntoTheReactor validates A-3 for this
// package's readers: the subscriber bridge and the CQM observer handle each
// of the three session events without reaching the reactor and without
// publishing an L2TP session event from inside their handler. Either would
// wait behind the order ticket its own caller holds. The reactor a reader can
// reach is the published Service, here a tripwire.
//
// The readers in other packages (accounting, pool, shaper, CoS, CoA health)
// import this package, so this test binary cannot link them: their handlers
// were read instead.
func TestSessionEventReadersDoNotCallBackIntoTheReactor(t *testing.T) {
	bus := &sessionNestingBus{sessionOrderBus: newSessionOrderBus()}
	tripwire := &reactorTripwire{}
	previous := LookupService()
	PublishService(tripwire)
	t.Cleanup(func() { PublishService(previous) })
	stop := StartSessionEventReadersForTest(bus, subscriber.NewRegistry())
	t.Cleanup(stop)

	_, err := l2tpevents.SessionIPAssigned.Emit(bus, &l2tpevents.SessionIPAssignedPayload{
		TunnelID: orderTestTID, SessionID: orderTestSID, Username: "grace",
		PeerAddr: "192.0.2.7", PppInterface: "ppp7",
	})
	require.NoError(t, err)
	_, err = l2tpevents.SessionUp.Emit(bus, &l2tpevents.SessionUpPayload{
		TunnelID: orderTestTID, SessionID: orderTestSID, Interface: "ppp7",
	})
	require.NoError(t, err)
	_, err = l2tpevents.SessionDown.Emit(bus, &l2tpevents.SessionDownPayload{
		TunnelID: orderTestTID, SessionID: orderTestSID, Username: "grace",
		Cause: l2tpevents.TerminateCauseAdminReset,
	})
	require.NoError(t, err)

	require.Empty(t, tripwire.calls(), "a reader of an L2TP session event called into the reactor from its handler")
	require.Empty(t, bus.nestedPublications(), "a reader of an L2TP session event published an L2TP session event from its handler")
}

// ticketReaderPanic is the value the panicking reader of the ticket tests
// panics with, so the driver recovers that panic and no other.
const ticketReaderPanic = "session event reader panicked"

// ticketPath is one of the twelve paths that publish a session notification:
// drive runs it on a reactor holding tunnel orderTestTID with its session
// orderTestSID established, and the tunnel's order is the one the path takes
// its ticket from.
type ticketPath struct {
	name  string
	drive func(t *testing.T, r *l2tpReactor, tun *L2TPTunnel) error
}

// ticketPaths lists the twelve paths: the nine that deliver drained
// session-downs, then handleLCPDown, handleSessionIPAssigned and
// handleSessionUp.
func ticketPaths() []ticketPath {
	stopTB := []byte{5, 5, 5, 5, 5, 5, 5, 5}
	return []ticketPath{
		{name: "an SCCRQ whose equal tie breaker discards both tunnels", drive: func(t *testing.T, r *l2tpReactor, tun *L2TPTunnel) error {
			setTieBreaker(r, tun, stopTB)
			r.handle(rxPacket{from: orderTestPeer, bytes: buildSCCRQWithTieBreaker(t, 99, "peer", stopTB), release: func() {}})
			return nil
		}},
		{name: "an SCCRQ whose lower tie breaker discards the tunnel", drive: func(t *testing.T, r *l2tpReactor, tun *L2TPTunnel) error {
			setTieBreaker(r, tun, []byte{9, 9, 9, 9, 9, 9, 9, 9})
			r.handle(rxPacket{from: orderTestPeer, bytes: buildSCCRQWithTieBreaker(t, 99, "peer", []byte{1, 1, 1, 1, 1, 1, 1, 1}), release: func() {}})
			return nil
		}},
		{name: "a tick for another tunnel that reaps the tunnel", drive: func(_ *testing.T, r *l2tpReactor, tun *L2TPTunnel) error {
			r.tunnelsMu.Lock()
			tun.state = L2TPTunnelClosed
			tun.engine.closed = true
			tun.engine.closedAt = time.Unix(0, 0).UTC()
			tun.engine.retention = time.Second
			r.tunnelsMu.Unlock()
			r.handleTick(tickReq{tunnelID: orderTestTID + 1})
			return nil
		}},
		{name: "a tick that finds the peer dead", drive: func(_ *testing.T, r *l2tpReactor, tun *L2TPTunnel) error {
			r.params.HelloRetries = 2
			r.params.HelloInterval = 5 * time.Second
			r.tunnelsMu.Lock()
			tun.lastLiveness = time.Unix(0, 0).UTC()
			r.tunnelsMu.Unlock()
			r.handleTick(tickReq{tunnelID: orderTestTID})
			return nil
		}},
		{name: "a PPP session down", drive: func(_ *testing.T, r *l2tpReactor, _ *L2TPTunnel) error {
			r.handlePPPEvent(ppp.EventSessionDown{
				TunnelID: orderTestTID, SessionID: orderTestSID, Reason: "test", Cause: l2tpevents.TerminateCauseUserRequest,
			})
			return nil
		}},
		{name: "a kernel setup failure", drive: func(_ *testing.T, r *l2tpReactor, _ *L2TPTunnel) error {
			r.handleKernelError(kernelSetupFailed{localTID: orderTestTID, localSID: orderTestSID})
			return nil
		}},
		{name: "an operator tunnel teardown", drive: func(_ *testing.T, r *l2tpReactor, _ *L2TPTunnel) error {
			return r.teardownTunnel(orderTestTID, ResultCodeValue{Result: resultAdministrative}, l2tpevents.TerminateCauseAdminReset)
		}},
		{name: "a session teardown by session ID", drive: func(_ *testing.T, r *l2tpReactor, _ *L2TPTunnel) error {
			return r.teardownSessionByID(orderTestSID, l2tpevents.TerminateCauseAdminReset)
		}},
		{name: "a session teardown on a named tunnel", drive: func(_ *testing.T, r *l2tpReactor, _ *L2TPTunnel) error {
			return r.teardownSessionOnTunnelWith(orderTestTID, orderTestSID, cdnResultAdministrative, l2tpevents.TerminateCauseAdminReset)
		}},
		{name: "an LCP down in the network phase", drive: func(_ *testing.T, r *l2tpReactor, _ *L2TPTunnel) error {
			r.handleLCPDown(ppp.EventLCPDown{TunnelID: orderTestTID, SessionID: orderTestSID, NetworkPhase: true})
			return nil
		}},
		{name: "an IP assigned", drive: func(_ *testing.T, r *l2tpReactor, _ *L2TPTunnel) error {
			r.handleSessionIPAssigned(orderTestIPv4())
			return nil
		}},
		{name: "a session up", drive: func(_ *testing.T, r *l2tpReactor, _ *L2TPTunnel) error {
			r.handleSessionUp(ppp.EventSessionUp{TunnelID: orderTestTID, SessionID: orderTestSID})
			return nil
		}},
	}
}

func setTieBreaker(r *l2tpReactor, tun *L2TPTunnel, tb []byte) {
	r.tunnelsMu.Lock()
	tun.tieBreaker = tb
	r.tunnelsMu.Unlock()
}

// newReactorForTicketPaths builds an unstarted reactor with the channels the
// packet and tick paths send on, a route observer and the bus, holding tunnel
// orderTestTID with session orderTestSID established. Its listener is never
// started, so every send fails and is logged, which no test here reads.
func newReactorForTicketPaths(t *testing.T, bus ze.EventBus) (*l2tpReactor, *L2TPTunnel) {
	t.Helper()
	ln := newUDPListener(netip.MustParseAddrPort("127.0.0.1:0"), slog.Default())
	r := newL2TPReactor(ln, slog.Default(), reactorParams{Clock: func() time.Time { return time.Unix(1000, 0).UTC() }})
	r.eventBus = bus
	r.routeObserver = newSubscriberRouteObserver(slog.Default(), bus)
	insertEstablishedTunnel(t, r, orderTestTID, 22, orderTestPeer, orderTestSession(orderTestSID, "ppp7"))
	r.tunnelsMu.Lock()
	tun := r.tunnelsByLocalID[orderTestTID]
	r.tunnelsMu.Unlock()
	return r, tun
}

// ticketPathReader reads the three session events and panics on each when
// panicking is set, counting the panics. Safe for concurrent use.
type ticketPathReader struct {
	panicking bool
	mu        sync.Mutex
	panics    int
}

func subscribeTicketPathReader(bus ze.EventBus, panicking bool) *ticketPathReader {
	reader := &ticketPathReader{panicking: panicking}
	l2tpevents.SessionIPAssigned.Subscribe(bus, func(*l2tpevents.SessionIPAssignedPayload) { reader.read() })
	l2tpevents.SessionUp.Subscribe(bus, func(*l2tpevents.SessionUpPayload) { reader.read() })
	l2tpevents.SessionDown.Subscribe(bus, func(*l2tpevents.SessionDownPayload) { reader.read() })
	return reader
}

func (p *ticketPathReader) read() {
	if !p.panicking {
		return
	}
	p.mu.Lock()
	p.panics++
	p.mu.Unlock()
	panic(ticketReaderPanic)
}

func (p *ticketPathReader) panicCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.panics
}

// driveTicketPath runs path, and recovers the panicking reader's panic when
// the path does not contain it itself. Any other panic goes on.
func driveTicketPath(t *testing.T, path ticketPath, r *l2tpReactor, tun *L2TPTunnel) (err error) {
	t.Helper()
	defer func() {
		v := recover()
		if v == nil {
			return
		}
		if v != ticketReaderPanic {
			panic(v)
		}
	}()
	return path.drive(t, r, tun)
}

// TestEveryIssuedTicketIsFinished validates R-3: after each of the twelve
// publishing paths, the tunnel's order has finished every ticket it issued,
// so its later notifications are not stopped. Each path runs once with quiet
// readers and once with a reader that panics on every session event, which
// the deferred finish in sessionNoticeTicket.deliver must survive. The path
// must have issued a ticket, or the equality would hold for a path that
// delivers nothing.
func TestEveryIssuedTicketIsFinished(t *testing.T) {
	for _, path := range ticketPaths() {
		for _, panicking := range []bool{false, true} {
			name := path.name
			if panicking {
				name += ", with a reader that panics"
			}
			t.Run(name, func(t *testing.T) {
				bus := newSessionOrderBus()
				reader := subscribeTicketPathReader(bus, panicking)
				r, tun := newReactorForTicketPaths(t, bus)

				err := driveTicketPath(t, path, r, tun)

				if panicking {
					require.Positive(t, reader.panicCount(), "the reader never ran, so the path did not deliver to it")
				} else {
					require.NoError(t, err)
				}
				issued, finished := ticketCounts(r, tun)
				require.Positive(t, issued, "the path took no order ticket for what it delivered")
				require.Equal(t, issued, finished,
					"the tunnel's order finished %d of the %d tickets it issued: its later notifications would wait for ever", finished, issued)
			})
		}
	}
}
