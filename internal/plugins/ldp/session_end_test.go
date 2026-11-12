// Design: docs/architecture/ldp/mpls-ldp.md -- a failed session setup backs off on the Hello cadence
// Related: rfc5443_sessionup_test.go -- exchangeInitialization, encodeKeepAlivePDU
// Related: rfc5036_test.go -- readLDPPDU, encodeInitPDU, ldpReadTimeout
// Related: adjacency_expiry_test.go -- TestAdjacencyExpiryLeavesNoBackoffEntry
//
// What a session end leaves behind. These tests drive startSessionForAdj, the
// function discovery calls on every Hello, over an in-memory connection the
// dialSession seam hands it, and play the neighbor on the far end. They read the
// setup backoff table and record the session events the plugin publishes, so they
// judge the end-of-session code where it runs, in the closure startSessionForAdj
// hands runSession.
//
// VALIDATES: a session that reached operational clears the backoff and publishes
// one SessionDown; a setup attempt that ended before operational records a setup
// failure and publishes no SessionDown, also beside an operational neighbor on the
// same interface.
// PREVENTS: a session that ran for days being redialed only after 15 seconds, and
// an OSPF LDP-sync link costed out for a setup attempt that never was a session.
package ldp

import (
	"context"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/ze"
)

// sessionEndQuiet is how long a test waits to prove that no SessionDown follows.
// The end-of-session closure publishes within microseconds of removing the session
// from the table, so an event that is due arrives well inside it.
const sessionEndQuiet = 300 * time.Millisecond

// sessionEventRecorder is an event bus that keeps every LDP SessionUp and
// SessionDown the plugin emits. Safe for concurrent use: each session publishes
// from its own goroutine.
type sessionEventRecorder struct {
	mu    sync.Mutex
	ups   []SessionEvent
	downs []SessionEvent
	// upArrived and downArrived carry one token per recorded event, so a test
	// waits on them instead of polling.
	upArrived   chan struct{}
	downArrived chan struct{}
}

func (r *sessionEventRecorder) Emit(namespace, eventType string, payload any) (int, error) {
	if namespace != Namespace {
		return 0, nil
	}
	evt, ok := payload.(*SessionEvent)
	if !ok {
		return 0, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	switch eventType {
	case EventSessionUp:
		r.ups = append(r.ups, *evt)
		signalArrival(r.upArrived)
	case EventSessionDown:
		r.downs = append(r.downs, *evt)
		signalArrival(r.downArrived)
	}
	return 0, nil
}

func (r *sessionEventRecorder) Subscribe(string, string, func(any)) func() { return func() {} }

// recorded returns a copy of the SessionUps and SessionDowns recorded so far.
func (r *sessionEventRecorder) recorded() (ups, downs []SessionEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]SessionEvent(nil), r.ups...), append([]SessionEvent(nil), r.downs...)
}

var _ ze.EventBus = (*sessionEventRecorder)(nil)

// signalArrival puts one token on arrived, dropping it when the buffer is full:
// a waiter needs one token, not a count.
func signalArrival(arrived chan struct{}) {
	select {
	case arrived <- struct{}{}:
	default:
	}
}

// sessionEndHarness holds the session table and backoff table startSessionForAdj
// shares with the engine, and the neighbor end of every connection it dialed.
type sessionEndHarness struct {
	t        *testing.T
	ctx      context.Context
	lib      *LIB
	fib      *ldpFIB
	bus      *sessionEventRecorder
	mu       sync.Mutex // the engine's sessionsMu: guards sessions and retries
	sessions map[string]*Session
	retries  map[string]*setupRetry
	// dialed receives the neighbor end of each connection dialSession opened.
	dialed  chan net.Conn
	remotes []net.Conn
	started []string // the adjacency keys of the sessions this harness opened
}

// newSessionEndHarness installs the recording bus and the in-memory dialer for
// the test. At the end it closes every neighbor end, waits for each session's
// end-of-session work, and restores the previous bus and dialer, so no session
// of this test publishes into the next test's recorder.
func newSessionEndHarness(t *testing.T) *sessionEndHarness {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	h := &sessionEndHarness{
		t:        t,
		ctx:      ctx,
		lib:      newLIB(),
		fib:      newLDPFIB(nil, slogutil.DiscardLogger()),
		bus:      &sessionEventRecorder{upArrived: make(chan struct{}, 8), downArrived: make(chan struct{}, 8)},
		sessions: map[string]*Session{},
		retries:  map[string]*setupRetry{},
		dialed:   make(chan net.Conn, 8),
	}

	prevBus := eventBusPtr.Load()
	prevDial := dialSession
	setEventBus(h.bus)
	dialSession = func(context.Context, netip.Addr, string) (net.Conn, error) {
		local, remote := net.Pipe()
		h.dialed <- remote
		return local, nil
	}
	t.Cleanup(func() {
		dialSession = prevDial
		eventBusPtr.Store(prevBus)
		cancel()
	})
	t.Cleanup(h.drain)
	return h
}

// start runs startSessionForAdj for adj, as discovery does on a Hello, and
// returns the neighbor end of the connection it dialed, or nil when it dialed
// nothing.
func (h *sessionEndHarness) start(adj *Adjacency) net.Conn {
	h.t.Helper()
	startSessionForAdj(h.ctx, slogutil.DiscardLogger(), adj, [4]byte{10, 0, 0, 1}, netip.Addr{},
		DefaultKeepaliveTime, DefaultHopCountMax, h.lib, h.sessions, h.retries, &h.mu, h.fib)
	select {
	case remote := <-h.dialed:
		h.remotes = append(h.remotes, remote)
		h.started = append(h.started, AdjacencyKey(adj.PeerLSRID, adj.PeerLabelSpace))
		return remote
	default: // startSessionForAdj dials synchronously: no connection means no attempt
		return nil
	}
}

// mustStart is start for a test whose attempt the backoff must allow.
func (h *sessionEndHarness) mustStart(adj *Adjacency) net.Conn {
	h.t.Helper()
	remote := h.start(adj)
	if remote == nil {
		h.t.Fatalf("startSessionForAdj dialed nothing for %s: the setup backoff refused the attempt", adj.TransportAddr)
	}
	return remote
}

// driveToOperational plays the neighbor through the Initialization exchange and
// its KeepAlive, then waits for the SessionUp the session publishes.
func (h *sessionEndHarness) driveToOperational(remote net.Conn) {
	h.t.Helper()
	exchangeInitialization(h.t, remote)
	if _, err := remote.Write(encodeKeepAlivePDU()); err != nil {
		h.t.Fatalf("write neighbor KeepAlive: %v", err)
	}
	select {
	case <-h.bus.upArrived:
	case <-time.After(ldpReadTimeout):
		h.t.Fatal("no SessionUp within 2s of the neighbor's KeepAlive")
	}
}

// readInitialization reads ze's Initialization on remote: the session is then in
// OPENSENT and has not reached operational.
func (h *sessionEndHarness) readInitialization(remote net.Conn) {
	h.t.Helper()
	if _, hdr, _ := readLDPPDU(h.t, remote); hdr.Type != MsgTypeInitialize {
		h.t.Fatalf("first message = %#x, want Initialization (%#x)", hdr.Type, MsgTypeInitialize)
	}
}

// awaitSessionEnded waits until the end-of-session closure removed key from the
// session table. The backoff table is updated under the same lock, so it is
// settled when this returns; the session events follow after the unlock.
func (h *sessionEndHarness) awaitSessionEnded(key string) {
	h.t.Helper()
	deadline := time.Now().Add(ldpReadTimeout)
	for h.holdsSession(key) {
		if time.Now().After(deadline) {
			h.t.Fatalf("session %s still in the session table 2s after its connection closed", key)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (h *sessionEndHarness) holdsSession(key string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, ok := h.sessions[key]
	return ok
}

// backoff returns the backoff entry for key, nil when there is none.
func (h *sessionEndHarness) backoff(key string) *setupRetry {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.retries[key]
}

// expectNoSessionDown fails the test when a SessionDown is published within
// sessionEndQuiet, or was published before.
func (h *sessionEndHarness) expectNoSessionDown(what string) {
	h.t.Helper()
	select {
	case <-h.bus.downArrived:
	case <-time.After(sessionEndQuiet):
	}
	if _, downs := h.bus.recorded(); len(downs) != 0 {
		h.t.Errorf("SessionDown published %d times (%+v), want none: %s", len(downs), downs, what)
	}
}

// drain closes every neighbor end and waits until each session this harness
// opened has left the table and every SessionUp has had its SessionDown, so the
// end-of-session work is finished before the bus is restored.
func (h *sessionEndHarness) drain() {
	for _, remote := range h.remotes {
		_ = remote.Close()
	}
	for _, key := range h.started {
		h.awaitSessionEnded(key)
	}
	deadline := time.Now().Add(ldpReadTimeout)
	for {
		ups, downs := h.bus.recorded()
		if len(downs) >= len(ups) {
			return
		}
		if time.Now().After(deadline) {
			h.t.Errorf("%d SessionUps and %d SessionDowns 2s after every connection closed", len(ups), len(downs))
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// secondNeighbor is a second LDP neighbor discovered on the same interface as
// testAdjacency, with its own LSR-ID and transport address.
func secondNeighbor() *Adjacency {
	return &Adjacency{
		PeerLSRID:     [4]byte{10, 0, 0, 3},
		TransportAddr: netip.MustParseAddr("10.0.0.3"),
		Interface:     "eth0",
	}
}

// runSessionReportingEnd starts runSession on sess and returns a channel that
// receives the value runSession hands onDone when the session ends.
func runSessionReportingEnd(t *testing.T, sess *Session) <-chan bool {
	t.Helper()
	ended := make(chan bool, 1)
	go runSession(t.Context(), slogutil.DiscardLogger(), sess, sess.lib, "10.0.0.2:0",
		newLDPFIB(nil, slogutil.DiscardLogger()), sessionUpEvent(testAdjacency(), "10.0.0.2:0"),
		func(operational bool) { ended <- operational })
	t.Cleanup(sess.Stop)
	return ended
}

// awaitEnd waits for the value runSession hands onDone.
func awaitEnd(t *testing.T, ended <-chan bool) bool {
	t.Helper()
	select {
	case operational := <-ended:
		return operational
	case <-time.After(ldpReadTimeout):
		t.Fatal("runSession did not call onDone within 2s of the neighbor closing the connection")
		return false
	}
}

// R-4, A-1: a session driven to operational over an in-memory connection hands
// true to onDone when it ends, although Stop has reset its state to
// non-existent by then. The value is the one the on-operational callback set,
// read when the deferred call runs; the race detector checks that the callback
// and the deferred call share one goroutine.
func TestRunSessionReportsOperationalToOnDone(t *testing.T) {
	rec := recordSessionUps(t)
	local, remote := sessionPipe(t)
	sess := rfcTestSession(local)
	ended := runSessionReportingEnd(t, sess)

	exchangeInitialization(t, remote)
	if _, err := remote.Write(encodeKeepAlivePDU()); err != nil {
		t.Fatalf("write neighbor KeepAlive: %v", err)
	}
	select {
	case <-rec.arrived:
	case <-time.After(ldpReadTimeout):
		t.Fatal("no SessionUp within 2s of the neighbor's KeepAlive")
	}
	_ = remote.Close()

	if !awaitEnd(t, ended) {
		t.Error("onDone got operational = false for a session that published SessionUp, want true")
	}
}

// A session whose neighbor closes the connection during the Initialization
// exchange hands false to onDone.
func TestRunSessionReportsNotOperationalToOnDone(t *testing.T) {
	local, remote := sessionPipe(t)
	sess := rfcTestSession(local)
	ended := runSessionReportingEnd(t, sess)

	if _, hdr, _ := readLDPPDU(t, remote); hdr.Type != MsgTypeInitialize {
		t.Fatalf("first message = %#x, want Initialization (%#x)", hdr.Type, MsgTypeInitialize)
	}
	_ = remote.Close()

	if awaitEnd(t, ended) {
		t.Error("onDone got operational = true for a session that never left OPENSENT, want false")
	}
}

// AC-1: a session that reached operational and then ended leaves no backoff
// entry for its adjacency, even one an earlier failed attempt created, so the
// neighbor's next Hello starts a new session at once.
func TestSessionEndAfterOperationalClearsBackoff(t *testing.T) {
	h := newSessionEndHarness(t)
	adj := testAdjacency()
	key := AdjacencyKey(adj.PeerLSRID, adj.PeerLabelSpace)
	// An earlier attempt failed an hour ago: its delay has elapsed, so this
	// attempt is allowed and the entry is still in the table.
	recordSetupFailure(slogutil.DiscardLogger(), h.retries, key, time.Now().Add(-time.Hour))

	remote := h.mustStart(adj)
	h.driveToOperational(remote)
	_ = remote.Close()
	h.awaitSessionEnded(key)

	if r := h.backoff(key); r != nil {
		t.Errorf("backoff entry after an operational session ended: delay %s, next attempt not before %s; want none",
			r.delay, r.notBefore.Format(time.RFC3339))
	}
}

// AC-2: a session that reached operational and then ended publishes exactly one
// SessionDown, naming the neighbor's address, its LDP Identifier and the
// interface its adjacency was discovered on.
func TestSessionEndAfterOperationalPublishesOneSessionDown(t *testing.T) {
	h := newSessionEndHarness(t)
	adj := testAdjacency()
	key := AdjacencyKey(adj.PeerLSRID, adj.PeerLabelSpace)

	remote := h.mustStart(adj)
	h.driveToOperational(remote)
	_ = remote.Close()
	select {
	case <-h.bus.downArrived:
	case <-time.After(ldpReadTimeout):
		t.Fatal("no SessionDown within 2s of an operational session's connection closing")
	}
	// A second SessionDown, if any, follows at once.
	select {
	case <-h.bus.downArrived:
	case <-time.After(sessionEndQuiet):
	}

	_, downs := h.bus.recorded()
	if len(downs) != 1 {
		t.Fatalf("SessionDown published %d times, want exactly 1: %+v", len(downs), downs)
	}
	want := SessionEvent{
		PeerAddress:   "10.0.0.2",
		LDPIdentifier: key,
		SessionState:  "non-existent",
		Interface:     "eth0",
	}
	if downs[0] != want {
		t.Errorf("SessionDown = %+v, want %+v", downs[0], want)
	}
}

// AC-3: a setup attempt whose transport closed during the Initialization
// exchange, before the session was operational, records a setup failure: the
// adjacency's next attempt waits the first delay, 15 seconds.
func TestSessionEndBeforeOperationalRecordsSetupFailure(t *testing.T) {
	h := newSessionEndHarness(t)
	adj := testAdjacency()
	key := AdjacencyKey(adj.PeerLSRID, adj.PeerLabelSpace)

	remote := h.mustStart(adj)
	h.readInitialization(remote)
	_ = remote.Close()
	h.awaitSessionEnded(key)

	r := h.backoff(key)
	if r == nil {
		t.Fatal("no backoff entry after a setup attempt ended before operational, want one")
	}
	if r.delay != setupRetryDelayMin {
		t.Errorf("backoff delay = %s, want %s for the first failed attempt", r.delay, setupRetryDelayMin)
	}
}

// AC-4: a setup attempt that ended before the session was operational published
// no SessionUp, so it publishes no SessionDown either: a reader such as OSPF
// LDP-IGP synchronization never hears of it.
func TestSessionEndBeforeOperationalPublishesNoSessionDown(t *testing.T) {
	h := newSessionEndHarness(t)
	adj := testAdjacency()
	key := AdjacencyKey(adj.PeerLSRID, adj.PeerLabelSpace)

	remote := h.mustStart(adj)
	h.readInitialization(remote)
	_ = remote.Close()
	h.awaitSessionEnded(key)

	h.expectNoSessionDown("the attempt never reached operational and published no SessionUp")
}

// AC-21: two neighbors on one interface, one session operational, and a setup
// attempt to the other fails. No SessionDown is published, so an LDP-sync machine
// keyed by the interface keeps the state the operational session gave it.
func TestFailedSetupBesideAnOperationalNeighborPublishesNoSessionDown(t *testing.T) {
	h := newSessionEndHarness(t)
	operational := testAdjacency()
	failing := secondNeighbor()
	failingKey := AdjacencyKey(failing.PeerLSRID, failing.PeerLabelSpace)

	h.driveToOperational(h.mustStart(operational))
	remote := h.mustStart(failing)
	h.readInitialization(remote)
	_ = remote.Close()
	h.awaitSessionEnded(failingKey)

	h.expectNoSessionDown("the failed attempt to 10.0.0.3 on eth0 never reached operational, and 10.0.0.2's session is still up")
}
