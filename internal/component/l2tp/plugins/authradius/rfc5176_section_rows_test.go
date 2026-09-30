// Design: docs/guide/l2tp.md -- CoA/DM listener
//
// RFC 5176 rows split out of the sign-off walk (rfc/extraction/rfc5176.json):
// Sections 2.2, 3, 3.3, 3.5, 3.6 (Note 7) and 6.3. Every test drives the
// Dynamic Authorization Server over a real UDP socket, the entry point a
// Dynamic Authorization Client reaches, and reads the response it sends.
//
// VALIDATES: the Service-Type NAK, the Disconnect-Request attribute limit, State
// left uninterpreted, Error-Cause placement for 201, 202, 502 and 504, the
// Vendor-Specific single purpose, and one window for duplicates and timestamps.
// PREVENTS: a response path sending an Error-Cause Section 3.5 forbids, and a
// request attribute read for a purpose RFC 5176 does not give it.

package l2tpauthradius

import (
	"bytes"
	"encoding/binary"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/radius"
)

// coaPathResponse is one response the listener sent on one of its paths.
type coaPathResponse struct {
	name   string
	code   uint8
	causes []uint32
}

// errorCauses answers every Error-Cause value a response carries, in order.
func errorCauses(t *testing.T, resp *radius.Packet) []uint32 {
	t.Helper()
	var out []uint32
	for i := range resp.Attrs {
		if resp.Attrs[i].Type != radius.AttrErrorCause {
			continue
		}
		if len(resp.Attrs[i].Value) != 4 {
			t.Fatalf("Error-Cause length: got %d, want 4", len(resp.Attrs[i].Value))
		}
		out = append(out, binary.BigEndian.Uint32(resp.Attrs[i].Value))
	}
	return out
}

// twoSessionService answers a service holding two sessions of user alice:
// tunnel 10, sessions 20 and 21.
func twoSessionService() *fakeL2TPService {
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

// coaResponsePaths drives every response the listener can send, each through
// its own listener and service, and fails unless each path answers the code and
// the Error-Cause the listener's documented table names (docs/guide/l2tp.md).
func coaResponsePaths(t *testing.T) []coaPathResponse {
	t.Helper()
	secret := []byte("test-rfc5176-paths-secret")
	session := radius.Attr{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")}
	rate := radius.Attr{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")}
	unknown := radius.Attr{Type: radius.AttrAcctSessionID, Value: radius.AttrString("99-99-1")}
	alice := radius.Attr{Type: radius.AttrUserName, Value: radius.AttrString("alice")}

	cases := []struct {
		name      string
		service   l2tp.Service
		noBus     bool
		code      uint8
		attrs     []radius.Attr
		noStamp   bool
		wantCode  uint8
		wantCause uint32
	}{
		{"coa accepted", oneSessionService(), false, radius.CodeCoARequest, []radius.Attr{session, rate}, false, radius.CodeCoAACK, 0},
		{"disconnect accepted", oneSessionService(), false, radius.CodeDisconnectRequest, []radius.Attr{session}, false, radius.CodeDisconnectACK, 0},
		{"coa asking no change", oneSessionService(), false, radius.CodeCoARequest, []radius.Attr{session}, false, radius.CodeCoANAK, radius.ErrorCauseUnsupportedAttribute},
		{"disconnect with filter-id", oneSessionService(), false, radius.CodeDisconnectRequest, []radius.Attr{session, rate}, false, radius.CodeDisconnectNAK, radius.ErrorCauseUnsupportedAttribute},
		{"coa with service-type", oneSessionService(), false, radius.CodeCoARequest, []radius.Attr{session, rate, {Type: radius.AttrServiceType, Value: radius.AttrUint32(2)}}, false, radius.CodeCoANAK, radius.ErrorCauseUnsupportedService},
		{"coa with an unknown filter-id", oneSessionService(), false, radius.CodeCoARequest, []radius.Attr{session, {Type: radius.AttrFilterID, Value: radius.AttrString("no-such-profile")}}, false, radius.CodeCoANAK, radius.ErrorCauseInvalidAttributeValue},
		{"disconnect with a short nas-port", oneSessionService(), false, radius.CodeDisconnectRequest, []radius.Attr{session, {Type: radius.AttrNASPort, Value: []byte{0, 20}}}, false, radius.CodeDisconnectNAK, radius.ErrorCauseInvalidAttributeValue},
		{"coa for an unknown session", oneSessionService(), false, radius.CodeCoARequest, []radius.Attr{unknown, rate}, false, radius.CodeCoANAK, radius.ErrorCauseSessionNotFound},
		{"disconnect for an unknown session", oneSessionService(), false, radius.CodeDisconnectRequest, []radius.Attr{unknown}, false, radius.CodeDisconnectNAK, radius.ErrorCauseSessionNotFound},
		{"coa matching two sessions", twoSessionService(), false, radius.CodeCoARequest, []radius.Attr{alice, rate}, false, radius.CodeCoANAK, radius.ErrorCauseMultiSessionUnsupported},
		{"disconnect matching two sessions", twoSessionService(), false, radius.CodeDisconnectRequest, []radius.Attr{alice}, false, radius.CodeDisconnectNAK, radius.ErrorCauseMultiSessionUnsupported},
		{"coa that cannot be carried out", oneSessionService(), true, radius.CodeCoARequest, []radius.Attr{session, rate}, false, radius.CodeCoANAK, radius.ErrorCauseResourcesUnavailable},
		{"disconnect whose teardown fails", &teardownRefusingService{fakeL2TPService: oneSessionService()}, false, radius.CodeDisconnectRequest, []radius.Attr{session}, false, radius.CodeDisconnectNAK, radius.ErrorCauseSessionNotRemovable},
		{"coa with no event-timestamp", oneSessionService(), false, radius.CodeCoARequest, []radius.Attr{session, rate}, true, radius.CodeCoANAK, radius.ErrorCauseInvalidRequest},
		{"disconnect with no event-timestamp", oneSessionService(), false, radius.CodeDisconnectRequest, []radius.Attr{session}, true, radius.CodeDisconnectNAK, radius.ErrorCauseInvalidRequest},
	}

	var out []coaPathResponse
	for _, tc := range cases {
		l2tp.PublishService(tc.service)
		var addr string
		if tc.noBus {
			addr = coaTestListener(t, secret, nil, nil)
		} else {
			addr = coaTestListener(t, secret, &recordingBus{}, nil)
		}
		stamp := time.Now()
		if tc.noStamp {
			stamp = time.Time{}
		}
		resp := sendRawCoAPacket(t, addr, buildCoAPacket(t, tc.code, secret, tc.attrs, stamp))
		l2tp.PublishService(nil)

		causes := errorCauses(t, resp)
		if resp.Code != tc.wantCode {
			t.Fatalf("%s: code %d, want %d", tc.name, resp.Code, tc.wantCode)
		}
		want := []uint32(nil)
		if tc.wantCause != 0 {
			want = []uint32{tc.wantCause}
		}
		if !slices.Equal(causes, want) {
			t.Fatalf("%s: Error-Cause %v, want %v", tc.name, causes, want)
		}
		out = append(out, coaPathResponse{name: tc.name, code: resp.Code, causes: causes})
	}
	return out
}

// wantCauseOnlyIn fails when cause appears in a response whose code is not in
// allowed, and fails unless the paths cover all four response codes.
func wantCauseOnlyIn(t *testing.T, paths []coaPathResponse, cause uint32, allowed ...uint8) {
	t.Helper()
	seen := map[uint8]bool{}
	for _, p := range paths {
		seen[p.code] = true
		if slices.Contains(p.causes, cause) && !slices.Contains(allowed, p.code) {
			t.Errorf("%s: code %d carries Error-Cause %d", p.name, p.code, cause)
		}
	}
	for _, code := range []uint8{radius.CodeCoAACK, radius.CodeCoANAK, radius.CodeDisconnectACK, radius.CodeDisconnectNAK} {
		if !seen[code] {
			t.Errorf("the paths never produced response code %d", code)
		}
	}
}

// wantForbiddenCauseNeverEchoed sends a CoA-Request and a Discon-Request that
// each carry the forbidden Error-Cause, and a Proxy-State whose octets are that
// Error-Cause attribute, and fails unless each is refused with the single
// Error-Cause 401 and the Proxy-State comes back whole.
func wantForbiddenCauseNeverEchoed(t *testing.T, cause uint32) {
	t.Helper()
	secret := []byte("test-rfc5176-forbidden-secret")
	fake := oneSessionService()
	addr := coaTestListener(t, secret, &recordingBus{}, fake)

	var value [4]byte
	binary.BigEndian.PutUint32(value[:], cause)
	tlv := append([]byte{radius.AttrErrorCause, 6}, value[:]...)
	for _, code := range []uint8{radius.CodeCoARequest, radius.CodeDisconnectRequest} {
		resp := sendCoAPacket(t, addr, code, secret, []radius.Attr{
			{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
			{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
			{Type: radius.AttrErrorCause, Value: value[:]},
			{Type: radius.AttrProxyState, Value: tlv},
		})
		if resp.Code != nakCode(code) {
			t.Errorf("request code %d carrying Error-Cause %d: response %d, want %d", code, cause, resp.Code, nakCode(code))
		}
		if got := errorCauses(t, resp); !slices.Equal(got, []uint32{radius.ErrorCauseUnsupportedAttribute}) {
			t.Errorf("request code %d carrying Error-Cause %d: response Error-Cause %v, want [401]", code, cause, got)
		}
		if got := resp.FindAttr(radius.AttrProxyState); !bytes.Equal(got, tlv) {
			t.Errorf("Proxy-State: got %x, want %x", got, tlv)
		}
	}
	if got := fake.teardowns.Load(); got != 0 {
		t.Errorf("teardowns for refused requests: got %d, want 0", got)
	}
}

// RFC requirement: RFC5176-2.2-1 positive -- a CoA-Request carrying a Service-Type
// of Login, Framed, Administrative or Authenticate Only, none of which this NAS
// supports in a CoA-Request, is answered with a CoA-NAK carrying Error-Cause 405
// (Unsupported Service), and no authorization change is made.
func TestRFC5176UnsupportedServiceTypeValueIsNAKed(t *testing.T) {
	secret := []byte("test-rfc5176-2.2-secret")
	bus := &recordingBus{}
	addr := coaTestListener(t, secret, bus, oneSessionService())

	for _, value := range []uint32{1, 2, 6, 8} {
		resp := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
			{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
			{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
			{Type: radius.AttrServiceType, Value: radius.AttrUint32(value)},
		})
		if resp.Code != radius.CodeCoANAK {
			t.Errorf("Service-Type %d: code %d, want %d (CoA-NAK)", value, resp.Code, radius.CodeCoANAK)
		}
		if got := errorCauses(t, resp); !slices.Equal(got, []uint32{radius.ErrorCauseUnsupportedService}) {
			t.Errorf("Service-Type %d: Error-Cause %v, want [405]", value, got)
		}
	}
	if got := len(bus.recorded()); got != 0 {
		t.Errorf("changes made behind a CoA-NAK: got %d, want 0", got)
	}
}

// RFC requirement: RFC5176-2.2-1 negative -- the same CoA-Request without a
// Service-Type is answered with a CoA-ACK carrying no Error-Cause and makes the
// one rate change it names, so the CoA-NAK is owed to the Service-Type value.
func TestRFC5176CoAWithoutServiceTypeIsACKed(t *testing.T) {
	secret := []byte("test-rfc5176-2.2-neg-secret")
	bus := &recordingBus{}
	addr := coaTestListener(t, secret, bus, oneSessionService())

	resp := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
	})
	if resp.Code != radius.CodeCoAACK {
		t.Fatalf("code: got %d, want %d (CoA-ACK)", resp.Code, radius.CodeCoAACK)
	}
	if got := errorCauses(t, resp); len(got) != 0 {
		t.Errorf("CoA-ACK Error-Cause: got %v, want none", got)
	}
	if got := len(bus.recorded()); got != 1 {
		t.Errorf("changes made: got %d, want 1", got)
	}
}

// RFC requirement: RFC5176-3-1 positive -- a Disconnect-Request that carries,
// beside its session identification, an attribute that is neither NAS nor
// session identification (Filter-Id, Framed-MTU, Session-Timeout, Idle-Timeout,
// Service-Type, State) is answered with a Disconnect-NAK carrying Error-Cause
// 401 (Unsupported Attribute), and the session is not torn down.
// Vendor-Specific is not in the list: Section 3 names it a session
// identification attribute, and Section 3.6 Note 7 lets a VSA identify a
// session in a Disconnect-Request, so its refusal is RFC5176-3.6-2's.
// EAP-Message is not in the list either: the Section 3.6 Disconnect table
// admits it in a Request (0+, Note 2), so it is not an "other attribute"; this
// NAS refuses it only because it offers no EAP service (Section 2.3).
func TestRFC5176DisconnectWithANonIdentificationAttributeIsNAKed(t *testing.T) {
	secret := []byte("test-rfc5176-3-secret")
	fake := oneSessionService()
	addr := coaTestListener(t, secret, &recordingBus{}, fake)

	others := []struct {
		name string
		attr radius.Attr
	}{
		{"Filter-Id", radius.Attr{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")}},
		{"Framed-MTU", radius.Attr{Type: 12, Value: radius.AttrUint32(1492)}},
		{"Session-Timeout", radius.Attr{Type: 27, Value: radius.AttrUint32(3600)}},
		{"Idle-Timeout", radius.Attr{Type: 28, Value: radius.AttrUint32(600)}},
		{"Service-Type", radius.Attr{Type: radius.AttrServiceType, Value: radius.AttrUint32(2)}},
		{"State", radius.Attr{Type: radius.AttrState, Value: []byte{0xde, 0xad}}},
	}
	for _, other := range others {
		resp := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, []radius.Attr{
			{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
			{Type: radius.AttrUserName, Value: radius.AttrString("alice")},
			other.attr,
		})
		if resp.Code != radius.CodeDisconnectNAK {
			t.Errorf("%s: code %d, want %d (Disconnect-NAK)", other.name, resp.Code, radius.CodeDisconnectNAK)
		}
		if got := errorCauses(t, resp); !slices.Equal(got, []uint32{radius.ErrorCauseUnsupportedAttribute}) {
			t.Errorf("%s: Error-Cause %v, want [401]", other.name, got)
		}
	}
	if got := fake.teardowns.Load(); got != 0 {
		t.Errorf("teardowns behind a Disconnect-NAK: got %d, want 0", got)
	}
}

// RFC requirement: RFC5176-3-1 negative -- a Disconnect-Request carrying only NAS
// identification (NAS-IP-Address, NAS-Identifier) and session identification
// (Acct-Session-Id, User-Name, NAS-Port, Called-Station-Id, Calling-Station-Id)
// attributes is answered with a Disconnect-ACK and one teardown, so the
// Disconnect-NAK is owed to the other attribute alone.
func TestRFC5176DisconnectCarryingOnlyIdentificationIsACKed(t *testing.T) {
	secret := []byte("test-rfc5176-3-neg-secret")
	fake := oneSessionService()
	addr := coaTestListener(t, secret, &recordingBus{}, fake)

	resp := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, []radius.Attr{
		{Type: radius.AttrNASIPAddress, Value: []byte{127, 0, 0, 1}},
		{Type: radius.AttrNASIdentifier, Value: radius.AttrString("ze-nas")},
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrUserName, Value: radius.AttrString("alice")},
		{Type: radius.AttrNASPort, Value: radius.AttrUint32(20)},
		{Type: radius.AttrCalledStationID, Value: radius.AttrString("00:11:22:33:44:55")},
		{Type: radius.AttrCallingStationID, Value: radius.AttrString("66:77:88:99:aa:bb")},
	})
	if resp.Code != radius.CodeDisconnectACK {
		t.Fatalf("code: got %d, want %d (Disconnect-ACK)", resp.Code, radius.CodeDisconnectACK)
	}
	if got := fake.teardowns.Load(); got != 1 {
		t.Errorf("teardowns: got %d, want 1", got)
	}
}

// RFC requirement: RFC5176-3-1 negative -- the Section 3.6 Disconnect table
// admits Reply-Message (0+), Class (0+) and Acct-Terminate-Cause (0-1) in a
// Disconnect-Request, so none of them is an "other attribute": a
// Disconnect-Request carrying one of them, or all three, beside its session
// identification is answered with a Disconnect-ACK carrying no Error-Cause, and
// the session is torn down each time.
//
// Method: one listener, one fresh request per case; the teardown count rises by
// one for every ACK.
func TestRFC5176DisconnectAdmitsTheSection36TableAttributes(t *testing.T) {
	secret := []byte("test-rfc5176-3-table-secret")
	fake := oneSessionService()
	addr := coaTestListener(t, secret, &recordingBus{}, fake)

	replyMessage := radius.Attr{Type: radius.AttrReplyMessage, Value: radius.AttrString("session ended by operator")}
	class := radius.Attr{Type: radius.AttrClass, Value: []byte{0x01, 0x02, 0x03}}
	terminateCause := radius.Attr{Type: radius.AttrAcctTerminateCause, Value: radius.AttrUint32(6)}
	cases := []struct {
		name  string
		attrs []radius.Attr
	}{
		{"Reply-Message", []radius.Attr{replyMessage}},
		{"Class", []radius.Attr{class}},
		{"Acct-Terminate-Cause", []radius.Attr{terminateCause}},
		{"all three", []radius.Attr{replyMessage, class, terminateCause}},
	}
	for i, tc := range cases {
		attrs := append([]radius.Attr{
			{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
			{Type: radius.AttrUserName, Value: radius.AttrString("alice")},
		}, tc.attrs...)
		resp := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, attrs)
		if resp.Code != radius.CodeDisconnectACK {
			t.Errorf("%s: code %d, want %d (Disconnect-ACK)", tc.name, resp.Code, radius.CodeDisconnectACK)
		}
		if got := errorCauses(t, resp); len(got) != 0 {
			t.Errorf("%s: Error-Cause %v, want none", tc.name, got)
		}
		if got := fake.teardowns.Load(); got != int32(i+1) {
			t.Errorf("%s: teardowns %d, want %d", tc.name, got, i+1)
		}
	}
}

// RFC requirement: RFC5176-3.3-3 positive -- the same CoA-Request sent with no
// State and with State values shaped as a rate, as another session's
// Acct-Session-Id and as a Filter-Id attribute gets the same outcome every time:
// a CoA-ACK and one rate change of 10 Mbit/s on session 20, with the State
// returned octet for octet. The listener reads nothing out of the State.
func TestRFC5176StateNeverChangesTheOutcome(t *testing.T) {
	secret := []byte("test-rfc5176-3.3-secret")
	states := []struct {
		name  string
		state []byte
	}{
		{"no State", nil},
		{"State shaped as a rate", []byte("1mbit")},
		{"State shaped as another session", []byte("10-21-1")},
		{"State shaped as a Filter-Id attribute", []byte{radius.AttrFilterID, 7, '1', 'm', 'b', 'i', 't'}},
	}
	for _, tc := range states {
		t.Run(tc.name, func(t *testing.T) {
			bus := &recordingBus{}
			addr := coaTestListener(t, secret, bus, oneSessionService())
			attrs := []radius.Attr{
				{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
				{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
			}
			if tc.state != nil {
				attrs = append(attrs, radius.Attr{Type: radius.AttrState, Value: tc.state})
			}
			resp := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, attrs)
			if resp.Code != radius.CodeCoAACK {
				t.Fatalf("code: got %d, want %d (CoA-ACK)", resp.Code, radius.CodeCoAACK)
			}
			if got := resp.FindAttr(radius.AttrState); !bytes.Equal(got, tc.state) {
				t.Errorf("State returned: got %x, want %x", got, tc.state)
			}
			events := bus.recorded()
			if len(events) != 1 {
				t.Fatalf("changes made: got %d, want 1", len(events))
			}
			payload, ok := events[0].payload.(*l2tpevents.SessionRateChangePayload)
			if !ok {
				t.Fatalf("payload type: got %T", events[0].payload)
			}
			if payload.SessionID != 20 {
				t.Errorf("session changed: got %d, want 20", payload.SessionID)
			}
			if payload.DownloadRate != 10_000_000 {
				t.Errorf("rate: got %d, want 10000000", payload.DownloadRate)
			}
		})
	}
}

// RFC requirement: RFC5176-3.3-3 negative -- a State shaped as a rate, carried
// without any authorization-change attribute, is not read as a change: CoA-NAK
// with Error-Cause 401. A State naming the one existing session, carried with an
// Acct-Session-Id naming none, is not read as identification: CoA-NAK with
// Error-Cause 503. Neither makes a change, and each returns the State unmodified.
func TestRFC5176StateIsNeverReadAsAChangeOrAnIdentity(t *testing.T) {
	secret := []byte("test-rfc5176-3.3-neg-secret")
	bus := &recordingBus{}
	addr := coaTestListener(t, secret, bus, oneSessionService())

	rateState := []byte("10mbit")
	resp := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrState, Value: rateState},
	})
	wantNAK(t, resp, radius.CodeCoANAK, radius.ErrorCauseUnsupportedAttribute)
	if got := resp.FindAttr(radius.AttrState); !bytes.Equal(got, rateState) {
		t.Errorf("State returned: got %x, want %x", got, rateState)
	}

	sessionState := []byte("10-20-1")
	resp = sendCoAPacket(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("99-99-1")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
		{Type: radius.AttrState, Value: sessionState},
	})
	wantNAK(t, resp, radius.CodeCoANAK, radius.ErrorCauseSessionNotFound)
	if got := resp.FindAttr(radius.AttrState); !bytes.Equal(got, sessionState) {
		t.Errorf("State returned: got %x, want %x", got, sessionState)
	}
	if got := len(bus.recorded()); got != 0 {
		t.Errorf("changes made: got %d, want 0", got)
	}
}

// RFC requirement: RFC5176-3.5-6 positive -- across every response path the
// listener has (both ACKs, and NAKs carrying 401, 404, 405, 407, 503, 504, 506
// and 508), no CoA-ACK, CoA-NAK or Disconnect-NAK carries Error-Cause 201
// (Residual Session Context Removed).
func TestRFC5176ResidualSessionCauseOnlyInADisconnectACK(t *testing.T) {
	wantCauseOnlyIn(t, coaResponsePaths(t), radius.ErrorCauseResidualSession, radius.CodeDisconnectACK)
}

// RFC requirement: RFC5176-3.5-6 negative -- a CoA-Request and a
// Disconnect-Request each carrying Error-Cause 201, and a Proxy-State holding an
// Error-Cause 201 attribute, are refused with a NAK whose only Error-Cause is
// 401: the 201 the client sent never reaches a NAK.
func TestRFC5176ResidualSessionCauseInARequestNeverReachesANAK(t *testing.T) {
	wantForbiddenCauseNeverEchoed(t, radius.ErrorCauseResidualSession)
}

// RFC requirement: RFC5176-3.5-7 positive -- across every response path the
// listener has, no response carries Error-Cause 202 (Invalid EAP Packet
// (Ignored)).
func TestRFC5176InvalidEAPPacketCauseNeverSent(t *testing.T) {
	wantCauseOnlyIn(t, coaResponsePaths(t), radius.ErrorCauseInvalidEAPPacket)
}

// RFC requirement: RFC5176-3.5-7 negative -- a CoA-Request and a
// Disconnect-Request each carrying Error-Cause 202, and a Proxy-State holding an
// Error-Cause 202 attribute, are refused with a NAK whose only Error-Cause is
// 401, and a Disconnect-Request carrying a malformed EAP-Message is refused with
// 401, never 202.
func TestRFC5176InvalidEAPPacketCauseNeverSentForAnEAPRequest(t *testing.T) {
	wantForbiddenCauseNeverEchoed(t, radius.ErrorCauseInvalidEAPPacket)

	secret := []byte("test-rfc5176-3.5-7-secret")
	addr := coaTestListener(t, secret, &recordingBus{}, oneSessionService())
	resp := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: 79, Value: []byte{2, 1, 0, 9, 1}},
	})
	if got := errorCauses(t, resp); !slices.Equal(got, []uint32{radius.ErrorCauseUnsupportedAttribute}) {
		t.Errorf("malformed EAP-Message: Error-Cause %v, want [401]", got)
	}
}

// RFC requirement: RFC5176-3.5-8 positive -- across every response path the
// listener has, including a request for a session this NAS does not hold, no
// response carries Error-Cause 502 (Request Not Routable).
func TestRFC5176RequestNotRoutableCauseNeverSent(t *testing.T) {
	wantCauseOnlyIn(t, coaResponsePaths(t), 502)
}

// RFC requirement: RFC5176-3.5-8 negative -- a CoA-Request and a
// Disconnect-Request each carrying Error-Cause 502 are refused with 401 alone,
// and a Disconnect-Request naming a user of another realm through another NAS
// is answered with Error-Cause 503 (Session Context Not Found), never 502.
func TestRFC5176RequestNotRoutableCauseNeverSentForAForeignSession(t *testing.T) {
	wantForbiddenCauseNeverEchoed(t, 502)

	secret := []byte("test-rfc5176-3.5-8-secret")
	addr := coaTestListener(t, secret, &recordingBus{}, oneSessionService())
	resp := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, []radius.Attr{
		{Type: radius.AttrUserName, Value: radius.AttrString("bob@elsewhere.example")},
		{Type: radius.AttrNASIdentifier, Value: radius.AttrString("other-nas")},
	})
	if got := errorCauses(t, resp); !slices.Equal(got, []uint32{radius.ErrorCauseSessionNotFound}) {
		t.Errorf("foreign session: Error-Cause %v, want [503]", got)
	}
}

// RFC requirement: RFC5176-3.5-9 positive -- a Disconnect-Request matching a
// session whose teardown fails is answered with a Disconnect-NAK carrying
// Error-Cause 504 (Session Context Not Removable), and across every other
// response path no CoA-ACK, CoA-NAK or Disconnect-ACK carries 504.
func TestRFC5176SessionNotRemovableOnlyInADisconnectNAK(t *testing.T) {
	paths := coaResponsePaths(t)
	wantCauseOnlyIn(t, paths, radius.ErrorCauseSessionNotRemovable, radius.CodeDisconnectNAK)
	found := false
	for _, p := range paths {
		if p.name == "disconnect whose teardown fails" {
			found = true
		}
	}
	if !found {
		t.Fatal("the paths never drove a refused teardown")
	}
}

// RFC requirement: RFC5176-3.5-9 negative -- a CoA-Request and a
// Disconnect-Request each carrying Error-Cause 504 are refused with 401 alone,
// and a CoA-Request whose change cannot be carried out is answered with a
// CoA-NAK carrying 506 (Resources Unavailable), never 504.
func TestRFC5176SessionNotRemovableNeverInACoANAK(t *testing.T) {
	wantForbiddenCauseNeverEchoed(t, radius.ErrorCauseSessionNotRemovable)

	secret := []byte("test-rfc5176-3.5-9-secret")
	addr := coaTestListener(t, secret, nil, oneSessionService())
	resp := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
	})
	wantNAK(t, resp, radius.CodeCoANAK, radius.ErrorCauseResourcesUnavailable)
}

// RFC requirement: RFC5176-3.6-2 positive -- a Vendor-Specific Attribute in a
// CoA-Request is used for authorization change only: beside an Acct-Session-Id
// it is answered with a CoA-ACK and one 10 Mbit/s rate change on the session the
// Acct-Session-Id names.
func TestRFC5176VendorSpecificIsAChangeOnly(t *testing.T) {
	secret := []byte("test-rfc5176-3.6-secret")
	bus := &recordingBus{}
	addr := coaTestListener(t, secret, bus, oneSessionService())

	resp := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		vsa(t, radius.VendorMikrotik, radius.MikrotikRateLimit, "10M/10M"),
	})
	if resp.Code != radius.CodeCoAACK {
		t.Fatalf("code: got %d, want %d (CoA-ACK)", resp.Code, radius.CodeCoAACK)
	}
	events := bus.recorded()
	if len(events) != 1 {
		t.Fatalf("changes made: got %d, want 1", len(events))
	}
	payload, ok := events[0].payload.(*l2tpevents.SessionRateChangePayload)
	if !ok {
		t.Fatalf("payload type: got %T", events[0].payload)
	}
	if payload.SessionID != 20 {
		t.Errorf("session changed: got %d, want 20", payload.SessionID)
	}
	if payload.DownloadRate != 10_000_000 {
		t.Errorf("rate: got %d, want 10000000", payload.DownloadRate)
	}
}

// RFC requirement: RFC5176-3.6-2 negative -- a Vendor-Specific Attribute is
// never also used for identification: a CoA-Request carrying it with no
// identification attribute, or beside an Acct-Session-Id naming no session, is
// answered with a CoA-NAK carrying 503, and a Disconnect-Request carrying it is
// answered with a Disconnect-NAK carrying 401. No change and no teardown follow.
func TestRFC5176VendorSpecificIsNeverAnIdentity(t *testing.T) {
	secret := []byte("test-rfc5176-3.6-neg-secret")
	bus := &recordingBus{}
	fake := oneSessionService()
	addr := coaTestListener(t, secret, bus, fake)
	rate := vsa(t, radius.VendorMikrotik, radius.MikrotikRateLimit, "10M/10M")

	alone := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, []radius.Attr{rate})
	wantNAK(t, alone, radius.CodeCoANAK, radius.ErrorCauseSessionNotFound)

	elsewhere := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("99-99-1")},
		rate,
	})
	wantNAK(t, elsewhere, radius.CodeCoANAK, radius.ErrorCauseSessionNotFound)

	disconnect := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		rate,
	})
	wantNAK(t, disconnect, radius.CodeDisconnectNAK, radius.ErrorCauseUnsupportedAttribute)

	if got := len(bus.recorded()); got != 0 {
		t.Errorf("changes made: got %d, want 0", got)
	}
	if got := fake.teardowns.Load(); got != 0 {
		t.Errorf("teardowns: got %d, want 0", got)
	}
}

// replayWindowSlack is how far each side of the window the 6.3 tests probe.
// Event-Timestamp has one-second resolution, so the slack is well above it.
const replayWindowSlack = 10 * time.Second

// ageCoAReplay makes every cached response look age old.
func ageCoAReplay(cl *coaListener, age time.Duration) {
	cl.replayMu.Lock()
	defer cl.replayMu.Unlock()

	for key, entry := range cl.replay {
		entry.seen = time.Now().Add(-age)
		cl.replay[key] = entry
	}
}

// newWindowListener starts a listener trusting loopback over fake.
func newWindowListener(t *testing.T, secret []byte, fake *fakeL2TPService) *coaListener {
	t.Helper()
	l2tp.PublishService(fake)
	t.Cleanup(func() { l2tp.PublishService(nil) })
	cl, err := newCoAListener(coaListenerConfig{AllowedSources: coaLoopbackSources(), DefaultSecret: secret})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := cl.Close(); closeErr != nil {
			t.Logf("close: %v", closeErr)
		}
	})
	return cl
}

// RFC requirement: RFC5176-6.3-2 positive -- just inside the replay window both
// mechanisms still act: a duplicate whose first copy is the window less 10
// seconds old is answered from the cache (the same octets, no second teardown),
// and an Event-Timestamp that old is current (the request is answered).
func TestRFC5176DuplicateAndTimestampWindowsBothHoldInside(t *testing.T) {
	secret := []byte("test-rfc5176-6.3-secret")
	fake := oneSessionService()
	cl := newWindowListener(t, secret, fake)
	addr := cl.conn.LocalAddr().String()
	attrs := func() []radius.Attr {
		return []radius.Attr{{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")}}
	}
	inside := coaReplayWindow - replayWindowSlack

	wire := buildCoAPacket(t, radius.CodeDisconnectRequest, secret, attrs(), time.Now())
	first := coaRawExchange(t, addr, wire)
	ageCoAReplay(cl, inside)
	again := coaRawExchange(t, addr, wire)
	if !bytes.Equal(first, again) {
		t.Errorf("duplicate inside the window: response %x, want the cached %x", again, first)
	}
	if got := fake.teardowns.Load(); got != 1 {
		t.Errorf("teardowns after a duplicate inside the window: got %d, want 1", got)
	}

	resp := sendRawCoAPacket(t, addr,
		buildCoAPacket(t, radius.CodeDisconnectRequest, secret, attrs(), time.Now().Add(-inside)))
	if resp.Code != radius.CodeDisconnectACK {
		t.Errorf("Event-Timestamp inside the window: code %d, want %d", resp.Code, radius.CodeDisconnectACK)
	}
}

// RFC requirement: RFC5176-6.3-2 negative -- just past the replay window both
// mechanisms let go together: a duplicate whose first copy is the window plus 10
// seconds old is no longer answered from the cache (it is processed again, a
// second teardown), and an Event-Timestamp that old is stale (silently
// discarded, no teardown).
func TestRFC5176DuplicateAndTimestampWindowsBothEndOutside(t *testing.T) {
	secret := []byte("test-rfc5176-6.3-neg-secret")
	fake := oneSessionService()
	cl := newWindowListener(t, secret, fake)
	addr := cl.conn.LocalAddr().String()
	attrs := func() []radius.Attr {
		return []radius.Attr{{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")}}
	}
	outside := coaReplayWindow + replayWindowSlack

	wire := buildCoAPacket(t, radius.CodeDisconnectRequest, secret, attrs(), time.Now())
	coaRawExchange(t, addr, wire)
	ageCoAReplay(cl, outside)
	coaRawExchange(t, addr, wire)
	if got := fake.teardowns.Load(); got != 2 {
		t.Errorf("teardowns after a duplicate past the window: got %d, want 2", got)
	}

	sendRawCoAPacketExpectNoResponse(t, addr,
		buildCoAPacket(t, radius.CodeDisconnectRequest, secret, attrs(), time.Now().Add(-outside)))
	if got := fake.teardowns.Load(); got != 2 {
		t.Errorf("teardowns after a stale Event-Timestamp: got %d, want 2", got)
	}
}
