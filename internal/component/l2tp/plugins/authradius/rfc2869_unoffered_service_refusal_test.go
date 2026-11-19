// Design: docs/guide/l2tp.md -- RADIUS Change-of-Authorization and Disconnect
// Related: coa.go -- unsupportedAttr, which refuses an attribute the NAS does not support
// RFC: rfc/short/rfc2869.md -- Section 1.1, attributes of a service the NAS does not offer
//
// VALIDATES: the subscriber NAS, which offers PPP framed service and no ARAP,
// refuses a CoA-Request or Disconnect-Request carrying an ARAP attribute, and
// acts on the same request without it.
// PREVENTS: a NAS that honors, or silently ignores, a RADIUS attribute of a
// service it cannot deliver.

package l2tpauthradius

import (
	"testing"

	"github.com/ze-software/ze/internal/component/radius"
)

// rfc2869AttrARAPFeatures is ARAP-Features, RFC 2869 Section 5.10. The radius
// dictionary declares no constant for it on purpose (the radius package's
// rfc2869_unoffered_service_attributes_test.go refuses one), so the number is
// written here.
const rfc2869AttrARAPFeatures = 71

// rfc2869ARAPFeatures is an ARAP-Features value: 14 octets of change-password
// flag, minimum password length, password creation date, password expiration
// delta and current time. Its content is never read; the attribute type alone
// belongs to a service Ze does not offer.
var rfc2869ARAPFeatures = []byte{0, 6, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

// TestRFC2869SubscriberNASActsOnAnOfferedServiceRequest is the control for the
// refusal below: a CoA-Request and a Disconnect-Request built only from
// attributes of the PPP framed service Ze does offer are honored.
//
// RFC 2869 Section 1.1: "A NAS that does not implement a given service MUST NOT
// implement the RADIUS attributes for that service."
//
// RFC requirement: RFC2869-1.1-1 positive -- a CoA-Request naming a live
// session and a Filter-ID is answered with a CoA-ACK and applied, and a
// Disconnect-Request naming it is answered with a Disconnect-ACK and tears the
// session down, so the attributes of the offered service are implemented
// (coa.go unsupportedAttr, read by handlePacket).
func TestRFC2869SubscriberNASActsOnAnOfferedServiceRequest(t *testing.T) {
	secret := []byte("test-rfc2869-offered-secret")
	fake := oneSessionService(t)
	bus := &recordingBus{}
	addr := coaTestListener(t, secret, bus, fake)

	coa := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
	})
	if coa.Code != radius.CodeCoAACK {
		t.Errorf("CoA-Request code: got %d, want %d (CoA-ACK)", coa.Code, radius.CodeCoAACK)
	}
	if got := len(bus.recorded()); got != 1 {
		t.Errorf("events emitted for an offered-service CoA-Request: got %d, want 1", got)
	}

	disconnect := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
	})
	if disconnect.Code != radius.CodeDisconnectACK {
		t.Errorf("Disconnect-Request code: got %d, want %d (Disconnect-ACK)",
			disconnect.Code, radius.CodeDisconnectACK)
	}
	if got := fake.teardowns.Load(); got != 1 {
		t.Errorf("teardowns for an offered-service Disconnect-Request: got %d, want 1", got)
	}
}

// TestRFC2869SubscriberNASRefusesARAPAttributes sends the requests of the
// control above with one ARAP attribute added.
//
// RFC 2869 Section 1.1: "For example, a NAS that is unable to offer ARAP
// service MUST NOT implement the RADIUS attributes for ARAP." RFC 5176
// Section 2.3 gives the answer for an attribute the NAS does not support: a
// NAK, with Error-Cause 401 (Unsupported Attribute).
//
// RFC requirement: RFC2869-1.1-1 negative -- a CoA-Request and a
// Disconnect-Request carrying ARAP-Features are each answered with a NAK and
// Error-Cause 401, and neither changes an authorization nor tears the session
// down, so the NAS does not implement the ARAP attribute (coa.go
// unsupportedAttr, read by handlePacket).
func TestRFC2869SubscriberNASRefusesARAPAttributes(t *testing.T) {
	secret := []byte("test-rfc2869-noarap-secret")
	fake := oneSessionService(t)
	bus := &recordingBus{}
	addr := coaTestListener(t, secret, bus, fake)

	coa := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
		{Type: rfc2869AttrARAPFeatures, Value: rfc2869ARAPFeatures},
	})
	wantNAK(t, coa, radius.CodeCoANAK, radius.ErrorCauseUnsupportedAttribute)
	if got := len(bus.recorded()); got != 0 {
		t.Errorf("events emitted for a CoA-Request carrying ARAP-Features: got %d, want 0", got)
	}

	disconnect := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: rfc2869AttrARAPFeatures, Value: rfc2869ARAPFeatures},
	})
	wantNAK(t, disconnect, radius.CodeDisconnectNAK, radius.ErrorCauseUnsupportedAttribute)
	if got := fake.teardowns.Load(); got != 0 {
		t.Errorf("teardowns for a Disconnect-Request carrying ARAP-Features: got %d, want 0", got)
	}
}
