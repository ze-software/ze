// RFC 5176 (Dynamic Authorization Extensions) NAS-side receiver obligations.
//
// VALIDATES: RFC 5176 Section 3.3 -- a request lacking any session-identification
// attribute cannot act on a session (the NAS NAKs and tears nothing down), and Section
// 3.5 -- the response the NAS emits carries a correct Response Authenticator.
//
// The authenticator-verification and silent-discard requirements (§3.5-1, §3.5-2) and the
// positive session-identification case (§3.3-1) are tagged on the existing listener tests
// in coa_test.go; this file adds the negative session-id case and the emitted-response
// authenticator check. Producers: coaListener.handleDisconnect/findSession (coa.go:259,
// 309) and coaListener.sendResponse (coa.go:377) via radius.ResponseAuthenticator
// (internal/component/radius/packet.go:145).

package l2tpauthradius

import (
	"net"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/l2tp"
	"codeberg.org/thomas-mangin/ze/internal/component/radius"
)

// RFC requirement: RFC5176-3.3-1 negative -- a Disconnect-Request carrying no
// session-identification attribute cannot identify a session, so the NAS acts on no
// session (no teardown) and returns a Disconnect-NAK.
func TestRFC5176NoSessionIdNotActedOn(t *testing.T) {
	secret := []byte("test-no-sessid-secret")
	fake := &fakeL2TPService{snap: l2tp.Snapshot{
		Tunnels: []l2tp.TunnelSnapshot{{
			LocalTID: 10,
			Sessions: []l2tp.SessionSnapshot{{
				LocalSID:       20,
				TunnelLocalTID: 10,
				Username:       "alice",
			}},
		}},
	}}
	l2tp.PublishService(fake)
	defer l2tp.PublishService(nil)

	cl, err := newCoAListener(0, nil, secret, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cl.Close(); err != nil {
			t.Logf("close: %v", err)
		}
	}()
	addr := cl.conn.LocalAddr().String()

	// Valid authenticator, but no Acct-Session-Id, User-Name, or NAS-Port: nothing
	// identifies the target session.
	resp := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, nil)

	if resp.Code != radius.CodeDisconnectNAK {
		t.Errorf("code: got %d, want %d (Disconnect-NAK)", resp.Code, radius.CodeDisconnectNAK)
	}
	if got := fake.teardowns.Load(); got != 0 {
		t.Errorf("teardowns: got %d, want 0 (no session identified, must not act)", got)
	}
}

// RFC requirement: RFC5176-3.5-3 positive -- the CoA/Disconnect response the NAS emits
// carries a Response Authenticator computed per RFC 2865 Section 3; it verifies against
// radius.ResponseAuthenticator for the request authenticator and shared secret.
func TestRFC5176ResponseAuthenticator(t *testing.T) {
	secret := []byte("test-respauth-secret")
	cl, err := newCoAListener(0, nil, secret, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cl.Close(); err != nil {
			t.Logf("close: %v", err)
		}
	}()
	addr := cl.conn.LocalAddr().String()

	// Unknown session -> Disconnect-NAK, but still an authenticated, signed response.
	wire := buildCoAPacket(t, radius.CodeDisconnectRequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("999-999")},
	}, time.Now())

	var reqAuth [radius.AuthenticatorLen]byte
	copy(reqAuth[:], wire[4:4+radius.AuthenticatorLen])

	conn, err := net.DialUDP("udp4", nil, mustResolveUDP(t, addr))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("conn close: %v", err)
		}
	}()

	if _, err := conn.Write(wire); err != nil {
		t.Fatal(err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	respBuf := make([]byte, radius.MaxPacketLen)
	rn, err := conn.Read(respBuf)
	if err != nil {
		t.Fatal("no response:", err)
	}

	if !radius.VerifyResponseAuth(respBuf[:rn], reqAuth, secret) {
		t.Error("emitted response carries an invalid Response Authenticator")
	}
}
