// RFC: rfc/short/rfc2865.md -- RFC2865-1.1-2 and RFC2865-5.6-1 on the L2TP subscriber login
// Related: handler.go -- doRADIUS, the Access-Accept authorization checks
// Related: rfc2865_handler_test.go -- setupAuthWithAttrs, fakeResponder

// VALIDATES: an Access-Accept is honored only when every service it authorizes
// is one the LNS provides: Service-Type Framed-User over Framed-Protocol PPP.
// PREVENTS: a subscriber session coming up under an authorization for a
// framing (SLIP, ARAP, X.75) or a Service-Type the LNS cannot bring up.
package l2tpauthradius

import (
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/radius"
)

// acceptOutcome drives one PAP subscriber login against a server that answers
// Access-Accept with reply, and returns whether the LNS let the session up.
func acceptOutcome(t *testing.T, reply []radius.Attr) bool {
	t.Helper()
	a, resp, cleanup := setupAuthWithAttrs(t, []byte("testing123"), radius.CodeAccessAccept, reply)
	defer cleanup()
	a.handle(ppp.EventAuthRequest{
		TunnelID: 1, SessionID: 2, Method: ppp.AuthMethodPAP,
		Username: "alice", Response: []byte("pw"),
	}, resp.respond)
	return resp.waitOne(t).accept
}

// TestRFC2865AcceptForPPPFramingIsHonored proves an Access-Accept whose
// Framed-Protocol is PPP, the one framing an L2TP LNS carries, brings the
// session up. Method: a mock server answers Framed-User plus Framed-Protocol 1.
func TestRFC2865AcceptForPPPFramingIsHonored(t *testing.T) {
	// RFC requirement: RFC2865-1.1-2 positive -- an Access-Accept authorizing
	// Framed-User over Framed-Protocol PPP names a service the LNS provides,
	// so the session comes up.
	if !acceptOutcome(t, []radius.Attr{
		{Type: radius.AttrServiceType, Value: radius.AttrUint32(radius.ServiceTypeFramed)},
		{Type: radius.AttrFramedProtocol, Value: radius.AttrUint32(radius.FramedProtocolPPP)},
	}) {
		t.Fatal("Framed-Protocol PPP is the service the LNS offers and MUST be accepted")
	}
}

// TestRFC2865AcceptForAFramingTheLNSLacksIsRejected proves an Access-Accept
// that authorizes framed access over a framing other than PPP is treated as an
// Access-Reject. The Service-Type is Framed-User, which the LNS supports, so
// only the Framed-Protocol names the unavailable service. Method: each framing
// RFC 2865 Section 5.7 lists beside PPP (SLIP, ARAP, Gandalf, Xylogics, X.75)
// and one value it does not list.
func TestRFC2865AcceptForAFramingTheLNSLacksIsRejected(t *testing.T) {
	for _, framing := range []uint32{2, 3, 4, 5, 6, 255} {
		// RFC requirement: RFC2865-1.1-2 negative -- an Access-Accept whose
		// Framed-Protocol is not PPP authorizes a service the LNS cannot bring
		// up, so it is treated as an Access-Reject.
		if acceptOutcome(t, []radius.Attr{
			{Type: radius.AttrServiceType, Value: radius.AttrUint32(radius.ServiceTypeFramed)},
			{Type: radius.AttrFramedProtocol, Value: radius.AttrUint32(framing)},
		}) {
			t.Fatalf("Framed-Protocol %d is unavailable on the LNS and MUST be treated as a reject", framing)
		}
	}
}

// TestRFC2865SupportedServiceTypeIsHonored proves an Access-Accept naming the
// Service-Type the LNS implements, Framed-User, brings the session up.
func TestRFC2865SupportedServiceTypeIsHonored(t *testing.T) {
	// RFC requirement: RFC2865-5.6-1 positive -- Service-Type Framed-User is
	// the service the LNS implements, so the Access-Accept is honored.
	if !acceptOutcome(t, []radius.Attr{
		{Type: radius.AttrServiceType, Value: radius.AttrUint32(radius.ServiceTypeFramed)},
	}) {
		t.Fatal("Framed-User is supported and MUST be accepted")
	}
}

// TestRFC2865UnknownOrUnsupportedServiceTypeIsARejection proves both halves of
// "unknown or unsupported": a Service-Type RFC 2865 defines and the LNS does
// not implement (Login-User 1, Callback-Framed-User 4), and a value RFC 2865
// does not define (65535), each make the Access-Accept an Access-Reject.
func TestRFC2865UnknownOrUnsupportedServiceTypeIsARejection(t *testing.T) {
	for _, serviceType := range []uint32{1, 4, 65535} {
		// RFC requirement: RFC2865-5.6-1 negative -- an unknown or unsupported
		// Service-Type is treated as though an Access-Reject had been received.
		if acceptOutcome(t, []radius.Attr{
			{Type: radius.AttrServiceType, Value: radius.AttrUint32(serviceType)},
		}) {
			t.Fatalf("Service-Type %d MUST be treated as an Access-Reject", serviceType)
		}
	}
}
