// Design: docs/guide/l2tp.md -- RADIUS accounting
// Related: handler.go -- doRADIUS, which sends the login's Access-Request and
//   reads the Access-Accept
// Related: acct.go -- onSessionIPAssigned, onSessionDown and buildAcctPacket,
//   which account the session that login opened
// Related: rfc2866_attribute_table_test.go -- the builder-level half of the
//   same Section 5.13 legend line
// RFC: rfc/short/rfc2866.md -- Section 5.13
//
// VALIDATES: RFC 2866 Section 5.13, "0 This attribute MUST NOT be present",
// over the five attributes the Accounting-Request table marks 0: User-Password,
// CHAP-Password, Reply-Message, State and CHAP-Challenge. The two tests run a
// subscriber login end to end: the registered provider takes its config over
// the plugin RPC, the registered auth handler sends the Access-Request, the
// server answers it, and the session's up and down events reach accounting
// through the event bus. The server decodes every datagram, so the assertions
// read what a server receives.
// METHOD: the negative gives the session every source of a 0-marked
// attribute a NAS holds. A CHAP login puts CHAP-Password and CHAP-Challenge on
// its Access-Request, a PAP login puts User-Password there, and the
// Access-Accept carries State and Reply-Message. The test first proves each
// value really was on the login's wire, then that no Accounting-Request of
// either session carries the attribute or its octets.
// PREVENTS: accounting that copies a subscriber's credentials, or the server's
// State and Reply-Message, into the records it bills from.

package l2tpauthradius

import (
	"bytes"
	"encoding/binary"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/radius"
)

// loginRADIUS is one server that authenticates and accounts, as the single
// server entry of an `l2tp auth radius` config does. It answers every
// Access-Request with an Access-Accept carrying acceptAttrs and every
// Accounting-Request with an Accounting-Response, and it keeps each request it
// decoded.
type loginRADIUS struct {
	mu       sync.Mutex
	access   []*radius.Packet
	accounts chan *radius.Packet
}

// startLoginRADIUS listens on loopback and answers with sharedKey. The server
// stops when the test ends.
func startLoginRADIUS(t *testing.T, sharedKey []byte, acceptAttrs []radius.Attr) (*loginRADIUS, string) {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // test cleanup
	srv := &loginRADIUS{accounts: make(chan *radius.Packet, 16)}
	go srv.serve(conn, sharedKey, acceptAttrs)
	return srv, conn.LocalAddr().String()
}

// serve answers until conn closes. A datagram that does not decode gets no
// answer, so the client under test times out and the test fails on the wait.
func (s *loginRADIUS) serve(conn *net.UDPConn, sharedKey []byte, acceptAttrs []radius.Attr) {
	buf := make([]byte, radius.MaxPacketLen)
	for {
		n, from, readErr := conn.ReadFromUDP(buf)
		if readErr != nil {
			return
		}
		pkt, decErr := radius.Decode(buf[:n])
		if decErr != nil {
			continue
		}
		var reply *radius.Packet
		switch pkt.Code {
		case radius.CodeAccessRequest:
			s.mu.Lock()
			s.access = append(s.access, pkt)
			s.mu.Unlock()
			reply = &radius.Packet{Code: radius.CodeAccessAccept, Identifier: pkt.Identifier, Attrs: acceptAttrs}
		case radius.CodeAccountingReq:
			s.accounts <- pkt
			reply = &radius.Packet{Code: radius.CodeAccountingResp, Identifier: pkt.Identifier}
		default:
			continue
		}
		resp := make([]byte, radius.MaxPacketLen)
		nResp, encErr := reply.EncodeTo(resp, 0)
		if encErr != nil {
			continue
		}
		resp = resp[:nResp]
		var reqAuth [radius.AuthenticatorLen]byte
		copy(reqAuth[:], buf[4:4+radius.AuthenticatorLen])
		auth := radius.ResponseAuthenticator(reply.Code, pkt.Identifier, uint16(nResp), reqAuth, resp[radius.HeaderLen:], sharedKey)
		copy(resp[4:4+radius.AuthenticatorLen], auth[:])
		conn.WriteToUDP(resp, from) //nolint:errcheck // test mock
	}
}

// accessRequestFor answers the Access-Request the server received for user.
func (s *loginRADIUS) accessRequestFor(t *testing.T, user string) *radius.Packet {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, pkt := range s.access {
		if string(pkt.FindAttr(radius.AttrUserName)) == user {
			return pkt
		}
	}
	t.Fatalf("the server received no Access-Request for %q", user)
	return nil
}

// nextAccounting answers the next Accounting-Request the server received.
func (s *loginRADIUS) nextAccounting(t *testing.T) *radius.Packet {
	t.Helper()
	select {
	case pkt := <-s.accounts:
		return pkt
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for an Accounting-Request")
		return nil
	}
}

// loginSession logs req in through the registered auth handler, brings its
// session up and down over the event bus, and answers the Start and the Stop
// the server received for it, in that order.
func loginSession(t *testing.T, bus *providerLifecycleBus, srv *loginRADIUS, req ppp.EventAuthRequest) (start, stop *radius.Packet) {
	t.Helper()
	t.Cleanup(func() { l2tp.ClearSessionMetadata(req.TunnelID, req.SessionID) })
	handler := l2tp.GetAuthHandler()
	if handler == nil {
		t.Fatal("no authentication provider is registered")
	}
	responder := newFakeResponder()
	result := handler(req, responder.respond)
	if !result.Handled {
		t.Fatalf("the RADIUS provider did not take the %q login (accept=%t, message=%q)", req.Username, result.Accept, result.Message)
	}
	if call := responder.waitOne(t); !call.accept {
		t.Fatalf("the %q login was refused: %q", req.Username, call.message)
	}

	if _, err := bus.Emit(l2tpevents.Namespace, l2tpevents.SessionIPAssignedEvent, &l2tpevents.SessionIPAssignedPayload{
		TunnelID: req.TunnelID, SessionID: req.SessionID, Username: req.Username, PeerAddr: "192.0.2.80",
	}); err != nil {
		t.Fatal(err)
	}
	start = srv.nextAccounting(t)
	if _, err := bus.Emit(l2tpevents.Namespace, l2tpevents.SessionDownEvent, &l2tpevents.SessionDownPayload{
		TunnelID: req.TunnelID, SessionID: req.SessionID, Username: req.Username, Cause: l2tpevents.TerminateCauseUserRequest,
	}); err != nil {
		t.Fatal(err)
	}
	stop = srv.nextAccounting(t)
	assertAcctStatus(t, req.Username, start, radius.AcctStatusStart)
	assertAcctStatus(t, req.Username, stop, radius.AcctStatusStop)
	return start, stop
}

// assertAcctStatus checks that pkt is the session's record of the given kind:
// an Accounting-Request naming the login's user.
func assertAcctStatus(t *testing.T, user string, pkt *radius.Packet, status uint8) {
	t.Helper()
	if pkt.Code != radius.CodeAccountingReq {
		t.Fatalf("%s: server received Code %d, want Accounting-Request", user, pkt.Code)
	}
	if v := pkt.FindAttr(radius.AttrAcctStatusType); len(v) != 4 || binary.BigEndian.Uint32(v) != uint32(status) {
		t.Fatalf("%s: Acct-Status-Type = %x, want %d", user, v, status)
	}
	if got := string(pkt.FindAttr(radius.AttrUserName)); got != user {
		t.Fatalf("%s: record names User-Name %q", user, got)
	}
	if len(pkt.FindAttr(radius.AttrAcctSessionID)) == 0 {
		t.Fatalf("%s: record carries no Acct-Session-Id", user)
	}
}

// assertNoZeroMarkedAttribute fails when pkt carries any attribute the Section
// 5.13 table marks 0, or any attribute whose value holds one of leaks.
func assertNoZeroMarkedAttribute(t *testing.T, where string, pkt *radius.Packet, leaks map[string][]byte) {
	t.Helper()
	for _, zero := range zeroMarkedAttrs {
		if pkt.FindAttr(zero.attr) != nil {
			t.Errorf("%s: Accounting-Request carries %s (type %d); RFC 2866 Section 5.13 marks it 0", where, zero.name, zero.attr)
		}
	}
	for name, leak := range leaks {
		for _, attr := range pkt.Attrs {
			if bytes.Contains(attr.Value, leak) {
				t.Errorf("%s: attribute type %d carries the login's %s", where, attr.Type, name)
			}
		}
	}
}

// loginAcceptServiceAttrs is what an Access-Accept for a framed PPP subscriber
// names, so the LNS accepts it (handler.go doRADIUS).
func loginAcceptServiceAttrs() []radius.Attr {
	return []radius.Attr{
		{Type: radius.AttrServiceType, Value: radius.AttrUint32(radius.ServiceTypeFramed)},
		{Type: radius.AttrFramedProtocol, Value: radius.AttrUint32(radius.FramedProtocolPPP)},
	}
}

// startLoginProvider registers the RADIUS provider against a fresh login
// server over the plugin RPC, with the event bus accounting subscribes to.
func startLoginProvider(t *testing.T, acceptAttrs []radius.Attr) (*providerLifecycleBus, *loginRADIUS) {
	t.Helper()
	preserveLifecycleLocal(t)
	bus := installProviderLifecycleBus(t)
	srv, address := startLoginRADIUS(t, []byte("lifecycle-key"), acceptAttrs)
	startAuthProvider(t, Name, lifecycleRadiusConfig(t, address, 1))
	return bus, srv
}

// TestRFC2866LoggedInSessionAccountsWithoutZeroMarkedAttributes logs a CHAP
// subscriber in against an Access-Accept that authorizes framed PPP and nothing
// else, then brings the session up and down.
//
// RFC 2866 Section 5.13: "0 This attribute MUST NOT be present".
//
// RFC requirement: RFC2866-5.13-1 positive -- the Start and the Stop a real
// CHAP login's session sends are Accounting-Requests naming its user and an
// Acct-Session-Id, and neither carries User-Password, CHAP-Password,
// Reply-Message, State or CHAP-Challenge (handler.go doRADIUS, acct.go
// onSessionIPAssigned, onSessionDown, buildAcctPacket).
func TestRFC2866LoggedInSessionAccountsWithoutZeroMarkedAttributes(t *testing.T) {
	bus, srv := startLoginProvider(t, loginAcceptServiceAttrs())

	start, stop := loginSession(t, bus, srv, ppp.EventAuthRequest{
		TunnelID: 80, SessionID: 21, Method: ppp.AuthMethodCHAPMD5, Identifier: 3,
		Username: "dave", Challenge: bytes.Repeat([]byte{0x5a}, 16), Response: bytes.Repeat([]byte{0x6b}, 16),
	})
	assertNoZeroMarkedAttribute(t, "Start", start, nil)
	assertNoZeroMarkedAttribute(t, "Stop", stop, nil)
}

// TestRFC2866LoginCredentialsAndAcceptStateNeverReachAccounting gives the
// accounting path every source of a 0-marked attribute a NAS holds, and reads
// the records it sends.
//
// RFC 2866 Section 5.13: "0 This attribute MUST NOT be present".
//
// A CHAP login and a PAP login run against an Access-Accept that carries State
// and Reply-Message. The test reads each Access-Request off the server first:
// the CHAP one carried CHAP-Password and CHAP-Challenge, the PAP one
// User-Password. The session therefore held every value, and a Start or Stop
// holding one is a copy, not an accident of the test.
//
// RFC requirement: RFC2866-5.13-1 negative -- after logins whose Access-Requests
// carried User-Password, CHAP-Password and CHAP-Challenge and whose
// Access-Accept carried State and Reply-Message, no Start or Stop of either
// session carries one of those attributes or the value any of them held
// (handler.go doRADIUS, acct.go buildAcctPacket).
func TestRFC2866LoginCredentialsAndAcceptStateNeverReachAccounting(t *testing.T) {
	state := []byte("state-cookie-4e1d")
	replyMessage := []byte("Welcome back, your plan renews monthly")
	acceptAttrs := append(loginAcceptServiceAttrs(),
		radius.Attr{Type: attrState, Value: state},
		radius.Attr{Type: radius.AttrReplyMessage, Value: replyMessage},
	)
	bus, srv := startLoginProvider(t, acceptAttrs)

	challenge := bytes.Repeat([]byte{0xc7}, 16)
	chapResponse := bytes.Repeat([]byte{0xd9}, 16)
	chapStart, chapStop := loginSession(t, bus, srv, ppp.EventAuthRequest{
		TunnelID: 81, SessionID: 22, Method: ppp.AuthMethodCHAPMD5, Identifier: 9,
		Username: "carol", Challenge: challenge, Response: chapResponse,
	})
	papPassword := []byte("pap-secret-9q")
	papStart, papStop := loginSession(t, bus, srv, ppp.EventAuthRequest{
		TunnelID: 81, SessionID: 23, Method: ppp.AuthMethodPAP,
		Username: "erin", Response: papPassword,
	})

	chapRequest := srv.accessRequestFor(t, "carol")
	if !bytes.Equal(chapRequest.FindAttr(radius.AttrCHAPChallenge), challenge) {
		t.Fatalf("the CHAP Access-Request carries CHAP-Challenge %x, want %x", chapRequest.FindAttr(radius.AttrCHAPChallenge), challenge)
	}
	chapPassword := chapRequest.FindAttr(radius.AttrCHAPPassword)
	if len(chapPassword) != 17 {
		t.Fatalf("the CHAP Access-Request carries a %d-octet CHAP-Password, want 17", len(chapPassword))
	}
	papRequest := srv.accessRequestFor(t, "erin")
	papHidden := papRequest.FindAttr(radius.AttrUserPassword)
	if len(papHidden) == 0 {
		t.Fatal("the PAP Access-Request carries no User-Password")
	}

	leaks := map[string][]byte{
		"State":                         state,
		"Reply-Message":                 replyMessage,
		"CHAP-Challenge":                challenge,
		"CHAP response":                 chapResponse,
		"CHAP-Password":                 chapPassword,
		"PAP password":                  papPassword,
		"User-Password as the NAS sent": papHidden,
	}
	assertNoZeroMarkedAttribute(t, "CHAP Start", chapStart, leaks)
	assertNoZeroMarkedAttribute(t, "CHAP Stop", chapStop, leaks)
	assertNoZeroMarkedAttribute(t, "PAP Start", papStart, leaks)
	assertNoZeroMarkedAttribute(t, "PAP Stop", papStop, leaks)
}
