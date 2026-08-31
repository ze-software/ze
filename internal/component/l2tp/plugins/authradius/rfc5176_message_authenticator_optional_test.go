// Design: docs/research/l2tpv2-ze-integration.md -- CoA/DM listener
// RFC: rfc/short/rfc5176.md -- Section 3.4, the Message-Authenticator MAY
// Related: coa.go -- coaListener.handlePacket, the only gate on an inbound request
// Related: yang/ze-l2tp-auth-radius-conf.yang -- leaf require-message-authenticator
//
// VALIDATES: that the Message-Authenticator attribute is optional on the wire,
// which is what RFC 5176 Section 3.4 says, and that an operator who wants it
// mandatory gets that from `require-message-authenticator` rather than from the
// default. Both cases are driven over UDP through the listener.
// PREVENTS: the interop failure found on 2026-08-31. handlePacket discarded
// every CoA-Request and Disconnect-Request carrying no Message-Authenticator,
// so a conformant Dynamic Authorization Client that omits the attribute, which
// accel-ppp's dm_coa sender does unless blast-protection is on, was answered by
// nothing at all.

package l2tpauthradius

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/radius"
)

// RFC requirement: RFC5176-3.4-4 positive -- with require-message-authenticator
// false, the YANG default, a Disconnect-Request carrying no Message-Authenticator
// is acted on: the session it names is torn down and a Disconnect-ACK is sent
// (coa.go coaListener.handlePacket).
//
// The teardown count is asserted, not just the response code, because a reply
// alone would not separate "acted on" from "answered".
func TestRFC5176MessageAuthenticatorAbsentIsAcceptedByDefault(t *testing.T) {
	secret := []byte("test-coa-ma-optional")
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

	cl, err := newCoAListener(coaListenerConfig{DefaultSecret: secret})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cl.Close(); err != nil {
			t.Logf("close: %v", err)
		}
	}()

	wire := buildCoAPacketWithoutMessageAuthenticator(t, radius.CodeDisconnectRequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
	}, time.Now())

	resp := sendRawCoAPacket(t, cl.conn.LocalAddr().String(), wire)
	if resp.Code != radius.CodeDisconnectACK {
		t.Errorf("code: got %d, want %d (Disconnect-ACK)", resp.Code, radius.CodeDisconnectACK)
	}
	if got := fake.teardowns.Load(); got != 1 {
		t.Errorf("teardowns: got %d, want 1", got)
	}
}

// RFC requirement: RFC5176-3.4-3 negative -- with require-message-authenticator
// false, a Disconnect-Request whose Message-Authenticator is present and wrong is
// still silently discarded. The leaf gates the PRESENCE check alone; the
// verification RFC 5176 Section 3.4 makes mandatory is never configurable
// (coa.go coaListener.handlePacket).
func TestRFC5176WrongMessageAuthenticatorDiscardedWhenNotRequired(t *testing.T) {
	secret := []byte("test-coa-ma-wrong-not-required")
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
