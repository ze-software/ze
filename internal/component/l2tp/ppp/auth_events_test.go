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
func newTestDriverNoResponder(backend testIfaceBackend, ops pppOps) *Driver {
	return newDriver(driverConfig{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Backend: backend,
		Ops:     ops,
	})
}

// VALIDATES: AuthResponse returns ErrSessionNotFound for an unbound
// authentication request that cannot identify a live admitted worker.
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

	err := d.AuthResponse(EventAuthRequest{TunnelID: 99, SessionID: 99}, true, "", nil)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("AuthResponse(unknown) err = %v, want ErrSessionNotFound", err)
	}
}

// VALIDATES: a second decision for the same still-active request is refused
// with ErrAuthResponsePending and cannot overwrite the first decision.
// Method: an actual emitted request waits while a one-byte net.Pipe read
// blocks the worker in its Echo-Reply write, before decision consumption.
func TestDriverAuthResponseDoubleCallReturnsPending(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 7901)
	t.Cleanup(func() { closeConn(pair.peerEnd) })
	ops, _, _ := newFakeOps()
	driver := newTestDriverNoResponder(&fakeBackend{}, ops)
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	// Unblock a partial write before driver.Stop on every failure path.
	t.Cleanup(func() { closeConn(pair.peerEnd) })
	stream := buildOptionStream([]LCPOption{mruOpt(1500), magicOpt(0xCAFEBABE)})
	driver.SessionsIn() <- testStartForDriver(driver, &StartSession{
		LCP: testLCPConfig(), TunnelID: 7, SessionID: 8, LNSMode: true,
		MaxMRU: 1500, DisableIPCP: true, DisableIPv6CP: true,
		ProxyLCPInitialRecv: stream, ProxyLCPLastSent: stream, ProxyLCPLastRecv: stream,
	}, 7901, 7902, 7)
	var request EventAuthRequest
	select {
	case event := <-driver.AuthEventsOut():
		var ok bool
		request, ok = event.(EventAuthRequest)
		if !ok {
			t.Fatalf("event %T, want actual authentication request", event)
		}
	case <-time.After(time.Second):
		t.Fatal("no actual authentication request")
	}
	var frame [64]byte
	n := WriteFrame(frame[:], 0, ProtoLCP, nil)
	n += WriteLCPEcho(frame[:], n, LCPEchoRequest, 0x71, 0xCAFEBABE, nil)
	if err := pair.peerEnd.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := pair.peerEnd.Write(frame[:n]); err != nil {
		t.Fatal(err)
	}
	var reply [64]byte
	if _, err := io.ReadFull(pair.peerEnd, reply[:1]); err != nil {
		t.Fatal(err)
	}
	// net.Pipe has no write buffer: reading only one byte leaves the real
	// worker blocked sending the remainder, before it can consume a decision.
	if err := driver.AuthResponse(request, true, "", nil); err != nil {
		t.Fatalf("first AuthResponse: %v", err)
	}
	if err := driver.AuthResponse(request, true, "", nil); !errors.Is(err, ErrAuthResponsePending) {
		t.Errorf("second AuthResponse err = %v, want ErrAuthResponsePending", err)
	}
	if _, err := io.ReadFull(pair.peerEnd, reply[1:n]); err != nil {
		t.Fatal(err)
	}
	proto, payload, _, err := ParseFrame(reply[:n])
	if err != nil {
		t.Fatal(err)
	}
	packet, err := ParseLCPPacket(payload)
	if err != nil || proto != ProtoLCP || packet.Code != LCPEchoReply || packet.Identifier != 0x71 {
		t.Fatalf("blocked reply was not the requested echo: proto=%x packet=%+v err=%v", proto, packet, err)
	}
	if _, opened := waitForEventOfType[EventSessionUp](t, driver.EventsOut(), time.Second); !opened {
		t.Fatal("accepted authentication did not complete")
	}
	if err := driver.AuthResponse(request, true, "", nil); !errors.Is(err, ErrRequestExpired) {
		t.Fatalf("retired authentication reply err = %v, want ErrRequestExpired", err)
	}
}

// VALIDATES: StopSession unblocks a session parked in the auth phase
//
//	waiting on its private request reply lane, rather than hanging until the
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
	d.SessionsIn() <- testStartForDriver(d, &StartSession{
		LCP:                 testLCPConfig(),
		TunnelID:            11,
		SessionID:           22,
		LNSMode:             true,
		MaxMRU:              1500,
		ProxyLCPInitialRecv: stream,
		ProxyLCPLastSent:    stream,
		ProxyLCPLastRecv:    stream,
	}, 7001, 999, 1)

	// Wait for the auth request to hit the channel -- this proves
	// the session is parked in runAuthPhase's receive select.
	var request EventAuthRequest
	select {
	case ev := <-d.AuthEventsOut():
		observed, ok := ev.(EventAuthRequest)
		if !ok {
			t.Fatalf("first auth event %T, want EventAuthRequest", ev)
		}
		request = observed
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for EventAuthRequest")
	}

	// StopSession must return promptly. Before the fix, this line
	// blocked until Driver.Stop (the deferred cleanup) fired.
	done := make(chan error, 1)
	go func() { done <- d.StopSession(request.Control) }()
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
	d.SessionsIn() <- testStartForDriver(d, &StartSession{
		LCP:                 testLCPConfig(),
		TunnelID:            30,
		SessionID:           40,
		LNSMode:             true,
		MaxMRU:              1500,
		AuthTimeout:         100 * time.Millisecond,
		ProxyLCPInitialRecv: stream,
		ProxyLCPLastSent:    stream,
		ProxyLCPLastRecv:    stream,
	}, 7101, 1001, 2)

	// Drain EventAuthRequest so it does not pin the buffer.
	<-d.AuthEventsOut()

	// Expect EventAuthFailure{timeout} on auth channel within ~300ms.
	select {
	case ev := <-d.AuthEventsOut():
		fail, ok := ev.(eventAuthFailure)
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
	d.SessionsIn() <- testStartForDriver(d, &StartSession{
		LCP:                 testLCPConfig(),
		TunnelID:            50,
		SessionID:           60,
		LNSMode:             true,
		MaxMRU:              1500,
		ProxyLCPInitialRecv: stream,
		ProxyLCPLastSent:    stream,
		ProxyLCPLastRecv:    stream,
	}, 7201, 2001, 3)

	// First event must be the auth request.
	var capturedAuthRequest EventAuthRequest
	select {
	case ev := <-d.AuthEventsOut():
		req, ok := ev.(EventAuthRequest)
		capturedAuthRequest = req
		if !ok {
			t.Fatalf("first auth event %T, want EventAuthRequest", ev)
		}
		if req.TunnelID != 50 || req.SessionID != 60 {
			t.Errorf("request IDs = (%d,%d), want (50,60)", req.TunnelID, req.SessionID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for EventAuthRequest")
	}

	if err := d.AuthResponse(capturedAuthRequest, false, "credentials denied", nil); err != nil {
		t.Fatalf("AuthResponse(reject): %v", err)
	}

	// EventAuthFailure with the supplied reason.
	select {
	case ev := <-d.AuthEventsOut():
		fail, ok := ev.(eventAuthFailure)
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
	d.SessionsIn() <- testStartForDriver(d, &StartSession{
		LCP:                 testLCPConfig(),
		TunnelID:            55,
		SessionID:           66,
		LNSMode:             true,
		MaxMRU:              1500,
		AuthTimeout:         100 * time.Millisecond,
		ProxyLCPInitialRecv: stream,
		ProxyLCPLastSent:    stream,
		ProxyLCPLastRecv:    stream,
	}, 7301, 3001, 4)

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
	d.SessionsIn() <- testStartForDriver(d, &StartSession{
		LCP:                 testLCPConfig(),
		TunnelID:            70,
		SessionID:           80,
		LNSMode:             true,
		MaxMRU:              1500,
		DisableIPCP:         true,
		DisableIPv6CP:       true,
		ProxyLCPInitialRecv: stream,
		ProxyLCPLastSent:    stream,
		ProxyLCPLastRecv:    stream,
	}, 7401, 4001, 5)

	// EventAuthRequest arrives first on the auth channel.
	var capturedAuthRequest EventAuthRequest
	select {
	case ev := <-d.AuthEventsOut():
		observedRequest, ok := ev.(EventAuthRequest)
		capturedAuthRequest = observedRequest
		if !ok {
			t.Fatalf("first auth event %T, want EventAuthRequest", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for EventAuthRequest")
	}

	if err := d.AuthResponse(capturedAuthRequest, true, "", nil); err != nil {
		t.Fatalf("AuthResponse(accept): %v", err)
	}

	// EventAuthSuccess follows on the auth channel.
	select {
	case ev := <-d.AuthEventsOut():
		success, ok := ev.(eventAuthSuccess)
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

// VALIDATES: the defensive auth-phase guard refuses required authentication
// with no negotiated method. The normal driver refuses missing required proxy
// auth or LCP policy exhaustion before Opened; TestProxyLCPPolicy and
// TestAuthRefusalTerminationCleanup cover those wire paths.
// PREVENTS: a direct auth-phase caller accepting AuthMethodNone when
// authentication is mandatory.
func TestRunAuthPhaseRequiredAuthRejectsNoMethod(t *testing.T) {
	s, _, events := newRFC1661Session(LCPStateOpened)
	s.tunnelID = 71
	s.sessionID = 81
	s.authRequired = true
	s.negotiatedAuthMethod = AuthMethodNone
	auth := make(chan AuthEvent, 4)
	s.authEventsOut = auth
	if s.runAuthPhase() {
		t.Fatal("required authentication accepted no negotiated method")
	}
	select {
	case ev := <-auth:
		fail, ok := ev.(eventAuthFailure)
		if !ok {
			t.Fatalf("auth event %T, want EventAuthFailure", ev)
		}
		if fail.Reason != "no negotiated authentication method" {
			t.Errorf("failure reason = %q, want no negotiated authentication method", fail.Reason)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for EventAuthFailure")
	}
	if !waitForSessionDown(t, events, 500*time.Millisecond) {
		t.Fatal("timed out waiting for EventSessionDown after required auth failed")
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
