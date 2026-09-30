// Design: docs/guide/l2tp.md -- RADIUS Change-of-Authorization and Disconnect
// Related: coa.go -- sendResponse, which copies Proxy-State and State into the reply
// RFC: rfc/short/rfc5176.md -- Section 3.1 Proxy-State
// RFC: rfc/short/rfc3579.md -- Section 1, EAP attributes on a NAS without EAP
//
// VALIDATES: the clauses of RFC 5176 Section 3.1 that TestRFC5176ProxyStateReturnedUnmodified
// leaves open: State and Class are not modified, Proxy-State is opaque, and the
// listener's decision does not depend on what a Proxy-State holds. Also that the
// subscriber NAS, which offers no EAP, refuses EAP-Message (RFC 3579 Section 1).
// PREVENTS: a listener that rewrites State on the way back, answers a Class it
// altered, or reads a Proxy-State value as if it were one of its own attributes.

package l2tpauthradius

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/radius"
)

// proxyStateLookingLikeASession is a Proxy-State value whose octets are a
// complete Acct-Session-Id attribute naming the live session "10-20-1". A
// listener that parsed Proxy-State content would find that session in it.
var proxyStateLookingLikeASession = []byte{radius.AttrAcctSessionID, 9, '1', '0', '-', '2', '0', '-', '1'}

// TestRFC5176StateAndClassUnmodifiedProxyStateOpaque sends the attributes a
// forwarding path carries and reads what the reply holds.
//
// VALIDATES: a CoA-Request's State comes back byte-equal in the CoA-ACK; a
// Disconnect-Request's Class is never answered altered (the reply carries it
// byte-equal or not at all); a Proxy-State of every octet value, including one
// that encodes an attribute, comes back byte-equal; two requests that differ
// only in Proxy-State content get the same code and cause the same teardown.
// PREVENTS: modification of State or Class, and any use of Proxy-State content.
//
// RFC requirement: RFC5176-3.1-1 positive -- "A forwarding proxy or NAS MUST
// NOT modify existing Proxy-State, State, or Class attributes present in the
// packet", Proxy-State is treated "as opaque data", and the operation does not
// "depend on the content of Proxy-State attributes" (coa.go sendResponse).
func TestRFC5176StateAndClassUnmodifiedProxyStateOpaque(t *testing.T) {
	secret := []byte("test-rfc5176-opaque-secret")
	fake := oneSessionService()
	bus := &recordingBus{}
	addr := coaTestListener(t, secret, bus, fake)

	state := []byte{0x00, 0x53, 0x54, 0xff}
	coa := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
		{Type: radius.AttrState, Value: state},
		{Type: radius.AttrProxyState, Value: proxyStateLookingLikeASession},
	})
	if coa.Code != radius.CodeCoAACK {
		t.Fatalf("CoA-Request carrying State: got code %d, want %d (CoA-ACK)", coa.Code, radius.CodeCoAACK)
	}
	if got := coa.FindAttr(radius.AttrState); !bytes.Equal(got, state) {
		t.Errorf("State in the CoA-ACK: got %x, want %x unmodified", got, state)
	}
	if got := coa.FindAllAttr(radius.AttrProxyState); len(got) != 1 || !bytes.Equal(got[0], proxyStateLookingLikeASession) {
		t.Errorf("Proxy-State in the CoA-ACK: got %x, want exactly %x", got, proxyStateLookingLikeASession)
	}

	every := make([]byte, 253)
	for index := range every {
		every[index] = byte(index)
	}
	class := []byte("class-from-the-server")
	outcomes := make([]uint8, 0, 2)
	for _, proxyState := range [][]byte{every, {0xaa}} {
		resp := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, []radius.Attr{
			{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
			{Type: radius.AttrClass, Value: class},
			{Type: radius.AttrProxyState, Value: proxyState},
		})
		outcomes = append(outcomes, resp.Code)
		for _, got := range resp.FindAllAttr(radius.AttrClass) {
			if !bytes.Equal(got, class) {
				t.Errorf("Class in the reply: got %x, want %x or no Class at all", got, class)
			}
		}
		if got := resp.FindAllAttr(radius.AttrProxyState); len(got) != 1 || !bytes.Equal(got[0], proxyState) {
			t.Errorf("Proxy-State of %d octets: got %x back, want it unmodified", len(proxyState), got)
		}
	}
	if outcomes[0] != radius.CodeDisconnectACK || outcomes[1] != radius.CodeDisconnectACK {
		t.Errorf("Disconnect outcomes by Proxy-State content: got %v, want both %d (Disconnect-ACK)",
			outcomes, radius.CodeDisconnectACK)
	}
	if got := fake.teardowns.Load(); got != 2 {
		t.Errorf("teardowns: got %d, want 2, one for each request whatever its Proxy-State held", got)
	}
}

// TestRFC3579SubscriberNASRefusesEAPMessage sends EAP-Message to the L2TP
// subscriber NAS, which authenticates PPP peers with PAP, CHAP and MS-CHAPv2
// and offers them no EAP.
//
// VALIDATES: a CoA-Request and a Disconnect-Request carrying EAP-Message are each
// refused with a NAK and Error-Cause 401 (Unsupported Attribute), and neither
// changes an authorization nor tears the session down.
// PREVENTS: a subscriber NAS that accepts an EAP attribute for a service it
// does not run.
//
// RFC requirement: RFC3579-1-1 negative -- "a NAS that is unable to offer EAP
// service MUST NOT implement the RADIUS attributes for EAP": the subscriber
// NAS does not implement EAP-Message and refuses it (coa.go unsupportedAttr,
// which reads coaSupportedAttrs and disconnectSupportedAttrs for handlePacket).
func TestRFC3579SubscriberNASRefusesEAPMessage(t *testing.T) {
	secret := []byte("test-rfc3579-noeap-secret")
	fake := oneSessionService()
	bus := &recordingBus{}
	addr := coaTestListener(t, secret, bus, fake)
	eapResponse := []byte{2, 1, 0, 6, 1, 'a'}

	coa := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
		{Type: radius.AttrEAPMessage, Value: eapResponse},
	})
	wantNAK(t, coa, radius.CodeCoANAK, radius.ErrorCauseUnsupportedAttribute)
	if got := len(bus.recorded()); got != 0 {
		t.Errorf("events emitted for a CoA-Request carrying EAP-Message: got %d, want 0", got)
	}

	disconnect := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrEAPMessage, Value: eapResponse},
	})
	wantNAK(t, disconnect, radius.CodeDisconnectNAK, radius.ErrorCauseUnsupportedAttribute)
	if got := fake.teardowns.Load(); got != 0 {
		t.Errorf("teardowns for a Disconnect-Request carrying EAP-Message: got %d, want 0", got)
	}
}

// TestRFC5176ProxyStateContentIsNeverRead gives the listener a Proxy-State that
// names a live session, beside identification that names none.
//
// VALIDATES: the request is refused with a Disconnect-NAK exactly as it is with
// no Proxy-State, and no session is torn down: the session named inside the
// Proxy-State is not found through it.
// PREVENTS: a listener whose session match reads Proxy-State content.
//
// RFC requirement: RFC5176-3.1-1 negative -- "Its operation MUST NOT depend on
// the content of Proxy-State attributes added by previous proxies": a request
// for an unknown session stays refused although its Proxy-State encodes the
// Acct-Session-Id of a live one (coa.go handleDisconnect).
func TestRFC5176ProxyStateContentIsNeverRead(t *testing.T) {
	secret := []byte("test-rfc5176-unread-secret")
	fake := oneSessionService()
	addr := coaTestListener(t, secret, nil, fake)

	unknown := []radius.Attr{{Type: radius.AttrAcctSessionID, Value: radius.AttrString("99-99-1")}}
	bare := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, unknown)
	carried := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, append(unknown,
		radius.Attr{Type: radius.AttrProxyState, Value: proxyStateLookingLikeASession}))

	if bare.Code != radius.CodeDisconnectNAK {
		t.Fatalf("unknown session without Proxy-State: got code %d, want %d (Disconnect-NAK)", bare.Code, radius.CodeDisconnectNAK)
	}
	if carried.Code != bare.Code {
		t.Errorf("unknown session with a session-shaped Proxy-State: got code %d, want %d as without it", carried.Code, bare.Code)
	}
	if got, want := carried.FindAttr(radius.AttrErrorCause), bare.FindAttr(radius.AttrErrorCause); !bytes.Equal(got, want) {
		t.Errorf("Error-Cause with a session-shaped Proxy-State: got %x, want %x as without it", got, want)
	}
	if got := fake.teardowns.Load(); got != 0 {
		t.Errorf("teardowns: got %d, want 0; the session named inside Proxy-State was reached", got)
	}
}
