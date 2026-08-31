// RFC 2865 obligations over the shape of the packet ze transmits.
//
// VALIDATES: the Code field is bound to the wish to authenticate a user
// (Section 4.1), an Access-Request names this NAS even when the operator
// configured neither identity leaf (Section 4.1, restated at Section 5.32), and
// the User-Name attribute is sent only when the name is available
// (Section 5.1).
// PREVENTS: Code 1 written on every packet regardless of intent, an
// Access-Request that carries neither NAS-IP-Address nor NAS-Identifier, and a
// User-Name attribute invented for a peer that supplied no name.
//
// ze is the RADIUS client (NAS) on both paths, so each requirement here binds
// the packet ze builds. The producers exercised are appendNASIdentity and
// radius.AppendTextAttr through the live doRADIUS path, and buildAcctPacket.

package l2tpauthradius

import (
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/radius"
)

// setupCapturingAuthIdentity is setupCapturingAuth with the two NAS identity
// leaves under the caller's control. What ze sends when the operator set
// neither of them is the obligation this file checks, and setupCapturingAuth
// fixes both.
func setupCapturingAuthIdentity(t *testing.T, sharedKey []byte, nasID string,
	sourceAddr net.IP) (*radiusAuth, *capturingRADIUS, *fakeResponder) {
	t.Helper()
	srv := startCapturingRADIUS(t, sharedKey, radius.CodeAccessAccept, nil)
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
	a.swapClient(client, nasID, srv.addr, sourceAddr, "")
	return a, srv, newFakeResponder()
}

// RFC requirement: RFC2865-4.1-2 negative -- the configuration that would break
// the obligation is one that sets neither source-address nor nas-identifier.
// Under it the Access-Request ze puts on the wire still names this NAS: it
// carries a NAS-Identifier of non-zero length, and no NAS-IP-Address is
// invented for an address the operator never gave.
func TestRFC2865AccessRequestNamesTheNASWithNoIdentityConfigured(t *testing.T) {
	a, srv, resp := setupCapturingAuthIdentity(t, []byte("testing123"), "", nil)
	a.handle(ppp.EventAuthRequest{
		TunnelID: 1, SessionID: 2, Method: ppp.AuthMethodPAP,
		Username: "alice", Response: []byte("password123"),
	}, resp.respond)
	resp.waitOne(t)

	pkt := decodeCaptured(t, srv)
	if pkt.FindAttr(radius.AttrNASIPAddress) != nil {
		t.Error("no source-address is configured, so no NAS-IP-Address is invented")
	}
	nasID := pkt.FindAttr(radius.AttrNASIdentifier)
	if nasID == nil {
		t.Fatal("an Access-Request MUST contain either a NAS-IP-Address or a NAS-Identifier")
	}
	if len(nasID) == 0 {
		t.Error("NAS-Identifier is text of length zero, which names no NAS")
	}
}

// RFC requirement: RFC2865-5-1 negative -- the obligation is conditional on the
// name being available, and a PAP peer that sent Peer-ID-Length 0 leaves it
// unavailable. That login's Access-Request reaches the wire carrying its
// User-Password credential and no User-Name attribute at all.
func TestRFC2865AccessRequestOmitsAnUnavailableUserName(t *testing.T) {
	a, srv, resp := setupCapturingAuthIdentity(t, []byte("testing123"), "lns1", nil)
	a.handle(ppp.EventAuthRequest{
		TunnelID: 1, SessionID: 2, Method: ppp.AuthMethodPAP,
		Username: "", Response: []byte("password123"),
	}, resp.respond)
	resp.waitOne(t)

	pkt := decodeCaptured(t, srv)
	if pkt.FindAttr(radius.AttrUserName) != nil {
		t.Error("the peer supplied no name, so no User-Name attribute is sent")
	}
	if pkt.FindAttr(radius.AttrUserPassword) == nil {
		t.Error("the request is still an Access-Request and still carries its credential")
	}
}

// RFC requirement: RFC2865-4.1-1 negative -- Code 1 is bound to the wish to
// authenticate a user rather than written on every packet ze sends. The
// accounting path authenticates nobody, and the record it builds for each of
// the three Acct-Status-Type values carries 4 (Accounting-Request).
func TestRFC2865AccountingRequestDoesNotCarryTheAccessRequestCode(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{username: "alice", acctSessID: "1-2-1", peerAddr: "192.0.2.10"}

	for _, status := range acctStatuses {
		pkt := acct.buildAcctPacket(sess, "lns1", nil, status, 60)
		if pkt.Code != radius.CodeAccountingReq {
			t.Errorf("status %d: Code field: got %d, want %d (Accounting-Request)",
				status, pkt.Code, radius.CodeAccountingReq)
		}
	}
}
