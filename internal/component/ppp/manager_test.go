package ppp

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

// VALIDATES: Driver Start/Stop lifecycle. Second Start returns
//
//	ErrAlreadyStarted; Stop is idempotent.
func TestDriverStartStop(t *testing.T) {
	ops, _, _ := newFakeOps()
	d := makeTestDriver(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := d.Start(); !errors.Is(err, ErrAlreadyStarted) {
		t.Errorf("second Start err = %v, want ErrAlreadyStarted", err)
	}
	d.Stop()
	d.Stop() // idempotent
}

// VALIDATES: NewDriver panics on missing required dependencies.
//
// The "nil AuthHook" case from 6a was removed when spec-l2tp-6b-auth
// replaced the AuthHook interface with channel-based dispatch; auth is
// no longer a NewDriver-time required dependency.
func TestDriverNewDriverPanics(t *testing.T) {
	cases := []struct {
		name    string
		mutator func(*DriverConfig)
	}{
		{"nil Logger", func(c *DriverConfig) { c.Logger = nil }},
		{"nil Backend", func(c *DriverConfig) { c.Backend = nil }},
		{"nil Ops.setMRU", func(c *DriverConfig) { c.Ops = pppOps{} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("expected panic")
				}
			}()
			ops, _, _ := newFakeOps()
			cfg := DriverConfig{
				Logger:  discardLogger(),
				Backend: &fakeBackend{},
				Ops:     ops,
			}
			tc.mutator(&cfg)
			NewDriver(cfg)
		})
	}
}

// VALIDATES: A StartSession with proxy LCP AVPs short-circuits the
//
//	FSM and emits EventLCPUp + EventSessionUp without any wire I/O.
//
// PREVENTS: regression where proxy LCP either does not fire or sends
//
//	a CR before short-circuiting.
func TestDriverProxyLCPSkipsNegotiation(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 1001)
	defer closeConn(pair.peerEnd)

	backend := &fakeBackend{}
	ops, opsCalls, opsMu := newFakeOps()
	d := makeTestDriver(backend, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	// Build a valid proxy CONFREQ stream: MRU=1500 + Magic=0xCAFEBABE.
	stream := buildOptionStream([]LCPOption{mruOpt(1500), magicOpt(0xCAFEBABE)})

	d.SessionsIn() <- StartSession{
		TunnelID:            1,
		SessionID:           42,
		ChanFD:              1001,
		UnitFD:              999,
		UnitNum:             7,
		LNSMode:             true,
		MaxMRU:              1500,
		ProxyLCPInitialRecv: stream,
		ProxyLCPLastSent:    stream,
		ProxyLCPLastRecv:    stream,
	}

	// Expect EventLCPUp then EventSessionUp on the events channel.
	got := drainTwoEvents(t, d.EventsOut(), time.Second)
	if _, ok := got[0].(EventLCPUp); !ok {
		t.Errorf("event 0 = %T, want EventLCPUp", got[0])
	}
	if _, ok := got[1].(EventSessionUp); !ok {
		t.Errorf("event 1 = %T, want EventSessionUp", got[1])
	}

	// Backend should record SetMTU(ppp7, 1496) and SetAdminUp(ppp7).
	mtuCalls := backend.MTUCalls()
	if len(mtuCalls) != 1 || mtuCalls[0].name != "ppp7" || mtuCalls[0].mtu != 1496 {
		t.Errorf("MTU calls = %+v, want one ppp7=1496", mtuCalls)
	}
	upCalls := backend.UpCalls()
	if len(upCalls) != 1 || upCalls[0] != "ppp7" {
		t.Errorf("Up calls = %v, want [ppp7]", upCalls)
	}

	// pppOps.setMRU should have been called once with the unit fd.
	opsMu.Lock()
	calls := append([]fakeOpsCall(nil), (*opsCalls)...)
	opsMu.Unlock()
	if len(calls) != 1 || calls[0].fd != 999 || calls[0].mru != 1500 {
		t.Errorf("ops calls = %+v, want one {fd=999, mru=1500}", calls)
	}
}

// VALIDATES: SessionByID returns a snapshot of the active session
//
//	state and false for unknown IDs.
func TestDriverSessionByID(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 2001)
	defer closeConn(pair.peerEnd)

	backend := &fakeBackend{}
	ops, _, _ := newFakeOps()
	d := makeTestDriver(backend, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	stream := buildOptionStream([]LCPOption{mruOpt(1492), magicOpt(0xDEADBEEF)})
	d.SessionsIn() <- StartSession{
		TunnelID:            5,
		SessionID:           77,
		ChanFD:              2001,
		UnitFD:              123,
		UnitNum:             3,
		LNSMode:             true,
		ProxyLCPInitialRecv: stream,
		ProxyLCPLastSent:    stream,
		ProxyLCPLastRecv:    stream,
	}
	// Wait for SessionUp so we know the goroutine has settled.
	drainTwoEvents(t, d.EventsOut(), time.Second)

	info, ok := d.SessionByID(5, 77)
	if !ok {
		t.Fatalf("SessionByID returned not found")
	}
	if info.TunnelID != 5 || info.SessionID != 77 {
		t.Errorf("info IDs wrong: %+v", info)
	}
	if info.State != LCPStateOpened {
		t.Errorf("state = %s, want opened", info.State)
	}
	if info.NegotiatedMRU != 1492 {
		t.Errorf("negotiated MRU = %d, want 1492", info.NegotiatedMRU)
	}
	if info.UnitNum != 3 {
		t.Errorf("unit = %d, want 3", info.UnitNum)
	}

	if _, ok := d.SessionByID(99, 99); ok {
		t.Errorf("SessionByID(99,99) returned ok; expected not found")
	}
}

// VALIDATES: StopSession closes the chan fd, waits for the goroutine
//
//	to exit, and removes the entry. Second call returns
//	ErrSessionNotFound.
func TestDriverStopSession(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 3001)
	defer closeConn(pair.peerEnd)

	backend := &fakeBackend{}
	ops, _, _ := newFakeOps()
	d := makeTestDriver(backend, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	stream := buildOptionStream([]LCPOption{mruOpt(1500), magicOpt(0x12345678)})
	d.SessionsIn() <- StartSession{
		TunnelID:            10,
		SessionID:           20,
		ChanFD:              3001,
		UnitFD:              500,
		UnitNum:             1,
		ProxyLCPInitialRecv: stream,
		ProxyLCPLastSent:    stream,
		ProxyLCPLastRecv:    stream,
	}
	drainTwoEvents(t, d.EventsOut(), time.Second)

	if err := d.StopSession(10, 20); err != nil {
		t.Fatalf("StopSession: %v", err)
	}
	if err := d.StopSession(10, 20); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("second StopSession err = %v, want ErrSessionNotFound", err)
	}

	// Drain any down event.
	drainEventsBest(t, d.EventsOut(), 1, 200*time.Millisecond)
}

// VALIDATES: Standard LCP negotiation against a scripted PPP peer
//
//	drives the FSM to Opened, sets MTU, brings interface up, and
//	emits EventLCPUp + EventSessionUp.
//
// PREVENTS: regression where LCP negotiation hangs or never reaches
//
//	the post-Open side effects (setMRU / SetMTU / SetAdminUp).
func TestDriverLCPNegotiationViaPipe(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 4001)
	defer closeConn(pair.peerEnd)

	backend := &fakeBackend{}
	ops, opsCalls, opsMu := newFakeOps()
	d := makeTestDriver(backend, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	peerDone := make(chan struct{})
	go scriptedPeer(t, pair.peerEnd, peerDone)
	// ISSUE 5 fix: wait for the peer goroutine to exit before the
	// test returns so its t.Errorf calls cannot fire after test
	// completion. The driver's Stop (deferred above) closes the
	// driver end of the pipe, which unblocks the peer's Read.
	t.Cleanup(func() { <-peerDone })

	d.SessionsIn() <- StartSession{
		TunnelID:  100,
		SessionID: 200,
		ChanFD:    4001,
		UnitFD:    777,
		UnitNum:   2,
		LNSMode:   true,
		MaxMRU:    1500,
	}

	// The negotiation completes when both sides exchange CR + CA.
	// Expect EventLCPUp then EventSessionUp.
	got := drainTwoEvents(t, d.EventsOut(), 2*time.Second)
	if _, ok := got[0].(EventLCPUp); !ok {
		t.Errorf("event 0 = %T, want EventLCPUp", got[0])
	}
	// Note: in 6a the LCP-Opened path emits EventLCPUp via
	// openedFromProxy only; the standard path emits EventSessionUp
	// directly after afterLCPOpen. The first event we'll see for the
	// non-proxy path is therefore EventSessionUp; check by type
	// regardless of order.
	gotSessionUp := false
	for _, e := range got {
		if _, ok := e.(EventSessionUp); ok {
			gotSessionUp = true
		}
	}
	if !gotSessionUp {
		t.Errorf("did not receive EventSessionUp; got %+v", got)
	}

	// Backend calls.
	mtuCalls := backend.MTUCalls()
	if len(mtuCalls) != 1 || mtuCalls[0].name != "ppp2" || mtuCalls[0].mtu != 1496 {
		t.Errorf("MTU calls = %+v, want one ppp2=1496", mtuCalls)
	}
	if up := backend.UpCalls(); len(up) != 1 || up[0] != "ppp2" {
		t.Errorf("Up calls = %v, want [ppp2]", up)
	}

	// pppOps.setMRU should fire.
	opsMu.Lock()
	calls := append([]fakeOpsCall(nil), (*opsCalls)...)
	opsMu.Unlock()
	if len(calls) != 1 || calls[0].fd != 777 || calls[0].mru != 1500 {
		t.Errorf("ops calls = %+v, want one {fd=777, mru=1500}", calls)
	}
}

// VALIDATES: StartSession with ChanFD <= 0 or UnitFD <= 0 is rejected
//
//	before any goroutine spawns, and the transport is informed via
//	EventSessionRejected (not EventSessionDown).
//
// PREVENTS: silent drop of malformed StartSessions -- transport would
//
//	leave its in-flight tracking stuck.
func TestDriverRejectsInvalidFDs(t *testing.T) {
	backend := &fakeBackend{}
	ops, _, _ := newFakeOps()
	d := makeTestDriver(backend, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	cases := []struct {
		name  string
		start StartSession
	}{
		{"chan fd zero", StartSession{TunnelID: 1, SessionID: 1, ChanFD: 0, UnitFD: 10}},
		{"unit fd zero", StartSession{TunnelID: 1, SessionID: 2, ChanFD: 10, UnitFD: 0}},
		{"chan fd negative", StartSession{TunnelID: 1, SessionID: 3, ChanFD: -1, UnitFD: 10}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d.SessionsIn() <- tc.start
			ev := waitForEvent(t, d.EventsOut(), time.Second)
			rej, ok := ev.(EventSessionRejected)
			if !ok {
				t.Fatalf("got %T, want EventSessionRejected", ev)
			}
			if rej.TunnelID != tc.start.TunnelID || rej.SessionID != tc.start.SessionID {
				t.Errorf("ids mismatch: got (%d,%d) want (%d,%d)",
					rej.TunnelID, rej.SessionID, tc.start.TunnelID, tc.start.SessionID)
			}
			if rej.Reason == "" {
				t.Errorf("empty rejection reason")
			}
		})
	}

	// Sanity: the driver still accepts a valid session after the
	// rejections.
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 8001)
	defer closeConn(pair.peerEnd)
	stream := buildOptionStream([]LCPOption{mruOpt(1500), magicOpt(0xDEADBEEF)})
	d.SessionsIn() <- StartSession{
		TunnelID:            99,
		SessionID:           99,
		ChanFD:              8001,
		UnitFD:              8002,
		UnitNum:             1,
		ProxyLCPInitialRecv: stream,
		ProxyLCPLastSent:    stream,
		ProxyLCPLastRecv:    stream,
	}
	drainTwoEvents(t, d.EventsOut(), time.Second)
}

// VALIDATES: Duplicate (tunnelID, sessionID) is rejected with
//
//	EventSessionRejected, while the original session continues to
//	run undisturbed.
func TestDriverRejectsDuplicate(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 9001)
	defer closeConn(pair.peerEnd)

	backend := &fakeBackend{}
	ops, _, _ := newFakeOps()
	d := makeTestDriver(backend, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	stream := buildOptionStream([]LCPOption{mruOpt(1500), magicOpt(0xCAFEBABE)})
	first := StartSession{
		TunnelID:            7,
		SessionID:           8,
		ChanFD:              9001,
		UnitFD:              9002,
		UnitNum:             2,
		ProxyLCPInitialRecv: stream,
		ProxyLCPLastSent:    stream,
		ProxyLCPLastRecv:    stream,
	}
	d.SessionsIn() <- first
	// Drain the first session's LCPUp + SessionUp.
	drainTwoEvents(t, d.EventsOut(), time.Second)

	// Duplicate -- same (tunnelID, sessionID), different fd values
	// (the pipe registry is not consulted because the duplicate
	// gets rejected before fd wrapping).
	dup := first
	dup.ChanFD = 9101 // would fail wrap if reached, but must not reach
	dup.UnitFD = 9102
	d.SessionsIn() <- dup

	ev := waitForEvent(t, d.EventsOut(), time.Second)
	rej, ok := ev.(EventSessionRejected)
	if !ok {
		t.Fatalf("got %T, want EventSessionRejected", ev)
	}
	if rej.TunnelID != 7 || rej.SessionID != 8 {
		t.Errorf("rejection ids = (%d,%d), want (7,8)", rej.TunnelID, rej.SessionID)
	}

	// Original session is still present.
	if _, present := d.SessionByID(7, 8); !present {
		t.Errorf("original session removed after duplicate rejected")
	}
}

// waitForEvent reads one event from ch or fails the test on timeout.
func waitForEvent(t *testing.T, ch <-chan Event, timeout time.Duration) Event {
	t.Helper()
	select {
	case ev := <-ch:
		return ev
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for event")
	}
	return nil
}

// drainTwoEvents reads exactly two events from the channel, failing
// the test if not received within the timeout. Two is the canonical
// "LCP up + session up" pair fired on the success path.
func drainTwoEvents(t *testing.T, ch <-chan Event, timeout time.Duration) []Event {
	t.Helper()
	out := make([]Event, 0, 2)
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for len(out) < 2 {
		select {
		case ev := <-ch:
			out = append(out, ev)
		case <-deadline.C:
			t.Fatalf("timed out waiting for 2 events; got %d: %+v", len(out), out)
		}
	}
	return out
}

// drainEventsBest reads up to n events before the timeout fires and
// drops them on the floor. Best-effort, no failure. Used by tests
// that need to consume trailing lifecycle events during cleanup so
// subsequent channel observers do not receive them.
func drainEventsBest(t *testing.T, ch <-chan Event, n int, timeout time.Duration) {
	t.Helper()
	drained := 0
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for drained < n {
		select {
		case <-ch:
			drained++
		case <-deadline.C:
			return
		}
	}
}

func closeConn(c interface{ Close() error }) {
	if err := c.Close(); err != nil {
		// In tests, a close error after the driver already closed
		// the same fd is expected; ignore.
		_ = err
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
