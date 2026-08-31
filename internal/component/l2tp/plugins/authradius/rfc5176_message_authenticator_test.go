// Design: docs/research/l2tpv2-ze-integration.md -- CoA/DM listener
// RFC: rfc/short/rfc5176.md -- Section 3.4 Message-Authenticator on a CoA-Request
// Related: coa.go -- coaListener.handlePacket, the only gate on an inbound request
//
// VALIDATES: that the CoA/DM listener acts on a Disconnect-Request signed the
// way RFC 5176 Section 3.4 fixes, and silently discards one whose
// Message-Authenticator does not verify, driven over UDP through the listener
// rather than through radius.VerifyCoAMessageAuthenticator alone.
// PREVENTS: the defect found on 2026-08-31. handlePacket verified the two
// authenticators in the inverted order, so every datagram a conformant Dynamic
// Authorization Client could produce was discarded and Ze answered none of them.
//
// The negative fixture carries a VALID Request Authenticator over a corrupted
// Message-Authenticator, so the Message-Authenticator check is the only gate
// that can refuse it.

package l2tpauthradius

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/radius"
)

// RFC requirement: RFC5176-3.4-3 positive -- a Disconnect-Request whose
// Message-Authenticator a Dynamic Authorization Client computed per RFC 5176
// Section 3.4 is not discarded: the listener acts on it and emits a response
// (coa.go coaListener.handlePacket).
func TestRFC5176ListenerAcceptsConformantMessageAuthenticator(t *testing.T) {
	secret := []byte("test-coa-ma-conformant")
	cl, err := newCoAListener(coaListenerConfig{DefaultSecret: secret})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cl.Close(); err != nil {
			t.Logf("close: %v", err)
		}
	}()

	resp := sendRawCoAPacket(t, cl.conn.LocalAddr().String(),
		buildCoAPacket(t, radius.CodeDisconnectRequest, secret, []radius.Attr{
			{Type: radius.AttrAcctSessionID, Value: radius.AttrString("999-999")},
		}, time.Now()))

	if resp.Code != radius.CodeDisconnectNAK {
		t.Errorf("code: got %d, want %d (Disconnect-NAK for an unknown session)", resp.Code, radius.CodeDisconnectNAK)
	}
}

// RFC requirement: RFC5176-3.4-3 negative -- a Disconnect-Request whose
// Message-Authenticator does not match the value RFC 5176 Section 3.4 names is
// silently discarded, so no response datagram is emitted, even though its
// Request Authenticator verifies (coa.go coaListener.handlePacket).
func TestRFC5176ListenerDiscardsWrongMessageAuthenticator(t *testing.T) {
	secret := []byte("test-coa-ma-wrong")
	cl, err := newCoAListener(coaListenerConfig{DefaultSecret: secret})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cl.Close(); err != nil {
			t.Logf("close: %v", err)
		}
	}()

	wire := buildCoAPacket(t, radius.CodeDisconnectRequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("999-999")},
	}, time.Now())

	// Corrupt the Message-Authenticator, then re-sign the Request Authenticator
	// over the corrupted packet: the first gate passes and only the second can
	// refuse this datagram.
	wire[messageAuthenticatorOffsetForTest(t, wire)]++
	signCoARequestAuthenticator(wire, secret)

	if !radius.VerifyCoARequestAuth(wire, secret) {
		t.Fatal("the fixture's Request Authenticator does not verify; the discard below would prove nothing about the Message-Authenticator")
	}

	sendRawCoAPacketExpectNoResponse(t, cl.conn.LocalAddr().String(), wire)
}
