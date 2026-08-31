// RFC 5176 (Dynamic Authorization Extensions to RADIUS) Dynamic Authorization Server
// obligations, extracted by the sign-off walk recorded in rfc/extraction/rfc5176.json.
//
// VALIDATES: the CoA/Disconnect listener is the one RADIUS surface where ze runs a
// server rather than a client, so every packet it reads is unsolicited and every guard
// below stands between the network and a session teardown. The producers are
// coaListener.handlePacket, handleCoA, handleDisconnect, oneSession, findSessions,
// sendResponse and eventTimestampState in coa.go, plus radius.VerifyCoARequestAuth and
// radius.VerifyCoAMessageAuthenticator in internal/component/radius/packet.go.
//
// Each test drives the listener over a real UDP socket, which is the entry point a
// Dynamic Authorization Client reaches. No test calls a helper directly.

package l2tpauthradius

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"encoding/binary"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/pkg/ze"
)

// serviceTypeAuthorizeOnly is the Service-Type value RFC 5176 Section 3.2 names.
const serviceTypeAuthorizeOnly = 17

// recordingBus is an event bus that records what it was asked to publish and can be
// told to refuse. It stands in for the shaper's route out of the listener.
type recordingBus struct {
	mu     sync.Mutex
	events []recordedEvent
	err    error
}

type recordedEvent struct {
	namespace string
	eventType string
	payload   any
}

func (b *recordingBus) Emit(namespace, eventType string, payload any) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.err != nil {
		return 0, b.err
	}
	b.events = append(b.events, recordedEvent{namespace: namespace, eventType: eventType, payload: payload})
	return 0, nil
}

func (b *recordingBus) Subscribe(_, _ string, _ func(any)) func() { return func() {} }

func (b *recordingBus) recorded() []recordedEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]recordedEvent(nil), b.events...)
}

var _ ze.EventBus = (*recordingBus)(nil)

// coaTestListener starts a listener on an ephemeral port with the loopback
// address trusted, publishes fake, and answers the listener's address.
func coaTestListener(t *testing.T, secret []byte, bus ze.EventBus, fake *fakeL2TPService) string {
	t.Helper()
	if fake != nil {
		l2tp.PublishService(fake)
		t.Cleanup(func() { l2tp.PublishService(nil) })
	}
	cl, err := newCoAListener(coaListenerConfig{AllowedSources: coaLoopbackSources(), DefaultSecret: secret, Bus: bus})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := cl.Close(); closeErr != nil {
			t.Logf("close: %v", closeErr)
		}
	})
	return cl.conn.LocalAddr().String()
}

// oneSessionService answers a service holding a single session: tunnel 10, session 20,
// user alice. Its Acct-Session-Id prefix is "10-20-".
func oneSessionService() *fakeL2TPService {
	return &fakeL2TPService{snap: l2tp.Snapshot{
		Tunnels: []l2tp.TunnelSnapshot{{
			LocalTID: 10,
			Sessions: []l2tp.SessionSnapshot{{
				LocalSID:       20,
				TunnelLocalTID: 10,
				Username:       "alice",
			}},
		}},
	}}
}

// errorCause answers the Error-Cause value a response carries, and whether it carries
// one at all.
func errorCause(t *testing.T, resp *radius.Packet) (uint32, bool) {
	t.Helper()
	raw := resp.FindAttr(radius.AttrErrorCause)
	if raw == nil {
		return 0, false
	}
	if len(raw) != 4 {
		t.Fatalf("Error-Cause length: got %d, want 4", len(raw))
	}
	return binary.BigEndian.Uint32(raw), true
}

// wantNAK fails unless the response is the expected NAK carrying the expected cause.
func wantNAK(t *testing.T, resp *radius.Packet, code uint8, cause uint32) {
	t.Helper()
	if resp.Code != code {
		t.Errorf("code: got %d, want %d", resp.Code, code)
	}
	got, present := errorCause(t, resp)
	if !present {
		t.Fatal("response carries no Error-Cause attribute")
	}
	if got != cause {
		t.Errorf("Error-Cause: got %d, want %d", got, cause)
	}
}

// RFC requirement: RFC5176-2.3-1 positive -- a packet whose Code names a response
// rather than a request is silently discarded, so a reflected CoA-ACK cannot drive the
// listener.
// RFC requirement: RFC5176-2.3-1 negative -- a packet whose Code is CoA-Request is not
// discarded; it is answered, so the discard is specific to the invalid Code.
func TestRFC5176InvalidCodeDiscarded(t *testing.T) {
	secret := []byte("test-rfc5176-code-secret")
	addr := coaTestListener(t, secret, nil, nil)

	sendRawCoAPacketExpectNoResponse(t, addr, buildCoAPacket(t, radius.CodeCoAACK, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
	}, time.Now()))

	resp := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
	})
	if resp.Code != radius.CodeCoANAK {
		t.Errorf("valid code: got %d, want %d (CoA-NAK)", resp.Code, radius.CodeCoANAK)
	}
}

// RFC requirement: RFC5176-2.3-2 positive -- a duplicate request carrying the same
// source, Identifier and Request Authenticator is answered from the cache, so the
// session is torn down once however many copies arrive.
// RFC requirement: RFC5176-2.3-2 negative -- a request carrying a different Identifier
// is not a duplicate: it is processed afresh and answered under its own Identifier.
func TestRFC5176DuplicateRequestAnsweredFromCache(t *testing.T) {
	secret := []byte("test-rfc5176-dup-secret")
	fake := oneSessionService()
	addr := coaTestListener(t, secret, nil, fake)

	attrs := []radius.Attr{{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")}}
	wire := buildCoAPacket(t, radius.CodeDisconnectRequest, secret, attrs, time.Now())

	first := sendRawCoAPacket(t, addr, append([]byte(nil), wire...))
	second := sendRawCoAPacket(t, addr, append([]byte(nil), wire...))

	if first.Code != radius.CodeDisconnectACK || second.Code != radius.CodeDisconnectACK {
		t.Errorf("codes: got %d and %d, want %d twice", first.Code, second.Code, radius.CodeDisconnectACK)
	}
	if got := fake.teardowns.Load(); got != 1 {
		t.Errorf("teardowns after a duplicate: got %d, want 1", got)
	}

	fresh := signCoAPacket(t, encodeCoAPacket(t, radius.CodeDisconnectRequest, 7, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrMessageAuthenticator, Value: make([]byte, radius.AuthenticatorLen)},
	}, time.Now()), secret)
	third := sendRawCoAPacket(t, addr, fresh)

	if third.Identifier != 7 {
		t.Errorf("Identifier: got %d, want 7 (a new Identifier is not a duplicate)", third.Identifier)
	}
	if got := fake.teardowns.Load(); got != 2 {
		t.Errorf("teardowns after a fresh request: got %d, want 2", got)
	}
}

// RFC requirement: RFC5176-2.3-3 positive -- octets past the Length field are padding:
// they are ignored on reception and the request is processed as if they were absent.
// RFC requirement: RFC5176-2.3-3 negative -- a datagram shorter than its own Length
// field is silently discarded rather than read to the end of what arrived.
func TestRFC5176LengthFieldGovernsTheOctetsRead(t *testing.T) {
	secret := []byte("test-rfc5176-length-secret")
	fake := oneSessionService()
	addr := coaTestListener(t, secret, nil, fake)

	wire := buildCoAPacket(t, radius.CodeDisconnectRequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
	}, time.Now())

	padded := append(append([]byte(nil), wire...), make([]byte, 8)...)
	resp := sendRawCoAPacket(t, addr, padded)
	if resp.Code != radius.CodeDisconnectACK {
		t.Errorf("padded request: got code %d, want %d (Disconnect-ACK)", resp.Code, radius.CodeDisconnectACK)
	}

	// The Length field still claims the whole packet, but four octets never arrive.
	truncated := append([]byte(nil), wire[:len(wire)-4]...)
	sendRawCoAPacketExpectNoResponse(t, addr, truncated)
}

// RFC requirement: RFC5176-2.3-4 positive -- the shared secret is chosen by the source
// address of the request, so a client with its own secret is verified against it and
// not against the default.
// RFC requirement: RFC5176-2.3-4 negative -- a request from that source signed with
// the default secret fails verification and is discarded, which proves the choice is
// by address rather than a try-every-secret search.
func TestRFC5176SharedSecretChosenBySourceAddress(t *testing.T) {
	perSource := []byte("test-rfc5176-per-source-secret")
	defaultSecret := []byte("test-rfc5176-default-secret")

	fake := oneSessionService()
	l2tp.PublishService(fake)
	defer l2tp.PublishService(nil)

	cl, err := newCoAListener(coaListenerConfig{
		AllowedSources: coaLoopbackSources(),
		Secrets:        map[string][]byte{"127.0.0.1": perSource},
		DefaultSecret:  defaultSecret,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := cl.Close(); closeErr != nil {
			t.Logf("close: %v", closeErr)
		}
	}()
	addr := cl.conn.LocalAddr().String()

	attrs := []radius.Attr{{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")}}
	resp := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, perSource, attrs)
	if resp.Code != radius.CodeDisconnectACK {
		t.Errorf("per-source secret: got code %d, want %d (Disconnect-ACK)", resp.Code, radius.CodeDisconnectACK)
	}

	sendRawCoAPacketExpectNoResponse(t, addr, buildCoAPacket(t, radius.CodeDisconnectRequest, defaultSecret,
		[]radius.Attr{{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")}}, time.Now()))
}

// RFC requirement: RFC5176-2.3-5 positive -- an attribute the NAS does not support is
// answered with a NAK carrying Error-Cause 401, in a CoA-Request and in a
// Disconnect-Request, and a Disconnect-Request carrying an authorization-change
// attribute tears nothing down.
// RFC requirement: RFC5176-2.3-5 negative -- a request carrying only identification
// attributes is not NAKed for an unsupported attribute; it is honored.
func TestRFC5176UnsupportedAttributeNAKed(t *testing.T) {
	secret := []byte("test-rfc5176-unsupported-secret")
	fake := oneSessionService()
	addr := coaTestListener(t, secret, &recordingBus{}, fake)

	coaResp := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
		{Type: radius.AttrSessionTimeout, Value: radius.AttrUint32(600)},
	})
	wantNAK(t, coaResp, radius.CodeCoANAK, radius.ErrorCauseUnsupportedAttribute)

	// RFC 5176 Section 3: "A Disconnect-Request MUST contain only NAS and session
	// identification attributes."
	dmResp := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
	})
	wantNAK(t, dmResp, radius.CodeDisconnectNAK, radius.ErrorCauseUnsupportedAttribute)
	if got := fake.teardowns.Load(); got != 0 {
		t.Errorf("teardowns for a refused Disconnect-Request: got %d, want 0", got)
	}

	cleanResp := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
	})
	if cleanResp.Code != radius.CodeDisconnectACK {
		t.Errorf("identification-only Disconnect-Request: got code %d, want %d",
			cleanResp.Code, radius.CodeDisconnectACK)
	}
}

// RFC requirement: RFC5176-2.3-6 positive -- a CoA-Request whose authorization change
// cannot be carried out is answered with a CoA-NAK, and no change is made.
// RFC requirement: RFC5176-2.3-6 negative -- the same request against a working route
// is answered with a CoA-ACK and the change is made, so the NAK is specific to the
// failure rather than the shape of the request.
func TestRFC5176ChangeThatCannotBeCarriedOutIsNAKed(t *testing.T) {
	secret := []byte("test-rfc5176-atomic-secret")
	fake := oneSessionService()

	// No event bus: the rate change has no route to the shaper.
	addr := coaTestListener(t, secret, nil, fake)
	attrs := []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
	}
	wantNAK(t, sendCoAPacket(t, addr, radius.CodeCoARequest, secret, attrs),
		radius.CodeCoANAK, radius.ErrorCauseResourcesUnavailable)

	bus := &recordingBus{}
	busAddr := coaTestListener(t, secret, bus, fake)
	resp := sendCoAPacket(t, busAddr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
	})
	if resp.Code != radius.CodeCoAACK {
		t.Errorf("code: got %d, want %d (CoA-ACK)", resp.Code, radius.CodeCoAACK)
	}
	if got := len(bus.recorded()); got != 1 {
		t.Fatalf("events emitted: got %d, want 1", got)
	}
}

// RFC requirement: RFC5176-2.3-7 positive -- identification attributes matching more
// than one session are answered with a NAK carrying Error-Cause 508, and no session is
// torn down, because this NAS does not apply a request to several sessions.
// RFC requirement: RFC5176-2.3-7 negative -- the same attributes matching exactly one
// session are honored, so the 508 is specific to the multiple match.
func TestRFC5176MultipleMatchingSessionsNAKed(t *testing.T) {
	secret := []byte("test-rfc5176-multi-secret")
	fake := &fakeL2TPService{snap: l2tp.Snapshot{
		Tunnels: []l2tp.TunnelSnapshot{{
			LocalTID: 10,
			Sessions: []l2tp.SessionSnapshot{
				{LocalSID: 20, TunnelLocalTID: 10, Username: "alice"},
				{LocalSID: 21, TunnelLocalTID: 10, Username: "alice"},
			},
		}},
	}}
	addr := coaTestListener(t, secret, nil, fake)

	resp := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, []radius.Attr{
		{Type: radius.AttrUserName, Value: radius.AttrString("alice")},
	})
	wantNAK(t, resp, radius.CodeDisconnectNAK, radius.ErrorCauseMultiSessionUnsupported)
	if got := fake.teardowns.Load(); got != 0 {
		t.Errorf("teardowns for a multiple match: got %d, want 0", got)
	}

	single := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, []radius.Attr{
		{Type: radius.AttrUserName, Value: radius.AttrString("alice")},
		{Type: radius.AttrNASPort, Value: radius.AttrUint32(21)},
	})
	if single.Code != radius.CodeDisconnectACK {
		t.Errorf("single match: got code %d, want %d (Disconnect-ACK)", single.Code, radius.CodeDisconnectACK)
	}
	if got := fake.teardowns.Load(); got != 1 {
		t.Errorf("teardowns for a single match: got %d, want 1", got)
	}
}

// RFC requirement: RFC5176-3.1-1 positive -- every Proxy-State attribute of the request
// comes back in the response, unmodified and in the order it arrived.
// RFC requirement: RFC5176-3.1-1 negative -- a request carrying no Proxy-State gets a
// response carrying none, so the listener returns what it received and invents
// nothing.
func TestRFC5176ProxyStateReturnedUnmodified(t *testing.T) {
	secret := []byte("test-rfc5176-proxystate-secret")
	fake := oneSessionService()
	addr := coaTestListener(t, secret, nil, fake)

	first := []byte{0x01, 0x02, 0x03}
	second := []byte{0xfe, 0xed}
	resp := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, []radius.Attr{
		{Type: radius.AttrProxyState, Value: first},
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrProxyState, Value: second},
	})

	got := resp.FindAllAttr(radius.AttrProxyState)
	if len(got) != 2 {
		t.Fatalf("Proxy-State attributes returned: got %d, want 2", len(got))
	}
	if !bytes.Equal(got[0], first) || !bytes.Equal(got[1], second) {
		t.Errorf("Proxy-State values: got %x and %x, want %x and %x", got[0], got[1], first, second)
	}

	bare := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
	})
	if n := len(bare.FindAllAttr(radius.AttrProxyState)); n != 0 {
		t.Errorf("Proxy-State attributes in a response to a request carrying none: got %d, want 0", n)
	}
}

// RFC requirement: RFC5176-3.2-1 positive -- a CoA-Request carrying a Service-Type
// Attribute is answered with a CoA-NAK and Error-Cause 405, and no authorization
// change is made, because this NAS supports no Service-Type value in a CoA-Request.
// RFC requirement: RFC5176-3.2-1 negative -- the same request without the Service-Type
// Attribute is answered with a CoA-ACK, so the refusal is specific to that attribute.
func TestRFC5176ServiceTypeNAKed(t *testing.T) {
	secret := []byte("test-rfc5176-servicetype-secret")
	fake := oneSessionService()
	bus := &recordingBus{}
	addr := coaTestListener(t, secret, bus, fake)

	resp := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
		{Type: radius.AttrServiceType, Value: radius.AttrUint32(serviceTypeAuthorizeOnly)},
	})
	wantNAK(t, resp, radius.CodeCoANAK, radius.ErrorCauseUnsupportedService)
	if got := len(bus.recorded()); got != 0 {
		t.Errorf("events emitted for a refused Service-Type: got %d, want 0", got)
	}

	accepted := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
	})
	if accepted.Code != radius.CodeCoAACK {
		t.Errorf("code without Service-Type: got %d, want %d (CoA-ACK)", accepted.Code, radius.CodeCoAACK)
	}
}

// RFC requirement: RFC5176-3.3-2 positive -- the State Attribute comes back unmodified
// in the response, which is what the listener owes a client that sent one.
// RFC requirement: RFC5176-3.3-2 negative -- a request carrying no State gets a
// response carrying none, so the listener never invents or rewrites the value.
func TestRFC5176StateReturnedUnmodified(t *testing.T) {
	secret := []byte("test-rfc5176-state-secret")
	fake := oneSessionService()
	addr := coaTestListener(t, secret, &recordingBus{}, fake)

	state := []byte{0xde, 0xad, 0xbe, 0xef}
	resp := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
		{Type: radius.AttrState, Value: state},
	})
	got := resp.FindAttr(radius.AttrState)
	if !bytes.Equal(got, state) {
		t.Errorf("State returned: got %x, want %x", got, state)
	}

	bare := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
	})
	if bare.FindAttr(radius.AttrState) != nil {
		t.Error("State returned for a request carrying none")
	}
}

// RFC requirement: RFC5176-3.4-1 positive -- a Message-Authenticator computed with the
// Request Authenticator field and the attribute value each taken as sixteen octets of
// zero is accepted, which is the computation a conformant Dynamic Authorization Client
// performs.
// RFC requirement: RFC5176-3.4-1 negative -- a Message-Authenticator computed over the
// packet with the Request Authenticator left in place is refused, and the packet is
// silently discarded.
func TestRFC5176MessageAuthenticatorZeroesBothFields(t *testing.T) {
	secret := []byte("test-rfc5176-ma-secret")
	fake := oneSessionService()
	addr := coaTestListener(t, secret, nil, fake)

	attrs := func() []radius.Attr {
		return []radius.Attr{
			{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
			{Type: radius.AttrMessageAuthenticator, Value: make([]byte, radius.AuthenticatorLen)},
		}
	}

	conformant := signCoAPacket(t, encodeCoAPacket(t, radius.CodeDisconnectRequest, 3, attrs(), time.Now()), secret)
	resp := sendRawCoAPacket(t, addr, conformant)
	if resp.Code != radius.CodeDisconnectACK {
		t.Errorf("conformant Message-Authenticator: got code %d, want %d",
			resp.Code, radius.CodeDisconnectACK)
	}

	// The computation ze performed before this walk: the Request Authenticator was
	// written first and then hashed as part of the Message-Authenticator, which RFC
	// 5176 Section 3.4 forbids. No conformant client produces these bytes.
	wrong := encodeCoAPacket(t, radius.CodeDisconnectRequest, 4, attrs(), time.Now())
	maOff := messageAuthenticatorOffsetForTest(t, wrong)
	auth := radius.AccountingRequestAuth(wrong, len(wrong), secret)
	copy(wrong[4:4+radius.AuthenticatorLen], auth[:])
	mac := hmac.New(md5.New, secret) //nolint:gosec // reproducing the pre-fix computation.
	mac.Write(wrong)
	copy(wrong[maOff:maOff+radius.AuthenticatorLen], mac.Sum(nil))

	sendRawCoAPacketExpectNoResponse(t, addr, wrong)
}

// RFC requirement: RFC5176-3.5-5 positive -- every NAK this listener emits carries an
// Error-Cause in the 400-599 range, which RFC 5176 Section 3.5 reserves for the fatal
// errors a NAK reports.
// RFC requirement: RFC5176-3.5-5 negative -- no ACK carries an Error-Cause at all, and
// no response carries 201, 202 or 502, which Section 3.5 places elsewhere or forbids a
// NAS to send.
func TestRFC5176ErrorCausePlacement(t *testing.T) {
	secret := []byte("test-rfc5176-cause-secret")
	fake := oneSessionService()
	addr := coaTestListener(t, secret, &recordingBus{}, fake)

	cases := []struct {
		name  string
		code  uint8
		attrs []radius.Attr
		stamp time.Time
	}{
		{
			name: "coa unknown session",
			code: radius.CodeCoARequest,
			attrs: []radius.Attr{
				{Type: radius.AttrAcctSessionID, Value: radius.AttrString("99-99-1")},
				{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
			},
			stamp: time.Now(),
		},
		{
			name: "coa with no authorization change",
			code: radius.CodeCoARequest,
			attrs: []radius.Attr{
				{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
			},
			stamp: time.Now(),
		},
		{
			name: "coa with an unsupported service-type",
			code: radius.CodeCoARequest,
			attrs: []radius.Attr{
				{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
				{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
				{Type: radius.AttrServiceType, Value: radius.AttrUint32(serviceTypeAuthorizeOnly)},
			},
			stamp: time.Now(),
		},
		{
			name: "disconnect with an unsupported attribute",
			code: radius.CodeDisconnectRequest,
			attrs: []radius.Attr{
				{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
				{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
			},
			stamp: time.Now(),
		},
		{
			name: "coa with no event-timestamp",
			code: radius.CodeCoARequest,
			attrs: []radius.Attr{
				{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
				{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
			},
		},
		{
			name: "disconnect that succeeds",
			code: radius.CodeDisconnectRequest,
			attrs: []radius.Attr{
				{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
			},
			stamp: time.Now(),
		},
	}

	sawNAK := false
	sawACK := false
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := sendRawCoAPacket(t, addr, buildCoAPacket(t, tc.code, secret, tc.attrs, tc.stamp))
			cause, present := errorCause(t, resp)

			switch resp.Code {
			case radius.CodeCoANAK, radius.CodeDisconnectNAK:
				sawNAK = true
				if !present {
					t.Fatal("a NAK carries no Error-Cause")
				}
				if cause < 400 || cause > 599 {
					t.Errorf("Error-Cause in a NAK: got %d, want 400-599", cause)
				}
			case radius.CodeCoAACK, radius.CodeDisconnectACK:
				sawACK = true
				if present {
					t.Errorf("an ACK carries Error-Cause %d, want none", cause)
				}
			default:
				t.Fatalf("unexpected response code %d", resp.Code)
			}

			if present && (cause == radius.ErrorCauseResidualSession ||
				cause == radius.ErrorCauseInvalidEAPPacket ||
				cause == 502) {
				t.Errorf("Error-Cause %d is not one a NAS sends", cause)
			}
		})
	}
	if !sawNAK || !sawACK {
		t.Fatalf("the table must cover both replies: NAK seen %t, ACK seen %t", sawNAK, sawACK)
	}
}

// RFC requirement: RFC5176-3.6-1 positive -- identification attributes select the
// session and change nothing: a CoA-Request carrying several of them alongside one
// Filter-Id emits exactly the rate change the Filter-Id names.
// RFC requirement: RFC5176-3.6-1 negative -- a request carrying identification
// attributes and no authorization-change attribute is refused rather than read as a
// change, so no identification attribute is ever taken for one.
func TestRFC5176IdentificationAttributesIdentifyOnly(t *testing.T) {
	secret := []byte("test-rfc5176-identify-secret")
	fake := oneSessionService()
	bus := &recordingBus{}
	addr := coaTestListener(t, secret, bus, fake)

	resp := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrUserName, Value: radius.AttrString("alice")},
		{Type: radius.AttrNASPort, Value: radius.AttrUint32(20)},
		{Type: radius.AttrCalledStationID, Value: radius.AttrString("00:11:22:33:44:55")},
		{Type: radius.AttrNASIdentifier, Value: radius.AttrString("ze-nas")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
	})
	if resp.Code != radius.CodeCoAACK {
		t.Fatalf("code: got %d, want %d (CoA-ACK)", resp.Code, radius.CodeCoAACK)
	}
	events := bus.recorded()
	if len(events) != 1 {
		t.Fatalf("events emitted: got %d, want 1", len(events))
	}
	payload, ok := events[0].payload.(*l2tpevents.SessionRateChangePayload)
	if !ok {
		t.Fatalf("payload type: got %T, want *l2tpevents.SessionRateChangePayload", events[0].payload)
	}
	if payload.SessionID != 20 {
		t.Errorf("session identified: got %d, want 20", payload.SessionID)
	}
	if payload.DownloadRate != 10_000_000 {
		t.Errorf("rate: got %d, want 10000000", payload.DownloadRate)
	}

	bare := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrUserName, Value: radius.AttrString("alice")},
		{Type: radius.AttrNASPort, Value: radius.AttrUint32(20)},
		{Type: radius.AttrCalledStationID, Value: radius.AttrString("00:11:22:33:44:55")},
	})
	wantNAK(t, bare, radius.CodeCoANAK, radius.ErrorCauseUnsupportedAttribute)
	if got := len(bus.recorded()); got != 1 {
		t.Errorf("events after a request carrying only identification attributes: got %d, want 1", got)
	}
}

// RFC requirement: RFC5176-6.1-1 positive -- a request from a source the listener does
// not trust is silently discarded, and a listener trusting nobody discards every
// request, so a failure to resolve a client address never opens the port.
// RFC requirement: RFC5176-6.1-1 negative -- the same request from a trusted source is
// answered, so the discard is specific to the source address.
func TestRFC5176UntrustedSourceDiscarded(t *testing.T) {
	secret := []byte("test-rfc5176-source-secret")
	fake := oneSessionService()
	l2tp.PublishService(fake)
	defer l2tp.PublishService(nil)

	attrs := func() []radius.Attr {
		return []radius.Attr{{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")}}
	}

	elsewhere, err := newCoAListener(coaListenerConfig{AllowedSources: []net.IP{net.IPv4(192, 0, 2, 1)}, DefaultSecret: secret})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := elsewhere.Close(); closeErr != nil {
			t.Logf("close: %v", closeErr)
		}
	}()
	sendRawCoAPacketExpectNoResponse(t, elsewhere.conn.LocalAddr().String(),
		buildCoAPacket(t, radius.CodeDisconnectRequest, secret, attrs(), time.Now()))

	nobody, err := newCoAListener(coaListenerConfig{DefaultSecret: secret})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := nobody.Close(); closeErr != nil {
			t.Logf("close: %v", closeErr)
		}
	}()
	sendRawCoAPacketExpectNoResponse(t, nobody.conn.LocalAddr().String(),
		buildCoAPacket(t, radius.CodeDisconnectRequest, secret, attrs(), time.Now()))

	if got := fake.teardowns.Load(); got != 0 {
		t.Errorf("teardowns from untrusted sources: got %d, want 0", got)
	}

	trusted, err := newCoAListener(coaListenerConfig{AllowedSources: coaLoopbackSources(), DefaultSecret: secret})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := trusted.Close(); closeErr != nil {
			t.Logf("close: %v", closeErr)
		}
	}()
	resp := sendCoAPacket(t, trusted.conn.LocalAddr().String(), radius.CodeDisconnectRequest, secret, attrs())
	if resp.Code != radius.CodeDisconnectACK {
		t.Errorf("trusted source: got code %d, want %d (Disconnect-ACK)", resp.Code, radius.CodeDisconnectACK)
	}
}

// RFC requirement: RFC5176-6.3-1 positive -- an Event-Timestamp outside the duplicate
// detection window is not current, so the packet is silently discarded and no session
// is torn down. A NAK would answer a replayed packet and confirm the secret to whoever
// replayed it.
// RFC requirement: RFC5176-6.3-1 negative -- a current Event-Timestamp is answered, so
// the silent discard is specific to the stale one.
func TestRFC5176StaleEventTimestampDiscarded(t *testing.T) {
	secret := []byte("test-rfc5176-timestamp-secret")
	fake := oneSessionService()
	addr := coaTestListener(t, secret, nil, fake)

	attrs := func() []radius.Attr {
		return []radius.Attr{{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")}}
	}

	stale := time.Now().Add(-2 * coaReplayWindow)
	sendRawCoAPacketExpectNoResponse(t, addr,
		buildCoAPacket(t, radius.CodeDisconnectRequest, secret, attrs(), stale))
	if got := fake.teardowns.Load(); got != 0 {
		t.Errorf("teardowns for a stale Event-Timestamp: got %d, want 0", got)
	}

	resp := sendRawCoAPacket(t, addr,
		buildCoAPacket(t, radius.CodeDisconnectRequest, secret, attrs(), time.Now()))
	if resp.Code != radius.CodeDisconnectACK {
		t.Errorf("current Event-Timestamp: got code %d, want %d", resp.Code, radius.CodeDisconnectACK)
	}
}
