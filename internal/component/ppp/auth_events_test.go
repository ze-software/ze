package ppp

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

// newTestDriverNoResponder constructs a Driver without an auto-accept
// responder. Tests that exercise the auth channel directly -- reject,
// timeout, request-content assertions -- use this instead of
// makeTestDriver so autoAcceptAuth does not race them to the channel.
func newTestDriverNoResponder(backend IfaceBackend, ops pppOps) *Driver {
	return NewDriver(DriverConfig{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Backend: backend,
		Ops:     ops,
	})
}

// VALIDATES: AuthResponse returns ErrSessionNotFound for a
//
//	(tunnelID, sessionID) pair that was never registered with
//	the Driver.
//
// PREVENTS: regression where the error path silently returns nil
//
//	and callers assume the consumer side actually delivered a
//	decision that was in fact dropped.
func TestDriverAuthResponseUnknownSession(t *testing.T) {
	ops, _, _ := newFakeOps()
	d := newTestDriverNoResponder(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	err := d.AuthResponse(99, 99, true, "", nil)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("AuthResponse(unknown) err = %v, want ErrSessionNotFound", err)
	}
}

// VALIDATES: AuthResponse returns ErrAuthResponsePending when the
//
//	per-session buffered(1) authRespCh already holds an
//	unconsumed decision.
//
// PREVENTS: regression where a duplicate AuthResponse blocks the
//
//	caller or silently overwrites the first decision.
//
// Trade-off: this test inserts a pppSession shell directly into
// d.sessions instead of spawning a real session. The real-session
// path is race-sensitive (must land the second AuthResponse before
// the first is consumed by runAuthPhase), whereas the shell
// approach is deterministic. The cost is fragility against future
// spawnSession invariant changes -- if spawnSession starts
// initializing fields this test does not set, they will read as
// zero-values here. Accept that cost for the determinism.
func TestDriverAuthResponseDoubleCallReturnsPending(t *testing.T) {
	ops, _, _ := newFakeOps()
	d := newTestDriverNoResponder(&fakeBackend{}, ops)
	// No Start: we only manipulate the sessions map for this unit test.

	// Register a pppSession shell with a buffered(1) authRespCh.
	// No goroutine runs; we only exercise the AuthResponse path.
	k := sessionKey{tunnelID: 7, sessionID: 8}
	s := &pppSession{
		tunnelID:   k.tunnelID,
		sessionID:  k.sessionID,
		authRespCh: make(chan authResponseMsg, 1),
		sessStop:   make(chan struct{}),
		done:       make(chan struct{}),
	}
	d.mu.Lock()
	d.sessions[k] = s
	d.mu.Unlock()

	if err := d.AuthResponse(7, 8, true, "", nil); err != nil {
		t.Fatalf("first AuthResponse: %v", err)
	}
	if err := d.AuthResponse(7, 8, true, "", nil); !errors.Is(err, ErrAuthResponsePending) {
		t.Errorf("second AuthResponse err = %v, want ErrAuthResponsePending", err)
	}

	// Drain the buffered message so the channel is not GC-pinned.
	<-s.authRespCh
}

// VALIDATES: StopSession unblocks a session parked in the auth phase
//
//	waiting on authRespCh, rather than hanging until the
//	whole Driver is stopped.
//
// PREVENTS: regression of the BLOCKER identified in /ze-review where
//
//	a session in auth phase could not be individually torn
//	down because chanFile close only unblocks readFrames
//	(which is not running during the proxy-LCP synchronous
//	afterLCPOpen path).
func TestDriverStopSessionDuringAuthPhase(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 7001)
	defer closeConn(pair.peerEnd)

	ops, _, _ := newFakeOps()
	d := newTestDriverNoResponder(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	// Proxy-LCP short-circuit so the session reaches afterLCPOpen
	// (and therefore runAuthPhase) without needing a scripted peer.
	stream := buildOptionStream([]LCPOption{mruOpt(1500), magicOpt(0xCAFEBABE)})
	d.SessionsIn() <- StartSession{
		TunnelID:            11,
		SessionID:           22,
		ChanFD:              7001,
		UnitFD:              999,
		UnitNum:             1,
		LNSMode:             true,
		MaxMRU:              1500,
		ProxyLCPInitialRecv: stream,
		ProxyLCPLastSent:    stream,
		ProxyLCPLastRecv:    stream,
	}

	// Wait for the auth request to hit the channel -- this proves
	// the session is parked in runAuthPhase's receive select.
	select {
	case ev := <-d.AuthEventsOut():
		if _, ok := ev.(EventAuthRequest); !ok {
			t.Fatalf("first auth event %T, want EventAuthRequest", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for EventAuthRequest")
	}

	// StopSession must return promptly. Before the fix, this line
	// blocked until Driver.Stop (the deferred cleanup) fired.
	done := make(chan error, 1)
	go func() { done <- d.StopSession(11, 22) }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("StopSession: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("StopSession hung; sessStop unblock regression")
	}
}

// VALIDATES: With no responder, runAuthPhase fires defaultAuthTimeout
//
//	and emits EventAuthFailure{Reason: "timeout"} on the
//	auth channel plus EventSessionDown on the lifecycle
//	channel.
//
// PREVENTS: production resource exhaustion when the auth consumer
//
//	is missing or stalled; a session with no decision must
//	not park forever.
func TestRunAuthPhaseTimeout(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 7101)
	defer closeConn(pair.peerEnd)

	ops, _, _ := newFakeOps()
	d := newTestDriverNoResponder(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	stream := buildOptionStream([]LCPOption{mruOpt(1500), magicOpt(0xCAFEBABE)})
	d.SessionsIn() <- StartSession{
		TunnelID:            30,
		SessionID:           40,
		ChanFD:              7101,
		UnitFD:              1001,
		UnitNum:             2,
		LNSMode:             true,
		MaxMRU:              1500,
		AuthTimeout:         100 * time.Millisecond,
		ProxyLCPInitialRecv: stream,
		ProxyLCPLastSent:    stream,
		ProxyLCPLastRecv:    stream,
	}

	// Drain EventAuthRequest so it does not pin the buffer.
	<-d.AuthEventsOut()

	// Expect EventAuthFailure{timeout} on auth channel within ~300ms.
	select {
	case ev := <-d.AuthEventsOut():
		fail, ok := ev.(EventAuthFailure)
		if !ok {
			t.Fatalf("auth event %T, want EventAuthFailure", ev)
		}
		if fail.Reason != "timeout" {
			t.Errorf("failure reason = %q, want \"timeout\"", fail.Reason)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for EventAuthFailure{timeout}")
	}

	// Expect EventSessionDown on lifecycle channel.
	if !waitForSessionDown(t, d.EventsOut(), 500*time.Millisecond) {
		t.Fatal("timed out waiting for EventSessionDown after auth timeout")
	}
}

// VALIDATES: AuthResponse(accept=false) causes runAuthPhase to emit
//
//	EventAuthFailure with the operator-supplied reason on
//	the auth channel and EventSessionDown on the lifecycle
//	channel.
//
// PREVENTS: regression where a reject decision is silently dropped
//
//	or where the failure reason from the external handler
//	does not propagate into logs and events.
func TestRunAuthPhaseReject(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 7201)
	defer closeConn(pair.peerEnd)

	ops, _, _ := newFakeOps()
	d := newTestDriverNoResponder(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	stream := buildOptionStream([]LCPOption{mruOpt(1500), magicOpt(0xCAFEBABE)})
	d.SessionsIn() <- StartSession{
		TunnelID:            50,
		SessionID:           60,
		ChanFD:              7201,
		UnitFD:              2001,
		UnitNum:             3,
		LNSMode:             true,
		MaxMRU:              1500,
		ProxyLCPInitialRecv: stream,
		ProxyLCPLastSent:    stream,
		ProxyLCPLastRecv:    stream,
	}

	// First event must be the auth request.
	select {
	case ev := <-d.AuthEventsOut():
		req, ok := ev.(EventAuthRequest)
		if !ok {
			t.Fatalf("first auth event %T, want EventAuthRequest", ev)
		}
		if req.TunnelID != 50 || req.SessionID != 60 {
			t.Errorf("request IDs = (%d,%d), want (50,60)", req.TunnelID, req.SessionID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for EventAuthRequest")
	}

	if err := d.AuthResponse(50, 60, false, "credentials denied", nil); err != nil {
		t.Fatalf("AuthResponse(reject): %v", err)
	}

	// EventAuthFailure with the supplied reason.
	select {
	case ev := <-d.AuthEventsOut():
		fail, ok := ev.(EventAuthFailure)
		if !ok {
			t.Fatalf("auth event %T, want EventAuthFailure", ev)
		}
		if fail.Reason != "credentials denied" {
			t.Errorf("failure reason = %q, want \"credentials denied\"", fail.Reason)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for EventAuthFailure")
	}

	// EventSessionDown on lifecycle channel.
	if !waitForSessionDown(t, d.EventsOut(), 500*time.Millisecond) {
		t.Fatal("timed out waiting for EventSessionDown after auth reject")
	}
}

// VALIDATES: EventAuthRequest content reflects the session's tunnel
//
//	and session IDs and carries AuthMethodNone in Phase 1
//	(wire codecs land in later phases).
//
// PREVENTS: regression where session identity is lost between the
//
//	Driver and the external auth handler.
func TestEventAuthRequestContent(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 7301)
	defer closeConn(pair.peerEnd)

	ops, _, _ := newFakeOps()
	d := newTestDriverNoResponder(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	stream := buildOptionStream([]LCPOption{mruOpt(1500), magicOpt(0xCAFEBABE)})
	d.SessionsIn() <- StartSession{
		TunnelID:            55,
		SessionID:           66,
		ChanFD:              7301,
		UnitFD:              3001,
		UnitNum:             4,
		LNSMode:             true,
		MaxMRU:              1500,
		AuthTimeout:         100 * time.Millisecond,
		ProxyLCPInitialRecv: stream,
		ProxyLCPLastSent:    stream,
		ProxyLCPLastRecv:    stream,
	}

	select {
	case ev := <-d.AuthEventsOut():
		req, ok := ev.(EventAuthRequest)
		if !ok {
			t.Fatalf("auth event %T, want EventAuthRequest", ev)
		}
		if req.TunnelID != 55 || req.SessionID != 66 {
			t.Errorf("IDs = (%d,%d), want (55,66)", req.TunnelID, req.SessionID)
		}
		if req.Method != AuthMethodNone {
			t.Errorf("method = %v, want AuthMethodNone", req.Method)
		}
		if req.Username != "" || len(req.Challenge) != 0 || len(req.Response) != 0 {
			t.Errorf("unexpected Phase 1 content: %+v", req)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for EventAuthRequest")
	}
}

// VALIDATES: AuthResponse(accept=true) causes runAuthPhase to emit
//
//	EventAuthSuccess on the auth channel and EventSessionUp
//	on the lifecycle channel.
//
// PREVENTS: regression where the accept path silently drops the
//
//	EventAuthSuccess emission; happy-path tests drain it via
//	autoAcceptAuth and would not catch the gap.
func TestRunAuthPhaseAccept(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 7401)
	defer closeConn(pair.peerEnd)

	ops, _, _ := newFakeOps()
	d := newTestDriverNoResponder(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	stream := buildOptionStream([]LCPOption{mruOpt(1500), magicOpt(0xCAFEBABE)})
	d.SessionsIn() <- StartSession{
		TunnelID:            70,
		SessionID:           80,
		ChanFD:              7401,
		UnitFD:              4001,
		UnitNum:             5,
		LNSMode:             true,
		MaxMRU:              1500,
		ProxyLCPInitialRecv: stream,
		ProxyLCPLastSent:    stream,
		ProxyLCPLastRecv:    stream,
	}

	// EventAuthRequest arrives first on the auth channel.
	select {
	case ev := <-d.AuthEventsOut():
		if _, ok := ev.(EventAuthRequest); !ok {
			t.Fatalf("first auth event %T, want EventAuthRequest", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for EventAuthRequest")
	}

	if err := d.AuthResponse(70, 80, true, "", nil); err != nil {
		t.Fatalf("AuthResponse(accept): %v", err)
	}

	// EventAuthSuccess follows on the auth channel.
	select {
	case ev := <-d.AuthEventsOut():
		success, ok := ev.(EventAuthSuccess)
		if !ok {
			t.Fatalf("auth event %T, want EventAuthSuccess", ev)
		}
		if success.TunnelID != 70 || success.SessionID != 80 {
			t.Errorf("success IDs = (%d,%d), want (70,80)", success.TunnelID, success.SessionID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for EventAuthSuccess")
	}

	// EventSessionUp on the lifecycle channel (after the earlier
	// EventLCPUp that the proxy path emits).
	if !waitForSessionUp(t, d.EventsOut(), 500*time.Millisecond) {
		t.Fatal("timed out waiting for EventSessionUp after auth accept")
	}
}

// waitForSessionUp drains lifecycle events until EventSessionUp
// arrives or the deadline fires. Returns true on receipt.
func waitForSessionUp(t *testing.T, ch <-chan Event, d time.Duration) bool {
	t.Helper()
	deadline := time.After(d)
	for {
		select {
		case ev := <-ch:
			if _, ok := ev.(EventSessionUp); ok {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

// waitForSessionDown drains lifecycle events until EventSessionDown
// arrives or the deadline fires. Returns true on receipt.
func waitForSessionDown(t *testing.T, ch <-chan Event, d time.Duration) bool {
	t.Helper()
	deadline := time.After(d)
	for {
		select {
		case ev := <-ch:
			if _, ok := ev.(EventSessionDown); ok {
				return true
			}
		case <-deadline:
			return false
		}
	}
}
