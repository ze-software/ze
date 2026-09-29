// RFC: rfc/short/rfc5176.md -- the CoA-Request and NAK branches the Disconnect tests leave open
// Related: coa.go -- handleCoA, handleDisconnect, oneSession, sendResponse
// Related: rfc5176_walk_test.go -- coaTestListener, recordingBus, wantNAK

// The RFC 5176 Section 2.3 and Section 3 sentences each name a CoA-Request and
// a Disconnect-Request, or an ACK and a NAK. These tests drive the halves the
// older units did not reach, through the real listener socket, and observe the
// change through the event bus or the session service each time.
package l2tpauthradius

import (
	"bytes"
	"crypto/md5" //nolint:gosec // RFC 5176 Section 2.3 mandates MD5 for the Response Authenticator
	"encoding/binary"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/radius"
)

// twoAliceSessions answers a service holding two sessions of user alice:
// tunnel 10, sessions 20 and 21.
func twoAliceSessions() *fakeL2TPService {
	return &fakeL2TPService{snap: l2tp.Snapshot{
		Tunnels: []l2tp.TunnelSnapshot{{
			LocalTID: 10,
			Sessions: []l2tp.SessionSnapshot{
				{LocalSID: 20, TunnelLocalTID: 10, Username: "alice"},
				{LocalSID: 21, TunnelLocalTID: 10, Username: "alice"},
			},
		}},
	}}
}

// teardownRefusingService is the one-session service whose teardown fails, so
// a Disconnect-Request matches a session that cannot be terminated.
type teardownRefusingService struct {
	*fakeL2TPService
	attempts atomic.Int32
}

func (s *teardownRefusingService) TeardownSession(uint16) error {
	s.attempts.Add(1)
	return errors.New("teardown refused by the test")
}

// coaRawExchange sends one encoded request to the listener and answers the raw
// response octets, so a test can read the Authenticator field as sent.
func coaRawExchange(t *testing.T, addr string, wire []byte) []byte {
	t.Helper()
	conn, err := net.DialUDP("udp4", nil, mustResolveUDP(t, addr))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			t.Logf("conn close: %v", closeErr)
		}
	}()
	if _, err := conn.Write(wire); err != nil {
		t.Fatal(err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, radius.MaxPacketLen)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatal("no response:", err)
	}
	return append([]byte{}, buf[:n]...)
}

// rateChange is the CoA attribute set that asks for a 10 Mbit/s rate on the
// session the identification attributes name.
func rateChange(identification ...radius.Attr) []radius.Attr {
	return append(identification, radius.Attr{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")})
}

// RFC requirement: RFC5176-2.3-7 positive -- a CoA-Request whose User-Name matches
// two sessions is answered with a CoA-NAK carrying Error-Cause 508, and no rate
// change reaches the event bus.
func TestRFC5176CoAMatchingSeveralSessionsIsNAKed(t *testing.T) {
	secret := []byte("test-rfc5176-coa-multi-secret")
	bus := &recordingBus{}
	addr := coaTestListener(t, secret, bus, twoAliceSessions())

	resp := sendCoAPacket(t, addr, radius.CodeCoARequest, secret,
		rateChange(radius.Attr{Type: radius.AttrUserName, Value: radius.AttrString("alice")}))
	wantNAK(t, resp, radius.CodeCoANAK, radius.ErrorCauseMultiSessionUnsupported)
	if got := len(bus.recorded()); got != 0 {
		t.Errorf("events emitted for a multiple match: got %d, want 0", got)
	}
}

// RFC requirement: RFC5176-2.3-7 negative -- the same CoA-Request narrowed by
// NAS-Port to one of the two sessions is answered with a CoA-ACK and one rate
// change, so the NAK is specific to the multiple match.
func TestRFC5176CoAMatchingOneSessionIsACKed(t *testing.T) {
	secret := []byte("test-rfc5176-coa-single-secret")
	bus := &recordingBus{}
	addr := coaTestListener(t, secret, bus, twoAliceSessions())

	resp := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, rateChange(
		radius.Attr{Type: radius.AttrUserName, Value: radius.AttrString("alice")},
		radius.Attr{Type: radius.AttrNASPort, Value: radius.AttrUint32(21)},
	))
	if resp.Code != radius.CodeCoAACK {
		t.Fatalf("code: got %d, want %d (CoA-ACK)", resp.Code, radius.CodeCoAACK)
	}
	if got := len(bus.recorded()); got != 1 {
		t.Errorf("events emitted for a single match: got %d, want 1", got)
	}
}

// RFC requirement: RFC5176-3.3-1 positive -- a CoA-Request and a
// Disconnect-Request whose Acct-Session-Id matches no session are answered with
// a CoA-NAK and a Disconnect-NAK carrying Error-Cause 503, with no rate change
// on the bus and no teardown.
func TestRFC5176UnmatchedIdentificationIsNAKed(t *testing.T) {
	secret := []byte("test-rfc5176-nomatch-secret")
	bus := &recordingBus{}
	fake := oneSessionService()
	addr := coaTestListener(t, secret, bus, fake)
	unmatched := radius.Attr{Type: radius.AttrAcctSessionID, Value: radius.AttrString("99-99-1")}

	wantNAK(t, sendCoAPacket(t, addr, radius.CodeCoARequest, secret, rateChange(unmatched)),
		radius.CodeCoANAK, radius.ErrorCauseSessionNotFound)
	wantNAK(t, sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, []radius.Attr{unmatched}),
		radius.CodeDisconnectNAK, radius.ErrorCauseSessionNotFound)
	if got := len(bus.recorded()); got != 0 {
		t.Errorf("events emitted with no matching session: got %d, want 0", got)
	}
	if got := fake.teardowns.Load(); got != 0 {
		t.Errorf("teardowns with no matching session: got %d, want 0", got)
	}
}

// RFC requirement: RFC5176-3.3-1 negative -- the same two requests with an
// Acct-Session-Id that matches the one session succeed: a CoA-ACK with one rate
// change, then a Disconnect-ACK with one teardown.
func TestRFC5176MatchedIdentificationSucceeds(t *testing.T) {
	secret := []byte("test-rfc5176-match-secret")
	bus := &recordingBus{}
	fake := oneSessionService()
	addr := coaTestListener(t, secret, bus, fake)
	matched := radius.Attr{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")}

	if resp := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, rateChange(matched)); resp.Code != radius.CodeCoAACK {
		t.Fatalf("CoA code: got %d, want %d (CoA-ACK)", resp.Code, radius.CodeCoAACK)
	}
	if got := len(bus.recorded()); got != 1 {
		t.Errorf("events emitted: got %d, want 1", got)
	}
	if resp := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, []radius.Attr{matched}); resp.Code != radius.CodeDisconnectACK {
		t.Fatalf("Disconnect code: got %d, want %d (Disconnect-ACK)", resp.Code, radius.CodeDisconnectACK)
	}
	if got := fake.teardowns.Load(); got != 1 {
		t.Errorf("teardowns: got %d, want 1", got)
	}
}

// RFC requirement: RFC5176-3.3-2 positive -- a CoA-Request carrying State that is
// answered with a NAK, once because the change cannot be carried out (506) and
// once because no session matches (503), gets its State back byte for byte in
// each CoA-NAK.
func TestRFC5176StateReturnedUnmodifiedInANAK(t *testing.T) {
	secret := []byte("test-rfc5176-state-nak-secret")
	addr := coaTestListener(t, secret, nil, oneSessionService())
	state := []byte{0xde, 0xad, 0x00, 0xbe, 0xef}

	for _, session := range []string{"10-20-1", "99-99-1"} {
		resp := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, rateChange(
			radius.Attr{Type: radius.AttrAcctSessionID, Value: radius.AttrString(session)},
			radius.Attr{Type: radius.AttrState, Value: state},
		))
		if resp.Code != radius.CodeCoANAK {
			t.Fatalf("session %s: code %d, want %d (CoA-NAK)", session, resp.Code, radius.CodeCoANAK)
		}
		if got := resp.FindAttr(radius.AttrState); !bytes.Equal(got, state) {
			t.Errorf("session %s: State in the NAK: got %x, want %x", session, got, state)
		}
	}
}

// RFC requirement: RFC5176-3.3-2 negative -- a CoA-Request carrying no State
// that is answered with a CoA-NAK gets a NAK carrying no State.
func TestRFC5176NAKForARequestWithoutStateCarriesNone(t *testing.T) {
	secret := []byte("test-rfc5176-nostate-nak-secret")
	addr := coaTestListener(t, secret, nil, oneSessionService())

	resp := sendCoAPacket(t, addr, radius.CodeCoARequest, secret,
		rateChange(radius.Attr{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")}))
	if resp.Code != radius.CodeCoANAK {
		t.Fatalf("code: got %d, want %d (CoA-NAK)", resp.Code, radius.CodeCoANAK)
	}
	if got := resp.FindAttr(radius.AttrState); got != nil {
		t.Errorf("State in the NAK of a request carrying none: %x", got)
	}
}

// RFC requirement: RFC5176-3.5-3 positive -- for a CoA-ACK, a CoA-NAK, a
// Disconnect-ACK and a Disconnect-NAK read off the socket, the Authenticator
// field equals an MD5 the test computes itself over the Code, Identifier and
// Length of the response, the Request Authenticator of the request it answers,
// the response attributes and the shared secret.
func TestRFC5176ResponseAuthenticatorMatchesAnIndependentMD5(t *testing.T) {
	secret := []byte("test-rfc5176-response-auth-secret")
	addr := coaTestListener(t, secret, &recordingBus{}, oneSessionService())
	matched := radius.Attr{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")}
	unmatched := radius.Attr{Type: radius.AttrAcctSessionID, Value: radius.AttrString("99-99-1")}

	cases := []struct {
		name  string
		code  uint8
		attrs []radius.Attr
		want  uint8
	}{
		{"CoA-ACK", radius.CodeCoARequest, rateChange(matched), radius.CodeCoAACK},
		{"CoA-NAK", radius.CodeCoARequest, rateChange(unmatched), radius.CodeCoANAK},
		{"Disconnect-NAK", radius.CodeDisconnectRequest, []radius.Attr{unmatched}, radius.CodeDisconnectNAK},
		{"Disconnect-ACK", radius.CodeDisconnectRequest, []radius.Attr{matched}, radius.CodeDisconnectACK},
	}
	for _, tc := range cases {
		request := buildCoAPacket(t, tc.code, secret, tc.attrs, time.Now())
		resp := coaRawExchange(t, addr, request)
		if resp[0] != tc.want {
			t.Fatalf("%s: code %d, want %d", tc.name, resp[0], tc.want)
		}
		length := binary.BigEndian.Uint16(resp[2:4])
		h := md5.New() //nolint:gosec // RFC 5176 Section 2.3 mandates MD5
		h.Write(resp[:4])
		h.Write(request[4 : 4+radius.AuthenticatorLen])
		h.Write(resp[radius.HeaderLen:length])
		h.Write(secret)
		if want := h.Sum(nil); !bytes.Equal(resp[4:4+radius.AuthenticatorLen], want) {
			t.Errorf("%s: Response Authenticator %x, want %x", tc.name, resp[4:4+radius.AuthenticatorLen], want)
		}
	}
}

// RFC requirement: RFC5176-2.3-6 positive -- a CoA-Request asking for a rate
// change and a CoS profile on an L2TP session with no access interface cannot be
// carried out whole: it is answered with a CoA-NAK carrying Error-Cause 506, and
// the rate change it could have made alone does not reach the event bus.
func TestRFC5176CoANAKMakesNoPartialChange(t *testing.T) {
	secret := []byte("test-rfc5176-partial-secret")
	bus := &recordingBus{}
	addr := coaTestListener(t, secret, bus, oneSessionService())

	resp := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("cos:gold")},
	})
	wantNAK(t, resp, radius.CodeCoANAK, radius.ErrorCauseResourcesUnavailable)
	if got := len(bus.recorded()); got != 0 {
		t.Errorf("events emitted behind a CoA-NAK: got %d, want 0", got)
	}
}

// RFC requirement: RFC5176-2.3-6 positive -- a Disconnect-Request matching one
// session whose termination fails is answered with a Disconnect-NAK after one
// termination attempt.
func TestRFC5176DisconnectThatCannotTerminateIsNAKed(t *testing.T) {
	secret := []byte("test-rfc5176-noterm-secret")
	svc := &teardownRefusingService{fakeL2TPService: oneSessionService()}
	l2tp.PublishService(svc)
	t.Cleanup(func() { l2tp.PublishService(nil) })
	addr := coaTestListener(t, secret, nil, nil)

	resp := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
	})
	if resp.Code != radius.CodeDisconnectNAK {
		t.Errorf("code: got %d, want %d (Disconnect-NAK)", resp.Code, radius.CodeDisconnectNAK)
	}
	if got := svc.attempts.Load(); got != 1 {
		t.Errorf("termination attempts: got %d, want 1", got)
	}
}

// RFC requirement: RFC5176-2.3-6 negative -- the same Disconnect-Request against
// a session whose termination succeeds is answered with a Disconnect-ACK after
// one teardown, so the NAK is specific to the failed termination.
func TestRFC5176DisconnectThatTerminatesIsACKed(t *testing.T) {
	secret := []byte("test-rfc5176-term-secret")
	fake := oneSessionService()
	addr := coaTestListener(t, secret, nil, fake)

	resp := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
	})
	if resp.Code != radius.CodeDisconnectACK {
		t.Errorf("code: got %d, want %d (Disconnect-ACK)", resp.Code, radius.CodeDisconnectACK)
	}
	if got := fake.teardowns.Load(); got != 1 {
		t.Errorf("teardowns: got %d, want 1", got)
	}
}

// RFC requirement: RFC5176-2.3-5 positive -- a CoA-Request whose only
// authorization attribute is a supported attribute carrying a value the NAS does
// not support (a Filter-Id that is neither a rate nor a CoS profile, and a CoS
// Filter-Id with an empty name) is answered with a CoA-NAK, and no change
// reaches the event bus.
func TestRFC5176CoAWithAnUnsupportedAttributeValueIsNAKed(t *testing.T) {
	secret := []byte("test-rfc5176-badvalue-secret")
	bus := &recordingBus{}
	addr := coaTestListener(t, secret, bus, oneSessionService())

	for _, value := range []string{"not-a-rate", "cos:"} {
		resp := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
			{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
			{Type: radius.AttrFilterID, Value: radius.AttrString(value)},
		})
		if resp.Code != radius.CodeCoANAK {
			t.Errorf("Filter-Id %q: code %d, want %d (CoA-NAK)", value, resp.Code, radius.CodeCoANAK)
		}
	}
	if got := len(bus.recorded()); got != 0 {
		t.Errorf("events emitted for an unsupported value: got %d, want 0", got)
	}
}
