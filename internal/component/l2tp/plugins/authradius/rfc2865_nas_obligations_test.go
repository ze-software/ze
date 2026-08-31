// RFC 2865 NAS obligations the L2TP subscriber Access-Request path owes.
//
// VALIDATES: an Access-Request always carries a credential attribute
// (Section 4.1), a zero-length text attribute is omitted rather than sent
// (Section 5), and an Access-Accept naming a Service-Type this NAS does not
// offer is treated as an Access-Reject (Sections 5.6 and 1.1).
// PREVENTS: a credential-less Access-Request reaching a RADIUS server, a
// zero-length User-Name on the wire, and a subscriber session brought up under
// an Access-Accept that authorizes a service the LNS cannot provide.

package l2tpauthradius

import (
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/radius"
)

// capturingRADIUS answers every request with code plus attrs and records the
// raw request bytes it read.
type capturingRADIUS struct {
	conn *net.UDPConn
	addr string
	reqs chan []byte
}

func startCapturingRADIUS(t *testing.T, sharedKey []byte, code uint8, attrs []radius.Attr) *capturingRADIUS {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	c := &capturingRADIUS{conn: conn, addr: conn.LocalAddr().String(), reqs: make(chan []byte, 8)}
	go func() {
		buf := make([]byte, radius.MaxPacketLen)
		for {
			n, from, readErr := conn.ReadFromUDP(buf)
			if readErr != nil {
				return
			}
			if n < radius.MinPacketLen {
				continue
			}
			cp := make([]byte, n)
			copy(cp, buf[:n])
			select {
			case c.reqs <- cp:
			default:
			}

			pkt := &radius.Packet{Code: code, Identifier: buf[1], Attrs: attrs}
			resp := make([]byte, radius.MaxPacketLen)
			nResp, encErr := pkt.EncodeTo(resp, 0)
			if encErr != nil {
				continue
			}
			resp = resp[:nResp]
			var reqAuth [radius.AuthenticatorLen]byte
			copy(reqAuth[:], buf[4:4+radius.AuthenticatorLen])
			auth := radius.ResponseAuthenticator(code, buf[1], uint16(nResp), reqAuth,
				resp[radius.HeaderLen:], sharedKey)
			copy(resp[4:4+radius.AuthenticatorLen], auth[:])
			conn.WriteToUDP(resp, from) //nolint:errcheck // test mock best-effort
		}
	}()
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // test cleanup
	return c
}

// setupCapturingAuth wires a radiusAuth to a capturing mock server that answers
// every request with an Access-Accept. Every caller here reads the
// Access-Request off the wire, or requires that none was sent, so the reply code
// is fixed.
func setupCapturingAuth(t *testing.T, sharedKey []byte, attrs []radius.Attr) (*radiusAuth, *capturingRADIUS, *fakeResponder) {
	t.Helper()
	srv := startCapturingRADIUS(t, sharedKey, radius.CodeAccessAccept, attrs)
	client, err := radius.NewClient(radius.ClientConfig{
		Servers: []radius.Server{{Address: srv.addr, SharedKey: sharedKey}},
		Timeout: 300 * time.Millisecond,
		Retries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() }) //nolint:errcheck // test cleanup
	a := newRADIUSAuth()
	a.swapClient(client, "test-nas", srv.addr, nil, "")
	return a, srv, newFakeResponder()
}

// TestRFC2865SubscriberAccessRequestCarriesCredential drives the auth handler
// for every credential method the LNS can meet. RFC 2865 Section 4.1 admits
// exactly three credential attributes, so a method that can produce none of them
// must not put an Access-Request on the wire at all.
func TestRFC2865SubscriberAccessRequestCarriesCredential(t *testing.T) {
	key := []byte("testing123")

	// RFC requirement: RFC2865-4.1-2 positive -- a PAP request carries the
	// User-Password attribute, one of the three Section 4.1 admits.
	a, srv, resp := setupCapturingAuth(t, key, nil)
	a.handle(ppp.EventAuthRequest{
		TunnelID: 1, SessionID: 2, Method: ppp.AuthMethodPAP,
		Username: "alice", Response: []byte("password123"),
	}, resp.respond)
	resp.waitOne(t)
	pap := decodeCaptured(t, srv)
	if pap.FindAttr(radius.AttrUserPassword) == nil {
		t.Fatal("a PAP Access-Request MUST carry a User-Password attribute")
	}

	// RFC requirement: RFC2865-4.1-2 positive -- a CHAP request carries the
	// CHAP-Password attribute instead.
	a, srv, resp = setupCapturingAuth(t, key, nil)
	a.handle(ppp.EventAuthRequest{
		TunnelID: 1, SessionID: 2, Method: ppp.AuthMethodCHAPMD5,
		Username: "alice", Identifier: 3,
		Challenge: make([]byte, 16), Response: make([]byte, 16),
	}, resp.respond)
	resp.waitOne(t)
	chap := decodeCaptured(t, srv)
	if chap.FindAttr(radius.AttrCHAPPassword) == nil {
		t.Fatal("a CHAP Access-Request MUST carry a CHAP-Password attribute")
	}

	// RFC requirement: RFC2865-4.1-2 negative -- a peer that offered no
	// credential (AuthMethodNone) yields no User-Password, no CHAP-Password and
	// no State, so no Access-Request is sent and the session is denied.
	a, srv, resp = setupCapturingAuth(t, key, nil)
	a.handle(ppp.EventAuthRequest{
		TunnelID: 1, SessionID: 2, Method: ppp.AuthMethodNone, Username: "alice",
	}, resp.respond)
	if call := resp.waitOne(t); call.accept {
		t.Fatal("a request with no credential MUST NOT be accepted")
	}
	assertNoRequest(t, srv, "AuthMethodNone")

	// RFC requirement: RFC2865-4.1-2 negative -- an MS-CHAPv2 response too short
	// to yield a peer challenge and an NT response produces no credential
	// attribute either, so no Access-Request is sent.
	a, srv, resp = setupCapturingAuth(t, key, nil)
	a.handle(ppp.EventAuthRequest{
		TunnelID: 1, SessionID: 2, Method: ppp.AuthMethodMSCHAPv2,
		Username: "alice", Challenge: make([]byte, 16), Response: make([]byte, 39),
	}, resp.respond)
	if call := resp.waitOne(t); call.accept {
		t.Fatal("a short MS-CHAPv2 response MUST NOT be accepted")
	}
	assertNoRequest(t, srv, "short MS-CHAPv2 response")
}

// TestRFC2865SubscriberZeroLengthUserNameOmitted reads the Access-Request off
// the wire for a peer that supplied no Peer-ID.
func TestRFC2865SubscriberZeroLengthUserNameOmitted(t *testing.T) {
	key := []byte("testing123")

	// RFC requirement: RFC2865-5-4 positive -- a non-empty Peer-ID is text of
	// non-zero length, so the User-Name attribute is sent and carries it.
	a, srv, resp := setupCapturingAuth(t, key, nil)
	a.handle(ppp.EventAuthRequest{
		TunnelID: 1, SessionID: 2, Method: ppp.AuthMethodPAP,
		Username: "alice", Response: []byte("pw"),
	}, resp.respond)
	resp.waitOne(t)
	if got := string(decodeCaptured(t, srv).FindAttr(radius.AttrUserName)); got != "alice" {
		t.Fatalf("User-Name: got %q, want %q", got, "alice")
	}

	// RFC requirement: RFC2865-5-4 negative -- a PAP peer may send Peer-ID-Length
	// 0, and text of length zero MUST NOT be sent, so the User-Name attribute is
	// omitted entirely rather than sent empty.
	a, srv, resp = setupCapturingAuth(t, key, nil)
	a.handle(ppp.EventAuthRequest{
		TunnelID: 1, SessionID: 2, Method: ppp.AuthMethodPAP,
		Username: "", Response: []byte("pw"),
	}, resp.respond)
	resp.waitOne(t)
	anonymous := decodeCaptured(t, srv)
	if anonymous.FindAttr(radius.AttrUserName) != nil {
		t.Fatal("a zero-length User-Name MUST be omitted, not sent empty")
	}
	if anonymous.FindAttr(radius.AttrUserPassword) == nil {
		t.Fatal("omitting User-Name MUST NOT drop the rest of the request")
	}
}

// TestRFC2865SubscriberServiceTypeAuthorization drives the subscriber login
// against an Access-Accept carrying a Service-Type. The LNS asks for Framed-User
// and provides no other service.
func TestRFC2865SubscriberServiceTypeAuthorization(t *testing.T) {
	key := []byte("testing123")
	framed := []radius.Attr{{Type: radius.AttrServiceType, Value: radius.AttrUint32(radius.ServiceTypeFramed)}}
	login := []radius.Attr{{Type: radius.AttrServiceType, Value: radius.AttrUint32(1)}}

	accept := func(t *testing.T, reply []radius.Attr) bool {
		t.Helper()
		a, resp, cleanup := setupAuthWithAttrs(t, key, radius.CodeAccessAccept, reply)
		defer cleanup()
		a.handle(ppp.EventAuthRequest{
			TunnelID: 1, SessionID: 2, Method: ppp.AuthMethodPAP,
			Username: "alice", Response: []byte("pw"),
		}, resp.respond)
		return resp.waitOne(t).accept
	}

	// RFC requirement: RFC2865-5.6-1 positive -- an Access-Accept whose
	// Service-Type is Framed-User names the service the LNS asked for and
	// provides, so the session comes up.
	if !accept(t, framed) {
		t.Fatal("Framed-User is the service the LNS offers and MUST be accepted")
	}

	// RFC requirement: RFC2865-5.6-1 negative -- an Access-Accept whose
	// Service-Type is Login-User names a service the LNS does not implement, so
	// it is treated as an Access-Reject.
	if accept(t, login) {
		t.Fatal("an unsupported Service-Type MUST be treated as an Access-Reject")
	}

	// RFC requirement: RFC2865-1.1-1 positive -- an Access-Accept carrying no
	// Service-Type authorizes the service the Access-Request named, which the LNS
	// provides, so the session comes up.
	if !accept(t, nil) {
		t.Fatal("an Accept with no Service-Type authorizes what was asked")
	}

	// RFC requirement: RFC2865-1.1-1 negative -- Service-Type 7 (NAS-Prompt-User)
	// authorizes a service this LNS cannot offer at all, so the Access-Accept is
	// treated as an Access-Reject rather than granting an unavailable service.
	if accept(t, []radius.Attr{{Type: radius.AttrServiceType, Value: radius.AttrUint32(7)}}) {
		t.Fatal("an Access-Accept authorizing an unavailable service MUST be a reject")
	}
}

func decodeCaptured(t *testing.T, srv *capturingRADIUS) *radius.Packet {
	t.Helper()
	select {
	case raw := <-srv.reqs:
		pkt, err := radius.Decode(raw)
		if err != nil {
			t.Fatalf("decode captured Access-Request: %v", err)
		}
		return pkt
	case <-time.After(5 * time.Second):
		t.Fatal("no Access-Request reached the server")
		return nil
	}
}

func assertNoRequest(t *testing.T, srv *capturingRADIUS, what string) {
	t.Helper()
	select {
	case <-srv.reqs:
		t.Fatalf("%s MUST NOT put an Access-Request on the wire", what)
	default:
	}
}
