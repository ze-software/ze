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

// VALIDATES: selectAuthFallback picks the peer's suggested method when
//
//	it is present in ze's fallback order, and returns
//	AuthMethodNone otherwise. The order list is consulted
//	for membership only -- position does not downgrade a
//	matched method to an earlier one.
//
// PREVENTS: regression where an empty order or an order that omits the
//
//	peer's choice silently permits the rejected method, and
//	regression where a matched suggestion is rewritten to the
//	first list entry instead of returned verbatim.
func TestSelectAuthFallback(t *testing.T) {
	full := []AuthMethod{AuthMethodCHAPMD5, AuthMethodMSCHAPv2, AuthMethodPAP}
	cases := []struct {
		name       string
		suggestion AuthMethod
		order      []AuthMethod
		want       AuthMethod
	}{
		{"peer suggests CHAP-MD5, in full order", AuthMethodCHAPMD5, full, AuthMethodCHAPMD5},
		{"peer suggests MS-CHAPv2, in full order", AuthMethodMSCHAPv2, full, AuthMethodMSCHAPv2},
		{"peer suggests PAP, in full order", AuthMethodPAP, full, AuthMethodPAP},
		{"peer suggests PAP, CHAP-only order", AuthMethodPAP, []AuthMethod{AuthMethodCHAPMD5}, AuthMethodNone},
		{"peer suggests None (degenerate)", AuthMethodNone, full, AuthMethodNone},
		{"nil order", AuthMethodPAP, nil, AuthMethodNone},
		{"empty order", AuthMethodCHAPMD5, []AuthMethod{}, AuthMethodNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := selectAuthFallback(tc.suggestion, tc.order)
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// VALIDATES: adjustAuthOnNakOrReject mutates configuredAuthMethod
//
//	correctly for each RFC 1661 §5.3/§5.4 code+payload
//	combination, and leaves the field untouched when the
//	packet carries no Auth-Protocol option or when the
//	option bytes cannot be decoded.
//
// PREVENTS: regression where the helper silently mutates for unrelated
//
//	options (e.g., a Nak of MRU should NOT clear the auth
//	method), or where malformed Auth-Protocol bytes are
//	interpreted as valid methods, or where a short option
//	(<2 bytes) crashes the uint16 decode.
func TestAdjustAuthOnNakOrReject(t *testing.T) {
	// Helper: build an LCP options payload from the supplied options.
	buildData := func(opts []LCPOption) []byte {
		buf := make([]byte, 256)
		n := WriteLCPOptions(buf, 0, opts)
		return buf[:n]
	}

	cases := []struct {
		name       string
		start      AuthMethod
		order      []AuthMethod
		code       uint8
		data       []byte
		wantMethod AuthMethod
	}{
		{
			name:       "Reject echoing Auth-Protocol clears method",
			start:      AuthMethodPAP,
			order:      []AuthMethod{AuthMethodCHAPMD5, AuthMethodPAP},
			code:       LCPConfigureReject,
			data:       buildData([]LCPOption{authProtoOpt(authProtoPAP)}),
			wantMethod: AuthMethodNone,
		},
		{
			name:       "Reject of unrelated MRU option leaves method alone",
			start:      AuthMethodPAP,
			order:      []AuthMethod{AuthMethodCHAPMD5, AuthMethodPAP},
			code:       LCPConfigureReject,
			data:       buildData([]LCPOption{mruOpt(1500)}),
			wantMethod: AuthMethodPAP,
		},
		{
			name:       "Nak suggesting CHAP-MD5 when in order accepts it",
			start:      AuthMethodPAP,
			order:      []AuthMethod{AuthMethodCHAPMD5, AuthMethodPAP},
			code:       LCPConfigureNak,
			data:       buildData([]LCPOption{authProtoOpt(authProtoCHAP, chapAlgorithmMD5)}),
			wantMethod: AuthMethodCHAPMD5,
		},
		{
			name:       "Nak suggesting method not in order clears to None",
			start:      AuthMethodCHAPMD5,
			order:      []AuthMethod{AuthMethodCHAPMD5},
			code:       LCPConfigureNak,
			data:       buildData([]LCPOption{authProtoOpt(authProtoPAP)}),
			wantMethod: AuthMethodNone,
		},
		{
			name:       "Nak of unrelated MRU option leaves method alone",
			start:      AuthMethodPAP,
			order:      []AuthMethod{AuthMethodPAP},
			code:       LCPConfigureNak,
			data:       buildData([]LCPOption{mruOpt(1500)}),
			wantMethod: AuthMethodPAP,
		},
		{
			name:       "Auth-Protocol option with 1-byte data clears to None",
			start:      AuthMethodPAP,
			order:      []AuthMethod{AuthMethodPAP},
			code:       LCPConfigureNak,
			data:       []byte{LCPOptAuthProto, 0x03, 0xC0}, // Length=3, one proto byte
			wantMethod: AuthMethodNone,
		},
		{
			name:       "Malformed option stream (parse error) leaves method alone",
			start:      AuthMethodPAP,
			order:      []AuthMethod{AuthMethodPAP},
			code:       LCPConfigureNak,
			data:       []byte{0x03, 0x00}, // Length=0 is below header minimum
			wantMethod: AuthMethodPAP,
		},
		{
			name:       "Empty data leaves method alone",
			start:      AuthMethodPAP,
			order:      []AuthMethod{AuthMethodPAP},
			code:       LCPConfigureReject,
			data:       nil,
			wantMethod: AuthMethodPAP,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &pppSession{
				configuredAuthMethod: tc.start,
				authFallbackOrder:    tc.order,
				logger:               discardLogger(),
			}
			pkt := LCPPacket{
				Code:       tc.code,
				Identifier: 1,
				Data:       tc.data,
			}
			s.adjustAuthOnNakOrReject(pkt)
			if s.configuredAuthMethod != tc.wantMethod {
				t.Errorf("configuredAuthMethod = %v, want %v", s.configuredAuthMethod, tc.wantMethod)
			}
		})
	}
}

// VALIDATES: defaultAuthFallbackOrder matches the spec AC-13 guidance
//
//	"prefer CHAP > MS-CHAPv2 > PAP" and returns a fresh
//	slice on every call (no shared mutation hazard).
//
// PREVENTS: regression where the default order is silently reshuffled
//
//	to put PAP (cleartext on wire) before CHAP.
func TestDefaultAuthFallbackOrder(t *testing.T) {
	a := defaultAuthFallbackOrder()
	want := []AuthMethod{AuthMethodCHAPMD5, AuthMethodMSCHAPv2, AuthMethodPAP}
	if len(a) != len(want) {
		t.Fatalf("length = %d, want %d", len(a), len(want))
	}
	for i, m := range want {
		if a[i] != m {
			t.Errorf("index %d = %v, want %v", i, a[i], m)
		}
	}

	// Mutate one copy and verify the next call is unaffected.
	a[0] = AuthMethodPAP
	b := defaultAuthFallbackOrder()
	if b[0] != AuthMethodCHAPMD5 {
		t.Errorf("mutation of first call leaked into second: b[0] = %v", b[0])
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
