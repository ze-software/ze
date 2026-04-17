package ppp

import (
	"bytes"
	"testing"
	"time"
)

// VALIDATES: authMethodFromAuthProto decodes every known Auth-Protocol
//
//	value + algorithm pair into the matching AuthMethod, and
//	returns AuthMethodNone for unknown / malformed inputs.
//
// PREVENTS: regression where adding a new AuthMethod silently falls
//
//	through to None, or where a malformed CHAP option is
//	accepted as one of the concrete variants.
func TestAuthMethodFromAuthProto(t *testing.T) {
	cases := []struct {
		name      string
		proto     uint16
		algorithm []byte
		want      AuthMethod
	}{
		{"zero proto", 0, nil, AuthMethodNone},
		{"PAP, empty algorithm", 0xC023, nil, AuthMethodPAP},
		{"PAP, trailing byte ignored", 0xC023, []byte{0xFF}, AuthMethodPAP},
		{"CHAP, empty algorithm", 0xC223, nil, AuthMethodNone},
		{"CHAP-MD5", 0xC223, []byte{0x05}, AuthMethodCHAPMD5},
		{"CHAP-MD5, trailing byte ignored", 0xC223, []byte{0x05, 0xAA}, AuthMethodCHAPMD5},
		{"MS-CHAPv2", 0xC223, []byte{0x81}, AuthMethodMSCHAPv2},
		{"CHAP unknown algorithm", 0xC223, []byte{0x42}, AuthMethodNone},
		{"EAP (not supported)", 0xC227, nil, AuthMethodNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := authMethodFromAuthProto(tc.proto, tc.algorithm)
			if got != tc.want {
				t.Errorf("proto=0x%04x algo=%x got %v, want %v",
					tc.proto, tc.algorithm, got, tc.want)
			}
		})
	}
}

// VALIDATES: authMethodToLCPOptions emits the on-wire Auth-Protocol
//
//	value and any CHAP Algorithm bytes for every AuthMethod,
//	and returns (0, nil) for AuthMethodNone.
//
// PREVENTS: regression where CHAP-MD5 and MS-CHAPv2 advertise the
//
//	wrong Algorithm byte, producing a CONFREQ the peer ACKs
//	under a different method than ze will run.
func TestAuthMethodToLCPOptions(t *testing.T) {
	cases := []struct {
		name      string
		m         AuthMethod
		wantProto uint16
		wantData  []byte
	}{
		{"None", AuthMethodNone, 0, nil},
		{"PAP", AuthMethodPAP, 0xC023, nil},
		{"CHAP-MD5", AuthMethodCHAPMD5, 0xC223, []byte{0x05}},
		{"MS-CHAPv2", AuthMethodMSCHAPv2, 0xC223, []byte{0x81}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proto, data := authMethodToLCPOptions(tc.m)
			if proto != tc.wantProto {
				t.Errorf("proto = 0x%04x, want 0x%04x", proto, tc.wantProto)
			}
			if !bytes.Equal(data, tc.wantData) {
				t.Errorf("data = %x, want %x", data, tc.wantData)
			}
		})
	}
}

// VALIDATES: authMethodToLCPOptions and authMethodFromAuthProto
//
//	round-trip for every concrete method: encode yields the
//	same (proto, algorithm) pair that decode consumes.
//
// PREVENTS: drift between the encoder and decoder if only one side is
//
//	updated when a new method is added.
func TestAuthMethodRoundTrip(t *testing.T) {
	for _, m := range []AuthMethod{AuthMethodPAP, AuthMethodCHAPMD5, AuthMethodMSCHAPv2} {
		proto, algorithm := authMethodToLCPOptions(m)
		got := authMethodFromAuthProto(proto, algorithm)
		if got != m {
			t.Errorf("round-trip %v: decode yielded %v", m, got)
		}
	}
}

// VALIDATES: awaitAuthDecision emits the request, returns the handler's
//
//	decision on authRespCh, and does NOT fire the timeout
//	timer when the decision arrives promptly.
//
// PREVENTS: regression where a handler that responds quickly still
//
//	suffers a timeout fail-path because the timer races the
//	decision receive.
func TestAwaitAuthDecisionAccept(t *testing.T) {
	authEventsOut := make(chan AuthEvent, 4)
	s := &pppSession{
		tunnelID:      11,
		sessionID:     22,
		authEventsOut: authEventsOut,
		authRespCh:    make(chan authResponseMsg, 1),
		stopCh:        make(chan struct{}),
		sessStop:      make(chan struct{}),
		done:          make(chan struct{}),
		authTimeout:   2 * time.Second,
		logger:        discardLogger(),
	}
	s.authRespCh <- authResponseMsg{accept: true, message: "hello"}

	req := EventAuthRequest{TunnelID: 11, SessionID: 22, Method: AuthMethodPAP}
	got, ok := s.awaitAuthDecision(req, "pap")
	if !ok {
		t.Fatalf("awaitAuthDecision returned ok=false on accept")
	}
	if !got.accept || got.message != "hello" {
		t.Errorf("decision = %+v, want accept=true message=hello", got)
	}

	select {
	case ev := <-authEventsOut:
		if _, ok := ev.(EventAuthRequest); !ok {
			t.Errorf("emitted event %T, want EventAuthRequest", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("EventAuthRequest not emitted")
	}
}

// VALIDATES: awaitAuthDecision fires the per-session auth timeout,
//
//	emits EventAuthFailure{Reason:"timeout"} on the auth
//	channel and EventSessionDown on the lifecycle channel,
//	and returns ok=false.
//
// PREVENTS: regression where a stalled auth handler blocks the
//
//	session indefinitely.
func TestAwaitAuthDecisionTimeout(t *testing.T) {
	authEventsOut := make(chan AuthEvent, 4)
	eventsOut := make(chan Event, 4)
	s := &pppSession{
		tunnelID:      33,
		sessionID:     44,
		eventsOut:     eventsOut,
		authEventsOut: authEventsOut,
		authRespCh:    make(chan authResponseMsg, 1),
		stopCh:        make(chan struct{}),
		sessStop:      make(chan struct{}),
		done:          make(chan struct{}),
		authTimeout:   60 * time.Millisecond,
		logger:        discardLogger(),
	}

	req := EventAuthRequest{TunnelID: 33, SessionID: 44, Method: AuthMethodCHAPMD5}
	start := time.Now()
	_, ok := s.awaitAuthDecision(req, "chap")
	elapsed := time.Since(start)
	if ok {
		t.Fatalf("awaitAuthDecision returned ok=true on timeout")
	}
	if elapsed < 40*time.Millisecond {
		t.Errorf("returned too fast: %v (want >= 40ms)", elapsed)
	}

	<-authEventsOut // drop the EventAuthRequest
	select {
	case ev := <-authEventsOut:
		fail, ok := ev.(EventAuthFailure)
		if !ok {
			t.Fatalf("auth event %T, want EventAuthFailure", ev)
		}
		if fail.Reason != "timeout" {
			t.Errorf("failure reason = %q, want \"timeout\"", fail.Reason)
		}
	case <-time.After(time.Second):
		t.Fatal("EventAuthFailure not emitted")
	}
	select {
	case ev := <-eventsOut:
		if _, ok := ev.(EventSessionDown); !ok {
			t.Errorf("lifecycle event %T, want EventSessionDown", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("EventSessionDown not emitted")
	}
}

// VALIDATES: awaitAuthDecision returns ok=false without emitting a
//
//	timeout failure when s.stopCh closes first (driver-wide
//	shutdown path).
//
// PREVENTS: regression where Driver.Stop during auth leaks a spurious
//
//	EventAuthFailure{Reason:"timeout"} on the channel.
func TestAwaitAuthDecisionStopCh(t *testing.T) {
	authEventsOut := make(chan AuthEvent, 4)
	stopCh := make(chan struct{})
	s := &pppSession{
		tunnelID:      55,
		sessionID:     66,
		authEventsOut: authEventsOut,
		authRespCh:    make(chan authResponseMsg, 1),
		stopCh:        stopCh,
		sessStop:      make(chan struct{}),
		done:          make(chan struct{}),
		authTimeout:   2 * time.Second,
		logger:        discardLogger(),
	}
	close(stopCh)

	_, ok := s.awaitAuthDecision(EventAuthRequest{}, "chap-v2")
	if ok {
		t.Fatalf("awaitAuthDecision returned ok=true with stopCh closed")
	}
	// Tolerated race: the emit select can either win (pushing one
	// EventAuthRequest onto the buffered channel before noticing
	// stopCh) or lose (returning without emitting). Both outcomes
	// satisfy the contract "return ok=false on stopCh", so the test
	// accepts either. The assertion is that NO EventAuthFailure is
	// queued -- timeout / reject paths synthesize failure events,
	// the stopCh path must not.
	select {
	case ev := <-authEventsOut:
		if _, isReq := ev.(EventAuthRequest); !isReq {
			t.Errorf("unexpected auth event %T after stopCh close", ev)
		}
	default:
	}
}
